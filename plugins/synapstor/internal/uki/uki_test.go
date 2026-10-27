package uki

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func gravar(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestListar_DesceNasSubpastas: um UKI na raiz e outro numa subpasta de domínio contam
// os dois; o que não é .md e as pastas ocultas ficam de fora.
func TestListar_DesceNasSubpastas(t *testing.T) {
	pasta := t.TempDir()
	gravar(t, filepath.Join(pasta, "A.md"), "# A")
	gravar(t, filepath.Join(pasta, "dominio", "B.md"), "# B")
	gravar(t, filepath.Join(pasta, "dominio", "notas.txt"), "x")
	gravar(t, filepath.Join(pasta, ".obsidian", "C.md"), "# C")

	arquivos, err := Listar(pasta)
	if err != nil {
		t.Fatal(err)
	}
	var relativos []string
	for _, a := range arquivos {
		relativos = append(relativos, a.Relativo)
		if a.Caminho != filepath.Join(pasta, filepath.FromSlash(a.Relativo)) {
			t.Errorf("caminho %q não bate com o relativo %q", a.Caminho, a.Relativo)
		}
	}
	if !reflect.DeepEqual(relativos, []string{"A.md", "dominio/B.md"}) {
		t.Errorf("esperado A.md e dominio/B.md, obtido %v", relativos)
	}
}

func TestListar_PastaInexistente(t *testing.T) {
	if _, err := Listar(filepath.Join(t.TempDir(), "nada")); err == nil {
		t.Error("esperado erro para pasta inexistente")
	}
}

// TestLer_Formatos: o frontmatter em PT-BR, o da especificação antiga (em inglês), o dos
// UKIs do repositório (uki_id e version) e o formato antigo em negrito dão os mesmos
// metadados, normalizados para PT-BR.
func TestLer_Formatos(t *testing.T) {
	casos := []struct {
		nome, conteudo string
		esperado       Metadados
		corpo          string
	}{
		{
			nome: "frontmatter em PT-BR",
			conteudo: "---\nid: cli.ux.saida-json\ntipo: decisao\nstatus: rascunho\ncriado_em: 2026-10-01\n" +
				"atualizado_em: 2026-10-20\ntags: [cli, saida]\n---\n\n# Saída em JSON\n\n## Problema\n",
			esperado: Metadados{ID: "cli.ux.saida-json", Tipo: TipoDecisao, Status: StatusRascunho,
				CriadoEm: dia(2026, 10, 1), AtualizadoEm: dia(2026, 10, 20), Tags: []string{"cli", "saida"},
				Titulo: "Saída em JSON", Frontmatter: true},
			corpo: "# Saída em JSON\n\n## Problema\n",
		},
		{
			nome: "frontmatter da especificação em inglês",
			conteudo: "---\nuki_id: cli.ux.saida-json\ntype: decision\nstatus: proposal\ncreated_at: 2026-10-01\n" +
				"updated_at: \"2026-10-20\"\ntags:\n  - cli\n  - saida\n---\n# Saída em JSON\n",
			esperado: Metadados{ID: "cli.ux.saida-json", Tipo: TipoDecisao, Status: StatusRascunho,
				CriadoEm: dia(2026, 10, 1), AtualizadoEm: dia(2026, 10, 20), Tags: []string{"cli", "saida"},
				Titulo: "Saída em JSON", Frontmatter: true},
			corpo: "# Saída em JSON\n",
		},
		{
			nome:     "frontmatter dos UKIs do repositório",
			conteudo: "---\r\nuki_id: UKI_TECH_STACK\r\nversion: 1.0.0\r\nstatus: active\r\ntags: [stack, tools]\r\n---\r\n\r\n# Pilha\r\n",
			esperado: Metadados{ID: "UKI_TECH_STACK", Status: StatusAtivo, Tags: []string{"stack", "tools"},
				Titulo: "Pilha", Frontmatter: true},
			corpo: "# Pilha\r\n",
		},
		{
			nome:     "formato antigo em negrito",
			conteudo: "# Deploy\n**ID:** UKI-DEPLOY\n**Tipo:** Decisão\n**Status:** Rascunho\n\n## Contexto\n",
			esperado: Metadados{ID: "UKI-DEPLOY", Tipo: TipoDecisao, Status: StatusRascunho, Titulo: "Deploy"},
			corpo:    "# Deploy\n**ID:** UKI-DEPLOY\n**Tipo:** Decisão\n**Status:** Rascunho\n\n## Contexto\n",
		},
		{
			nome:     "formato antigo do study, em inglês",
			conteudo: "# Plugins\n**ID:** UKI-1-plugins\n**Type:** Reference\n**Status:** Active\n",
			esperado: Metadados{ID: "UKI-1-plugins", Tipo: TipoReferencia, Status: StatusAtivo, Titulo: "Plugins"},
			corpo:    "# Plugins\n**ID:** UKI-1-plugins\n**Type:** Reference\n**Status:** Active\n",
		},
		{
			nome:     "sem metadados",
			conteudo: "# Rascunho\n\nanotar depois",
			esperado: Metadados{Titulo: "Rascunho"},
			corpo:    "# Rascunho\n\nanotar depois",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			meta, corpo := Ler(c.conteudo)
			if !reflect.DeepEqual(meta, c.esperado) {
				t.Errorf("metadados:\nesperado %+v\nobtido   %+v", c.esperado, meta)
			}
			if corpo != c.corpo {
				t.Errorf("corpo: esperado %q, obtido %q", c.corpo, corpo)
			}
		})
	}
}

