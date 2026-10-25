package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// gravarHistorico escreve as entradas no histórico do diretório atual.
func gravarHistorico(t *testing.T, entries ...HistoryEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(historyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		dados, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(dados)
		b.WriteString("\n")
	}
	if err := os.WriteFile(historyFile, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// entrada monta uma entrada do histórico de h horas atrás.
func entrada(sessao, papel, texto string, h int) HistoryEntry {
	ts := time.Now().Add(-time.Duration(h) * time.Hour).Format(time.RFC3339)
	return HistoryEntry{Role: papel, Content: texto, Timestamp: ts, SessionID: sessao}
}

func TestSessoesSalvas(t *testing.T) {
	defer chdir(t, t.TempDir())()
	gravarHistorico(t,
		entrada("antiga", "assistant", "olá", 50),
		entrada("antiga", "user", "primeira da antiga", 49),
		entrada("recente", "user", "qual a situação do storage?", 2),
		entrada("recente", "assistant", "seis volumes", 1),
	)
	lista, err := sessoesSalvas()
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 2 || lista[0].ID != "recente" || lista[1].ID != "antiga" {
		t.Fatalf("da mais recente para a mais antiga: %+v", lista)
	}
	if lista[0].Mensagens != 2 || lista[0].Primeira != "qual a situação do storage?" || lista[1].Primeira != "primeira da antiga" {
		t.Errorf("resumo das sessões: %+v", lista)
	}
	if lista[0].Inicio.IsZero() || !lista[0].Fim.After(lista[0].Inicio) {
		t.Errorf("início e fim: %+v", lista[0])
	}

	// Sem histórico, nenhuma sessão e nenhum erro.
	defer chdir(t, t.TempDir())()
	if lista, err := sessoesSalvas(); err != nil || len(lista) != 0 {
		t.Errorf("sem histórico: %v %v", lista, err)
	}
}

func TestRetomador(t *testing.T) {
	defer chdir(t, t.TempDir())()
	var entries []HistoryEntry
	for i := 0; i < maxHistoryEntries+5; i++ {
		entries = append(entries, entrada("s1", "user", "pergunta antiga", 40))
	}
	entries = append(entries, entrada("s1", "user", "pergunta final", 1), entrada("s2", "user", "outra sessão", 1))
	gravarHistorico(t, entries...)

	retomar := retomador(map[string]interface{}{}, BardConfig{}, nil)
	r, err := retomar("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Mensagens) != maxHistoryEntries+6 {
		t.Errorf("a tela recebe a sessão inteira: %d mensagens", len(r.Mensagens))
	}
	if ultima := r.Mensagens[len(r.Mensagens)-1]; ultima.Papel != "user" || ultima.Texto != "pergunta final" || ultima.Hora.IsZero() {
		t.Errorf("última mensagem: %+v", ultima)
	}
	if !strings.Contains(r.SystemPrompt, "pergunta final") || strings.Contains(r.SystemPrompt, "outra sessão") {
		t.Errorf("o system prompt leva só a sessão retomada")
	}
	if n := strings.Count(r.SystemPrompt, "pergunta antiga"); n != maxHistoryEntries-1 {
		t.Errorf("o system prompt leva as últimas %d mensagens: %d antigas", maxHistoryEntries, n)
	}
	if r.Historico.Mensagens != maxHistoryEntries || r.Historico.Sessoes != 1 {
		t.Errorf("a conta do histórico é a do prompt da sessão retomada: %+v", r.Historico)
	}
	if r, err := retomar("nenhuma"); err != nil || len(r.Mensagens) != 0 || r.SystemPrompt != "" {
		t.Errorf("sessão que não existe: %+v %v", r, err)
	}
}

func TestProjetoAtual(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "meu-projeto")
	if err := os.MkdirAll(filepath.Join(dir, ".yby"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := projetoAtual(dir); got != "meu-projeto" {
		t.Errorf("sem manifesto: %q", got)
	}
	manifesto := "metadata:\n  name: yby-cli\nspec:\n  topology: standard\n"
	if err := os.WriteFile(filepath.Join(dir, ".yby", "project.yaml"), []byte(manifesto), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projetoAtual(dir); got != "yby-cli · topologia standard" {
		t.Errorf("com manifesto: %q", got)
	}
}

func TestRotuloEmbeddings(t *testing.T) {
	casos := []struct {
		p    ai.Provider
		want string
	}{
		{nil, "sem embeddings"},
		{&ai.OllamaProvider{}, "embeddings locais"},
		{nomeado{nome: "Local Embedding (all-MiniLM-L6-v2)"}, "embeddings locais"},
		{&ai.OpenAIProvider{}, "embeddings openai"},
	}
	for _, c := range casos {
		if got := rotuloEmbeddings(c.p); got != c.want {
			t.Errorf("rotuloEmbeddings(%T) = %q, esperado %q", c.p, got, c.want)
		}
	}
}

func TestInfoContexto(t *testing.T) {
	t.Setenv("YBY_ENV", "")
	dir := t.TempDir()
	defer chdir(t, dir)()
	info := infoContexto(nil, nil, filepath.Join(dir, ".synapstor", ".index"), dir, "staging")()
	if info.Synapstor || info.Embeddings != "sem embeddings" || info.Projeto != filepath.Base(dir) || info.Cluster != "staging" {
		t.Errorf("sem Synapstor nem ambiente: %+v", info)
	}
}

// TestContarHistorico: a conta é do que foi para o prompt. As mensagens que a conversa
// grava depois no mesmo arquivo não entram nela.
func TestContarHistorico(t *testing.T) {
	defer chdir(t, t.TempDir())()
	gravarHistorico(t, entrada("s1", "user", "a", 3), entrada("s2", "user", "b", 2), entrada("s2", "assistant", "c", 1))
	historico := loadHistory()
	conta := contarHistorico(historico)
	if conta.Mensagens != 3 || conta.Sessoes != 2 {
		t.Errorf("histórico do prompt: %+v", conta)
	}
	saveMessage("user", "oi", "atual")
	saveMessage("assistant", "olá", "atual")
	if deNovo := contarHistorico(historico); deNovo != conta {
		t.Errorf("a conta não muda com a conversa: %+v", deNovo)
	}
	if vazio := contarHistorico(nil); vazio.Mensagens != 0 || vazio.Sessoes != 0 {
		t.Errorf("sem histórico: %+v", vazio)
	}
}

func TestClusterPorExtenso(t *testing.T) {
	t.Setenv("YBY_ENV", "")
	dir := t.TempDir()
	if got := clusterPorExtenso(dir, "staging"); got != "staging" {
		t.Errorf("sem environments.yaml: %q", got)
	}
	if got := clusterPorExtenso(dir, ""); got != "" {
		t.Errorf("sem cluster: %q", got)
	}
	ambientes := "current: staging\nenvironments:\n" +
		"  staging:\n    type: eks\n    kube_context: staging\n    cloud:\n      provider: aws\n      region: us-east-1\n" +
		"  gke:\n    type: gke\n    cloud:\n      zone: us-central1-a\n" +
		"  local:\n    type: local\n"
	if err := os.MkdirAll(filepath.Join(dir, ".yby"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".yby", "environments.yaml"), []byte(ambientes), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := clusterPorExtenso(dir, "staging"); got != "staging · eks · us-east-1" {
		t.Errorf("ambiente eks: %q", got)
	}
	// O kubectl aponta para outro contexto: o tipo e a região do ambiente não valem.
	if got := clusterPorExtenso(dir, "k3d-local"); got != "k3d-local" {
		t.Errorf("outro contexto: %q", got)
	}
	t.Setenv("YBY_ENV", "gke")
	if got := clusterPorExtenso(dir, "gke_proj_us-central1-a_c1"); got != "gke_proj_us-central1-a_c1 · gke · us-central1-a" {
		t.Errorf("ambiente gke sem kube_context, com zona: %q", got)
	}
	t.Setenv("YBY_ENV", "local")
	if got := clusterPorExtenso(dir, "k3d-yby"); got != "k3d-yby · local" {
		t.Errorf("ambiente local: %q", got)
	}
}
