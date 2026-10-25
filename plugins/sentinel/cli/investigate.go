//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/pkg/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// semProvider é o erro de quando nenhum provider de IA responde.
func semProvider() error {
	return yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA disponível").
		WithHint("rode ollama serve, exporte OPENAI_API_KEY ou GEMINI_API_KEY, ou defina ai.provider no ~/.yby/config.yaml")
}

// investigate junta logs, eventos e métricas do pod e, se ele não estiver saudável, pede
// à IA a causa provável e a correção. O título e as etapas vão para saida, ou para erros
// quando o resultado é para scripts.
func investigate(ctx context.Context, saida, erros io.Writer, op opcoes) error {
	podName, namespace := op.pod, op.namespace

	k8sClient, err := novoClienteKube()
	if err != nil {
		return semCluster(err)
	}

	// O pod é lido antes das etapas: um pod que não existe, sem permissão ou um cluster
	// que não responde param aqui, antes de logs e eventos que só repetiriam a falha.
	pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return erroAoLerPod(err, podName, namespace)
	}

	// O título só sai depois dessas verificações rápidas, para o erro delas não ficar
	// separado dele por duas linhas em branco.
	tela := saida
	if op.paraScripts() {
		tela = erros
	} else {
		ui.Title(saida, "sentinel", "investigate · "+namespace+"/"+podName)
		fmt.Fprintln(saida)
	}

	prog := novoProgresso(tela)
	parar := animar(prog, erros)
	defer parar()

	// 1. Logs
	prog.Iniciar("logs")
	tailLines := int64(50)
	podLogs, err := k8sClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	}).DoRaw(ctx)
	logsStr := ""
	if err != nil {
		prog.Falhar("indisponíveis")
		prog.Detalhe(err.Error())
	} else {
		logsStr = string(podLogs)
		prog.Concluir(contar(contarLinhas(logsStr), "linha", "linhas"))
	}

	// Análise guardada para os mesmos logs
	if !op.semCache {
		if cached, ok := loadCache(namespace, podName, logsStr); ok {
			prog.Iniciar("cache")
			prog.Concluir("análise guardada")
			parar()
			return mostrarAnalise(saida, *cached, podName, namespace, op, true)
		}
	}

	// 2. Eventos
	prog.Iniciar("eventos")
	events, err := k8sClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
	})
	eventsStr := "[]"
	if err != nil {
		prog.Falhar("indisponíveis")
		prog.Detalhe(err.Error())
	} else {
		eventsBytes, _ := json.Marshal(events.Items)
		eventsStr = string(eventsBytes)
		prog.Concluir(contar(len(events.Items), "evento", "eventos"))
	}

	// 3. Métricas: o client-go não traz a API de métricas; o kubectl top serve quando o
	// contexto diz qual kubeconfig usar.
	prog.Iniciar("métricas")
	metricsStr := "métricas indisponíveis"
	if fullCtx := sdk.GetFullContext(); fullCtx != nil {
		args := []string{"top", "pod", podName, "-n", namespace, "--no-headers"}
		if fullCtx.Infra.KubeConfig != "" {
			args = append(args, "--kubeconfig", fullCtx.Infra.KubeConfig)
		}
		if fullCtx.Infra.KubeContext != "" {
			args = append(args, "--context", fullCtx.Infra.KubeContext)
		}
		if out, err := execCommand("kubectl", args...).CombinedOutput(); err == nil {
			metricsStr = string(out)
			prog.Concluir("kubectl top")
		} else {
			slog.Debug("kubectl top falhou", "erro", err, "saida", string(out))
			prog.Pular("indisponíveis")
		}
	} else {
		prog.Pular("indisponíveis")
	}

	// Pod saudável: nada para a IA analisar. Nos formatos para scripts, o relatório de pod
	// saudável vai para o stdout (ou para o arquivo), para quem lê o resultado não receber
	// um stdout vazio.
	if isPodHealthy(pod, events) {
		parar()
		escreverPodSaudavel(tela, pod)
		return relatorioPodSaudavel(saida, pod, op)
	}

	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s\n\nMETRICS:\n%s", logsStr, eventsStr, metricsStr)
	if strings.TrimSpace(logsStr) == "" && (events == nil || len(events.Items) == 0) {
		parar()
		return yerrors.New(yerrors.ErrCodeValidation, "não há logs nem eventos do pod para analisar").
			WithContext("pod", namespace+"/"+podName).
			WithHint("confira o nome do pod e o namespace com kubectl get pods -n " + namespace)
	}

	provider := provedorDeIA(ctx)
	if provider == nil {
		parar()
		return semProvider()
	}

	// 4. Análise da IA
	prog.Iniciar("análise da IA")
	analysisJSON, err := provider.Completion(ctx, prompts.Get("sentinel.investigate"), realContext)
	if err != nil {
		prog.Falhar("falhou")
		parar()
		return yerrors.Wrap(err, yerrors.ErrCodePlugin, "a IA não respondeu").
			WithHint("rode de novo em instantes ou troque o provider em ai.provider no ~/.yby/config.yaml")
	}

	// Tirar a cerca de código markdown, se houver (```json ... ```)
	analysisClean := strings.ReplaceAll(analysisJSON, "```json", "")
	analysisClean = strings.ReplaceAll(analysisClean, "```", "")
	var result AnalysisResult
	if err := json.Unmarshal([]byte(analysisClean), &result); err != nil {
		prog.Falhar("resposta inválida")
		parar()
		slog.Debug("resposta da IA fora do formato", "resposta", analysisJSON)
		// O --log-level do core não chega ao plugin, que roda em outro processo: o nível
		// do log vem do YBY_LOG_LEVEL (ou do log.level do config).
		return yerrors.Wrap(err, yerrors.ErrCodePlugin, "a resposta da IA não veio no formato esperado").
			WithContext("resposta", inicioDaResposta(analysisJSON)).
			WithHint("rode de novo; para ver a resposta inteira no log, rode com YBY_LOG_LEVEL=debug")
	}
	prog.Concluir(provider.Name())
	parar()

	saveCache(namespace, podName, logsStr, result)
	return mostrarAnalise(saida, result, podName, namespace, op, false)
}

