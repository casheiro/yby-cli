package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
)

// EstimateTokens retorna uma estimativa de tokens para o texto informado.
// Usa a heurística ~4 caracteres por token (padrão para modelos BPE).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4 // arredondamento para cima
}

// ModelMetadata armazena metadados de um modelo de IA.
type ModelMetadata struct {
	Name          string
	ContextWindow int // limite de tokens da context window
}

// knownModels é o registry de modelos conhecidos e seus limites de contexto.
var knownModels = map[string]ModelMetadata{
	// OpenAI
	"gpt-4o-mini":   {Name: "gpt-4o-mini", ContextWindow: 128_000},
	"gpt-4o":        {Name: "gpt-4o", ContextWindow: 128_000},
	"gpt-4-turbo":   {Name: "gpt-4-turbo", ContextWindow: 128_000},
	"gpt-4":         {Name: "gpt-4", ContextWindow: 8_192},
	"gpt-3.5-turbo": {Name: "gpt-3.5-turbo", ContextWindow: 16_385},

	// Google Gemini
	"gemini-2.5-flash": {Name: "gemini-2.5-flash", ContextWindow: 1_000_000},
	"gemini-2.5-pro":   {Name: "gemini-2.5-pro", ContextWindow: 1_000_000},
	"gemini-2.0-flash": {Name: "gemini-2.0-flash", ContextWindow: 1_000_000},
	"gemini-1.5-flash": {Name: "gemini-1.5-flash", ContextWindow: 1_000_000},
	"gemini-1.5-pro":   {Name: "gemini-1.5-pro", ContextWindow: 2_000_000},

	// AWS Bedrock
	"anthropic.claude-3-5-sonnet-20241022-v2:0": {Name: "claude-3.5-sonnet-bedrock", ContextWindow: 200_000},
	"anthropic.claude-3-haiku-20240307-v1:0":    {Name: "claude-3-haiku-bedrock", ContextWindow: 200_000},
	"amazon.titan-embed-text-v2:0":              {Name: "titan-embed-v2", ContextWindow: 8_192},

	// Ollama (modelos comuns — limites variam por quantização). O nome vale com qualquer
	// tag ("llama3.1:8b", "qwen2.5:7b-instruct-q4_K_M").
	"llama3":        {Name: "llama3", ContextWindow: 8_192},
	"llama3.1":      {Name: "llama3.1", ContextWindow: 128_000},
	"llama3.2":      {Name: "llama3.2", ContextWindow: 128_000},
	"llama3.3":      {Name: "llama3.3", ContextWindow: 128_000},
	"qwen2.5":       {Name: "qwen2.5", ContextWindow: 32_768},
	"qwen2.5-coder": {Name: "qwen2.5-coder", ContextWindow: 32_768},
	"mistral":       {Name: "mistral", ContextWindow: 32_768},
	"mixtral":       {Name: "mixtral", ContextWindow: 32_768},
	"codellama":     {Name: "codellama", ContextWindow: 16_384},
}

const defaultContextWindow = 8_192

// GetModelMetadata retorna os metadados de um modelo conhecido, ou um fallback com context window padrão.
// O nome exato vale primeiro (os IDs do Bedrock têm ":" no meio); sem ele, o nome base
// antes da tag do Ollama ("llama3.1:8b" é o llama3.1). Name continua o nome pedido.
func GetModelMetadata(model string) ModelMetadata {
	if meta, ok := knownModels[model]; ok {
		return meta
	}
	if base, _, ok := strings.Cut(model, ":"); ok {
		if meta, ok := knownModels[base]; ok {
			return ModelMetadata{Name: model, ContextWindow: meta.ContextWindow}
		}
	}
	return ModelMetadata{Name: model, ContextWindow: defaultContextWindow}
}

// Janelas de contexto dos modelos que os providers CLI aceitam (models.go): os do Claude
// (sonnet, opus, haiku e claude-*) e os atuais do Gemini.
const (
	janelaClaude = 200_000
	janelaGemini = 1_000_000
)

// metadadosCLI devolve a janela de contexto de um provider CLI. Sem modelo o próprio CLI
// escolhe, e qualquer modelo que ele escolha tem pelo menos essa janela; o padrão de 8192
// do GetModelMetadata cortaria os prompts que o CLI aguenta.
func metadadosCLI(provider, model string) ModelMetadata {
	if meta, ok := knownModels[model]; ok {
		return meta
	}
	nome := model
	if nome == "" {
		nome = "padrão do " + provider
	}
	switch provider {
	case "claude-cli":
		return ModelMetadata{Name: nome, ContextWindow: janelaClaude}
	case "gemini-cli":
		return ModelMetadata{Name: nome, ContextWindow: janelaGemini}
	}
	return GetModelMetadata(model)
}

// TokenAwareProvider é um decorator que valida o tamanho do input antes de chamar o provider.
// Se o texto exceder 90% da context window, retorna erro. Se exceder 80%, emite warning.
type TokenAwareProvider struct {
	inner Provider
	model string
	meta  ModelMetadata // a janela usada na conta
}

// NewTokenAwareProvider cria um TokenAwareProvider que envolve o provider informado.
func NewTokenAwareProvider(inner Provider, model string) *TokenAwareProvider {
	return &TokenAwareProvider{inner: inner, model: model, meta: GetModelMetadata(model)}
}

// newTokenAwareProviderCLI cria o TokenAwareProvider de um provider CLI, com a janela do CLI.
func newTokenAwareProviderCLI(inner Provider, provider, model string) *TokenAwareProvider {
	return &TokenAwareProvider{inner: inner, model: model, meta: metadadosCLI(provider, model)}
}

// Unwrap devolve o provider embrulhado.
func (t *TokenAwareProvider) Unwrap() Provider { return t.inner }

func (t *TokenAwareProvider) Name() string                         { return t.inner.Name() }
func (t *TokenAwareProvider) IsAvailable(ctx context.Context) bool { return t.inner.IsAvailable(ctx) }

func (t *TokenAwareProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if err := t.checkTokens(systemPrompt + userPrompt); err != nil {
		return "", err
	}
	return t.inner.Completion(ctx, systemPrompt, userPrompt)
}

func (t *TokenAwareProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	if err := t.checkTokens(systemPrompt + userPrompt); err != nil {
		return err
	}
	return t.inner.StreamCompletion(ctx, systemPrompt, userPrompt, out)
}

func (t *TokenAwareProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	if err := t.checkTokens(description); err != nil {
		return nil, err
	}
	return t.inner.GenerateGovernance(ctx, description)
}

func (t *TokenAwareProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	return t.inner.EmbedDocuments(ctx, texts)
}

// checkTokens valida o tamanho estimado do input contra a context window do modelo.
func (t *TokenAwareProvider) checkTokens(text string) error {
	meta := t.meta
	tokens := EstimateTokens(text)
	ratio := float64(tokens) / float64(meta.ContextWindow)

	if ratio > 0.9 {
		return errors.New(errors.ErrCodeTokenLimit,
			fmt.Sprintf("Input estimado em ~%d tokens excede 90%% da context window do modelo %s (%d tokens)",
				tokens, meta.Name, meta.ContextWindow)).
			WithHint("Reduza o tamanho do input ou use um modelo com context window maior.")
	}

	if ratio > 0.8 {
		slog.Warn("Input próximo do limite da context window",
			"tokens_estimados", tokens,
			"limite", meta.ContextWindow,
			"modelo", meta.Name,
			"uso_pct", fmt.Sprintf("%.0f%%", ratio*100),
		)
	}

	return nil
}
