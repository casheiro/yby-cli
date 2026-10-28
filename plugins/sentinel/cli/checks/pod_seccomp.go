//go:build k8s

package checks

import (
	"context"
	"fmt"

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
	pods, err := listarPods(ctx, client, namespace)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		// O perfil do pod vale para todos os contêineres; sem ele, cada contêiner precisa
		// ter o seu.
		hasSeccomp := pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.SeccompProfile != nil
		if !hasSeccomp && len(pod.Spec.Containers) > 0 {
			hasSeccomp = true
			for _, container := range pod.Spec.Containers {
				if container.SecurityContext == nil || container.SecurityContext.SeccompProfile == nil {
					hasSeccomp = false
					break
				}
			}
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
