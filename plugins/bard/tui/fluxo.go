package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/redacao"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/cenkalti/backoff/v4"
	tea "github.com/charmbracelet/bubbletea"
)

// Etapa identifica uma fase do trabalho do Bard ao responder uma pergunta.
type Etapa int

const (
	// EtapaContexto busca notas do Synapstor relacionadas à pergunta.
	EtapaContexto Etapa = iota
	// EtapaIntencao classifica o pedido e decide se usa uma ferramenta.
	EtapaIntencao
	// EtapaFerramenta executa a ferramenta escolhida.
	EtapaFerramenta
	// EtapaResposta gera a resposta pelo provider.
	EtapaResposta
)

// Rótulos fixos das etapas; a de ferramenta usa o nome da ferramenta.
const (
	rotuloContexto = "contexto do projeto"
	rotuloIntencao = "entender o pedido"
	rotuloResposta = "gerar resposta"
)

// EtapaMsg informa o início ou o fim de uma etapa. Cada etapa chega primeiro
// com Estado ui.EtapaAtual e depois com o estado final (feita ou falha).
type EtapaMsg struct {
	Rodada  int
	Etapa   Etapa
	Rotulo  string
	Estado  ui.EstadoEtapa
	Nota    string        // resultado curto (ex.: "3 notas do Synapstor")
	Duracao time.Duration // preenchida no estado final
	Saida   string        // saída completa da ferramenta, para "↵ ver"
	// Aviso é o texto inteiro do que pede ação do usuário (ex.: o índice do Synapstor feito
	// com outros embeddings). A nota é curta e a linha da etapa a corta; o aviso fica
	// depois da rodada, na conversa da TUI e abaixo das etapas no -p.
	Aviso string
	// Tentativa, na etapa de resposta, conta as chamadas ao provider. Acima
	// de 1 indica que a resposta recomeçou: os trechos anteriores valem nada.
	// A falha de uma tentativa já anuncia a seguinte (NotaNovaTentativa)
	// antes da espera entre as duas; a seguinte pode nem começar, se as
	// novas tentativas acabaram.
	Tentativa int
}

// NotaNovaTentativa é a nota da etapa de resposta logo depois de uma falha do
// provider, enquanto a próxima tentativa espera.
const NotaNovaTentativa = "tentando de novo"

// TrechoMsg traz um pedaço da resposta enquanto ela é gerada.
type TrechoMsg struct {
	Rodada int
	Texto  string
}

// FimMsg é sempre o último evento de uma rodada; depois dele o canal fecha.
type FimMsg struct {
	Rodada       int
	Resposta     string        // resposta completa, ou o trecho já recebido se interrompida
	Ferramentas  []string      // ferramentas executadas na rodada
	Duracao      time.Duration // tempo total da rodada
	Err          error         // falha do provider depois das novas tentativas
	Interrompida bool          // o contexto foi cancelado (esc) antes do fim
}

// Buscador é a parte do VectorStore que o fluxo usa para buscar contexto.
type Buscador interface {
	Search(ctx context.Context, query string, topK int) ([]ai.UnknownDocument, error)
}

// retryPadrao é a política de novas tentativas da resposta final.
var retryPadrao = retry.Options{
	InitialInterval:     2 * time.Second,
	MaxInterval:         10 * time.Second,
	MaxElapsedTime:      30 * time.Second,
	RandomizationFactor: 0.3,
	Multiplier:          2.0,
}

// Fluxo transforma uma pergunta em eventos: contexto, intenção, ferramenta e
// resposta, com os pedaços da resposta à medida que o provider os escreve.
type Fluxo struct {
	Provider     ai.Provider
	Buscador     Buscador // opcional; nil desliga a busca no Synapstor
	SystemPrompt string
	TopK         int
	Limiar       float64 // score mínimo de uma nota para entrar no contexto
	Namespace    string  // namespace da sessão; fica fora da nota da ferramenta
	Cluster      string  // contexto do cluster da sessão; vai ao classificador
	// BuscarFerramenta resolve a ferramenta de uma intenção; nil usa o registry.
	BuscarFerramenta func(intent string) *tools.Tool
	Retry            retry.Options
}

