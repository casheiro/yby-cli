package tui

import (
	"context"
	"errors"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// O /modelo: a lista dos providers com os modelos de cada um e a troca, que vale só para
// a sessão (nada vai para o config.yaml).

// textoDicaModelo é a linha de estado com o /modelo aberto; cabe a 80 colunas ao lado do
// resumo da sessão.
const textoDicaModelo = "fora da lista: /modelo provider/modelo"

// receberModelos preenche a lista do /modelo com os providers detectados e os modelos de
// cada um, agrupados: a linha do provider, com o modelo padrão, e uma linha por modelo
// conhecido ("claude-cli/opus"). O que está em uso abre marcado. Uma detecção de uma
// lista já fechada é descartada.
func (m *Model) receberModelos(msg modelosMsg) tea.Cmd {
	sel := m.seletor
	if sel == nil || sel.tipo != seletorModelo || sel.pedido != msg.pedido {
		return nil
	}
	sel.estado, sel.modelos, sel.itens, sel.opcoes, sel.marcado = nil, nil, nil, nil, 0
	for _, mod := range msg.lista {
		if mod.Provider == nil {
			continue
		}
		i := len(sel.modelos)
		sel.modelos = append(sel.modelos, mod)
		padrao := mod.Padrao
		if padrao == "" {
			padrao = "modelo padrão do CLI"
		}
		// O que importa vem primeiro (o aviso e o modelo padrão): o nome longo do provider
		// é o que a largura corta.
		descricao := padrao + " · " + mod.Provider.Name()
		if mod.Aviso != "" {
			descricao = mod.Aviso + " · " + descricao
		}
		m.incluirOpcao(sel, mod.Rotulo, descricao, opcaoModelo{mod: i}, m.emUso(mod, ""))
		if mod.ComModelo == nil {
			continue
		}
		for _, modelo := range mod.Modelos {
			if modelo != mod.Padrao {
				m.incluirOpcao(sel, mod.Rotulo+"/"+modelo, "", opcaoModelo{mod: i, modelo: modelo}, m.emUso(mod, modelo))
			}
		}
		// Um modelo digitado fora da lista ganha a linha dele, para o ◆ abrir no que a
		// sessão usa e não no primeiro provider.
		if atual := m.config.ModeloIA; mod.Rotulo == m.config.AIModel && atual != "" &&
			!mesmoModelo(atual, mod.Padrao) && !contemModelo(mod.Modelos, atual) {
			m.incluirOpcao(sel, mod.Rotulo+"/"+atual, "", opcaoModelo{mod: i, modelo: atual}, true)
		}
	}
	if len(sel.itens) == 0 {
		e := ui.NovoEstadoVazio("nenhum provider de IA disponível", "")
		sel.estado = &e
	}
	if sel.pedir == "" {
		return nil
	}
	// /modelo <nome> fecha a lista em todos os casos: com ela aberta, a linha de estado
	// mostraria a frase da lista no lugar do aviso.
	m.seletor = nil
	return m.pedirModelo(sel.modelos, strings.TrimSpace(sel.pedir))
}

// incluirOpcao põe uma linha no /modelo; a que está em uso fica marcada, com o "em uso"
// na frente da descrição, onde a largura não corta.
func (m *Model) incluirOpcao(sel *seletor, nome, descricao string, op opcaoModelo, emUso bool) {
	if emUso {
		descricao = strings.TrimSuffix("em uso · "+descricao, " · ")
		sel.marcado = len(sel.itens)
	}
	sel.itens = append(sel.itens, itemLista{nome: nome, descricao: descricao})
	sel.opcoes = append(sel.opcoes, op)
}

// emUso diz se o provider com o modelo (vazio é o padrão) é o que a sessão usa.
func (m Model) emUso(mod Modelo, modelo string) bool {
	if mod.Rotulo != m.config.AIModel {
		return false
	}
	atual := m.config.ModeloIA
	if mesmoModelo(atual, mod.Padrao) {
		atual = ""
	}
	if mesmoModelo(modelo, mod.Padrao) {
		modelo = ""
	}
	return mesmoModelo(atual, modelo)
}

// pedirModelo resolve o /modelo <pedido>: "provider" troca o provider (com o modelo
// padrão), "provider/modelo" troca os dois e "modelo" troca o modelo no provider da
// sessão, ou no provider que tem o modelo na lista. Fora das listas, quem decide se o
// modelo existe é o provider da sessão. Só é "provider/modelo" quando o trecho antes da
// barra é um provider: "hf.co/autor/modelo" é um nome de modelo do Ollama.
func (m *Model) pedirModelo(lista []Modelo, pedido string) tea.Cmd {
	achar := func(rotulo string) (Modelo, bool) {
		for _, mod := range lista {
			if strings.EqualFold(mod.Rotulo, rotulo) {
				return mod, true
			}
		}
		return Modelo{}, false
	}
	if nome, modelo, ok := strings.Cut(pedido, "/"); ok {
		nome, modelo = strings.TrimSpace(nome), strings.TrimSpace(modelo)
		if mod, ok := achar(nome); ok {
			return m.trocarModelo(mod, modelo)
		}
		if ai.IsKnownProvider(strings.ToLower(nome)) {
			return m.avisoFalha("nenhum provider " + nome + " disponível")
		}
	}
	if mod, ok := achar(pedido); ok {
		return m.trocarModelo(mod, "")
	}
	if ai.IsKnownProvider(strings.ToLower(pedido)) {
		return m.avisoFalha("nenhum provider " + pedido + " disponível")
	}
	atual, temAtual := achar(m.config.AIModel)
	if temAtual && contemModelo(atual.Modelos, pedido) {
		return m.trocarModelo(atual, pedido)
	}
	for _, mod := range lista {
		if mod.ComModelo != nil && contemModelo(mod.Modelos, pedido) {
			return m.trocarModelo(mod, pedido)
		}
	}
	if temAtual {
		return m.trocarModelo(atual, pedido)
	}
	return m.avisoFalha("nenhum provider de IA tem o modelo " + pedido)
}

// contemModelo diz se o modelo está na lista, pela regra de mesmoModelo.
func contemModelo(lista []string, modelo string) bool {
	for _, m := range lista {
		if mesmoModelo(m, modelo) {
			return true
		}
	}
	return false
}

// mesmoModelo compara dois nomes de modelo sem diferenciar maiúsculas; como na factory,
// "llama3" é o "llama3:latest" que o /api/tags do Ollama devolve.
func mesmoModelo(a, b string) bool {
	return strings.EqualFold(a, b) || strings.EqualFold(a+":latest", b) || strings.EqualFold(a, b+":latest")
}

// trocarModelo passa a sessão para o provider com o modelo escolhido (vazio é o padrão
// dele). Com modelo, o provider é criado fora do Update (o do Ollama confere os modelos
// baixados), a linha de estado diz "trocando para …" e a troca acontece quando
// modeloCriadoMsg chega.
func (m *Model) trocarModelo(mod Modelo, modelo string) tea.Cmd {
	// A escolha nova vale sobre a criação que ainda não chegou.
	m.trocas++
	m.trocando, m.trocaPronta = "", nil
	for _, conhecido := range mod.Modelos {
		if mesmoModelo(conhecido, modelo) {
			modelo = conhecido
		}
	}
	if mesmoModelo(modelo, mod.Padrao) {
		modelo = ""
	}
	if m.emUso(mod, modelo) {
		return m.mostrarAviso(&avisoEstado{ok: true, texto: "a sessão já usa " + juntarIA(mod.Rotulo, modelo)})
	}
	if modelo == "" {
		m.usarProvider(mod, "", mod.Provider)
		return nil
	}
	if mod.ComModelo == nil {
		return m.avisoFalha("o " + mod.Rotulo + " não troca de modelo")
	}
	m.trocando = juntarIA(mod.Rotulo, modelo)
	troca, criar := m.trocas, mod.ComModelo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), tempoCriar)
		defer cancel()
		p, canonico, err := criar(ctx, modelo)
		if canonico == "" {
			canonico = modelo
		}
		return modeloCriadoMsg{troca: troca, mod: mod, modelo: canonico, provider: p, err: err}
	}
}

