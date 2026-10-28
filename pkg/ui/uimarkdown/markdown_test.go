package uimarkdown

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// atualizar regrava os arquivos golden: go test ./pkg/ui/uimarkdown/ -update
var atualizar = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

// usarModo configura tema, perfil e modo visual e restaura tudo no fim do teste.
func usarModo(t *testing.T, o ui.Opcoes, perfil termenv.Profile) {
	t.Helper()
	escuroAnterior := lipgloss.HasDarkBackground()
	perfilAnterior := lipgloss.ColorProfile()
	opcoesAnteriores := ui.Atual()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetHasDarkBackground(escuroAnterior)
		lipgloss.SetColorProfile(perfilAnterior)
	})
	ui.Configurar(o)
	lipgloss.SetColorProfile(perfil)
}

func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome+".golden")
	if *atualizar {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	bruto, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("lendo golden %s: %v (rode com -update para criar)", caminho, err)
	}
	if esperado := strings.ReplaceAll(string(bruto), "\r\n", "\n"); esperado != obtido {
		t.Errorf("golden %s difere:\n--- esperado\n%s\n--- obtido\n%s", caminho, esperado, obtido)
	}
}

// comandoLongo passa da largura do quadro: tem de quebrar, não ser cortado.
const comandoLongo = `kubectl -n monitoring patch deployment grafana --type=json -p='[{"op":"replace","path":"/spec/replicas","value":3}]'`

// tabelaLarga tem células de texto que não cabem lado a lado: sai em registros.
const tabelaLarga = "| Recurso | Problema | Recomendação |\n|---|---|---|\n" +
	"| Deployment/api | O container roda como root e não define securityContext no values.yaml do chart | " +
	"Defina securityContext.runAsNonRoot e resources.limits no values.yaml antes do próximo deploy |\n" +
	"| Service/web | | Pending |\n"

const exemplo = "# Volumes do staging\n\n" +
	"O staging tem **6 volumes**. Quatro estão em uso e dois sobraram de apps removidos.\n" +
	"Os nomes vêm do `kubectl get pvc,pv -A`.\n\n" +
	"| Volume | Namespace | Tamanho | Status |\n" +
	"|---|---|---:|---|\n" +
	"| data-postgres-0 | payments | 20Gi | Bound |\n" +
	"| **pvc-3f1a9c2e** | sem dono | 8Gi | Released |\n" +
	"| worker | jobs \\| batch | 1Gi | CrashLoopBackOff |\n\n" +
	"Se não precisar dos dados:\n\n" +
	"```bash\nkubectl delete pv pvc-3f1a9c2e pvc-91cd04b7\n```\n\n" +
	"```bash\n" + comandoLongo + "\n```\n\n" +
	"~~~yaml\nkind: PersistentVolume\n\tspec: {}\n~~~\n\n" +
	"```\nsem linguagem\n```\n\n" +
	tabelaLarga + "\n" +
	"## Próximos passos\n\n" +
	"### Antes de apagar\n\n" +
	"- confira o *dono* de cada volume\n- [x] backup feito\n- [ ] apagar\n\n" +
	"1. rode o comando\n2. veja [a doc](https://kubernetes.io)\n\n" +
	"> volumes Released não voltam sozinhos\n\n---\n\nfim ~~velho~~\n"

func TestRenderizar_Golden(t *testing.T) {
	casos := []struct {
		nome   string
		o      ui.Opcoes
		perfil termenv.Profile
	}{
		{"escuro", ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor},
		{"claro", ui.Opcoes{Tema: ui.TemaClaro}, termenv.TrueColor},
		{"ascii", ui.Opcoes{Tema: ui.TemaEscuro, ASCII: true}, termenv.Ascii},
		{"semcor", ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.ANSI},
		{"escuro_256", ui.Opcoes{Tema: ui.TemaEscuro}, termenv.ANSI256},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarModo(t, c.o, c.perfil)
			r, err := Novo(72)
			if err != nil {
				t.Fatal(err)
			}
			saida, err := r.Renderizar(exemplo)
			if err != nil {
				t.Fatal(err)
			}
			for i, l := range strings.Split(saida, "\n") {
				if w := lipgloss.Width(l); w > 72 {
					t.Errorf("linha %d passa da largura: %d: %q", i+1, w, l)
				}
				if v := ansi.Strip(l); strings.TrimRight(v, " ") != v {
					t.Errorf("linha %d com espaço no fim: %q", i+1, v)
				}
			}
			conferirGolden(t, "renderizar_"+c.nome, saida)
		})
	}
}

// semFios tira ANSI, fios do quadro, espaços e quebras: sobra o texto, para conferir
// que nada se perdeu na quebra.
func semFios(s string) string {
	return strings.NewReplacer("│", "", "|", "", " ", "", "\n", "").Replace(ansi.Strip(s))
}

