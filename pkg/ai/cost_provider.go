package ai

import (
	"context"
	"io"
	"log/slog"
)

// modelPricing armazena preços por 1M tokens (USD).
type modelPricing struct {
	InputPer1M  float64
	OutputPer1M float64
}

// knownPricing tabela de preços conhecidos por modelo.
var knownPricing = map[string]modelPricing{
	// OpenAI
	"gpt-4o-mini":   {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4o":        {InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4-turbo":   {InputPer1M: 10.00, OutputPer1M: 30.00},
	"gpt-4":         {InputPer1M: 30.00, OutputPer1M: 60.00},
	"gpt-3.5-turbo": {InputPer1M: 0.50, OutputPer1M: 1.50},

	// Google Gemini
	"gemini-2.5-flash": {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gemini-2.5-pro":   {InputPer1M: 1.25, OutputPer1M: 10.00},
	"gemini-2.0-flash": {InputPer1M: 0.10, OutputPer1M: 0.40},
	"gemini-1.5-flash": {InputPer1M: 0.075, OutputPer1M: 0.30},
	"gemini-1.5-pro":   {InputPer1M: 1.25, OutputPer1M: 5.00},

	// Ollama (local, sem custo monetário)
	"llama3":    {InputPer1M: 0, OutputPer1M: 0},
	"llama3.1":  {InputPer1M: 0, OutputPer1M: 0},
	"mistral":   {InputPer1M: 0, OutputPer1M: 0},
	"mixtral":   {InputPer1M: 0, OutputPer1M: 0},
	"codellama": {InputPer1M: 0, OutputPer1M: 0},
}

// CostTrackingProvider é um decorator que intercepta chamadas ao provider
// e loga informações de uso de tokens e custo estimado.
type CostTrackingProvider struct {
	inner Provider
	model string
}

// NewCostTrackingProvider cria um CostTrackingProvider que envolve o provider informado.
func NewCostTrackingProvider(inner Provider, model string) *CostTrackingProvider {
	return &CostTrackingProvider{inner: inner, model: model}
}

func (c *CostTrackingProvider) Name() string                         { return c.inner.Name() }
func (c *CostTrackingProvider) IsAvailable(ctx context.Context) bool { return c.inner.IsAvailable(ctx) }

func (c *CostTrackingProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := c.inner.Completion(ctx, systemPrompt, userPrompt)
	c.logUsage(ctx, "completion")
	return result, err
}

func (c *CostTrackingProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	err := c.inner.StreamCompletion(ctx, systemPrompt, userPrompt, out)
	c.logUsage(ctx, "streaming")
	return err
}

func (c *CostTrackingProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	result, err := c.inner.EmbedDocuments(ctx, texts)
	c.logUsage(ctx, "embedding")
	return result, err
}

func (c *CostTrackingProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	result, err := c.inner.GenerateGovernance(ctx, description)
	c.logUsage(ctx, "governance")
	return result, err
}

// logUsage loga informações de uso se disponíveis no contexto.
func (c *CostTrackingProvider) logUsage(ctx context.Context, operation string) {
	usage := GetUsage(ctx)
	if usage == nil {
		return
	}

	cost := c.estimateCost(usage)

	slog.Info("ai.usage",
		"provider", usage.Provider,
		"model", usage.Model,
		"operation", operation,
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"total_tokens", usage.TotalTokens,
		"estimated_cost_usd", cost,
	)
}

// estimateCost calcula o custo estimado baseado na tabela de preços.
func (c *CostTrackingProvider) estimateCost(usage *UsageMetadata) float64 {
	pricing, ok := knownPricing[c.model]
	if !ok {
		return 0
	}

	inputCost := float64(usage.PromptTokens) / 1_000_000 * pricing.InputPer1M
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * pricing.OutputPer1M

	return inputCost + outputCost
}
