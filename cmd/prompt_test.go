package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
)

// semTerminal simula a entrada padrão fora de um terminal (pipe, CI).
func semTerminal(t *testing.T) {
	t.Helper()
	anterior := stdinTerminal
	t.Cleanup(func() { stdinTerminal = anterior })
	stdinTerminal = func() bool { return false }
}

func TestHuhPrompter_SemTerminal_NaoAbreNenhumPrompt(t *testing.T) {
	semTerminal(t)
	p := &HuhPrompter{}

	conferir := func(t *testing.T, err error) {
		t.Helper()
		var yerr *errors.YbyError
		require.ErrorAs(t, err, &yerr)
		assert.Equal(t, errors.ErrCodeValidation, yerr.Code)
	}

	v, err := p.Input("nome", "padrão")
	conferir(t, err)
	assert.Equal(t, "padrão", v)

	_, err = p.Password("senha")
	conferir(t, err)

	ok, err := p.Confirm("continuar?", true)
	conferir(t, err)
	assert.False(t, ok, "Confirm sem terminal nunca devolve o padrão sim")

	_, err = p.Select("opção", []string{"a", "b"}, "b")
	conferir(t, err)

	_, err = p.MultiSelect("opções", []string{"a", "b"}, []string{"a"})
	conferir(t, err)

	ok, err = p.ConfirmarNome("destruir cluster", "prod")
	conferir(t, err)
	assert.False(t, ok)
}

func TestConfirmacao_ConfirmaECancela(t *testing.T) {
	m := confirmacao{dialogo: ui.NovoConfirmarDestrutivo("destruir cluster", "ab")}
	assert.NotEmpty(t, m.View(), "o quadro aparece enquanto espera")

	// digitar repassa as teclas ao diálogo
	modelo, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")})
	m = modelo.(confirmacao)
	assert.Equal(t, "ab", m.dialogo.Digitado())

	modelo, cmd := m.Update(ui.ConfirmadoMsg{Nome: "ab"})
	m = modelo.(confirmacao)
	assert.True(t, m.confirmado)
	assert.Empty(t, m.View(), "o quadro some ao terminar")
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())

	modelo, cmd = confirmacao{dialogo: ui.NovoConfirmarDestrutivo("destruir cluster", "ab")}.Update(ui.CanceladoMsg{Nome: "ab"})
	m = modelo.(confirmacao)
	assert.False(t, m.confirmado)
	assert.Empty(t, m.View())
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
	assert.Nil(t, m.Init())
}
