package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var frameHora = time.Date(2026, 10, 23, 14, 7, 9, 0, time.UTC)

func frameLista(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "payments-api-%02d  Running  %d/%d  %dm\n", i, 1, 1, i*3)
	}
	return b.String()
}

func framePartes() Partes {
	k := DefaultKeys()
	s := S()
	return Partes{
		Instrumento:   "viz",
		Rotulo:        "instr. 05 · observação",
		Contexto:      CaminhoContexto("staging", "payments") + "   " + IndicadorVivo("ao vivo · 2 s"),
		Abas:          s.Selecionado.Render(" Pods ") + "  Deployments  StatefulSets  Services",
		AbasCompactas: "1 Pods · 2 Deploy… · 3 Stateful… · 4 Svc",
		Lista:         frameLista(30),
		Detalhe:       "Pod payments-api-03\nNamespace: payments\nNode: ip-10-0-1-23\nStatus: Running\nReinícios: 0",
		Mensagem:      s.Ok.Render(G().Feito) + " 30 pods carregados",
		Hora:          frameHora,
		Teclas: []key.Binding{k.Global.Ajuda, k.Lista.Cima, k.Lista.Abrir, k.Lista.Buscar,
			k.Lista.Logs, k.Lista.Yaml, k.Global.Recarregar, k.Global.Sair},
	}
}

func frameConferirDimensoes(t *testing.T, saida string, largura, altura int) {
	t.Helper()
	linhas := strings.Split(saida, "\n")
	if len(linhas) != altura {
		t.Errorf("altura: %d linhas, esperado %d", len(linhas), altura)
	}
	for i, l := range linhas {
		if w := lipgloss.Width(l); w > largura {
			t.Errorf("linha %d passa da largura: %d > %d: %q", i+1, w, largura, l)
		}
	}
}

func TestMoldura_Golden(t *testing.T) {
	casos := []struct{ largura, altura int }{{60, 24}, {100, 28}, {140, 30}}
	temas := []struct {
		nome   string
		escuro bool
	}{{"escuro", true}, {"claro", false}}
	for _, tema := range temas {
		for _, c := range casos {
			t.Run(fmt.Sprintf("%s_%d", tema.nome, c.largura), func(t *testing.T) {
				usarTema(t, tema.escuro, termenv.TrueColor, false)
				saida := Moldura(c.largura, c.altura, framePartes())
				frameConferirDimensoes(t, saida, c.largura, c.altura)
				conferirGolden(t, fmt.Sprintf("frame_moldura_%s_%d", tema.nome, c.largura), saida)
			})
		}
	}
}

func TestMoldura_ASCIISemCor(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	saida := Moldura(140, 30, framePartes())
	frameConferirDimensoes(t, saida, 140, 30)
	if strings.Contains(saida, "\x1b[") {
		t.Error("perfil Ascii não deveria emitir escapes de cor")
	}
	if strings.ContainsAny(saida, "◆━│●✓") {
		t.Error("modo ASCII não deveria conter glifos Unicode")
	}
	conferirGolden(t, "frame_moldura_ascii_140", saida)
}

func TestModoPara(t *testing.T) {
	casos := []struct {
		l, a int
		want Modo
	}{
		{140, 30, ModoLadoALado}, {120, 20, ModoLadoALado}, {119, 20, ModoColuna},
		{80, 16, ModoColuna}, {79, 16, ModoCompacto}, {60, 16, ModoCompacto},
		{59, 30, ModoJanelaPequena}, {100, 15, ModoJanelaPequena},
	}
	for _, c := range casos {
		if got := ModoPara(c.l, c.a); got != c.want {
			t.Errorf("ModoPara(%d,%d) = %v, esperado %v", c.l, c.a, got, c.want)
		}
	}
}

func TestMoldura_JanelaPequena(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	for _, dim := range [][2]int{{50, 12}, {30, 3}, {10, 1}, {5, 2}} {
		saida := Moldura(dim[0], dim[1], framePartes())
		frameConferirDimensoes(t, saida, dim[0], dim[1])
	}
	saida := Moldura(50, 12, framePartes())
	if !strings.Contains(saida, "Aumente a janela") {
		t.Errorf("falta o aviso: %q", saida)
	}
	conferirGolden(t, "frame_janela_pequena_50x12", saida)
	if Moldura(0, 0, framePartes()) != "" {
		t.Error("janela vazia deveria render vazio")
	}
}