// NovoFluxo monta o fluxo a partir da configuração da TUI; o -p e o batch usam o mesmo.
func NovoFluxo(provider ai.Provider, vectorStore *ai.VectorStore, config Config) *Fluxo {
	f := &Fluxo{
		Provider:     provider,
		SystemPrompt: config.SystemPrompt,
		TopK:         config.TopK,
		Limiar:       config.LimiarRelevancia,
		Namespace:    config.Namespace,
		Cluster:      config.Cluster,
		Retry:        retryPadrao,
	}
	// Um *VectorStore nil dentro da interface não seria nil; só atribui se existir.
	if vectorStore != nil {
		f.Buscador = vectorStore
	}
	if f.TopK <= 0 {
		f.TopK = 5
	}
	// O padrão (o corte do pkg/ai, o mesmo do Synapstor) já vem do loadBardConfig; aqui
	// só se corrige um valor inválido, porque 0 explícito no bard.yaml aceita todas as notas.
	if f.Limiar < 0 {
		f.Limiar = ai.RelevanciaMinima
	}
	return f
}

// Iniciar roda o fluxo em segundo plano e devolve o canal de eventos. O canal
// termina sempre com um FimMsg e é fechado em seguida; quem consome deve ler
// até o FimMsg. Cancelar ctx interrompe a rodada e antecipa o FimMsg, mesmo
// que o provider ou a ferramenta não respeitem o contexto. A conversa são as trocas
// anteriores da sessão, que vão com a pergunta à resposta e ao classificador.
func (f *Fluxo) Iniciar(ctx context.Context, rodada int, entrada string, conversa ...Troca) <-chan tea.Msg {
	e := &emissor{ctx: ctx, ch: make(chan tea.Msg, 16), rodada: rodada}
	// A memória sai aqui, antes da goroutine: quem chamou pode acrescentar trocas à fatia.
	go f.rodar(ctx, e, entrada, memoriaDaConversa(conversa))
	return e.ch
}

