//go:build k8s

package main

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/sentinel/cli/checks"
)

// --- exportScanMarkdown ---

func TestExportScanMarkdown_SemFindings(t *testing.T) {
	report := ScanReport{
		Namespace: "prod",
		Findings:  []checks.SecurityFinding{},
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "prod") {
		t.Error("markdown deveria conter o nome do namespace")
	}
	if !strings.Contains(content, "**Resumo:** nenhum achado") {
		t.Errorf("resumo esperado 'nenhum achado', conteúdo: %s", content)
	}
	if strings.Contains(content, "## Recomendações") {
		t.Error("não deveria conter seção de recomendações quando não há findings")
	}
}

func TestExportScanMarkdown_ContaCriticaisEAvisos(t *testing.T) {
	report := ScanReport{
		Namespace: "staging",
		Findings: []checks.SecurityFinding{
			{Type: "critical", Category: checks.CategoryPodSecurity, Resource: "pod-a/app", Namespace: "staging", Description: "roda como root"},
			{Type: "critical", Category: checks.CategorySecrets, Resource: "pod-b/app", Namespace: "staging", Description: "senha hardcoded"},
			{Type: "warning", Category: checks.CategoryPodSecurity, Resource: "pod-c/app", Namespace: "staging", Description: "sem limites"},
		},
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "**Resumo:** 2 críticos, 1 médio") {
		t.Errorf("resumo esperado '2 críticos, 1 médio', conteúdo: %s", content)
	}
}

// TestExportScanMarkdown_SeveridadeComoNoTerminal: o relatório lê a severidade como o
// terminal (Severity antes de Type), então um check interno com Severity alta e Type
// "critical" sai como alto (▲) nos dois, e o baixo sai neutro (○).
func TestExportScanMarkdown_SeveridadeComoNoTerminal(t *testing.T) {
	content := exportScanMarkdown(ScanReport{
		Namespace: "prod",
		Findings: []checks.SecurityFinding{
			{Severity: checks.SeverityHigh, Type: "critical", Category: checks.CategorySupplyChain, Resource: "Deployment/api", Description: "imagem sem assinatura"},
			{Severity: "low", Type: "low", Category: checks.CategorySupplyChain, Resource: "StatefulSet/redis", Description: "pull policy"},
		},
	})
	for _, esperado := range []string{
		"**Resumo:** 1 alto, 1 baixo\n",
		"- ▲ **[supply-chain]** `Deployment/api`",
		"- ○ **[supply-chain]** `StatefulSet/redis`",
	} {
		if !strings.Contains(content, esperado) {
			t.Errorf("esperava %q no relatório:\n%s", esperado, content)
		}
	}
}

func TestExportScanMarkdown_ComRecomendacoes(t *testing.T) {
	report := ScanReport{
		Namespace: "default",
		Findings: []checks.SecurityFinding{
			{Type: "warning", Category: checks.CategoryPodSecurity, Resource: "pod-a/app", Namespace: "default", Description: "sem limites"},
		},
		Recommendations: "Adicione limites de CPU e memória a todos os containers.",
	}

	content := exportScanMarkdown(report)

	if !strings.Contains(content, "## Recomendações") {
		t.Error("markdown deveria conter seção '## Recomendações'")
	}
	if !strings.Contains(content, "Adicione limites de CPU e memória") {
		t.Errorf("markdown deveria conter o texto das recomendações, conteúdo: %s", content)
	}
}
