package ui

import (
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

// Check escreve uma linha de verificação com colunas alinhadas por largura em células:
// glifo, nome, versão e nota. Versão e nota são opcionais.
func Check(w io.Writer, tipo TipoCheck, nome, versao, nota string) {
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
	corpo := nome
	if nota != "" || versao != "" {
		corpo = padDireita(nome, msgColNome) + padDireita(versao, msgColVersao) + nota
	}
	corpo = strings.TrimRight(corpo, " ")
	if tipo == CheckNeutro {
		fmt.Fprintln(w, "  "+s.TintaSuave.Render(glifo+" "+corpo))
		return
	}
	fmt.Fprintln(w, "  "+est.Render(glifo)+" "+s.Tinta.Render(corpo))
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
// tem um) e "detalhes" (contexto diagnóstico ou, na falta dele, a dica de depuração).
func RenderError(e *yerrors.YbyError) string {
	if e == nil {
		return ""
	}
	s := S()
	var b strings.Builder
	titulo := s.Erro.Render(G().Falha) + " " + s.Negrito.Render(linhaUnica(e.Message))
	b.WriteString(duasPontas(titulo, s.TintaSuave.Render(e.Code), msgLargura) + "\n")

	linha := func(rotulo, texto string) {
		b.WriteString("  " + s.Rotulo.Render(padDireita(rotulo, 10)) + texto + "\n")
	}
	if e.Cause != nil {
		linha("causa", s.Tinta.Render(linhaUnica(e.Cause.Error())))
	}
	hint := e.GetHint()
	if hint != "" {
		linha("faça", s.Negrito.Render(linhaUnica(hint)))
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
		linha("detalhes", s.TintaSuave.Render(linhaUnica(strings.Join(pares, " · "))))
	} else if hint != yerrors.GenericHint {
		linha("detalhes", s.TintaSuave.Render("rode de novo com --log-level=debug"))
	}
	return b.String()
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
