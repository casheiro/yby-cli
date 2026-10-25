package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/labels"
)

// ResourceTab representa uma aba de recurso, na ordem da prancha (1 a 9).
type ResourceTab int

const (
	TabPods ResourceTab = iota
	TabDeployments
	TabStatefulSets
	TabServices
	TabNodes
	TabJobs
	TabIngresses
	TabConfigMaps
	TabEvents
	tabCount
)

// grupo ordena a lista: problemas primeiro, depois os saudáveis e, por último,
// os concluídos (Completed/Succeeded), em tinta suave.
type grupo int

const (
	grupoFalha grupo = iota
	grupoAlerta
	grupoSaudavel
	grupoConcluido
)

// problema diz se o grupo conta como problema (resumo e linha de estado).
func (g grupo) problema() bool { return g == grupoFalha || g == grupoAlerta }

// itemLista é uma linha da lista de uma aba, já com as células da tabela.
type itemLista struct {
	nome, namespace string
	chave           string // identidade para manter a seleção (padrão: namespace/nome)
	status          string // texto do filtro de status; vazio se o recurso não tem status
	labels          map[string]string
	grupo           grupo
	celulas         []string // na ordem de defAba.colunas
	frase           string   // frase da linha de estado quando é problema (com glifo)
}

// defAba descreve uma aba: título, nomes para as frases, colunas e linhas.
type defAba struct {
	titulo           string // "Pods"
	singular, plural string // "pod", "pods"
	kind             string // tipo usado pelas ações e pelo YAML
	ordem            string // como a lista está ordenada (linha de filtros e filtrarEOrdenar)
	filtroStatus     bool   // o filtro S vale nesta aba (o STATUS dos pods)
	semNamespace     bool   // recurso do cluster (nós): sem coluna de namespace
	colunas          []yui.Coluna
	itens            func(m Model) []itemLista
}

const (
	ordemProblemas = "problemas primeiro"
	ordemNome      = "por nome"
	ordemRecentes  = "mais recentes primeiro"
)

// colunaNamespace aparece por último, só com todos os namespaces na tela; com
// Prioridade 4, sai depois das colunas de prioridade maior (IP e nó dos pods)
// e antes das outras.
var colunaNamespace = yui.Coluna{Titulo: "Namespace", Min: 9, Prioridade: 4}

