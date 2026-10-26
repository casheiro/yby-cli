package ui

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// fixarHomes faz AbreviarHome conhecer só as homes dadas durante o teste.
func fixarHomes(t *testing.T, homes ...string) {
	t.Helper()
	anterior := homesDoUsuario
	t.Cleanup(func() { homesDoUsuario = anterior })
	homesDoUsuario = func() []string { return homes }
}

func TestAbreviarHome(t *testing.T) {
	fixarHomes(t, "", `C:\Users\ana`, "/home/ana/")
	casos := map[string]string{
		`C:\Users\ana\.yby\reports\scan.md`: `~\.yby\reports\scan.md`,
		`C:\Users\ana`:                      "~",
		"/home/ana/.yby/reports/scan.md":    "~/.yby/reports/scan.md",
		// a home precisa acabar num separador: anabela não é ana
		`C:\Users\anabela\scan.md`: `C:\Users\anabela\scan.md`,
		"/srv/relatorios/scan.md":  "/srv/relatorios/scan.md",
		".synapstor/.uki":          ".synapstor/.uki",
	}
	for entrada, esperado := range casos {
		if got := AbreviarHome(entrada); got != esperado {
			t.Errorf("AbreviarHome(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
	// no Windows a letra do disco e as maiúsculas não importam, e a barra pode vir trocada
	if runtime.GOOS == "windows" {
		if got := AbreviarHome(`c:/users/ANA/.yby/x.md`); got != "~/.yby/x.md" {
			t.Errorf("no Windows a comparação ignora maiúsculas e a barra: %q", got)
		}
	}
}

func TestQuebrarCaminho(t *testing.T) {
	caminho := `C:\Users\anton\AppData\Local\Temp\claude\scratchpad\captura\home_f5\.yby\reports\sentinel-scan-payments-2026-10-25.md`
	linhas := QuebrarCaminho(caminho, 40)
	if strings.Join(linhas, "") != caminho {
		t.Fatalf("as linhas juntas refazem o caminho: %q", linhas)
	}
	for i, l := range linhas[:len(linhas)-1] {
		if !strings.HasSuffix(l, `\`) {
			t.Errorf("linha %d quebrou fora de um separador: %q", i+1, l)
		}
		if lipgloss.Width(l) > 40 {
			t.Errorf("linha %d passa de 40 células: %q", i+1, l)
		}
	}
	// um nome mais largo que a linha fica inteiro
	if got := QuebrarCaminho("a/"+strings.Repeat("x", 30)+"/b", 20); len(got) != 3 || got[1] != strings.Repeat("x", 30)+"/" {
		t.Errorf("nome comprido fica inteiro na sua linha: %q", got)
	}
	if got := QuebrarCaminho("~/.yby/reports/scan.md", 40); len(got) != 1 {
		t.Errorf("caminho que cabe fica numa linha: %q", got)
	}
}

func TestCamposComCaminho(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	fixarHomes(t, "/home/ana")
	fixarLargura(t, 40)
	var b bytes.Buffer
	Campos(&b,
		Campo{Rotulo: "fontes", Valor: "polaris, opa"},
		Campo{Rotulo: "relatório", Valor: "/home/ana/.yby/reports/sentinel-scan-payments-2026-10-25.md", Caminho: true},
	)
	esperado := "  fontes     polaris, opa\n" +
		"  relatório  ~/.yby/reports/\n" +
		"             sentinel-scan-payments-2026-10-25.md\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%q\nesperado:\n%q", b.String(), esperado)
	}

	// fora do terminal o caminho não quebra, mas a home vira ~ do mesmo jeito
	fixarLargura(t, 0)
	b.Reset()
	Campos(&b, Campo{Rotulo: "relatório", Valor: "/home/ana/.yby/reports/scan.md", Caminho: true})
	if b.String() != "  relatório  ~/.yby/reports/scan.md\n" {
		t.Errorf("fora do terminal: %q", b.String())
	}
}
