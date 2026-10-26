package ui

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Helpers de teste compartilhados por todo o pacote pkg/ui. Os testes das demais tarefas
// devem reutilizá-los em vez de criar variantes; não edite este arquivo fora da onda 0.

// atualizar regrava os arquivos golden: go test ./pkg/ui/... -update
var atualizar = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// usarTema configura tema, perfil de cor e modo ASCII para o teste e restaura o estado anterior
// (renderer padrão do lipgloss e opções de pkg/ui) no t.Cleanup.
// escuro escolhe o mapa noturno (true) ou o kraft (false); perfil é o perfil de cor forçado
// (termenv.TrueColor, ANSI256, ANSI ou Ascii); ascii liga os glifos ASCII.
// Não use em testes paralelos: altera o renderer global.
func usarTema(t *testing.T, escuro bool, perfil termenv.Profile, ascii bool) {
	t.Helper()
	escuroAnterior := lipgloss.HasDarkBackground()
	perfilAnterior := lipgloss.ColorProfile()
	opcoesAnteriores := Atual()
	terminalAnterior := SaidaNoTerminal
	SaidaNoTerminal = func() bool { return true }
	t.Cleanup(func() {
		SaidaNoTerminal = terminalAnterior
		Configurar(opcoesAnteriores)
		// Configurar não desfaz tema/perfil forçados; restaura-os explicitamente.
		lipgloss.SetHasDarkBackground(escuroAnterior)
		lipgloss.SetColorProfile(perfilAnterior)
	})

	tema := TemaClaro
	if escuro {
		tema = TemaEscuro
	}
	Configurar(Opcoes{Tema: tema, ASCII: ascii})
	lipgloss.SetColorProfile(perfil)
}

// normalizarQuebras troca CRLF por LF.
func normalizarQuebras(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// conferirGolden compara obtido com testdata/<nome>.golden (CRLF normalizado para LF).
// Com -update, grava o arquivo. Em caso de diferença, mostra as linhas divergentes.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome+".golden")
	obtido = normalizarQuebras(obtido)

	if *atualizar {
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatalf("criando testdata: %v", err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatalf("gravando golden %s: %v", caminho, err)
		}
		return
	}

	bruto, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("lendo golden %s: %v (rode com -update para criar)", caminho, err)
	}
	esperado := normalizarQuebras(string(bruto))
	if esperado == obtido {
		return
	}
	t.Errorf("golden %s difere:\n%s", caminho, diffLinhas(esperado, obtido))
}

// diffLinhas devolve um diff simples linha a linha ("-" esperado, "+" obtido).
func diffLinhas(esperado, obtido string) string {
	e, o := strings.Split(esperado, "\n"), strings.Split(obtido, "\n")
	n := len(e)
	if len(o) > n {
		n = len(o)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		var le, lo string
		temE, temO := i < len(e), i < len(o)
		if temE {
			le = e[i]
		}
		if temO {
			lo = o[i]
		}
		if temE && temO && le == lo {
			continue
		}
		if temE {
			b.WriteString("  linha " + strconv.Itoa(i+1) + " - " + strconv.Quote(le) + "\n")
		}
		if temO {
			b.WriteString("  linha " + strconv.Itoa(i+1) + " + " + strconv.Quote(lo) + "\n")
		}
	}
	return b.String()
}
