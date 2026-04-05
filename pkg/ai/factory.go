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

// GetProvider returns the requested AI provider or defaults to the best available.
// preferred: "ollama", "gemini", "openai"
func GetProvider(ctx context.Context, preferred string) Provider {
	// 1. Explicit Preference (Argument or Env Var)
	target := preferred
	if target == "" || target == "auto" {
		if env := os.Getenv("YBY_AI_PROVIDER"); env != "" {
			target = strings.ToLower(env)
		}
	}

	if target != "" && target != "auto" {
		switch target {
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
		}
		// Se a preferencia explicita nao esta disponivel, retorna nil (Strict)
		return nil
	}

	// 2. Auto-Detect: Preferir inferencia local (privacidade e custo)
	ollama := NewOllamaProvider()
	if ollama.IsAvailable(ctx) {
		return wrapProvider(ollama, ollama.Model)
	}

	// 3. Google Gemini (rapido e tier gratuito generoso)
	gemini := NewGeminiProvider()
	if gemini != nil && gemini.IsAvailable(ctx) {
		return wrapProvider(gemini, gemini.Model)
	}

	// 4. OpenAI (padrao)
	openai := NewOpenAIProvider()
	if openai != nil && openai.IsAvailable(ctx) {
		return wrapProvider(openai, openai.Model)
	}

	return nil
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