var abas = [tabCount]defAba{
	TabPods: {
		titulo: "Pods", singular: "pod", plural: "pods", kind: "pod", ordem: ordemProblemas, filtroStatus: true,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Ok", Min: 3, Prioridade: 2}, {Titulo: "Status", Min: 9, Prioridade: 1},
			{Titulo: "Rein", Min: 4, Prioridade: 3}, {Titulo: "Idade", Min: 5, Prioridade: 2},
			{Titulo: "Nó", Min: 6, Max: 24, Prioridade: 5}, {Titulo: "IP", Min: 7, Prioridade: 6},
		},
		itens: itensPods,
	},
	TabDeployments: {
		titulo: "Deployments", singular: "deployment", plural: "deployments", kind: "deployment", ordem: ordemProblemas,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Prontos", Min: 7, Prioridade: 1}, {Titulo: "Atual.", Min: 6, Prioridade: 3},
			{Titulo: "Disponíveis", Min: 6, Prioridade: 2}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensDeployments,
	},
	TabStatefulSets: {
		titulo: "StatefulSets", singular: "statefulset", plural: "statefulsets", kind: "statefulset", ordem: ordemProblemas,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Prontos", Min: 7, Prioridade: 1}, {Titulo: "Atual.", Min: 6, Prioridade: 3},
			{Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensStatefulSets,
	},
	TabServices: {
		titulo: "Services", singular: "service", plural: "services", kind: "service", ordem: ordemNome,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Tipo", Min: 9, Prioridade: 1}, {Titulo: "Cluster-IP", Min: 10, Prioridade: 3},
			{Titulo: "Portas", Min: 8, Prioridade: 2}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensServices,
	},
	TabNodes: {
		titulo: "Nodes", singular: "nó", plural: "nós", kind: "node", ordem: ordemProblemas, semNamespace: true,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Status", Min: 9, Prioridade: 1}, {Titulo: "CPU", Min: 3, Prioridade: 2},
			{Titulo: "Memória", Min: 7, Prioridade: 2}, {Titulo: "Versão", Min: 7, Prioridade: 3}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensNodes,
	},
	TabJobs: {
		titulo: "Jobs", singular: "job", plural: "jobs", kind: "job", ordem: ordemProblemas,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Status", Min: 9, Prioridade: 1}, {Titulo: "Conclusões", Min: 5, Prioridade: 2},
			{Titulo: "Falhas", Min: 6, Prioridade: 3}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensJobs,
	},
	TabIngresses: {
		titulo: "Ingresses", singular: "ingress", plural: "ingresses", kind: "ingress", ordem: ordemNome,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Classe", Min: 6, Prioridade: 3}, {Titulo: "Hosts", Min: 10, Prioridade: 1},
			{Titulo: "Caminhos", Min: 8, Prioridade: 2}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensIngresses,
	},
	TabConfigMaps: {
		titulo: "ConfigMaps", singular: "configmap", plural: "configmaps", kind: "configmap", ordem: ordemNome,
		colunas: []yui.Coluna{
			{Titulo: "Nome", Min: 12}, {Titulo: "Chaves", Min: 6, Prioridade: 1}, {Titulo: "Tamanho", Min: 7, Prioridade: 2},
			{Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensConfigMaps,
	},
	TabEvents: {
		titulo: "Eventos", singular: "evento", plural: "eventos", kind: "event", ordem: ordemRecentes,
		colunas: []yui.Coluna{
			{Titulo: "Objeto", Min: 12, Max: 40}, {Titulo: "Tipo", Min: 9, Prioridade: 1}, {Titulo: "Razão", Min: 8, Prioridade: 2},
			{Titulo: "Mensagem", Min: 12, Prioridade: 3}, {Titulo: "Idade", Min: 5, Prioridade: 2},
		},
		itens: itensEventos,
	},
}

// String retorna o nome da aba
func (t ResourceTab) String() string {
	if t >= 0 && t < tabCount {
		return abas[t].titulo
	}
	return "?"
}

// contar escreve "1 pod" ou "8 pods".
func (d defAba) contar(n int) string {
	if n == 1 {
		return "1 " + d.singular
	}
	return strconv.Itoa(n) + " " + d.plural
}

// itensDaAba devolve as linhas da aba com os filtros da tela (label, status e
// busca por nome) e na ordem da tela: é a única fonte da lista visível, usada
// pelo desenho, pela contagem, pela seleção e pelas ações.
func (m Model) itensDaAba(tab ResourceTab) []itemLista {
	return m.filtrarEOrdenar(tab, abas[tab].itens(m))
}

// filtrarEOrdenar aplica label, status (só nas abas em que ele vale) e busca
// e ordena como a aba diz: problemas primeiro ou por nome (ordenação estável:
// no empate vale a ordem da API); os eventos ficam na ordem do tempo.
func (m Model) filtrarEOrdenar(tab ResourceTab, todos []itemLista) []itemLista {
	d := abas[tab]
	busca := strings.ToLower(m.filter.SearchQuery)
	var seletor labels.Selector
	if m.filter.LabelSelector != "" {
		if sel, err := labels.Parse(m.filter.LabelSelector); err == nil {
			seletor = sel
		}
	}
	saida := make([]itemLista, 0, len(todos))
	for _, it := range todos {
		if seletor != nil && !seletor.Matches(labels.Set(it.labels)) {
			continue
		}
		if d.filtroStatus && m.filter.StatusFilter != "" && !strings.EqualFold(it.status, m.filter.StatusFilter) {
			continue
		}
		if busca != "" && !strings.Contains(strings.ToLower(it.nome), busca) {
			continue
		}
		if it.chave == "" {
			it.chave = it.namespace + "/" + it.nome
		}
		saida = append(saida, it)
	}
	switch d.ordem {
	case ordemProblemas:
		sort.SliceStable(saida, func(a, b int) bool { return saida[a].grupo < saida[b].grupo })
	case ordemNome:
		sort.SliceStable(saida, func(a, b int) bool {
			if saida[a].nome != saida[b].nome {
				return saida[a].nome < saida[b].nome
			}
			return saida[a].namespace < saida[b].namespace
		})
	}
	return saida
}

// comNamespace acrescenta a célula de namespace (a última coluna).
func comNamespace(celulas []string, ns string) []string {
	return append(celulas, ns)
}

// frase monta a frase da linha de estado com o glifo do tipo (parado é o
// neutro, como uma ação cancelada), sempre numa linha só.
func frase(tipo yui.TipoStatus, texto string) string {
	s, g := yui.S(), yui.G()
	texto = umaLinha(texto)
	switch tipo {
	case yui.StatusFalha:
		return s.Erro.Render(g.Falha) + " " + s.Tinta.Render(texto)
	case yui.StatusAlerta:
		return s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render(texto)
	case yui.StatusParado:
		return s.TintaSuave.Render(g.Parado) + " " + s.Tinta.Render(texto)
	}
	return s.Ok.Render(g.Feito) + " " + s.Tinta.Render(texto)
}

// quebras são os caracteres que tirariam uma célula ou frase da sua linha.
var quebras = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// umaLinha troca quebras de linha e tabulações por espaço: a mensagem de um
// evento (a saída de uma probe exec, por exemplo) pode ter várias linhas.
func umaLinha(s string) string {
	return quebras.Replace(s)
}

// nomeDoTipo é o nome em PT-BR do tipo de recurso (o kind das ações).
func nomeDoTipo(kind string) string {
	for _, d := range abas {
		if d.kind == kind {
			return d.singular
		}
	}
	return kind
}

// reinicioRecente é a janela em que um reinício ainda conta como problema.
const reinicioRecente = time.Hour

// statusFalhaDePod são os pedaços de estado que indicam falha do pod.
var statusFalhaDePod = []string{"BackOff", "Err", "Error", "Fail", "OOM", "Evicted", "Invalid", "ExitCode"}

// classificarPod decide grupo, glifo e frase do pod: concluído; falha
// (CrashLoopBackOff, Error…); alerta (Pending, ContainerCreating, Running com
// contêiner fora do ar ou reinício na última hora); ou saudável.
func classificarPod(p monitor.Pod, agora time.Time) (grupo, yui.TipoStatus, string) {
	switch p.Status {
	case "Succeeded", "Completed":
		return grupoConcluido, yui.StatusConcluido, ""
	case "Running":
		switch {
		case p.Containers > 0 && p.Prontos < p.Containers:
			return grupoAlerta, yui.StatusAlerta, frase(yui.StatusAlerta,
				fmt.Sprintf("%s com %d/%d contêineres prontos", p.Name, p.Prontos, p.Containers))
		case p.Reinicios > 0 && !p.UltimoReinicio.IsZero() && agora.Sub(p.UltimoReinicio) < reinicioRecente:
			return grupoAlerta, yui.StatusRodando, frase(yui.StatusAlerta,
				fmt.Sprintf("%s reiniciou %s, a última há %s", p.Name, vezes(p.Reinicios), monitor.Idade(p.UltimoReinicio, agora)))
		}
		return grupoSaudavel, yui.StatusRodando, ""
	}
	for _, f := range statusFalhaDePod {
		if strings.Contains(p.Status, f) {
			texto := p.Name + " em " + p.Status
			if p.Reinicios > 0 {
				texto += ", reiniciou " + vezes(p.Reinicios)
			}
			return grupoFalha, yui.StatusFalha, frase(yui.StatusFalha, texto)
		}
	}
	texto := p.Name + " em " + p.Status
	if idade := monitor.Idade(p.CriadoEm, agora); idade != "" {
		texto += " há " + idade
	}
	return grupoAlerta, yui.StatusAlerta, frase(yui.StatusAlerta, texto)
}

// vezes escreve "1 vez" ou "14 vezes".
func vezes(n int32) string {
	if n == 1 {
		return "1 vez"
	}
	return fmt.Sprintf("%d vezes", n)
}

func itensPods(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.pods))
	for _, p := range m.pods {
		g, tipo, fr := classificarPod(p, agora)
		ok := fmt.Sprintf("%d/%d", p.Prontos, p.Containers)
		itens = append(itens, itemLista{
			nome: p.Name, namespace: p.Namespace, status: p.Status, labels: p.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{p.Name, ok, yui.Status(tipo, p.Status), strconv.Itoa(int(p.Reinicios)),
				monitor.Idade(p.CriadoEm, agora), p.No, p.IP}, p.Namespace),
		})
	}
	return itens
}

