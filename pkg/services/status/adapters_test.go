package status

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestKubectlInspector_GetNodes(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, []string{"get", "nodes"}, args)
			return []byte("NAME    STATUS   ROLES\nnode1   Ready    control-plane\n"), nil
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetNodes(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, out, "node1")
}

func TestKubectlInspector_GetNodes_Erro(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("connection refused"), fmt.Errorf("exit status 1")
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetNodes(context.Background())

	assert.Error(t, err)
	assert.Contains(t, out, "connection refused")
}

func TestKubectlInspector_GetArgoCDPods(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, []string{"get", "pods", "-n", "argocd"}, args)
			return []byte("argocd-server-abc   1/1   Running\n"), nil
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetArgoCDPods(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, out, "argocd-server")
}

func TestKubectlInspector_GetIngresses(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, []string{"get", "ingress", "-A"}, args)
			return []byte("default   my-ing   nginx   example.com\n"), nil
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetIngresses(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, out, "my-ing")
}

func TestKubectlInspector_GetScaledObjects(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, []string{"get", "scaledobjects", "-A"}, args)
			return []byte("default   my-scaler   Deployment\n"), nil
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetScaledObjects(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, out, "my-scaler")
}

func TestKubectlInspector_GetKeplerPods(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			assert.Equal(t, []string{"get", "pods", "-n", "kepler", "-l", "app.kubernetes.io/name=kepler"}, args)
			return []byte("kepler-abc   1/1   Running   0\n"), nil
		},
	}

	inspector := &KubectlInspector{Runner: runner}
	out, err := inspector.GetKeplerPods(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, out, "kepler-abc")
}
