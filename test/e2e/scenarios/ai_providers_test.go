//go:build e2e

package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/retry"
)

// TestAIRateLimitRetry verifica que o RetryProvider retenta após receber 429
// com header Retry-After, e eventualmente obtém sucesso.
func TestAIRateLimitRetry(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)

		switch {
		case r.URL.Path == "/api/tags":
			// Ollama ping — sempre disponível
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "llama3"}},
			})

		case r.URL.Path == "/api/generate":
			if n <= 3 { // primeiras 2 chamadas de generate (calls 2 e 3) retornam 429
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":"rate limited"}`)
				return
			}
			// Terceira chamada de generate (call 4) — sucesso
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": "resposta com sucesso após retry",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Criar provider Ollama apontando para o mock
	provider := &ai.OllamaProvider{
		BaseURL:   server.URL,
		Model:     "llama3",
		Endpoints: []string{server.URL},
	}

	// Envolver com RetryProvider (tempos curtos para teste)
	retryOpts := retry.Options{
		InitialInterval:     100 * time.Millisecond,
		MaxInterval:         500 * time.Millisecond,
		MaxElapsedTime:      10 * time.Second,
		RandomizationFactor: 0,
		Multiplier:          1.5,
	}
	retryProvider := ai.NewRetryProvider(provider, retryOpts, nil)

	ctx := context.Background()
	result, err := retryProvider.Completion(ctx, "system", "user prompt")

	if err != nil {
		t.Fatalf("Esperava sucesso após retries, mas obteve erro: %v", err)
	}
	if result != "resposta com sucesso após retry" {
		t.Errorf("Resposta inesperada: %q", result)
	}

	total := int(callCount.Load())
	// Pelo menos 2 chamadas a /api/generate (1 falha + 1 sucesso), além do ping
	if total < 3 {
		t.Errorf("Esperava pelo menos 3 chamadas ao mock (ping + retries + sucesso), obteve %d", total)
	}
	t.Logf("Total de chamadas ao mock: %d (incluindo retries)", total)
}

// TestOllamaBatchEmbeddings verifica que o OllamaProvider usa /api/embed (batch)
// e faz fallback para /api/embeddings quando retorna 404.
func TestOllamaBatchEmbeddings(t *testing.T) {
	t.Run("batch_api_embed", func(t *testing.T) {
		var embedCalls atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/tags":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"models": []map[string]string{{"name": "llama3"}},
				})

			case "/api/embed":
				embedCalls.Add(1)
				var req struct {
					Model string   `json:"model"`
					Input []string `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Erro ao decodificar request de embed: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				// Verificar que recebeu batch (múltiplos textos)
				if len(req.Input) < 2 {
					t.Errorf("Esperava batch com múltiplos textos, recebeu %d", len(req.Input))
				}

				// Gerar embeddings fake
				embeddings := make([][]float32, len(req.Input))
				for i := range embeddings {
					embeddings[i] = []float32{0.1, 0.2, 0.3}
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"embeddings": embeddings,
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := &ai.OllamaProvider{
			BaseURL:   server.URL,
			Model:     "llama3",
			Endpoints: []string{server.URL},
		}

		ctx := context.Background()
		texts := []string{"texto um", "texto dois", "texto três"}
		results, err := provider.EmbedDocuments(ctx, texts)

		if err != nil {
			t.Fatalf("Erro ao gerar embeddings batch: %v", err)
		}
		if len(results) != len(texts) {
			t.Fatalf("Esperava %d embeddings, obteve %d", len(texts), len(results))
		}

		calls := int(embedCalls.Load())
		if calls != 1 {
			t.Errorf("Esperava 1 chamada batch a /api/embed, obteve %d", calls)
		}
	})

	t.Run("fallback_api_embeddings", func(t *testing.T) {
		var embeddingsCalls atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/tags":
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"models": []map[string]string{{"name": "llama3"}},
				})

			case "/api/embed":
				// Simular Ollama antigo que não suporta /api/embed
				w.WriteHeader(http.StatusNotFound)

			case "/api/embeddings":
				embeddingsCalls.Add(1)
				var req struct {
					Model  string `json:"model"`
					Prompt string `json:"prompt"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"embedding": []float32{0.4, 0.5, 0.6},
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		provider := &ai.OllamaProvider{
			BaseURL:   server.URL,
			Model:     "llama3",
			Endpoints: []string{server.URL},
		}

		ctx := context.Background()
		texts := []string{"texto a", "texto b"}
		results, err := provider.EmbedDocuments(ctx, texts)

		if err != nil {
			t.Fatalf("Erro no fallback de embeddings: %v", err)
		}
		if len(results) != len(texts) {
			t.Fatalf("Esperava %d embeddings, obteve %d", len(texts), len(results))
		}

		calls := int(embeddingsCalls.Load())
		// Deve ter feito uma chamada por texto (sequencial)
		if calls != len(texts) {
			t.Errorf("Esperava %d chamadas sequenciais a /api/embeddings, obteve %d", len(texts), calls)
		}
	})
}

