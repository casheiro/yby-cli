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
	model   string // vazio deixa o CLI escolher o modelo
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

// args monta os argumentos do claude: -p sem texto, que lê o prompt do stdin, e, com modelo
// escolhido, --model (o CLI aceita os apelidos sonnet, opus e haiku e os nomes completos).
func (p *ClaudeCLIProvider) args() []string {
	if p.model == "" {
		return []string{"-p"}
	}
	return []string{"--model", p.model, "-p"}
}

// GenerateGovernance não é suportado pelo CLI.
func (p *ClaudeCLIProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, fmt.Errorf("GenerateGovernance nao suportado pelo %s", p.Name())
}

// Completion executa claude -p com o prompt pelo stdin e retorna a resposta.
func (p *ClaudeCLIProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var stdout strings.Builder
	stderr, err := executarCLI(ctx, p.command, p.args(), systemPrompt+"\n\n"+userPrompt, &stdout)
	if err != nil {
		return "", erroCLI("claude cli", "", err, stderr)
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", fmt.Errorf("resposta vazia do claude cli")
	}
	return result, nil
}

// StreamCompletion executa claude -p e escreve a saída no writer.
func (p *ClaudeCLIProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	stderr, err := executarCLI(ctx, p.command, p.args(), systemPrompt+"\n\n"+userPrompt, out)
	if err != nil {
		return erroCLI("claude cli", " streaming", err, stderr)
	}
	return nil
}

// EmbedDocuments não é suportado pelo CLI.
func (p *ClaudeCLIProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("embeddings nao suportados pelo %s", p.Name())
}
