//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HostPortsCheck detecta containers usando hostPort.
type HostPortsCheck struct{}

func init() { Register(&HostPortsCheck{}) }

func (c *HostPortsCheck) ID() string         { return "POD_HOST_PORTS" }
func (c *HostPortsCheck) Name() string       { return "Host Ports" }
func (c *HostPortsCheck) Category() Category { return CategoryPodSecurity }
func (c *HostPortsCheck) Severity() Severity { return SeverityMedium }
func (c *HostPortsCheck) Description() string {
	return "Detecta containers que expõem portas diretamente no host"
}

func (c *HostPortsCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			for _, port := range container.Ports {
				if port.HostPort != 0 {
					findings = append(findings, SecurityFinding{
						CheckID:        c.ID(),
						Severity:       SeverityMedium,
						Category:       CategoryPodSecurity,
						Pod:            pod.Name,
						Container:      container.Name,
						Namespace:      namespace,
						Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
						Message:        fmt.Sprintf("Container '%s' no pod '%s' usa hostPort %d", container.Name, pod.Name, port.HostPort),
						Recommendation: "Use Services do tipo NodePort ou LoadBalancer em vez de hostPort",
						Type:           "warning",
						Description:    fmt.Sprintf("Container '%s' no pod '%s' usa hostPort %d", container.Name, pod.Name, port.HostPort),
					})
				}
			}
		}
	}
	return findings, nil
}
