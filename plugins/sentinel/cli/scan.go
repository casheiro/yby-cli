//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityFinding representa uma vulnerabilidade encontrada.
type SecurityFinding struct {
	Resource    string `json:"resource"`
	Namespace   string `json:"namespace"`
	Type        string `json:"type"`     // "warning", "critical"
	Category    string `json:"category"` // "root_container", "no_limits", "image_pull_policy", "exposed_secrets"
	Description string `json:"description"`
}

// ScanReport contém o resultado completo do scan de segurança.
type ScanReport struct {
	Namespace       string            `json:"namespace"`
	Findings        []SecurityFinding `json:"findings"`
	Recommendations string            `json:"recommendations,omitempty"`
}

func scanNamespace(namespace, outputFormat, outputFile string) {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Padding(0, 1)
	fmt.Println(titleStyle.Render(fmt.Sprintf("\n🔍 Sentinel Security Scan: %s", namespace)))

	k8sClient, err := sdk.GetKubeClient()
	if err != nil {
		fmt.Printf("⚠️  Falha ao obter cliente Kubernetes: %v\n", err)
		return
	}

	ctx := context.Background()

	// Listar pods no namespace
	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("❌ Falha ao listar pods: %v\n", err)
		return
	}

	var findings []SecurityFinding

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			findings = append(findings, checkRootContainer(pod, container, namespace)...)
			findings = append(findings, checkResourceLimits(pod, container, namespace)...)
			findings = append(findings, checkImagePullPolicy(pod, container, namespace)...)
			findings = append(findings, checkExposedSecrets(pod, container, namespace)...)
		}
	}

	report := ScanReport{
		Namespace: namespace,
		Findings:  findings,
	}

	// Gerar recomendações via IA se houver findings
	if len(findings) > 0 {
		fmt.Printf("\n⚠️  %d vulnerabilidades encontradas. Gerando recomendações...\n", len(findings))

		provider := ai.GetProvider(ctx, "auto")
		if provider != nil {
			findingsJSON, _ := json.MarshalIndent(findings, "", "  ")
			recommendations, err := provider.Completion(ctx, ScanSystemPrompt, string(findingsJSON))
			if err == nil {
				report.Recommendations = recommendations
			}
		}
	} else {
		fmt.Println("\n✅ Nenhuma vulnerabilidade encontrada!")
	}

	// Output
	if outputFormat == "json" {
		data, _ := json.MarshalIndent(report, "", "  ")
		content := string(data)
		if err := writeReport(content, outputFile); err != nil {
			fmt.Printf("❌ Erro ao escrever relatório: %v\n", err)
			return
		}
		if outputFile != "" {
			fmt.Printf("✅ Relatório JSON salvo em %s\n", outputFile)
		}
		return
	}

	if outputFormat == "markdown" {
		content := exportScanMarkdown(report)
		if err := writeReport(content, outputFile); err != nil {
			fmt.Printf("❌ Erro ao escrever relatório: %v\n", err)
			return
		}
		if outputFile != "" {
			fmt.Printf("✅ Relatório Markdown salvo em %s\n", outputFile)
		}
		return
	}

	// Renderização visual (padrão)
	renderScanResult(report)
}

// checkRootContainer verifica se um container roda como root ou não define runAsNonRoot.
func checkRootContainer(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding {
	var findings []SecurityFinding

	if container.SecurityContext != nil && container.SecurityContext.RunAsUser != nil && *container.SecurityContext.RunAsUser == 0 {
		findings = append(findings, SecurityFinding{
			Resource:    fmt.Sprintf("%s/%s", pod.Name, container.Name),
			Namespace:   namespace,
			Type:        "critical",
			Category:    "root_container",
			Description: fmt.Sprintf("Container '%s' no pod '%s' roda como root (UID 0)", container.Name, pod.Name),
		})
	}

	if container.SecurityContext == nil || container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		if container.SecurityContext == nil || container.SecurityContext.RunAsUser == nil {
			findings = append(findings, SecurityFinding{
				Resource:    fmt.Sprintf("%s/%s", pod.Name, container.Name),
				Namespace:   namespace,
				Type:        "warning",
				Category:    "root_container",
				Description: fmt.Sprintf("Container '%s' no pod '%s' não define runAsNonRoot=true", container.Name, pod.Name),
			})
		}
	}

	return findings
}

