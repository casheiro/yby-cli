package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestQuadroGolden(t *testing.T) {
	corpo := "nome      api-7f9c-x2k\nnamespace payments\numa linha bem comprida que precisa ser truncada"
	etapasPorModo(t, "quadro_normal", func() string { return Quadro("detalhe · pod", corpo, 40, QuadroNormal) })
	etapasPorModo(t, "quadro_foco", func() string { return Quadro("detalhe · pod", corpo, 40, QuadroFoco) })
	etapasPorModo(t, "quadro_destrutivo", func() string { return Quadro("excluir pod", "digite api", 40, QuadroDestrutivo) })
}

func TestQuadroGeometria(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, w := range []int{4, 8, 20, 50} {
		for _, rotulo := range []string{"", "curto", "um rótulo extremamente longo para o fio"} {
			saida := Quadro(rotulo, "ação\n\ncoração de açúcar muito longo para a caixa", w, QuadroNormal)
			etapasLarguraMax(t, saida, max(w, larguraMinQuadro))
			linhas := strings.Split(saida, "\n")
			// Todas as linhas têm a mesma largura (borda reta).
			for _, l := range linhas {
				if got, want := lipgloss.Width(l), lipgloss.Width(linhas[0]); got != want {
					t.Errorf("w=%d rótulo=%q: linha torta (%d != %d): %q", w, rotulo, got, want, l)
				}
			}
		}
	}
}

// QuadroComNome usa o separador do modo: "·" no Unicode (igual ao QuadroNomeado) e "/" no
// ASCII; o QuadroNomeado guarda o "·" no ASCII, como as telas do Viz.
func TestQuadroComNome(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	unicode := QuadroComNome("aks", "aks-dados", "conta  x", 40, QuadroFoco)
	if unicode != QuadroNomeado("aks", "aks-dados", "conta  x", 40, QuadroFoco) {
		t.Errorf("no Unicode deveria ser igual ao QuadroNomeado:\n%s", unicode)
	}
	Configurar(Opcoes{Tema: TemaEscuro, ASCII: true})
	topo := strings.Split(QuadroComNome("aks", "aks-dados", "conta  x", 40, QuadroFoco), "\n")[0]
	if !strings.HasPrefix(topo, "+ AKS / aks-dados -") {
		t.Errorf("fio de cima no ASCII: %q", topo)
	}
	if antigo := strings.Split(QuadroNomeado("pod", "api", "x", 30, QuadroFoco), "\n")[0]; !strings.Contains(antigo, "POD · api") {
		t.Errorf("o QuadroNomeado mudou no ASCII: %q", antigo)
	}
	if sem, comum := QuadroComNome("logs", "", "a", 30, QuadroNormal), Quadro("logs", "a", 30, QuadroNormal); sem != comum {
		t.Errorf("sem nome deveria ser o Quadro comum:\n%s\n%s", sem, comum)
	}
}

// TestQuadroNomeado: o rótulo vai em caixa-alta e o nome fica como veio; sem
// nome, é o Quadro comum; a borda continua reta com nome comprido.
func TestQuadroNomeado(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	topo := strings.Split(QuadroNomeado("pod", "worker-5d8b6-q7tvn", "status  Running", 40, QuadroFoco), "\n")[0]
	if !strings.HasPrefix(topo, "┌ POD · worker-5d8b6-q7tvn ─") {
		t.Errorf("fio de cima: %q", topo)
	}
	if sem, comum := QuadroNomeado("logs", "", "a", 30, QuadroNormal), Quadro("logs", "a", 30, QuadroNormal); sem != comum {
		t.Errorf("sem nome deveria ser o Quadro comum:\n%s\n%s", sem, comum)
	}
	saida := QuadroNomeado("pod", strings.Repeat("nome-comprido-", 6), "x", 30, QuadroNormal)
	for _, l := range strings.Split(saida, "\n") {
		if w := lipgloss.Width(l); w != 30 {
			t.Errorf("linha com %d células: %q", w, l)
		}
	}
}

// TestQuadroSuave: o fio e o corpo são os do QuadroNormal; só o rótulo muda, para
// TintaSuave sem negrito, como o do QuadroComTecla (legível sobre o fundo escuro).
func TestQuadroSuave(t *testing.T) {
	etapasPorModo(t, "quadro_suave", func() string { return Quadro("diagnóstico", "o worker não acha o banco", 40, QuadroSuave) })

	usarTema(t, true, termenv.TrueColor, false)
	suave := strings.Split(Quadro("diagnóstico", "a\nb", 40, QuadroSuave), "\n")
	normal := strings.Split(Quadro("diagnóstico", "a\nb", 40, QuadroNormal), "\n")
	if strings.Join(suave[1:], "\n") != strings.Join(normal[1:], "\n") {
		t.Errorf("o corpo e o fio de baixo são os do QuadroNormal:\n%s\n%s", strings.Join(suave, "\n"), strings.Join(normal, "\n"))
	}
	if !strings.Contains(suave[0], S().TintaSuave.Render("DIAGNÓSTICO")) || strings.Contains(suave[0], "\x1b[1;") {
		t.Errorf("o rótulo sai em TintaSuave, sem negrito: %q", suave[0])
	}
	if !strings.HasPrefix(suave[0], S().Fio.Render("┌ ")) {
		t.Errorf("o canto e o fio continuam em Fio: %q", suave[0])
	}
}
