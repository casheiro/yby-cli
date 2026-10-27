package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ukisExemplo() []UKIFile {
	return []UKIFile{
		{
			Path:    "/tmp/UKI-1234-plugin-arch.md",
			Title:   "Arquitetura de Plugins",
			Content: "# Arquitetura de Plugins\n\nConteúdo sobre [Protocolo](UKI-1234-protocol.md).\n",
			Tags:    []string{"kubernetes", "plugins"},
		},
		{
			Path:    "/tmp/UKI-1234-protocol.md",
			Title:   "Protocolo de Comunicação",
			Content: "# Protocolo de Comunicação\n\nDetalhes do protocolo JSON.\n",
			Tags:    []string{"protocol", "json"},
		},
	}
}

func TestNewExporter_FormatosValidos(t *testing.T) {
	formatos := []string{"docusaurus", "obsidian", "markdown"}
	for _, f := range formatos {
		exp, err := NewExporter(f)
		if err != nil {
			t.Errorf("erro para formato %q: %v", f, err)
		}
		if exp == nil {
			t.Errorf("exportador nil para formato %q", f)
		}
	}
}

func TestNewExporter_FormatoInvalido(t *testing.T) {
	_, err := NewExporter("pdf")
	if err == nil {
		t.Error("esperado erro para formato inválido")
	}
}

func TestDocusaurusExporter_AdicionaFrontmatter(t *testing.T) {
	dir := t.TempDir()
	exp := &DocusaurusExporter{}
	ukis := ukisExemplo()

	if err := exp.Export(ukis, dir); err != nil {
		t.Fatalf("erro na exportação: %v", err)
	}

	// Verificar que o arquivo foi criado em docs/
	outPath := filepath.Join(dir, "docs", "UKI-1234-plugin-arch.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("arquivo não encontrado: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		t.Error("esperado frontmatter YAML no início")
	}
	if !strings.Contains(content, "title:") {
		t.Error("frontmatter deve conter title")
	}
	if !strings.Contains(content, "sidebar_position:") {
		t.Error("frontmatter deve conter sidebar_position")
	}
	if !strings.Contains(content, "- kubernetes") {
		t.Error("frontmatter deve conter tags")
	}
}

func TestObsidianExporter_ConverteWikilinks(t *testing.T) {
	dir := t.TempDir()
	exp := &ObsidianExporter{}
	ukis := ukisExemplo()

	if err := exp.Export(ukis, dir); err != nil {
		t.Fatalf("erro na exportação: %v", err)
	}

	outPath := filepath.Join(dir, "UKI-1234-plugin-arch.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("arquivo não encontrado: %v", err)
	}

	content := string(data)
	// Link markdown deve ter sido convertido para wikilink
	if !strings.Contains(content, "[[UKI-1234-protocol]]") {
		t.Errorf("esperado wikilink [[UKI-1234-protocol]], obtido:\n%s", content)
	}
	// Não deve conter link markdown original
	if strings.Contains(content, "[Protocolo](UKI-1234-protocol.md)") {
		t.Error("link markdown original não deveria existir após conversão")
	}
	// Deve ter frontmatter
	if !strings.HasPrefix(content, "---\n") {
		t.Error("esperado frontmatter YAML")
	}
}

