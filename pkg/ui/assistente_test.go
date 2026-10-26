package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/muesli/termenv"
)

// executarCmd roda cmd e devolve as mensagens que ele produz, abrindo tea.Batch e
// tea.Sequence. Comandos que demoram (o cursor piscando) são descartados.
func executarCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(30 * time.Millisecond):
		return nil
	}
	if msg == nil {
		return nil
	}
	// tea.BatchMsg e o sequenceMsg (não exportado) são listas de comandos
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var msgs []tea.Msg
		for i := 0; i < v.Len(); i++ {
			c, _ := v.Index(i).Interface().(tea.Cmd)
			msgs = append(msgs, executarCmd(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// conduzirFila entrega as mensagens ao modelo, uma de cada vez, junto com as que os
// comandos devolvidos produzirem.
func conduzirFila(m tea.Model, fila []tea.Msg) {
	for i := 0; len(fila) > 0 && i < 1000; i++ {
		msg := fila[0]
		fila = fila[1:]
		_, cmd := m.Update(msg)
		fila = append(fila, executarCmd(cmd)...)
	}
}

// iniciarAssistente roda o Init e entrega o tamanho da janela, como o Bubble Tea faria.
func iniciarAssistente(a *Assistente) {
	conduzirFila(a, append(executarCmd(a.Init()), tea.WindowSizeMsg{Width: 64, Height: 30}))
}

// teclar entrega cada tecla (e o que ela desencadear) ao assistente.
func teclar(a *Assistente, teclas ...tea.Msg) {
	for _, t := range teclas {
		conduzirFila(a, []tea.Msg{t})
	}
}

var (
	teclaEnter   = tea.KeyMsg{Type: tea.KeyEnter}
	teclaVoltar  = tea.KeyMsg{Type: tea.KeyShiftTab}
	teclaEsc     = tea.KeyMsg{Type: tea.KeyEsc}
	teclaBaixo   = tea.KeyMsg{Type: tea.KeyDown}
	teclaApagar  = tea.KeyMsg{Type: tea.KeyCtrlU}
	teclaEspaco  = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	digitarTexto = func(s string) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
)

// respostasTeste guarda os valores ligados aos campos do assistente de teste.
type respostasTeste struct {
	nome      string
	tipo      string
	extras    []string
	detalhar  bool
	detalhe   string
	confirmar bool
}

// assistenteTeste monta um assistente de 4 marcos: NOME, TIPO, EXTRAS (com um passo
// oculto conforme a resposta) e FIM.
func assistenteTeste(r *respostasTeste) *Assistente {
	r.confirmar = true
	return NovoAssistente(64,
		PassoAssistente{Marco: "nome", Campos: []huh.Field{
			huh.NewInput().Key("nome").Title("Nome do projeto").Placeholder("yby-project").Value(&r.nome),
		}},
		PassoAssistente{Marco: "vazio"},
		PassoAssistente{Marco: "tipo", Campos: []huh.Field{
			huh.NewSelect[string]().Key("tipo").Title("Topologia").
				Options(huh.NewOptions("single", "standard", "complete")...).Value(&r.tipo),
		}},
		PassoAssistente{Marco: "extras", Campos: []huh.Field{
			huh.NewMultiSelect[string]().Key("extras").Title("Módulos").
				Options(huh.NewOptions("kepler", "minio")...).Value(&r.extras),
			huh.NewConfirm().Key("detalhar").Title("Detalhar?").Affirmative("Sim").Negative("Não").Value(&r.detalhar),
		}},
		PassoAssistente{Marco: "extras", Ocultar: func() bool { return !r.detalhar }, Campos: []huh.Field{
			huh.NewInput().Key("detalhe").Title("Detalhe").Value(&r.detalhe),
		}},
		PassoAssistente{Marco: "fim", Campos: []huh.Field{
			huh.NewConfirm().Key("confirmar").Title("Gerar?").Affirmative("Gerar").Negative("Cancelar").Value(&r.confirmar),
		}},
	)
}

func TestAssistenteMarcosEChaves(t *testing.T) {
	var r respostasTeste
	a := assistenteTeste(&r)
	if got := strings.Join(a.Marcos(), ","); got != "nome,tipo,extras,fim" {
		t.Errorf("marcos = %q; passo sem campos fica de fora e passos seguidos dividem o marco", got)
	}
	if got := strings.Join(a.Chaves(), ","); got != "nome,tipo,extras,detalhar,detalhe,confirmar" {
		t.Errorf("chaves = %q", got)
	}
	if a.Vazio() || a.Concluido() || a.Cancelado() || a.Etapa() != 0 {
		t.Errorf("assistente novo: vazio=%v concluido=%v cancelado=%v etapa=%d", a.Vazio(), a.Concluido(), a.Cancelado(), a.Etapa())
	}
}

func TestAssistenteVazio(t *testing.T) {
	a := NovoAssistente(0, PassoAssistente{Marco: "nada"})
	if !a.Vazio() || !a.Concluido() || a.Cancelado() || a.Etapa() != 0 || a.View() != "" {
		t.Fatalf("assistente sem campos deveria estar vazio e concluído")
	}
	if a.largura != larguraAssistente {
		t.Errorf("largura padrão = %d, quer %d", a.largura, larguraAssistente)
	}
	if msg := a.Init()(); msg != (tea.QuitMsg{}) {
		t.Errorf("Init de assistente vazio deveria sair, veio %T", msg)
	}
	if _, cmd := a.Update(nil); cmd == nil || cmd() != (tea.QuitMsg{}) {
		t.Error("Update de assistente vazio deveria sair")
	}
}

func TestAssistenteAvancaAteOFim(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var r respostasTeste
	a := assistenteTeste(&r)
	iniciarAssistente(a)

	teclar(a, digitarTexto("loja"), teclaEnter)
	if a.Etapa() != 1 {
		t.Fatalf("depois do nome, etapa = %d, quer 1", a.Etapa())
	}
	teclar(a, teclaBaixo, teclaEnter)  // standard
	teclar(a, teclaEspaco, teclaEnter) // marca kepler
	if a.Etapa() != 2 {
		t.Fatalf("etapa = %d, quer 2 (extras)", a.Etapa())
	}
	teclar(a, teclaEnter) // detalhar = não: o passo do detalhe fica oculto
	if a.Etapa() != 3 {
		t.Fatalf("com detalhar = não, deveria pular o detalhe; etapa = %d", a.Etapa())
	}
	var cmd tea.Cmd
	for i := 0; i < 5 && !a.Concluido(); i++ {
		_, cmd = a.Update(teclaEnter)
		conduzirFila(a, executarCmd(cmd))
	}
	if !a.Concluido() || a.Cancelado() {
		t.Fatalf("deveria concluir; concluido=%v cancelado=%v", a.Concluido(), a.Cancelado())
	}
	if a.View() != "" {
		t.Errorf("view depois do fim deveria ficar vazia, veio %q", a.View())
	}
	if r.nome != "loja" || r.tipo != "standard" || strings.Join(r.extras, ",") != "kepler" || r.detalhar || !r.confirmar {
		t.Errorf("respostas = %+v", r)
	}
	if _, cmd := a.Update(teclaEnter); cmd == nil || cmd() != (tea.QuitMsg{}) {
		t.Error("depois do fim, Update deveria pedir para sair")
	}
}

func TestAssistenteVoltaUmaEtapa(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var r respostasTeste
	a := assistenteTeste(&r)
	iniciarAssistente(a)

	teclar(a, digitarTexto("loja"), teclaEnter)
	if a.Etapa() != 1 {
		t.Fatalf("etapa = %d, quer 1", a.Etapa())
	}
	teclar(a, teclaVoltar)
	if a.Etapa() != 0 {
		t.Fatalf("⇧⇥ deveria voltar à etapa 0; etapa = %d", a.Etapa())
	}
	if !strings.Contains(a.View(), "01/04") {
		t.Errorf("a rota deveria mostrar 01/04 depois de voltar:\n%s", a.View())
	}
	teclar(a, teclaApagar, digitarTexto("api"), teclaEnter)
	if r.nome != "api" || a.Etapa() != 1 {
		t.Errorf("depois de corrigir: nome = %q, etapa = %d", r.nome, a.Etapa())
	}
	// no segundo campo do passo, ⇧⇥ volta ao primeiro campo do mesmo passo
	teclar(a, teclaEnter, teclaEnter, teclaVoltar)
	if a.Etapa() != 2 {
		t.Fatalf("etapa = %d, quer 2", a.Etapa())
	}
	teclar(a, teclaVoltar)
	if a.Etapa() != 1 {
		t.Errorf("do segundo campo, dois ⇧⇥ deveriam voltar à etapa 1; etapa = %d", a.Etapa())
	}
}

func TestAssistenteCancela(t *testing.T) {
	for _, tecla := range []tea.KeyMsg{teclaEsc, {Type: tea.KeyCtrlC}} {
		t.Run(tecla.String(), func(t *testing.T) {
			var r respostasTeste
			a := assistenteTeste(&r)
			iniciarAssistente(a)
			teclar(a, digitarTexto("loja"), teclaEnter)
			_, cmd := a.Update(tecla)
			if !a.Cancelado() || a.Concluido() {
				t.Fatalf("%s deveria cancelar", tecla)
			}
			if cmd == nil || cmd() != (tea.QuitMsg{}) {
				t.Error("cancelar deveria pedir para sair")
			}
			if a.View() != "" {
				t.Error("view depois de cancelar deveria ficar vazia")
			}
		})
	}
}

func TestAssistenteListaSemFiltro(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var r respostasTeste
	a := assistenteTeste(&r)
	iniciarAssistente(a)
	teclar(a, digitarTexto("lo/ja"), teclaEnter) // no campo de texto, "/" é texto
	if r.nome != "lo/ja" {
		t.Fatalf("nome = %q, quer lo/ja", r.nome)
	}
	teclar(a, digitarTexto("/"))
	if !strings.Contains(a.View(), "TOPOLOGIA") {
		t.Fatalf("\"/\" na lista não deveria abrir o filtro (o título some):\n%s", a.View())
	}
	teclar(a, teclaEsc)
	if !a.Cancelado() {
		t.Error("depois do \"/\", esc deveria cancelar o assistente")
	}
}

func TestAssistenteAcompanhaAJanela(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var r respostasTeste
	a := assistenteTeste(&r)
	iniciarAssistente(a)
	teclar(a, tea.WindowSizeMsg{Width: 30, Height: 20})
	for _, l := range strings.Split(a.View(), "\n") {
		if w := len([]rune(l)); w > 30 {
			t.Errorf("linha com %d células numa janela de 30: %q", w, l)
		}
	}
	teclar(a, tea.WindowSizeMsg{Width: 200, Height: 20})
	if a.largura != 64 {
		t.Errorf("largura não passa da máxima: %d", a.largura)
	}
}

// Um texto que cresce depois do início (resumo da revisão) aparece inteiro.
func TestAssistenteTextoQueCresceNaoCorta(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	var nome string
	gerar := true
	resumo := func() string {
		return strings.Repeat("linha de resumo\n", 6) + "fim do resumo " + nome
	}
	a := NovoAssistente(64,
		PassoAssistente{Marco: "nome", Campos: []huh.Field{huh.NewInput().Key("nome").Value(&nome)}},
		PassoAssistente{Marco: "fim", Campos: []huh.Field{
			huh.NewConfirm().Key("gerar").Title("Gerar?").DescriptionFunc(resumo, &nome).Value(&gerar),
		}},
	)
	iniciarAssistente(a)
	teclar(a, digitarTexto("loja"), teclaEnter)
	if v := a.View(); !strings.Contains(v, "fim do resumo loja") {
		t.Errorf("o resumo saiu cortado:\n%s", v)
	}
}

// O campo de texto mostra só o "› " da carta, sem o "> " padrão do huh ao lado.
func TestHuhThemePromptUnico(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	var v string
	in := huh.NewInput().Title("nome").Value(&v)
	in.WithTheme(HuhTheme())
	in.Focus()
	if got := in.View(); strings.Contains(got, ">") || !strings.Contains(got, "› ") {
		t.Errorf("prompt duplicado ou ausente:\n%s", got)
	}
}

func TestAssistenteGolden(t *testing.T) {
	casos := []struct {
		nome   string
		escuro bool
		perfil termenv.Profile
		ascii  bool
	}{
		{"escuro", true, termenv.TrueColor, false},
		{"ascii", true, termenv.Ascii, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			var r respostasTeste
			a := assistenteTeste(&r)
			iniciarAssistente(a)
			var b strings.Builder
			b.WriteString("== nome\n" + a.View())
			teclar(a, digitarTexto("loja"), teclaEnter)
			b.WriteString("== tipo\n" + a.View())
			teclar(a, teclaEnter)
			b.WriteString("== extras\n" + a.View())
			conferirGolden(t, "assistente_"+c.nome, b.String())
		})
	}
}