func (f *Fluxo) rodar(ctx context.Context, e *emissor, entrada, conversa string) {
	inicio := time.Now()
	var usadas []string
	fim := func(err error) {
		resposta := e.texto()
		interrompida := ctx.Err() != nil
		if interrompida {
			err = nil
		}
		e.fim(FimMsg{
			Rodada:       e.rodada,
			Resposta:     resposta,
			Ferramentas:  usadas,
			Duracao:      time.Since(inicio),
			Err:          err,
			Interrompida: interrompida,
		})
	}
	// O pipeline roda fora dos comandos do bubbletea, que recuperariam um
	// panic e restaurariam o terminal. Um panic do provider, da ferramenta ou
	// da busca (esperar o repassa para cá) vira erro da rodada, e o Bard segue
	// aberto em vez de derrubar o processo com o terminal em modo raw.
	defer func() {
		if r := recover(); r != nil {
			e.falharAberta("falha interna")
			fim(fmt.Errorf("falha interna do Bard: %v", r))
		}
	}()

	// 1. Contexto do projeto (Synapstor)
	t0 := time.Now()
	e.etapa(EtapaMsg{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: ui.EtapaAtual})
	notas, ok := esperar(ctx, func() contextoBuscado { return f.buscarContexto(ctx, entrada) })
	if !ok {
		e.etapaForcada(interrompida(EtapaContexto, rotuloContexto, time.Since(t0)))
		fim(nil)
		return
	}
	// O índice que precisa ser refeito fica marcado como falha; a pergunta segue sem notas.
	estado := ui.EtapaFeita
	if notas.falhou {
		estado = ui.EtapaFalha
	}
	e.etapa(EtapaMsg{Etapa: EtapaContexto, Rotulo: rotuloContexto, Estado: estado, Nota: notas.nota, Aviso: notas.aviso, Duracao: time.Since(t0)})

	entradaFinal := entrada
	if notas.texto != "" || conversa != "" {
		entradaFinal = "Pergunta do Usuário: " + entrada
	}

	// 2. Intenção
	t0 = time.Now()
	e.etapa(EtapaMsg{Etapa: EtapaIntencao, Rotulo: rotuloIntencao, Estado: ui.EtapaAtual})
	intent, ok := esperar(ctx, func() *tools.IntentResult {
		return classifyIntent(ctx, f.Provider, entrada, conversa, f.Cluster, f.Namespace)
	})
	if !ok || ctx.Err() != nil {
		e.etapaForcada(interrompida(EtapaIntencao, rotuloIntencao, time.Since(t0)))
		fim(nil)
		return
	}
	var ferramenta *tools.Tool
	if intent != nil && !intent.Direct && intent.Intent != "direct" {
		ferramenta = f.ferramenta(intent.Intent)
	}
	notaIntencao := "responder direto"
	if ferramenta != nil {
		notaIntencao = pedidoEntendido(intent.Intent, ferramenta)
	}
	e.etapa(EtapaMsg{Etapa: EtapaIntencao, Rotulo: rotuloIntencao, Estado: ui.EtapaFeita, Nota: notaIntencao, Duracao: time.Since(t0)})

	// 3. Ferramenta
	if ferramenta != nil {
		rotulo := nomeFerramenta(ferramenta)
		alvo := alvoFerramenta(ferramenta, intent.Params, f.Namespace)
		t0 = time.Now()
		e.etapa(EtapaMsg{Etapa: EtapaFerramenta, Rotulo: rotulo, Estado: ui.EtapaAtual, Nota: alvo})
		res, ok := esperar(ctx, func() resultadoFerramenta {
			if faltam := ferramenta.Faltando(intent.Params); len(faltam) > 0 {
				return resultadoFerramenta{err: fmt.Errorf("faltou o parâmetro obrigatório %s para %s: peça ao usuário que informe", strings.Join(faltam, ", "), ferramenta.Name)}
			}
			saida, err := ferramenta.Execute(ctx, intent.Params)
			return resultadoFerramenta{saida: saida, err: err}
		})
		if !ok || ctx.Err() != nil {
			e.etapaForcada(interrompida(EtapaFerramenta, rotulo, time.Since(t0)))
			fim(nil)
			return
		}
		usadas = append(usadas, ferramenta.Name)
		// A saída de toda ferramenta (logs, describe, achados do Sentinel, tools
		// externas) e o erro dela, que costuma trazer a saída junto, passam pela
		// redação e depois pelo teto (a redação vê o texto inteiro) antes do prompt.
		// O "↵ ver" mostra o mesmo texto que a IA recebeu.
		saida := Cortar(redacao.Redigir(res.saida), TetoFerramenta)
		msg := EtapaMsg{Etapa: EtapaFerramenta, Rotulo: rotulo, Estado: ui.EtapaFeita, Nota: alvo, Duracao: time.Since(t0), Saida: saida}
		if res.err != nil {
			saida = Cortar(redacao.Redigir(fmt.Sprintf("Erro: %v", res.err)), TetoFerramenta)
			msg.Estado = ui.EtapaFalha
			msg.Nota = primeiraLinha(redacao.Redigir(res.err.Error()))
			msg.Saida = saida
		}
		e.etapa(msg)
		entradaFinal = fmt.Sprintf("Pergunta do usuario: %s\n\nResultado da ferramenta %s:\n%s\n\nAnalise o resultado e responda ao usuario de forma clara e acionavel.", entrada, ferramenta.Name, saida)
	}
	// A conversa atual vem antes da pergunta, e as notas do Synapstor, que continuam
	// valendo quando uma ferramenta roda, antes de tudo.
	entradaFinal = comConversa(conversa, entradaFinal)
	if notas.texto != "" {
		entradaFinal = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\n%s", notas.texto, entradaFinal)
	}

	// 4. Resposta, em pedaços, com novas tentativas
	t0 = time.Now()
	tentativa := 0
	err, ok := esperar(ctx, func() error {
		return retry.Do(ctx, f.Retry, func() error {
			tentativa++
			if ctx.Err() != nil {
				return backoff.Permanent(ctx.Err())
			}
			e.recomecar()
			msg := EtapaMsg{Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual, Tentativa: tentativa}
			if tentativa > 1 {
				msg.Nota = fmt.Sprintf("tentativa %d", tentativa)
			}
			e.etapa(msg)
			errStream := f.Provider.StreamCompletion(ctx, f.SystemPrompt, entradaFinal, e)
			if errStream != nil && ctx.Err() != nil {
				return backoff.Permanent(ctx.Err())
			}
			if errStream != nil {
				// O texto da tentativa falha some já, e não só quando a
				// próxima começa: um esc durante a espera não o leva ao fim.
				e.recomecar()
				// Chave inválida, modelo inexistente ou prompt grande demais
				// falham igual na próxima vez: só tenta de novo o que o pkg/ai
				// classifica como passageiro.
				if !ai.ErroRetentavel(errStream) {
					return backoff.Permanent(errStream)
				}
				e.etapa(EtapaMsg{Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaAtual, Tentativa: tentativa + 1, Nota: NotaNovaTentativa})
			}
			return errStream
		})
	})
	if !ok || ctx.Err() != nil {
		e.etapaForcada(interrompida(EtapaResposta, rotuloResposta, time.Since(t0)))
		fim(nil)
		return
	}
	if err != nil {
		e.etapa(EtapaMsg{Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaFalha, Nota: primeiraLinha(err.Error()), Duracao: time.Since(t0)})
		fim(err)
		return
	}
	e.etapa(EtapaMsg{Etapa: EtapaResposta, Rotulo: rotuloResposta, Estado: ui.EtapaFeita, Duracao: time.Since(t0)})
	fim(nil)
}

