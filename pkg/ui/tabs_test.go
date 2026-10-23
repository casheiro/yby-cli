package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// tabsNenhumaLinhaPassa falha se alguma linha de s exceder largura células.
func tabsNenhumaLinhaPassa(t *testing.T, s string, largura int) {
	t.Helper()
	for i, l := range strings.Split(s, "\n") {
		if w := lipgloss.Width(l); w > largura {
			t.Errorf("linha %d tem %d células (> %d): %q", i+1, w, largura, l)
		}
	}
}

func tabsAbasExemplo() []Aba {
	return []Aba{{"Pods", 8}, {"Deployments", 4}, {"StatefulSets", 1}, {"Services", 12}, {"Ingresses", 3}}
}

func TestAbas_Golden(t *testing.T) {
	casos := []struct {
		nome    string
		escuro  bool
		perfil  termenv.Profile
		ascii   bool
		largura int
		ativa   int
	}{
		{"tabs_escuro_120", true, termenv.TrueColor, false, 120, 0},
		{"tabs_claro_120", false, termenv.TrueColor, false, 120, 1},
		{"tabs_escuro_60_mais", true, termenv.TrueColor, false, 60, 3},
		{"tabs_escuro_compacto_50", true, termenv.TrueColor, false, 50, 1},
		{"tabs_ascii_70", true, termenv.Ascii, true, 70, 2},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			saida := Abas(tabsAbasExemplo(), c.ativa, c.largura)
			tabsNenhumaLinhaPassa(t, saida, c.largura)
			conferirGolden(t, c.nome, saida)
		})
	}
}

func TestAbas_GoldenMais(t *testing.T) {
	abas := append(tabsAbasExemplo(), Aba{"ConfigMaps", 7}, Aba{"PersistentVolumes", 2})
	usarTema(t, true, termenv.TrueColor, false)
	conferirGolden(t, "tabs_escuro_80_mais", Abas(abas, 5, 80))
	usarTema(t, false, termenv.TrueColor, false)
	conferirGolden(t, "tabs_claro_80_mais", Abas(abas, 0, 80))
}

func TestAbas_LarguraNuncaExcede(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, n := range []int{1, 5, 10, 20, 30, 40, 60, 79, 80, 100} {
		for ativa := -1; ativa <= 5; ativa++ {
			tabsNenhumaLinhaPassa(t, Abas(tabsAbasExemplo(), ativa, n), n)
		}
	}
}

func TestAbas_AtivaSempreVisivel(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	abas := append(tabsAbasExemplo(), Aba{"ConfigMaps", 7}, Aba{"PersistentVolumes", 2})
	for ativa := range abas {
		saida := Abas(abas, ativa, 80)
		if !strings.Contains(saida, "mais") {
			t.Errorf("ativa=%d: esperava indicador de mais em %q", ativa, saida)
		}
		if !strings.Contains(saida, abas[ativa].Titulo) {
			t.Errorf("ativa=%d: aba ativa fora da janela: %q", ativa, saida)
		}
	}
}

func TestAbas_CabemTodasSemMais(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := Abas(tabsAbasExemplo(), 0, 200)
	if strings.Contains(saida, "mais") {
		t.Errorf("não esperava 'mais': %q", saida)
	}
}

func TestAbas_CompactoNumerado(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := Abas([]Aba{{"Pods", 8}, {"Deployments", 4}}, 0, 30)
	if !strings.Contains(saida, "1 Pods") || !strings.Contains(saida, "2 Deplo") {
		t.Errorf("modo compacto inesperado: %q", saida)
	}
}

func TestAbas_SemContagemEVazio(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if got := Abas(nil, 0, 80); got != "" {
		t.Errorf("sem abas deveria ser vazio: %q", got)
	}
	if got := Abas(tabsAbasExemplo(), 0, 0); got != "" {
		t.Errorf("largura 0 deveria ser vazio: %q", got)
	}
	saida := Abas([]Aba{{"Logs", -1}}, 0, 80)
	if strings.Contains(saida, "-1") {
		t.Errorf("contagem negativa deve ser oculta: %q", saida)
	}
}

func TestAbas_Acentuadas(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	abas := []Aba{{"Configurações", 2}, {"Serviços", 5}, {"日本語のタブ", 1}}
	for _, n := range []int{20, 40, 70, 120} {
		tabsNenhumaLinhaPassa(t, Abas(abas, 2, n), n)
	}
}