func TestMoldura_LadoALadoProporcao(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	p := framePartes()
	p.Lista, p.Detalhe = "AAAA", "BBBB"
	linhas := strings.Split(Moldura(120, 24, p), "\n")
	for _, l := range linhas {
		if i := strings.Index(l, "AAAA"); i >= 0 {
			j := strings.Index(l, "BBBB")
			// 117 células úteis: 70 para a lista, 47 para o detalhe, mais " | ".
			if j-i != 70+3 {
				t.Errorf("detalhe começa %d células após a lista, esperado 73", j-i)
			}
			return
		}
	}
	t.Error("corpo não encontrado")
}

func TestMoldura_ColunaDetalhePorCima(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	p := framePartes()
	p.Lista, p.Detalhe = "LISTA", "DETALHE"
	saida := Moldura(100, 24, p)
	if strings.Index(saida, "DETALHE") > strings.Index(saida, "LISTA") {
		t.Error("no modo coluna o detalhe deve vir antes da lista")
	}
	frameConferirDimensoes(t, saida, 100, 24)
}

func TestMoldura_CompactoUsaAbasNumeradas(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	saida := Moldura(70, 24, framePartes())
	if !strings.Contains(saida, "1 Pods") || strings.Contains(saida, "StatefulSets") {
		t.Error("compacto deveria usar AbasCompactas")
	}
	p := framePartes()
	p.AbasCompactas = ""
	if !strings.Contains(Moldura(70, 24, p), "Deployments") {
		t.Error("sem AbasCompactas deveria cair em Abas")
	}
}

func TestMoldura_SemDetalheEFiltros(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	p := framePartes()
	p.Detalhe = ""
	p.Filtros = "ns: payments"
	saida := Moldura(140, 20, p)
	frameConferirDimensoes(t, saida, 140, 20)
	if !strings.Contains(saida, "ns: payments") || strings.Contains(saida, " | ") {
		t.Error("filtro ausente ou separador vertical sem detalhe")
	}
}

func TestMoldura_TopoExcessivoMantemAltura(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	p := framePartes()
	p.Filtros = strings.Repeat("filtro\n", 30)
	saida := Moldura(100, 16, p)
	frameConferirDimensoes(t, saida, 100, 16)
	linhas := strings.Split(saida, "\n")
	if !strings.Contains(linhas[len(linhas)-1], "sair") {
		t.Error("a legenda deve permanecer na última linha")
	}
}

func TestMoldura_SairNaLegenda(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	p := framePartes()
	linhas := strings.Split(Moldura(120, 20, p), "\n")
	if l := linhas[len(linhas)-1]; !strings.HasSuffix(l, "q sair") {
		t.Errorf("sem Sair, a legenda termina em q sair: %q", l)
	}
	p.Sair = DefaultKeys().Global.SairForcado
	p.Teclas = p.Teclas[:len(p.Teclas)-1] // sem o q sair, que numa camada não sai
	linhas = strings.Split(Moldura(120, 20, p), "\n")
	if l := linhas[len(linhas)-1]; !strings.HasSuffix(l, "ctrl+c sair") || strings.Contains(l, "q sair") {
		t.Errorf("com Sair, a legenda termina na tecla dada: %q", l)
	}
}

func TestMoldura_SemAjudaTiraOMais(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	k := DefaultKeys()
	p := framePartes()
	p.Sair = k.Global.SairForcado
	p.Teclas = []key.Binding{k.Lista.Abrir, k.Global.Voltar}
	ultima := func() string {
		linhas := strings.Split(Moldura(72, 20, p), "\n")
		return linhas[len(linhas)-1]
	}
	if l := ultima(); !strings.Contains(l, "? mais") {
		t.Errorf("estreita, a legenda oferece ? mais: %q", l)
	}
	p.SemAjuda = true
	if l := ultima(); strings.Contains(l, "?") || !strings.HasSuffix(l, "ctrl+c sair") || !strings.Contains(l, "abrir") {
		t.Errorf("com SemAjuda, a legenda não oferece o ?: %q", l)
	}
	// Sem espaço, as teclas do fim somem sem prometer o ?.
	p.Teclas = []key.Binding{k.Lista.Abrir, k.Lista.Buscar, k.Lista.Filtrar, k.Lista.Logs, k.Lista.Yaml}
	if l := ultima(); strings.Contains(l, "?") || strings.Contains(l, "logs") {
		t.Errorf("com SemAjuda e sem espaço: %q", l)
	}
}

