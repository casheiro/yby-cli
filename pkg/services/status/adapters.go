package status

import (
	"context"
	"strings"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// KubectlInspector implementa ClusterInspector usando kubectl via shared.Runner.
type KubectlInspector struct {
	Runner shared.Runner
}

func (k *KubectlInspector) GetNodes(ctx context.Context) (string, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "nodes")
	return strings.TrimSpace(string(out)), err
}

func (k *KubectlInspector) GetArgoCDPods(ctx context.Context) (string, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "pods", "-n", "argocd")
	return strings.TrimSpace(string(out)), err
}

func (k *KubectlInspector) GetIngresses(ctx context.Context) (string, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "ingress", "-A")
	return strings.TrimSpace(string(out)), err
}

func (k *KubectlInspector) GetScaledObjects(ctx context.Context) (string, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "scaledobjects", "-A")
	return strings.TrimSpace(string(out)), err
}

func (k *KubectlInspector) GetKeplerPods(ctx context.Context) (string, error) {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "pods", "-n", "kepler", "-l", "app.kubernetes.io/name=kepler")
	return strings.TrimSpace(string(out)), err
}
