package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/plugins/bard/tui"
)

// PodInfo contém informações resumidas de um pod.
type PodInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Ready  string `json:"ready"`
}

// ClusterContext contém o contexto atual do cluster Kubernetes.
type ClusterContext struct {
	Namespace    string    `json:"namespace"`
	Cluster      string    `json:"cluster"`
	Pods         []PodInfo `json:"pods"`
	RecentEvents []string  `json:"recent_events"`
	// SemCluster é o motivo de não haver cluster (kubectl ausente, kubeconfig sem contexto
	// atual, contexto que não existe); com ele, o resto fica vazio. O prompt diz o motivo,
	// para a IA não sair procurando o cluster por conta própria.
	SemCluster string `json:"sem_cluster,omitempty"`
	// SemResposta é o erro do kubectl quando o cluster do kubeconfig existe mas não
	// respondeu (fora do ar, credencial vencida); pods e eventos ficam desconhecidos.
	SemResposta string `json:"sem_resposta,omitempty"`
}

// EnrichContext coleta informações do cluster Kubernetes atual. Sem cluster, devolve só o
// motivo em SemCluster. contexto e namespace, quando dados (yby bard --context,
// --namespace), trocam o contexto atual do kubeconfig e o namespace dele.
func EnrichContext(ctx context.Context, contexto, namespace string) *ClusterContext {
	// Cluster fora do ar não pode segurar a abertura: as chamadas são em sequência, então
	// o prazo vale para o conjunto, além do --request-timeout de cada uma.
	ctx, cancelar := context.WithTimeout(ctx, prazoEnriquecimento)
	defer cancelar()
	cc := enriquecer(func(args ...string) (string, error) { return runKubectl(ctx, args...) }, contexto, namespace)
	if cc.SemCluster != "" && ctx.Err() != nil {
		cc.SemCluster = fmt.Sprintf("o kubectl não respondeu em %s", prazoEnriquecimento)
	}
	return cc
}

// motivoSemCluster diz, para a IA e para quem usa, por que o kubectl não deu o cluster.
func motivoSemCluster(err error, contexto string) string {
	if errors.Is(err, exec.ErrNotFound) {
		return "o kubectl não está instalado ou não está no PATH"
	}
	if contexto != "" {
		return fmt.Sprintf("o contexto %q não existe no kubeconfig", contexto)
	}
	texto := err.Error()
	var saida *exec.ExitError
	if errors.As(err, &saida) && len(strings.TrimSpace(string(saida.Stderr))) > 0 {
		texto = strings.TrimSpace(string(saida.Stderr))
	}
	if strings.Contains(texto, "current-context is not set") {
		return "o kubeconfig não tem contexto atual (current-context)"
	}
	// error loading config file "C:\Users\…\.kube\config": couldn't get version/kind; …
	if _, resto, ok := strings.Cut(texto, `error loading config file "`); ok {
		if arquivo, _, ok := strings.Cut(resto, `"`); ok {
			return fmt.Sprintf("o kubeconfig %s não é válido (o kubectl não consegue ler o arquivo)", arquivo)
		}
	}
	linha, _, _ := strings.Cut(texto, "\n")
	return "o kubectl não leu o kubeconfig: " + tui.CortarLinha(linha, maxEventoContexto)
}

