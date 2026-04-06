package ai

import (
	"context"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/retry"
)

// GetLanguage retorna o idioma configurado para IA.
// Usa config global que já aplica precedência: env > arquivo > default (pt-BR).
func GetLanguage() string {
	cfg, err := config.Load()
	if err != nil || cfg.AI.Language == "" {
		if lang := os.Getenv("YBY_AI_LANGUAGE"); lang != "" {
			return lang
		}
		return "pt-BR"
	}
	return cfg.AI.Language
}

// getConfiguredModel retorna o modelo configurado globalmente, ou vazio se não definido.
func getConfiguredModel() string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	return cfg.AI.Model
}

// defaultPriority define a ordem padrão de tentativa dos providers.
// Ollama primeiro (local, sem custo), CLIs depois (modelo potente, auth resolvida),
// APIs por último (dependem de key + rede).
var defaultPriority = []string{
	"ollama",
	"claude-cli",
	"gemini-cli",
	"gemini",
	"openai",
}

// getProviderPriority retorna a ordem de prioridade configurada pelo usuário,
// ou a ordem padrão se não configurada.
func getProviderPriority() []string {
	cfg, err := config.Load()
	if err == nil && len(cfg.AI.Priority) > 0 {
		return cfg.AI.Priority
	}
	return defaultPriority
}

// createProvider cria e retorna um provider pelo nome, se disponível.
func createProvider(ctx context.Context, name string) Provider {
	switch name {
	case "ollama":
		p := NewOllamaProvider()
		if p.IsAvailable(ctx) {
			return wrapProvider(p, p.Model)
		}
	case "gemini":
		p := NewGeminiProvider()
		if p != nil && p.IsAvailable(ctx) {
			return wrapProvider(p, p.Model)
		}
	case "openai":
		p := NewOpenAIProvider()
		if p != nil && p.IsAvailable(ctx) {
			return wrapProvider(p, p.Model)
		}
	case "claude-cli":
		p := NewClaudeCLIProvider()
		if p.IsAvailable(ctx) {
			return p
		}
	case "gemini-cli":
		p := NewGeminiCLIProvider()
		if p.IsAvailable(ctx) {
			return p
		}
	}
	return nil
}

// GetProvider retorna o primeiro provider disponível, respeitando a ordem de prioridade.
// Se preferred for especificado (não "auto" nem ""), tenta apenas esse provider.
func GetProvider(ctx context.Context, preferred string) Provider {
	// 1. Preferência explícita via argumento ou env var
	target := preferred
	if target == "" || target == "auto" {
		if env := os.Getenv("YBY_AI_PROVIDER"); env != "" {
			target = strings.ToLower(env)
		}
	}

	if target != "" && target != "auto" {
		return createProvider(ctx, target)
	}

	// 2. Seguir ordem de prioridade configurada
	for _, name := range getProviderPriority() {
		if p := createProvider(ctx, name); p != nil {
			return p
		}
	}

	return nil
}

// GetAllAvailableProviders retorna todos os providers disponíveis na ordem de prioridade.
// Usado para cascata: tentar um, se falhar tentar o próximo.
func GetAllAvailableProviders(ctx context.Context) []Provider {
	var providers []Provider
	for _, name := range getProviderPriority() {
		if p := createProvider(ctx, name); p != nil {
			providers = append(providers, p)
		}
	}
	return providers
}

// wrapProvider encadeia os decorators na ordem:
// Raw -> CachedEmbedding -> TokenAware -> CostTracking -> RateLimit -> Retry.
func wrapProvider(p Provider, model string) Provider {
	if p == nil {
		return nil
	}
	cached := NewCachedEmbeddingProvider(p, defaultEmbeddingCacheSize, defaultEmbeddingCacheTTL)
	tokenAware := NewTokenAwareProvider(cached, model)
	costTracking := NewCostTrackingProvider(tokenAware, model)
	rps := getRateLimitConfig(p.Name())
	rateLimited := NewRateLimitProvider(costTracking, rps)
	return NewRetryProvider(rateLimited, retry.DefaultOptions(), nil)
}

// getRateLimitConfig retorna a taxa de req/s para o provider,
// priorizando configuração do usuário sobre o default.
func getRateLimitConfig(providerName string) float64 {
	cfg, err := config.Load()
	if err == nil && cfg.AI.RateLimit.RequestsPerSecond > 0 {
		return cfg.AI.RateLimit.RequestsPerSecond
	}
	return getDefaultRateForProvider(providerName)
}
