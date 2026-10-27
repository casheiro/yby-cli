package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// TestModeloDetectado: o /modelo recebe de cada provider os modelos da factory e a troca
// de modelo; um provider que a factory não conhece fica só com o próprio nome.
func TestModeloDetectado(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("YBY_AI_MODEL", "")
	t.Setenv("GEMINI_MODEL", "")

	claude := modeloDetectado(context.Background(), ai.NewClaudeCLIProvider())
	if claude.Rotulo != "claude-cli" || claude.Padrao != "" || !reflect.DeepEqual(claude.Modelos, []string{"sonnet", "opus", "haiku"}) {
		t.Errorf("claude-cli: %+v", claude)
	}
	if claude.ComModelo == nil {
		t.Fatal("o claude-cli troca de modelo")
	}
	// A regra é da factory: o modelo que o provider não tem volta como erro, sem IA.
	if _, _, err := claude.ComModelo(context.Background(), "gpt-4o"); err == nil || !strings.Contains(err.Error(), "o claude-cli não tem o modelo gpt-4o") {
		t.Errorf("modelo inválido: %v", err)
	}

	gemini := modeloDetectado(context.Background(), &ai.GeminiProvider{})
	if gemini.Padrao != "gemini-2.5-flash" || len(gemini.Modelos) != 3 {
		t.Errorf("gemini: %+v", gemini)
	}

	outro := modeloDetectado(context.Background(), nomeado{nome: "Meu Provider"})
	if outro.Rotulo != "meu provider" || outro.ComModelo != nil || len(outro.Modelos) != 0 {
		t.Errorf("provider desconhecido: %+v", outro)
	}
}

func TestModeloEmUso(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("YBY_AI_MODEL", "")
	claude := nomeado{nome: "Claude Code CLI"}

	if got := modeloEmUso(claude); got != "" {
		t.Errorf("sem config, esperava o padrão do provider (vazio), veio %q", got)
	}
	if got := modeloEmUso(nil); got != "" {
		t.Errorf("provider nil: esperava vazio, veio %q", got)
	}

	if err := os.MkdirAll(filepath.Join(home, ".yby"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "ai:\n  model: sonnet\n  models:\n    claude-cli: opus\n"
	if err := os.WriteFile(filepath.Join(home, ".yby", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := modeloEmUso(claude); got != "opus" {
		t.Errorf("modelo do provider no config: esperava opus, veio %q", got)
	}
	if got := modeloEmUso(nomeado{nome: "Gemini CLI"}); got != "sonnet" {
		t.Errorf("sem modelo próprio, vale o global: esperava sonnet, veio %q", got)
	}
}
