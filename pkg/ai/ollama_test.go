package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newOllamaTestServer cria um servidor httptest e um OllamaProvider apontando para ele.
func newOllamaTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *OllamaProvider) {
	t.Helper()
	server := httptest.NewServer(handler)
	provider := &OllamaProvider{
		BaseURL: server.URL,
		Model:   "llama3",
	}
	return server, provider
}

// ollamaRouterHandler retorna um handler que roteia /api/tags, /api/generate, /api/embed e /api/embeddings
// para handlers distintos.
func ollamaRouterHandler(tagsHandler, generateHandler, embeddingsHandler http.HandlerFunc) http.HandlerFunc {
	return ollamaRouterHandlerFull(tagsHandler, generateHandler, nil, embeddingsHandler)
}

// ollamaRouterHandlerFull roteia incluindo /api/embed (batch, Ollama v0.5+).
func ollamaRouterHandlerFull(tagsHandler, generateHandler, embedHandler, embeddingsHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			if tagsHandler != nil {
				tagsHandler(w, r)
				return
			}
		case "/api/generate":
			if generateHandler != nil {
				generateHandler(w, r)
				return
			}
		case "/api/embed":
			if embedHandler != nil {
				embedHandler(w, r)
				return
			}
		case "/api/embeddings":
			if embeddingsHandler != nil {
				embeddingsHandler(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}
}

// tagsOK retorna um handler de /api/tags com um modelo disponível.
func tagsOK(modelName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: modelName},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// ─── Testes básicos do provider ──────────────────────────────────────────────

func TestOllamaProvider_Name(t *testing.T) {
	t.Run("com BaseURL definida", func(t *testing.T) {
		p := &OllamaProvider{BaseURL: "http://localhost:11434"}
		assert.Equal(t, "Ollama (Local @ http://localhost:11434)", p.Name())
	})

	t.Run("sem BaseURL", func(t *testing.T) {
		p := &OllamaProvider{}
		assert.Equal(t, "Ollama (Local)", p.Name())
	})
}

func TestOllamaProvider_IsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer server.Close()

	p := &OllamaProvider{
		Endpoints: []string{server.URL},
	}
	available := p.IsAvailable(context.Background())
	assert.True(t, available)
	assert.Equal(t, server.URL, p.BaseURL, "BaseURL deve ser resolvido para o endpoint disponível")
}

func TestOllamaProvider_IsAvailable_NoEndpoints(t *testing.T) {
	p := &OllamaProvider{
		Endpoints: []string{"http://127.0.0.1:1"}, // porta inválida, não conecta
	}
	available := p.IsAvailable(context.Background())
	assert.False(t, available)
}

// ─── Testes de ping ─────────────────────────────────────────────────────────

func TestOllamaProvider_ping(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/tags", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		p := &OllamaProvider{}
		assert.True(t, p.ping(context.Background(), server.URL))
	})

	t.Run("falha - status 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		p := &OllamaProvider{}
		assert.False(t, p.ping(context.Background(), server.URL))
	})

	t.Run("falha - conexão recusada", func(t *testing.T) {
		p := &OllamaProvider{}
		assert.False(t, p.ping(context.Background(), "http://127.0.0.1:1"))
	})
}

// ─── Testes de resolveModel ─────────────────────────────────────────────────

func TestOllamaProvider_resolveModel(t *testing.T) {
	t.Run("sucesso - retorna o primeiro modelo", func(t *testing.T) {
		server, provider := newOllamaTestServer(t, tagsOK("mistral"))
		defer server.Close()

		err := provider.resolveModel(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "mistral", provider.Model)
	})

	t.Run("nenhum modelo disponível", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ollamaTagsResponse{})
		}
		server, provider := newOllamaTestServer(t, handler)
		defer server.Close()

		err := provider.resolveModel(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nenhum modelo encontrado")
	})

	t.Run("erro HTTP", func(t *testing.T) {
		// Servidor já fechado causa erro de conexão
		server, provider := newOllamaTestServer(t, nil)
		server.Close()

		err := provider.resolveModel(context.Background())
		assert.Error(t, err)
	})
}

// ─── Testes de GenerateGovernance ───────────────────────────────────────────

func TestOllamaProvider_GenerateGovernance_Success(t *testing.T) {
	blueprint := GovernanceBlueprint{
		Domain:    "fintech",
		RiskLevel: "alto",
		Summary:   "Projeto de pagamentos",
		Files: []GeneratedFile{
			{Path: ".synapstor/test.md", Content: "# Teste"},
		},
	}
	bpJSON, _ := json.Marshal(blueprint)

	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			resp := ollamaResponse{Response: string(bpJSON)}
			json.NewEncoder(w).Encode(resp)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	result, err := provider.GenerateGovernance(context.Background(), "Um projeto fintech")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "fintech", result.Domain)
	assert.Equal(t, "alto", result.RiskLevel)
	assert.Len(t, result.Files, 1)
}

