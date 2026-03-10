//go:build k8s

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ReportMetadata contém metadados do relatório de investigação.
type ReportMetadata struct {
	Pod       string    `json:"pod"`
	Namespace string    `json:"namespace"`
	Timestamp time.Time `json:"timestamp"`
}

// FullReport combina resultado da análise com metadados.
type FullReport struct {
	Metadata ReportMetadata `json:"metadata"`
	Analysis AnalysisResult `json:"analysis"`
}

// exportJSON serializa o relatório em formato JSON.
func exportJSON(result AnalysisResult, podName, namespace string) (string, error) {
	report := FullReport{
		Metadata: ReportMetadata{
			Pod:       podName,
			Namespace: namespace,
			Timestamp: time.Now(),
		},
		Analysis: result,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("falha ao serializar relatório JSON: %w", err)
	}
	return string(data), nil
}

// exportMarkdown gera o relatório em formato Markdown.
func exportMarkdown(result AnalysisResult, podName, namespace string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório Sentinel — %s/%s\n\n", namespace, podName))
	sb.WriteString(fmt.Sprintf("**Data:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("## Causa Raiz\n\n")
	sb.WriteString(result.RootCause + "\n\n")
	sb.WriteString("## Detalhe Técnico\n\n")
	sb.WriteString(result.TechnicalDetail + "\n\n")
	sb.WriteString(fmt.Sprintf("## Confiança\n\n%d%%\n\n", result.Confidence))
	sb.WriteString("## Sugestão de Correção\n\n")
	sb.WriteString(result.SuggestedFix + "\n\n")
	if result.KubectlPatch != nil && *result.KubectlPatch != "none" && *result.KubectlPatch != "" {
		sb.WriteString("## Comando Sugerido\n\n")
		sb.WriteString(fmt.Sprintf("```bash\n%s\n```\n", *result.KubectlPatch))
	}
	return sb.String()
}

// writeReport escreve o relatório no arquivo ou stdout.
func writeReport(content, filePath string) error {
	if filePath == "" {
		fmt.Print(content)
		return nil
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}
