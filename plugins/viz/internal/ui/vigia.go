package ui

import (
	"context"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
)

// O watch das abas: com um client que observa (monitor.Observador), as listas
// das nove abas chegam pelo watch do namespace atual, em lotes, e as contagens
// das abas de fundo ficam exatas sem leitura nenhuma. O relógio de 2 s segue
// lendo a aba ativa: a resposta mostra se a conexão está de pé (o watch tenta
// de novo em silêncio) e só vira lista quando o watch não cobre a aba (o tipo
// falhou ou o client não observa).

// vigiarMsg pede para abrir o watch: o Init não guarda nada no modelo.
type vigiarMsg struct{}

// observacao é o watch aberto: o canal dos lotes e como pará-lo.
type observacao struct {
	id       int // sobe a cada watch novo: o lote de um anterior é descartado
	canal    <-chan monitor.Lote
	cancelar context.CancelFunc
}

// loteMsg é um lote do watch, ou o fim do canal (aberto falso).
type loteMsg struct {
	id     int
	lote   monitor.Lote
	aberto bool
}

// esperarLote espera o próximo lote do canal.
func esperarLote(id int, canal <-chan monitor.Lote) tea.Cmd {
	return func() tea.Msg {
		l, ok := <-canal
		return loteMsg{id: id, lote: l, aberto: ok}
	}
}

// vigiar para o watch aberto e abre outro no namespace atual; sem client que
// observa, nada abre.
func (m *Model) vigiar() tea.Cmd {
	m.Parar()
	m.vigiadas = 0
	obs, ok := m.client.(monitor.Observador)
	if !ok {
		return nil
	}
	ctx, cancelar := context.WithCancel(context.Background())
	canal := obs.Observar(ctx, m.filter.Namespace)
	if canal == nil {
		cancelar()
		return nil
	}
	m.vigia = observacao{id: m.vigia.id + 1, canal: canal, cancelar: cancelar}
	return esperarLote(m.vigia.id, canal)
}

// Parar fecha o watch aberto (sair do Viz, trocar de namespace). As cópias do
// modelo dividem o mesmo watch: parar uma para todas.
func (m Model) Parar() {
	if m.vigia.cancelar != nil {
		m.vigia.cancelar()
	}
}

// sair fecha o watch e encerra o programa.
func (m Model) sair() (tea.Model, tea.Cmd) {
	m.Parar()
	return m, tea.Quit
}

// abaDoKind é a aba de um tipo do watch.
func abaDoKind(kind string) (ResourceTab, bool) {
	for t := ResourceTab(0); t < tabCount; t++ {
		if abas[t].kind == kind {
			return t, true
		}
	}
	return 0, false
}

// vigiada diz se a lista da aba vem do watch.
func (m Model) vigiada(tab ResourceTab) bool {
	return m.vigiadas&(1<<uint(tab)) != 0
}

// receberLote guarda as listas do lote e espera o próximo. A aba que o watch
// não conseguiu listar sai do watch (a aba ativa volta ao relógio); sem
// permissão (RBAC), ela também perde a lista e fica sem número.
func (m *Model) receberLote(msg loteMsg) tea.Cmd {
	if msg.id != m.vigia.id || !msg.aberto {
		return nil // lote de um watch já parado, ou o fim do canal
	}
	if msg.lote.Namespace == m.filter.Namespace {
		for _, r := range msg.lote.Retratos {
			tab, ok := abaDoKind(r.Kind)
			if !ok {
				continue
			}
			bit := uint16(1) << uint(tab)
			if r.Err != nil {
				m.vigiadas &^= bit
				if r.Negado {
					m.esquecerAba(tab)
				}
				continue
			}
			m.vigiadas |= bit
			if tab == m.activeTab {
				m.receber(r.Lista)
			} else {
				m.guardarLista(r.Lista)
			}
		}
	}
	return esperarLote(m.vigia.id, m.vigia.canal)
}
