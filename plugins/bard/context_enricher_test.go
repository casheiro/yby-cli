package main

import (
	"strings"
	"testing"
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

// TestBuildSystemPrompt_ComClusterContext verifica injeção do cluster context.
func TestBuildSystemPrompt_ComClusterContext(t *testing.T) {
	cc := &ClusterContext{
		Cluster:   "k3d-test",
		Namespace: "default",
	}

	prompt := buildSystemPrompt(nil, BardConfig{MaxTokens: 32000}, nil, cc)

	if !strings.Contains(prompt, "k3d-test") {
		t.Error("prompt deveria conter informação do cluster")
	}
}

// TestBuildSystemPrompt_SemClusterContext verifica prompt sem cluster context.
func TestBuildSystemPrompt_SemClusterContext(t *testing.T) {
	prompt := buildSystemPrompt(nil, BardConfig{MaxTokens: 32000}, nil)

	if strings.Contains(prompt, "Estado Atual do Cluster") {
		t.Error("prompt não deveria conter contexto de cluster quando nil")
	}
}