// classificarReplicas: nenhuma réplica pronta é falha, parte delas é alerta.
func classificarReplicas(rotulo, nome string, prontas, desejadas int32) (grupo, string) {
	switch {
	case desejadas > 0 && prontas == 0:
		return grupoFalha, frase(yui.StatusFalha, fmt.Sprintf("%s %s sem réplicas prontas (0/%d)", rotulo, nome, desejadas))
	case prontas < desejadas:
		return grupoAlerta, frase(yui.StatusAlerta, fmt.Sprintf("%s %s com %d/%d réplicas prontas", rotulo, nome, prontas, desejadas))
	}
	return grupoSaudavel, ""
}

func itensDeployments(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.deployments))
	for _, d := range m.deployments {
		g, fr := classificarReplicas("Deployment", d.Name, d.Ready, d.Replicas)
		itens = append(itens, itemLista{
			nome: d.Name, namespace: d.Namespace, labels: d.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{d.Name, fmt.Sprintf("%d/%d", d.Ready, d.Replicas), strconv.Itoa(int(d.Atualizados)),
				strconv.Itoa(int(d.Available)), monitor.Idade(d.CriadoEm, agora)}, d.Namespace),
		})
	}
	return itens
}

func itensStatefulSets(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.statefulsets))
	for _, s := range m.statefulsets {
		g, fr := classificarReplicas("StatefulSet", s.Name, s.Ready, s.Replicas)
		itens = append(itens, itemLista{
			nome: s.Name, namespace: s.Namespace, labels: s.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{s.Name, fmt.Sprintf("%d/%d", s.Ready, s.Replicas), strconv.Itoa(int(s.Atualizados)),
				monitor.Idade(s.CriadoEm, agora)}, s.Namespace),
		})
	}
	return itens
}

