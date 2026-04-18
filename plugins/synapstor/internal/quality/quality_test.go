package quality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScoreUKI_DocumentoCompleto(t *testing.T) {
	content := `# Arquitetura de Plugins
**ID:** UKI-PLUGIN-ARCH
**Type:** Reference
**Status:** Active

## Context
Documentação da arquitetura de plugins do Yby CLI.

## Architecture
O sistema de plugins usa processos separados comunicando via JSON.

## Code References
- ` + "`pkg/plugin/manager.go`" + ` — descoberta e instalação
- ` + "`pkg/plugin/executor.go`" + ` — execução e comunicação

` + "```go" + `
func (m *Manager) Discover() ([]Plugin, error) {
    // Escaneia diretórios de plugins
}
` + "```" + `

Veja também [Protocolo](UKI-1234-protocol.md) para detalhes.

Este componente é essencial para a extensibilidade do CLI, permitindo que
terceiros desenvolvam funcionalidades sem modificar o core. A comunicação
é feita via stdin/stdout com payloads JSON, seguindo o protocolo definido
no documento referenciado. Cada plugin é um binário independente que
implementa os hooks necessários conforme seu manifesto. O manager é
responsável pela descoberta automática e o executor pela comunicação.
Palavras extras para completar contagem adequada de palavras no teste.
`

	score := ScoreUKI(content)

	if score.Title != "Arquitetura de Plugins" {
		t.Errorf("título esperado 'Arquitetura de Plugins', obtido %q", score.Title)
	}

	if !score.Breakdown.HasContext {
		t.Error("esperado HasContext = true")
	}
	if !score.Breakdown.HasExamples {
		t.Error("esperado HasExamples = true")
	}
	if !score.Breakdown.HasHeaders {
		t.Error("esperado HasHeaders = true")
	}
	if !score.Breakdown.HasMetadata {
		t.Error("esperado HasMetadata = true")
	}
	if score.Breakdown.LinkCount == 0 {
		t.Error("esperado LinkCount > 0")
	}
	if score.Score < 70 {
		t.Errorf("esperado score >= 70 para documento completo, obtido %d", score.Score)
	}
}

func TestScoreUKI_DocumentoMinimo(t *testing.T) {
	content := `# Nota rápida
Texto curto.
`
	score := ScoreUKI(content)

	if score.Score > 20 {
		t.Errorf("esperado score <= 20 para documento mínimo, obtido %d", score.Score)
	}
	if score.Breakdown.HasContext {
		t.Error("não deveria ter contexto")
	}
	if score.Breakdown.HasMetadata {
		t.Error("não deveria ter metadata")
	}
}

func TestScoreUKI_PontuacaoHeaders(t *testing.T) {
	// Documento com muitos headers
	content := `# Título
## Header 1
texto
## Header 2
texto
## Header 3
texto
## Header 4
texto
## Header 5
texto
`
	score := ScoreUKI(content)

	if score.Breakdown.HeaderCount != 5 {
		t.Errorf("esperado 5 headers, obtido %d", score.Breakdown.HeaderCount)
	}
	// 10 base + 2*2 extra = 14
	// Score total deve incluir pontos de headers
	if score.Score < 14 {
		t.Errorf("esperado score >= 14 com 5 headers, obtido %d", score.Score)
	}
}

func TestScoreUKI_WordCount(t *testing.T) {
	tests := []struct {
		name     string
		words    int
		minScore int
	}{
		{"menos de 100 palavras", 50, 0},
		{"mais de 100 palavras", 150, 10},
		{"mais de 300 palavras", 350, 15},
		{"mais de 500 palavras", 550, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Gerar conteúdo com N palavras
			words := make([]string, tt.words)
			for i := range words {
				words[i] = "palavra"
			}
			content := "# T\n" + joinWords(words)

			score := ScoreUKI(content)
			if score.Score < tt.minScore {
				t.Errorf("esperado score >= %d para %d palavras, obtido %d", tt.minScore, tt.words, score.Score)
			}
		})
	}
}

func joinWords(words []string) string {
	result := ""
	for i, w := range words {
		if i > 0 {
			result += " "
		}
		result += w
	}
	return result
}

func TestScoreAll_EscaneiaUKIs(t *testing.T) {
	dir := t.TempDir()

	uki1 := `# Doc Completo
**ID:** UKI-1
**Type:** Reference
**Status:** Active

## Context
Contexto aqui.
`
	uki2 := `# Doc Simples
Texto simples.
`

	if err := os.WriteFile(filepath.Join(dir, "UKI-1.md"), []byte(uki1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UKI-2.md"), []byte(uki2), 0644); err != nil {
		t.Fatal(err)
	}
	// Arquivo não-md deve ser ignorado
	if err := os.WriteFile(filepath.Join(dir, "notas.txt"), []byte("ignorar"), 0644); err != nil {
		t.Fatal(err)
	}

	scores, err := ScoreAll(dir)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(scores) != 2 {
		t.Errorf("esperado 2 scores, obtido %d", len(scores))
	}
}

func TestScoreAll_DiretorioInexistente(t *testing.T) {
	_, err := ScoreAll("/caminho/inexistente")
	if err == nil {
		t.Error("esperado erro para diretório inexistente")
	}
}

func TestFormatScore(t *testing.T) {
	qs := QualityScore{
		Path:  "/tmp/UKI-1.md",
		Title: "Teste",
		Score: 75,
	}

	result := FormatScore(qs)
	if result == "" {
		t.Error("resultado não deve ser vazio")
	}
	if !contains(result, "75") {
		t.Error("resultado deve conter o score")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