// contextoBuscado é o resultado da etapa de contexto.
type contextoBuscado struct {
	texto  string // trecho injetado na pergunta; vazio sem notas
	nota   string // resultado curto exibido na etapa
	aviso  string // o que o usuário precisa fazer, inteiro (ver EtapaMsg.Aviso)
	falhou bool   // a busca falhou de um jeito que o usuário resolve (ex.: índice de outro modelo)
}

// notaBuscada são os trechos de um arquivo do índice que passaram do corte, na ordem da
// busca.
type notaBuscada struct {
	titulo, caminho string
	trechos         []ai.UnknownDocument
}

// notasPorArquivo junta os trechos acima do limiar por arquivo, na ordem do trecho de
// maior pontuação de cada um: o índice guarda trechos de seção, e o mesmo arquivo não
// deve aparecer várias vezes. Trecho sem source nem ID fica sozinho. O corte compara em
// float32, a precisão da pontuação, como o Synapstor: em float64, o trecho com pontuação
// float32(ai.RelevanciaMinima) ficaria de fora no Bard e dentro no Synapstor.
func notasPorArquivo(resultados []ai.UnknownDocument, limiar float64) []*notaBuscada {
	var notas []*notaBuscada
	porCaminho := map[string]*notaBuscada{}
	corte := float32(limiar)
	for _, res := range resultados {
		if res.Score < corte {
			continue
		}
		caminho := res.Metadata["source"]
		if caminho == "" {
			caminho, _, _ = strings.Cut(res.ID, "#")
		}
		n := porCaminho[caminho]
		if n == nil || caminho == "" {
			n = &notaBuscada{titulo: res.Metadata["title"], caminho: caminho}
			notas = append(notas, n)
			if caminho != "" {
				porCaminho[caminho] = n
			}
		}
		n.trechos = append(n.trechos, res)
	}
	return notas
}

