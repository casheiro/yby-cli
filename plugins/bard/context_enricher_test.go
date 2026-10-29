package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/plugins/bard/tui"
)

// TestFormatClusterContext_Nil verifica que nil retorna string vazia.
func TestFormatClusterContext_Nil(t *testing.T) {
	result := FormatClusterContext(nil)
	if result != "" {
		t.Errorf("esperava string vazia para nil, obteve: %q", result)
	}
}

// TestFormatClusterContext_ComDados verifica formatação com dados completos.
func TestFormatClusterContext_ComDados(t *testing.T) {
	cc := &ClusterContext{
		Cluster:   "k3d-local",
		Namespace: "default",
		Pods: []PodInfo{
			{Name: "nginx-abc123", Status: "Running", Ready: "1/1"},
			{Name: "redis-xyz789", Status: "Running", Ready: "1/1"},
		},
		RecentEvents: []string{
			"Normal   Scheduled   Pod nginx-abc123 scheduled",
			"Normal   Pulled      Container image pulled",
		},
	}

	result := FormatClusterContext(cc)

	if !strings.Contains(result, "Estado Atual do Cluster") {
		t.Error("resultado deveria conter cabeçalho")
	}
	if !strings.Contains(result, "k3d-local") {
		t.Error("resultado deveria conter nome do cluster")
	}
	if !strings.Contains(result, "default") {
		t.Error("resultado deveria conter namespace")
	}
	if !strings.Contains(result, "nginx-abc123") {
		t.Error("resultado deveria conter nome do pod")
	}
	if !strings.Contains(result, "Running") {
		t.Error("resultado deveria conter status do pod")
	}
	if !strings.Contains(result, "Eventos Recentes") {
		t.Error("resultado deveria conter seção de eventos")
	}
}

// TestFormatClusterContext_SemPods verifica formatação sem pods.
func TestFormatClusterContext_SemPods(t *testing.T) {
	cc := &ClusterContext{
		Cluster:   "test-cluster",
		Namespace: "staging",
	}

	result := FormatClusterContext(cc)

	if !strings.Contains(result, "test-cluster") {
		t.Error("resultado deveria conter nome do cluster")
	}
	if strings.Contains(result, "Pods") {
		t.Error("resultado não deveria conter seção de pods quando vazio")
	}
}

// Com 1000 pods, o estado do cluster fica no teto: os pods com problema primeiro, até
// maxPodsContexto, e uma linha com quantos ficaram de fora.
func TestFormatClusterContext_MilPodsNoTeto(t *testing.T) {
	cc := &ClusterContext{Cluster: "prod", Namespace: "payments"}
	for i := 0; i < 1000; i++ {
		pod := PodInfo{Name: fmt.Sprintf("app-%04d-5d8b6-q7tvn", i), Status: "Running", Ready: "1/1"}
		if i == 990 {
			pod = PodInfo{Name: "worker-quebrado", Status: "Running", Ready: "0/1"}
		}
		if i == 995 {
			pod = PodInfo{Name: "job-pendente", Status: "Pending", Ready: "0/1"}
		}
		cc.Pods = append(cc.Pods, pod)
	}
	for i := 0; i < 5; i++ {
		cc.RecentEvents = append(cc.RecentEvents, "Warning BackOff "+strings.Repeat("mensagem longa ", 100))
	}

	result := FormatClusterContext(cc)

	if len(result) > tui.TetoCluster+64 {
		t.Errorf("o estado do cluster deveria ficar no teto: %d bytes", len(result))
	}
	if strings.Count(result, "app-") > maxPodsContexto {
		t.Errorf("no máximo %d pods listados", maxPodsContexto)
	}
	for _, trecho := range []string{"worker-quebrado", "job-pendente", "e mais", "1000 pods"} {
		if !strings.Contains(result, trecho) {
			t.Errorf("o estado do cluster deveria ter %q:\n%s", trecho, result)
		}
	}
}

// TestBuildSystemPrompt_ComClusterContext verifica injeção do cluster context.
func TestBuildSystemPrompt_ComClusterContext(t *testing.T) {
	cc := &ClusterContext{
		Cluster:   "k3d-test",
		Namespace: "default",
	}

	prompt := buildSystemPrompt(nil, BardConfig{}, nil, cc)

	if !strings.Contains(prompt, "k3d-test") {
		t.Error("prompt deveria conter informação do cluster")
	}
}

// TestBuildSystemPrompt_SemClusterContext verifica prompt sem cluster context.
func TestBuildSystemPrompt_SemClusterContext(t *testing.T) {
	prompt := buildSystemPrompt(nil, BardConfig{}, nil)

	if strings.Contains(prompt, "Estado Atual do Cluster") {
		t.Error("prompt não deveria conter contexto de cluster quando nil")
	}
}
