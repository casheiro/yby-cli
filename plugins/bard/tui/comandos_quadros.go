package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// Quadros que /contexto, /ferramentas e /ajuda põem na conversa, como o "CONTEXTO DESTA
// SESSÃO" da prancha "tema kraft".

// InfoContexto é o que o /contexto mostra além do cabeçalho: o cluster por extenso, de
// onde vêm as notas e o projeto. O histórico que vai no system prompt vem da Config
// (Historico), contado quando o prompt foi montado.
type InfoContexto struct {
	Cluster    string    // o cluster por extenso ("staging · eks · us-east-1"); vazio usa o do cabeçalho
	Projeto    string    // "yby-cli · topologia standard"
	Embeddings string    // de onde vêm os embeddings ("embeddings locais")
	Synapstor  bool      // há índice do Synapstor
	Notas      int       // UKIs indexadas
	NotasEm    time.Time // última indexação; zero omite
}

// quadroMsg é o resultado de um comando na conversa: um quadro com linhas chave e valor.
type quadroMsg struct {
	rotulo string
	linhas []linhaQuadro
}

// linhaQuadro é uma linha do quadro, já com os estilos; as duas vazias dão uma linha em
// branco.
type linhaQuadro struct {
	chave, valor string
}

// larguraChaveQuadro é a coluna das chaves, como na prancha (16 células com o recuo).
const larguraChaveQuadro = 14

// renderizarQuadro desenha o quadro na largura dada, com as chaves numa coluna.
func renderizarQuadro(q quadroMsg, largura int) string {
	return renderizarQuadroCom(q, largura, ui.QuadroNormal)
}

// renderizarQuadroCom é o renderizarQuadro com a variante do quadro (em foco nas camadas).
func renderizarQuadroCom(q quadroMsg, largura int, variante ui.VarianteQuadro) string {
	coluna := larguraChaveQuadro
	for _, l := range q.linhas {
		coluna = max(coluna, lipgloss.Width(l.chave)+2)
	}
	linhas := make([]string, 0, len(q.linhas))
	for _, l := range q.linhas {
		if l.chave == "" && l.valor == "" {
			linhas = append(linhas, "")
			continue
		}
		linhas = append(linhas, " "+l.chave+strings.Repeat(" ", coluna-lipgloss.Width(l.chave))+l.valor)
	}
	return ui.Quadro(q.rotulo, strings.Join(linhas, "\n"), largura, variante)
}

// quadroContexto monta o "contexto desta sessão": o que o Bard leva para a IA antes de
// cada pergunta.
func (m Model) quadroContexto(agora time.Time) quadroMsg {
	s := ui.S()
	var info InfoContexto
	if m.config.Contexto != nil {
		info = m.config.Contexto()
	}
	chave := func(c string) string { return s.TintaSuave.Render(c) }
	ou := func(v, nada string) string {
		if v == "" {
			return s.TintaSuave.Render(nada)
		}
		return v
	}
	cluster := info.Cluster
	if cluster == "" {
		cluster = m.config.Cluster
	}
	provider := ou(m.config.AIModel, "nenhum")
	if info.Embeddings != "" {
		provider += s.TintaSuave.Render(" · " + info.Embeddings)
	}
	notas := s.TintaSuave.Render("sem Synapstor")
	if info.Synapstor {
		notas = plural(info.Notas, "UKI indexada", "UKIs indexadas")
		if !info.NotasEm.IsZero() {
			notas += s.TintaSuave.Render(" · atualizado " + haQuanto(info.NotasEm, agora))
		}
	}
	historico := s.TintaSuave.Render("nenhuma conversa anterior")
	if h := m.config.Historico; h.Mensagens > 0 {
		n := h.Mensagens
		historico = fmt.Sprintf("últimas %s de %s", plural(n, "mensagem", "mensagens"), plural(max(h.Sessoes, 1), "sessão", "sessões"))
		if n == 1 {
			historico = "1 mensagem de 1 sessão"
		}
	}

	linhas := []linhaQuadro{
		{chave("cluster"), ou(cluster, "sem cluster")},
		{chave("namespace"), ou(m.config.Namespace, "sem namespace")},
		{chave("provider"), provider},
	}
	if info.Projeto != "" {
		linhas = append(linhas, linhaQuadro{chave("projeto"), info.Projeto})
	}
	linhas = append(linhas,
		linhaQuadro{chave("notas"), notas},
		linhaQuadro{chave("ferramentas"), resumoFerramentas(m.ferramentas())},
		linhaQuadro{chave("histórico"), historico},
	)
	return quadroMsg{rotulo: "contexto desta sessão", linhas: linhas}
}