// cabecalho apresenta a nota pelo caminho no índice, que identifica o arquivo. Os trechos
// do Synapstor já começam com "# Título" (e "## Seção"): o título só entra no cabeçalho
// quando o primeiro trecho não o traz, ou quando não há caminho.
func (n *notaBuscada) cabecalho() string {
	nome := n.caminho
	trazTitulo := len(n.trechos) > 0 && strings.HasPrefix(n.trechos[0].Content, "# ")
	switch {
	case n.titulo == "" || (trazTitulo && nome != ""):
	case nome == "":
		nome = n.titulo
	default:
		nome = n.titulo + " (" + nome + ")"
	}
	return fmt.Sprintf("\n--- Contexto: %s ---\n", nome)
}

// marcaDeSecao é o "[seção: X]" do trecho que não começa com o "## X" da seção (os do
// Synapstor começam, logo depois do "# Título"); vazio sem seção ou quando o trecho já a traz.
func marcaDeSecao(res ai.UnknownDocument) string {
	secao := res.Metadata["section"]
	if secao == "" {
		return ""
	}
	linhas := strings.SplitN(res.Content, "\n", 3)
	for i, linha := range linhas[:min(len(linhas), 2)] {
		if linha == "## "+secao {
			return ""
		}
		if i == 0 && !strings.HasPrefix(linha, "# ") {
			break
		}
	}
	return "[seção: " + secao + "]\n"
}

// NotaIndiceDeOutroModelo é a nota da etapa de contexto quando o índice do Synapstor foi
// feito com outros embeddings. É curta para caber na linha da etapa ao lado do tempo,
// num terminal de 80 colunas; o comando que o refaz vai inteiro no aviso.
const NotaIndiceDeOutroModelo = "índice de outro modelo"

// linhaReindexar é a última linha do aviso do índice de outro modelo: numa linha só, o
// comando não se parte na quebra do texto e se copia inteiro.
const linhaReindexar = "rode yby synapstor index --full para refazer o índice"

// avisoDoIndice é o aviso da etapa de contexto quando o índice foi feito com outros
// embeddings: com o que ele foi feito, de onde vêm os embeddings agora e, na linha de
// baixo, o comando que o refaz. O Bard não reindexa.
func avisoDoIndice(err error) (string, bool) {
	var outro *ai.IndiceDeOutroModeloError
	switch {
	case errors.As(err, &outro):
		return outro.Diferenca() + ".\n" + linhaReindexar, true
	case errors.Is(err, ai.ErrIndiceDeOutroModelo):
		return ai.ErrIndiceDeOutroModelo.Error() + ".\n" + linhaReindexar, true
	}
	return "", false
}

func (f *Fluxo) buscarContexto(ctx context.Context, entrada string) contextoBuscado {
	if f.Buscador == nil {
		return contextoBuscado{nota: "sem Synapstor"}
	}
	resultados, err := f.Buscador.Search(ctx, entrada, f.TopK)
	if aviso, ok := avisoDoIndice(err); ok {
		return contextoBuscado{nota: NotaIndiceDeOutroModelo, aviso: aviso, falhou: true}
	}
	if err != nil {
		return contextoBuscado{nota: "Synapstor indisponível"}
	}
	// As notas vão na ordem da busca até o teto, cada uma com o caminho e os trechos com
	// o título e a seção que trazem: o trecho que não cabe inteiro entra cortado, se ainda
	// houver espaço para um pedaço útil, e os seguintes ficam de fora.
	var sb strings.Builder
	n := 0
notas:
	for _, nota := range notasPorArquivo(resultados, f.Limiar) {
		cabecalho := nota.cabecalho()
		for _, res := range nota.trechos {
			secao := marcaDeSecao(res)
			resta := TetoNotas - sb.Len() - len(cabecalho) - len(secao) - 1
			conteudo := res.Content
			if len(conteudo) > resta {
				if resta < minimoNota {
					break notas
				}
				conteudo = Cortar(conteudo, resta)
			}
			if cabecalho != "" {
				n++
			}
			sb.WriteString(cabecalho + secao + conteudo + "\n")
			cabecalho = ""
		}
	}
	switch n {
	case 0:
		return contextoBuscado{nota: "nenhuma nota relevante"}
	case 1:
		return contextoBuscado{texto: sb.String(), nota: "1 nota do Synapstor"}
	default:
		return contextoBuscado{texto: sb.String(), nota: fmt.Sprintf("%d notas do Synapstor", n)}
	}
}