// receberModeloCriado aplica a troca pedida no /modelo, ou mostra por que o provider
// recusou o modelo: o motivo na linha de estado e, com a dica do provider (os modelos que
// ele conhece), uma nota na conversa, que não corta. A criação de uma escolha que outra
// já substituiu é descartada; a que chega durante a resposta espera o fim da rodada.
func (m *Model) receberModeloCriado(msg modeloCriadoMsg) tea.Cmd {
	if msg.troca != m.trocas {
		return nil
	}
	m.trocando = ""
	if msg.err != nil || msg.provider == nil {
		texto := "não deu para usar " + juntarIA(msg.mod.Rotulo, msg.modelo)
		var ye *ybyerrors.YbyError
		if errors.As(msg.err, &ye) && ye.Message != "" {
			texto = ye.Message
			if ye.Hint != "" {
				m.messages = append(m.messages, chatMessage{role: "nota", content: texto + ". " + ye.Hint})
				m.irParaOFim()
			}
		}
		return m.avisoFalha(texto)
	}
	if mesmoModelo(msg.modelo, msg.mod.Padrao) {
		msg.modelo, msg.provider = "", msg.mod.Provider
	}
	if m.emUso(msg.mod, msg.modelo) {
		return m.mostrarAviso(&avisoEstado{ok: true, texto: "a sessão já usa " + juntarIA(msg.mod.Rotulo, msg.modelo)})
	}
	if m.state != stateIdle {
		m.trocaPronta, m.trocando = &msg, juntarIA(msg.mod.Rotulo, msg.modelo)
		return nil
	}
	m.usarProvider(msg.mod, msg.modelo, msg.provider)
	m.remarcarModelos()
	return nil
}

