package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"
)

func ajudaExemplo() Ajuda {
	return Ajuda{
		Comando:   "yby env create",
		Descricao: "Cria um novo ambiente e gera o values correspondente.\n\nO ambiente entra em .yby/environments.yaml.",
		Uso:       []string{"yby env create [nome] [flags]"},
		Apelidos:  []string{"novo"},
		Exemplos:  "  yby env create qa --type remote\n  # Cria config/values-qa.yaml\n",
		Flags: []Termo{
			{"    --description texto", "Descrição do ambiente"},
			{"    --uma-flag-com-nome-muito-comprido texto", "desce para a linha de baixo"},
			{"-h, --help", "mostra esta ajuda"},
		},
		Globais: []Termo{{"    --log-level texto", "Nível de log (debug, info, warn, error) (padrão info)"}},
		Rodape:  "yby env create --help mostra os detalhes",
	}
}

func TestEscreverAjuda_Golden(t *testing.T) {
	fixarLargura(t, 64)
	usarTema(t, true, termenv.TrueColor, false)
	var b bytes.Buffer
	EscreverAjuda(&b, ajudaExemplo())
	conferirGolden(t, "ajuda_escuro", b.String())

	usarTema(t, true, termenv.Ascii, true)
	b.Reset()
	EscreverAjuda(&b, ajudaExemplo())
	conferirGolden(t, "ajuda_ascii", b.String())
}

func TestEscreverAjuda_Partes(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	EscreverAjuda(&b, ajudaExemplo())
	saida := b.String()
	for _, esperado := range []string{
		"◆ YBY  ENV CREATE", "USO\n  yby env create [nome] [flags]\n  também yby env novo",
		"EXEMPLOS\n  yby env create qa --type remote\n  # Cria config/values-qa.yaml",
		"FLAGS GLOBAIS", "\n  yby env create --help mostra os detalhes\n",
		"  -h, --help               mostra esta ajuda",
		"  Cria um novo ambiente e gera o values correspondente.\n\n  O ambiente entra",
	} {
		if !strings.Contains(saida, esperado) {
			t.Errorf("faltou %q em:\n%s", esperado, saida)
		}
	}
	// nome mais largo que a coluna fica sozinho e a descrição desce
	if !strings.Contains(saida, "texto\n                           desce para a linha de baixo") {
		t.Errorf("nome comprido deveria ficar sozinho na linha:\n%s", saida)
	}
	for _, l := range strings.Split(saida, "\n") {
		if l != strings.TrimRight(l, " ") {
			t.Errorf("linha com espaço à direita: %q", l)
		}
	}
}

func TestEscreverAjuda_RaizSemTituloESemSecoesVazias(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	EscreverAjuda(&b, Ajuda{Comando: "yby", Descricao: "Assistente", Uso: []string{"yby <comando>"}, SemTitulo: true})
	saida := b.String()
	if strings.Contains(saida, "YBY") || strings.Contains(saida, "COMANDOS") || strings.Contains(saida, "FLAGS") {
		t.Errorf("sem título nem seções vazias, veio:\n%s", saida)
	}

	b.Reset()
	EscreverAjuda(&b, Ajuda{Comando: "yby"})
	if primeira := strings.SplitN(b.String(), "\n", 2)[0]; primeira != "* YBY  AJUDA" && primeira != "◆ YBY  AJUDA" {
		t.Errorf("título da raiz = %q", primeira)
	}
}

func TestExemplos_TiraRecuoComum(t *testing.T) {
	got := exemplos("\n    yby a\n      # recuado\n    yby b  \n\n")
	want := []string{"yby a", "  # recuado", "yby b"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("exemplos = %q", got)
	}
	if exemplos("  \n") != nil {
		t.Error("bloco vazio deveria devolver nil")
	}
}

func TestEscreverAjuda_ExemploLongoQuebraComContinuacao(t *testing.T) {
	fixarLargura(t, 40)
	usarTema(t, true, termenv.Ascii, false)
	var b bytes.Buffer
	EscreverAjuda(&b, Ajuda{Comando: "yby env create", Exemplos: "yby env create qa --type remote --description \"Quality Assurance\"\n# Cria config/values-qa.yaml e adiciona o ambiente"})
	saida := b.String()
	if !strings.Contains(saida, "\n  yby env create qa --type remote\n    --description \"Quality Assurance\"\n") {
		t.Errorf("comando longo deveria continuar recuado:\n%s", saida)
	}
	if !strings.Contains(saida, "\n  # Cria config/values-qa.yaml e\n  # adiciona o ambiente\n") {
		t.Errorf("comentário longo deveria continuar com #:\n%s", saida)
	}
}
