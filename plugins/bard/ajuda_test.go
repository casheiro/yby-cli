package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
)

// usarVisualAjuda fixa tema, glifos e perfil de cor e restaura tudo no fim do teste.
func usarVisualAjuda(t *testing.T, o ui.Opcoes, perfil termenv.Profile) {
	t.Helper()
	perfilAnterior, opcoesAnteriores := lipgloss.ColorProfile(), ui.Atual()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetColorProfile(perfilAnterior)
	})
	ui.Configurar(o)
	lipgloss.SetColorProfile(perfil)
}

// A ajuda do Bard sai pelo ui.EscreverAjuda, como a dos outros plugins: título com ◆,
// seções em caixa-alta e as flags do b do Viz. Para regravar:
// go test -run TestAjuda_Golden ./plugins/bard/ -update
func TestAjuda_Golden(t *testing.T) {
	usarVisualAjuda(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	var b bytes.Buffer
	ui.EscreverAjuda(&b, ajudaBard())
	texto := ansi.Strip(b.String())
	for _, trecho := range []string{"◆ YBY  BARD", "USO", "EXEMPLOS", "FLAGS", "--pergunta", "--context", "-n, --namespace", "--voltar"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("a ajuda deveria ter %q:\n%s", trecho, texto)
		}
	}
	golden.RequireEqual(t, b.Bytes())
}

// O hook command com --help escreve a ajuda no stdout, no modo visual do ambiente (aqui
// ASCII e sem cor), e não abre nada.
func TestAjuda_PeloHook(t *testing.T) {
	saida, erros, codigo := rodarPlugin(t, "http://127.0.0.1:1", []string{"--help"}, "", "YBY_ASCII=1", "NO_COLOR=1")
	if codigo != 0 || erros != "" {
		t.Fatalf("código %d, stderr %q", codigo, erros)
	}
	if !strings.Contains(saida, "BARD") || !strings.Contains(saida, "--pergunta") || strings.ContainsRune(saida, '◆') || strings.Contains(saida, "38;") {
		t.Errorf("a ajuda no modo ASCII deveria sair sem o ◆ e sem cor:%c%s", '\n', saida)
	}
}
