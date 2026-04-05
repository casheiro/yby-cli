package status

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockInspector implementa ClusterInspector para testes.
type mockInspector struct {
	getNodesFunc         func(ctx context.Context) (string, error)
	getArgoCDPodsFunc    func(ctx context.Context) (string, error)
	getIngressesFunc     func(ctx context.Context) (string, error)
	getScaledObjectsFunc func(ctx context.Context) (string, error)
	getKeplerPodsFunc    func(ctx context.Context) (string, error)
}

func (m *mockInspector) GetNodes(ctx context.Context) (string, error) {
	if m.getNodesFunc != nil {
		return m.getNodesFunc(ctx)
	}
	return "", nil
}

func (m *mockInspector) GetArgoCDPods(ctx context.Context) (string, error) {
	if m.getArgoCDPodsFunc != nil {
		return m.getArgoCDPodsFunc(ctx)
	}
	return "", nil
}

func (m *mockInspector) GetIngresses(ctx context.Context) (string, error) {
	if m.getIngressesFunc != nil {
		return m.getIngressesFunc(ctx)
	}
	return "", nil
}

func (m *mockInspector) GetScaledObjects(ctx context.Context) (string, error) {
	if m.getScaledObjectsFunc != nil {
		return m.getScaledObjectsFunc(ctx)
	}
	return "", nil
}

func (m *mockInspector) GetKeplerPods(ctx context.Context) (string, error) {
	if m.getKeplerPodsFunc != nil {
		return m.getKeplerPodsFunc(ctx)
	}
	return "", nil
}

func TestCheck_SucessoTotal(t *testing.T) {
	inspector := &mockInspector{
		getNodesFunc: func(_ context.Context) (string, error) {
			return "NAME       STATUS   ROLES\nnode1      Ready    control-plane", nil
		},
		getArgoCDPodsFunc: func(_ context.Context) (string, error) {
			return "NAME                          READY   STATUS\nargocd-server-abc123   1/1     Running", nil
		},
		getIngressesFunc: func(_ context.Context) (string, error) {
			return "NAMESPACE   NAME      CLASS   HOSTS\ndefault     my-ing    nginx   example.com", nil
		},
		getScaledObjectsFunc: func(_ context.Context) (string, error) {
			return "NAMESPACE   NAME        SCALETARGETKIND\ndefault     my-scaler   Deployment", nil
		},
		getKeplerPodsFunc: func(_ context.Context) (string, error) {
			return "NAME           READY   STATUS    RESTARTS\nkepler-abc     1/1     Running   0", nil
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.True(t, report.Nodes.Available, "nodes devem estar disponíveis")
	assert.Contains(t, report.Nodes.Output, "node1")

	assert.True(t, report.ArgoCD.Available, "ArgoCD deve estar disponível")
	assert.Contains(t, report.ArgoCD.Output, "argocd-server")

	assert.True(t, report.Ingress.Available, "ingresses devem estar disponíveis")
	assert.Contains(t, report.Ingress.Output, "my-ing")

	assert.True(t, report.KEDA.Available, "KEDA deve estar disponível")
	assert.Contains(t, report.KEDA.Output, "my-scaler")

	assert.True(t, report.Kepler.Available, "Kepler deve estar disponível")
	assert.Contains(t, report.Kepler.Message, "ATIVO")
}

func TestCheck_NodesFalhando(t *testing.T) {
	inspector := &mockInspector{
		getNodesFunc: func(_ context.Context) (string, error) {
			return "", fmt.Errorf("connection refused")
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.False(t, report.Nodes.Available, "nodes não devem estar disponíveis quando kubectl falha")
	assert.Contains(t, report.Nodes.Message, "Erro ao obter nodes")
}

func TestCheck_KeplerRunning(t *testing.T) {
	inspector := &mockInspector{
		getKeplerPodsFunc: func(_ context.Context) (string, error) {
			return "NAME           READY   STATUS    RESTARTS\nkepler-abc     1/1     Running   0", nil
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.True(t, report.Kepler.Available)
	assert.Contains(t, report.Kepler.Message, "ATIVO")
}

func TestCheck_KeplerNaoRunning(t *testing.T) {
	inspector := &mockInspector{
		getKeplerPodsFunc: func(_ context.Context) (string, error) {
			return "NAME           READY   STATUS         RESTARTS\nkepler-abc     0/1     CrashLoopBackOff   5", nil
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.True(t, report.Kepler.Available)
	assert.Contains(t, report.Kepler.Message, "não está 'Running'")
}

func TestCheck_KeplerNaoEncontrado(t *testing.T) {
	inspector := &mockInspector{
		getKeplerPodsFunc: func(_ context.Context) (string, error) {
			return "", fmt.Errorf("namespace kepler not found")
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.False(t, report.Kepler.Available)
	assert.Contains(t, report.Kepler.Message, "não encontrado")
}

func TestCheck_KEDAAusente(t *testing.T) {
	inspector := &mockInspector{
		getScaledObjectsFunc: func(_ context.Context) (string, error) {
			return "", fmt.Errorf("the server doesn't have a resource type \"scaledobjects\"")
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.False(t, report.KEDA.Available)
	assert.Contains(t, report.KEDA.Message, "KEDA não detectado")
}

func TestCheck_KEDASemRegras(t *testing.T) {
	inspector := &mockInspector{
		getScaledObjectsFunc: func(_ context.Context) (string, error) {
			return "", nil
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.True(t, report.KEDA.Available)
	assert.Contains(t, report.KEDA.Message, "sem regras")
}

func TestCheck_IngressVazio(t *testing.T) {
	inspector := &mockInspector{
		getIngressesFunc: func(_ context.Context) (string, error) {
			return "", nil
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.True(t, report.Ingress.Available)
	assert.Contains(t, report.Ingress.Message, "Nenhum ingress encontrado")
	assert.Empty(t, report.Ingress.Output)
}

func TestCheck_ArgoCDNaoEncontrado(t *testing.T) {
	inspector := &mockInspector{
		getArgoCDPodsFunc: func(_ context.Context) (string, error) {
			return "", fmt.Errorf("namespace argocd not found")
		},
	}

	svc := NewService(inspector)
	report := svc.Check(context.Background())

	assert.False(t, report.ArgoCD.Available)
	assert.Contains(t, report.ArgoCD.Message, "argocd não encontrado")
}
