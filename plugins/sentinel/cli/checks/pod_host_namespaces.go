//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HostNamespacesCheck detecta pods usando host PID, IPC ou Network.
type HostNamespacesCheck struct{}

func init() { Register(&HostNamespacesCheck{}) }

func (c *HostNamespacesCheck) ID() string         { return "POD_HOST_NAMESPACES" }
func (c *HostNamespacesCheck) Name() string       { return "Host Namespaces" }
func (c *HostNamespacesCheck) Category() Category { return CategoryPodSecurity }
func (c *HostNamespacesCheck) Severity() Severity { return SeverityHigh }
func (c *HostNamespacesCheck) Description() string {
	return "Detecta pods com hostPID, hostIPC ou hostNetwork habilitados"
}

func (c *HostNamespacesCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		if pod.Spec.HostPID {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityHigh,
				Category:       CategoryPodSecurity,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' usa hostPID — acesso ao namespace PID do host", pod.Name),
				Recommendation: "Remova hostPID: true a menos que seja estritamente necessário",
				Type:           "critical",
				Description:    fmt.Sprintf("Pod '%s' usa hostPID — acesso ao namespace PID do host", pod.Name),
			})
		}
		if pod.Spec.HostIPC {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityHigh,
				Category:       CategoryPodSecurity,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' usa hostIPC — acesso ao namespace IPC do host", pod.Name),
				Recommendation: "Remova hostIPC: true a menos que seja estritamente necessário",
				Type:           "critical",
				Description:    fmt.Sprintf("Pod '%s' usa hostIPC — acesso ao namespace IPC do host", pod.Name),
			})
		}
		if pod.Spec.HostNetwork {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityHigh,
				Category:       CategoryPodSecurity,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' usa hostNetwork — acesso direto à rede do host", pod.Name),
				Recommendation: "Remova hostNetwork: true e use Services/Ingress para exposição de rede",
				Type:           "critical",
				Description:    fmt.Sprintf("Pod '%s' usa hostNetwork — acesso direto à rede do host", pod.Name),
			})
		}
	}
	return findings, nil
}