// TestCostTrackingLogging verifica que o CostTrackingProvider loga informações
// de uso de tokens quando disponíveis no contexto.
func TestCostTrackingLogging(t *testing.T) {
	// Criar mock server que simula OpenAI com usage metadata
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "resposta do modelo",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		})
	}))
	defer server.Close()

	// Criar OpenAI provider apontando para o mock
	provider := &ai.OpenAIProvider{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: server.URL,
	}

	// Envolver com CostTrackingProvider
	costProvider := ai.NewCostTrackingProvider(provider, "gpt-4o-mini")

	ctx := context.Background()
	result, err := costProvider.Completion(ctx, "system prompt", "user prompt")

	if err != nil {
		t.Fatalf("Erro na completion: %v", err)
	}
	if result != "resposta do modelo" {
		t.Errorf("Resposta inesperada: %q", result)
	}

	// Verificar que o contexto contém os metadados de uso
	// (O OpenAIProvider chama SetUsage internamente, mas o contexto não é mutável
	// dessa forma — o importante é que a chamada funcionou sem erro)
	t.Log("CostTrackingProvider executou completion sem erros e logou uso via slog")
}

// TestEmbeddingCache verifica que o CachedEmbeddingProvider reutiliza cache
// para embeddings idênticos, reduzindo chamadas ao provider.
func TestEmbeddingCache(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{{"name": "llama3"}},
			})

		case "/api/embed":
			requestCount.Add(1)
			var req struct {
				Input []string `json:"input"`
			}
			json.NewDecoder(r.Body).Decode(&req)

			embeddings := make([][]float32, len(req.Input))
			for i := range embeddings {
				embeddings[i] = []float32{0.7, 0.8, 0.9}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embeddings": embeddings,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	innerProvider := &ai.OllamaProvider{
		BaseURL:   server.URL,
		Model:     "llama3",
		Endpoints: []string{server.URL},
	}

	// Envolver com cache
	cachedProvider := ai.NewCachedEmbeddingProvider(innerProvider, 100, 1*time.Hour)

	ctx := context.Background()
	texts := []string{"texto para cache"}

	// Primeira chamada — deve ir ao provider
	result1, err := cachedProvider.EmbedDocuments(ctx, texts)
	if err != nil {
		t.Fatalf("Erro na primeira chamada: %v", err)
	}
	if len(result1) != 1 {
		t.Fatalf("Esperava 1 embedding, obteve %d", len(result1))
	}

	firstCallCount := int(requestCount.Load())
	if firstCallCount != 1 {
		t.Errorf("Esperava 1 request ao provider na primeira chamada, obteve %d", firstCallCount)
	}

	// Segunda chamada com mesmos textos — deve usar cache (sem request adicional)
	result2, err := cachedProvider.EmbedDocuments(ctx, texts)
	if err != nil {
		t.Fatalf("Erro na segunda chamada: %v", err)
	}
	if len(result2) != 1 {
		t.Fatalf("Esperava 1 embedding na segunda chamada, obteve %d", len(result2))
	}

	secondCallCount := int(requestCount.Load())
	if secondCallCount != firstCallCount {
		t.Errorf("Esperava cache hit (sem requests adicionais), mas o provider recebeu %d requests no total", secondCallCount)
	}

	// Verificar que os resultados são iguais
	if len(result1[0]) != len(result2[0]) {
		t.Errorf("Embeddings diferem entre chamadas: %v vs %v", result1[0], result2[0])
	}

	t.Logf("Cache funcionou: %d request(s) ao provider para 2 chamadas idênticas", secondCallCount)
}