func (f *Fluxo) ferramenta(intent string) *tools.Tool {
	if f.BuscarFerramenta != nil {
		return f.BuscarFerramenta(intent)
	}
	return tools.FindByIntent(intent)
}

// resultadoFerramenta é o retorno de Execute levado pelo canal de esperar.
type resultadoFerramenta struct {
	saida string
	err   error
}

// nomeFerramenta troca os sublinhados do nome por espaços ("sentinel investigate").
func nomeFerramenta(t *tools.Tool) string {
	return strings.ReplaceAll(t.Name, "_", " ")
}

// pedidos descreve, como a prancha "trabalhando", o que o Bard entendeu de
// cada intenção das ferramentas nativas.
var pedidos = map[string]string{
	"investigate_pod":      "investigar pod com falha",
	"diagnose_pod":         "investigar pod com falha",
	"pod_health":           "verificar a saúde do pod",
	"list_resources":       "listar recursos",
	"get_resources":        "listar recursos",
	"list_pods":            "listar pods",
	"list_services":        "listar serviços",
	"pod_logs":             "ver logs do pod",
	"view_logs":            "ver logs do pod",
	"check_logs":           "ver logs do pod",
	"describe_resource":    "detalhar recurso",
	"resource_details":     "detalhar recurso",
	"pod_events":           "ver eventos do pod",
	"cluster_events":       "ver eventos do cluster",
	"check_events":         "ver eventos do cluster",
	"security_scan":        "auditar a segurança do cluster",
	"scan_vulnerabilities": "auditar a segurança do cluster",
	"check_security":       "auditar a segurança do cluster",
	"infra_topology":       "mapear a topologia do projeto",
	"project_architecture": "mapear a topologia do projeto",
	"blueprint":            "mapear a topologia do projeto",
}

// pedidoEntendido é a nota da etapa de intenção: o pedido em poucas palavras,
// em minúsculas e sem ponto. Ferramentas externas (YAML) caem na descrição.
func pedidoEntendido(intent string, t *tools.Tool) string {
	if p, ok := pedidos[intent]; ok {
		return p
	}
	d := strings.TrimSuffix(strings.TrimSpace(primeiraLinha(t.Description)), ".")
	if d == "" {
		return strings.ReplaceAll(intent, "_", " ")
	}
	r, n := utf8.DecodeRuneInString(d)
	return string(unicode.ToLower(r)) + d[n:]
}

// alvoFerramenta é a nota da etapa da ferramenta: os parâmetros obrigatórios
// em forma de comando ("pod worker-1"), na ordem declarada. Sem eles, entram
// os opcionais, menos o namespace da sessão, que o cabeçalho já mostra. O
// " · " fica livre para separar o alvo das ações ("· ↵ ver").
func alvoFerramenta(t *tools.Tool, params map[string]string, namespace string) string {
	var obrigatorios, opcionais []string
	for _, p := range t.Parameters {
		v := strings.TrimSpace(params[p.Name])
		switch {
		case v == "":
		case p.Required:
			obrigatorios = append(obrigatorios, v)
		case p.Name == "namespace" && v == namespace:
		default:
			opcionais = append(opcionais, v)
		}
	}
	if len(obrigatorios) > 0 {
		return strings.Join(obrigatorios, " ")
	}
	return strings.Join(opcionais, " ")
}

