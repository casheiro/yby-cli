package monitor

import (
	"fmt"
	"testing"
)

// FakeClient implementa a interface Client para testes,
// sem necessidade de conexão real com o cluster K8s.
type FakeClient struct {
	pods        []Pod
	deployments []Deployment
	services    []Service
	nodes       []Node
	err         error
}

func (f *FakeClient) GetPods(_ ListFilter) ([]Pod, error)               { return f.pods, f.err }
func (f *FakeClient) GetDeployments(_ ListFilter) ([]Deployment, error) { return f.deployments, f.err }
func (f *FakeClient) GetServices(_ ListFilter) ([]Service, error)       { return f.services, f.err }
func (f *FakeClient) GetNodes(_ ListFilter) ([]Node, error)             { return f.nodes, f.err }

// TestClientInterface verifica que FakeClient satisfaz a interface Client.
func TestClientInterface(t *testing.T) {
	var _ Client = &FakeClient{}
	var _ Client = &K8sClient{}
}

// --- Testes de Pods ---

// TestFakeClient_GetPods_Sucesso verifica que o fake client retorna pods
// corretamente quando não há erro.
func TestFakeClient_GetPods_Sucesso(t *testing.T) {
	fake := &FakeClient{
		pods: []Pod{
			{Name: "nginx-abc123", Namespace: "default", Status: "Running", CPU: "10m", Memory: "64Mi"},
			{Name: "redis-xyz789", Namespace: "cache", Status: "Running", CPU: "5m", Memory: "32Mi"},
		},
	}

	pods, err := fake.GetPods(ListFilter{})
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

	pods, err := fake.GetPods(ListFilter{})
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

	pods, err := fake.GetPods(ListFilter{})
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
		Memory:    "128Mi",
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
	if pod.Memory != "128Mi" {
		t.Errorf("memory esperado '128Mi', obtido '%s'", pod.Memory)
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
				Memory:    "N/A",
			}

			if pod.Status != tt.status {
				t.Errorf("status esperado '%s', obtido '%s'", tt.status, pod.Status)
			}
		})
	}
}

// --- Testes de Deployments ---

