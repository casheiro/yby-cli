package ui

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	yerrors "github.com/casheiro/yby-cli/pkg/errors"
)

// Largura (em células) das saídas de comando sem TUI e das colunas de Check.
const (
	msgLargura   = 64
	msgColNome   = 12
	msgColVersao = 10
)

// TipoCheck classifica uma linha de verificação.
type TipoCheck int

const (
	// CheckOk indica item saudável (✓).
	CheckOk TipoCheck = iota
	// CheckAviso indica alerta (▲).
	CheckAviso
	// CheckErro indica falha (✕).
	CheckErro
	// CheckNeutro indica item opcional ou desconhecido (○), todo em TintaSuave.
	CheckNeutro
)

// padDireita completa s com espaços até w células (nunca trunca).
func padDireita(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// coluna completa s até w células; se s já ocupa a coluna, separa com 2 espaços.
func coluna(s string, w int) string {
	if s != "" && lipgloss.Width(s) >= w {
		return s + "  "
	}
	return padDireita(s, w)
}

// duasPontas coloca esq e dir nas pontas de uma linha de largura w (mínimo 2 espaços).
func duasPontas(esq, dir string, w int) string {
	folga := w - lipgloss.Width(esq) - lipgloss.Width(dir)
	if folga < 2 {
		folga = 2
	}
	return esq + strings.Repeat(" ", folga) + dir
}

// Title escreve o cabeçalho "◆ YBY  INSTRUMENTO" com o texto da direita (data, contexto) em TintaSuave.
func Title(w io.Writer, instrumento, direita string) {
	s := S()
	esq := s.Ok.Render(G().Ativo) + " " + s.Negrito.Render("YBY  "+strings.ToUpper(instrumento))
	fmt.Fprintln(w, duasPontas(esq, s.TintaSuave.Render(direita), msgLargura))
}

// Section escreve o nome de uma seção em TintaSuave e caixa-alta, precedido de linha em branco.
func Section(w io.Writer, nome string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, S().Rotulo.Render(strings.ToUpper(nome)))
}

// Colunas define as larguras, em células, das colunas de nome e de versão de CheckCom.
type Colunas struct{ Nome, Versao int }

// ColunasPara devolve as colunas padrão alargadas para caber o nome mais largo de
// nomes (mais 2 espaços), para alinhar um relatório com nomes longos.
func ColunasPara(nomes ...string) Colunas {
	c := Colunas{Nome: msgColNome, Versao: msgColVersao}
	for _, n := range nomes {
		c.Nome = max(c.Nome, lipgloss.Width(n)+2)
	}
	return c
}

// Check escreve uma linha de verificação com colunas alinhadas por largura em células:
// glifo, nome, versão e nota. Versão e nota são opcionais.
func Check(w io.Writer, tipo TipoCheck, nome, versao, nota string) {
	CheckCom(w, Colunas{Nome: msgColNome, Versao: msgColVersao}, tipo, nome, versao, nota)
}

// CheckCom é Check com larguras de coluna escolhidas pelo chamador (ver ColunasPara).
// Quando w é um terminal e a nota não cabe na linha, ela quebra entre palavras e
// continua alinhada na coluna da nota.
func CheckCom(w io.Writer, col Colunas, tipo TipoCheck, nome, versao, nota string) {
	s, g := S(), G()
	var glifo string
	var est lipgloss.Style
	switch tipo {
	case CheckOk:
		glifo, est = g.Feito, s.Ok
	case CheckAviso:
		glifo, est = g.Alerta, s.Aviso
	case CheckErro:
		glifo, est = g.Falha, s.Erro
	default:
		glifo, est = g.Parado, s.TintaSuave
	}
	corpo, notas, recuo := nome, []string{""}, 0
	if nota != "" || versao != "" {
		corpo = coluna(nome, col.Nome) + coluna(versao, col.Versao)
		// a nota começa depois da margem, do glifo com espaço e das colunas
		recuo = 3 + lipgloss.Width(glifo) + lipgloss.Width(corpo)
		notas = Quebrar(nota, larguraDe(w)-recuo)
		corpo += notas[0]
	}
	corpo = strings.TrimRight(corpo, " ")
	texto := s.Tinta
	if tipo == CheckNeutro {
		texto = s.TintaSuave
		fmt.Fprintln(w, "  "+texto.Render(glifo+" "+corpo))
	} else {
		fmt.Fprintln(w, "  "+est.Render(glifo)+" "+texto.Render(corpo))
	}
	for _, n := range notas[1:] {
		fmt.Fprintln(w, strings.Repeat(" ", recuo)+texto.Render(n))
	}
}

func plural(n int, singular, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plur)
}