func TestEmpilhado_AlturaPequena(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	got := empilhado("lista", "detalhe", 20, 4)
	if len(got) != 4 || !strings.Contains(got[0], "detalhe") {
		t.Errorf("detalhe deveria prevalecer em corpo pequeno: %q", got)
	}
	got = empilhado("lista", "a\nb\nc\nd\ne\nf\ng", 20, 10)
	if len(got) != 10 {
		t.Errorf("altura %d, esperado 10", len(got))
	}
}

func TestMasthead(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	m := Masthead(100, "viz", "instr. 05", "ctx")
	if lipgloss.Width(m) != 100 || !strings.Contains(m, "* YBY  VIZ") || !strings.Contains(m, "INSTR. 05") {
		t.Errorf("masthead largo inesperado: %q", m)
	}
	// Abaixo de 80 colunas o rótulo some.
	if m = Masthead(70, "viz", "instr. 05", "ctx"); strings.Contains(m, "INSTR") || lipgloss.Width(m) != 70 {
		t.Errorf("masthead estreito inesperado: %q", m)
	}
	// Rótulo não cabe junto do contexto: omite o rótulo.
	if m = Masthead(85, "viz", "instr. 05 · observacao", strings.Repeat("c", 60)); strings.Contains(m, "INSTR") {
		t.Errorf("rótulo deveria ser omitido: %q", m)
	}
	// Contexto longo é truncado com reticências (marca preservada).
	m = Masthead(40, "viz", "", strings.Repeat("x", 80))
	if lipgloss.Width(m) != 40 || !strings.Contains(m, "YBY  VIZ") || !strings.Contains(m, "~") {
		t.Errorf("truncamento inesperado: %q", m)
	}
	// Sem espaço nem para reticências: só a marca.
	if m = Masthead(12, "viz", "", "contexto"); lipgloss.Width(m) != 12 {
		t.Errorf("largura %d", lipgloss.Width(m))
	}
	if linhaEsqDir("a", "b", 0) != "" || Rule(0) != "" {
		t.Error("largura zero deveria render vazio")
	}
}

func TestCaminhoContexto_Cores(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	got := CaminhoContexto("staging", "payments")
	if !strings.Contains(got, "staging") || !strings.Contains(got, "payments") || !strings.Contains(got, "▸") {
		t.Errorf("contexto inesperado: %q", got)
	}
	// Último segmento em Tinta (#DDD0B5 = 221;208;181), anteriores em TintaSuave (#A1937A).
	if !strings.Contains(got, "38;2;221;208;181mpayments") || !strings.Contains(got, "38;2;161;147;121mstaging") {
		t.Errorf("cores dos segmentos inesperadas: %q", got)
	}
}

func TestRule(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	if got := Rule(10); got != strings.Repeat("─", 10) {
		t.Errorf("Rule = %q", got)
	}
}

func TestStatusLine(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	got := StatusLine(60, "tudo certo", frameHora)
	if lipgloss.Width(got) != 60 || !strings.HasPrefix(got, "tudo certo") || !strings.HasSuffix(got, "atualizado 14:07:09") {
		t.Errorf("status inesperado: %q", got)
	}
	got = StatusLine(30, strings.Repeat("m", 50), frameHora)
	if lipgloss.Width(got) != 30 || !strings.HasSuffix(got, "atualizado 14:07:09") || !strings.Contains(got, "~") {
		t.Errorf("status longo inesperado: %q", got)
	}
	got = StatusLine(20, "sem hora", time.Time{})
	if lipgloss.Width(got) != 20 || strings.Contains(got, "atualizado") {
		t.Errorf("status sem hora inesperado: %q", got)
	}
}

