//go:build k8s

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/backends"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/profiles"
	"github.com/casheiro/yby-cli/plugins/sentinel/cli/remediation"
	"github.com/charmbracelet/lipgloss"
)

// ScanReport contém o resultado completo do scan de segurança.
type ScanReport struct {
	Namespace       string                   `json:"namespace"`
	Findings        []checks.SecurityFinding `json:"findings"`
	Sources         []string                 `json:"sources,omitempty"`
	Recommendations string                   `json:"recommendations,omitempty"`
}

func scanNamespace(namespace, outputFormat, outputFile, profile string, fix, fixDryRun bool) {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true).Padding(0, 1)
	fmt.Println(titleStyle.Render(fmt.Sprintf("\n Sentinel Security Scan: %s", namespace)))

	k8sClient, err := getKubeClient()
	if err != nil {
		fmt.Printf("Falha ao obter cliente Kubernetes: %v\n", err)
		return
	}

	ctx := context.Background()

	// 1. Rodar backends de seguranca (Polaris, OPA)
	allBackends := []backends.SecurityBackend{
		backends.NewPolarisBackend(),
		backends.NewOPABackend(),
	}

	var backendFindings []backends.Finding
	var sources []string

	for _, b := range allBackends {
		if !b.IsAvailable() {
			continue
		}
		bf, err := b.ScanCluster(ctx, k8sClient, namespace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "aviso: backend %s falhou: %v\n", b.Name(), err)
			continue
		}
		if len(bf) > 0 {
			backendFindings = append(backendFindings, bf...)
			sources = append(sources, b.Name())
		}
	}

	// 2. Converter findings dos backends para o formato SecurityFinding
	var findings []checks.SecurityFinding
	for _, bf := range backendFindings {
		findings = append(findings, checks.SecurityFinding{
			CheckID:        bf.ID,
			Severity:       checks.Severity(bf.Severity),
			Category:       checks.Category(bf.Category),
			Namespace:      bf.Namespace,
			Resource:       bf.Resource,
			Message:        bf.Message,
			Recommendation: bf.Recommendation,
			Type:           bf.Severity,
			Description:    bf.Message,
		})
	}

	// 3. Se nenhum backend rodou, fallback para checks artesanais
	if len(sources) == 0 {
		fmt.Println("Usando checks internos (nenhum backend disponivel)")
		sources = append(sources, "checks-internos")

		var selectedChecks []checks.SecurityCheck
		if profile != "" {
			p, ok := profiles.GetProfile(profile)
			if !ok {
				fmt.Printf("Perfil '%s' nao encontrado. Disponiveis:\n", profile)
				for _, prof := range profiles.ListProfiles() {
					fmt.Printf("  - %s: %s\n", prof.Name, prof.Description)
				}
				return
			}
			selectedChecks = checks.GetByIDs(p.CheckIDs)
		} else {
			selectedChecks = checks.GetAll()
		}

		for _, check := range selectedChecks {
			checkFindings, err := check.Run(ctx, k8sClient, namespace)
			if err != nil {
				fmt.Fprintf(os.Stderr, "aviso: check '%s' falhou: %v\n", check.Name(), err)
				continue
			}
			findings = append(findings, checkFindings...)
		}
	}

	// 4. Deduplicar findings (mesmo recurso + mesma mensagem)
	findings = deduplicateFindings(findings)

	report := ScanReport{
		Namespace: namespace,
		Findings:  findings,
		Sources:   sources,
	}

	if len(findings) == 0 {
		fmt.Println("\nNenhuma vulnerabilidade encontrada!")
		return
	}

	fmt.Printf("\n%d vulnerabilidades encontradas.\n", len(findings))

	// Gerar recomendações via IA
	provider := ai.GetProvider(ctx, "auto")
	if provider != nil {
		fmt.Println("Gerando recomendacoes com IA...")
		findingsJSON, _ := json.MarshalIndent(findings, "", "  ")
		recommendations, err := provider.Completion(ctx, prompts.Get("sentinel.scan"), string(findingsJSON))
		if err == nil {
			report.Recommendations = recommendations
		} else {
			fmt.Fprintf(os.Stderr, "aviso: recomendacoes IA indisponiveis: %v\n", err)
		}
	}

	// Remediation
	if fix || fixDryRun {
		patches := remediation.GeneratePatches(findings)
		if len(patches) == 0 {
			fmt.Println("\nNenhum patch de remediacao disponivel para os findings encontrados.")
		} else if fixDryRun {
			fmt.Printf("\nDry-run: %d patches de remediacao gerados:\n", len(patches))
			for i, p := range patches {
				fmt.Printf("  %d. [%s] %s/%s - %s\n", i+1, p.ResourceKind, p.Namespace, p.ResourceName, p.Description)
				fmt.Printf("     Patch: %s\n", p.Patch)
			}
		} else {
			fmt.Printf("\nAplicando %d patches de remediacao...\n", len(patches))
			errs := remediation.ApplyPatches(ctx, k8sClient, patches)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Printf("  Erro: %v\n", e)
				}
			} else {
				fmt.Printf("  Todos os %d patches aplicados com sucesso!\n", len(patches))
			}
		}
	}

	// Output: JSON ou Markdown explícito vai pro arquivo especificado
	if outputFormat == "json" {
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := writeReport(string(data), outputFile); err != nil {
			fmt.Printf("Erro ao escrever relatorio: %v\n", err)
			return
		}
		if outputFile != "" {
			fmt.Printf("Relatorio JSON salvo em %s\n", outputFile)
		}
		return
	}

	if outputFormat == "markdown" && outputFile != "" {
		content := exportScanMarkdown(report)
		if err := writeReport(content, outputFile); err != nil {
			fmt.Printf("Erro ao escrever relatorio: %v\n", err)
			return
		}
		fmt.Printf("Relatorio Markdown salvo em %s\n", outputFile)
		return
	}

	// Padrão: resumo no terminal + relatório completo em ~/.yby/reports/
	reportPath := saveReportToGlobal(report, namespace)
	renderScanSummary(report, reportPath)
}

