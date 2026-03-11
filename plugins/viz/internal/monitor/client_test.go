package monitor

import (
	"fmt"
	"testing"
)

// FakeClient implementa a interface Client para testes,
// sem necessidade de conexão real com o cluster K8s.
type FakeClient struct {
	pods []Pod
	err  error
}

func (f *FakeClient) GetPods() ([]Pod, error) {
	return f.pods, f.err
}

// TestClientInterface verifica que FakeClient satisfaz a interface Client.
func TestClientInterface(t *testing.T) {
	var _ Client = &FakeClient{}
	var _ Client = &K8sClient{}
}

// TestFakeClient_GetPods_Sucesso verifica que o fake client retorna pods
// corretamente quando não há erro.
func TestFakeClient_GetPods_Sucesso(t *testing.T) {
	fake := &FakeClient{
		pods: []Pod{
			{Name: "nginx-abc123", Namespace: "default", Status: "Running", CPU: "10m"},
			{Name: "redis-xyz789", Namespace: "cache", Status: "Running", CPU: "5m"},
		},
	}

	pods, err := fake.GetPods()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(pods) != 2 {
		t.Fatalf("esperado 2 pods, obtido %d", len(pods))
	}

	if pods[0].Name != "nginx-abc123" {
		t.Errorf("nome do pod[0] esperado 'nginx-abc123', obtido '%s'", pods[0].Name)
	}
	if pods[0].Namespace != "default" {
		t.Errorf("namespace do pod[0] esperado 'default', obtido '%s'", pods[0].Namespace)
	}
	if pods[1].Namespace != "cache" {
		t.Errorf("namespace do pod[1] esperado 'cache', obtido '%s'", pods[1].Namespace)
	}
}

// TestFakeClient_GetPods_Erro verifica que o fake client propaga erros
// corretamente.
func TestFakeClient_GetPods_Erro(t *testing.T) {
	fake := &FakeClient{
		err: fmt.Errorf("conexão recusada"),
	}

	pods, err := fake.GetPods()
	if err == nil {
		t.Fatal("esperava erro, mas obteve nil")
	}

	if pods != nil {
		t.Errorf("esperava pods nil quando há erro, obtido %v", pods)
	}
}

// TestFakeClient_GetPods_Vazio verifica que retorna lista vazia quando
// não há pods no cluster.
func TestFakeClient_GetPods_Vazio(t *testing.T) {
	fake := &FakeClient{
		pods: []Pod{},
	}

	pods, err := fake.GetPods()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(pods) != 0 {
		t.Errorf("esperado 0 pods, obtido %d", len(pods))
	}
}

// TestPodStruct verifica a estrutura Pod e seus campos.
func TestPodStruct(t *testing.T) {
	pod := Pod{
		Name:      "meu-app-abc123",
		Namespace: "producao",
		Status:    "Running",
		CPU:       "100m",
	}

	if pod.Name != "meu-app-abc123" {
		t.Errorf("nome esperado 'meu-app-abc123', obtido '%s'", pod.Name)
	}
	if pod.Namespace != "producao" {
		t.Errorf("namespace esperado 'producao', obtido '%s'", pod.Namespace)
	}
	if pod.Status != "Running" {
		t.Errorf("status esperado 'Running', obtido '%s'", pod.Status)
	}
	if pod.CPU != "100m" {
		t.Errorf("cpu esperado '100m', obtido '%s'", pod.CPU)
	}
}

// TestPodStatus verifica os diferentes estados possíveis de um pod.
func TestPodStatus(t *testing.T) {
	statuses := []struct {
		name   string
		status string
	}{
		{name: "pod em execução", status: "Running"},
		{name: "pod pendente", status: "Pending"},
		{name: "pod com sucesso", status: "Succeeded"},
		{name: "pod com falha", status: "Failed"},
		{name: "pod desconhecido", status: "Unknown"},
	}

	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			pod := Pod{
				Name:      "test-pod",
				Namespace: "default",
				Status:    tt.status,
				CPU:       "N/A",
			}

			if pod.Status != tt.status {
				t.Errorf("status esperado '%s', obtido '%s'", tt.status, pod.Status)
			}
		})
	}
}
