//go:build k8s

package checks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImagePullPolicyCheck verifica se o ImagePullPolicy do container é Always.
type ImagePullPolicyCheck struct{}

func init() { Register(&ImagePullPolicyCheck{}) }

func (c *ImagePullPolicyCheck) ID() string         { return "POD_IMAGE_PULL_POLICY" }
func (c *ImagePullPolicyCheck) Name() string       { return "ImagePullPolicy" }
func (c *ImagePullPolicyCheck) Category() Category { return CategoryPodSecurity }
func (c *ImagePullPolicyCheck) Severity() Severity { return SeverityLow }
func (c *ImagePullPolicyCheck) Description() string {
	return "Detecta containers sem ImagePullPolicy=Always"
}

func (c *ImagePullPolicyCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if container.ImagePullPolicy != corev1.PullAlways {
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityLow,
					Category:       CategoryPodSecurity,
					Pod:            pod.Name,
					Container:      container.Name,
					Namespace:      namespace,
					Resource:       fmt.Sprintf("%s/%s", pod.Name, container.Name),
					Message:        fmt.Sprintf("Container '%s' no pod '%s' com ImagePullPolicy=%s (recomendado: Always)", container.Name, pod.Name, container.ImagePullPolicy),
					Recommendation: "Defina imagePullPolicy: Always para garantir que imagens sejam sempre baixadas do registry",
					Type:           "warning",
					Description:    fmt.Sprintf("Container '%s' no pod '%s' com ImagePullPolicy=%s (recomendado: Always)", container.Name, pod.Name, container.ImagePullPolicy),
				})
			}
		}
	}
	return findings, nil
}