// remarcarModelos refaz o /modelo aberto depois de uma troca, para o ◆ e o "em uso"
// seguirem a sessão.
func (m *Model) remarcarModelos() {
	if sel := m.seletor; sel != nil && sel.tipo == seletorModelo && sel.estado == nil && sel.pedir == "" {
		m.receberModelos(modelosMsg{pedido: sel.pedido, lista: sel.modelos})
	}
}

// usarProvider passa a sessão para o provider e registra a troca na conversa; o
// cabeçalho mostra o provider e o modelo novos na hora.
func (m *Model) usarProvider(mod Modelo, modelo string, p ai.Provider) {
	rotulo := mod.Rotulo
	nota := "modelo trocado para " + juntarIA(rotulo, modelo)
	switch {
	case modelo == "" && rotulo != m.config.AIModel:
		nota = "provider trocado para " + rotulo
	case modelo == "":
		nota = "modelo trocado para o padrão do " + rotulo
	}
	m.provider = p
	// Modelo vazio é o padrão do provider: quando o config fixa um (CLIs), é ele que roda e
	// o cabeçalho mostra.
	exibido := modelo
	if exibido == "" {
		exibido = mod.Padrao
	}
	m.config.AIModel, m.config.ModeloIA = rotulo, exibido
	f := *m.fluxo
	f.Provider = p
	m.fluxo = &f
	m.messages = append(m.messages, chatMessage{role: "nota", content: nota})
	m.irParaOFim()
}

// rotuloIA é o provider e o modelo da sessão, como no cabeçalho: "claude-cli · opus".
func (m Model) rotuloIA() string {
	return juntarIA(m.config.AIModel, m.config.ModeloIA)
}

// juntarIA junta o provider e o modelo; sem modelo, fica só o provider.
func juntarIA(rotulo, modelo string) string {
	if rotulo == "" || modelo == "" {
		return rotulo
	}
	return rotulo + " · " + modelo
}
