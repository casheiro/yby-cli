package decay

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockRunner simula execução de comandos para testes.
type MockRunner struct {
	Responses map[string]string
}

// Run retorna respostas pré-configuradas baseado no path do arquivo.
func (m *MockRunner) Run(name string, args ...string) (string, error) {
	// O último argumento é o path do arquivo
	if len(args) > 0 {
		lastArg := args[len(args)-1]
		if resp, ok := m.Responses[lastArg]; ok {
			return resp, nil
		}
	}
	return "", fmt.Errorf("sem resposta mock")
}

func criarUKIsDecay(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// UKI recente (timestamp = agora - 30 dias)
	recentTS := time.Now().Add(-30 * 24 * time.Hour).Unix()
	recentFile := fmt.Sprintf("UKI-%d-recente.md", recentTS)
	recentContent := `# Documento Recente
**ID:** UKI-RECENTE
**Type:** Reference
**Status:** Active

## Content
Este documento é recente.
`
	if err := os.WriteFile(filepath.Join(dir, recentFile), []byte(recentContent), 0644); err != nil {
		t.Fatal(err)
	}

	// UKI antigo (timestamp = agora - 200 dias)
	oldTS := time.Now().Add(-200 * 24 * time.Hour).Unix()
	oldFile := fmt.Sprintf("UKI-%d-antigo.md", oldTS)
	oldContent := `# Documento Antigo
**ID:** UKI-ANTIGO
**Type:** Decision
**Status:** Draft

## Content
Este documento é antigo e não tem atividade git.
`
	if err := os.WriteFile(filepath.Join(dir, oldFile), []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeDecayWithRunner_DetectaStale(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsDecay(t, ukiDir)

	// Mock: nenhuma atividade git (retorna erro)
	mock := &MockRunner{Responses: map[string]string{}}

	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("esperado 2 infos, obtido %d", len(infos))
	}

	// O documento antigo (>90 dias) deve ser stale
	stale := FindStale(infos)
	if len(stale) == 0 {
		t.Error("esperado ao menos 1 UKI stale")
	}

	// Verificar que o recente não é stale
	for _, info := range infos {
		if info.Title == "Documento Recente" && info.IsStale {
			t.Error("documento recente não deveria ser stale")
		}
	}
}

func TestAnalyzeDecayWithRunner_ComAtividadeGit(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	criarUKIsDecay(t, ukiDir)

	// Mock: atividade git recente para todos os arquivos
	recentDate := time.Now().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05 -0700")
	entries, _ := os.ReadDir(ukiDir)
	responses := make(map[string]string)
	for _, e := range entries {
		responses[filepath.Join(ukiDir, e.Name())] = recentDate
	}

	mock := &MockRunner{Responses: responses}

	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	stale := FindStale(infos)
	if len(stale) != 0 {
		t.Errorf("nenhum UKI deveria ser stale com atividade git recente, obtido %d", len(stale))
	}
}

func TestAnalyzeDecayWithRunner_DiretorioInexistente(t *testing.T) {
	mock := &MockRunner{}
	_, err := AnalyzeDecayWithRunner("/caminho/inexistente", "/tmp", mock)
	if err == nil {
		t.Error("esperado erro para diretório inexistente")
	}
}

func TestFindStale_Filtra(t *testing.T) {
	infos := []DecayInfo{
		{Title: "Ativo", IsStale: false},
		{Title: "Obsoleto", IsStale: true},
		{Title: "Também Ativo", IsStale: false},
	}

	stale := FindStale(infos)
	if len(stale) != 1 {
		t.Errorf("esperado 1 stale, obtido %d", len(stale))
	}
	if stale[0].Title != "Obsoleto" {
		t.Errorf("esperado 'Obsoleto', obtido %q", stale[0].Title)
	}
}

func TestAnalyzeDecayWithRunner_ExtraiTimestampDoFilename(t *testing.T) {
	dir := t.TempDir()
	ukiDir := filepath.Join(dir, "ukis")
	if err := os.MkdirAll(ukiDir, 0755); err != nil {
		t.Fatal(err)
	}

	targetDate := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ts := targetDate.Unix()
	filename := fmt.Sprintf("UKI-%d-teste.md", ts)
	content := "# Teste\nConteúdo.\n"
	if err := os.WriteFile(filepath.Join(ukiDir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mock := &MockRunner{Responses: map[string]string{}}
	infos, err := AnalyzeDecayWithRunner(ukiDir, dir, mock)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("esperado 1 info, obtido %d", len(infos))
	}

	if infos[0].CreatedAt.IsZero() {
		t.Error("CreatedAt não deveria ser zero")
	}
	if infos[0].CreatedAt.Year() != targetDate.Year() {
		t.Errorf("esperado ano %d, obtido %d", targetDate.Year(), infos[0].CreatedAt.Year())
	}
}