// checkResourceLimits verifica se um container possui limites de CPU/memória definidos.
func checkResourceLimits(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding {
	var findings []SecurityFinding

	if container.Resources.Limits == nil || (container.Resources.Limits.Cpu().IsZero() && container.Resources.Limits.Memory().IsZero()) {
		findings = append(findings, SecurityFinding{
			Resource:    fmt.Sprintf("%s/%s", pod.Name, container.Name),
			Namespace:   namespace,
			Type:        "warning",
			Category:    "no_limits",
			Description: fmt.Sprintf("Container '%s' no pod '%s' sem limites de CPU/memória definidos", container.Name, pod.Name),
		})
	}

	return findings
}

// checkImagePullPolicy verifica se o ImagePullPolicy do container é Always.
func checkImagePullPolicy(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding {
	var findings []SecurityFinding

	if container.ImagePullPolicy != corev1.PullAlways {
		findings = append(findings, SecurityFinding{
			Resource:    fmt.Sprintf("%s/%s", pod.Name, container.Name),
			Namespace:   namespace,
			Type:        "warning",
			Category:    "image_pull_policy",
			Description: fmt.Sprintf("Container '%s' no pod '%s' com ImagePullPolicy=%s (recomendado: Always)", container.Name, pod.Name, container.ImagePullPolicy),
		})
	}

	return findings
}

// checkExposedSecrets verifica se variáveis de ambiente contêm secrets com valores hardcoded.
func checkExposedSecrets(pod corev1.Pod, container corev1.Container, namespace string) []SecurityFinding {
	var findings []SecurityFinding

	for _, env := range container.Env {
		lowerName := strings.ToLower(env.Name)
		if (strings.Contains(lowerName, "password") || strings.Contains(lowerName, "secret") || strings.Contains(lowerName, "token") || strings.Contains(lowerName, "key")) && env.ValueFrom == nil {
			findings = append(findings, SecurityFinding{
				Resource:    fmt.Sprintf("%s/%s", pod.Name, container.Name),
				Namespace:   namespace,
				Type:        "critical",
				Category:    "exposed_secrets",
				Description: fmt.Sprintf("Container '%s' no pod '%s' tem env '%s' com valor hardcoded (use secretKeyRef)", container.Name, pod.Name, env.Name),
			})
		}
	}

	return findings
}

// exportScanMarkdown gera o relatório de scan em formato Markdown.
func exportScanMarkdown(report ScanReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório de Segurança Sentinel — %s\n\n", report.Namespace))

	criticals := 0
	warnings := 0
	for _, f := range report.Findings {
		if f.Type == "critical" {
			criticals++
		} else {
			warnings++
		}
	}

	sb.WriteString(fmt.Sprintf("**Resumo:** %d críticos, %d avisos\n\n", criticals, warnings))
	sb.WriteString("## Vulnerabilidades\n\n")

	for _, f := range report.Findings {
		icon := "⚠️"
		if f.Type == "critical" {
			icon = "🚨"
		}
		sb.WriteString(fmt.Sprintf("- %s **[%s]** `%s`: %s\n", icon, f.Category, f.Resource, f.Description))
	}

	if report.Recommendations != "" {
		sb.WriteString("\n## Recomendações\n\n")
		sb.WriteString(report.Recommendations + "\n")
	}

	return sb.String()
}

// renderScanResult renderiza o resultado do scan com estilo visual.
func renderScanResult(report ScanReport) {
	width := 80
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		BorderForeground(lipgloss.Color("63")).
		Width(width)

	var sb strings.Builder

	for _, f := range report.Findings {
		color := "220" // amarelo
		icon := "⚠️"
		if f.Type == "critical" {
			color = "196" // vermelho
			icon = "🚨"
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(
			fmt.Sprintf("%s [%s] %s: %s", icon, f.Category, f.Resource, f.Description)) + "\n")
	}

	if report.Recommendations != "" {
		sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("💡 Recomendações da IA:") + "\n")
		sb.WriteString(report.Recommendations + "\n")
	}

	fmt.Println(boxStyle.Render(sb.String()))
}
