//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/pkg/redacao"
	"github.com/casheiro/yby-cli/pkg/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	// 1. Logs: um pedido por contêiner, e o da execução anterior nos que reiniciaram
	prog.Iniciar("logs")
	logsStr, nLinhas, errLogs := lerLogsDoPod(ctx, k8sClient, pod)
	if errLogs != nil {
		prog.Falhar("indisponíveis")
		prog.Detalhe(errLogs.Error())
	} else {
		prog.Concluir(contar(nLinhas, "linha", "linhas"))
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
		eventsBytes, _ := json.Marshal(resumirEventos(events.Items))
		eventsStr = string(eventsBytes)
		prog.Concluir(contar(len(events.Items), "evento", "eventos"))
	}

	// Pod saudável: nada para a IA analisar. Vem antes do cache, para uma análise guardada
	// de quando o pod estava com problema não reaparecer depois que ele se recuperou. Nos
	// formatos para scripts, o relatório de pod saudável vai para o stdout (ou para o
	// arquivo), para quem lê o resultado não receber um stdout vazio.
	if isPodHealthy(pod, events) {
		parar()
		escreverPodSaudavel(tela, pod)
		return relatorioPodSaudavel(saida, pod, op)
	}

	// Análise guardada para o mesmo pod (contexto, UID e reinícios) e os mesmos logs
	id := identidadeDoPod(pod)
	if !op.semCache {
		if cached, ok := loadCache(id, logsStr); ok {
			prog.Iniciar("cache")
			prog.Concluir("análise guardada")
			parar()
			return mostrarAnalise(saida, *cached, podName, namespace, op, true)
		}
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

	// O provider pode ser remoto: tokens, senhas e chaves dos logs, dos eventos e das
	// métricas são redigidos antes de sair da máquina. Cada parte é redigida sozinha,
	// para o fim de uma não ser lido como valor do título da seguinte.
	realContext := fmt.Sprintf("LOGS:\n%s\n\nEVENTS (JSON):\n%s\n\nMETRICS:\n%s",
		redacao.Redigir(logsStr), redacao.Redigir(eventsStr), redacao.Redigir(metricsStr))
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
		// A causa do encoding/json vem em inglês; na tela vai a causa em PT-BR, e o
		// detalhe técnico, para o log de depuração.
		return yerrors.Wrap(ai.CausaForaDoFormato(err), yerrors.ErrCodePlugin, "não deu para ler a análise da IA").
			WithContext("resposta", inicioDaResposta(analysisJSON)).
			WithHint("rode de novo; para ver a resposta inteira no log, rode com YBY_LOG_LEVEL=debug")
	}
	prog.Concluir(provider.Name())
	parar()

	saveCache(id, logsStr, result)
	return mostrarAnalise(saida, result, podName, namespace, op, false)
}

