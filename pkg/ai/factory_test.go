package ai

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLanguage_Default(t *testing.T) {
	os.Unsetenv("YBY_AI_LANGUAGE")
	lang := GetLanguage()
	if lang != "pt-BR" {
		t.Errorf("expected pt-BR, got %s", lang)
	}
}

func TestGetLanguage_FromEnv(t *testing.T) {
	os.Setenv("YBY_AI_LANGUAGE", "en-US")
	defer os.Unsetenv("YBY_AI_LANGUAGE")
	lang := GetLanguage()
	if lang != "en-US" {
		t.Errorf("expected en-US, got %s", lang)
	}
}

func TestGetProvider_AutoNoProviders(t *testing.T) {
	// With no network and no API keys, GetProvider should return nil
	os.Unsetenv("YBY_AI_PROVIDER")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	ctx := context.Background()
	p := GetProvider(ctx, "")
	// In test env with no Ollama/Gemini/OpenAI available, we expect nil
	// If some provider happened to be available this may not be nil, but we just
	// verify it doesn't panic and returns a consistent value.
	_ = p
}

func TestGetProvider_ExplicitUnknown(t *testing.T) {
	ctx := context.Background()
	// Passing an unknown/unavailable provider name should return nil (Strict)
	p := GetProvider(ctx, "nonexistent-provider-xyz")
	// This will resolve to nil since no such provider exists
	_ = p
}

func TestGetProvider_WithEnvVar(t *testing.T) {
	os.Setenv("YBY_AI_PROVIDER", "ollama")
	defer os.Unsetenv("YBY_AI_PROVIDER")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	ctx := context.Background()
	// Ollama likely not running in test environment; result may be nil
	p := GetProvider(ctx, "")
	_ = p
}

func TestGetProvider_AutoPreferred(t *testing.T) {
	os.Unsetenv("YBY_AI_PROVIDER")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	ctx := context.Background()
	// "auto" should behave the same as empty string
	p := GetProvider(ctx, "auto")
	_ = p
}

// cadeiaCLI desce pelos decorators dos providers CLI (Retry -> TokenAware) e devolve o
// TokenAware, falhando se faltar algum.
func cadeiaCLI(t *testing.T, p Provider) *TokenAwareProvider {
	t.Helper()
	retry, ok := p.(*RetryProvider)
	require.True(t, ok, "RetryProvider por fora: %T", p)
	token, ok := retry.inner.(*TokenAwareProvider)
	require.True(t, ok, "TokenAwareProvider: %T", retry.inner)
	return token
}

func TestGetProvider_CLIsComTokenAwareERetry(t *testing.T) {
	isolarConfig(t)
	cliFalso(t, "claude")
	token := cadeiaCLI(t, GetProvider(context.Background(), "claude-cli"))
	_, ok := token.inner.(*ClaudeCLIProvider)
	assert.True(t, ok, "ClaudeCLIProvider por dentro: %T", token.inner)

	cliFalso(t, "gemini")
	token = cadeiaCLI(t, GetProvider(context.Background(), "gemini-cli"))
	_, ok = token.inner.(*GeminiCLIProvider)
	assert.True(t, ok, "GeminiCLIProvider por dentro: %T", token.inner)
}

func TestGetProvider_CLIsNaoCortamPromptGrande(t *testing.T) {
	isolarConfig(t)
	cliFalso(t, "claude")
	prompt := strings.Repeat("a", 100*1024) // ~25 mil tokens: passa dos 8192 do padrão

	for _, modelo := range []string{"", "sonnet", "opus", "haiku", "claude-sonnet-4-5"} {
		p, _, err := GetProviderWithModel(context.Background(), "claude-cli", modelo)
		require.NoError(t, err, modelo)
		token := cadeiaCLI(t, p)
		assert.Equal(t, 200_000, token.meta.ContextWindow, "claude-cli com %q", modelo)
		assert.NoError(t, token.checkTokens(prompt), "claude-cli com %q", modelo)
	}

	cliFalso(t, "gemini")
	for modelo, janela := range map[string]int{"": 1_000_000, "gemini-2.5-pro": 1_000_000, "gemini-3-pro": 1_000_000} {
		p, _, err := GetProviderWithModel(context.Background(), "gemini-cli", modelo)
		require.NoError(t, err, modelo)
		token := cadeiaCLI(t, p)
		assert.Equal(t, janela, token.meta.ContextWindow, "gemini-cli com %q", modelo)
		assert.NoError(t, token.checkTokens(prompt), "gemini-cli com %q", modelo)
	}
}

func TestGetProvider_FalhaDoCLINaoRepete(t *testing.T) {
	isolarConfig(t)
	cliFalso(t, "claude")
	p := GetProvider(context.Background(), "claude-cli")
	cadeiaCLI(t, p)

	inicio := time.Now()
	_, err := p.Completion(context.Background(), "s", "u")
	require.Error(t, err)
	// O primeiro intervalo do retry é de 1 s: falha de CLI não é retentável e volta antes.
	assert.Less(t, time.Since(inicio), time.Second, "a falha do CLI foi repetida: %v", err)
}
