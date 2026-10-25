package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// teclasCompartilhadas lista as teclas que a seção 8 da especificação reutiliza em grupos
// distintos com sentido equivalente. Os grupos Lista e Texto nunca estão ativos ao mesmo
// tempo (uma tela é de lista ou de texto), então não há ambiguidade de despacho:
//   - esc: Global "voltar" (fecha camada/cancela) e Texto "interromper" (cancela a resposta).
//   - /: Lista "buscar" e Texto "comandos"; ambos abrem uma entrada de texto.
//   - pgup/pgdown: Lista "página" e Texto "rolar"; ambos movem a janela de conteúdo.
//   - enter: Lista "abrir" e Texto "enviar"; ambos confirmam o item/entrada atual.
//   - shift+tab: Lista "aba anterior" e Texto "etapa anterior"; ambos recuam.
//   - up: Lista "subir" e Texto "editar última"; em texto o cursor está na entrada.
var teclasCompartilhadas = map[string]bool{
	"esc": true, "/": true, "pgup": true, "pgdown": true,
	"enter": true, "shift+tab": true, "up": true,
}

func gruposDeTeclas() map[string][]key.Binding {
	k := DefaultKeys()
	return map[string][]key.Binding{
		"global": k.Global.Todas(),
		"lista":  k.Lista.Todas(),
		"texto":  k.Texto.Todas(),
	}
}

func TestTeclas_SemConflitoEntreGrupos(t *testing.T) {
	dono := map[string]string{} // tecla -> "grupo:ação"
	for _, grupo := range []string{"global", "lista", "texto"} {
		vistas := map[string]bool{}
		for _, b := range gruposDeTeclas()[grupo] {
			for _, tk := range b.Keys() {
				if vistas[tk] {
					t.Errorf("tecla %q repetida dentro do grupo %s", tk, grupo)
				}
				vistas[tk] = true
				if anterior, ok := dono[tk]; ok && !teclasCompartilhadas[tk] {
					t.Errorf("tecla %q em dois grupos com ações diferentes: %s e %s:%s", tk, anterior, grupo, b.Help().Desc)
				}
				dono[tk] = grupo + ":" + b.Help().Desc
			}
		}
	}
}

func TestTeclas_ExcecoesNaoObsoletas(t *testing.T) {
	contagem := map[string]int{}
	for _, bs := range gruposDeTeclas() {
		vistas := map[string]bool{}
		for _, b := range bs {
			for _, tk := range b.Keys() {
				vistas[tk] = true
			}
		}
		for tk := range vistas {
			contagem[tk]++
		}
	}
	for tk := range teclasCompartilhadas {
		if contagem[tk] < 2 {
			t.Errorf("exceção %q não é compartilhada por mais de um grupo", tk)
		}
	}
}

func TestTeclas_BindingsCompletos(t *testing.T) {
	for g, bs := range gruposDeTeclas() {
		for _, b := range bs {
			if len(b.Keys()) == 0 || b.Help().Key == "" || b.Help().Desc == "" {
				t.Errorf("binding incompleto no grupo %s: %+v", g, b.Help())
			}
			if !b.Enabled() {
				t.Errorf("binding desabilitado por padrão no grupo %s: %+v", g, b.Help())
			}
		}
	}
}

func TestTeclas_Especificas(t *testing.T) {
	k := DefaultKeys()
	if k.Global.Sair.Keys()[0] != "q" || k.Global.SairForcado.Keys()[0] != "ctrl+c" {
		t.Error("teclas de saída incorretas")
	}
	if len(k.Lista.AbaNumero.Keys()) != 9 {
		t.Errorf("AbaNumero deveria cobrir 1-9, tem %d", len(k.Lista.AbaNumero.Keys()))
	}
}

func TestTeclas_HelpKeyMap(t *testing.T) {
	k := DefaultKeys()
	if got := len(k.ShortHelp()); got != 2 {
		t.Errorf("ShortHelp: %d bindings, esperado 2", got)
	}
	if got := len(k.FullHelp()); got != 3 {
		t.Errorf("FullHelp: %d colunas, esperado 3", got)
	}
}

func TestNovoAjuda_Renderiza(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	k := DefaultKeys()
	m := NovoAjuda(100)
	curto := m.ShortHelpView(k.ShortHelp())
	if !strings.Contains(curto, "ajuda") || !strings.Contains(curto, "sair") {
		t.Errorf("ajuda curta inesperada: %q", curto)
	}
	m.ShowAll = true
	completo := m.FullHelpView(k.FullHelp())
	if !strings.Contains(completo, "recarregar") || !strings.Contains(completo, "logs") {
		t.Errorf("ajuda completa inesperada: %q", completo)
	}
}

func TestRotuloTecla_ASCII(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, true)
	casos := map[string]string{"↵": "enter", "⇧⇥": "shift+tab", "⇧↵": "shift+enter", "↑/k": "up/k", "␣": "space", "esc": "esc"}
	for entrada, esperado := range casos {
		if obtido := RotuloTecla(entrada); obtido != esperado {
			t.Errorf("RotuloTecla(%q) = %q, esperado %q", entrada, obtido, esperado)
		}
	}
	if h := DefaultKeys().Lista.Abrir.Help().Key; h != "enter" {
		t.Errorf("Abrir em ASCII = %q, esperado enter", h)
	}
}

func TestRotuloTecla_Unicode(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	if obtido := RotuloTecla("⇧⇥"); obtido != "⇧⇥" {
		t.Errorf("RotuloTecla em Unicode alterou o rótulo: %q", obtido)
	}
}

// TestTeclaAltGr: o símbolo digitado com AltGr vira a tecla simples, o alt+NUL do
// próprio AltGr é descartado e alt com letra ou dígito continua atalho.
func TestTeclaAltGr(t *testing.T) {
	casos := []struct {
		entrada tea.KeyMsg
		quer    string
		fica    bool
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}, Alt: true}, "/", true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}, Alt: true}, "?", true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}, Alt: true}, "", false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: true}, "alt+b", true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true}, "alt+1", true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, "/", true},
		{tea.KeyMsg{Type: tea.KeyEnter, Alt: true}, "alt+enter", true},
	}
	for _, c := range casos {
		k, fica := TeclaAltGr(c.entrada)
		if fica != c.fica || (fica && k.String() != c.quer) {
			t.Errorf("TeclaAltGr(%q) = %q, %v; quer %q, %v", c.entrada.String(), k.String(), fica, c.quer, c.fica)
		}
	}
}
