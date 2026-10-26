package guarda

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// raizDoRepositorio sobe de pkg/ui/guarda até a pasta com o go.mod do yby-cli.
func raizDoRepositorio(t *testing.T) string {
	t.Helper()
	raiz, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(raiz, "go.mod"))
	if err != nil || !strings.Contains(string(mod), "module github.com/casheiro/yby-cli\n") {
		t.Fatalf("raiz do repositório não encontrada em %s", raiz)
	}
	return raiz
}

// TestGuarda_Repositorio é a guarda de regressão: nenhum .go fora do pkg/ui (e das exceções
// do pacote) pode ter emoji de status em texto nem chamar lipgloss.Color.
func TestGuarda_Repositorio(t *testing.T) {
	raiz := raizDoRepositorio(t)
	achados, conferidos, err := varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range achados {
		t.Error(a.String())
	}
	// Sanidade: a varredura passou pelos módulos próprios e por um arquivo com build tag
	// (o Sentinel é todo k8s), não só pela raiz.
	vistos := map[string]bool{}
	for _, c := range conferidos {
		vistos[c] = true
	}
	for _, alvo := range []string{"plugins/viz/main.go", "plugins/synapstor/main.go", "plugins/sentinel/cli/scan.go"} {
		if !vistos[alvo] {
			t.Errorf("a varredura não conferiu %s", alvo)
		}
	}
}

// TestGuarda_EntraNosModulosProprios: uma pasta com go.mod próprio não para a varredura.
func TestGuarda_EntraNosModulosProprios(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "go.mod", "module exemplo\n")
	escrever(t, raiz, "plugins/viz/go.mod", "module exemplo/viz\n")
	escrever(t, raiz, "plugins/viz/x.go", "package viz\n\nvar pronto = \"✅ pronto\"\n")
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(achados) != 1 || !strings.HasPrefix(achados[0].String(), `plugins/viz/x.go:3: emoji de status "✅"`) {
		t.Errorf("achados inesperados: %v", achados)
	}
}

// escrever grava conteudo em raiz/rel, criando as pastas.
func escrever(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	caminho := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

const exemploRuim = `package exemplo

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var verde = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))

func Pronto() {
	fmt.Println("✅ pronto")
}
`

func TestGuarda_FalhaComExemplo(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "cmd/ruim.go", exemploRuim)

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(achados) != 2 {
		t.Fatalf("esperava 2 achados, vieram %d: %v", len(achados), achados)
	}
	cor, emoji := achados[0].String(), achados[1].String()
	if !strings.HasPrefix(cor, "cmd/ruim.go:9: lipgloss.Color fora do pkg/ui") || !strings.Contains(cor, "ui.Tinta") {
		t.Errorf("achado de cor inesperado: %s", cor)
	}
	if !strings.HasPrefix(emoji, `cmd/ruim.go:12: emoji de status "✅"`) || !strings.Contains(emoji, "ui.Check") {
		t.Errorf("achado de emoji inesperado: %s", emoji)
	}
}

func TestGuarda_ApelidoEscapeESeletor(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "plugins/x/a.go", `package x

import lg "github.com/charmbracelet/lipgloss"

var c = lg.Color("1")
var aviso = "⚠️ cuidado"
var parada = '🛑'
`)
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	var linhas []string
	for _, a := range achados {
		linhas = append(linhas, a.String())
	}
	junto := strings.Join(linhas, "\n")
	if len(achados) != 3 || !strings.Contains(junto, "a.go:5: lipgloss.Color fora") ||
		!strings.Contains(junto, `a.go:6: emoji de status "⚠️"`) || !strings.Contains(junto, `a.go:7: emoji de status "🛑"`) {
		t.Errorf("achados inesperados:\n%s", junto)
	}
}

