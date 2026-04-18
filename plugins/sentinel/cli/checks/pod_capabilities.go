//go:build k8s

package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CapabilitiesCheck detecta containers sem drop ALL ou com SYS_ADMIN.
type CapabilitiesCheck struct{}

func init() { Register(&CapabilitiesCheck{}) }

func (c *CapabilitiesCheck) ID() string         { return "POD_CAPABILITIES" }
func (c *CapabilitiesCheck) Name() string       { return "Capabilities Linux" }
func (c *CapabilitiesCheck) Category() Category { return CategoryPodSecurity }
func (c *CapabilitiesCheck) Severity() Severity { return SeverityMedium }
func (c *CapabilitiesCheck) Description() string {
	return "Detecta capabilities perigosas ou ausência de drop ALL"
}

func (c *CapabilitiesCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			// Verificar SYS_ADMIN nas capabilities adicionadas
			if container.SecurityContext != nil && container.SecurityContext.Capabilities != nil {
				for _, cap := range container.SecurityContext.Capabilities.Add {
					if cap == "SYS_ADMIN" {
						findings = append(findings, SecurityFinding{
							CheckID:        c.ID(),
							Severity:       SeverityCritical,
							Category:       CategoryPodSecurity,
							Pod:            pod.Name,
							Container:      container.Name,
							Namespace:      namespace,
							Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
							Message:        fmt.Sprintf("Container '%s' no pod '%s' possui capability SYS_ADMIN", container.Name, pod.Name),
							Recommendation: "Remova a capability SYS_ADMIN — ela concede privilégios quase equivalentes a root",
							Type:           "critical",
							Description:    fmt.Sprintf("Container '%s' no pod '%s' possui capability SYS_ADMIN", container.Name, pod.Name),
						})
					}
				}
			}

			// Verificar se tem drop ALL
			hasDropAll := false
			if container.SecurityContext != nil && container.SecurityContext.Capabilities != nil {
				for _, cap := range container.SecurityContext.Capabilities.Drop {
					if cap == corev1.Capability("ALL") {
						hasDropAll = true
						break
					}
				}
			}
			if !hasDropAll {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityMedium,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' não possui drop ALL nas capabilities", container.Name, pod.Name),
					Recommendation: "Adicione securityContext.capabilities.drop: [\"ALL\"] e adicione apenas as capabilities necessárias",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' não possui drop ALL nas capabilities", container.Name, pod.Name),
				})
			}
		}
	}
	return findings, nil
}
