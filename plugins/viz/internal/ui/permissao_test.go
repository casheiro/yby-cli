package ui

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

// negado é o Forbidden que o RetryClient devolve ao listar o recurso.
func negado(recurso string) error {
	return fmt.Errorf("falha ao listar %s: %w", recurso,
		apierrors.NewForbidden(schema.GroupResource{Resource: recurso}, "", errors.New("RBAC")))
}

// leituraDe é a resposta de uma leitura da aba.
func leituraDe(tab ResourceTab, resposta tea.Msg) leituraMsg {
	return leituraMsg{abas: []ResourceTab{tab}, respostas: []tea.Msg{resposta}}
}

// TestPermissao_NegadoNaoReconecta: o Forbidden da aba ativa não é queda do
// cluster: sem reconexão nem "sem acesso", a aba diz o que falta e onde, e a
// dica muda com o namespace e com o tipo do recurso.
func TestPermissao_NegadoNaoReconecta(t *testing.T) {
	casos := []struct {
		nome, ns    string
		tab         ResourceTab
		recurso     string
		frase, dica string
		comDados    bool
	}{
		{"todos os namespaces, com dados", "", TabPods, "pods", "sem permissão para listar pods em todos os namespaces", "n escolhe um namespace", true},
		{"todos os namespaces, sem dados", "", TabPods, "pods", "sem permissão para listar pods em todos os namespaces", "n escolhe um namespace", false},
		{"um namespace", "payments", TabDeployments, "deployments", "sem permissão para listar deployments em payments", "n troca o namespace", true},
		{"nós, do cluster", "", TabNodes, "nodes", "sem permissão para listar nós no cluster", "r tenta de novo", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			m := comJanela(NewModel(&MockClient{}))
			m.filter.Namespace = c.ns
			if c.comDados {
				m, _ = atualizar(m, []monitor.Service{{Name: "api", Namespace: "loja"}})
			}
			m.activeTab = c.tab
			for i := 0; i < maxRetries+1; i++ { // várias leituras do relógio seguidas
				m, _ = atualizar(m, leituraMsg{namespace: c.ns, relogio: true, abas: []ResourceTab{c.tab}, respostas: []tea.Msg{negado(c.recurso)}})
			}
			if m.err != nil || m.reconnecting || m.retryCount != 0 {
				t.Fatalf("o negado derrubou a conexão: err %v, reconectando %v, tentativas %d", m.err, m.reconnecting, m.retryCount)
			}
			if !m.negada(c.tab) || m.carregada(c.tab) {
				t.Fatalf("a aba deveria ficar negada e sem número: negada %v, carregada %v", m.negada(c.tab), m.carregada(c.tab))
			}
			tela := ansi.Strip(m.View())
			if !strings.Contains(tela, c.frase) || !strings.Contains(tela, c.dica) {
				t.Fatalf("esperava %q com a dica %q:\n%s", c.frase, c.dica, tela)
			}
			if strings.Contains(tela, "sem acesso ao cluster") || strings.Contains(tela, "reconectando") {
				t.Fatalf("o negado não é falta de conexão:\n%s", tela)
			}
			if c.dica != "n escolhe um namespace" && strings.Contains(tela, "n escolhe um namespace") {
				t.Fatalf("a dica do namespace não serve aqui:\n%s", tela)
			}
			// A lista que chega depois (o RBAC mudou, o r) tira o negado.
			m, _ = atualizar(m, muitosPods(1))
			if c.tab == TabPods && m.negada(TabPods) {
				t.Fatal("a lista que chegou deveria tirar o negado")
			}
		})
	}
}

// TestPermissao_RelogioNaoListaAbaNegada: com o watch negado na aba, o relógio
// para de fazer LIST dela a cada 2 s (só a sonda de conexão), e a tela diz que
// falta permissão; trocar de namespace tenta de novo.
func TestPermissao_RelogioNaoListaAbaNegada(t *testing.T) {
	c := novoCluster(podDe("payments", "web-1"))
	var listas atomic.Int32
	c.cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		listas.Add(1)
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "", errors.New("RBAC"))
	})
	m, b := abrir(t, vizVigiado(c, "payments"))
	defer m.Parar()
	m = b.ate(t, m, func(m Model) bool { return contagem(m, TabPods) == 1 && m.negada(TabNodes) })
	m.activeTab = TabNodes
	antes := listas.Load()
	for i := 0; i < 3; i++ {
		var cmd tea.Cmd
		m, cmd = atualizar(m, tickMsg(time.Now()))
		for _, msg := range rodarTudo(cmd) {
			if l, ok := msg.(leituraMsg); ok {
				m, _ = atualizar(m, l)
			}
		}
		if m.err != nil || m.reconnecting {
			t.Fatalf("o tique da aba negada derrubou a conexão: err %v", m.err)
		}
	}
	if n := listas.Load() - antes; n != 0 {
		t.Fatalf("o relógio listou nós %d vez(es) com a aba negada", n)
	}
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "sem permissão para listar nós no cluster") {
		t.Fatalf("a aba negada deveria dizer que falta permissão:\n%s", tela)
	}
	m.esquecerListas()
	if m.negada(TabNodes) {
		t.Fatal("o namespace novo deveria tentar a aba de novo")
	}
}

// rodarTudo roda o comando e os de dentro de um Batch, devolvendo as mensagens.
func rodarTudo(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	lote, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, sub := range lote {
		msgs = append(msgs, rodarTudo(sub)...)
	}
	return msgs
}

// TestErro_CredencialExec: o plugin exec do kubeconfig que não dá a credencial
// leva à dica do yby cloud refresh, e não à de conferir se o cluster está no ar.
func TestErro_CredencialExec(t *testing.T) {
	m := comJanela(NewModel(&MockClient{}))
	exec := fmt.Errorf(`falha ao listar pods: Get "https://eks/api/v1/pods": getting credentials: exec: executable aws failed with exit code 255`)
	m, _ = atualizar(m, leituraDe(TabPods, exec))
	tela := ansi.Strip(m.View())
	if !strings.Contains(tela, "yby cloud refresh") || strings.Contains(tela, "Confira se o cluster está no ar") {
		t.Fatalf("a falha da credencial deveria sugerir o yby cloud refresh:\n%s", tela)
	}

	m = comJanela(NewModel(&MockClient{}))
	m, _ = atualizar(m, leituraDe(TabPods, errors.New("dial tcp 127.0.0.1:6443: connection refused")))
	if tela := ansi.Strip(m.View()); !strings.Contains(tela, "Confira se o cluster está no ar") || strings.Contains(tela, "yby cloud refresh") {
		t.Fatalf("a queda de conexão segue com a dica de conferir o cluster:\n%s", tela)
	}
}
