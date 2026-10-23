// Comando ui-catalogo imprime todos os componentes de pkg/ui, em seções, com os dados de
// exemplo da especificação (docs/design/tui/especificacao-pkg-ui.md). Serve para ver o efeito
// de tema, largura, ASCII e cor sem subir uma TUI: só imprime e sai.
//
// Uso:
//
//	go run ./tools/ui-catalogo [--tema claro|escuro] [--largura N] [--ascii] [--sem-cor]
package main

import (
	"bytes"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/casheiro/yby-cli/pkg/config"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
)

const (
	larguraPadrao = 100
	larguraMinima = 40
	alturaMoldura = 20
)

func main() {
	if err := executar(os.Args[1:], os.Stdout, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "ui-catalogo:", err)
		os.Exit(1)
	}
}

// executar interpreta as flags, configura o modo visual (flags vencem env, que vence config)
// e imprime o catálogo em w.
func executar(args []string, w io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("ui-catalogo", flag.ContinueOnError)
	tema := fs.String("tema", "", "tema: claro ou escuro (padrão: automático)")
	largura := fs.Int("largura", larguraPadrao, "largura do catálogo em colunas")
	ascii := fs.Bool("ascii", false, "força glifos ASCII")
	semCor := fs.Bool("sem-cor", false, "desliga as cores")
	if err := fs.Parse(args); err != nil {
		return err
	}

	flagOpc := ui.Opcoes{ASCII: *ascii, SemCor: *semCor}
	switch strings.ToLower(*tema) {
	case "":
	case "claro", "light":
		flagOpc.Tema = ui.TemaClaro
	case "escuro", "dark":
		flagOpc.Tema = ui.TemaEscuro
	default:
		return fmt.Errorf("tema inválido %q (use claro ou escuro)", *tema)
	}
	definida := flagOpc.Tema != ui.TemaAuto || flagOpc.ASCII || flagOpc.SemCor
	if *largura < larguraMinima {
		return fmt.Errorf("largura mínima é %d", larguraMinima)
	}

	opc := ui.ResolverOpcoes(flagOpc, definida, getenv, config.Get().UI)
	ui.Configurar(opc)
	Gerar(w, *largura)
	return nil
}

// Gerar imprime o catálogo completo em w; nenhuma linha passa de largura células.
// Usa as opções já aplicadas por ui.Configurar.
func Gerar(w io.Writer, largura int) {
	secoes := []struct {
		nome string
		fn   func(w io.Writer, largura int)
	}{
		{"paleta", paleta},
		{"glifos", glifos},
		{"masthead e moldura", moldura},
		{"abas", abas},
		{"filtros", filtros},
		{"tabela com status", tabela},
		{"quadro", quadros},
		{"etapas", etapas},
		{"cota", cotas},
		{"rota", rota},
		{"toast", toast},
		{"confirmar", confirmar},
		{"estados", estados},
		{"doctor", doctor},
		{"up", up},
		{"erro", erro},
	}
	for _, s := range secoes {
		var b bytes.Buffer
		ui.Section(&b, s.nome)
		s.fn(&b, largura)
		escrever(w, b.String(), largura)
	}
}

// escrever imprime o texto cortando, com reticências, as linhas que passariam de largura
// (as saídas de comando têm largura fixa de 64 células).
func escrever(w io.Writer, texto string, largura int) {
	for _, l := range strings.Split(strings.TrimRight(texto, "\n"), "\n") {
		if ansi.StringWidth(l) > largura {
			l = ansi.Truncate(l, largura, ui.G().Reticencias)
		}
		fmt.Fprintln(w, l)
	}
}

func paleta(w io.Writer, _ int) {
	escuro := ui.Atual().Tema != ui.TemaClaro
	fundos := []string{"#FFFFFF", "#FAFAFA", "#F2F2F2"}
	if escuro {
		fundos = []string{"#000000", "#0C0C0C", "#1E1E1E", "#1F1C15"}
	}
	papeis := []struct {
		nome  string
		cor   lipgloss.CompleteAdaptiveColor
		texto bool
	}{
		{"Tinta", ui.Tinta, true}, {"TintaSuave", ui.TintaSuave, true}, {"Fio", ui.Fio, false},
		{"Ok", ui.Ok, true}, {"Aviso", ui.Aviso, true}, {"Erro", ui.Erro, true},
	}
	fmt.Fprintf(w, "  %-11s %-8s %s\n", "papel", "hex", "vs "+strings.Join(fundos, " "))
	for _, p := range papeis {
		hex := p.cor.Light.TrueColor
		if escuro {
			hex = p.cor.Dark.TrueColor
		}
		amostra := ui.Colorir(ui.S().Negrito, p.cor).Render(fmt.Sprintf("%-11s", p.nome))
		var razoes []string
		for _, f := range fundos {
			razoes = append(razoes, strings.Replace(fmt.Sprintf("%.1f", ui.Contraste(hex, f)), ".", ",", 1)+":1")
		}
		nota := ""
		if !p.texto {
			nota = "  (fio)"
		}
		fmt.Fprintf(w, "  %s %-8s %s%s\n", amostra, hex, strings.Join(razoes, "  "), nota)
	}
}