// TestLer_SoFrontmatterTemMetadados: um UKI só com o frontmatter da especificação tem os
// metadados completos (o quality procurava só o **ID:** em negrito).
func TestLer_SoFrontmatterTemMetadados(t *testing.T) {
	meta, _ := Ler("---\nuki_id: a.b.c\ntype: rule\nstatus: active\n---\n# A\n")
	if !meta.Completos() {
		t.Errorf("esperado metadados completos, obtido %+v", meta)
	}
	if meta, _ := Ler("---\nuki_id: a\nstatus: active\n---\n# A\n"); meta.Completos() {
		t.Errorf("sem tipo, os metadados não estão completos: %+v", meta)
	}
}

// TestLer_FrontmatterInvalido: um bloco --- com YAML quebrado sai do corpo (não vira um
// segundo frontmatter no export), mas não dá metadados.
func TestLer_FrontmatterInvalido(t *testing.T) {
	meta, corpo := Ler("---\nid: [quebrado\n---\n# A\n")
	if corpo != "# A\n" || meta.ID != "" || !meta.Frontmatter {
		t.Errorf("obtido %+v e corpo %q", meta, corpo)
	}
	// sem o fecho, não é frontmatter
	if _, corpo := Ler("---\nid: a\n# A\n"); corpo != "---\nid: a\n# A\n" {
		t.Errorf("sem fecho, o conteúdo fica inteiro: %q", corpo)
	}
}

func TestNormalizarTipoEStatus(t *testing.T) {
	tipos := map[string]string{
		"regra": TipoRegra, "Rule": TipoRegra, "decisão": TipoDecisao, "decision": TipoDecisao,
		"Padrão": TipoPadrao, "pattern": TipoPadrao, "métrica": TipoMetrica, "metric": TipoMetrica,
		"Referência": TipoReferencia, "reference": TipoReferencia, " Procedimento ": "procedimento", "": "",
	}
	for entrada, esperado := range tipos {
		if obtido := NormalizarTipo(entrada); obtido != esperado {
			t.Errorf("NormalizarTipo(%q) = %q, esperado %q", entrada, obtido, esperado)
		}
	}
	if TipoValido("procedimento") || !TipoValido(TipoMetrica) {
		t.Error("TipoValido aceita só os tipos da especificação")
	}
	status := map[string]string{
		"ativo": StatusAtivo, "Active": StatusAtivo, "proposal": StatusRascunho, "draft": StatusRascunho,
		"Rascunho": StatusRascunho, "proposta": StatusRascunho, "deprecated": StatusObsoleto, "obsoleto": StatusObsoleto,
	}
	for entrada, esperado := range status {
		if obtido := NormalizarStatus(entrada); obtido != esperado {
			t.Errorf("NormalizarStatus(%q) = %q, esperado %q", entrada, obtido, esperado)
		}
	}
}

// TestMontar_IdaEVolta: o UKI montado pelo código tem um frontmatter só, no formato da
// especificação, e é lido de volta com os mesmos metadados.
func TestMontar_IdaEVolta(t *testing.T) {
	meta := Metadados{ID: "cli.ux.saida-json", Tipo: TipoPadrao, Status: StatusRascunho,
		CriadoEm: dia(2026, 10, 26), AtualizadoEm: dia(2026, 10, 26), Tags: []string{"cli", "json"}}
	conteudo := Montar(meta, "\n\n# Saída em JSON\n\n## Quando usar\n")
	esperado := "---\nid: cli.ux.saida-json\ntipo: padrao\nstatus: rascunho\ncriado_em: 2026-10-26\n" +
		"atualizado_em: 2026-10-26\ntags: [cli, json]\n---\n\n# Saída em JSON\n\n## Quando usar\n"
	if conteudo != esperado {
		t.Fatalf("esperado:\n%s\nobtido:\n%s", esperado, conteudo)
	}
	lido, corpo := Ler(conteudo)
	meta.Titulo, meta.Frontmatter = "Saída em JSON", true
	if !reflect.DeepEqual(lido, meta) || corpo != "# Saída em JSON\n\n## Quando usar\n" {
		t.Errorf("ida e volta: %+v / %q", lido, corpo)
	}
	if strings.Count(Montar(Metadados{}, "# A"), "---") != 2 {
		t.Error("sem metadados, o frontmatter continua um bloco só")
	}
}

func TestLimparIDETags(t *testing.T) {
	if obtido := LimparID(" CLI/UX: Saída em JSON "); obtido != "cli-ux-saida-em-json" {
		t.Errorf("LimparID = %q", obtido)
	}
	if obtido := LimparID("cli.ux.saida-json"); obtido != "cli.ux.saida-json" {
		t.Errorf("LimparID mexeu num id válido: %q", obtido)
	}
	obtido := LimparTags([]string{" Kubernetes ", "ci cd", "kubernetes", "", "a,b", "[x]"})
	if !reflect.DeepEqual(obtido, []string{"kubernetes", "ci-cd", "ab", "x"}) {
		t.Errorf("LimparTags = %v", obtido)
	}
}

func dia(a int, m time.Month, d int) time.Time {
	return time.Date(a, m, d, 0, 0, 0, 0, time.UTC)
}