func itensServices(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.services))
	for _, s := range m.services {
		itens = append(itens, itemLista{
			nome: s.Name, namespace: s.Namespace, labels: s.Labels, grupo: grupoSaudavel,
			celulas: comNamespace([]string{s.Name, s.Type, s.ClusterIP, s.Ports, monitor.Idade(s.CriadoEm, agora)}, s.Namespace),
		})
	}
	return itens
}

func itensNodes(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.nodes))
	for _, n := range m.nodes {
		g, tipo, fr := grupoSaudavel, yui.StatusRodando, ""
		if n.Status != "Ready" {
			g, tipo, fr = grupoFalha, yui.StatusFalha, frase(yui.StatusFalha, "nó "+n.Name+" em "+n.Status)
		}
		itens = append(itens, itemLista{
			nome: n.Name, status: n.Status, labels: n.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{n.Name, yui.Status(tipo, n.Status), n.CPUCapacity, n.MemoryCapacity, n.Version,
				monitor.Idade(n.CriadoEm, agora)}, ""),
		})
	}
	return itens
}

// classificarJob: pelas condições do Job (Complete, Failed) quando existem;
// sem elas, concluído quando as conclusões saíram (ou, na fila de trabalho sem
// spec.completions, quando um pod terminou bem e nenhum roda), rodando com pods
// ativos (alerta se já houve falha e ele tenta de novo), falha com falhas e
// nada rodando, aguardando no resto.
func classificarJob(j monitor.Job) (grupo, yui.TipoStatus, string, string) {
	concluido := j.Concluido || (j.Completions > 0 && j.Succeeded >= j.Completions) ||
		(j.Completions == 0 && j.Succeeded > 0 && j.Active == 0 && j.Failed == 0)
	switch {
	case j.Falhou:
		return grupoFalha, yui.StatusFalha, "Falhou", frase(yui.StatusFalha, fmt.Sprintf("Job %s falhou %s", j.Name, vezes(j.Failed)))
	case concluido:
		return grupoConcluido, yui.StatusConcluido, "Concluído", ""
	case j.Active > 0 && j.Failed > 0:
		return grupoAlerta, yui.StatusAlerta, "Rodando", frase(yui.StatusAlerta,
			fmt.Sprintf("Job %s falhou %s e tenta de novo", j.Name, vezes(j.Failed)))
	case j.Active > 0:
		return grupoSaudavel, yui.StatusRodando, "Rodando", ""
	case j.Failed > 0:
		return grupoFalha, yui.StatusFalha, "Falhou", frase(yui.StatusFalha, fmt.Sprintf("Job %s falhou %s", j.Name, vezes(j.Failed)))
	}
	return grupoAlerta, yui.StatusAlerta, "Aguardando", frase(yui.StatusAlerta, "Job "+j.Name+" aguardando pods")
}

