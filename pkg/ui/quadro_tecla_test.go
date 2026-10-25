package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestQuadroComTecla_Golden(t *testing.T) {
	casos := []struct {
		nome          string
		escuro, ascii bool
		perfil        termenv.Profile
		v             VarianteQuadro
	}{
		{"escuro", true, false, termenv.TrueColor, QuadroNormal},
		{"claro", false, false, termenv.TrueColor, QuadroNormal},
		{"ascii", true, true, termenv.Ascii, QuadroNormal},
		{"foco", true, false, termenv.TrueColor, QuadroFoco},
		{"destrutivo", true, false, termenv.TrueColor, QuadroDestrutivo},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			saida := QuadroComTecla("comando", "c", "copia", "kubectl delete pv pvc-3f1a9c2e pvc-91cd04b7", 60, c.v)
			for i, l := range strings.Split(saida, "\n") {
				if w := lipgloss.Width(l); w != 60 {
					t.Errorf("linha %d com %d células: %q", i+1, w, l)
				}
			}
			conferirGolden(t, "quadro_tecla_"+c.nome, saida)
		})
	}
}

func TestQuadroComTecla_Texto(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := QuadroComTecla("comando", "c", "copia", "kubectl get pods", 40, QuadroNormal)
	if !strings.HasPrefix(saida, "┌ COMANDO · c copia ─") {
		t.Errorf("rótulo em caixa-alta e tecla como veio: %q", strings.Split(saida, "\n")[0])
	}
	if strings.Count(saida, "\n") != 2 || !strings.Contains(saida, "│ kubectl get pods") {
		t.Errorf("corpo do quadro: %q", saida)
	}
}

func TestQuadroComTecla_SemEspacoViraQuadro(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, c := range []struct{ rotulo, tecla string }{{"comando", ""}, {"", "c"}, {"comando muito comprido", "c"}} {
		got := QuadroComTecla(c.rotulo, c.tecla, "copia", "ls", 24, QuadroNormal)
		if want := Quadro(c.rotulo, "ls", 24, QuadroNormal); got != want {
			t.Errorf("%+v: deveria cair no Quadro comum:\n%s\n%s", c, got, want)
		}
	}
}
