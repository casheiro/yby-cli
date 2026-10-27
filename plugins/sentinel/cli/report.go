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

// HealthyReport é o relatório JSON do investigate quando o pod está saudável: a IA não é
// chamada, então não há análise.
type HealthyReport struct {
	Metadata ReportMetadata `json:"metadata"`
	Healthy  bool           `json:"healthy"`
	Phase    string         `json:"phase"`
}

// exportHealthyJSON serializa o relatório de pod saudável em JSON.
func exportHealthyJSON(podName, namespace, phase string) (string, error) {
	data, err := json.MarshalIndent(HealthyReport{
		Metadata: ReportMetadata{Pod: podName, Namespace: namespace, Timestamp: time.Now()},
		Healthy:  true,
		Phase:    phase,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("falha ao serializar relatório JSON: %w", err)
	}
	return string(data), nil
}

// exportHealthyMarkdown gera o relatório de pod saudável em Markdown, com o mesmo
// cabeçalho do relatório da análise.
func exportHealthyMarkdown(podName, namespace, phase string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório Sentinel — %s/%s\n\n", namespace, podName))
	sb.WriteString(fmt.Sprintf("**Data:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("## Estado\n\n")
	sb.WriteString(fmt.Sprintf("O pod está saudável (%s): nenhum sinal de problema nos contêineres nem nos eventos; a IA não foi chamada.\n", phase))
	return sb.String()
}

// writeReport escreve o relatório no arquivo ou stdout.
func writeReport(content, filePath string) error {
	if filePath == "" {
		fmt.Print(content)
		return nil
	}
	return os.WriteFile(filePath, []byte(content), 0600)
}

// ConteinerBruto é o estado de um contêiner nos dados brutos do investigate --no-ai.
type ConteinerBruto struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	// State é running, waiting ou terminated; Reason, o motivo do Kubernetes (CrashLoopBackOff,
	// OOMKilled...), quando há.
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// DadosBrutos é o relatório JSON do investigate --no-ai: o que foi coletado do pod, sem
// análise da IA (quem pede, como o Bard, é a própria IA). Logs e mensagens de eventos já
// vão redigidos. É contrato com o Bard: mudar um campo exige mudar o lado dele.
type DadosBrutos struct {
	Metadata   ReportMetadata   `json:"metadata"`
	Healthy    bool             `json:"healthy"`
	Phase      string           `json:"phase"`
	Containers []ConteinerBruto `json:"containers"`
	Events     []eventoResumido `json:"events"`
	// Logs é o fim dos logs de cada contêiner (e da execução anterior dos que reiniciaram).
	Logs string `json:"logs"`
}

// exportDadosBrutosJSON serializa os dados brutos em JSON.
func exportDadosBrutosJSON(d DadosBrutos) (string, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("falha ao serializar relatório JSON: %w", err)
	}
	return string(data), nil
}

// exportDadosBrutosMarkdown gera os dados brutos em Markdown.
func exportDadosBrutosMarkdown(d DadosBrutos) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Relatório Sentinel — %s/%s\n\n", d.Metadata.Namespace, d.Metadata.Pod))
	sb.WriteString(fmt.Sprintf("**Data:** %s\n\n", d.Metadata.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("## Estado\n\nFase %s (dados brutos; a IA não foi chamada).\n\n", d.Phase))
	if len(d.Containers) > 0 {
		sb.WriteString("## Contêineres\n\n")
		for _, c := range d.Containers {
			sb.WriteString(fmt.Sprintf("- %s: %s, %d reinícios\n", c.Name, estadoDoConteiner(c), c.RestartCount))
		}
		sb.WriteString("\n")
	}
	if len(d.Events) > 0 {
		sb.WriteString("## Eventos\n\n")
		for _, e := range d.Events {
			sb.WriteString(fmt.Sprintf("- %s %s (x%d): %s\n", e.Tipo, e.Motivo, e.Contagem, e.Mensagem))
		}
		sb.WriteString("\n")
	}
	if strings.TrimSpace(d.Logs) != "" {
		sb.WriteString("## Logs\n\n```\n" + strings.TrimRight(d.Logs, "\n") + "\n```\n")
	}
	return sb.String()
}

// estadoDoConteiner descreve o estado do contêiner numa palavra ou duas.
func estadoDoConteiner(c ConteinerBruto) string {
	if c.Reason != "" {
		return c.State + " (" + c.Reason + ")"
	}
	return c.State
}
