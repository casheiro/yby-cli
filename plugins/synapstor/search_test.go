package main

import (
	"strings"
	"testing"
)

// TestParseSearchArgs_QuerySimples verifica extração de query sem flags.
func TestParseSearchArgs_QuerySimples(t *testing.T) {
	query, topK := parseSearchArgs([]string{"kubernetes", "deploy"})
	if query != "kubernetes deploy" {
		t.Errorf("query esperada 'kubernetes deploy', obtida %q", query)
	}
	if topK != defaultTopK {
		t.Errorf("topK esperado %d, obtido %d", defaultTopK, topK)
	}
}

// TestParseSearchArgs_ComTopK verifica extração de --top-k separado.
func TestParseSearchArgs_ComTopK(t *testing.T) {
	query, topK := parseSearchArgs([]string{"argo", "cd", "--top-k", "10"})
	if query != "argo cd" {
		t.Errorf("query esperada 'argo cd', obtida %q", query)
	}
	if topK != 10 {
		t.Errorf("topK esperado 10, obtido %d", topK)
	}
}

// TestParseSearchArgs_ComTopKInline verifica extração de --top-k=N.
func TestParseSearchArgs_ComTopKInline(t *testing.T) {
	query, topK := parseSearchArgs([]string{"helm", "--top-k=3", "charts"})
	if query != "helm charts" {
		t.Errorf("query esperada 'helm charts', obtida %q", query)
	}
	if topK != 3 {
		t.Errorf("topK esperado 3, obtido %d", topK)
	}
}

// TestParseSearchArgs_TopKInvalido verifica que --top-k inválido usa padrão.
func TestParseSearchArgs_TopKInvalido(t *testing.T) {
	query, topK := parseSearchArgs([]string{"busca", "--top-k", "abc"})
	if query != "busca" {
		t.Errorf("query esperada 'busca', obtida %q", query)
	}
	if topK != defaultTopK {
		t.Errorf("topK esperado %d (padrão), obtido %d", defaultTopK, topK)
	}
}

// TestParseSearchArgs_TopKZero verifica que --top-k=0 usa padrão.
func TestParseSearchArgs_TopKZero(t *testing.T) {
	_, topK := parseSearchArgs([]string{"busca", "--top-k", "0"})
	if topK != defaultTopK {
		t.Errorf("topK esperado %d (padrão para valor <= 0), obtido %d", defaultTopK, topK)
	}
}

// TestParseSearchArgs_SemArgs verifica query vazia sem argumentos.
func TestParseSearchArgs_SemArgs(t *testing.T) {
	query, topK := parseSearchArgs([]string{})
	if query != "" {
		t.Errorf("query esperada vazia, obtida %q", query)
	}
	if topK != defaultTopK {
		t.Errorf("topK esperado %d, obtido %d", defaultTopK, topK)
	}
}

// TestFormatSearchResults_ComResultados verifica a formatação com resultados.
func TestFormatSearchResults_ComResultados(t *testing.T) {
	results := []searchResult{
		{Index: 1, Score: 0.95, Title: "Deploy com Argo CD", Preview: "Como configurar deploy..."},
		{Index: 2, Score: 0.82, Title: "Helm Charts", Preview: "Criando charts personalizados..."},
	}

	output := formatSearchResults("deploy", results)

	if !strings.Contains(output, `"deploy"`) {
		t.Error("saída deve conter a query")
	}
	if !strings.Contains(output, "2 encontrados") {
		t.Error("saída deve indicar quantidade de resultados")
	}
	if !strings.Contains(output, "[1] (0.95) Deploy com Argo CD") {
		t.Error("saída deve conter primeiro resultado formatado")
	}
	if !strings.Contains(output, "[2] (0.82) Helm Charts") {
		t.Error("saída deve conter segundo resultado formatado")
	}
	if !strings.Contains(output, "Como configurar deploy...") {
		t.Error("saída deve conter preview do primeiro resultado")
	}
}

// TestFormatSearchResults_SemResultados verifica mensagem quando não há resultados.
func TestFormatSearchResults_SemResultados(t *testing.T) {
	output := formatSearchResults("inexistente", nil)

	if !strings.Contains(output, "Nenhum resultado") {
		t.Error("saída deve indicar ausência de resultados")
	}
	if !strings.Contains(output, `"inexistente"`) {
		t.Error("saída deve conter a query pesquisada")
	}
}

// TestTruncatePreview_TextoCurto verifica que texto curto não é truncado.
func TestTruncatePreview_TextoCurto(t *testing.T) {
	result := truncatePreview("texto curto", 200)
	if result != "texto curto" {
		t.Errorf("esperado 'texto curto', obtido %q", result)
	}
}

// TestTruncatePreview_TextoLongo verifica que texto longo é truncado com "...".
func TestTruncatePreview_TextoLongo(t *testing.T) {
	longText := strings.Repeat("a", 300)
	result := truncatePreview(longText, 200)

	if len(result) != 203 { // 200 + "..."
		t.Errorf("esperado 203 caracteres, obtido %d", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("texto truncado deve terminar com '...'")
	}
}

// TestTruncatePreview_RemoveQuebrasLinha verifica remoção de newlines.
func TestTruncatePreview_RemoveQuebrasLinha(t *testing.T) {
	result := truncatePreview("linha1\nlinha2\nlinha3", 200)
	if strings.Contains(result, "\n") {
		t.Error("preview não deve conter quebras de linha")
	}
	if result != "linha1 linha2 linha3" {
		t.Errorf("esperado 'linha1 linha2 linha3', obtido %q", result)
	}
}