func glifos(w io.Writer, _ int) {
	g, s := ui.G(), ui.S()
	pares := []struct{ nome, glifo string }{
		{"ativo", g.Ativo}, {"pendente", g.Pendente}, {"andamento", g.Andamento}, {"feito", g.Feito},
		{"falha", g.Falha}, {"alerta", g.Alerta}, {"rodando", g.Rodando}, {"parado", g.Parado},
		{"cheio", g.Cheio}, {"tracejado", g.Tracejado}, {"linha", g.Linha}, {"caminho", g.Caminho},
		{"prompt", g.Prompt}, {"reticencias", g.Reticencias},
	}
	var itens []string
	for _, p := range pares {
		itens = append(itens, s.Tinta.Render(p.glifo)+" "+s.TintaSuave.Render(p.nome))
	}
	fmt.Fprintln(w, "  "+strings.Join(itens[:7], "   "))
	fmt.Fprintln(w, "  "+strings.Join(itens[7:], "   "))
	fmt.Fprintln(w, "  "+s.TintaSuave.Render("spinner")+" "+strings.Join(g.Spinner, " "))
}

func moldura(w io.Writer, largura int) {
	detalhe := strings.Join([]string{
		"api-7f9c-x2k", "namespace  payments", "nó         ip-10-0-3-12", "imagem     api:1.42.0",
	}, "\n")
	t := ui.DefaultKeys()
	partes := ui.Partes{
		Instrumento:   "viz",
		Rotulo:        "INSTR. 05 · OBSERVAÇÃO",
		Contexto:      ui.CaminhoContexto("staging", "payments") + "   " + ui.IndicadorVivo("ao vivo · 2 s"),
		Abas:          ui.Abas(abasExemplo(), 0, largura),
		AbasCompactas: ui.Abas(abasExemplo(), 0, largura),
		Filtros:       ui.Filtros(chipsExemplo(), 8, 42, largura),
		Lista:         tabelaDe(largura),
		Detalhe:       detalhe,
		Mensagem:      ui.Status(ui.StatusRodando, "8 pods rodando"),
		Hora:          time.Date(2026, 10, 23, 12, 10, 4, 0, time.UTC),
		Teclas:        []key.Binding{t.Lista.Abrir, t.Lista.Buscar, t.Lista.Filtrar, t.Lista.Acoes, t.Global.Ajuda, t.Global.Sair},
	}
	fmt.Fprintln(w, ui.Moldura(largura, alturaMoldura, partes))
}

func abasExemplo() []ui.Aba {
	return []ui.Aba{{Titulo: "Pods", Contagem: 8}, {Titulo: "Deployments", Contagem: 4},
		{Titulo: "StatefulSets", Contagem: 1}, {Titulo: "Services", Contagem: 6}}
}

func chipsExemplo() []ui.Chip {
	return []ui.Chip{{Chave: "status", Valor: "com problema"}, {Chave: "app=api"}}
}

func abas(w io.Writer, largura int) {
	fmt.Fprintln(w, ui.Abas(abasExemplo(), 0, largura))
	fmt.Fprintln(w, ui.Abas(abasExemplo(), 2, largura))
}

func filtros(w io.Writer, largura int) {
	fmt.Fprintln(w, ui.Filtros(chipsExemplo(), 8, 42, largura))
}

func tabelaDe(largura int) string {
	colunas := []ui.Coluna{
		{Titulo: "Nome", Min: 12, Max: 28, Prioridade: 0},
		{Titulo: "Status", Min: 16, Prioridade: 1},
		{Titulo: "Prontos", Min: 7, Prioridade: 2, Direita: true},
		{Titulo: "Reinícios", Min: 9, Prioridade: 3, Direita: true},
		{Titulo: "Idade", Min: 6, Prioridade: 4, Direita: true},
	}
	linhas := []ui.Linha{
		{Celulas: []string{"api-7f9c-x2k", ui.Status(ui.StatusRodando, "Running"), "1/1", "0", "3 d"}},
		{Celulas: []string{"worker-5d8b-q9m", ui.Status(ui.StatusFalha, "CrashLoopBackOff"), "0/1", "14", "3 d"}},
		{Celulas: []string{"cache-0", ui.Status(ui.StatusAlerta, "Pending"), "0/1", "0", "2 min"}},
		{Celulas: []string{"migracao-29381", ui.Status(ui.StatusConcluido, "Completed"), "0/1", "0", "1 d"}, Concluida: true},
		{Celulas: []string{"batch-parado", ui.Status(ui.StatusParado, "Parado"), "0/0", "0", "9 d"}, Concluida: true},
	}
	return ui.Tabela(colunas, linhas, 1, largura)
}

