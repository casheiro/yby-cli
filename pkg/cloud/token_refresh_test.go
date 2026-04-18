package cloud

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTokenGenerator é um gerador de tokens para testes.
type mockTokenGenerator struct {
	token     *Token
	err       error
	callCount atomic.Int64
}

func (m *mockTokenGenerator) GenerateToken(_ context.Context) (*Token, error) {
	m.callCount.Add(1)
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}

// mockRoundTripper é um RoundTripper para testes que retorna respostas configuráveis.
type mockRoundTripper struct {
	responses []*http.Response
	mu        sync.Mutex
	callIdx   int
	requests  []*http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clonar headers para captura
	cloned := req.Clone(req.Context())
	m.requests = append(m.requests, cloned)

	idx := m.callIdx
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.callIdx++

	return m.responses[idx], nil
}

func TestAutoRefreshTransport_InjectsToken(t *testing.T) {
	gen := &mockTokenGenerator{
		token: &Token{Value: "meu-token-123", ExpiresAt: time.Now().Add(5 * time.Minute)},
	}
	cache := &TokenCache{}

	rt := &mockRoundTripper{
		responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
		},
	}

	transport := &AutoRefreshTransport{
		Base:      rt,
		Generator: gen,
		Cache:     cache,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://k8s.example.com/api/v1/pods", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip inesperado: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	if len(rt.requests) != 1 {
		t.Fatalf("esperado 1 request, obteve %d", len(rt.requests))
	}

	authHeader := rt.requests[0].Header.Get("Authorization")
	expected := "Bearer meu-token-123"
	if authHeader != expected {
		t.Errorf("Authorization = %q, esperado %q", authHeader, expected)
	}
}

func TestAutoRefreshTransport_RefreshOn401(t *testing.T) {
	tokenOriginal := &Token{Value: "token-expirado", ExpiresAt: time.Now().Add(-1 * time.Second)}
	tokenNovo := &Token{Value: "token-novo", ExpiresAt: time.Now().Add(5 * time.Minute)}

	gen := &mockTokenGenerator{token: tokenNovo}
	cache := &TokenCache{}

	rt := &mockRoundTripper{
		responses: []*http.Response{
			{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("unauthorized"))},
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
		},
	}

	transport := &AutoRefreshTransport{
		Base:      rt,
		Generator: gen,
		Cache:     cache,
	}

	// Setar token expirado diretamente no cache (vai falhar no Get por causa da margem)
	cache.Set(tokenOriginal)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://k8s.example.com/api/v1/pods", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip inesperado: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, esperado %d", resp.StatusCode, http.StatusOK)
	}

	// Deve ter feito 2 requests: primeiro com token gerado (cache miss), segundo após refresh
	if len(rt.requests) != 2 {
		t.Fatalf("esperado 2 requests, obteve %d", len(rt.requests))
	}

	// Segundo request deve ter o token novo
	authHeader := rt.requests[1].Header.Get("Authorization")
	expected := "Bearer token-novo"
	if authHeader != expected {
		t.Errorf("Authorization retry = %q, esperado %q", authHeader, expected)
	}
}

func TestAutoRefreshTransport_NoRetryOn403(t *testing.T) {
	gen := &mockTokenGenerator{
		token: &Token{Value: "meu-token", ExpiresAt: time.Now().Add(5 * time.Minute)},
	}
	cache := &TokenCache{}

	rt := &mockRoundTripper{
		responses: []*http.Response{
			{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("forbidden"))},
		},
	}

	transport := &AutoRefreshTransport{
		Base:      rt,
		Generator: gen,
		Cache:     cache,
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://k8s.example.com/api/v1/secrets", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip inesperado: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, esperado %d", resp.StatusCode, http.StatusForbidden)
	}

	// Deve ter feito apenas 1 request (sem retry)
	if len(rt.requests) != 1 {
		t.Errorf("esperado 1 request (sem retry), obteve %d", len(rt.requests))
	}

	// Generator deve ter sido chamado apenas 1 vez (para obter token inicial)
	if gen.callCount.Load() != 1 {
		t.Errorf("GenerateToken chamado %d vezes, esperado 1", gen.callCount.Load())
	}
}

func TestAutoRefreshTransport_ConcurrentRefresh(t *testing.T) {
	gen := &mockTokenGenerator{
		token: &Token{Value: "token-concorrente", ExpiresAt: time.Now().Add(5 * time.Minute)},
	}
	cache := &TokenCache{}

	// Todas as respostas retornam 401 na primeira vez, 200 na segunda
	rt := &concurrentMockRT{
		firstResponse: &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("unauthorized"))},
		retryResponse: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
	}

	transport := &AutoRefreshTransport{
		Base:      rt,
		Generator: gen,
		Cache:     cache,
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://k8s.example.com/api/v1/pods", nil)
			_, _ = transport.RoundTrip(req)
		}()
	}

	wg.Wait()

	// O mutex no refreshToken garante que apenas refresh calls são serializados.
	// Porém múltiplos goroutines podem ver cache miss inicialmente e depois o mutex
	// garante double-check. No pior caso, temos 1 geração inicial + 1 refresh por
	// goroutine que recebe 401, mas o double-check no mutex reduz significativamente.
	// O importante é que NÃO haja race condition.
	calls := gen.callCount.Load()
	t.Logf("GenerateToken chamado %d vezes com %d goroutines concorrentes", calls, numGoroutines)

	// Deve ser MUITO menor que numGoroutines graças ao double-check locking
	if calls > 10 {
		t.Errorf("GenerateToken chamado %d vezes, esperado <=10 (double-check locking deve reduzir chamadas)", calls)
	}
}

// concurrentMockRT retorna 401 na primeira chamada de cada goroutine,
// e 200 no retry, de forma thread-safe.
type concurrentMockRT struct {
	firstResponse *http.Response
	retryResponse *http.Response
	mu            sync.Mutex
	callCount     map[string]int
}

func (m *concurrentMockRT) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	if m.callCount == nil {
		m.callCount = make(map[string]int)
	}
	// Usar o header Authorization para distinguir: se já tem um token "renovado", retorna 200
	auth := req.Header.Get("Authorization")
	m.callCount[auth]++
	count := m.callCount[auth]
	m.mu.Unlock()

	// Primeira chamada com qualquer token retorna 401, retry retorna 200
	if count <= 1 && auth != "" {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("unauthorized")),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}
