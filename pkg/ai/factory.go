package ai

import (
	"context"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/retry"
)

// GetLanguage returns the configured AI language or defaults to pt-BR
func GetLanguage() string {
	if lang := os.Getenv("YBY_AI_LANGUAGE"); lang != "" {
		return lang
	}
	return "pt-BR"
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
				return wrapWithRetry(p)
			}
		case "gemini":
			p := NewGeminiProvider()
			if p != nil && p.IsAvailable(ctx) {
				return wrapWithRetry(p)
			}
		case "openai":
			p := NewOpenAIProvider()
			if p != nil && p.IsAvailable(ctx) {
				return wrapWithRetry(p)
			}
		}
		// Se a preferencia explicita nao esta disponivel, retorna nil (Strict)
		return nil
	}

	// 2. Auto-Detect: Preferir inferencia local (privacidade e custo)
	ollama := NewOllamaProvider()
	if ollama.IsAvailable(ctx) {
		return wrapWithRetry(ollama)
	}

	// 3. Google Gemini (rapido e tier gratuito generoso)
	gemini := NewGeminiProvider()
	if gemini != nil && gemini.IsAvailable(ctx) {
		return wrapWithRetry(gemini)
	}

	// 4. OpenAI (padrao)
	openai := NewOpenAIProvider()
	if openai != nil && openai.IsAvailable(ctx) {
		return wrapWithRetry(openai)
	}

	return nil
}

// wrapWithRetry envolve um Provider com retry automatico via backoff exponencial.
func wrapWithRetry(p Provider) Provider {
	if p == nil {
		return nil
	}
	return NewRetryProvider(p, retry.DefaultOptions(), nil)
}
