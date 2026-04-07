package ai

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai/prompts"
)

func TestSystemPrompt_NotEmpty(t *testing.T) {
	if prompts.Get("governance.system") == "" {
		t.Error("prompt governance.system não deveria estar vazio")
	}
}

func TestSystemPrompt_ContainsJSON(t *testing.T) {
	if !strings.Contains(prompts.Get("governance.system"), "JSON") {
		t.Error("prompt governance.system deveria mencionar formato JSON")
	}
}

func TestSystemPrompt_ContainsMandatoryFiles(t *testing.T) {
	mandatoryPaths := []string{
		".synapstor/00_PROJECT_OVERVIEW.md",
		".synapstor/.uki/",
		".synapstor/.personas/",
	}
	governancePrompt := prompts.Get("governance.system")
	for _, p := range mandatoryPaths {
		if !strings.Contains(governancePrompt, p) {
			t.Errorf("prompt governance.system deveria conter o caminho obrigatório %q", p)
		}
	}
}

func TestSystemPrompt_ContainsLanguageInstruction(t *testing.T) {
	if !strings.Contains(prompts.Get("governance.system"), "DETECT THE LANGUAGE") {
		t.Error("prompt governance.system deveria conter instrução de detecção de idioma")
	}
}

func TestSystemPrompt_ContainsDefaultLanguage(t *testing.T) {
	if !strings.Contains(prompts.Get("governance.system"), "PT-BR") {
		t.Error("prompt governance.system deveria mencionar PT-BR como idioma padrão")
	}
}