// haQuanto diz há quanto tempo foi t: "há 5 min", "há 3 horas", "ontem", "há 2 dias".
func haQuanto(t, agora time.Time) string {
	d := agora.Sub(t)
	switch {
	case d < time.Minute:
		return "agora há pouco"
	case d < time.Hour:
		return fmt.Sprintf("há %d min", int(d/time.Minute))
	case d < 24*time.Hour:
		return "há " + plural(int(d/time.Hour), "hora", "horas")
	case d < 48*time.Hour:
		return "ontem"
	}
	return "há " + plural(int(d/(24*time.Hour)), "dia", "dias")
}

// ferramentas é a situação das ferramentas: a da Config (nos testes) ou a do registry.
func (m Model) ferramentas() []tools.Situacao {
	if m.config.Ferramentas != nil {
		return m.config.Ferramentas()
	}
	return tools.Situacoes()
}

// ordemFamilias põe as nativas na ordem da prancha; as outras vêm depois, pelo nome.
var ordemFamilias = map[string]int{"kubectl": 0, "sentinel": 1, "atlas": 2}

// resumoFerramentas é a linha "ferramentas" do /contexto: cada família nativa com ✓
// (todas disponíveis) ou ▲, e as externas que não carregaram.
func resumoFerramentas(lista []tools.Situacao) string {
	s, g := ui.S(), ui.G()
	disponivel := map[string]bool{}
	var familias []string
	invalidas := 0
	for _, f := range lista {
		switch {
		case f.Invalida:
			invalidas++
			continue
		case f.Externa:
			continue
		}
		if _, ok := disponivel[f.Familia]; !ok {
			familias = append(familias, f.Familia)
			disponivel[f.Familia] = true
		}
		disponivel[f.Familia] = disponivel[f.Familia] && f.Disponivel
	}
	sort.SliceStable(familias, func(i, j int) bool {
		oi, ok := ordemFamilias[familias[i]]
		if !ok {
			oi = len(ordemFamilias)
		}
		oj, ok := ordemFamilias[familias[j]]
		if !ok {
			oj = len(ordemFamilias)
		}
		if oi != oj {
			return oi < oj
		}
		return familias[i] < familias[j]
	})
	var partes []string
	for _, f := range familias {
		marca := s.Ok.Render(g.Feito)
		if !disponivel[f] {
			marca = s.Aviso.Render(g.Alerta)
		}
		partes = append(partes, marca+" "+f)
	}
	if invalidas > 0 {
		partes = append(partes, s.Aviso.Render(g.Alerta)+" "+plural(invalidas, "externa inválida", "externas inválidas"))
	}
	if len(partes) == 0 {
		return s.TintaSuave.Render("nenhuma")
	}
	return strings.Join(partes, "  ")
}

// quadroFerramentas lista cada ferramenta com o que faz, ou por que não está disponível.
func (m Model) quadroFerramentas() quadroMsg {
	s, g := ui.S(), ui.G()
	var linhas []linhaQuadro
	for _, f := range m.ferramentas() {
		nome := f.Nome
		if !f.Invalida {
			nome = strings.ReplaceAll(nome, "_", " ")
		}
		switch {
		case f.Invalida:
			linhas = append(linhas, linhaQuadro{s.Erro.Render(g.Falha) + " " + nome, s.TintaSuave.Render(f.Nota)})
		case !f.Disponivel:
			linhas = append(linhas, linhaQuadro{s.Aviso.Render(g.Alerta) + " " + nome, s.TintaSuave.Render(f.Nota)})
		default:
			descricao := minuscula(strings.TrimSuffix(primeiraLinha(f.Descricao), "."))
			if f.Externa {
				descricao = strings.TrimPrefix(descricao+" · externa", " · ")
			}
			linhas = append(linhas, linhaQuadro{s.Ok.Render(g.Feito) + " " + nome, s.TintaSuave.Render(descricao)})
		}
	}
	if len(linhas) == 0 {
		linhas = []linhaQuadro{{s.TintaSuave.Render("nenhuma ferramenta registrada"), ""}}
	}
	return quadroMsg{rotulo: "ferramentas", linhas: linhas}
}

