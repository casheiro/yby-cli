package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func filtroChipsExemplo() []Chip {
	return []Chip{{"status", "com problema"}, {Chave: "app=api"}, {"ns", "payments"}}
}

func TestFiltros_Golden(t *testing.T) {
	casos := []struct {
		nome    string
		escuro  bool
		perfil  termenv.Profile
		ascii   bool
		largura int
	}{
		{"filtro_escuro_80", true, termenv.TrueColor, false, 80},
		{"filtro_claro_80", false, termenv.TrueColor, false, 80},
		{"filtro_escuro_40_ocultos", true, termenv.TrueColor, false, 40},
		{"filtro_ascii_80", true, termenv.Ascii, true, 80},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			saida := Filtros(filtroChipsExemplo(), 8, 42, c.largura)
			tabsNenhumaLinhaPassa(t, saida, c.largura)
			conferirGolden(t, c.nome, saida)
		})
	}
}

func TestFiltros_LarguraNuncaExcede(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, n := range []int{1, 8, 15, 25, 40, 60, 100} {
		tabsNenhumaLinhaPassa(t, Filtros(filtroChipsExemplo(), 8, 42, n), n)
		tabsNenhumaLinhaPassa(t, Filtros(nil, 0, 0, n), n)
		tabsNenhumaLinhaPassa(t, Filtros(filtroChipsExemplo(), 8, 0, n), n)
	}
}

func TestFiltros_ContadorAlinhadoADireita(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := Filtros(filtroChipsExemplo(), 8, 42, 80)
	if !strings.HasSuffix(saida, "8 de 42") {
		t.Errorf("contador deveria terminar a linha: %q", saida)
	}
	if lipgloss.Width(saida) != 80 {
		t.Errorf("largura = %d, esperado 80", lipgloss.Width(saida))
	}
}

func TestFiltros_SemFiltrosESemTotal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if got := Filtros(nil, 0, 0, 80); !strings.Contains(got, "nenhum") || strings.Contains(got, " de ") {
		t.Errorf("saída inesperada: %q", got)
	}
	if got := Filtros(nil, 3, 3, 80); !strings.HasSuffix(got, "3 de 3") {
		t.Errorf("saída inesperada: %q", got)
	}
	if got := Filtros(filtroChipsExemplo(), 1, 1, 0); got != "" {
		t.Errorf("largura 0 deveria ser vazio: %q", got)
	}
}

func TestFiltros_ChipsOcultosViramContagem(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := Filtros(filtroChipsExemplo(), 8, 42, 45)
	if !strings.Contains(saida, "+") {
		t.Errorf("esperava indicador +N: %q", saida)
	}
	// Nenhum chip cabe: só o total de ocultos.
	saida = Filtros(filtroChipsExemplo(), 8, 42, 24)
	if !strings.Contains(saida, "+3") {
		t.Errorf("esperava +3: %q", saida)
	}
}