// primeiraLinha corta uma mensagem de erro na primeira quebra de linha.
func primeiraLinha(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// interrompida monta o estado final de uma etapa cortada pelo cancelamento.
func interrompida(etapa Etapa, rotulo string, d time.Duration) EtapaMsg {
	return EtapaMsg{Etapa: etapa, Rotulo: rotulo, Estado: ui.EtapaFalha, Nota: "interrompida", Duracao: d}
}

// esperar roda f em segundo plano e devolve o resultado, ou ok=false se ctx
// for cancelado antes. Assim o esc responde na hora mesmo com um provider ou
// uma ferramenta que ignore o contexto. Um panic de f é repassado a quem
// chamou, para o recover de rodar; depois do cancelamento, é descartado.
func esperar[T any](ctx context.Context, f func() T) (T, bool) {
	ch := make(chan T, 1)
	panico := make(chan any, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panico <- r
			}
		}()
		ch <- f()
	}()
	select {
	case v := <-ch:
		return v, true
	case r := <-panico:
		panic(r)
	case <-ctx.Done():
		var zero T
		return zero, false
	}
}

// emissor envia os eventos de uma rodada e acumula o texto da resposta. Ele
// também é o io.Writer passado ao StreamCompletion: cada Write vira um TrechoMsg.
type emissor struct {
	ctx     context.Context
	ch      chan tea.Msg
	rodada  int
	mu      sync.Mutex
	fechado bool
	buf     strings.Builder
	// aberta é a última etapa iniciada e ainda sem estado final, e desde o
	// momento em que começou; servem para fechá-la num panic.
	aberta *EtapaMsg
	desde  time.Time
}

// enviar entrega msg, desistindo se a rodada já terminou ou foi cancelada.
// Chamado com mu travado.
func (e *emissor) enviar(msg tea.Msg) bool {
	if e.fechado {
		return false
	}
	select {
	case e.ch <- msg:
		return true
	case <-e.ctx.Done():
		return false
	}
}

func (e *emissor) etapa(msg EtapaMsg) {
	msg.Rodada = e.rodada
	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case msg.Estado == ui.EtapaAtual:
		if e.aberta == nil || e.aberta.Etapa != msg.Etapa {
			e.desde = time.Now()
		}
		e.aberta = &msg
	case e.aberta != nil && e.aberta.Etapa == msg.Etapa:
		e.aberta = nil
	}
	e.enviar(msg)
}

// falharAberta fecha como falha a etapa em andamento, se houver.
func (e *emissor) falharAberta(nota string) {
	e.mu.Lock()
	aberta, desde := e.aberta, e.desde
	e.mu.Unlock()
	if aberta == nil {
		return
	}
	e.etapaForcada(EtapaMsg{Etapa: aberta.Etapa, Rotulo: aberta.Rotulo, Estado: ui.EtapaFalha, Nota: nota, Duracao: time.Since(desde)})
}

// etapaForcada entrega o estado final de uma etapa mesmo com o contexto
// cancelado, para que quem consome saiba onde a rodada parou. Como o FimMsg,
// conta com o consumidor lendo até o fim.
func (e *emissor) etapaForcada(msg EtapaMsg) {
	msg.Rodada = e.rodada
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.fechado {
		e.ch <- msg
	}
}

// Write implementa io.Writer para o StreamCompletion.
func (e *emissor) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	texto := string(p)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.fechado || e.ctx.Err() != nil {
		return 0, errEscritaEncerrada(e.ctx)
	}
	if !e.enviar(TrechoMsg{Rodada: e.rodada, Texto: texto}) {
		return 0, errEscritaEncerrada(e.ctx)
	}
	e.buf.WriteString(texto)
	return len(p), nil
}

// recomecar descarta o texto de uma tentativa anterior.
func (e *emissor) recomecar() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buf.Reset()
}

func (e *emissor) texto() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.buf.String()
}

// fim entrega o FimMsg mesmo com o contexto cancelado e fecha o canal; o que
// chegar depois (de um provider que ignorou o cancelamento) é descartado.
func (e *emissor) fim(msg FimMsg) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.fechado {
		return
	}
	e.fechado = true
	e.ch <- msg
	close(e.ch)
}

// errEscritaEncerrada é o erro devolvido ao provider depois do fim da rodada.
func errEscritaEncerrada(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("rodada encerrada")
}