// TestGuarda_CorForaDeChamada: a cor montada sem chamar Color também é acusada (tipo de
// variável, literal composto, AdaptiveColor e cia.), no lipgloss v1 e no v2.
func TestGuarda_CorForaDeChamada(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "plugins/x/a.go", `package x

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var a lipgloss.Color = "#0f0"
var b = []lipgloss.Color{"#1", "#2"}
var c = lipgloss.AdaptiveColor{Light: "#000", Dark: "#fff"}
var d = lipgloss.CompleteColor{TrueColor: "#123"}
var e = lipgloss.ANSIColor(1)
var f = lipgloss.Color
var _ = table.New()
var _ = lipgloss.NewStyle()

type cor = lipgloss.Color
`)
	escrever(t, raiz, "plugins/x/v2.go", `package x

import lg "charm.land/lipgloss/v2"

var g = lg.Color("#fff")
`)
	escrever(t, raiz, "plugins/x/v2b.go", `package x

import "github.com/charmbracelet/lipgloss/v2"

var h = lipgloss.Color("#fff")
`)
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	var linhas []string
	for _, a := range achados {
		linhas = append(linhas, a.Arquivo+":"+strconv.Itoa(a.Linha))
	}
	esperado := "plugins/x/a.go:8 plugins/x/a.go:9 plugins/x/a.go:10 plugins/x/a.go:11 plugins/x/a.go:12 plugins/x/a.go:13 plugins/x/a.go:17 plugins/x/v2.go:5 plugins/x/v2b.go:5"
	if junto := strings.Join(linhas, " "); junto != esperado {
		t.Errorf("achados inesperados:\n%s\nesperava:\n%s\n%v", junto, esperado, achados)
	}
	if !strings.Contains(achados[2].Motivo, "lipgloss.AdaptiveColor fora do pkg/ui") {
		t.Errorf("motivo devia citar AdaptiveColor: %s", achados[2])
	}
}

// TestGuarda_ImportComPonto: com ponto, Color("#fff") vira nome solto; o import é recusado.
func TestGuarda_ImportComPonto(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "cmd/ponto.go", `package cmd

import . "github.com/charmbracelet/lipgloss"

var a = Color("#fff")
`)
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(achados) != 1 || !strings.HasPrefix(achados[0].String(), "cmd/ponto.go:3: import com ponto do lipgloss") {
		t.Errorf("achados inesperados: %v", achados)
	}
}

func TestGuarda_Excecoes(t *testing.T) {
	raiz := t.TempDir()
	// O dono da paleta, testes, testdata, vendor e pastas ocultas ficam de fora.
	escrever(t, raiz, "pkg/ui/tokens.go", exemploRuim)
	escrever(t, raiz, "cmd/ruim_test.go", exemploRuim)
	escrever(t, raiz, "cmd/testdata/fixture.go", exemploRuim)
	escrever(t, raiz, "vendor/x/y.go", exemploRuim)
	escrever(t, raiz, ".git/z.go", exemploRuim)
	// Comentários, os glifos do pkg/ui e um Color que não é do lipgloss passam.
	escrever(t, raiz, "cmd/ok.go", `package cmd

import "image/color"

// Antes era "✅ pronto" com lipgloss.Color("#0f0"); agora é ui.Check.
var glifos = "✓ ✕ ▲ ◆ ◇ ◈ ● ○ ▸ ━ ┄ ─ › … —"

type paleta struct{}

func (paleta) Color(string) color.Color { return nil }

var p paleta
var _ = p.Color("x")
`)
	// O tipo de cor só como tipo (campo, parâmetro, var sem valor, asserção) não monta cor:
	// é como o catálogo guarda os tokens do pkg/ui.
	escrever(t, raiz, "tools/catalogo/tipo.go", `package catalogo

import "github.com/charmbracelet/lipgloss"

type papel struct {
	cor lipgloss.CompleteAdaptiveColor
}

func tom(c lipgloss.TerminalColor) (lipgloss.Color, bool) {
	var vazio lipgloss.Color
	if x, ok := c.(lipgloss.Color); ok {
		return x, true
	}
	return vazio, false
}
`)
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range achados {
		t.Errorf("não devia acusar: %s", a)
	}
}

func TestGuarda_PermitidoValeSoParaAqueleEmoji(t *testing.T) {
	antes := permitidos
	t.Cleanup(func() { permitidos = antes })
	permitidos = []permitido{{"cmd/gen_docs.go", "📖", "exemplo do teste"}}
	raiz := t.TempDir()
	escrever(t, raiz, "cmd/gen_docs.go", "package cmd\n\nvar titulo = \"# \\U0001F4D6 CLI Reference\"\nvar fim = \"✅ gerado\"\n")
	escrever(t, raiz, "cmd/outro.go", "package cmd\n\nvar titulo2 = \"# 📖 Outro\"\n")
	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(achados) != 2 || achados[0].String()[:18] != "cmd/gen_docs.go:4:" || !strings.HasPrefix(achados[1].String(), "cmd/outro.go:3:") {
		t.Errorf("achados inesperados: %v", achados)
	}
}

func TestGuarda_ArquivoInvalidoDizQual(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, raiz, "quebrado.go", "package x\nfunc {")
	if _, err := Varrer(raiz); err == nil || !strings.Contains(err.Error(), "quebrado.go") {
		t.Errorf("esperava erro citando o arquivo, veio %v", err)
	}
}
