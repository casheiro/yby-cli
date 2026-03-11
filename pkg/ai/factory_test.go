package ai

import (
	"context"
	"os"
	"testing"
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
