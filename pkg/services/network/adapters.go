package network

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// RealClusterNetworkManager implementa ClusterNetworkManager usando shared.Runner
type RealClusterNetworkManager struct {
	Runner shared.Runner
}

// NewClusterNetworkAdapter cria um adaptador de rede com o runner real
func NewClusterNetworkAdapter() *RealClusterNetworkManager {
	return &RealClusterNetworkManager{
		Runner: &shared.RealRunner{},
	}
}

func (m *RealClusterNetworkManager) GetCurrentContext() (string, error) {
	out, err := m.Runner.RunCombinedOutput(context.Background(), "kubectl", "config", "current-context")
	return strings.TrimSpace(string(out)), err
}

func (m *RealClusterNetworkManager) GetSecretValue(ctx context.Context, kubeContext, ns, secret, jsonPathKey string) (string, error) {
	out, err := m.Runner.RunCombinedOutput(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "-n", ns, "get", "secret", secret, fmt.Sprintf("-o=jsonpath={.data.%s}", jsonPathKey))
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(out))
	return string(decoded), err
}

func (m *RealClusterNetworkManager) HasService(ctx context.Context, kubeContext, ns, service string) bool {
	err := m.Runner.Run(ctx, "kubectl", "--context", kubeContext, "-n", ns, "get", "svc", service)
	return err == nil
}

func (m *RealClusterNetworkManager) PortForward(ctx context.Context, kubeContext, ns, resource, ports string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := m.Runner.Run(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "-n", ns, "port-forward", resource, ports); err != nil {
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
	out, err := m.Runner.RunCombinedOutput(ctx, "kubectl", "--context", kubeContext, "--insecure-skip-tls-verify", "create", "token", serviceAccount, "-n", ns, "--duration="+duration)
	return strings.TrimSpace(string(out)), err
}

func (m *RealClusterNetworkManager) KillPortForward(port string) {
	_ = m.Runner.Run(context.Background(), "pkill", "-f", fmt.Sprintf("port-forward.*%s", port))
}

// DockerContainerManager implementa LocalContainerManager usando shared.Runner
type DockerContainerManager struct {
	Runner shared.Runner
}

// NewContainerAdapter cria um adaptador de containers com o runner real
func NewContainerAdapter() *DockerContainerManager {
	return &DockerContainerManager{
		Runner: &shared.RealRunner{},
	}
}

func (d *DockerContainerManager) IsAvailable() bool {
	_, err := d.Runner.LookPath("docker")
	return err == nil
}

func (d *DockerContainerManager) StartGrafana(ctx context.Context) error {
	addHost := "--add-host=host.docker.internal:host-gateway"

	_ = d.Runner.Run(ctx, "docker", "volume", "create", "yby-grafana-data")
	_ = d.Runner.Run(ctx, "docker", "rm", "-f", "yby-grafana")

	out, err := d.Runner.RunCombinedOutput(ctx, "docker", "run", "-d",
		"--name", "yby-grafana",
		"-p", "3001:3000",
		"-v", "yby-grafana-data:/var/lib/grafana",
		addHost,
		"grafana/grafana:latest")

	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}
