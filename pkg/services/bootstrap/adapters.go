package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// RealK8sClient implements K8sClient using kubectl commands
type RealK8sClient struct {
	Runner shared.Runner
}

func (k *RealK8sClient) WaitPodReady(ctx context.Context, label, ns string, timeout int) error {
	return k.Runner.Run(ctx, "kubectl", "wait", "--for=condition=Ready", "pod", "-l", label, "-n", ns, fmt.Sprintf("--timeout=%ds", timeout))
}

func (k *RealK8sClient) WaitCRD(ctx context.Context, crdName string, timeout int) error {
	return k.Runner.Run(ctx, "kubectl", "wait", "--for", "condition=established", fmt.Sprintf("--timeout=%ds", timeout), "crd/"+crdName)
}

func (k *RealK8sClient) NamespaceExists(ctx context.Context, ns string) (bool, error) {
	err := k.Runner.Run(ctx, "kubectl", "get", "namespace", ns)
	return err == nil, nil
}

func (k *RealK8sClient) CreateNamespace(ctx context.Context, ns string) error {
	out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "create", "namespace", ns)
	if err != nil && strings.Contains(string(out), "already exists") {
		return nil
	}
	return err
}

func (k *RealK8sClient) ApplyManifest(ctx context.Context, path string, namespace string) error {
	args := []string{"apply", "-f", path}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return k.Runner.Run(ctx, "kubectl", args...)
}

func (k *RealK8sClient) PatchApplication(ctx context.Context, name, ns, patch string) error {
	return k.Runner.Run(ctx, "kubectl", "patch", "application", name, "-n", ns, "--type", "merge", "-p", patch)
}

func (k *RealK8sClient) WaitApplicationHealthy(ctx context.Context, name, namespace string, timeoutSeconds int) error {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := k.Runner.RunCombinedOutput(ctx, "kubectl", "get", "application", name,
			"-n", namespace, "-o", "jsonpath={.status.health.status}")
		if err == nil && strings.TrimSpace(string(out)) == "Healthy" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout aguardando application %s ficar healthy", name)
}
