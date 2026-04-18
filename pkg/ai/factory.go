package ai

import (
	"context"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/retry"
)

// providerFactory é uma função que cria um provider dado um contexto.
type providerFactory func(ctx context.Context) Provider

// registeredProviders armazena factories de providers registrados via init() (ex: build tags).
var registeredProviders = map[string]providerFactory{}

// registerProvider registra uma factory de provider pelo nome.
// Usado por arquivos com build tags para registrar providers condicionais.
func registerProvider(name string, factory providerFactory) {
	registeredProviders[name] = factory
}

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

// getConfiguredModel retorna o modelo configurado, ou vazio se não definido.
// Mantém backward compat — chamado sem args ou com provider name.
func getConfiguredModel(providerNames ...string) string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	// Tentar modelo específico do provider
	if len(providerNames) > 0 && cfg.AI.Models != nil {
		if model, ok := cfg.AI.Models[providerNames[0]]; ok && model != "" {
			return model
		}
	}
	// Fallback: modelo global
	return cfg.AI.Model
}

// GetEmbeddingModel retorna o modelo de embedding configurado para um provider específico.
// Se não configurado, retorna vazio (o provider usa seu default).
// Configuração em ~/.yby/config.yaml:
//
//	ai:
//	  embedding:
//	    ollama: nomic-embed-text
//	    gemini: gemini-embedding-001
//	    openai: text-embedding-3-small
func GetEmbeddingModel(providerName string) string {
	cfg, err := config.Load()
	if err != nil || cfg.AI.Embedding == nil {
		return ""
	}
	return cfg.AI.Embedding[providerName]
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
	"bedrock",
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
	default:
		// Verificar providers registrados via build tags (ex: bedrock com tag aws)
		if factory, ok := registeredProviders[name]; ok {
			return factory(ctx)
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

// embeddingCapableProviders lista os providers que suportam EmbedDocuments.
// CLIs (claude-cli, gemini-cli) não suportam embeddings.
var embeddingCapableProviders = map[string]bool{
	"ollama":  true,
	"gemini":  true,
	"openai":  true,
	"bedrock": true,
}

// GetEmbeddingProvider retorna o provider de embeddings mais adequado.
// Prioridade: Ollama (suporta modelos grandes, sem rate limit) > config explícito > local.
func GetEmbeddingProvider(ctx context.Context) Provider {
	// 1. Priorizar Ollama se disponível (suporta modelos grandes como nomic-embed-text, 8192 tokens)
	ollama := NewOllamaProvider()
	if ollama.IsAvailable(ctx) {
		// Forçar modelo de embedding se configurado
		if embModel := GetEmbeddingModel("ollama"); embModel != "" {
			ollama.Model = embModel
			ollama.modelConfigured = true
		}
		return wrapProvider(ollama, ollama.Model)
	}

	// 2. Se usuário configurou embedding explícito pra outro provider (gemini, openai)
	cfg, err := config.Load()
	if err == nil && cfg.AI.Embedding != nil {
		for _, name := range getProviderPriority() {
			if !embeddingCapableProviders[name] || name == "ollama" {
				continue
			}
			if _, hasConfig := cfg.AI.Embedding[name]; hasConfig {
				if p := createProvider(ctx, name); p != nil {
					return p
				}
			}
		}
	}

	// 3. Fallback: embeddings locais (all-MiniLM-L6-v2, 512 tokens com truncagem)
	return NewLocalEmbeddingProvider()
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