// erroAoLerPod classifica a falha de ler o pod: ele não existe, falta permissão ou o
// cluster não responde (o caso de todo o resto, como no scan).
func erroAoLerPod(err error, podName, namespace string) error {
	switch {
	case apierrors.IsNotFound(err):
		return yerrors.Wrap(err, yerrors.ErrCodeValidation, fmt.Sprintf("o pod %s não existe no namespace %s", podName, namespace)).
			WithHint("confira o nome com kubectl get pods -n " + namespace)
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		return yerrors.Wrap(err, yerrors.ErrCodeValidation, fmt.Sprintf("sem permissão para ler o pod %s no namespace %s", podName, namespace)).
			WithHint("confira as permissões com kubectl auth can-i get pods -n " + namespace)
	}
	return yerrors.Wrap(err, yerrors.ErrCodeClusterOffline, "não deu para consultar o cluster").
		WithContext("pod", namespace+"/"+podName).
		WithHint("confira se o cluster responde com kubectl get pods -n " + namespace)
}

// maxInicioResposta limita o trecho da resposta da IA que vai no contexto do erro.
const maxInicioResposta = 120

// inicioDaResposta devolve o começo da resposta da IA numa linha só, para o contexto do
// erro.
func inicioDaResposta(resposta string) string {
	r := strings.Join(strings.Fields(resposta), " ")
	if runas := []rune(r); len(runas) > maxInicioResposta {
		return string(runas[:maxInicioResposta]) + "…"
	}
	return r
}

// relatorioPodSaudavel grava, nos formatos json e markdown, o relatório de pod saudável no
// stdout ou no arquivo do -f; no formato terminal não faz nada.
func relatorioPodSaudavel(saida io.Writer, pod *corev1.Pod, op opcoes) error {
	phase := string(pod.Status.Phase)
	switch op.formato {
	case "json":
		content, err := exportHealthyJSON(op.pod, op.namespace, phase)
		if err != nil {
			return yerrors.Wrap(err, yerrors.ErrCodePlugin, "não deu para montar o relatório JSON")
		}
		return gravarRelatorio(saida, content, op.arquivo, "json")
	case "markdown":
		return gravarRelatorio(saida, exportHealthyMarkdown(op.pod, op.namespace, phase), op.arquivo, "markdown")
	}
	return nil
}

// contarLinhas conta as linhas de um texto (a última pode não ter \n).
func contarLinhas(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// escreverPodSaudavel diz que o pod está saudável e mostra os contêineres.
func escreverPodSaudavel(w io.Writer, pod *corev1.Pod) {
	fmt.Fprintln(w)
	ui.Check(w, ui.CheckOk, "pod saudável", string(pod.Status.Phase), "nenhum sinal de problema; a IA não foi chamada")
	if len(pod.Status.ContainerStatuses) == 0 {
		return
	}
	ui.Section(w, "contêineres")
	for _, cs := range pod.Status.ContainerStatuses {
		tipo, estado := ui.CheckOk, "pronto"
		if !cs.Ready {
			tipo, estado = ui.CheckAviso, "não pronto"
		}
		ui.Check(w, tipo, cs.Name, estado, contar(int(cs.RestartCount), "reinício", "reinícios"))
	}
}

// mostrarAnalise exporta a análise (json ou markdown) ou a escreve na tela.
func mostrarAnalise(saida io.Writer, result AnalysisResult, podName, namespace string, op opcoes, doCache bool) error {
	switch op.formato {
	case "json":
		content, err := exportJSON(result, podName, namespace)
		if err != nil {
			return yerrors.Wrap(err, yerrors.ErrCodePlugin, "não deu para montar o relatório JSON")
		}
		return gravarRelatorio(saida, content, op.arquivo, "json")
	case "markdown":
		return gravarRelatorio(saida, exportMarkdown(result, podName, namespace), op.arquivo, "markdown")
	}
	fmt.Fprintln(saida)
	escreverAnalise(saida, result)
	if doCache {
		fmt.Fprintln(saida)
		ui.Campos(saida, ui.Campo{Rotulo: "origem", Valor: "análise guardada; --no-cache pergunta à IA de novo"})
	}
	return nil
}

// isPodHealthy verifica se o pod está saudável baseado no status e eventos.
// Retorna true se não há sinais de problema.
func isPodHealthy(pod *corev1.Pod, events *corev1.EventList) bool {
	// Pod não está Running
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Algum container não está Ready ou tem restarts recentes
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
		if cs.RestartCount > 0 {
			// Verificar se o restart foi recente (últimas 2 horas)
			if cs.LastTerminationState.Terminated != nil {
				// Tem terminação recente → não saudável
				return false
			}
		}
		// Container em estado de waiting (CrashLoopBackOff, ImagePullBackOff, etc.)
		if cs.State.Waiting != nil {
			return false
		}
	}

	// Verificar eventos de Warning
	if events != nil {
		for _, e := range events.Items {
			if e.Type == "Warning" {
				return false
			}
		}
	}

	return true
}