// conclusoes é a coluna CONCLUSÕES: "1/3", ou só as conclusões no Job de fila
// de trabalho, que não tem um total.
func conclusoes(j monitor.Job) string {
	if j.Completions == 0 {
		return strconv.Itoa(int(j.Succeeded))
	}
	return fmt.Sprintf("%d/%d", j.Succeeded, j.Completions)
}

func itensJobs(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.jobs))
	for _, j := range m.jobs {
		g, tipo, st, fr := classificarJob(j)
		itens = append(itens, itemLista{
			nome: j.Name, namespace: j.Namespace, status: st, labels: j.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{j.Name, yui.Status(tipo, st), conclusoes(j),
				strconv.Itoa(int(j.Failed)), monitor.Idade(j.CriadoEm, agora)}, j.Namespace),
		})
	}
	return itens
}

func itensIngresses(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.ingresses))
	for _, i := range m.ingresses {
		itens = append(itens, itemLista{
			nome: i.Name, namespace: i.Namespace, labels: i.Labels, grupo: grupoSaudavel,
			celulas: comNamespace([]string{i.Name, i.Class, i.Hosts, i.Paths, monitor.Idade(i.CriadoEm, agora)}, i.Namespace),
		})
	}
	return itens
}

func itensConfigMaps(m Model) []itemLista {
	agora := m.agora()
	itens := make([]itemLista, 0, len(m.configmaps))
	for _, c := range m.configmaps {
		itens = append(itens, itemLista{
			nome: c.Name, namespace: c.Namespace, labels: c.Labels, grupo: grupoSaudavel,
			celulas: comNamespace([]string{c.Name, strconv.Itoa(c.Keys), c.DataSize, monitor.Idade(c.CriadoEm, agora)}, c.Namespace),
		})
	}
	return itens
}

