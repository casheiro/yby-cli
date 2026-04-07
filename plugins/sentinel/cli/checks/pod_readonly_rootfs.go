//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ReadOnlyRootfsCheck detecta containers sem readOnlyRootFilesystem.
type ReadOnlyRootfsCheck struct{}

func init() { Register(&ReadOnlyRootfsCheck{}) }

func (c *ReadOnlyRootfsCheck) ID() string         { return "POD_READONLY_ROOTFS" }
func (c *ReadOnlyRootfsCheck) Name() string       { return "Root Filesystem Somente-Leitura" }
func (c *ReadOnlyRootfsCheck) Category() Category { return CategoryPodSecurity }
func (c *ReadOnlyRootfsCheck) Severity() Severity { return SeverityLow }
func (c *ReadOnlyRootfsCheck) Description() string {
	return "Detecta containers sem readOnlyRootFilesystem: true"
}

func (c *ReadOnlyRootfsCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityLow,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' não possui readOnlyRootFilesystem: true", container.Name, pod.Name),
					Recommendation: "Defina securityContext.readOnlyRootFilesystem: true e use volumes para diretórios que precisam de escrita",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' não possui readOnlyRootFilesystem: true", container.Name, pod.Name),
				})
			}
		}
	}
	return findings, nil
}