func TestKeyHints(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	k := DefaultKeys()
	todas := []key.Binding{k.Global.Ajuda, k.Lista.Cima, k.Lista.Abrir, k.Lista.Buscar, k.Lista.Logs, k.Global.Sair}

	largo := KeyHints(120, todas...)
	if lipgloss.Width(largo) != 120 || !strings.HasSuffix(largo, "q sair") || !strings.Contains(largo, "logs") {
		t.Errorf("legenda larga inesperada: %q", largo)
	}
	if strings.Count(largo, "sair") != 1 {
		t.Errorf("q sair deveria aparecer uma vez: %q", largo)
	}

	estreito := KeyHints(70, todas...)
	if !strings.Contains(estreito, "? mais") || !strings.HasSuffix(estreito, "q sair") || strings.Contains(estreito, "logs") {
		t.Errorf("legenda estreita inesperada: %q", estreito)
	}
	if strings.Count(estreito, "ajuda") != 0 {
		t.Errorf("ajuda não deveria duplicar o '? mais': %q", estreito)
	}
	// 3 teclas + ? mais + q sair
	if got := strings.Count(strings.TrimSpace(estreito), "  "); got < 3 {
		t.Errorf("poucos itens na legenda estreita: %q", estreito)
	}

	// Em largura média, descarta do fim até caber e sinaliza "? mais".
	medio := KeyHints(82, todas...)
	if lipgloss.Width(medio) != 82 || !strings.HasSuffix(medio, "q sair") {
		t.Errorf("legenda média inesperada: %q", medio)
	}
	apertado := KeyHints(34, todas...)
	if lipgloss.Width(apertado) > 34 || !strings.Contains(apertado, "? mais") {
		t.Errorf("legenda apertada inesperada: %q", apertado)
	}

	// Bindings desabilitados ou sem ajuda são ignorados.
	desligado := k.Lista.Logs
	desligado.SetEnabled(false)
	semAjuda := key.NewBinding(key.WithKeys("z"))
	if got := KeyHints(120, desligado, semAjuda); strings.Contains(got, "logs") || strings.Contains(got, "z") {
		t.Errorf("bindings inválidos não deveriam aparecer: %q", got)
	}
}

func TestKeyHints_Cores(t *testing.T) {
	usarTema(t, false, termenv.TrueColor, false)
	k := DefaultKeys()
	got := KeyHints(100, k.Lista.Abrir)
	frameConferirDimensoes(t, got, 100, 1)
	conferirGolden(t, "frame_keyhints_claro_100", got)
}

// TestLargurasEAlturaDoDetalhe: as larguras lado a lado somam o corpo menos o
// fio e são as que a moldura usa; o detalhe empilhado vai até metade do corpo.
func TestLargurasEAlturaDoDetalhe(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	for _, w := range []int{120, 140, 201} {
		l, d := LargurasLadoALado(w)
		if l+d+3 != w || l != (w-3)*60/100 {
			t.Errorf("largura %d: lista %d, detalhe %d", w, l, d)
		}
		linha := ladoALado(strings.Repeat("x", l), strings.Repeat("y", d), w, 1)[0]
		if !strings.HasSuffix(linha, strings.Repeat("y", d)) || !strings.HasPrefix(linha, strings.Repeat("x", l)+" ") {
			t.Errorf("largura %d: a moldura não usa as larguras: %q", w, linha)
		}
	}
	for h, quer := range map[int]int{4: 4, 5: 5, 6: 2, 23: 11, 24: 11} {
		if got := LimiteDetalheEmpilhado(h); got != quer {
			t.Errorf("corpo de %d linhas: limite %d, esperado %d", h, got, quer)
		}
	}
	if got := empilhado("lista", strings.Repeat("d\n", 30), 20, 23); strings.Count(strings.Join(got[:12], "\n"), "d") != 11 {
		t.Errorf("o detalhe empilhado deveria ocupar o limite: %q", got)
	}
}

// TestTeclasDoPainel: as teclas do painel saem como na legenda, sem "q sair"
// nem "? mais", e as do fim somem quando não cabem.
func TestTeclasDoPainel(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	k := DefaultKeys().Lista
	if got := strings.TrimRight(TeclasDoPainel(40, k.Logs, k.Yaml, k.Eventos), " "); got != "l logs  y yaml  e eventos" {
		t.Errorf("teclas do painel: %q", got)
	}
	if got := strings.TrimRight(TeclasDoPainel(16, k.Logs, k.Yaml, k.Eventos), " "); got != "l logs  y yaml" {
		t.Errorf("teclas do painel estreito: %q", got)
	}
	// O painel tem menos de 80 colunas, mas não é a legenda compacta: as quatro teclas da
	// folha VizPods ficam quando cabem.
	if got := strings.TrimRight(TeclasDoPainel(53, k.Logs, k.Yaml, k.Eventos, k.Bard), " "); got != "l logs  y yaml  e eventos  b perguntar ao Bard" {
		t.Errorf("teclas do painel do pod: %q", got)
	}
}
