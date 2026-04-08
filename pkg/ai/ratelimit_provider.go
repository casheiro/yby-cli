package ai

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Taxas padrão de requisições por segundo por provider.
const (
	defaultOpenAIRate  = 50.0
	defaultGeminiRate  = 60.0
	defaultOllamaRate  = 0.0 // sem limite
	defaultBedrockRate = 10.0
)

// ─── Circuit Breaker ──────────────────────────────────────────────────────

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

const (
	circuitBreakerThreshold = 5
	circuitBreakerCooldown  = 30 * time.Second
)

// CircuitBreaker implementa um circuit breaker simples com estados closed/open/half-open.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	consecutiveFails int
	lastFailTime     time.Time
}

func newCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{state: circuitClosed}
}

// allow verifica se o circuit breaker permite uma requisição.
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(cb.lastFailTime) >= circuitBreakerCooldown {
			cb.state = circuitHalfOpen
			slog.Info("circuit breaker: transição para half-open")
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	}
	return false
}

// recordSuccess registra uma requisição bem-sucedida.
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	if cb.state == circuitHalfOpen {
		cb.state = circuitClosed
		slog.Info("circuit breaker: fechado após sucesso em half-open")
	}
}

// recordFailure registra uma falha. Retorna true se o circuito abriu.
func (cb *CircuitBreaker) recordFailure(statusCode int) bool {
	if statusCode < 500 {
		return false
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailTime = time.Now()

	if cb.state == circuitHalfOpen {
		cb.state = circuitOpen
		slog.Warn("circuit breaker: reaberto após falha em half-open", "erros_consecutivos", cb.consecutiveFails)
		return true
	}

	if cb.consecutiveFails >= circuitBreakerThreshold {
		cb.state = circuitOpen
		slog.Warn("circuit breaker: aberto", "erros_consecutivos", cb.consecutiveFails)
		return true
	}

	return false
}

// ─── Rate Limit Provider ──────────────────────────────────────────────────────

// RateLimitProvider é um decorator que implementa Provider com rate limiting
// via token bucket e circuit breaker embutido.
type RateLimitProvider struct {
	inner   Provider
	limiter *rate.Limiter
	cb      *CircuitBreaker
}

// NewRateLimitProvider cria um RateLimitProvider que envolve o provider informado.
// rps define o limite de requisições por segundo. Se rps <= 0, não aplica rate limiting.
func NewRateLimitProvider(inner Provider, rps float64) *RateLimitProvider {
	var limiter *rate.Limiter
	if rps > 0 {
		limiter = rate.NewLimiter(rate.Limit(rps), int(rps))
	}
	return &RateLimitProvider{
		inner:   inner,
		limiter: limiter,
		cb:      newCircuitBreaker(),
	}
}

func (r *RateLimitProvider) Name() string                         { return r.inner.Name() }
func (r *RateLimitProvider) IsAvailable(ctx context.Context) bool { return r.inner.IsAvailable(ctx) }

func (r *RateLimitProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if err := r.waitAndCheck(ctx); err != nil {
		return "", err
	}
	result, err := r.inner.Completion(ctx, systemPrompt, userPrompt)
	r.recordResult(err)
	return result, err
}

func (r *RateLimitProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	if err := r.waitAndCheck(ctx); err != nil {
		return err
	}
	err := r.inner.StreamCompletion(ctx, systemPrompt, userPrompt, out)
	r.recordResult(err)
	return err
}

func (r *RateLimitProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if err := r.waitAndCheck(ctx); err != nil {
		return nil, err
	}
	result, err := r.inner.EmbedDocuments(ctx, texts)
	r.recordResult(err)
	return result, err
}

func (r *RateLimitProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	if err := r.waitAndCheck(ctx); err != nil {
		return nil, err
	}
	result, err := r.inner.GenerateGovernance(ctx, description)
	r.recordResult(err)
	return result, err
}

// waitAndCheck aguarda o rate limiter e verifica o circuit breaker.
func (r *RateLimitProvider) waitAndCheck(ctx context.Context) error {
	if !r.cb.allow() {
		return &APIError{
			Provider:   r.inner.Name(),
			StatusCode: 503,
			Body:       "circuit breaker aberto: muitas falhas consecutivas do servidor",
		}
	}

	if r.limiter != nil {
		if err := r.limiter.Wait(ctx); err != nil {
			return err
		}
	}

	return nil
}

// recordResult registra o resultado de uma operação no circuit breaker.
// Se receber 429 com RetryAfter, faz sleep do RetryAfter.
func (r *RateLimitProvider) recordResult(err error) {
	if err == nil {
		r.cb.recordSuccess()
		return
	}

	if apiErr, ok := err.(*APIError); ok {
		r.cb.recordFailure(apiErr.StatusCode)

		if apiErr.StatusCode == 429 && apiErr.RetryAfter > 0 {
			slog.Info("rate limit atingido, aguardando retry-after",
				"provider", r.inner.Name(),
				"retry_after", apiErr.RetryAfter,
			)
			time.Sleep(apiErr.RetryAfter)
		}
	}
}

// getDefaultRateForProvider retorna a taxa padrão de req/s para o provider.
func getDefaultRateForProvider(providerName string) float64 {
	switch {
	case contains(providerName, "OpenAI"):
		return defaultOpenAIRate
	case contains(providerName, "Gemini"):
		return defaultGeminiRate
	case contains(providerName, "Ollama"):
		return defaultOllamaRate
	case contains(providerName, "Bedrock"):
		return defaultBedrockRate
	default:
		return 0
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
