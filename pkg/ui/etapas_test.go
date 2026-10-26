package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// etapasModo é um modo visual de teste: tema, perfil de cor e ASCII.
type etapasModo struct {
	nome   string
	escuro bool
	perfil termenv.Profile
	ascii  bool
}

var etapasModos = []etapasModo{
	{"escuro", true, termenv.TrueColor, false},
	{"claro", false, termenv.TrueColor, false},
	{"ascii", true, termenv.Ascii, true},
}

// etapasPorModo roda f em cada modo visual e confere o golden "<prefixo>_<modo>".
func etapasPorModo(t *testing.T, prefixo string, f func() string) {
	t.Helper()
	for _, m := range etapasModos {
		t.Run(m.nome, func(t *testing.T) {
			usarTema(t, m.escuro, m.perfil, m.ascii)
			conferirGolden(t, prefixo+"_"+m.nome, f())
		})
	}
}

// etapasLarguraMax confere que nenhuma linha passa da largura.
func etapasLarguraMax(t *testing.T, s string, largura int) {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if w := lipgloss.Width(l); w > largura {
			t.Errorf("linha com %d células (máx %d): %q", w, largura, l)
		}
	}
}

func TestFormatarDuracao(t *testing.T) {
	casos := map[time.Duration]string{
		-time.Second:                         "0,0 s",
		0:                                    "0,0 s",
		300 * time.Millisecond:               "0,3 s",
		2100 * time.Millisecond:              "2,1 s",
		9990 * time.Millisecond:              "10 s",
		38 * time.Second:                     "38 s",
		38600 * time.Millisecond:             "39 s",
		60 * time.Second:                     "1 min",
		72 * time.Second:                     "1 min 12 s",
		time.Hour + 5*time.Second:            "60 min 5 s",
		9*time.Second + 940*time.Millisecond: "9,9 s",
	}
	for d, esperado := range casos {
		if got := FormatarDuracao(d); got != esperado {
			t.Errorf("FormatarDuracao(%v) = %q, esperado %q", d, got, esperado)
		}
	}
}

func etapasExemplo() []Etapa {
	return []Etapa{
		{Rotulo: "contexto do projeto", Estado: EtapaFeita, Duracao: 300 * time.Millisecond},
		{Rotulo: "kubectl get pvc -A", Estado: EtapaAtual, Duracao: 2100 * time.Millisecond},
		{Rotulo: "resposta", Estado: EtapaPendente, Duracao: 0},
		{Rotulo: "gravar relatório", Estado: EtapaFalha, Duracao: 72 * time.Second},
	}
}

func TestEtapasGolden(t *testing.T) {
	etapasPorModo(t, "etapas", func() string { return Etapas(etapasExemplo(), 40) })
}

func TestEtapasLarguras(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	for _, w := range []int{16, 20, 40} {
		etapasLarguraMax(t, Etapas(etapasExemplo(), w), w)
	}
	// Largura natural e sem tempos.
	if got := Etapas([]Etapa{{Rotulo: "a", Estado: EtapaPendente, Duracao: 0}}, 0); !strings.Contains(got, "o a") {
		t.Errorf("saída inesperada: %q", got)
	}
	if Etapas(nil, 10) != "" {
		t.Error("lista vazia deve render vazio")
	}
	etapasLarguraMax(t, Etapas([]Etapa{{Rotulo: "árvore de decisão ✓ longa", Estado: EtapaFeita, Duracao: time.Second}}, 14), 14)
}

func TestCotaGolden(t *testing.T) {
	etapasPorModo(t, "cota", func() string {
		return strings.Join([]string{
			Cota("argo cd", 0.64, 40),
			Cota("prometheus", 1, 40),
			CotaTempo("sync", 0.5, 38*time.Second, 40),
			CotaTempo("sync", 0.9, 4*time.Second, 40),
		}, "\n")
	})
}

func TestCotaLarguras(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	for _, w := range []int{16, 30, 60} {
		etapasLarguraMax(t, Cota("argo cd", 0.3, w), w)
	}
	etapasLarguraMax(t, Cota("um rótulo muito comprido mesmo", 0.3, 24), 24)
	if got := Cota("x", -1, 0); !strings.Contains(got, "0%") {
		t.Errorf("fração negativa deve virar 0%%: %q", got)
	}
	if got := Cota("x", 2, 20); !strings.Contains(got, "100%") {
		t.Errorf("fração > 1 deve virar 100%%: %q", got)
	}
}