// erroAoLerPod classifica a falha de ler o pod: ele não existe, falta permissão ou o
// cluster não responde (o caso de todo o resto, como no scan).
func erroAoLerPod(err error, podName, namespace string) error {
	switch {
	case apierrors.IsNotFound(err):
		// A causa do Kubernetes ('pods "x" not found', em inglês) só repetiria o título.
		slog.Debug("o pod não existe", "pod", namespace+"/"+podName, "erro", err)
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o pod %s não existe no namespace %s", podName, namespace)).
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

// janelaDeReinicio é o quanto um reinício conta como recente na checagem de saúde.
const janelaDeReinicio = 2 * time.Hour

// isPodHealthy verifica se o pod está saudável baseado no status e eventos.
// Retorna true se não há sinais de problema.
func isPodHealthy(pod *corev1.Pod, events *corev1.EventList) bool {
	return podSaudavelEm(pod, events, time.Now())
}

// podSaudavelEm é o isPodHealthy com o relógio explícito. Um pod Succeeded (job que
// terminou) é saudável; um reinício só conta se a terminação foi nas últimas 2 horas.
func podSaudavelEm(pod *corev1.Pod, events *corev1.EventList, agora time.Time) bool {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		// os contêineres terminaram, então não estão Ready: não há o que checar neles
	case corev1.PodRunning:
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return false
			}
			if cs.RestartCount > 0 {
				if t := cs.LastTerminationState.Terminated; t != nil && agora.Sub(t.FinishedAt.Time) < janelaDeReinicio {
					return false
				}
			}
			// Container em estado de waiting (CrashLoopBackOff, ImagePullBackOff, etc.)
			if cs.State.Waiting != nil {
				return false
			}
		}
	default:
		return false
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

// opcoesDeLogs monta um pedido de logs por contêiner, com o da execução anterior
// (Previous) logo depois do dos contêineres que reiniciaram. Sem a lista de contêineres
// no spec, vale um pedido só, sem nome.
func opcoesDeLogs(pod *corev1.Pod, tail int64) []corev1.PodLogOptions {
	if len(pod.Spec.Containers) == 0 {
		return []corev1.PodLogOptions{{TailLines: &tail}}
	}
	reinicios := map[string]int32{}
	for _, cs := range pod.Status.ContainerStatuses {
		reinicios[cs.Name] = cs.RestartCount
	}
	var opcoes []corev1.PodLogOptions
	for _, c := range pod.Spec.Containers {
		opcoes = append(opcoes, corev1.PodLogOptions{Container: c.Name, TailLines: &tail})
		if reinicios[c.Name] > 0 {
			opcoes = append(opcoes, corev1.PodLogOptions{Container: c.Name, TailLines: &tail, Previous: true})
		}
	}
	return opcoes
}

// lerLogsDoPod lê o fim dos logs de cada contêiner e junta tudo, com um cabeçalho por
// contêiner quando há mais de um pedido. Devolve o texto, a quantidade de linhas de log
// (sem os cabeçalhos) e um erro só se nenhum pedido deu certo; o log da execução
// anterior falha à toa quando o kubelet já o descartou, então a falha dele é ignorada.
func lerLogsDoPod(ctx context.Context, c kubernetes.Interface, pod *corev1.Pod) (string, int, error) {
	opcoes := opcoesDeLogs(pod, 50)
	var sb strings.Builder
	var ultimoErro error
	linhas, lidos := 0, 0
	for _, o := range opcoes {
		raw, err := c.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &o).DoRaw(ctx)
		if err != nil {
			if !o.Previous {
				ultimoErro = err
			}
			continue
		}
		lidos++
		texto := string(raw)
		if len(opcoes) > 1 {
			titulo := "contêiner " + o.Container
			if o.Previous {
				titulo += " (execução anterior)"
			}
			fmt.Fprintf(&sb, "== %s ==\n", titulo)
		}
		sb.WriteString(texto)
		if texto != "" && !strings.HasSuffix(texto, "\n") {
			sb.WriteString("\n")
		}
		linhas += contarLinhas(texto)
	}
	if lidos == 0 && ultimoErro != nil {
		return "", 0, ultimoErro
	}
	return sb.String(), linhas, nil
}

// eventoResumido é o que de cada evento do Kubernetes vai para a IA: sem managedFields,
// metadados e referências, que só gastam contexto.
type eventoResumido struct {
	Tipo             string `json:"type"`
	Motivo           string `json:"reason"`
	Mensagem         string `json:"message"`
	Contagem         int32  `json:"count"`
	UltimaOcorrencia string `json:"lastTimestamp,omitempty"`
}

// resumirEventos reduz os eventos a tipo, motivo, mensagem, contagem e último instante.
func resumirEventos(items []corev1.Event) []eventoResumido {
	out := make([]eventoResumido, 0, len(items))
	for _, e := range items {
		quando := e.LastTimestamp.Time
		if quando.IsZero() {
			quando = e.EventTime.Time
		}
		r := eventoResumido{Tipo: e.Type, Motivo: e.Reason, Mensagem: e.Message, Contagem: e.Count}
		if !quando.IsZero() {
			r.UltimaOcorrencia = quando.UTC().Format(time.RFC3339)
		}
		out = append(out, r)
	}
	return out
}

// identidadeDoPod reúne o que distingue um pod de outro no cache de análises.
func identidadeDoPod(pod *corev1.Pod) idPod {
	id := idPod{Namespace: pod.Namespace, Pod: pod.Name, UID: string(pod.UID)}
	if fc := sdk.GetFullContext(); fc != nil {
		id.Contexto = fc.Infra.KubeConfig + "|" + fc.Infra.KubeContext
	}
	for _, cs := range pod.Status.ContainerStatuses {
		id.Reinicios += cs.RestartCount
	}
	return id
}
