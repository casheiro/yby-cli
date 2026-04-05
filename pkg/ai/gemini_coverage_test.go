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

// ─── GenerateGovernance — cenários adicionais de cobertura ───────────────────

func TestGeminiProvider_GenerateGovernance_HTTPError(t *testing.T) {
	// Servidor retorna 500 — cobre o path de erro de status != 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("erro interno simulado"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "projeto de teste")
	assert.Nil(t, blueprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "erro interno simulado")
}

func TestGeminiProvider_GenerateGovernance_MalformedJSON(t *testing.T) {
	// Servidor retorna 200 mas o body externo é JSON inválido (não parseia geminiResponse)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{json invalido sem aspas}"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "projeto de teste")
	assert.Nil(t, blueprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodificar")
}

func TestGeminiProvider_GenerateGovernance_EmptyCandidates(t *testing.T) {
	// Servidor retorna 200 com candidates vazio — cobre o path "resposta vazia"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []interface{}{},
		})
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "projeto de teste")
	assert.Nil(t, blueprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

func TestGeminiProvider_GenerateGovernance_EmptyParts(t *testing.T) {
	// Candidates presente mas Parts vazio — cobre o segundo check do len()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []interface{}{},
				}},
			},
		})
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "projeto de teste")
	assert.Nil(t, blueprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

func TestGeminiProvider_GenerateGovernance_SuccessCompleto(t *testing.T) {
	// Retorna blueprint completo com domain e risk_level — valida todos os campos
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "generateContent")

		bp := GovernanceBlueprint{
			Domain:    "Fintech",
			RiskLevel: "Alto",
			Summary:   "Gateway de pagamento",
			Files: []GeneratedFile{
				{Path: ".synapstor/decisoes.md", Content: "# Decisão 1"},
				{Path: ".github/workflows/ci.yaml", Content: "name: CI"},
			},
		}
		bpJSON, _ := json.Marshal(bp)
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": string(bpJSON)}},
				}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "gateway de pagamento cripto")
	require.NoError(t, err)
	require.NotNil(t, blueprint)
	assert.Equal(t, "Fintech", blueprint.Domain)
	assert.Equal(t, "Alto", blueprint.RiskLevel)
	assert.Len(t, blueprint.Files, 2)
}

func TestGeminiProvider_GenerateGovernance_BlueprintInvalidoNoParts(t *testing.T) {
	// Content do Parts é texto não-JSON — cobre falha no json.Unmarshal do blueprint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "texto que não é json válido para blueprint"}},
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "teste")
	assert.Nil(t, blueprint)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json")
}

// ─── Completion — cenários adicionais de cobertura ──────────────────────────

func TestGeminiProvider_Completion_HTTPError(t *testing.T) {
	// Status 400 (não retryable) — cobre o path de break imediato no loop
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "system", "user")
	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestGeminiProvider_Completion_EmptyCandidates(t *testing.T) {
	// Retorna 200 com candidates vazio — cobre path "resposta vazia"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []interface{}{},
		})
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "system", "user")
	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

func TestGeminiProvider_Completion_SuccessCompleto(t *testing.T) {
	// Retorna resposta válida — verifica que o texto é retornado corretamente
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "resposta completa do gemini"}},
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "analise isso", "texto do usuário")
	require.NoError(t, err)
	assert.Equal(t, "resposta completa do gemini", result)
}

func TestGeminiProvider_Completion_JSONInvalido(t *testing.T) {
	// Body retornado não é JSON válido com status 200 — cobre falha no Decode
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{{json quebrado"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "system", "user")
	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodificar")
}

func TestGeminiProvider_Completion_EmptyParts(t *testing.T) {
	// Candidates presente mas Parts vazio
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []interface{}{},
				}},
			},
		})
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.Completion(context.Background(), "system", "user")
	assert.Empty(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

// ─── StreamCompletion — cenários adicionais de cobertura ────────────────────

func TestGeminiProvider_StreamCompletion_HTTPError(t *testing.T) {
	// Servidor retorna 500 — streaming deve propagar como APIError
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Empty(t, buf.String())
}

func TestGeminiProvider_StreamCompletion_SuccessCompleto(t *testing.T) {
	// Verifica que StreamCompletion SSE escreve corretamente no writer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		chunk := `{"candidates":[{"content":{"parts":[{"text":"stream output completo"}]}}]}`
		fmt.Fprintf(w, "data: %s\n\n", chunk)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system prompt", "user prompt", &buf)
	require.NoError(t, err)
	assert.Equal(t, "stream output completo", buf.String())
}

// ─── EmbedDocuments — cenários adicionais de cobertura ──────────────────────

func TestGeminiProvider_EmbedDocuments_HTTPError(t *testing.T) {
	// Servidor retorna 500 — cobre path de erro de status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("embedding error"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.EmbedDocuments(context.Background(), []string{"texto para embedar"})
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGeminiProvider_EmbedDocuments_JSONInvalidoNoBody(t *testing.T) {
	// Servidor retorna 200 mas body não é JSON válido — cobre falha no Unmarshal
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("nao eh json"))
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	result, err := p.EmbedDocuments(context.Background(), []string{"texto"})
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodificar")
}

func TestGeminiProvider_GenerateGovernance_MarkdownFenceSemJson(t *testing.T) {
	// Testa limpeza de fences "```" sem prefixo "json"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bp := GovernanceBlueprint{
			Domain: "Test",
			Files:  []GeneratedFile{{Path: "a.md", Content: "ok"}},
		}
		bpJSON, _ := json.Marshal(bp)
		// Envolve com apenas ``` (sem json)
		wrapped := "```" + string(bpJSON) + "```"
		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": wrapped}},
				}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(server.URL)
	blueprint, err := p.GenerateGovernance(context.Background(), "teste")
	require.NoError(t, err)
	require.NotNil(t, blueprint)
	assert.Equal(t, "Test", blueprint.Domain)
}
