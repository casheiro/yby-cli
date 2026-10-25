package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestMarca_EscolhePeloEspaco(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	casos := []struct {
		nome            string
		largura, altura int
		linhas, maxLarg int
	}{
		{"grande", 80, 0, 12, 64},
		{"grande na medida", 64, 12, 12, 64},
		{"compacta por largura", 63, 0, 8, 42},
		{"compacta por altura", 120, 11, 8, 42},
		{"texto", 41, 0, 1, 5},
		{"texto por altura", 120, 7, 1, 5},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			linhas := strings.Split(Marca(c.largura, c.altura), "\n")
			if len(linhas) != c.linhas {
				t.Fatalf("esperava %d linhas, veio %d", c.linhas, len(linhas))
			}
			maior := 0
			for _, l := range linhas {
				maior = max(maior, ansi.StringWidth(l))
			}
			if maior != c.maxLarg {
				t.Errorf("esperava largura %d, veio %d", c.maxLarg, maior)
			}
		})
	}
}

func TestMarca_SoBlocosOuASCII(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, s := range []string{Marca(80, 0), Marca(50, 0)} {
		for _, r := range s {
			if r != ' ' && r != '\n' && r != '▀' && r != '▄' && r != '█' {
				t.Fatalf("caractere fora dos meio-blocos: %q", r)
			}
		}
	}
	usarTema(t, true, termenv.Ascii, true)
	for _, r := range Marca(80, 0) {
		if r > 127 {
			t.Fatalf("modo ASCII com caractere não ASCII: %q", r)
		}
	}
}

func TestMarca_Golden(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	conferirGolden(t, "marca_grande_escuro", Marca(80, 0))
	usarTema(t, false, termenv.TrueColor, false)
	conferirGolden(t, "marca_compacta_claro", Marca(50, 0))
	usarTema(t, true, termenv.Ascii, true)
	conferirGolden(t, "marca_traco_ascii", Marca(80, 0))
	conferirGolden(t, "marca_texto_ascii", Marca(20, 0))
}

func TestLogo(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)

	var b bytes.Buffer
	Logo(&b)
	if b.Len() != 0 {
		t.Errorf("fora de um terminal não deveria escrever nada, veio %q", b.String())
	}

	fixarLargura(t, 70)
	Logo(&b)
	linhas := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(linhas) != 13 || linhas[12] != "" {
		t.Fatalf("esperava a grande (12 linhas) e uma linha em branco, veio %d linhas", len(linhas))
	}
	for _, l := range linhas[:12] {
		if !strings.HasPrefix(l, "  ") {
			t.Errorf("linha sem recuo de 2: %q", l)
		}
	}
}
