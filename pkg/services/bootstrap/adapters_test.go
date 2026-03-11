package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// WaitPodReady
// ========================================================

func TestRealK8sClient_WaitPodReady_Success(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.WaitPodReady(context.Background(), "app=test", "default", 60)
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "kubectl")
	assert.Contains(t, capturedArgs, "--for=condition=Ready")
	assert.Contains(t, capturedArgs, "-l")
	assert.Contains(t, capturedArgs, "app=test")
	assert.Contains(t, capturedArgs, "-n")
	assert.Contains(t, capturedArgs, "default")
	assert.Contains(t, capturedArgs, "--timeout=60s")
}

func TestRealK8sClient_WaitPodReady_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("timeout esperando pod")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.WaitPodReady(context.Background(), "app=test", "kube-system", 30)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout esperando pod")
}

// ========================================================
// WaitCRD
// ========================================================

func TestRealK8sClient_WaitCRD_Success(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.WaitCRD(context.Background(), "applications.argoproj.io", 120)
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "kubectl")
	assert.Contains(t, capturedArgs, "--for")
	assert.Contains(t, capturedArgs, "condition=established")
	assert.Contains(t, capturedArgs, "--timeout=120s")
	assert.Contains(t, capturedArgs, "crd/applications.argoproj.io")
}

func TestRealK8sClient_WaitCRD_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("CRD não encontrado")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.WaitCRD(context.Background(), "missing.crd", 30)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CRD não encontrado")
}

// ========================================================
// NamespaceExists
// ========================================================

func TestRealK8sClient_NamespaceExists_True(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil // namespace existe
		},
	}
	k := &RealK8sClient{Runner: runner}

	exists, err := k.NamespaceExists(context.Background(), "argocd")
	assert.NoError(t, err)
	assert.True(t, exists, "deveria retornar true quando kubectl get namespace sucede")
	assert.Contains(t, capturedArgs, "get")
	assert.Contains(t, capturedArgs, "namespace")
	assert.Contains(t, capturedArgs, "argocd")
}

func TestRealK8sClient_NamespaceExists_False(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("namespace não encontrado")
		},
	}
	k := &RealK8sClient{Runner: runner}

	exists, err := k.NamespaceExists(context.Background(), "inexistente")
	assert.NoError(t, err, "NamespaceExists não deveria propagar o erro do kubectl")
	assert.False(t, exists, "deveria retornar false quando kubectl get namespace falha")
}

// ========================================================
// CreateNamespace
// ========================================================

func TestRealK8sClient_CreateNamespace_Success(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			capturedArgs = append([]string{name}, args...)
			return []byte("namespace/test-ns created"), nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.CreateNamespace(context.Background(), "test-ns")
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "create")
	assert.Contains(t, capturedArgs, "namespace")
	assert.Contains(t, capturedArgs, "test-ns")
}

func TestRealK8sClient_CreateNamespace_AlreadyExists(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(`Error from server (AlreadyExists): namespaces "argocd" already exists`), fmt.Errorf("exit status 1")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.CreateNamespace(context.Background(), "argocd")
	assert.NoError(t, err, "deveria ignorar erro quando namespace já existe")
}

func TestRealK8sClient_CreateNamespace_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Error from server (Forbidden): forbidden"), fmt.Errorf("exit status 1")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.CreateNamespace(context.Background(), "proibido-ns")
	assert.Error(t, err, "deveria retornar erro quando output não contém 'already exists'")
}

// ========================================================
// ApplyManifest
// ========================================================

func TestRealK8sClient_ApplyManifest_WithNamespace(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.ApplyManifest(context.Background(), "/tmp/manifest.yaml", "argocd")
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "apply")
	assert.Contains(t, capturedArgs, "-f")
	assert.Contains(t, capturedArgs, "/tmp/manifest.yaml")
	assert.Contains(t, capturedArgs, "-n")
	assert.Contains(t, capturedArgs, "argocd")
}

func TestRealK8sClient_ApplyManifest_WithoutNamespace(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.ApplyManifest(context.Background(), "/tmp/manifest.yaml", "")
	assert.NoError(t, err)

	// Verifica que NÃO contém -n quando namespace é vazio
	argsStr := strings.Join(capturedArgs, " ")
	assert.NotContains(t, argsStr, " -n ",
		"não deveria incluir -n quando namespace está vazio")
	assert.Contains(t, capturedArgs, "apply")
	assert.Contains(t, capturedArgs, "-f")
	assert.Contains(t, capturedArgs, "/tmp/manifest.yaml")
}

func TestRealK8sClient_ApplyManifest_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("falha ao aplicar manifesto")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.ApplyManifest(context.Background(), "/tmp/bad.yaml", "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao aplicar manifesto")
}

// ========================================================
// PatchApplication
// ========================================================

func TestRealK8sClient_PatchApplication_Success(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	k := &RealK8sClient{Runner: runner}

	patch := `{"spec":{"source":{"targetRevision":"main"}}}`
	err := k.PatchApplication(context.Background(), "meu-app", "argocd", patch)
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "kubectl")
	assert.Contains(t, capturedArgs, "patch")
	assert.Contains(t, capturedArgs, "application")
	assert.Contains(t, capturedArgs, "meu-app")
	assert.Contains(t, capturedArgs, "-n")
	assert.Contains(t, capturedArgs, "argocd")
	assert.Contains(t, capturedArgs, "--type")
	assert.Contains(t, capturedArgs, "merge")
	assert.Contains(t, capturedArgs, "-p")
	assert.Contains(t, capturedArgs, patch)
}

func TestRealK8sClient_PatchApplication_Error(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("falha ao aplicar patch")
		},
	}
	k := &RealK8sClient{Runner: runner}

	err := k.PatchApplication(context.Background(), "app", "ns", `{"spec":{}}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao aplicar patch")
}
