package ai

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitProvider_Completion_Sucesso(t *testing.T) {
	inner := &mockProvider{
		name:      "test",
		available: true,
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			return "ok", nil
		},
	}
	rl := NewRateLimitProvider(inner, 100) // alto para não bloquear

	result, err := rl.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestRateLimitProvider_SemLimite(t *testing.T) {
	inner := &mockProvider{
		name:      "test",
		available: true,
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			return "ok", nil
		},
	}
	rl := NewRateLimitProvider(inner, 0) // sem limite

	result, err := rl.Completion(context.Background(), "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestRateLimitProvider_Name(t *testing.T) {
	inner := &mockProvider{name: "meu-provider"}
	rl := NewRateLimitProvider(inner, 10)
	assert.Equal(t, "meu-provider", rl.Name())
}

func TestRateLimitProvider_IsAvailable(t *testing.T) {
	inner := &mockProvider{name: "test", available: true}
	rl := NewRateLimitProvider(inner, 10)
	assert.True(t, rl.IsAvailable(context.Background()))
}

// ─── Testes do Circuit Breaker ─────────────────────────────────────────────

func TestCircuitBreaker_AbreApos5Falhas(t *testing.T) {
	cb := newCircuitBreaker()

	// 5 falhas 5xx consecutivas
	for i := 0; i < 5; i++ {
		cb.recordFailure(500)
	}

	assert.False(t, cb.allow(), "circuito deve estar aberto após 5 falhas")
}

func TestCircuitBreaker_NaoAbreComErros4xx(t *testing.T) {
	cb := newCircuitBreaker()

	for i := 0; i < 10; i++ {
		cb.recordFailure(429)
	}

	assert.True(t, cb.allow(), "erros 4xx não devem abrir o circuito")
}

func TestCircuitBreaker_ResetAposSucesso(t *testing.T) {
	cb := newCircuitBreaker()

	// 4 falhas, depois sucesso
	for i := 0; i < 4; i++ {
		cb.recordFailure(500)
	}
	cb.recordSuccess()

	// Mais 4 falhas (não deve abrir pois resetou)
	for i := 0; i < 4; i++ {
		cb.recordFailure(500)
	}
	assert.True(t, cb.allow(), "circuito deve estar fechado pois sucesso resetou contador")
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cb := newCircuitBreaker()

	// Abrir o circuito
	for i := 0; i < 5; i++ {
		cb.recordFailure(500)
	}
	assert.False(t, cb.allow())

	// Simular passagem do cooldown
	cb.mu.Lock()
	cb.lastFailTime = time.Now().Add(-circuitBreakerCooldown - time.Second)
	cb.mu.Unlock()

	// Deve permitir (half-open)
	assert.True(t, cb.allow(), "deve permitir após cooldown (half-open)")

	// Sucesso em half-open fecha o circuito
	cb.recordSuccess()
	assert.True(t, cb.allow(), "circuito deve estar fechado após sucesso em half-open")
}

func TestCircuitBreaker_HalfOpen_FalhaReabre(t *testing.T) {
	cb := newCircuitBreaker()

	// Abrir o circuito
	for i := 0; i < 5; i++ {
		cb.recordFailure(500)
	}

	// Simular passagem do cooldown
	cb.mu.Lock()
	cb.lastFailTime = time.Now().Add(-circuitBreakerCooldown - time.Second)
	cb.mu.Unlock()

	// Transicionar para half-open
	assert.True(t, cb.allow())

	// Falha em half-open deve reabrir
	cb.recordFailure(500)
	assert.False(t, cb.allow(), "circuito deve reabrir após falha em half-open")
}

func TestRateLimitProvider_CircuitBreakerBloqueia(t *testing.T) {
	var calls atomic.Int32
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			calls.Add(1)
			return "", &APIError{Provider: "test", StatusCode: 500, Body: "erro interno"}
		},
	}
	rl := NewRateLimitProvider(inner, 0)

	// 5 chamadas com erro 500 abrem o circuito
	for i := 0; i < 5; i++ {
		_, _ = rl.Completion(context.Background(), "sys", "usr")
	}

	// Próxima chamada deve ser bloqueada pelo circuit breaker
	_, err := rl.Completion(context.Background(), "sys", "usr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
	assert.Equal(t, int32(5), calls.Load(), "inner não deve ser chamado quando circuito está aberto")
}

func TestRateLimitProvider_RetryAfterSleep(t *testing.T) {
	inner := &mockProvider{
		name: "test",
		completionFunc: func(_ context.Context, _, _ string) (string, error) {
			return "", &APIError{
				Provider:   "test",
				StatusCode: 429,
				Body:       "rate limited",
				RetryAfter: 10 * time.Millisecond,
			}
		},
	}
	rl := NewRateLimitProvider(inner, 0)

	start := time.Now()
	_, _ = rl.Completion(context.Background(), "sys", "usr")
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10),
		"deve ter aguardado pelo menos o retry-after")
}

func TestGetDefaultRateForProvider(t *testing.T) {
	assert.InDelta(t, defaultOpenAIRate, getDefaultRateForProvider("OpenAI (Cloud)"), 0.001)
	assert.InDelta(t, defaultGeminiRate, getDefaultRateForProvider("Google Gemini (Cloud)"), 0.001)
	assert.InDelta(t, defaultOllamaRate, getDefaultRateForProvider("Ollama (Local)"), 0.001)
	assert.InDelta(t, 0.0, getDefaultRateForProvider("desconhecido"), 0.001)
}