func conferirLargura(t *testing.T, saida string, largura int) {
	t.Helper()
	for i, l := range strings.Split(saida, "\n") {
		if w := lipgloss.Width(l); w > largura {
			t.Errorf("linha %d passa da largura: %d: %q", i+1, w, l)
		}
	}
}

func TestRenderizar_CodigoLongoQuebraSemCortar(t *testing.T) {
	yaml := "    chave: valor-muito-longo-que-passa-da-largura-do-quadro-e-continua"
	for _, c := range []struct{ md, texto string }{
		{"```bash\n" + comandoLongo + "\n```", comandoLongo},
		{"```yaml\nraiz:\n" + yaml + "\n```", "raiz:" + yaml},
	} {
		usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
		r, _ := Novo(60)
		saida, err := r.Renderizar(c.md)
		if err != nil {
			t.Fatal(err)
		}
		conferirLargura(t, saida, 60)
		if strings.Contains(ansi.Strip(saida), ui.G().Reticencias) {
			t.Errorf("o bloco de código não leva reticências:\n%s", ansi.Strip(saida))
		}
		if got, want := semFios(saida), strings.ReplaceAll(c.texto, " ", ""); !strings.Contains(got, want) {
			t.Errorf("o bloco perdeu texto:\n%s\nesperado conter %q", ansi.Strip(saida), want)
		}
	}
}

func TestQuebrarCodigo(t *testing.T) {
	casos := []struct {
		l    string
		w    int
		want []string
	}{
		{"curto", 10, []string{"curto"}},
		{"curto   ", 5, []string{"curto"}},
		{"aaa bbb ccc", 7, []string{"aaa bbb", "ccc"}},
		{"aaa --type=json -p=x", 16, []string{"aaa --type=json", "-p=x"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"    chave: valorlongo", 12, []string{"    chave:", "valorlongo"}},
		{"      xxxxxxxxxx", 8, []string{"      xx", "xxxxxxxx"}},
	}
	for _, c := range casos {
		got := quebrarCodigo(c.l, c.w)
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("quebrarCodigo(%q, %d) = %q, esperado %q", c.l, c.w, got, c.want)
		}
	}
}

func TestRenderizar_TabelaLargaViraRegistros(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	r, _ := Novo(80)
	saida, err := r.Renderizar(tabelaLarga)
	if err != nil {
		t.Fatal(err)
	}
	conferirLargura(t, saida, 80)
	visivel := ansi.Strip(saida)
	if strings.Contains(visivel, ui.G().Reticencias) {
		t.Errorf("a tabela larga não corta células:\n%s", visivel)
	}
	for _, celula := range celulas(strings.Split(tabelaLarga, "\n")[2]) {
		if !strings.Contains(semFios(saida), strings.ReplaceAll(celula, " ", "")) {
			t.Errorf("a célula %q sumiu:\n%s", celula, visivel)
		}
	}
	linhas := strings.Split(visivel, "\n")
	if !strings.HasPrefix(linhas[0], "RECURSO       Deployment/api") {
		t.Errorf("registro com o rótulo em caixa-alta e o valor alinhado: %q", linhas[0])
	}
	// A célula vazia (Problema do Service/web) não aparece; o registro segue com o resto.
	if !strings.Contains(visivel, "\n\nRECURSO       Service/web\nRECOMENDAÇÃO  "+ui.G().Alerta+" Pending") {
		t.Errorf("segundo registro:\n%s", visivel)
	}
}

func TestRenderizar_TabelaQueCabeFicaNaTabela(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	r, _ := Novo(40)
	md := "| Pod | Status |\n|---|---|\n| api | Running |\n"
	saida, _ := r.Renderizar(md)
	if want := "POD  STATUS\napi  " + ui.G().Rodando + " Running"; saida != want {
		t.Errorf("tabela que cabe sai na ui.Tabela:\n%q\nesperado\n%q", saida, want)
	}
	// Com um título largo demais para a coluna de rótulos, ele fica sozinho na linha.
	r, _ = Novo(20)
	saida, _ = r.Renderizar("| Recomendação de segurança | x |\n|---|---|\n| texto comprido aqui | y |\n")
	if !strings.HasPrefix(saida, "RECOMENDAÇÃO DE\nSEGURANÇA\n          texto") {
		t.Errorf("rótulo largo em linha própria:\n%s", saida)
	}
}

func TestRenderizar_TodosOsTitulosEmCaixaAlta(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	r, _ := Novo(40)
	for _, n := range []string{"#", "##", "###", "####", "#####", "######"} {
		if saida, _ := r.Renderizar(n + " Titulo misto"); saida != "TITULO MISTO" {
			t.Errorf("%s: %q", n, saida)
		}
	}
}

func TestRenderizar_SemCorMantemNegritoSemCores(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.ANSI)
	r, err := Novo(60)
	if err != nil {
		t.Fatal(err)
	}
	saida, _ := r.Renderizar("texto **forte**")
	cor := regexp.MustCompile(`\x1b\[([0-9;]*;)?(3[0-8]|9[0-7])[;m]`)
	if !strings.Contains(saida, "\x1b[1m") || cor.MatchString(saida) {
		t.Errorf("sem cor deveria manter o negrito e tirar as cores: %q", saida)
	}
}

