package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestQuadroAberto_Texto(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := QuadroAberto("comando", "ctrl+y", "copia", "kubectl get pods\n\n  -n web", 40)
	linhas := strings.Split(saida, "\n")
	esperado := []string{
		"─ COMANDO · ctrl+y copia ───────────────",
		"  kubectl get pods",
		"",
		"    -n web",
		"────────────────────────────────────────",
	}
	if strings.Join(linhas, "\n") != strings.Join(esperado, "\n") {
		t.Fatalf("QuadroAberto:\n%s\nesperado:\n%s", saida, strings.Join(esperado, "\n"))
	}
	for _, l := range []string{linhas[0], linhas[4]} {
		if w := lipgloss.Width(l); w != 40 {
			t.Errorf("os fios ocupam a largura toda: %d células em %q", w, l)
		}
	}
}

// Sem espaço para a tecla, fica só o rótulo; o rótulo longo demais é cortado. No modo
// ASCII, o fio é de hífens.
func TestQuadroAberto_Estreito(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	saida := QuadroAberto("comando", "ctrl+y", "copia", "ls", 16)
	topo := strings.Split(saida, "\n")[0]
	if topo != "- COMANDO ------" {
		t.Errorf("topo estreito: %q", topo)
	}
	topo = strings.Split(QuadroAberto("um rótulo bem comprido", "", "", "ls", 12), "\n")[0]
	if w := lipgloss.Width(topo); w > 12 {
		t.Errorf("o rótulo longo passa da largura: %d células em %q", w, topo)
	}
}
