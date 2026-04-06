//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/profiles"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/remediation"
	"github.com/charmbracelet/lipgloss"
)

// ScanReport contém o resultado completo do scan de segurança.
type ScanReport struct {
	Namespace       string                   `json:"namespace"`
	Findings        []checks.SecurityFinding `json:"findings"`
	Recommendations string                   `json:"recommendations,omitempty"`
}

func scanNamespace(namespace, outputFormat, outputFile, profile string, fix, fixDryRun bool) {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Padding(0, 1)
	fmt.Println(titleStyle.Render(fmt.Sprintf("\n🔍 Sentinel Security Scan: %s", namespace)))

	k8sClient, err := getKubeClient()
	if err != nil {
		fmt.Printf("⚠️  Falha ao obter cliente Kubernetes: %v\n", err)
		return
	}

	ctx := context.Background()

	// Selecionar checks baseado no profile ou usar todos
	var selectedChecks []checks.SecurityCheck
	if profile != "" {
		p, ok := profiles.GetProfile(profile)
		if !ok {
			fmt.Printf("❌ Perfil de compliance '%s' não encontrado. Perfis disponíveis:\n", profile)
			for _, prof := range profiles.ListProfiles() {
				fmt.Printf("  - %s: %s\n", prof.Name, prof.Description)
			}
			return
		}
		selectedChecks = checks.GetByIDs(p.CheckIDs)
		fmt.Printf("📋 Usando perfil de compliance: %s (%d checks)\n", p.Name, len(selectedChecks))
	} else {
		selectedChecks = checks.GetAll()
	}

	var findings []checks.SecurityFinding

	for _, check := range selectedChecks {
		checkFindings, err := check.Run(ctx, k8sClient, namespace)
		if err != nil {
			fmt.Printf("⚠️  Erro no check '%s': %v\n", check.Name(), err)
			continue
		}
		findings = append(findings, checkFindings...)
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

	// Remediation
	if len(findings) > 0 && (fix || fixDryRun) {
		patches := remediation.GeneratePatches(findings)
		if len(patches) == 0 {
			fmt.Println("\n⚠️  Nenhum patch de remediação disponível para os findings encontrados.")
		} else if fixDryRun {
			fmt.Printf("\n🔧 Dry-run: %d patches de remediação gerados:\n", len(patches))
			for i, p := range patches {
				fmt.Printf("  %d. [%s] %s/%s — %s\n", i+1, p.ResourceKind, p.Namespace, p.ResourceName, p.Description)
				fmt.Printf("     Patch: %s\n", p.Patch)
			}
		} else {
			fmt.Printf("\n🔧 Aplicando %d patches de remediação...\n", len(patches))
			errs := remediation.ApplyPatches(ctx, k8sClient, patches)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Printf("  ❌ %v\n", e)
				}
			} else {
				fmt.Printf("  ✅ Todos os %d patches aplicados com sucesso!\n", len(patches))
			}
		}
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
