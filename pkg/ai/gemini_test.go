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

// ─── Testes de serialização/desserialização de estruturas ────────────────────

func TestGeminiCompletion_MockServer_Success(t *testing.T) {
	// Valida que a estrutura geminiResponse desserializa corretamente
	payload := `{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`
	var resp geminiResponse
	err := json.Unmarshal([]byte(payload), &resp)
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Candidates[0].Content.Parts[0].Text)
}

func TestGeminiCompletion_MockServer_EmptyResponse(t *testing.T) {
	// Valida o caminho de erro "resposta vazia"
	payload := `{"candidates":[]}`
	var resp geminiResponse
	err := json.Unmarshal([]byte(payload), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Candidates)
}

func TestGeminiRequest_Serialization(t *testing.T) {
	req := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: "hello"}}},
		},
		GenerationConfig: geminiConfig{ResponseMimeType: "application/json"},
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "hello")
	assert.Contains(t, string(data), "application/json")
}

func TestGeminiEmbeddingRequest_Serialization(t *testing.T) {
	req := geminiBatchEmbeddingRequest{
		Requests: []geminiEmbeddingRequest{
			{
				Model: "models/text-embedding-004",
				Content: geminiEmbeddingContent{
					Parts: []geminiPart{{Text: "embed this text"}},
				},
			},
		},
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "embed this text")
}

func TestGeminiBatchResponse_Deserialization(t *testing.T) {
	raw := `{"embeddings":[{"values":[0.1,0.2,0.3]},{"values":[0.4,0.5,0.6]}]}`
	var resp geminiBatchEmbeddingResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Len(t, resp.Embeddings, 2)
	assert.InDelta(t, 0.1, resp.Embeddings[0].Values[0], 0.001)
	assert.InDelta(t, 0.4, resp.Embeddings[1].Values[0], 0.001)
}

func TestGeminiEmbedDocuments_EmptyList(t *testing.T) {
	p := &GeminiProvider{APIKey: "key", Model: "gemini", BaseURL: "http://localhost"}
	result, err := p.EmbedDocuments(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// ─── Testes com httptest (BaseURL mockável) ─────────────────────────────────

func newTestGeminiProvider(serverURL string) *GeminiProvider {
	return &GeminiProvider{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: serverURL,
	}
}

func TestGeminiCompletion_HTTPTest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "generateContent")
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{
				{Content: struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}{Parts: []struct {
					Text string `json:"text"`
				}{{Text: "resposta do teste"}}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, "resposta do teste", result)
}

func TestGeminiCompletion_HTTPTest_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"candidates": []interface{}{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

func TestGeminiCompletion_HTTPTest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGeminiCompletion_HTTPTest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
}

func TestGeminiStreamCompletion_HTTPTest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "streamGenerateContent")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Simular SSE com um chunk
		chunk := `{"candidates":[{"content":{"parts":[{"text":"resposta do teste"}]}}]}`
		fmt.Fprintf(w, "data: %s\n\n", chunk)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Equal(t, "resposta do teste", buf.String())
}

func TestGeminiGenerateGovernance_HTTPTest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blueprint := GovernanceBlueprint{
			Files: []GeneratedFile{
				{Path: ".synapstor/.uki/test.md", Content: "# Test"},
			},
		}
		bpJSON, _ := json.Marshal(blueprint)
		resp := geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{
				{Content: struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}{Parts: []struct {
					Text string `json:"text"`
				}{{Text: string(bpJSON)}}}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "teste")
	require.NoError(t, err)
	assert.NotNil(t, blueprint)
	assert.Len(t, blueprint.Files, 1)
}

func TestGeminiGenerateGovernance_HTTPTest_InvalidJSON(t *testing.T) {
	// Gemini retorna texto que não é JSON válido para o blueprint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "not a valid blueprint json"}},
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.GenerateGovernance(context.Background(), "teste")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json")
}

func TestGeminiGenerateGovernance_HTTPTest_MarkdownFences(t *testing.T) {
	// Testa limpeza de markdown fences que Gemini às vezes adiciona
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blueprint := GovernanceBlueprint{Files: []GeneratedFile{{Path: "test.md", Content: "ok"}}}
		bpJSON, _ := json.Marshal(blueprint)
		wrappedJSON := "```json" + string(bpJSON) + "```"
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": wrappedJSON}},
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "teste")
	require.NoError(t, err)
	assert.Len(t, blueprint.Files, 1)
}

func TestGeminiEmbedDocuments_HTTPTest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "batchEmbedContents")
		resp := geminiBatchEmbeddingResponse{
			Embeddings: []struct {
				Values []float32 `json:"values"`
			}{
				{Values: []float32{0.1, 0.2}},
				{Values: []float32{0.3, 0.4}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	embeddings, err := p.EmbedDocuments(context.Background(), []string{"text1", "text2"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)
}

func TestGeminiEmbedDocuments_HTTPTest_Mismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retorna 1 embedding para 2 textos
		resp := geminiBatchEmbeddingResponse{
			Embeddings: []struct {
				Values []float32 `json:"values"`
			}{
				{Values: []float32{0.1}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.EmbedDocuments(context.Background(), []string{"text1", "text2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestGeminiEmbedDocuments_HTTPTest_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	_, err := p.EmbedDocuments(context.Background(), []string{"text1"})
	assert.Error(t, err)
}

// ─── Testes do provider ─────────────────────────────────────────────────────

func TestGeminiProvider_Name(t *testing.T) {
	p := &GeminiProvider{}
	assert.Equal(t, "Google Gemini (Cloud)", p.Name())
}

func TestGeminiProvider_IsAvailable(t *testing.T) {
	p := &GeminiProvider{APIKey: "key"}
	assert.True(t, p.IsAvailable(context.Background()))

	p2 := &GeminiProvider{}
	assert.False(t, p2.IsAvailable(context.Background()))
}

func TestNewGeminiProvider_WithEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("GEMINI_MODEL", "custom-model")
	p := NewGeminiProvider()
	require.NotNil(t, p)
	assert.Equal(t, "test-key", p.APIKey)
	assert.Equal(t, "custom-model", p.Model)
	assert.Equal(t, "https://generativelanguage.googleapis.com", p.BaseURL)
}

func TestNewGeminiProvider_NoKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	p := NewGeminiProvider()
	assert.Nil(t, p)
}

func TestNewGeminiProvider_DefaultModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("GEMINI_MODEL", "")
	p := NewGeminiProvider()
	require.NotNil(t, p)
	assert.Equal(t, "gemini-2.5-flash", p.Model)
}

// ─── Testes de streaming SSE real ──────────────────────────────────────────────

func TestGeminiStreamCompletion_SSE_MultipleChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "streamGenerateContent")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"candidates":[{"content":{"parts":[{"text":"Ola "}]}}]}`,
			`{"candidates":[{"content":{"parts":[{"text":"mundo "}]}}]}`,
			`{"candidates":[{"content":{"parts":[{"text":"cruel"}]}}]}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Equal(t, "Ola mundo cruel", buf.String())
}

func TestGeminiStreamCompletion_SSE_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 429, apiErr.StatusCode)
	assert.Equal(t, "gemini", apiErr.Provider)
}

func TestGeminiStreamCompletion_SSE_MalformedData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Linha com JSON invalido seguida de chunk valido
		fmt.Fprintf(w, "data: {json invalido}\n\n")
		fmt.Fprintf(w, "data: %s\n\n", `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Equal(t, "ok", buf.String())
}
