package main

import (
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/retry"
)

// nomeado é um provider que só responde ao Name; os outros métodos não são chamados.
type nomeado struct {
	ai.Provider
	nome string
}

func (n nomeado) Name() string { return n.nome }

func TestRotuloProvider(t *testing.T) {
	casos := []struct {
		p    ai.Provider
		want string
	}{
		{ai.NewClaudeCLIProvider(), "claude-cli"},
		{ai.NewGeminiCLIProvider(), "gemini-cli"},
		{&ai.OllamaProvider{}, "ollama"},
		{&ai.OllamaProvider{BaseURL: "http://localhost:11434"}, "ollama"},
		{&ai.GeminiProvider{}, "gemini"},
		{&ai.OpenAIProvider{}, "openai"},
		{nomeado{nome: "AWS Bedrock (Cloud)"}, "bedrock"},
		// Os decorators repassam o Name do provider de dentro.
		{ai.NewRetryProvider(ai.NewClaudeCLIProvider(), retry.DefaultOptions(), nil), "claude-cli"},
		{nomeado{nome: "  Meu Provider  "}, "meu provider"},
		{nil, ""},
	}
	for _, c := range casos {
		if got := rotuloProvider(c.p); got != c.want {
			t.Errorf("rotuloProvider(%T) = %q, esperado %q", c.p, got, c.want)
		}
	}
}
