package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	ybyctx "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/plugins/bard/tui"
	"gopkg.in/yaml.v3"
)

// Ligação dos comandos / da TUI com o histórico (history.go), os providers de IA e o
// projeto. A TUI recebe só funções, para os testes trocarem cada uma.

// sessoesSalvas resume as conversas do histórico para o /sessões, da mais recente para
// a mais antiga, com a primeira pergunta de cada uma.
func sessoesSalvas() ([]tui.Sessao, error) {
	entries, err := loadAllEntries()
	if err != nil {
		return nil, err
	}
	primeiras := map[string]string{}
	for _, e := range entries {
		if _, ok := primeiras[e.SessionID]; !ok && e.Role == "user" {
			primeiras[e.SessionID] = e.Content
		}
	}
	var lista []tui.Sessao
	for _, s := range listSessions(entries) {
		lista = append(lista, tui.Sessao{
			ID:        s.SessionID,
			Mensagens: s.MessageCount,
			Inicio:    s.FirstMessage.Local(),
			Fim:       s.LastMessage.Local(),
			Primeira:  primeiras[s.SessionID],
		})
	}
	sort.SliceStable(lista, func(i, j int) bool { return lista[i].Fim.After(lista[j].Fim) })
	return lista, nil
}

// retomador devolve a função do /sessões que carrega uma conversa do histórico e refaz
// o system prompt com as últimas mensagens dela.
func retomador(ctxData map[string]interface{}, bardCfg BardConfig, clusterCtx *ClusterContext) func(string) (tui.Retomada, error) {
	return func(id string) (tui.Retomada, error) {
		entries, err := loadAllEntries()
		if err != nil {
			return tui.Retomada{}, err
		}
		todas := loadSessionHistory(entries, id, len(entries))
		if len(todas) == 0 {
			return tui.Retomada{}, nil
		}
		recentes := loadSessionHistory(entries, id, maxHistoryEntries)
		r := tui.Retomada{
			SystemPrompt: buildSystemPrompt(ctxData, bardCfg, recentes, clusterCtx),
			Historico:    contarHistorico(recentes),
		}
		for _, e := range todas {
			hora, _ := time.Parse(time.RFC3339, e.Timestamp)
			if !hora.IsZero() {
				hora = hora.Local()
			}
			r.Mensagens = append(r.Mensagens, tui.MensagemSalva{Papel: e.Role, Texto: e.Content, Hora: hora})
		}
		return r, nil
	}
}

// modelosDetectados lista os providers de IA disponíveis, na ordem de ai.priority, com os
// modelos de cada um (ai.ListModels), para o /modelo. Cada um confere se está disponível;
// nenhum é chamado para gerar texto.
func modelosDetectados(ctx context.Context) []tui.Modelo {
	var lista []tui.Modelo
	for _, p := range ai.GetAllAvailableProviders(ctx) {
		lista = append(lista, modeloDetectado(ctx, p))
	}
	return lista
}

// modeloDetectado junta ao provider os modelos dele e a criação com um modelo escolhido,
// que vale só para a sessão.
func modeloDetectado(ctx context.Context, p ai.Provider) tui.Modelo {
	rotulo := rotuloProvider(p)
	mod := tui.Modelo{Rotulo: rotulo, Provider: p}
	if !ai.IsKnownProvider(rotulo) {
		return mod
	}
	cat := ai.ListModels(ctx, rotulo)
	mod.Padrao, mod.Modelos, mod.Aviso = cat.Default, cat.Models, cat.Warning
	mod.ComModelo = func(ctx context.Context, modelo string) (ai.Provider, string, error) {
		return ai.GetProviderWithModel(ctx, rotulo, modelo)
	}
	return mod
}

// contarHistorico conta as entradas que vão para o system prompt e as sessões delas, para
// o /contexto mostrar o que foi no prompt e não o arquivo, que cresce com a conversa.
func contarHistorico(entries []HistoryEntry) tui.HistoricoPrompt {
	return tui.HistoricoPrompt{Mensagens: len(entries), Sessoes: len(listSessions(entries))}
}

// infoContexto devolve a função do /contexto: o cluster por extenso, as notas do
// Synapstor, o projeto e os embeddings, lidos na hora.
func infoContexto(vectorStore *ai.VectorStore, embeddings ai.Provider, storePath, dir, cluster string) func() tui.InfoContexto {
	return func() tui.InfoContexto {
		info := tui.InfoContexto{
			Cluster:    clusterPorExtenso(dir, cluster),
			Projeto:    projetoAtual(dir),
			Embeddings: rotuloEmbeddings(embeddings),
		}
		if vectorStore != nil {
			info.Synapstor, info.Notas = true, vectorStore.Count()
			if st, err := os.Stat(storePath); err == nil {
				info.NotasEm = st.ModTime()
			}
		}
		return info
	}
}

// clusterPorExtenso junta ao contexto do kubectl o tipo e a região do ambiente ativo do
// .yby/environments.yaml ("staging · eks · us-east-1"). Só quando o ambiente aponta para
// esse contexto (ou não diz qual): de outro cluster, o tipo e a região enganariam. Sem
// ambiente, fica o contexto como veio.
func clusterPorExtenso(dir, cluster string) string {
	if cluster == "" {
		return ""
	}
	_, env, err := ybyctx.NewManager(dir).GetCurrent()
	if err != nil || env == nil || (env.KubeContext != "" && env.KubeContext != cluster) {
		return cluster
	}
	partes := []string{cluster}
	if env.Type != "" {
		partes = append(partes, env.Type)
	}
	if c := env.Cloud; c != nil {
		switch {
		case c.Region != "":
			partes = append(partes, c.Region)
		case c.Zone != "":
			partes = append(partes, c.Zone)
		}
	}
	return strings.Join(partes, " · ")
}

// projetoAtual descreve o projeto pelo .yby/project.yaml do yby init ("yby-cli ·
// topologia standard"); sem ele, só o nome do diretório.
func projetoAtual(dir string) string {
	var manifesto struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Topology string `yaml:"topology"`
		} `yaml:"spec"`
	}
	if dados, err := os.ReadFile(filepath.Join(dir, ".yby", "project.yaml")); err == nil {
		_ = yaml.Unmarshal(dados, &manifesto)
	}
	nome := manifesto.Metadata.Name
	if nome == "" {
		nome = filepath.Base(dir)
	}
	if t := manifesto.Spec.Topology; t != "" {
		return nome + " · topologia " + t
	}
	return nome
}

// rotuloEmbeddings diz de onde vêm os embeddings da busca no Synapstor.
func rotuloEmbeddings(p ai.Provider) string {
	if p == nil {
		return "sem embeddings"
	}
	rotulo := rotuloProvider(p)
	if rotulo == "ollama" || strings.Contains(strings.ToLower(p.Name()), "local") {
		return "embeddings locais"
	}
	return "embeddings " + rotulo
}
