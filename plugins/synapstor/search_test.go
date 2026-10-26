package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// TestParseSearchArgs_QuerySimples verifica extração de query sem flags.
func TestParseSearchArgs_QuerySimples(t *testing.T) {
	query, topK, err := parseSearchArgs([]string{"kubernetes", "deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if query != "kubernetes deploy" {
		t.Errorf("query esperada 'kubernetes deploy', obtida %q", query)
	}
	if topK != defaultTopK {
		t.Errorf("topK esperado %d, obtido %d", defaultTopK, topK)
	}
}

// TestParseSearchArgs_ComTopK verifica extração de --top-k separado.
func TestParseSearchArgs_ComTopK(t *testing.T) {
	query, topK, err := parseSearchArgs([]string{"argo", "cd", "--top-k", "10"})
	if err != nil {
		t.Fatal(err)
	}
	if query != "argo cd" {
		t.Errorf("query esperada 'argo cd', obtida %q", query)
	}
	if topK != 10 {
		t.Errorf("topK esperado 10, obtido %d", topK)
	}
}

// TestParseSearchArgs_ComTopKInline verifica extração de --top-k=N.
func TestParseSearchArgs_ComTopKInline(t *testing.T) {
	query, topK, err := parseSearchArgs([]string{"helm", "--top-k=3", "charts"})
	if err != nil {
		t.Fatal(err)
	}
	if query != "helm charts" {
		t.Errorf("query esperada 'helm charts', obtida %q", query)
	}
	if topK != 3 {
		t.Errorf("topK esperado 3, obtido %d", topK)
	}
}

// TestParseSearchArgs_TopKInvalido verifica que --top-k sem número válido é erro de
// validação com hint, em vez de cair no padrão calado.
func TestParseSearchArgs_TopKInvalido(t *testing.T) {
	for _, args := range [][]string{
		{"busca", "--top-k", "abc"},
		{"busca", "--top-k", "0"},
		{"busca", "--top-k=-2"},
		{"busca", "--top-k"},
	} {
		_, _, err := parseSearchArgs(args)
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || ye.GetHint() == "" {
			t.Errorf("%v: esperado ERR_VALIDATION com hint, obtido %v", args, err)
		}
	}
}

// TestParseSearchArgs_SemArgs verifica query vazia sem argumentos.
func TestParseSearchArgs_SemArgs(t *testing.T) {
	query, topK, err := parseSearchArgs([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if query != "" {
		t.Errorf("query esperada vazia, obtida %q", query)
	}
	if topK != defaultTopK {
		t.Errorf("topK esperado %d, obtido %d", defaultTopK, topK)
	}
}

// TestTruncatePreview_TextoCurto verifica que texto curto não é truncado.
func TestTruncatePreview_TextoCurto(t *testing.T) {
	result := truncatePreview("texto curto", 200)
	if result != "texto curto" {
		t.Errorf("esperado 'texto curto', obtido %q", result)
	}
}

// TestTruncatePreview_TextoLongo verifica que texto longo é truncado com reticências,
// contando runas (um acento não parte no meio).
func TestTruncatePreview_TextoLongo(t *testing.T) {
	longText := strings.Repeat("ç", 300)
	result := truncatePreview(longText, 200)

	if n := len([]rune(result)); n != 201 { // 200 + "…"
		t.Errorf("esperado 201 runas, obtido %d", n)
	}
	if !strings.HasSuffix(result, "…") {
		t.Error("texto truncado deve terminar com '…'")
	}
	if strings.ContainsRune(result, '�') {
		t.Error("o corte partiu uma runa")
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

// TestTruncatePreview_TiraFrontmatter verifica que o frontmatter YAML não entra.
func TestTruncatePreview_TiraFrontmatter(t *testing.T) {
	result := truncatePreview("---\ntitle: x\n---\n# Título\n## Seção\ncorpo **forte**\n```bash\nargocd app sync\n```", 200)
	if result != "Seção corpo forte argocd app sync" {
		t.Errorf("obtido %q", result)
	}
}

// TestPontuacao_ComVirgula verifica a pontuação no formato brasileiro.
func TestPontuacao_ComVirgula(t *testing.T) {
	if p := pontuacao(0.8249); p != "0,82" {
		t.Errorf("obtido %q", p)
	}
}

// TestResultadosDaBusca_JuntaTrechosDoMesmoArquivo: o índice guarda trechos, mas a busca
// mostra cada UKI uma vez, com o trecho de maior pontuação.
func TestResultadosDaBusca_JuntaTrechosDoMesmoArquivo(t *testing.T) {
	docs := []ai.UnknownDocument{
		{ID: "a.md#1", Score: 0.9, Content: "melhor", Metadata: map[string]string{"source": "a.md", "title": "A", "section": "Deploy"}},
		{ID: "b.md#0", Score: 0.7, Content: "outro", Metadata: map[string]string{"source": "b.md", "title": "B"}},
		{ID: "a.md#0", Score: 0.6, Content: "pior", Metadata: map[string]string{"source": "a.md", "title": "A"}},
		{ID: "a.md#2", Score: 0.2, Content: "fraco", Metadata: map[string]string{"source": "a.md"}},
		{ID: "c.md#0", Score: 0.1, Content: "longe", Metadata: map[string]string{"source": "c.md"}},
		{ID: "c.md#1", Score: 0.05, Content: "longe também", Metadata: map[string]string{"source": "c.md"}},
	}
	results, fora := resultadosDaBusca(docs, 5)
	if len(results) != 2 || results[0].Path != "a.md" || results[1].Path != "b.md" {
		t.Fatalf("esperava a.md e b.md, veio %+v", results)
	}
	if results[0].Preview != "melhor" || results[0].Title != "A · Deploy" {
		t.Errorf("a.md devia vir pelo trecho de maior pontuação, veio %+v", results[0])
	}
	if fora != 1 {
		t.Errorf("fora do corte = %d, esperava 1", fora)
	}
}

// TestResultadosDaBusca_TrechosDeUmArquivoNaoComemAsVagas: os primeiros topK trechos são do
// mesmo UKI e a busca ainda chega a topK UKIs distintos, cortando o excedente.
func TestResultadosDaBusca_TrechosDeUmArquivoNaoComemAsVagas(t *testing.T) {
	var docs []ai.UnknownDocument
	for i := 0; i < 3; i++ {
		docs = append(docs, ai.UnknownDocument{ID: fmt.Sprintf("a.md#%d", i), Score: 0.9 - float32(i)*0.01, Metadata: map[string]string{"source": "a.md"}})
	}
	for _, f := range []string{"b.md", "c.md", "d.md"} {
		docs = append(docs, ai.UnknownDocument{ID: f + "#0", Score: 0.6, Metadata: map[string]string{"source": f}})
	}
	results, _ := resultadosDaBusca(docs, 3)
	if len(results) != 3 || results[0].Path != "a.md" || results[1].Path != "b.md" || results[2].Path != "c.md" {
		t.Fatalf("esperava a, b e c, veio %+v", results)
	}
}
