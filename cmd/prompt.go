package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
)

// Prompter abstrai operações de prompt interativo para facilitar testes.
type Prompter interface {
	Input(title string, defaultVal string) (string, error)
	Password(title string) (string, error)
	Confirm(title string, defaultVal bool) (bool, error)
	Select(title string, options []string, defaultVal string) (string, error)
	MultiSelect(title string, options []string, defaults []string) ([]string, error)
	// ConfirmarNome pede que o usuário digite nome para liberar uma ação destrutiva
	// (acao é o rótulo do quadro, ex.: "destruir cluster"); devolve false se cancelar.
	ConfirmarNome(acao, nome string) (bool, error)
}

// stdinTerminal informa se a entrada padrão é um terminal. Variável para os testes.
var stdinTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// erroSemTerminal é devolvido pelos prompts quando a entrada não é um terminal (pipe,
// CI): perguntar travaria o comando esperando uma resposta que não vem.
func erroSemTerminal() error {
	return errors.New(errors.ErrCodeValidation, "a pergunta precisa de um terminal interativo")
}

// HuhPrompter implementa Prompter com charmbracelet/huh no tema da carta do terminal.
// Sem terminal na entrada, nenhum prompt abre: todos devolvem erroSemTerminal.
type HuhPrompter struct{}

// rodar mostra um campo com o tema da carta.
func rodar(campo huh.Field) error {
	if !stdinTerminal() {
		return erroSemTerminal()
	}
	tema := ui.HuhTheme()
	// uma linha em branco separa a pergunta do que o comando já escreveu e some com ela
	tema.Form.Base = tema.Form.Base.MarginTop(1)
	return huh.NewForm(huh.NewGroup(campo)).WithTheme(tema).WithShowHelp(false).Run()
}

func (p *HuhPrompter) Input(title string, defaultVal string) (string, error) {
	val := defaultVal
	err := rodar(huh.NewInput().Title(title).Value(&val))
	return val, err
}

func (p *HuhPrompter) Password(title string) (string, error) {
	var val string
	err := rodar(huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&val))
	return val, err
}

// Confirm devolve false em qualquer erro (sem terminal, cancelado), nunca o padrão:
// quem ignora o erro não executa a ação por engano.
func (p *HuhPrompter) Confirm(title string, defaultVal bool) (bool, error) {
	val := defaultVal
	if err := rodar(huh.NewConfirm().Title(title).Affirmative("Sim").Negative("Não").Value(&val)); err != nil {
		return false, err
	}
	return val, nil
}

func (p *HuhPrompter) Select(title string, options []string, defaultVal string) (string, error) {
	val := defaultVal
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := rodar(huh.NewSelect[string]().Title(title).Options(opts...).Value(&val))
	return val, err
}

func (p *HuhPrompter) MultiSelect(title string, options []string, defaults []string) ([]string, error) {
	val := defaults
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := rodar(huh.NewMultiSelect[string]().Title(title).Options(opts...).Value(&val))
	return val, err
}

func (p *HuhPrompter) ConfirmarNome(acao, nome string) (bool, error) {
	if !stdinTerminal() {
		return false, erroSemTerminal()
	}
	final, err := tea.NewProgram(confirmacao{dialogo: ui.NovoConfirmarDestrutivo(acao, nome)}).Run()
	if err != nil {
		return false, err
	}
	return final.(confirmacao).confirmado, nil
}

// confirmacao roda o ui.Confirmar como programa: sai ao confirmar ou cancelar e apaga o
// quadro da tela.
type confirmacao struct {
	dialogo    ui.Confirmar
	confirmado bool
	fim        bool
}

func (m confirmacao) Init() tea.Cmd { return m.dialogo.Init() }

func (m confirmacao) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ui.ConfirmadoMsg:
		m.confirmado, m.fim = true, true
		return m, tea.Quit
	case ui.CanceladoMsg:
		m.fim = true
		return m, tea.Quit
	}
	d, cmd := m.dialogo.Update(msg)
	m.dialogo = d.(ui.Confirmar)
	return m, cmd
}

func (m confirmacao) View() string {
	if m.fim {
		return ""
	}
	return m.dialogo.View()
}

// prompter é a instância global usada pelos comandos. Pode ser substituída em testes.
var prompter Prompter = &HuhPrompter{}