func TestRotaGolden(t *testing.T) {
	rotulos := []string{"projeto", "cluster", "argo cd", "secrets", "apps", "revisão"}
	etapasPorModo(t, "rota", func() string {
		return Rota(rotulos, 2, 0) + "\n\n" + Rota(rotulos, 5, 60) + "\n\n" + Rota(rotulos, 0, 40)
	})
}

func TestRotaLarguras(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	rotulos := []string{"projeto", "cluster", "argo cd", "secrets", "apps", "revisão"}
	for _, w := range []int{24, 40, 60} {
		etapasLarguraMax(t, Rota(rotulos, 2, w), w)
	}
	if Rota(nil, 0, 10) != "" {
		t.Error("rota vazia deve render vazio")
	}
	if got := Rota(rotulos, 99, 0); !strings.Contains(got, "06/06") {
		t.Errorf("índice deve ser limitado: %q", got)
	}
	if got := Rota(rotulos, -3, 0); !strings.Contains(got, "01/06") {
		t.Errorf("índice deve ser limitado: %q", got)
	}
	if got := Rota([]string{"único"}, 0, 20); !strings.Contains(got, "01/01") {
		t.Errorf("um marco: %q", got)
	}
}

func TestSpinnerParado(t *testing.T) {
	semTTY := func() bool { return false }
	comTTY := func() bool { return true }

	t.Run("fora de TTY", func(t *testing.T) {
		usarTema(t, true, termenv.Ascii, false)
		m := NovoSpinnerCom("lendo pods", semTTY)
		if m.Animado() || m.Init() != nil {
			t.Fatal("fora de TTY o spinner não deve animar")
		}
		antes := m.View()
		novo, cmd := m.Update(SpinnerTickMsg{id: m.id})
		if cmd != nil || novo.View() != antes {
			t.Error("tick não deve alterar o spinner parado")
		}
		if !strings.Contains(antes, "◈ lendo pods") {
			t.Errorf("quadro fixo ◈ esperado: %q", antes)
		}
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		usarTema(t, true, termenv.TrueColor, false)
		Configurar(Opcoes{Tema: TemaEscuro, SemCor: true})
		m := NovoSpinnerCom("x", comTTY)
		if m.Animado() || m.Init() != nil {
			t.Fatal("com NO_COLOR o spinner não deve animar")
		}
		if !strings.HasPrefix(m.View(), "◈") {
			t.Errorf("quadro fixo ◈ esperado: %q", m.View())
		}
	})

	t.Run("ASCII parado", func(t *testing.T) {
		usarTema(t, true, termenv.Ascii, true)
		m := NovoSpinnerCom("", semTTY)
		if m.View() != "+" {
			t.Errorf("quadro ASCII fixo esperado: %q", m.View())
		}
	})
}

func TestSpinnerAnimado(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	m := NovoSpinnerCom("a", func() bool { return true })
	if !m.Animado() || m.Init() == nil {
		t.Fatal("em TTY com cor o spinner deve animar")
	}
	var vistos []string
	var modelo tea.Model = m
	for i := 0; i < 4; i++ {
		vistos = append(vistos, modelo.View())
		modelo, _ = modelo.Update(SpinnerTickMsg{id: m.id})
	}
	if got := strings.Join(vistos, "|"); got != "◇ a|◈ a|◆ a|◈ a" {
		t.Errorf("sequência de quadros: %s", got)
	}
	// Tick de outro spinner é ignorado.
	antes := modelo.View()
	modelo, _ = modelo.Update(SpinnerTickMsg{id: m.id + 100})
	if modelo.View() != antes {
		t.Error("tick alheio não deve avançar")
	}
	if got := m.DefinirRotulo("b").View(); !strings.HasSuffix(got, " b") {
		t.Errorf("rótulo: %q", got)
	}
	if ttyStdout() {
		t.Log("stdout é TTY neste ambiente")
	}
	if NovoSpinner("z").View() == "" {
		t.Error("view vazia")
	}
}
