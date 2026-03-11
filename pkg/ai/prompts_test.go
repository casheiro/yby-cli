package ai

import (
	"strings"
	"testing"
)

func TestSystemPrompt_NotEmpty(t *testing.T) {
	if SystemPrompt == "" {
		t.Error("SystemPrompt should not be empty")
	}
}

func TestSystemPrompt_ContainsJSON(t *testing.T) {
	if !strings.Contains(SystemPrompt, "JSON") {
		t.Error("SystemPrompt should mention JSON output format")
	}
}

func TestSystemPrompt_ContainsMandatoryFiles(t *testing.T) {
	mandatoryPaths := []string{
		".synapstor/00_PROJECT_OVERVIEW.md",
		".synapstor/.uki/",
		".synapstor/.personas/",
	}
	for _, p := range mandatoryPaths {
		if !strings.Contains(SystemPrompt, p) {
			t.Errorf("SystemPrompt should contain mandatory path %q", p)
		}
	}
}

func TestSystemPrompt_ContainsLanguageInstruction(t *testing.T) {
	if !strings.Contains(SystemPrompt, "DETECT THE LANGUAGE") {
		t.Error("SystemPrompt should contain language detection instruction")
	}
}

func TestSystemPrompt_ContainsDefaultLanguage(t *testing.T) {
	if !strings.Contains(SystemPrompt, "PT-BR") {
		t.Error("SystemPrompt should mention PT-BR as fallback language")
	}
}