func tabela(w io.Writer, largura int) {
	fmt.Fprintln(w, tabelaDe(largura))
}

func quadros(w io.Writer, largura int) {
	l := min(largura, 60)
	corpo := "api-7f9c-x2k\nnamespace  payments"
	fmt.Fprintln(w, ui.Quadro("Detalhe · pod", corpo, l, ui.QuadroNormal))
	fmt.Fprintln(w, ui.Quadro("Detalhe · pod (foco)", corpo, l, ui.QuadroFoco))
	fmt.Fprintln(w, ui.Quadro("Excluir pod", corpo, l, ui.QuadroDestrutivo))
}

func etapas(w io.Writer, largura int) {
	fmt.Fprintln(w, ui.Etapas([]ui.Etapa{
		{Rotulo: "contexto do projeto", Estado: ui.EtapaFeita, Duracao: 300 * time.Millisecond},
		{Rotulo: "kubectl get pvc -A", Estado: ui.EtapaAtual, Duracao: 2100 * time.Millisecond},
		{Rotulo: "resposta", Estado: ui.EtapaPendente},
		{Rotulo: "gravar relatório", Estado: ui.EtapaFalha, Duracao: 72 * time.Second},
	}, min(largura, 64)))
}

func cotas(w io.Writer, largura int) {
	l := min(largura, 64)
	fmt.Fprintln(w, ui.Cota("argo cd", 0.64, l))
	fmt.Fprintln(w, ui.CotaTempo("sincronização", 0.40, 38*time.Second, l))
	fmt.Fprintln(w, ui.CotaTempo("sincronização", 0.92, 4*time.Second, l))
}

func rota(w io.Writer, largura int) {
	fmt.Fprintln(w, ui.Rota([]string{"projeto", "cluster", "argo cd", "apps", "dns", "verificar"}, 2, min(largura, 64)))
}

func toast(w io.Writer, _ int) {
	t := ui.NovoToast()
	for _, e := range []struct {
		tipo  ui.TipoToast
		texto string
	}{
		{ui.ToastOk, "Deployment api reiniciado · 3 réplicas"},
		{ui.ToastAviso, "réplicas abaixo do esperado · 2 de 3"},
		{ui.ToastErro, "sem permissão para escalar · RBAC"},
	} {
		t, _ = t.Mostrar(e.tipo, e.texto)
		fmt.Fprintln(w, t.View())
	}
}

func confirmar(w io.Writer, _ int) {
	fmt.Fprintln(w, ui.NovoConfirmarDestrutivo("excluir pod", "api-7f9c-x2k").View())
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.NovoConfirmarReversivel("reiniciar", "Reiniciar o deployment api?").View())
}

func estados(w io.Writer, _ int) {
	parado := func() bool { return false }
	fmt.Fprintln(w, ui.NovoEstadoCarregandoCom("lendo pods de payments", parado).View())
	fmt.Fprintln(w, ui.NovoEstadoVazio("nenhum pod com esse filtro", "esc limpa").View())
	fmt.Fprintln(w, ui.NovoEstadoErro("sem acesso ao cluster", "r tenta de novo").View())
}

func doctor(w io.Writer, _ int) {
	ui.Title(w, "doctor", "23/10 12:10")
	ui.Section(w, "ferramentas")
	ui.Check(w, ui.CheckOk, "docker", "27.3", "")
	ui.Check(w, ui.CheckAviso, "helm", "3.12", "recomendado 3.14 ou mais · yby setup atualiza")
	ui.Check(w, ui.CheckErro, "kubectl", "ausente", "winget install Kubernetes.kubectl")
	ui.Check(w, ui.CheckNeutro, "az", "ausente", "opcional")
	ui.Summary(w, 4, 1, 1, "o erro impede yby up")
}

func up(w io.Writer, _ int) {
	ui.Title(w, "up", "23/10 12:10")
	ui.Steps(w, []ui.Etapa{
		{Rotulo: "cluster yby-local", Nota: "criado", Estado: ui.EtapaFeita, Duracao: 38 * time.Second},
		{Rotulo: "argo cd", Estado: ui.EtapaAtual, Fracao: 0.64, Duracao: 25 * time.Second},
		{Rotulo: "root-app", Estado: ui.EtapaPendente},
	}, 64)
}

func erro(w io.Writer, _ int) {
	e := yerrors.Wrap(stderrors.New("o token do EKS expirou há 12 min"), yerrors.ErrCodeClusterOffline,
		"Não deu para falar com o cluster staging").
		WithHint("yby cloud refresh --provider aws")
	fmt.Fprintln(w, ui.RenderError(e))
}
