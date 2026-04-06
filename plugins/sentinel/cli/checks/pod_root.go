//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RootContainerCheck verifica se containers rodam como root ou não definem runAsNonRoot.
type RootContainerCheck struct{}

func init() { Register(&RootContainerCheck{}) }

func (c *RootContainerCheck) ID() string         { return "POD_ROOT_CONTAINER" }
func (c *RootContainerCheck) Name() string       { return "Container como Root" }
func (c *RootContainerCheck) Category() Category { return CategoryPodSecurity }
func (c *RootContainerCheck) Severity() Severity { return SeverityCritical }
func (c *RootContainerCheck) Description() string {
	return "Detecta containers rodando como UID 0 ou sem runAsNonRoot definido"
}

func (c *RootContainerCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil && container.SecurityContext.RunAsUser != nil && *container.SecurityContext.RunAsUser == 0 {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityCritical,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' roda como root (UID 0)", container.Name, pod.Name),
					Recommendation: "Defina runAsNonRoot: true e runAsUser com UID não-root no securityContext",
					Type:           "critical",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' roda como root (UID 0)", container.Name, pod.Name),
				})
			}

			if container.SecurityContext == nil || container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
				if container.SecurityContext == nil || container.SecurityContext.RunAsUser == nil {
					findings = append(findings, SecurityFinding{
						CheckID:        c.ID(),
						Severity:       SeverityHigh,
						Category:       CategoryPodSecurity,
						Pod:            pod.Name,
						Container:      container.Name,
						Namespace:      namespace,
						Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
						Message:        fmt.Sprintf("Container '%s' no pod '%s' não define runAsNonRoot=true", container.Name, pod.Name),
						Recommendation: "Adicione securityContext.runAsNonRoot: true ao container",
						Type:           "warning",
						Description:    fmt.Sprintf("Container '%s' no pod '%s' não define runAsNonRoot=true", container.Name, pod.Name),
					})
				}
			}
		}
	}
	return findings, nil
}
