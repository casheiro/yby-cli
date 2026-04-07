package ai

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ClaudeCLIProvider implementa o Provider usando o Claude Code CLI (claude -p).
type ClaudeCLIProvider struct {
	command string
}

// NewClaudeCLIProvider cria um provider que usa o Claude Code CLI.
func NewClaudeCLIProvider() *ClaudeCLIProvider {
	return &ClaudeCLIProvider{command: "claude"}
}

// Name retorna o identificador do provider.
func (p *ClaudeCLIProvider) Name() string {
	return "Claude Code CLI"
}

// IsAvailable verifica se o CLI claude está instalado.
func (p *ClaudeCLIProvider) IsAvailable(_ context.Context) bool {
	_, err := exec.LookPath(p.command)
	return err == nil
}

// GenerateGovernance não é suportado pelo CLI.
func (p *ClaudeCLIProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, fmt.Errorf("GenerateGovernance nao suportado pelo %s", p.Name())
}

// Completion executa claude -p com o prompt e retorna a resposta.
func (p *ClaudeCLIProvider) Completion(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	cmd := exec.Command(p.command, "-p", fullPrompt)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude cli falhou: %w (stderr: %s)", err, stderr.String())
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", fmt.Errorf("resposta vazia do claude cli")
	}
	return result, nil
}

// StreamCompletion executa claude -p e escreve a saída no writer.
func (p *ClaudeCLIProvider) StreamCompletion(_ context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	cmd := exec.Command(p.command, "-p", fullPrompt)
	cmd.Stdout = out

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude cli streaming falhou: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// EmbedDocuments não é suportado pelo CLI.
func (p *ClaudeCLIProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embeddings nao suportados pelo %s", p.Name())
}
