package indexer

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
)

func TestDividirEmTrechos_UmPorSecaoH2(t *testing.T) {
	seccao := func(n string) string { return "## Seção " + n + "\n" + strings.Repeat("palavra ", 125) + "\n\n" }
	conteudo := "# Meu UKI\n\nIntrodução do documento.\n\n" + seccao("A") + seccao("B") + seccao("C")
	trechos := dividirEmTrechos("Meu UKI", conteudo)
	if len(trechos) != 4 {
		t.Fatalf("esperava 4 trechos (introdução + 3 seções), veio %d", len(trechos))
	}
	for i, nome := range []string{"", "Seção A", "Seção B", "Seção C"} {
		if trechos[i].Secao != nome {
			t.Errorf("trecho %d: seção %q, esperava %q", i, trechos[i].Secao, nome)
		}
	}
	if !strings.HasPrefix(trechos[2].Texto, "# Meu UKI\n## Seção B\n") {
		t.Errorf("o trecho tem de começar com título e seção, veio %q", trechos[2].Texto[:30])
	}
}

func TestDividirEmTrechos_SecaoLongaTemSobreposicao(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("frase número ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString(". ")
	}
	trechos := dividirEmTrechos("Longo", "# Longo\n\n## Parte\n"+b.String())
	if len(trechos) < 3 {
		t.Fatalf("seção de %d caracteres devia virar vários trechos, veio %d", b.Len(), len(trechos))
	}
	for i, tr := range trechos {
		if n := len([]rune(tr.Texto)); n > tamanhoTrecho+60 {
			t.Errorf("trecho %d com %d runas, passa do tamanho", i, n)
		}
		if i == 0 {
			continue
		}
		cauda := trechos[i-1].Texto[len(trechos[i-1].Texto)-40:]
		if !strings.Contains(tr.Texto, strings.TrimSpace(cauda)[len(strings.TrimSpace(cauda))-20:]) {
			t.Errorf("trecho %d não repete o fim do anterior", i)
		}
	}
}

func TestDividirEmTrechos_NaoPartePalavraDeRuna(t *testing.T) {
	trechos := dividirEmTrechos("", "## S\n"+strings.Repeat("ç", 4000))
	for _, tr := range trechos {
		if !utf8.ValidString(tr.Texto) {
			t.Fatal("trecho com UTF-8 inválido")
		}
	}
}

func TestDividirEmTrechos_VazioENaoTemTrecho(t *testing.T) {
	if got := dividirEmTrechos("x", "  \n\n"); len(got) != 0 {
		t.Errorf("documento vazio gerou %d trechos", len(got))
	}
	if got := dividirEmTrechos("", "só texto sem título"); len(got) != 1 {
		t.Errorf("texto sem H2 devia ser um trecho, veio %d", len(got))
	}
}

func uki3Secoes(corpo string) string {
	return "# Grande\n\n## A\n" + corpo + "\n\n## B\n" + corpo + "\n\n## C\n" + corpo + "\n"
}

func idsNoIndice(t *testing.T, raiz string, p ai.Provider) []string {
	t.Helper()
	vs, err := ai.NewVectorStore(context.Background(), filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := vs.Search(context.Background(), "x", vs.Count())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, d := range docs {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestRun_UKIGrandeViraVariosDocumentos(t *testing.T) {
	ctx := context.Background()
	raiz := t.TempDir()
	dir := filepath.Join(raiz, ".synapstor", ".uki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	corpo := strings.Repeat("o deploy passa pelo argo. ", 40)
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte(uki3Secoes(corpo)), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	rel, err := NewIndexer(p, raiz).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rel.EmbeddingsCreated != 3 {
		t.Errorf("EmbeddingsCreated = %d, esperava 3 trechos", rel.EmbeddingsCreated)
	}
	want := []string{".synapstor/.uki/x.md#0", ".synapstor/.uki/x.md#1", ".synapstor/.uki/x.md#2"}
	if got := idsNoIndice(t, raiz, p); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, esperava %v", got, want)
	}

	// o arquivo encolhe de 3 para 1 trecho: #1 e #2 não podem ficar órfãos
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte("# Grande\n\nAgora curto.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if got := idsNoIndice(t, raiz, p); !reflect.DeepEqual(got, want[:1]) {
		t.Fatalf("depois de encolher, IDs = %v, esperava %v", got, want[:1])
	}

	// nada mudou: segunda rodada pula por hash e o índice continua batendo com o manifest
	rel, err = NewIndexer(p, raiz).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rel.FilesSkipped != 1 || rel.EmbeddingsCreated != 0 {
		t.Errorf("esperava pular o arquivo, veio %+v", rel)
	}
}

func TestScanFiles_IgnoraRodapeELateral(t *testing.T) {
	raiz := t.TempDir()
	wiki := filepath.Join(raiz, "docs", "wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Home.md", "_Footer.md", "_Sidebar.md"} {
		if err := os.WriteFile(filepath.Join(wiki, n), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := NewIndexer(nil, raiz).scanFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "Home.md" {
		t.Errorf("esperava só Home.md, veio %v", files)
	}
}
