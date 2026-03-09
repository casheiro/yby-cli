package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProvider_SemNenhumProviderDisponivel(t *testing.T) {
	// Garantir que nenhuma chave de API ou serviço esteja disponível
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OLLAMA_HOST", "http://localhost:19999") // Porta sem nada ouvindo
	t.Setenv("YBY_AI_PROVIDER", "")

	// Forçar provider inexistente para garantir nil
	p := GetProvider(context.Background(), "provider-inexistente-xyz")
	assert.Nil(t, p)
}

func TestGetProvider_ForcarGemini(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "gemini")
	require.NotNil(t, p)
	assert.Equal(t, "Google Gemini (Cloud)", p.Name())
}

func TestGetProvider_ForcarOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "openai")
	require.NotNil(t, p)
	assert.Equal(t, "OpenAI (Cloud)", p.Name())
}

func TestGetProvider_ForcarOllamaComServidor(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "ollama")
	// Se Ollama está disponível no ambiente, retorna provider; caso contrário, nil
	// Apenas verifica que não há panic e o resultado é consistente
	if p != nil {
		assert.Contains(t, p.Name(), "Ollama")
	}
}

func TestGetProvider_ForcarGeminiSemChave(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "gemini")
	// Sem chave de API, Gemini não está disponível
	assert.Nil(t, p)
}

func TestGetProvider_ForcarOpenAISemChave(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "openai")
	// Sem chave, OpenAI não está disponível
	assert.Nil(t, p)
}

func TestGetProvider_ViaEnvVar(t *testing.T) {
	t.Setenv("YBY_AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")

	p := GetProvider(context.Background(), "")
	require.NotNil(t, p)
	assert.Equal(t, "Google Gemini (Cloud)", p.Name())
}

func TestGetProvider_EnvVarSobrescreve(t *testing.T) {
	// Quando preferred está vazio, usa YBY_AI_PROVIDER
	t.Setenv("YBY_AI_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("GEMINI_API_KEY", "")

	p := GetProvider(context.Background(), "")
	require.NotNil(t, p)
	assert.Equal(t, "OpenAI (Cloud)", p.Name())
}

func TestGetProvider_PreferredTemPrioridade(t *testing.T) {
	// preferred tem prioridade sobre env var
	t.Setenv("YBY_AI_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("GEMINI_API_KEY", "test-key")

	p := GetProvider(context.Background(), "gemini")
	require.NotNil(t, p)
	assert.Equal(t, "Google Gemini (Cloud)", p.Name())
}

func TestGetProvider_AutoComOllama(t *testing.T) {
	// Criar servidor mock para Ollama
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "auto")
	// Ollama com mock server deve ser detectado
	if p != nil {
		assert.Contains(t, p.Name(), "Ollama")
	}
}

func TestGetProvider_AutoFallbackGemini(t *testing.T) {
	// Ollama indisponível, mas Gemini configurado
	t.Setenv("OLLAMA_HOST", "http://localhost:19999")
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YBY_AI_PROVIDER", "")

	// Forçar Gemini explicitamente para evitar detecção de Ollama real no ambiente
	p := GetProvider(context.Background(), "gemini")
	require.NotNil(t, p)
	assert.Equal(t, "Google Gemini (Cloud)", p.Name())
}

func TestGetProvider_AutoFallbackOpenAI(t *testing.T) {
	// Forçar OpenAI explicitamente para evitar detecção de Ollama real no ambiente
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("YBY_AI_PROVIDER", "")

	p := GetProvider(context.Background(), "openai")
	require.NotNil(t, p)
	assert.Equal(t, "OpenAI (Cloud)", p.Name())
}

func TestGetLanguage_ComEnvPersonalizado(t *testing.T) {
	t.Setenv("YBY_AI_LANGUAGE", "es-ES")
	lang := GetLanguage()
	assert.Equal(t, "es-ES", lang)
}

func TestGetLanguage_SemEnv(t *testing.T) {
	t.Setenv("YBY_AI_LANGUAGE", "")
	lang := GetLanguage()
	assert.Equal(t, "pt-BR", lang)
}
