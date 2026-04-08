package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
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
func EnrichContext(ctx context.Context) *ClusterContext {
	cc := &ClusterContext{}

	// Obter contexto atual do kubectl
	cluster, err := runKubectl(ctx, "config", "current-context")
	if err != nil {
		slog.Debug("kubectl indisponível para context awareness", "erro", err)
		return nil
	}
	cc.Cluster = strings.TrimSpace(cluster)

	// Obter namespace atual
	ns, err := runKubectl(ctx, "config", "view", "--minify", "--output=jsonpath={..namespace}")
	if err == nil && strings.TrimSpace(ns) != "" {
		cc.Namespace = strings.TrimSpace(ns)
	} else {
		cc.Namespace = "default"
	}

	// Listar pods no namespace atual
	cc.Pods = fetchPods(ctx, cc.Namespace)

	// Obter eventos recentes
	cc.RecentEvents = fetchRecentEvents(ctx, cc.Namespace)

	return cc
}

// fetchPods obtém a lista de pods do namespace especificado.
func fetchPods(ctx context.Context, namespace string) []PodInfo {
	output, err := runKubectl(ctx, "get", "pods", "-n", namespace, "-o", "json")
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
func fetchRecentEvents(ctx context.Context, namespace string) []string {
	output, err := runKubectl(ctx, "get", "events", "-n", namespace,
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

// runKubectl executa um comando kubectl e retorna o output.
func runKubectl(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// FormatClusterContext formata o contexto do cluster para injeção no system prompt.
func FormatClusterContext(cc *ClusterContext) string {
	if cc == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Estado Atual do Cluster\n\n")
	sb.WriteString(fmt.Sprintf("**Cluster**: %s\n", cc.Cluster))
	sb.WriteString(fmt.Sprintf("**Namespace**: %s\n\n", cc.Namespace))

	if len(cc.Pods) > 0 {
		sb.WriteString("**Pods**:\n")
		for _, pod := range cc.Pods {
			sb.WriteString(fmt.Sprintf("- %s (Status: %s, Ready: %s)\n", pod.Name, pod.Status, pod.Ready))
		}
		sb.WriteString("\n")
	}

	if len(cc.RecentEvents) > 0 {
		sb.WriteString("**Eventos Recentes**:\n")
		for _, event := range cc.RecentEvents {
			sb.WriteString(fmt.Sprintf("- %s\n", event))
		}
	}

	return sb.String()
}
