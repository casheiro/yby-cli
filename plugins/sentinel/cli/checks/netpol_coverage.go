//go:build k8s

package checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// NetpolCoverageCheck verifica se pods estão cobertos por NetworkPolicies.
type NetpolCoverageCheck struct{}

func init() { Register(&NetpolCoverageCheck{}) }

func (c *NetpolCoverageCheck) ID() string         { return "NETPOL_COVERAGE" }
func (c *NetpolCoverageCheck) Name() string       { return "Cobertura de NetworkPolicy" }
func (c *NetpolCoverageCheck) Category() Category { return CategoryNetwork }
func (c *NetpolCoverageCheck) Severity() Severity { return SeverityMedium }
func (c *NetpolCoverageCheck) Description() string {
	return "Detecta pods sem cobertura de NetworkPolicy"
}

func (c *NetpolCoverageCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	netpols, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar NetworkPolicies: %w", err)
	}

	var findings []SecurityFinding
	for _, pod := range pods.Items {
		covered := false
		podLabels := labels.Set(pod.Labels)
		for _, np := range netpols.Items {
			selector, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
			if err != nil {
				continue
			}
			if selector.Matches(podLabels) {
				covered = true
				break
			}
		}
		if !covered {
			findings = append(findings, SecurityFinding{
				CheckID:        c.ID(),
				Severity:       SeverityMedium,
				Category:       CategoryNetwork,
				Pod:            pod.Name,
				Namespace:      namespace,
				Resource:       pod.Name,
				Message:        fmt.Sprintf("Pod '%s' não é coberto por nenhuma NetworkPolicy", pod.Name),
				Recommendation: "Crie uma NetworkPolicy que selecione este pod para restringir tráfego de rede",
				Type:           "warning",
				Description:    fmt.Sprintf("Pod '%s' não é coberto por nenhuma NetworkPolicy", pod.Name),
			})
		}
	}
	return findings, nil
}