// itensEventos ficam na ordem do tempo (mais recentes primeiro); um Warning
// conta como problema no resumo e na linha de estado.
func itensEventos(m Model) []itemLista {
	itens := make([]itemLista, 0, len(m.events))
	for _, e := range m.events {
		g, tipo, fr := grupoSaudavel, yui.StatusRodando, ""
		if e.Type == "Warning" {
			g, tipo = grupoAlerta, yui.StatusAlerta
			fr = frase(yui.StatusAlerta, e.Reason+" em "+e.Name+": "+e.Message)
		}
		chave := "" // sem ID, a seleção fica na linha (vários eventos do mesmo objeto)
		if e.ID != "" {
			chave = e.Namespace + "/evento/" + e.ID
		}
		itens = append(itens, itemLista{
			nome: e.Name, namespace: e.Namespace, chave: chave, status: e.Type, labels: e.Labels, grupo: g, frase: fr,
			celulas: comNamespace([]string{e.Name, yui.Status(tipo, e.Type), e.Reason, e.Message, e.Age}, e.Namespace),
		})
	}
	return itens
}

// tabelaDaAba devolve colunas e linhas da aba para yui.Tabela; a coluna de
// namespace só entra com todos os namespaces na tela.
func (m Model) tabelaDaAba(tab ResourceTab, itens []itemLista) ([]yui.Coluna, []yui.Linha) {
	d := abas[tab]
	colunas := append([]yui.Coluna(nil), d.colunas...)
	todosNS := m.filter.Namespace == "" && !d.semNamespace
	if todosNS {
		colunas = append(colunas, colunaNamespace)
	}
	linhas := make([]yui.Linha, len(itens))
	for i, it := range itens {
		celulas := make([]string, 0, len(it.celulas))
		for _, c := range it.celulas {
			celulas = append(celulas, umaLinha(c)) // uma linha por item: a janela conta assim
		}
		if !todosNS {
			celulas = celulas[:len(d.colunas)]
		}
		linhas[i] = yui.Linha{Celulas: celulas, Concluida: it.grupo == grupoConcluido}
	}
	return caberNaLargura(colunas, linhas, m.width)
}

// espacoEntreColunas é o espaço que yui.Tabela põe entre as colunas.
const espacoEntreColunas = 2

// caberNaLargura tira as colunas menos importantes (maior Prioridade; no
// empate, a mais à direita) enquanto a tabela, na largura ideal, passa da
// janela: em tela estreita somem o IP e o nó antes de o nome ser cortado. A
// yui.Tabela só tira uma coluna quando nem o mínimo dela cabe.
func caberNaLargura(colunas []yui.Coluna, linhas []yui.Linha, largura int) ([]yui.Coluna, []yui.Linha) {
	for len(colunas) > 1 && larguraIdeal(colunas, linhas) > largura {
		pior := 0
		for i, c := range colunas {
			if c.Prioridade >= colunas[pior].Prioridade {
				pior = i
			}
		}
		if colunas[pior].Prioridade == 0 {
			break
		}
		colunas = semIndice(colunas, pior)
		novas := make([]yui.Linha, len(linhas))
		for i, l := range linhas {
			novas[i] = yui.Linha{Celulas: semIndice(l.Celulas, pior), Concluida: l.Concluida}
		}
		linhas = novas
	}
	return colunas, linhas
}

// semIndice devolve uma cópia de s sem o elemento i.
func semIndice[T any](s []T, i int) []T {
	if i >= len(s) {
		return append([]T(nil), s...)
	}
	return append(append([]T(nil), s[:i]...), s[i+1:]...)
}

// larguraIdeal é a largura da tabela sem cortar nada: o maior conteúdo de
// cada coluna (ou o título), dentro de Min e Max, mais o espaço entre elas.
func larguraIdeal(colunas []yui.Coluna, linhas []yui.Linha) int {
	total := espacoEntreColunas * (len(colunas) - 1)
	for i, c := range colunas {
		w := max(lipgloss.Width(c.Titulo), c.Min)
		for _, l := range linhas {
			if i < len(l.Celulas) {
				w = max(w, lipgloss.Width(l.Celulas[i]))
			}
		}
		if c.Max > 0 {
			w = min(w, max(c.Max, c.Min))
		}
		total += w
	}
	return total
}