// enriquecer é o EnrichContext com o kubectl dado (os testes trocam o kubectl).
func enriquecer(rodar rodarKubectl, contexto, namespace string) *ClusterContext {
	cc := &ClusterContext{}
	kubectl := func(args ...string) (string, error) {
		if contexto != "" {
			args = append([]string{"--context", contexto}, args...)
		}
		return rodar(args...)
	}

	// O cluster é o contexto dado ou, sem ele, o atual do kubeconfig. O config
	// current-context ignora o --context e falha num kubeconfig sem contexto atual (o Viz
	// aberto com --context, como pelo ↵ do yby cloud dashboard): o contexto dado é
	// conferido pelo nome, no get-contexts.
	if contexto != "" {
		if _, err := rodar("config", "get-contexts", contexto, "-o", "name"); err != nil {
			slog.Debug("contexto do kubeconfig indisponível para context awareness", "contexto", contexto, "erro", err)
			return &ClusterContext{SemCluster: motivoSemCluster(err, contexto)}
		}
		cc.Cluster = contexto
	} else {
		cluster, err := rodar("config", "current-context")
		if err != nil {
			slog.Debug("kubectl indisponível para context awareness", "erro", err)
			return &ClusterContext{SemCluster: motivoSemCluster(err, "")}
		}
		cc.Cluster = strings.TrimSpace(cluster)
	}

	// Obter namespace atual
	ns, err := kubectl("config", "view", "--minify", "--output=jsonpath={..namespace}")
	switch {
	case namespace != "":
		cc.Namespace = namespace
	case err == nil && strings.TrimSpace(ns) != "":
		cc.Namespace = strings.TrimSpace(ns)
	default:
		cc.Namespace = "default"
	}

	// Listar pods no namespace atual; sem resposta do cluster, nem tenta os eventos
	pods, err := fetchPods(kubectl, cc.Namespace)
	if err != nil {
		slog.Debug("cluster sem resposta para context awareness", "cluster", cc.Cluster, "erro", err)
		cc.SemResposta = motivoSemResposta(err)
		return cc
	}
	cc.Pods = pods

	// Obter eventos recentes
	cc.RecentEvents = fetchRecentEvents(kubectl, cc.Namespace)

	return cc
}

// motivoSemResposta resume por que o cluster do kubeconfig não respondeu ao kubectl.
func motivoSemResposta(err error) string {
	texto := err.Error()
	var saida *exec.ExitError
	if errors.As(err, &saida) && len(strings.TrimSpace(string(saida.Stderr))) > 0 {
		texto = strings.TrimSpace(string(saida.Stderr))
	}
	linha, _, _ := strings.Cut(texto, "\n")
	return tui.CortarLinha(linha, maxEventoContexto)
}

// rodarKubectl roda o kubectl no contexto da sessão.
type rodarKubectl func(args ...string) (string, error)

// fetchPods obtém a lista de pods do namespace especificado; o erro é o do kubectl (o
// cluster não respondeu), e uma saída que não se lê dá lista vazia.
func fetchPods(kubectl rodarKubectl, namespace string) ([]PodInfo, error) {
	output, err := kubectl("get", "pods", "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}

	var podList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &podList); err != nil {
		slog.Debug("erro ao parsear lista de pods", "erro", err)
		return nil, nil
	}

	var pods []PodInfo
	for _, item := range podList.Items {
		ready := 0
		total := len(item.Status.ContainerStatuses)
		for _, cs := range item.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}
		pods = append(pods, PodInfo{
			Name:   item.Metadata.Name,
			Status: item.Status.Phase,
			Ready:  fmt.Sprintf("%d/%d", ready, total),
		})
	}

	return pods, nil
}

// fetchRecentEvents obtém os 5 eventos mais recentes do namespace.
func fetchRecentEvents(kubectl rodarKubectl, namespace string) []string {
	output, err := kubectl("get", "events", "-n", namespace,
		"--sort-by=.lastTimestamp", "-o", "custom-columns=TYPE:.type,REASON:.reason,MESSAGE:.message",
		"--no-headers")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Pegar os últimos 5 eventos
	start := 0
	if len(lines) > 5 {
		start = len(lines) - 5
	}
	var events []string
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) != "" {
			events = append(events, strings.TrimSpace(line))
		}
	}

	return events
}

// prazoEnriquecimento é o prazo total (e de cada requisição) da leitura do cluster.
const prazoEnriquecimento = 5 * time.Second

// binarioKubectl é o executável chamado; os testes trocam por um falso.
var binarioKubectl = "kubectl"

// comandoKubectl monta o kubectl com o prazo de requisição ao servidor.
func comandoKubectl(ctx context.Context, args ...string) *exec.Cmd {
	args = append([]string{"--request-timeout=" + prazoEnriquecimento.String()}, args...)
	cmd := exec.CommandContext(ctx, binarioKubectl, args...)
	cmd.WaitDelay = time.Second
	return cmd
}

