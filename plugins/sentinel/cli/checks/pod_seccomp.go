//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SeccompCheck detecta pods sem perfil seccomp definido.
type SeccompCheck struct{}

func init() { Register(&SeccompCheck{}) }

func (c *SeccompCheck) ID() string          { return "POD_SECCOMP" }
func (c *SeccompCheck) Name() string        { return "Perfil Seccomp" }
func (c *SeccompCheck) Category() Category  { return CategoryPodSecurity }
func (c *SeccompCheck) Severity() Severity  { return SeverityLow }
func (c *SeccompCheck) Description() string { return "Detecta pods sem perfil seccomp definido" }

func (c *SeccompCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		hasSeccomp := false
		if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.SeccompProfile != nil {
			hasSeccomp = true
		}
		if !hasSeccomp {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityLow,
				Category:       CategoryPodSecurity,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' não possui perfil seccomp definido", pod.Name),
				Recommendation: "Defina spec.securityContext.seccompProfile.type: RuntimeDefault",
				Type:           "warning",
				Description:    fmt.Sprintf("Pod '%s' não possui perfil seccomp definido", pod.Name),
			})
		}
	}
	return findings, nil
}