// deduplicateFindings remove findings duplicados (mesmo recurso + mesma mensagem).
func deduplicateFindings(findings []checks.SecurityFinding) []checks.SecurityFinding {
	seen := make(map[string]bool)
	var result []checks.SecurityFinding
	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s", f.Resource, f.Category, f.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, f)
	}
	return result
}

// saveReportToGlobal salva o relatório completo em ~/.yby/reports/.
func saveReportToGlobal(report ScanReport, namespace string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	reportsDir := filepath.Join(home, ".yby", "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return ""
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("sentinel-scan-%s-%s.md", namespace, date)
	reportPath := filepath.Join(reportsDir, filename)

	content := exportScanMarkdown(report)
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return ""
	}

	return reportPath
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

	sb.WriteString(fmt.Sprintf("**Resumo:** %d criticos, %d avisos\n", criticals, warnings))
	if len(report.Sources) > 0 {
		sb.WriteString(fmt.Sprintf("**Backends:** %s\n", strings.Join(report.Sources, ", ")))
	}
	sb.WriteString("\n")
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

// renderScanSummary renderiza um resumo compacto do scan no terminal.
func renderScanSummary(report ScanReport, reportPath string) {
	if len(report.Sources) > 0 {
		fmt.Printf("  Backends: %s\n", strings.Join(report.Sources, ", "))
	}
	// Contar por severidade
	bySeverity := make(map[string]int)
	byCategory := make(map[string]int)

	for _, f := range report.Findings {
		sev := string(f.Severity)
		if sev == "" {
			sev = f.Type // backward compat
		}
		bySeverity[sev]++
		byCategory[string(f.Category)]++
	}

	fmt.Println()
	fmt.Println("  Por severidade:")
	severityOrder := []string{"critical", "high", "medium", "low", "info"}
	for _, s := range severityOrder {
		if count, ok := bySeverity[s]; ok {
			fmt.Printf("    %-12s %d\n", s, count)
		}
	}

	fmt.Println()
	fmt.Println("  Por categoria:")
	for cat, count := range byCategory {
		fmt.Printf("    %-20s %d\n", cat, count)
	}

	if reportPath != "" {
		fmt.Println()
		fmt.Printf("  Relatorio completo: %s\n", reportPath)
	}
	fmt.Println()
}