// TestFakeClient_GetDeployments_Sucesso verifica que o fake client retorna
// deployments corretamente.
func TestFakeClient_GetDeployments_Sucesso(t *testing.T) {
	fake := &FakeClient{
		deployments: []Deployment{
			{Name: "nginx-deploy", Namespace: "default", Replicas: 3, Ready: 3, Available: 3},
			{Name: "api-deploy", Namespace: "backend", Replicas: 2, Ready: 1, Available: 1},
		},
	}

	deps, err := fake.GetDeployments(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(deps) != 2 {
		t.Fatalf("esperado 2 deployments, obtido %d", len(deps))
	}

	if deps[0].Name != "nginx-deploy" {
		t.Errorf("nome do deployment[0] esperado 'nginx-deploy', obtido '%s'", deps[0].Name)
	}
	if deps[0].Replicas != 3 {
		t.Errorf("replicas esperadas 3, obtido %d", deps[0].Replicas)
	}
	if deps[1].Ready != 1 {
		t.Errorf("ready esperado 1, obtido %d", deps[1].Ready)
	}
}

// TestFakeClient_GetDeployments_Erro verifica propagação de erro em deployments.
func TestFakeClient_GetDeployments_Erro(t *testing.T) {
	fake := &FakeClient{
		err: fmt.Errorf("acesso negado"),
	}

	deps, err := fake.GetDeployments(ListFilter{})
	if err == nil {
		t.Fatal("esperava erro, mas obteve nil")
	}
	if deps != nil {
		t.Errorf("esperava deployments nil quando há erro, obtido %v", deps)
	}
}

// TestFakeClient_GetDeployments_Vazio verifica lista vazia de deployments.
func TestFakeClient_GetDeployments_Vazio(t *testing.T) {
	fake := &FakeClient{
		deployments: []Deployment{},
	}

	deps, err := fake.GetDeployments(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("esperado 0 deployments, obtido %d", len(deps))
	}
}

// TestDeploymentStruct verifica a estrutura Deployment e seus campos.
func TestDeploymentStruct(t *testing.T) {
	dep := Deployment{
		Name:      "meu-deploy",
		Namespace: "producao",
		Replicas:  3,
		Ready:     2,
		Available: 2,
	}

	if dep.Name != "meu-deploy" {
		t.Errorf("nome esperado 'meu-deploy', obtido '%s'", dep.Name)
	}
	if dep.Replicas != 3 {
		t.Errorf("replicas esperadas 3, obtido %d", dep.Replicas)
	}
	if dep.Ready != 2 {
		t.Errorf("ready esperado 2, obtido %d", dep.Ready)
	}
	if dep.Available != 2 {
		t.Errorf("available esperado 2, obtido %d", dep.Available)
	}
}

// --- Testes de Services ---

// TestFakeClient_GetServices_Sucesso verifica que o fake client retorna
// services corretamente.
func TestFakeClient_GetServices_Sucesso(t *testing.T) {
	fake := &FakeClient{
		services: []Service{
			{Name: "nginx-svc", Namespace: "default", Type: "ClusterIP", ClusterIP: "10.96.0.1", Ports: "80/TCP"},
			{Name: "api-svc", Namespace: "backend", Type: "LoadBalancer", ClusterIP: "10.96.0.2", Ports: "443/TCP, 80/TCP"},
		},
	}

	svcs, err := fake.GetServices(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(svcs) != 2 {
		t.Fatalf("esperado 2 services, obtido %d", len(svcs))
	}

	if svcs[0].Name != "nginx-svc" {
		t.Errorf("nome do service[0] esperado 'nginx-svc', obtido '%s'", svcs[0].Name)
	}
	if svcs[0].Type != "ClusterIP" {
		t.Errorf("tipo esperado 'ClusterIP', obtido '%s'", svcs[0].Type)
	}
	if svcs[1].Ports != "443/TCP, 80/TCP" {
		t.Errorf("portas esperadas '443/TCP, 80/TCP', obtido '%s'", svcs[1].Ports)
	}
}

// TestFakeClient_GetServices_Erro verifica propagação de erro em services.
func TestFakeClient_GetServices_Erro(t *testing.T) {
	fake := &FakeClient{
		err: fmt.Errorf("timeout"),
	}

	svcs, err := fake.GetServices(ListFilter{})
	if err == nil {
		t.Fatal("esperava erro, mas obteve nil")
	}
	if svcs != nil {
		t.Errorf("esperava services nil quando há erro, obtido %v", svcs)
	}
}

// TestFakeClient_GetServices_Vazio verifica lista vazia de services.
func TestFakeClient_GetServices_Vazio(t *testing.T) {
	fake := &FakeClient{
		services: []Service{},
	}

	svcs, err := fake.GetServices(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(svcs) != 0 {
		t.Errorf("esperado 0 services, obtido %d", len(svcs))
	}
}

// TestServiceStruct verifica a estrutura Service e seus campos.
func TestServiceStruct(t *testing.T) {
	svc := Service{
		Name:      "meu-svc",
		Namespace: "producao",
		Type:      "NodePort",
		ClusterIP: "10.96.0.5",
		Ports:     "8080/TCP",
	}

	if svc.Name != "meu-svc" {
		t.Errorf("nome esperado 'meu-svc', obtido '%s'", svc.Name)
	}
	if svc.Type != "NodePort" {
		t.Errorf("tipo esperado 'NodePort', obtido '%s'", svc.Type)
	}
	if svc.ClusterIP != "10.96.0.5" {
		t.Errorf("clusterIP esperado '10.96.0.5', obtido '%s'", svc.ClusterIP)
	}
}

// --- Testes de Nodes ---

// TestFakeClient_GetNodes_Sucesso verifica que o fake client retorna
// nodes corretamente.
func TestFakeClient_GetNodes_Sucesso(t *testing.T) {
	fake := &FakeClient{
		nodes: []Node{
			{Name: "node-1", Status: "Ready", CPUCapacity: "4", MemoryCapacity: "8Gi", Version: "v1.29.0"},
			{Name: "node-2", Status: "NotReady", CPUCapacity: "2", MemoryCapacity: "4Gi", Version: "v1.29.0"},
		},
	}

	nodes, err := fake.GetNodes(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("esperado 2 nodes, obtido %d", len(nodes))
	}

	if nodes[0].Name != "node-1" {
		t.Errorf("nome do node[0] esperado 'node-1', obtido '%s'", nodes[0].Name)
	}
	if nodes[0].Status != "Ready" {
		t.Errorf("status esperado 'Ready', obtido '%s'", nodes[0].Status)
	}
	if nodes[1].Status != "NotReady" {
		t.Errorf("status esperado 'NotReady', obtido '%s'", nodes[1].Status)
	}
}

// TestFakeClient_GetNodes_Erro verifica propagação de erro em nodes.
func TestFakeClient_GetNodes_Erro(t *testing.T) {
	fake := &FakeClient{
		err: fmt.Errorf("cluster inacessível"),
	}

	nodes, err := fake.GetNodes(ListFilter{})
	if err == nil {
		t.Fatal("esperava erro, mas obteve nil")
	}
	if nodes != nil {
		t.Errorf("esperava nodes nil quando há erro, obtido %v", nodes)
	}
}

// TestFakeClient_GetNodes_Vazio verifica lista vazia de nodes.
func TestFakeClient_GetNodes_Vazio(t *testing.T) {
	fake := &FakeClient{
		nodes: []Node{},
	}

	nodes, err := fake.GetNodes(ListFilter{})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("esperado 0 nodes, obtido %d", len(nodes))
	}
}

// TestNodeStruct verifica a estrutura Node e seus campos.
func TestNodeStruct(t *testing.T) {
	node := Node{
		Name:           "meu-node",
		Status:         "Ready",
		CPUCapacity:    "8",
		MemoryCapacity: "16Gi",
		Version:        "v1.29.0",
	}

	if node.Name != "meu-node" {
		t.Errorf("nome esperado 'meu-node', obtido '%s'", node.Name)
	}
	if node.Status != "Ready" {
		t.Errorf("status esperado 'Ready', obtido '%s'", node.Status)
	}
	if node.CPUCapacity != "8" {
		t.Errorf("cpu esperado '8', obtido '%s'", node.CPUCapacity)
	}
	if node.MemoryCapacity != "16Gi" {
		t.Errorf("memória esperada '16Gi', obtida '%s'", node.MemoryCapacity)
	}
	if node.Version != "v1.29.0" {
		t.Errorf("versão esperada 'v1.29.0', obtida '%s'", node.Version)
	}
}
