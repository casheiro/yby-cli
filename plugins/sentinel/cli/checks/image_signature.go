//go:build k8s

package checks

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImageSignatureCheck verifica se imagens possuem assinatura cosign.
type ImageSignatureCheck struct {
	// ExecCommand permite injetar mock para testes.
	ExecCommand func(name string, args ...string) *exec.Cmd
}

func init() { Register(&ImageSignatureCheck{ExecCommand: exec.Command}) }

func (c *ImageSignatureCheck) ID() string         { return "IMAGE_SIGNATURE" }
func (c *ImageSignatureCheck) Name() string       { return "Assinatura de Imagem" }
func (c *ImageSignatureCheck) Category() Category { return CategorySupplyChain }
func (c *ImageSignatureCheck) Severity() Severity { return SeverityHigh }
func (c *ImageSignatureCheck) Description() string {
	return "Verifica se imagens de containers possuem assinatura cosign"
}

func (c *ImageSignatureCheck) Run(ctx context.Context, client kubernetes.Interface, namespace string) ([]SecurityFinding, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("falha ao listar pods: %w", err)
	}

	execCmd := c.ExecCommand
	if execCmd == nil {
		execCmd = exec.Command
	}

	// Verificar se cosign está disponível
	if _, err := execCmd("cosign", "version").CombinedOutput(); err != nil {
		return []SecurityFinding{{
			CheckID:        c.ID(),
			Severity:       SeverityInfo,
			Category:       CategorySupplyChain,
			Namespace:      namespace,
			Resource:       "cosign",
			Message:        "cosign não encontrado — verificação de assinatura de imagens indisponível",
			Recommendation: "Instale cosign: https://docs.sigstore.dev/cosign/system_config/installation/",
			Type:           "info",
			Description:    "cosign não encontrado — verificação de assinatura de imagens indisponível",
		}}, nil
	}

	// Coletar imagens únicas
	imageSet := make(map[string][]string) // imagem -> []pod/container
	for _, pod := range pods.Items {
		allContainers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
		for _, container := range allContainers {
			key := container.Image
			imageSet[key] = append(imageSet[key], fmt.Sprintf("%s/%s", pod.Name, container.Name))
		}
	}

	var findings []SecurityFinding
	for image, refs := range imageSet {
		cmd := execCmd("cosign", "verify", "--output", "text", image)
		if _, err := cmd.CombinedOutput(); err != nil {
			for _, ref := range refs {
				parts := strings.SplitN(ref, "/", 2)
				podName := parts[0]
				containerName := ""
				if len(parts) > 1 {
					containerName = parts[1]
				}
				findings = append(findings, SecurityFinding{
					CheckID:        c.ID(),
					Severity:       SeverityHigh,
					Category:       CategorySupplyChain,
					Pod:            podName,
					Container:      containerName,
					Namespace:      namespace,
					Resource:       ref,
					Message:        fmt.Sprintf("Imagem '%s' não possui assinatura cosign verificada", image),
					Recommendation: "Assine imagens com cosign antes do deploy: cosign sign <imagem>",
					Type:           "critical",
					Description:    fmt.Sprintf("Imagem '%s' não possui assinatura cosign verificada", image),
				})
			}
		}
	}

	return findings, nil
}