// minuscula baixa a primeira letra, para as notas seguirem o tom da carta.
func minuscula(t string) string {
	r, n := utf8.DecodeRuneInString(t)
	if n == 0 {
		return t
	}
	return string(unicode.ToLower(r)) + t[n:]
}

// exemplosPerguntas são as perguntas do /ajuda.
var exemplosPerguntas = []string{
	"por que o pod api está reiniciando?",
	"lista os pods com problema no namespace payments",
	"faz um scan de segurança no namespace default",
	"mostra a topologia do projeto",
}

// teclaAjuda é uma linha das teclas no quadro AJUDA: as teclas, com o rótulo e a ação
// que elas têm na legenda, e quando valem.
type teclaAjuda struct {
	teclas []key.Binding
	quando string
}

// teclasAjuda são as teclas da conversa e da resposta, na ordem da legenda; as das listas
// e o ctrl+c entram no quadroAjuda. Os rótulos e as ações vêm dos mesmos key.Binding da legenda
// (o keymap da carta e as teclas desta tela), para a ajuda não divergir dela.
func teclasAjuda() []teclaAjuda {
	k := ui.DefaultKeys()
	quebra := k.Texto.NovaLinha
	quebra.SetHelp(ui.RotuloTecla("ctrl+j"), quebra.Help().Desc)
	rolar := k.Texto.RolarCima
	rolar.SetHelp(rolar.Help().Key, "rolar")
	return []teclaAjuda{
		{[]key.Binding{k.Texto.Enviar}, "com texto no campo"},
		{[]key.Binding{quebra}, ""},
		{[]key.Binding{k.Texto.Comandos}, "no campo vazio"},
		{[]key.Binding{k.Global.Ajuda}, "no campo vazio, também durante a resposta"},
		{[]key.Binding{k.Texto.EditarUltima}, "no campo vazio, traz a última pergunta"},
		{[]key.Binding{rolar, k.Texto.RolarBaixo}, "também com a roda do mouse"},
		{[]key.Binding{teclaVerEtapa()}, "no campo vazio, abre a saída da ferramenta"},
		{[]key.Binding{k.Texto.Interromper}, "durante a resposta"},
		{[]key.Binding{teclaCopiarSaida()}, "o último comando, ou a saída aberta"},
	}
}

// quadroAjuda lista as teclas do Bard e, com exemplos, perguntas de exemplo. As teclas
// das listas (as da legenda de /comandos) vão numa linha só, como na legenda.
func quadroAjuda(exemplos bool) quadroMsg {
	s := ui.S()
	var linhas []linhaQuadro
	for _, t := range teclasAjuda() {
		rotulos := make([]string, len(t.teclas))
		for i, b := range t.teclas {
			rotulos[i] = b.Help().Key
		}
		valor := s.Tinta.Render(t.teclas[0].Help().Desc)
		if t.quando != "" {
			valor += s.TintaSuave.Render(" · " + t.quando)
		}
		linhas = append(linhas, linhaQuadro{s.Negrito.Render(strings.Join(rotulos, " ")), valor})
	}
	var listas []string
	for _, b := range teclasComandos() {
		listas = append(listas, s.Negrito.Render(b.Help().Key)+" "+s.TintaSuave.Render(b.Help().Desc))
	}
	sair := ui.DefaultKeys().Global.SairForcado
	linhas = append(linhas,
		linhaQuadro{s.TintaSuave.Render("nas listas"), strings.Join(listas, "  ")},
		linhaQuadro{s.Negrito.Render(sair.Help().Key), s.Tinta.Render(sair.Help().Desc)})
	if !exemplos {
		return quadroMsg{rotulo: "ajuda", linhas: linhas}
	}
	linhas = append(linhas, linhaQuadro{})
	for i, p := range exemplosPerguntas {
		chave := ""
		if i == 0 {
			chave = s.TintaSuave.Render("exemplos")
		}
		linhas = append(linhas, linhaQuadro{chave, p})
	}
	return quadroMsg{rotulo: "ajuda", linhas: linhas}
}

// camadaAjuda é o quadro AJUDA da camada que o ? abre durante a resposta: só as teclas,
// em foco, na largura da conversa.
func (m Model) camadaAjuda() string {
	return renderizarQuadroCom(quadroAjuda(false), m.viewport.Width, ui.QuadroFoco)
}