func TestMarkdownExporter_CriaIndice(t *testing.T) {
	dir := t.TempDir()
	exp := &MarkdownExporter{}
	ukis := ukisExemplo()

	if err := exp.Export(ukis, dir); err != nil {
		t.Fatalf("erro na exportação: %v", err)
	}

	// Verificar que README.md foi criado
	readmePath := filepath.Join(dir, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("README.md não encontrado: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Arquitetura de Plugins") {
		t.Error("índice deve conter título do primeiro UKI")
	}
	if !strings.Contains(content, "Protocolo de Comunicação") {
		t.Error("índice deve conter título do segundo UKI")
	}

	// Verificar que arquivos foram copiados sem modificação
	uki1Path := filepath.Join(dir, "UKI-1234-plugin-arch.md")
	uki1Data, err := os.ReadFile(uki1Path)
	if err != nil {
		t.Fatalf("UKI não encontrado: %v", err)
	}
	if string(uki1Data) != ukis[0].Content {
		t.Error("conteúdo do UKI não deveria ter sido modificado")
	}
}

func TestLoadUKIs_CarregaArquivos(t *testing.T) {
	dir := t.TempDir()

	content := "# Teste\nConteúdo.\n"
	if err := os.WriteFile(filepath.Join(dir, "UKI-1.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Arquivo não-md deve ser ignorado
	if err := os.WriteFile(filepath.Join(dir, "notas.txt"), []byte("ignorar"), 0644); err != nil {
		t.Fatal(err)
	}

	ukis, err := LoadUKIs(dir)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(ukis) != 1 {
		t.Errorf("esperado 1 UKI, obtido %d", len(ukis))
	}
	if ukis[0].Title != "Teste" {
		t.Errorf("título esperado 'Teste', obtido %q", ukis[0].Title)
	}
}

func TestLoadUKIs_DiretorioInexistente(t *testing.T) {
	_, err := LoadUKIs("/caminho/inexistente")
	if err == nil {
		t.Error("esperado erro para diretório inexistente")
	}
}

// TestLoadUKIs_SubpastasETags: o UKI de uma subpasta de domínio também entra, com as tags
// do frontmatter e o caminho relativo à pasta dos UKIs.
func TestLoadUKIs_SubpastasETags(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dominio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := "---\nid: d.b\ntipo: regra\nstatus: ativo\ntags: [k8s, deploy]\n---\n\n# B\n"
	if err := os.WriteFile(filepath.Join(dir, "dominio", "B.md"), []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	ukis, err := LoadUKIs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ukis) != 2 {
		t.Fatalf("esperados 2 UKIs, obtido %d", len(ukis))
	}
	if ukis[1].Relativo != "dominio/B.md" || ukis[1].Title != "B" || strings.Join(ukis[1].Tags, ",") != "k8s,deploy" {
		t.Errorf("UKI da subpasta inesperado: %+v", ukis[1])
	}
}

// TestExport_UmFrontmatterSo: o UKI que já tem frontmatter sai com um bloco --- só, no
// Docusaurus e no Obsidian, e o da subpasta não colide com o de mesmo nome na raiz.
func TestExport_UmFrontmatterSo(t *testing.T) {
	conteudo := "---\nid: d.b\ntipo: regra\nstatus: ativo\ntags: [k8s]\n---\n\n# Regra B\n\nTexto.\n"
	ukis := []UKIFile{
		{Path: "/tmp/B.md", Relativo: "B.md", Title: "Raiz", Content: "# Raiz\n"},
		{Path: "/tmp/dominio/B.md", Relativo: "dominio/B.md", Title: "Regra B", Content: conteudo, Tags: []string{"k8s"}},
	}
	for formato, pasta := range map[string]string{"docusaurus": "docs", "obsidian": ""} {
		t.Run(formato, func(t *testing.T) {
			out := t.TempDir()
			exp, _ := NewExporter(formato)
			if err := exp.Export(ukis, out); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(out, pasta, "dominio", "B.md"))
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(data), "---\n"); n != 2 {
				t.Errorf("esperado um frontmatter só (2 linhas ---), obtido %d:\n%s", n, data)
			}
			if !strings.Contains(string(data), "  - k8s\n") || !strings.Contains(string(data), "# Regra B\n\nTexto.\n") {
				t.Errorf("faltam as tags ou o corpo:\n%s", data)
			}
			if _, err := os.Stat(filepath.Join(out, pasta, "B.md")); err != nil {
				t.Errorf("o UKI da raiz sumiu: %v", err)
			}
		})
	}
}
