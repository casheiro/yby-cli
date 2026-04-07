package prompts

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestGet_DefaultPrompt(t *testing.T) {
	result := Get("sentinel.scan")
	if result == "" {
		t.Fatal("esperava prompt não vazio para 'sentinel.scan', obteve string vazia")
	}
	if result != SentinelScan {
		t.Errorf("esperava o conteúdo da constante SentinelScan, obteve valor diferente")
	}
}

func TestGet_UnknownPrompt(t *testing.T) {
	result := Get("nonexistent")
	if result != "" {
		t.Errorf("esperava string vazia para prompt inexistente, obteve: %q", result)
	}
}

func TestGetWithVars(t *testing.T) {
	vars := map[string]string{
		"blueprint_json_summary": "resumo do projeto",
		"cluster_context":        "contexto do cluster",
		"tools_prompt":           "ferramentas disponíveis",
	}

	result := GetWithVars("bard.system", vars)

	if result == "" {
		t.Fatal("esperava prompt não vazio, obteve string vazia")
	}

	// Verifica que as variáveis foram substituídas
	for key, value := range vars {
		if !contains(result, value) {
			t.Errorf("esperava que o resultado contivesse %q (substituição de {{%s}})", value, key)
		}
		placeholder := "{{" + key + "}}"
		if contains(result, placeholder) {
			t.Errorf("placeholder %s não foi substituído", placeholder)
		}
	}
}

func TestList(t *testing.T) {
	names := List()

	if len(names) != 8 {
		t.Errorf("esperava 8 prompts, obteve %d: %v", len(names), names)
	}

	expected := []string{
		"atlas.refine",
		"bard.system",
		"governance.system",
		"sentinel.investigate",
		"sentinel.scan",
		"synapstor.capture",
		"synapstor.study",
		"synapstor.tagger",
	}

	sort.Strings(names)
	sort.Strings(expected)

	for i, name := range expected {
		if i >= len(names) || names[i] != name {
			t.Errorf("esperava %q na posição %d, obteve %q", name, i, names[i])
		}
	}
}

func TestGet_OverrideFromProject(t *testing.T) {
	// Cria diretório temporário para simular o diretório do projeto
	tmpDir := t.TempDir()

	// Salva e restaura o diretório de trabalho
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Muda para o diretório temporário
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Cria o arquivo de override do projeto
	overrideDir := filepath.Join(tmpDir, ".yby", "prompts")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	overrideContent := "prompt customizado do projeto"
	overridePath := filepath.Join(overrideDir, "sentinel.scan.txt")
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Get("sentinel.scan")
	if result != overrideContent {
		t.Errorf("esperava override do projeto %q, obteve %q", overrideContent, result)
	}
}

func TestGet_OverrideFromGlobal(t *testing.T) {
	// Cria diretório temporário para simular o HOME
	tmpHome := t.TempDir()

	// Salva e restaura HOME
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Muda para um diretório sem override de projeto
	tmpProject := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpProject); err != nil {
		t.Fatal(err)
	}

	// Cria o arquivo de override global
	overrideDir := filepath.Join(tmpHome, ".yby", "prompts")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	overrideContent := "prompt customizado global"
	overridePath := filepath.Join(overrideDir, "sentinel.scan.txt")
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Get("sentinel.scan")
	if result != overrideContent {
		t.Errorf("esperava override global %q, obteve %q", overrideContent, result)
	}
}

// contains verifica se s contém substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
