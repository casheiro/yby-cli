package ai

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// GeminiCLIProvider implementa o Provider usando o Gemini CLI (gemini).
type GeminiCLIProvider struct {
	command string
}

// NewGeminiCLIProvider cria um provider que usa o Gemini CLI.
func NewGeminiCLIProvider() *GeminiCLIProvider {
	return &GeminiCLIProvider{command: "gemini"}
}

// Name retorna o identificador do provider.
func (p *GeminiCLIProvider) Name() string {
	return "Gemini CLI"
}

// IsAvailable verifica se o CLI gemini está instalado.
func (p *GeminiCLIProvider) IsAvailable(_ context.Context) bool {
	_, err := exec.LookPath(p.command)
	return err == nil
}

// GenerateGovernance não é suportado pelo CLI.
func (p *GeminiCLIProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, fmt.Errorf("GenerateGovernance nao suportado pelo %s", p.Name())
}

// Completion executa gemini com o prompt e retorna a resposta.
func (p *GeminiCLIProvider) Completion(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	cmd := exec.Command(p.command, "-p", fullPrompt)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gemini cli falhou: %w (stderr: %s)", err, stderr.String())
	}

	result := cleanGeminiCLIOutput(stdout.String())
	if result == "" {
		return "", fmt.Errorf("resposta vazia do gemini cli")
	}
	return result, nil
}

// cleanGeminiCLIOutput remove linhas de log/status que o Gemini CLI imprime no stdout.
func cleanGeminiCLIOutput(output string) string {
	lines := strings.Split(output, "\n")
	var cleaned []string
	for _, line := range lines {
		// Filtrar linhas de status do Gemini CLI
		if strings.HasPrefix(line, "Loaded cached credentials") ||
			strings.HasPrefix(line, "Using ") ||
			strings.HasPrefix(line, "Authenticating") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// StreamCompletion executa gemini e escreve a saída no writer.
func (p *GeminiCLIProvider) StreamCompletion(_ context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	cmd := exec.Command(p.command, "-p", fullPrompt)
	cmd.Stdout = out

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gemini cli streaming falhou: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// EmbedDocuments não é suportado pelo CLI.
func (p *GeminiCLIProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embeddings nao suportados pelo %s", p.Name())
}