func TestOllamaProvider_GenerateGovernance_Error(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	_, err := provider.GenerateGovernance(context.Background(), "teste")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaProvider_GenerateGovernance_MalformedJSON(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			resp := ollamaResponse{Response: "isso não é json válido {{{"}
			json.NewEncoder(w).Encode(resp)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	_, err := provider.GenerateGovernance(context.Background(), "teste")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json")
}

// ─── Testes de Completion ───────────────────────────────────────────────────

func TestOllamaProvider_Completion_Success(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			resp := ollamaResponse{Response: "resposta de completamento"}
			json.NewEncoder(w).Encode(resp)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	result, err := provider.Completion(context.Background(), "system prompt", "user prompt")
	require.NoError(t, err)
	assert.Equal(t, "resposta de completamento", result)
}

func TestOllamaProvider_Completion_Error(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	_, err := provider.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ─── Testes de StreamCompletion ─────────────────────────────────────────────

func TestOllamaProvider_StreamCompletion_Success(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			// Ollama streaming retorna múltiplas linhas NDJSON
			chunks := []ollamaResponse{
				{Response: "olá "},
				{Response: "mundo"},
			}
			for _, c := range chunks {
				json.NewEncoder(w).Encode(c)
			}
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	var buf bytes.Buffer
	err := provider.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Equal(t, "olá mundo", buf.String())
}

func TestOllamaProvider_StreamCompletion_Error(t *testing.T) {
	handler := ollamaRouterHandler(
		tagsOK("llama3"),
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	var buf bytes.Buffer
	err := provider.StreamCompletion(context.Background(), "system", "user", &buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ─── Testes de EmbedDocuments (batch via /api/embed) ────────────────────────

func TestOllamaProvider_EmbedDocuments_Batch_Success(t *testing.T) {
	handler := ollamaRouterHandlerFull(
		tagsOK("llama3"),
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req ollamaEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)
			// Retornar um embedding por input
			embeddings := make([][]float32, len(req.Input))
			for i := range req.Input {
				embeddings[i] = []float32{0.1, 0.2, 0.3}
			}
			resp := ollamaEmbedResponse{Embeddings: embeddings}
			json.NewEncoder(w).Encode(resp)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	results, err := provider.EmbedDocuments(context.Background(), []string{"texto1", "texto2"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.InDelta(t, float32(0.1), results[0][0], 0.001)
	assert.InDelta(t, float32(0.1), results[1][0], 0.001)
}

func TestOllamaProvider_EmbedDocuments_Fallback_Sequential(t *testing.T) {
	// /api/embed retorna 404 (Ollama antigo), fallback para /api/embeddings
	handler := ollamaRouterHandlerFull(
		tagsOK("llama3"),
		nil,
		nil, // embed handler nil → retorna 404
		func(w http.ResponseWriter, r *http.Request) {
			resp := ollamaEmbeddingResponse{
				Embedding: []float32{0.4, 0.5, 0.6},
			}
			json.NewEncoder(w).Encode(resp)
		},
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	results, err := provider.EmbedDocuments(context.Background(), []string{"texto1", "texto2"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.InDelta(t, float32(0.4), results[0][0], 0.001)
	assert.InDelta(t, float32(0.4), results[1][0], 0.001)
}

func TestOllamaProvider_EmbedDocuments_Batch_Error(t *testing.T) {
	handler := ollamaRouterHandlerFull(
		tagsOK("llama3"),
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	_, err := provider.EmbedDocuments(context.Background(), []string{"texto"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaProvider_EmbedDocuments_Empty(t *testing.T) {
	provider := &OllamaProvider{BaseURL: "http://localhost:11434", Model: "llama3"}
	results, err := provider.EmbedDocuments(context.Background(), []string{})
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestOllamaProvider_EmbedDocuments_LargeBatch(t *testing.T) {
	var batchCount int
	handler := ollamaRouterHandlerFull(
		tagsOK("llama3"),
		nil,
		func(w http.ResponseWriter, r *http.Request) {
			batchCount++
			var req ollamaEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)
			embeddings := make([][]float32, len(req.Input))
			for i := range req.Input {
				embeddings[i] = []float32{float32(batchCount), float32(i)}
			}
			resp := ollamaEmbedResponse{Embeddings: embeddings}
			json.NewEncoder(w).Encode(resp)
		},
		nil,
	)

	server, provider := newOllamaTestServer(t, handler)
	defer server.Close()

	// 75 textos = 2 batches (50 + 25)
	texts := make([]string, 75)
	for i := range texts {
		texts[i] = fmt.Sprintf("texto-%d", i)
	}

	results, err := provider.EmbedDocuments(context.Background(), texts)
	require.NoError(t, err)
	assert.Len(t, results, 75)
	assert.Equal(t, 75, batchCount, "deveria ter feito 75 requests (batch de 1)")
}