// runKubectl executa um comando kubectl e retorna o output.
func runKubectl(ctx context.Context, args ...string) (string, error) {
	cmd := comandoKubectl(ctx, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// Teto do estado do cluster: quantos pods ele lista e o tamanho de cada evento. Com nomes
// de pod de 63 caracteres, o bloco fica perto do tui.TetoCluster, que é a rede de proteção.
const (
	maxPodsContexto   = 20
	maxEventoContexto = 160
)

// FormatClusterContext formata o contexto do cluster para injeção no system prompt, no
// teto do orçamento: num namespace grande, os pods com problema vêm primeiro e só os
// primeiros maxPodsContexto entram; o resto vira uma linha com a contagem.
func FormatClusterContext(cc *ClusterContext) string {
	if cc == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Estado Atual do Cluster\n\n")
	if cc.SemCluster != "" {
		// Sem isso, a pergunta "qual é o meu cluster?" fazia a IA tentar o kubectl pelas
		// ferramentas do provider e responder que não conseguiu, sem dizer por quê.
		fmt.Fprintf(&sb, "**Nenhum cluster conectado**: %s. O Yby não leu pods nem eventos.\n", cc.SemCluster)
		sb.WriteString("Se a pergunta depende do cluster, comece dizendo isso e o motivo, e mostre como resolver conforme o motivo: " +
			"kubeconfig inválido precisa ser corrigido ou refeito (o kubectl não roda nada com ele); sem contexto atual, " +
			"`kubectl config get-contexts` lista os contextos e `kubectl config use-context <nome>` escolhe um; sem kubectl, " +
			"instale-o; num projeto do yby, `yby up` sobe o cluster local. Não tente descobrir o cluster pelas ferramentas do provider.\n")
		return sb.String()
	}
	sb.WriteString(fmt.Sprintf("**Cluster**: %s\n", cc.Cluster))
	sb.WriteString(fmt.Sprintf("**Namespace**: %s\n\n", cc.Namespace))
	if cc.SemResposta != "" {
		fmt.Fprintf(&sb, "**O cluster não respondeu** ao kubectl: %s.\n", cc.SemResposta)
		sb.WriteString("Pods e eventos são desconhecidos: não diga que o namespace está vazio. Se a pergunta depende " +
			"do cluster, comece dizendo que ele não respondeu, com o motivo, e sugira `yby doctor` e `kubectl cluster-info`.\n")
		return sb.String()
	}

	if len(cc.Pods) > 0 {
		pods := cc.Pods
		if len(pods) > maxPodsContexto {
			pods = append([]PodInfo(nil), pods...)
			sort.SliceStable(pods, func(i, j int) bool { return podComProblema(pods[i]) && !podComProblema(pods[j]) })
			fmt.Fprintf(&sb, "**Pods** (%d pods; os com problema primeiro):\n", len(cc.Pods))
			pods = pods[:maxPodsContexto]
		} else {
			sb.WriteString("**Pods**:\n")
		}
		for _, pod := range pods {
			sb.WriteString(fmt.Sprintf("- %s (Status: %s, Ready: %s)\n", pod.Name, pod.Status, pod.Ready))
		}
		if resto := len(cc.Pods) - len(pods); resto > 0 {
			fmt.Fprintf(&sb, "- … e mais %d pods\n", resto)
		}
		sb.WriteString("\n")
	}

	if len(cc.RecentEvents) > 0 {
		sb.WriteString("**Eventos Recentes**:\n")
		for _, event := range cc.RecentEvents {
			sb.WriteString(fmt.Sprintf("- %s\n", tui.CortarLinha(event, maxEventoContexto)))
		}
	}

	return tui.Cortar(sb.String(), tui.TetoCluster)
}

// podComProblema diz se o pod não está rodando (nem terminou bem) ou tem contêiner que
// não está pronto.
func podComProblema(p PodInfo) bool {
	if p.Status == "Succeeded" {
		return false
	}
	if p.Status != "Running" {
		return true
	}
	prontos, total, ok := strings.Cut(p.Ready, "/")
	return !ok || prontos != total
}