func TestRenderizar_AsciiSemEstilo(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	r, _ := Novo(60)
	saida, _ := r.Renderizar("texto **forte** e `código`")
	if saida != "texto forte e código" {
		t.Errorf("no perfil Ascii a saída não leva ANSI: %q", saida)
	}
}

func TestNovo_LarguraMinima(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	r, err := Novo(5)
	if err != nil {
		t.Fatal(err)
	}
	if r.Largura() != larguraMinima {
		t.Errorf("largura mínima: %d", r.Largura())
	}
	if saida, _ := r.Renderizar(""); saida != "" {
		t.Errorf("markdown vazio deveria render vazio: %q", saida)
	}
}

func TestSepararBlocos(t *testing.T) {
	md := "texto | com barra\nsem tabela\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\ndepois\n" +
		"| x | y |\n|---|\n| fora |\n" +
		"````go\ncódigo\n```\nainda código\n````\n" +
		"~~~\naberto sem fim"
	blocos := separarBlocos(md)
	tipos := make([]tipoBloco, len(blocos))
	for i, b := range blocos {
		tipos[i] = b.tipo
	}
	want := []tipoBloco{blocoTexto, blocoTabela, blocoTexto, blocoCodigo, blocoCodigo}
	if len(tipos) != len(want) {
		t.Fatalf("blocos: %v", tipos)
	}
	for i := range want {
		if tipos[i] != want[i] {
			t.Fatalf("blocos: %v, esperado %v", tipos, want)
		}
	}
	if len(blocos[1].linhas) != 3 {
		t.Errorf("a tabela termina na linha em branco: %q", blocos[1].linhas)
	}
	// Delimitador com outra contagem de colunas não é tabela.
	if !strings.Contains(strings.Join(blocos[2].linhas, "\n"), "| x | y |") {
		t.Errorf("tabela inválida deveria ficar no texto: %q", blocos[2].linhas)
	}
	if blocos[3].linguagem != "go" || strings.Join(blocos[3].linhas, "\n") != "código\n```\nainda código" {
		t.Errorf("cerca de 4 crases só fecha com 4: %+v", blocos[3])
	}
	if blocos[4].linguagem != "" || blocos[4].linhas[0] != "aberto sem fim" {
		t.Errorf("bloco sem fim vai até o fim do texto: %+v", blocos[4])
	}
}

func TestCelulas(t *testing.T) {
	got := celulas(`| a | b \| c | d |`)
	if len(got) != 3 || got[1] != "b | c" {
		t.Errorf("células: %q", got)
	}
	if got := celulas(`a | b \|`); len(got) != 2 || got[1] != "b |" {
		t.Errorf("barra escapada no fim: %q", got)
	}
}

func TestEstilo_Perfis(t *testing.T) {
	usarModo(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	casos := []struct {
		perfil termenv.Profile
		escuro bool
		want   string
	}{
		{termenv.TrueColor, true, ui.Tinta.Dark.TrueColor},
		{termenv.TrueColor, false, ui.Tinta.Light.TrueColor},
		{termenv.ANSI256, true, ui.Tinta.Dark.ANSI256},
		{termenv.ANSI, false, ui.Tinta.Light.ANSI},
	}
	for _, c := range casos {
		e := Estilo(c.perfil, c.escuro, 40)
		if e.Document.Color == nil || *e.Document.Color != c.want {
			t.Errorf("%v escuro=%v: cor do texto %v, esperado %s", c.perfil, c.escuro, e.Document.Color, c.want)
		}
		if e.Code.BackgroundColor != nil || e.CodeBlock.Chroma != nil {
			t.Error("o estilo não pinta fundo nem usa o tema do chroma")
		}
	}
	if e := Estilo(termenv.Ascii, true, 40); e.Document.Color != nil {
		t.Error("no perfil Ascii o estilo não tem cor")
	}
	if e := Estilo(termenv.Ascii, true, 10); ansi.StringWidth(strings.TrimSpace(e.HorizontalRule.Format)) != 10 {
		t.Errorf("a régua segue a largura: %q", e.HorizontalRule.Format)
	}
}

func TestApararLinha(t *testing.T) {
	casos := map[string]string{
		"":                                   "",
		"   ":                                "",
		"\x1b[1m   \x1b[0m":                  "",
		"abc":                                "abc",
		"abc   ":                             "abc",
		"\x1b[1mab\x1b[0m\x1b[31m   \x1b[0m": "\x1b[1mab\x1b[0m",
		"\x1b[31m\x1b[0m\x1b[1mx\x1b[0m":     "\x1b[1mx\x1b[0m",
	}
	for entrada, want := range casos {
		if got := apararLinha(entrada); got != want {
			t.Errorf("apararLinha(%q) = %q, esperado %q", entrada, got, want)
		}
	}
}
