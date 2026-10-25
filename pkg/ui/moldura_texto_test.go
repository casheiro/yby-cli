package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func textoPartes(linhasCorpo int) PartesTexto {
	k := DefaultKeys()
	s := S()
	var corpo strings.Builder
	for i := 1; i <= linhasCorpo; i++ {
		fmt.Fprintf(&corpo, "linha %02d da conversa\n", i)
	}
	return PartesTexto{
		Instrumento: "bard",
		Rotulo:      "instr. 03 · orientação",
		Contexto:    CaminhoContexto("staging", "default") + "   " + s.TintaSuave.Render("claude-cli"),
		Corpo:       corpo.String(),
		Campo:       s.Ok.Render(G().Prompt) + " " + s.TintaSuave.Render("Pergunte sobre o cluster") + "\n ",
		Mensagem:    s.Ok.Render(G().Feito) + " resposta em 21 s",
		Direita:     "sessão de hoje · 2 mensagens",
		Sair:        k.Global.SairForcado,
		Teclas: []key.Binding{k.Texto.Enviar, k.Texto.NovaLinha, k.Texto.Comandos,
			k.Texto.EditarUltima, k.Texto.RolarCima, k.Global.Ajuda},
	}
}

func TestMolduraTexto_Golden(t *testing.T) {
	casos := []struct {
		nome          string
		escuro, ascii bool
		perfil        termenv.Profile
		largura       int
	}{
		{"escuro_100", true, false, termenv.TrueColor, 100},
		{"claro_100", false, false, termenv.TrueColor, 100},
		{"ascii_70", true, true, termenv.Ascii, 70},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, c.ascii)
			saida := MolduraTexto(c.largura, 20, textoPartes(30))
			frameConferirDimensoes(t, saida, c.largura, 20)
			conferirGolden(t, "moldura_texto_"+c.nome, saida)
		})
	}
}

func TestMolduraTexto_Estrutura(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := MolduraTexto(100, 20, textoPartes(3))
	linhas := strings.Split(saida, "\n")
	if len(linhas) != 20 {
		t.Fatalf("altura: %d", len(linhas))
	}
	if !strings.HasPrefix(linhas[0], "◆ YBY  BARD  INSTR. 03 · ORIENTAÇÃO") || !strings.HasSuffix(linhas[0], "staging ▸ default   claude-cli") {
		t.Errorf("masthead: %q", linhas[0])
	}
	// O corpo fica alinhado ao topo: a conversa curta deixa linhas vazias antes do campo.
	if !strings.HasPrefix(linhas[2], "linha 01") || strings.TrimSpace(linhas[12]) != "" {
		t.Errorf("corpo deveria começar no topo: %q / %q", linhas[2], linhas[12])
	}
	if AlturaCorpoTexto(20, 2) != 12 {
		t.Errorf("altura do corpo: %d", AlturaCorpoTexto(20, 2))
	}
	if !strings.HasPrefix(linhas[15], "› Pergunte") || strings.TrimSpace(linhas[16]) != "" {
		t.Errorf("campo entre réguas: %q / %q", linhas[15], linhas[16])
	}
	for _, i := range []int{1, 14, 17} {
		if strings.Trim(linhas[i], "─") != "" {
			t.Errorf("linha %d deveria ser régua: %q", i+1, linhas[i])
		}
	}
	if !strings.HasSuffix(linhas[18], "sessão de hoje · 2 mensagens") {
		t.Errorf("estado: %q", linhas[18])
	}
	if !strings.HasPrefix(linhas[19], "↵ enviar  ⇧↵ nova linha") || !strings.HasSuffix(linhas[19], "ctrl+c sair") {
		t.Errorf("legenda: %q", linhas[19])
	}
}

func TestMolduraTexto_JanelaPequena(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	saida := MolduraTexto(50, 12, textoPartes(3))
	if !strings.Contains(saida, "Aumente a janela") {
		t.Errorf("abaixo de 60x16 deveria pedir para aumentar a janela:\n%s", saida)
	}
	frameConferirDimensoes(t, saida, 50, 12)
}

func TestMolduraTexto_CampoAltoNaoPassaDaAltura(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	p := textoPartes(5)
	p.Campo = strings.Repeat("› campo\n", 15)
	saida := MolduraTexto(80, 16, p)
	frameConferirDimensoes(t, saida, 80, 16)
	linhas := strings.Split(saida, "\n")
	if !strings.HasSuffix(linhas[15], "ctrl+c sair") {
		t.Errorf("o rodapé deveria ficar no fim: %q", linhas[15])
	}
}

func TestStatusLineTexto(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if got := StatusLineTexto(30, "✓ pronto", ""); got != "✓ pronto"+strings.Repeat(" ", 22) {
		t.Errorf("sem direita: %q", got)
	}
	got := StatusLineTexto(30, "✓ resposta em 21 s com muito texto", "2 mensagens")
	if ansi.StringWidth(got) != 30 || !strings.HasSuffix(got, " 2 mensagens") || !strings.Contains(got, G().Reticencias) {
		t.Errorf("a mensagem deveria ceder espaço à direita: %q", got)
	}
	// Direita maior que a largura: a mensagem some e a direita é truncada.
	if got := StatusLineTexto(8, "✓ ok", "sessão de hoje"); ansi.StringWidth(got) != 8 {
		t.Errorf("largura mínima: %q", got)
	}
}

func TestKeyHintsCom(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	k := DefaultKeys()
	sair := k.Global.SairForcado
	desligada := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "nada"), key.WithDisabled())

	got := KeyHintsCom(100, sair, k.Texto.Enviar, k.Global.SairForcado, desligada, k.Global.Ajuda)
	if !strings.HasPrefix(got, "↵ enviar  ? ajuda") || !strings.HasSuffix(got, "ctrl+c sair") {
		t.Errorf("legenda larga: %q", got)
	}
	if strings.Count(got, "ctrl+c") != 1 || strings.Contains(got, "nada") {
		t.Errorf("a saída aparece só no canto e teclas desligadas somem: %q", got)
	}

	// Estreita: 3 teclas e "? mais".
	got = KeyHintsCom(70, sair, k.Texto.Enviar, k.Texto.NovaLinha, k.Texto.Comandos, k.Texto.EditarUltima, k.Global.Ajuda)
	if !strings.HasPrefix(got, "↵ enviar  ⇧↵ nova linha  / comandos  ? mais") || strings.Contains(got, "editar") {
		t.Errorf("legenda compacta: %q", got)
	}

	// Sem espaço: descarta do fim e passa a mostrar "? mais".
	got = KeyHintsCom(90, sair, k.Texto.Enviar, k.Texto.NovaLinha, k.Texto.Comandos, k.Texto.EditarUltima,
		k.Texto.RolarCima, k.Texto.RolarBaixo, k.Texto.Interromper, k.Texto.Marcar)
	if ansi.StringWidth(got) != 90 || !strings.Contains(got, "? mais") || strings.Contains(got, "marcar") {
		t.Errorf("legenda cortada: %q", got)
	}

	// Sem tecla de saída, o canto direito fica vazio.
	if got := KeyHintsCom(40, key.Binding{}, k.Texto.Enviar); strings.TrimSpace(got) != "↵ enviar  ? mais" {
		t.Errorf("sem saída: %q", got)
	}
}
