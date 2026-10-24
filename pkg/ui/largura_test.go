package ui

import (
	"bytes"
	stderrors "errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// fixarLargura faz larguraDe devolver larg para qualquer writer durante o teste.
func fixarLargura(t *testing.T, larg int) {
	t.Helper()
	anterior := larguraDe
	t.Cleanup(func() { larguraDe = anterior })
	larguraDe = func(io.Writer) int { return larg }
}

func TestLarguraForaDeTerminal(t *testing.T) {
	if l := Largura(&bytes.Buffer{}); l != 0 {
		t.Errorf("buffer deveria ter largura 0, veio %d", l)
	}
	f, err := os.CreateTemp(t.TempDir(), "saida")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if l := Largura(f); l != 0 {
		t.Errorf("arquivo deveria ter largura 0, veio %d", l)
	}
}

func TestQuebrar(t *testing.T) {
	casos := []struct {
		nome    string
		texto   string
		largura int
		quer    []string
	}{
		{"cabe", "curto", 20, []string{"curto"}},
		{"sem largura", "um texto que não quebra", 0, []string{"um texto que não quebra"}},
		{"estreita demais", "um texto que não quebra", minQuebra - 1, []string{"um texto que não quebra"}},
		{"quebra entre palavras", "confira o KUBECONFIG ou se o cluster está no ar", 20,
			[]string{"confira o KUBECONFIG", "ou se o cluster está", "no ar"}},
		{"palavra longa inteira", "caminho C:\\ProgramData\\chocolatey\\bin\\kubectl.exe fim", 16,
			[]string{"caminho", "C:\\ProgramData\\chocolatey\\bin\\kubectl.exe", "fim"}},
		{"só espaços", strings.Repeat(" ", 30), 16, []string{""}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := Quebrar(c.texto, c.largura)
			if strings.Join(got, "|") != strings.Join(c.quer, "|") {
				t.Errorf("Quebrar(%q, %d) = %q, quer %q", c.texto, c.largura, got, c.quer)
			}
		})
	}
}

func TestCheckNotaDesceNoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	fixarLargura(t, 60)
	col := ColunasPara("Prometheus Operator")
	var b bytes.Buffer
	CheckCom(&b, col, CheckAviso, "conexão", "sem acesso", "confira o KUBECONFIG ou se o cluster está no ar")
	CheckCom(&b, col, CheckOk, "git", "instalado", `C:\Program Files\Git\cmd\git.exe`)
	CheckCom(&b, col, CheckOk, "docker", "ativo", "27.3")
	esperado := "  ▲ conexão              sem acesso\n" +
		"    confira o KUBECONFIG ou se o cluster está no ar\n" +
		"  ✓ git                  instalado\n" +
		`    C:\Program Files\Git\cmd\git.exe` + "\n" +
		"  ✓ docker               ativo     27.3\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%s\nesperado:\n%s", b.String(), esperado)
	}

	// abaixo, a nota ainda quebra entre palavras se não couber
	fixarLargura(t, 30)
	b.Reset()
	Check(&b, CheckNeutro, "az", "ausente", "opcional · instale com winget install Microsoft.AzureCLI")
	for i, l := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		if lipgloss.Width(l) > 30 {
			t.Errorf("linha passa da largura: %q", l)
		}
		if i > 0 && !strings.HasPrefix(l, "    ") {
			t.Errorf("continuação sem recuo sob o nome: %q", l)
		}
	}
}

func TestTitleESummaryNoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)

	// terminal mais estreito que a largura padrão: o título se ajusta a ele
	fixarLargura(t, 50)
	var b bytes.Buffer
	Title(&b, "doctor", "24/10 12:03")
	if l := strings.TrimRight(b.String(), "\n"); lipgloss.Width(l) != 50 || !strings.HasSuffix(l, "24/10 12:03") {
		t.Errorf("título inesperado: %q", l)
	}

	// estreito demais para os dois: a direita desce
	fixarLargura(t, 24)
	b.Reset()
	Title(&b, "validate", `B:\projetos\yby-cli`)
	if b.String() != "◆ YBY  VALIDATE\n  B:\\projetos\\yby-cli\n" {
		t.Errorf("título inesperado: %q", b.String())
	}

	fixarLargura(t, 50)
	b.Reset()
	Summary(&b, 3, 1, 3, "corrija os erros antes do yby up")
	if b.String() != "\nRESUMO  3 ok · 1 aviso · 3 erros\n        corrija os erros antes do yby up\n" {
		t.Errorf("resumo inesperado: %q", b.String())
	}
	b.Reset()
	Summary(&b, 3, 0, 0, "ok")
	if b.String() != "\nRESUMO  3 ok · 0 avisos · 0 erros  · ok\n" {
		t.Errorf("resumo que cabe não deveria quebrar: %q", b.String())
	}
}

func TestWriteErrorNoTerminal(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	e := yerrors.Wrap(stderrors.New(`exec: "helm": executable file not found in %PATH%`), yerrors.ErrCodeExec,
		`Erro ao atualizar subcharts de B:\projetos\yby-cli\charts\system`)

	// fora do terminal, igual a RenderError
	var b bytes.Buffer
	WriteError(&b, e)
	if b.String() != RenderError(e) {
		t.Errorf("fora do terminal deveria igualar RenderError:\n%s", b.String())
	}

	fixarLargura(t, 60)
	b.Reset()
	WriteError(&b, e)
	esperado := `✕ Erro ao atualizar subcharts de` + "\n" +
		`  B:\projetos\yby-cli\charts\system` + "\n" +
		`  causa     exec: "helm": executable file not found in` + "\n" +
		`            %PATH%` + "\n" +
		`  faça      A execução do comando falhou. Rode com` + "\n" +
		`            --log-level=debug para detalhes.` + "\n" +
		`  código    ERR_EXEC_FAILED` + "\n"
	if b.String() != esperado {
		t.Errorf("saída inesperada:\n%s\nesperado:\n%s", b.String(), esperado)
	}

	// título curto: o código continua à direita, sem passar da largura
	fixarLargura(t, 50)
	b.Reset()
	WriteError(&b, yerrors.New(yerrors.ErrCodeIO, "falha ao ler"))
	primeira := strings.SplitN(b.String(), "\n", 2)[0]
	if !strings.HasSuffix(primeira, "ERR_IO") || lipgloss.Width(primeira) != 50 {
		t.Errorf("título inesperado: %q", primeira)
	}
	if strings.Contains(b.String(), "código") {
		t.Errorf("código repetido numa linha própria:\n%s", b.String())
	}
}
