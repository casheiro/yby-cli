package cmd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

// atualizarGolden regrava os arquivos golden: go test ./cmd/ -run <Teste> -update
var atualizarGolden = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// quandoGolden é o instante fixo usado nos títulos das saídas conferidas por golden.
var quandoGolden = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

// usarSaidaTexto deixa as saídas de pkg/ui sem cor (glifos Unicode ou ASCII) e fixa o
// relógio dos títulos; restaura tudo no t.Cleanup. Não use em testes paralelos.
func usarSaidaTexto(t *testing.T, ascii bool) {
	t.Helper()
	perfilAnterior, opcoesAnteriores, agoraAnterior := lipgloss.ColorProfile(), ui.Atual(), agora
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetColorProfile(perfilAnterior)
		agora = agoraAnterior
	})
	ui.Configurar(ui.Opcoes{ASCII: ascii})
	lipgloss.SetColorProfile(termenv.Ascii)
	agora = func() time.Time { return quandoGolden }
}

// conferirGolden compara obtido com testdata/<nome>.golden (CRLF normalizado para LF).
// Com -update, grava o arquivo.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome+".golden")
	obtido = strings.ReplaceAll(obtido, "\r\n", "\n")

	if *atualizarGolden {
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
	if esperado := strings.ReplaceAll(string(bruto), "\r\n", "\n"); esperado != obtido {
		t.Errorf("golden %s difere.\nesperado:\n%s\nobtido:\n%s", caminho, esperado, obtido)
	}
}

// conferirSemRestos falha se a saída ainda tiver emoji de status ou glifo duplicado
// (restos de checkStyle.String() e SetString).
func conferirSemRestos(t *testing.T, saida string) {
	t.Helper()
	for _, resto := range []string{"✅", "❌", "⚠️", "✨", "🔍", "✓ ✓", "✕ ✕"} {
		if strings.Contains(saida, resto) {
			t.Errorf("saída ainda tem %q:\n%s", resto, saida)
		}
	}
}

// saidaDoComando direciona a saída de c para um buffer durante o teste. Outros testes
// mudam a saída do rootCmd, que os subcomandos herdam em OutOrStdout.
func saidaDoComando(t *testing.T, c *cobra.Command) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	c.SetOut(&b)
	t.Cleanup(func() { c.SetOut(nil) })
	return &b
}