// Summary escreve a linha "RESUMO  4 ok · 1 aviso · 1 erro" com a contagem colorida;
// frase, se não vazia, entra ao final depois de "· " em TintaSuave.
func Summary(w io.Writer, ok, aviso, erro int, frase string) {
	s := S()
	partes := []string{
		s.Ok.Render(fmt.Sprintf("%d ok", ok)),
		s.Aviso.Render(plural(aviso, "aviso", "avisos")),
		s.Erro.Render(plural(erro, "erro", "erros")),
	}
	sep := s.TintaSuave.Render(" · ")
	linha := s.Rotulo.Render("RESUMO") + "  " + strings.Join(partes, sep)
	if frase != "" {
		linha += "  " + s.TintaSuave.Render("· "+frase)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, linha)
}

// linhaUnica troca quebras de linha por espaço.
func linhaUnica(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// RenderError formata um YbyError: título com o código à direita, depois as linhas
// "causa", "faça" (hint em negrito; usa o hint automático do registry quando o erro não
// tem um) e "detalhes" (contexto diagnóstico ou, na falta dele, a dica de depuração
// quando o erro tem causa, que o debug detalha, e o hint ainda não a traz).
func RenderError(e *yerrors.YbyError) string { return renderErro(e, 0) }

// WriteError escreve em w o erro formatado por RenderError. Quando w é um terminal, os
// textos quebram entre palavras na largura dele e, se o título não cabe com o código à
// direita, o código vai para uma linha "código" no fim.
func WriteError(w io.Writer, e *yerrors.YbyError) {
	fmt.Fprint(w, renderErro(e, larguraDe(w)))
}

// renderErro implementa RenderError; com largura > 0 quebra os textos nessa largura.
func renderErro(e *yerrors.YbyError, largura int) string {
	if e == nil {
		return ""
	}
	s := S()
	var b strings.Builder
	msg := linhaUnica(e.Message)
	larguraTitulo := msgLargura
	if largura > 0 {
		larguraTitulo = min(msgLargura, largura)
	}
	codigoAoLado := e.Code != "" && (largura <= 0 || 4+lipgloss.Width(msg)+lipgloss.Width(e.Code) <= largura)
	partes := []string{msg}
	if !codigoAoLado {
		partes = Quebrar(msg, largura-2)
	}
	titulo := s.Erro.Render(G().Falha) + " " + s.Negrito.Render(partes[0])
	if codigoAoLado {
		titulo = duasPontas(titulo, s.TintaSuave.Render(e.Code), larguraTitulo)
	}
	b.WriteString(titulo + "\n")
	for _, p := range partes[1:] {
		b.WriteString("  " + s.Negrito.Render(p) + "\n")
	}

	linha := func(rotulo, texto string, estilo lipgloss.Style) {
		partes := Quebrar(linhaUnica(texto), largura-12)
		b.WriteString("  " + s.Rotulo.Render(padDireita(rotulo, 10)) + estilo.Render(partes[0]) + "\n")
		for _, p := range partes[1:] {
			b.WriteString(strings.Repeat(" ", 12) + estilo.Render(p) + "\n")
		}
	}
	if e.Cause != nil {
		linha("causa", textoCausa(e.Cause), s.Tinta)
	}
	hint := e.GetHint()
	if hint != "" {
		linha("faça", hint, s.Negrito)
	}
	if len(e.Context) > 0 {
		chaves := make([]string, 0, len(e.Context))
		for k := range e.Context {
			chaves = append(chaves, k)
		}
		sort.Strings(chaves)
		pares := make([]string, len(chaves))
		for i, k := range chaves {
			pares[i] = fmt.Sprintf("%s=%v", k, e.Context[k])
		}
		linha("detalhes", strings.Join(pares, " · "), s.TintaSuave)
	} else if e.Cause != nil && !strings.Contains(hint, "--log-level=debug") {
		linha("detalhes", "rode de novo com --log-level=debug", s.TintaSuave)
	}
	if e.Code != "" && !codigoAoLado {
		linha("código", e.Code, s.TintaSuave)
	}
	return b.String()
}

// textoCausa descreve a causa sem o prefixo "[CODE]" de YbyErrors aninhados: usa a
// mensagem de cada nível, separada por ": ".
func textoCausa(err error) string {
	var partes []string
	for err != nil {
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) {
			partes = append(partes, err.Error())
			break
		}
		partes = append(partes, ye.Message)
		err = ye.Cause
	}
	return strings.Join(partes, ": ")
}

// Steps escreve em w as etapas de um comando (ex.: yby up), uma por linha e recuadas em
// 2 células, no formato da seção 7 da especificação:
//
//	✓ cluster yby-local   criado   38 s
//	◈ argo cd  ━━━━┄┄ 64%   25 s
//	◇ root-app
//
// A etapa atual pode trazer Fracao (progresso) e vira uma cota. largura é o total em
// células, com o recuo incluído; com largura <= 0 usa a largura padrão das saídas.
func Steps(w io.Writer, etapas []Etapa, largura int) {
	if largura <= 0 {
		largura = msgLargura
	}
	corpo := Etapas(etapas, max(largura-2, 1))
	for _, l := range strings.Split(corpo, "\n") {
		fmt.Fprintln(w, "  "+l)
	}
}
