package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/plugins/bard/tui"
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
}

// EnrichContext coleta informações do cluster Kubernetes atual.
// Retorna nil graciosamente se kubectl não estiver disponível ou o cluster estiver offline.
// contexto e namespace, quando dados (yby bard --context, --namespace), trocam o contexto
// atual do kubeconfig e o namespace dele.
func EnrichContext(ctx context.Context, contexto, namespace string) *ClusterContext {
	// Cluster fora do ar não pode segurar a abertura: as chamadas são em sequência, então
	// o prazo vale para o conjunto, além do --request-timeout de cada uma.
	ctx, cancelar := context.WithTimeout(ctx, prazoEnriquecimento)
	defer cancelar()
	return enriquecer(func(args ...string) (string, error) { return runKubectl(ctx, args...) }, contexto, namespace)
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
			return nil
		}
		cc.Cluster = contexto
	} else {
		cluster, err := rodar("config", "current-context")
		if err != nil {
			slog.Debug("kubectl indisponível para context awareness", "erro", err)
			return nil
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

	// Listar pods no namespace atual
	cc.Pods = fetchPods(kubectl, cc.Namespace)

	// Obter eventos recentes
	cc.RecentEvents = fetchRecentEvents(kubectl, cc.Namespace)

	return cc
}

// rodarKubectl roda o kubectl no contexto da sessão.
type rodarKubectl func(args ...string) (string, error)

// fetchPods obtém a lista de pods do namespace especificado.
func fetchPods(kubectl rodarKubectl, namespace string) []PodInfo {
	output, err := kubectl("get", "pods", "-n", namespace, "-o", "json")
	if err != nil {
		return nil
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
		return nil
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

	return pods
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
	sb.WriteString(fmt.Sprintf("**Cluster**: %s\n", cc.Cluster))
	sb.WriteString(fmt.Sprintf("**Namespace**: %s\n\n", cc.Namespace))

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
