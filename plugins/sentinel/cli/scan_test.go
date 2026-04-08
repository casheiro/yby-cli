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
	if !strings.Contains(content, "0 criticos, 0 avisos") {
		t.Errorf("resumo esperado '0 criticos, 0 avisos', conteúdo: %s", content)
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

	if !strings.Contains(content, "2 criticos, 1 avisos") {
		t.Errorf("resumo esperado '2 criticos, 1 avisos', conteúdo: %s", content)
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
