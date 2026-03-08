package network

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type RealClusterNetworkManager struct{}

func NewClusterNetworkAdapter() *RealClusterNetworkManager {
	return &RealClusterNetworkManager{}
}

func (m *RealClusterNetworkManager) GetCurrentContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimSpace(string(out)), err
}

func (m *RealClusterNetworkManager) GetSecretValue(ctx context.Context, kubeContext, ns, secret, jsonPathKey string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "-n", ns, "get", "secret", secret, fmt.Sprintf("-o=jsonpath={.data.%s}", jsonPathKey))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(out))
	return string(decoded), err
}

func (m *RealClusterNetworkManager) HasService(ctx context.Context, kubeContext, ns, service string) bool {
	err := exec.CommandContext(ctx, "kubectl", "--context", kubeContext, "-n", ns, "get", "svc", service).Run()
	return err == nil
}

func (m *RealClusterNetworkManager) PortForward(ctx context.Context, kubeContext, ns, resource, ports string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cmd := exec.CommandContext(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "-n", ns, "port-forward", resource, ports)
			cmd.Stdout = nil
			cmd.Stderr = nil

			if err := cmd.Run(); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				time.Sleep(2 * time.Second)
				continue
			}
			return nil
		}
	}
}

func (m *RealClusterNetworkManager) CreateToken(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "create", "token", serviceAccount, "-n", ns, "--duration="+duration)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func (m *RealClusterNetworkManager) KillPortForward(port string) {
	_ = exec.Command("pkill", "-f", fmt.Sprintf("port-forward.*%s", port)).Run()
}

// Docker implementation
type DockerContainerManager struct{}

func NewContainerAdapter() *DockerContainerManager {
	return &DockerContainerManager{}
}

func (d *DockerContainerManager) IsAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func (d *DockerContainerManager) StartGrafana(ctx context.Context) error {
	addHost := "--add-host=host.docker.internal:host-gateway"

	_ = exec.CommandContext(ctx, "docker", "volume", "create", "yby-grafana-data").Run()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", "yby-grafana").Run()

	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", "yby-grafana",
		"-p", "3001:3000",
		"-v", "yby-grafana-data:/var/lib/grafana",
		addHost,
		"grafana/grafana:latest")

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}
