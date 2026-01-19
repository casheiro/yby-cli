package monitor

import "fmt"

type Pod struct {
	Name   string
	Status string
	CPU    string
}

type Client interface {
	GetPods() ([]Pod, error)
}

// MockClient simula um cliente Kubernetes
type MockClient struct{}

func (c *MockClient) GetPods() ([]Pod, error) {
	// MVP: Dados estáticos mockados
	return []Pod{
		{Name: "api-gateway-v1", Status: "Executando", CPU: "120m"},
		{Name: "auth-service", Status: "Executando", CPU: "45m"},
		{Name: "payment-worker", Status: "CrashLoopBackOff", CPU: "0m"},
		{Name: "database-primary", Status: "Executando", CPU: "800m"},
	}, nil
}

func NewMockClient() *MockClient {
	fmt.Println("🔌 Conectando ao Cluster K8s (Mock)...")
	return &MockClient{}
}
