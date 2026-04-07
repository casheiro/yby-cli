//go:build k8s

package checks

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LatestTagCheck detecta containers usando :latest ou sem tag.
type LatestTagCheck struct{}

func init() { Register(&LatestTagCheck{}) }

func (c *LatestTagCheck) ID() string         { return "IMAGE_LATEST_TAG" }
func (c *LatestTagCheck) Name() string       { return "Imagem com Tag Latest" }
func (c *LatestTagCheck) Category() Category { return CategorySupplyChain }
func (c *LatestTagCheck) Severity() Severity { return SeverityMedium }
func (c *LatestTagCheck) Description() string {
	return "Detecta containers usando :latest ou sem tag definida"
}

func (c *LatestTagCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			image := container.Image
			if isLatestOrNoTag(image) {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityMedium,
					Category:       CategorySupplyChain,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' usa imagem '%s' sem tag fixa", container.Name, pod.Name, image),
					Recommendation: "Use tags de imagem imutáveis (ex: sha256 digest) em vez de :latest",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' usa imagem '%s' sem tag fixa", container.Name, pod.Name, image),
				})
			}
		}
	}
	return findings, nil
}

// isLatestOrNoTag verifica se a imagem usa :latest ou não tem tag.
func isLatestOrNoTag(image string) bool {
	// Se contém @sha256, é imutável
	if strings.Contains(image, "@sha256:") {
		return false
	}
	// Sem tag (sem ":")
	parts := strings.Split(image, ":")
	if len(parts) == 1 {
		return true
	}
	// Com :latest
	tag := parts[len(parts)-1]
	return tag == "latest"
}
