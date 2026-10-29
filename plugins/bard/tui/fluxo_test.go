package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	ybyerrors "github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/retry"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/bard/tools"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// provedorFalso responde a classificação com intencao e escreve trechos no
// StreamCompletion. As primeiras chamadas podem falhar com falhas[i], depois
// de escrever parcialPorFalha. Com travar, escreve o primeiro trecho e espera
// o cancelamento (ou, com ignorarCtx, espera para sempre); cancelado, se dado,
// fecha quando o cancelamento chega.
type provedorFalso struct {
	intencao        string
	trechos         []string
	falhas          []error
	parcialPorFalha string
	travar          bool
	ignorarCtx      bool

	mu             sync.Mutex
	chamadas       int
	entradas       []string
	classificacoes []string // os prompts que o classificador mandou
	liberar        chan struct{}
	cancelado      chan struct{}
}

func (p *provedorFalso) Name() string                       { return "falso" }
func (p *provedorFalso) IsAvailable(_ context.Context) bool { return true }
func (p *provedorFalso) GenerateGovernance(_ context.Context, _ string) (*ai.GovernanceBlueprint, error) {
	return nil, errors.New("não usado")
}
func (p *provedorFalso) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errors.New("não usado")
}

func (p *provedorFalso) Completion(_ context.Context, _, userPrompt string) (string, error) {
	p.mu.Lock()
	p.classificacoes = append(p.classificacoes, userPrompt)
	p.mu.Unlock()
	if p.intencao == "" {
		return `{"intent":"direct","params":{},"direct":true}`, nil
	}
	return p.intencao, nil
}

func (p *provedorFalso) StreamCompletion(ctx context.Context, _, userPrompt string, out io.Writer) error {
	p.mu.Lock()
	p.chamadas++
	n := p.chamadas
	p.entradas = append(p.entradas, userPrompt)
	p.mu.Unlock()

	if n <= len(p.falhas) {
		if p.parcialPorFalha != "" {
			if _, err := out.Write([]byte(p.parcialPorFalha)); err != nil {
				return err
			}
		}
		return p.falhas[n-1]
	}
	for i, t := range p.trechos {
		if _, err := out.Write([]byte(t)); err != nil {
			return err
		}
		if i == 0 && p.travar {
			if p.ignorarCtx {
				<-p.liberar
				return nil
			}
			<-ctx.Done()
			if p.cancelado != nil {
				close(p.cancelado)
			}
			return ctx.Err()
		}
	}
	return nil
}

func (p *provedorFalso) totalChamadas() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.chamadas
}

func (p *provedorFalso) ultimaEntrada() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.entradas) == 0 {
		return ""
	}
	return p.entradas[len(p.entradas)-1]
}

// buscadorFalso devolve documentos fixos ou um erro.
type buscadorFalso struct {
	docs []ai.UnknownDocument
	err  error
}

func (b buscadorFalso) Search(_ context.Context, _ string, _ int) ([]ai.UnknownDocument, error) {
	return b.docs, b.err
}

// retryRapido mantém a forma da política padrão com intervalos de teste.
var retryRapido = retry.Options{
	InitialInterval:     time.Millisecond,
	MaxInterval:         2 * time.Millisecond,
	MaxElapsedTime:      200 * time.Millisecond,
	RandomizationFactor: 0,
	Multiplier:          1,
}

func isolarHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func fluxoDeTeste(p *provedorFalso) *Fluxo {
	// O limiar é o padrão do loadBardConfig, como chega pelo main.go.
	f := NovoFluxo(p, nil, Config{SystemPrompt: "sistema", LimiarRelevancia: ai.RelevanciaMinima})
	f.Retry = retryRapido
	f.BuscarFerramenta = func(string) *tools.Tool { return nil }
	return f
}

// coletar lê eventos até o canal fechar, com prazo para não travar o teste. O prazo pega
// fluxo travado, não mede desempenho: com -race no CI, os testes de orçamento (200 KB de
// ferramenta) passaram de 5 s.
func coletar(t *testing.T, ch <-chan tea.Msg, aoReceber func(tea.Msg)) []tea.Msg {
	t.Helper()
	var eventos []tea.Msg
	prazo := time.After(30 * time.Second)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return eventos
			}
			eventos = append(eventos, msg)
			if aoReceber != nil {
				aoReceber(msg)
			}
		case <-prazo:
			t.Fatalf("o fluxo não terminou; eventos até aqui: %#v", eventos)
		}
	}
}

func fimDe(t *testing.T, eventos []tea.Msg) FimMsg {
	t.Helper()
	if len(eventos) == 0 {
		t.Fatal("nenhum evento recebido")
	}
	fim, ok := eventos[len(eventos)-1].(FimMsg)
	if !ok {
		t.Fatalf("o último evento deveria ser FimMsg, veio %T", eventos[len(eventos)-1])
	}
	for _, ev := range eventos[:len(eventos)-1] {
		if _, dup := ev.(FimMsg); dup {
			t.Fatal("FimMsg deveria aparecer uma única vez")
		}
	}
	return fim
}

func etapasDe(eventos []tea.Msg) []EtapaMsg {
	var etapas []EtapaMsg
	for _, ev := range eventos {
		if e, ok := ev.(EtapaMsg); ok {
			etapas = append(etapas, e)
		}
	}
	return etapas
}

func trechosDe(eventos []tea.Msg) []string {
	var trechos []string
	for _, ev := range eventos {
		if tr, ok := ev.(TrechoMsg); ok {
			trechos = append(trechos, tr.Texto)
		}
	}
	return trechos
}

// ultimaEtapa devolve o estado final de uma etapa.
func ultimaEtapa(t *testing.T, eventos []tea.Msg, etapa Etapa) EtapaMsg {
	t.Helper()
	var achou *EtapaMsg
	for _, e := range etapasDe(eventos) {
		if e.Etapa == etapa {
			e := e
			achou = &e
		}
	}
	if achou == nil {
		t.Fatalf("etapa %d não emitida", etapa)
	}
	return *achou
}

func TestFluxo_RespostaEmTrechos(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"Olá", ", ", "mundo"}}
	eventos := coletar(t, fluxoDeTeste(p).Iniciar(context.Background(), 7, "oi"), nil)

	type passo struct {
		etapa  Etapa
		estado ui.EstadoEtapa
	}
	esperado := []passo{
		{EtapaContexto, ui.EtapaAtual}, {EtapaContexto, ui.EtapaFeita},
		{EtapaIntencao, ui.EtapaAtual}, {EtapaIntencao, ui.EtapaFeita},
		{EtapaResposta, ui.EtapaAtual}, {EtapaResposta, ui.EtapaFeita},
	}
	etapas := etapasDe(eventos)
	if len(etapas) != len(esperado) {
		t.Fatalf("esperava %d eventos de etapa, vieram %d: %#v", len(esperado), len(etapas), etapas)
	}
	for i, e := range etapas {
		if e.Etapa != esperado[i].etapa || e.Estado != esperado[i].estado {
			t.Errorf("etapa %d: esperava %v, veio %d/%d", i, esperado[i], e.Etapa, e.Estado)
		}
		if e.Rodada != 7 {
			t.Errorf("etapa %d com rodada %d", i, e.Rodada)
		}
	}
	if etapas[0].Rotulo != "contexto do projeto" || etapas[1].Nota != "sem Synapstor" {
		t.Errorf("etapa de contexto inesperada: %#v", etapas[1])
	}
	if etapas[3].Nota != "responder direto" {
		t.Errorf("nota da intenção inesperada: %q", etapas[3].Nota)
	}
	if etapas[4].Tentativa != 1 || etapas[4].Rotulo != "gerar resposta" {
		t.Errorf("início da resposta inesperado: %#v", etapas[4])
	}

	if got := strings.Join(trechosDe(eventos), "|"); got != "Olá|, |mundo" {
		t.Errorf("trechos inesperados: %q", got)
	}
	// Os trechos chegam entre o início e o fim da etapa de resposta.
	for i, ev := range eventos {
		if _, ok := ev.(TrechoMsg); ok {
			if e, ok := eventos[i-1].(EtapaMsg); ok && e.Estado != ui.EtapaAtual {
				t.Errorf("trecho fora da etapa de resposta: %#v", e)
			}
		}
	}

	fim := fimDe(t, eventos)
	if fim.Resposta != "Olá, mundo" || fim.Err != nil || fim.Interrompida || fim.Rodada != 7 {
		t.Errorf("fim inesperado: %#v", fim)
	}
	if len(fim.Ferramentas) != 0 {
		t.Errorf("nenhuma ferramenta deveria ter rodado: %v", fim.Ferramentas)
	}
}

func TestFluxo_Erro429TentaDeNovo(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{
		trechos:         []string{"resposta ", "certa"},
		falhas:          []error{&ai.APIError{Provider: "falso", StatusCode: 429, Body: "rate limited"}},
		parcialPorFalha: "lixo da primeira tentativa",
	}
	eventos := coletar(t, fluxoDeTeste(p).Iniciar(context.Background(), 1, "oi"), nil)

	if p.totalChamadas() != 2 {
		t.Fatalf("esperava 2 chamadas ao provider, vieram %d", p.totalChamadas())
	}
	// A falha anuncia a nova tentativa antes da espera; depois ela começa.
	var tentativas []string
	for _, e := range etapasDe(eventos) {
		if e.Etapa == EtapaResposta && e.Estado == ui.EtapaAtual {
			tentativas = append(tentativas, fmt.Sprintf("%d:%s", e.Tentativa, e.Nota))
		}
	}
	if got := strings.Join(tentativas, "|"); got != "1:|2:tentando de novo|2:tentativa 2" {
		t.Errorf("tentativas inesperadas: %q", got)
	}
	fim := fimDe(t, eventos)
	if fim.Err != nil {
		t.Fatalf("a segunda tentativa deveria dar certo: %v", fim.Err)
	}
	if fim.Resposta != "resposta certa" {
		t.Errorf("a resposta não deveria levar o texto da tentativa falha: %q", fim.Resposta)
	}
	if final := ultimaEtapa(t, eventos, EtapaResposta); final.Estado != ui.EtapaFeita {
		t.Errorf("etapa de resposta deveria terminar feita: %#v", final)
	}
}

func TestFluxo_ErroPermanenteUmaTentativa(t *testing.T) {
	casos := []struct {
		nome string
		err  error
	}{
		{"limite de tokens", ybyerrors.New(ybyerrors.ErrCodeTokenLimit, "prompt grande demais")},
		{"chave inválida", &ai.APIError{Provider: "falso", StatusCode: 401, Body: "unauthorized"}},
		{"modelo inexistente", &ai.APIError{Provider: "falso", StatusCode: 404, Body: "model not found"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			isolarHome(t)
			p := &provedorFalso{falhas: []error{c.err, c.err, c.err}, parcialPorFalha: "lixo"}
			eventos := coletar(t, fluxoDeTeste(p).Iniciar(context.Background(), 1, "oi"), nil)

			if p.totalChamadas() != 1 {
				t.Errorf("erro permanente não se repete: %d chamadas", p.totalChamadas())
			}
			for _, e := range etapasDe(eventos) {
				if e.Nota == NotaNovaTentativa {
					t.Errorf("não deveria anunciar nova tentativa: %#v", e)
				}
			}
			fim := fimDe(t, eventos)
			if !errors.Is(fim.Err, c.err) || fim.Interrompida || fim.Resposta != "" {
				t.Errorf("o fim deveria trazer o erro do provider, sem o parcial: %#v", fim)
			}
			if final := ultimaEtapa(t, eventos, EtapaResposta); final.Estado != ui.EtapaFalha {
				t.Errorf("etapa de resposta deveria falhar: %#v", final)
			}
		})
	}
}

func TestFluxo_Erro429Persistente(t *testing.T) {
	isolarHome(t)
	erro429 := &ai.APIError{Provider: "falso", StatusCode: 429, Body: "rate limited"}
	falhas := make([]error, 1000)
	for i := range falhas {
		falhas[i] = erro429
	}
	p := &provedorFalso{falhas: falhas}
	eventos := coletar(t, fluxoDeTeste(p).Iniciar(context.Background(), 1, "oi"), nil)

	fim := fimDe(t, eventos)
	var apiErr *ai.APIError
	if !errors.As(fim.Err, &apiErr) || apiErr.StatusCode != 429 {
		t.Fatalf("esperava o erro 429 no fim, veio %v", fim.Err)
	}
	if fim.Interrompida {
		t.Error("erro do provider não é interrupção")
	}
	if p.totalChamadas() < 2 {
		t.Errorf("deveria ter tentado de novo, chamadas: %d", p.totalChamadas())
	}
	final := ultimaEtapa(t, eventos, EtapaResposta)
	if final.Estado != ui.EtapaFalha || !strings.Contains(final.Nota, "429") {
		t.Errorf("etapa de resposta deveria falhar com o 429: %#v", final)
	}
}

func TestFluxo_CancelamentoNoMeio(t *testing.T) {
	casos := []struct {
		nome       string
		ignorarCtx bool
	}{
		{"provider respeita o contexto", false},
		{"provider ignora o contexto", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			isolarHome(t)
			p := &provedorFalso{
				trechos:    []string{"primeiro", " segundo"},
				travar:     true,
				ignorarCtx: c.ignorarCtx,
				liberar:    make(chan struct{}),
			}
			defer close(p.liberar)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			eventos := coletar(t, fluxoDeTeste(p).Iniciar(ctx, 3, "oi"), func(msg tea.Msg) {
				if _, ok := msg.(TrechoMsg); ok {
					cancel()
				}
			})

			fim := fimDe(t, eventos)
			if !fim.Interrompida || fim.Err != nil {
				t.Fatalf("esperava fim interrompido sem erro: %#v", fim)
			}
			if fim.Resposta != "primeiro" {
				t.Errorf("o fim deveria trazer o que já chegou: %q", fim.Resposta)
			}
			final := ultimaEtapa(t, eventos, EtapaResposta)
			if final.Estado != ui.EtapaFalha || final.Nota != "interrompida" {
				t.Errorf("etapa de resposta deveria ficar interrompida: %#v", final)
			}
			if p.totalChamadas() != 1 {
				t.Errorf("cancelamento não deveria gerar nova tentativa: %d chamadas", p.totalChamadas())
			}
		})
	}
}

func TestFluxo_CanceladoAntesDeComecar(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"nada"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eventos := coletar(t, fluxoDeTeste(p).Iniciar(ctx, 1, "oi"), nil)
	fim := fimDe(t, eventos)
	if !fim.Interrompida || fim.Resposta != "" {
		t.Errorf("esperava fim interrompido e vazio: %#v", fim)
	}
	if p.totalChamadas() != 0 {
		t.Errorf("o provider não deveria ser chamado: %d", p.totalChamadas())
	}
}

func ferramentaDeTeste(saida string, err error, chamadas *int) *tools.Tool {
	return &tools.Tool{
		Name:    "sentinel_investigate",
		Intents: []string{"investigate_pod"},
		Parameters: []tools.ToolParam{
			{Name: "pod", Required: true},
			{Name: "namespace"},
		},
		Execute: func(_ context.Context, params map[string]string) (string, error) {
			*chamadas++
			if params["pod"] != "worker-1" {
				return "", errors.New("parâmetro errado")
			}
			return saida, err
		},
	}
}

func TestFluxo_ComFerramenta(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{
		intencao: "```json\n{\"intent\":\"investigate_pod\",\"params\":{\"pod\":\"worker-1\",\"namespace\":\"payments\"},\"direct\":false}\n```",
		trechos:  []string{"O pod está em CrashLoopBackOff"},
	}
	chamadas := 0
	f := fluxoDeTeste(p)
	ferramenta := ferramentaDeTeste("Status: CrashLoopBackOff\nexit code 1", nil, &chamadas)
	f.BuscarFerramenta = func(intent string) *tools.Tool {
		if intent == "investigate_pod" {
			return ferramenta
		}
		return nil
	}

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "por que o worker reinicia?"), nil)

	if chamadas != 1 {
		t.Fatalf("a ferramenta deveria rodar uma vez, rodou %d", chamadas)
	}
	// Como na prancha "trabalhando": o pedido entendido e, na ferramenta, só o
	// alvo (o namespace opcional fica para o cabeçalho).
	if n := ultimaEtapa(t, eventos, EtapaIntencao).Nota; n != "investigar pod com falha" {
		t.Errorf("nota da intenção inesperada: %q", n)
	}
	var inicio *EtapaMsg
	for _, e := range etapasDe(eventos) {
		if e.Etapa == EtapaFerramenta && e.Estado == ui.EtapaAtual {
			e := e
			inicio = &e
		}
	}
	if inicio == nil || inicio.Rotulo != "sentinel investigate" || inicio.Nota != "worker-1" {
		t.Errorf("início da ferramenta inesperado: %#v", inicio)
	}
	final := ultimaEtapa(t, eventos, EtapaFerramenta)
	if final.Estado != ui.EtapaFeita || !strings.Contains(final.Saida, "CrashLoopBackOff") {
		t.Errorf("fim da ferramenta inesperado: %#v", final)
	}
	if entrada := p.ultimaEntrada(); !strings.Contains(entrada, "Resultado da ferramenta sentinel_investigate") ||
		!strings.Contains(entrada, "exit code 1") {
		t.Errorf("a saída da ferramenta deveria ir para o provider: %q", entrada)
	}
	fim := fimDe(t, eventos)
	if len(fim.Ferramentas) != 1 || fim.Ferramentas[0] != "sentinel_investigate" {
		t.Errorf("ferramentas do fim inesperadas: %v", fim.Ferramentas)
	}
	if fim.Resposta != "O pod está em CrashLoopBackOff" {
		t.Errorf("resposta inesperada: %q", fim.Resposta)
	}
}

func TestFluxo_FerramentaFalha(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{
		intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`,
		trechos:  []string{"não consegui investigar"},
	}
	chamadas := 0
	f := fluxoDeTeste(p)
	ferramenta := ferramentaDeTeste("", errors.New("kubectl não encontrado\ndetalhes"), &chamadas)
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "investiga"), nil)

	final := ultimaEtapa(t, eventos, EtapaFerramenta)
	if final.Estado != ui.EtapaFalha || final.Nota != "kubectl não encontrado" {
		t.Errorf("a ferramenta deveria falhar com a primeira linha do erro: %#v", final)
	}
	if !strings.Contains(p.ultimaEntrada(), "Erro: kubectl não encontrado") {
		t.Errorf("o erro da ferramenta deveria ir para o provider: %q", p.ultimaEntrada())
	}
	if fim := fimDe(t, eventos); fim.Err != nil || fim.Resposta != "não consegui investigar" {
		t.Errorf("a resposta deveria seguir mesmo com a ferramenta falha: %#v", fim)
	}
}

func TestFluxo_CancelamentoNaFerramenta(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`}
	liberar := make(chan struct{})
	defer close(liberar)
	f := fluxoDeTeste(p)
	f.BuscarFerramenta = func(string) *tools.Tool {
		return &tools.Tool{Name: "lenta", Execute: func(context.Context, map[string]string) (string, error) {
			<-liberar // ignora o contexto de propósito
			return "tarde demais", nil
		}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventos := coletar(t, f.Iniciar(ctx, 1, "investiga"), func(msg tea.Msg) {
		if e, ok := msg.(EtapaMsg); ok && e.Etapa == EtapaFerramenta && e.Estado == ui.EtapaAtual {
			cancel()
		}
	})

	if fim := fimDe(t, eventos); !fim.Interrompida || len(fim.Ferramentas) != 0 {
		t.Errorf("esperava fim interrompido sem ferramenta concluída: %#v", fim)
	}
	if final := ultimaEtapa(t, eventos, EtapaFerramenta); final.Nota != "interrompida" {
		t.Errorf("a ferramenta deveria ficar interrompida: %#v", final)
	}
	if p.totalChamadas() != 0 {
		t.Error("a resposta não deveria começar depois do cancelamento")
	}
}

func TestFluxo_ContextoDoSynapstor(t *testing.T) {
	isolarHome(t)
	casos := []struct {
		nome     string
		buscador Buscador
		nota     string
		injeta   bool
	}{
		{"duas notas acima do limiar", buscadorFalso{docs: []ai.UnknownDocument{
			{Content: "rede do postgres", Score: 0.9, Metadata: map[string]string{"title": "Postgres"}},
			{Content: "deploy do worker", Score: 0.7, Metadata: map[string]string{"title": "Worker"}},
			{Content: "irrelevante", Score: 0.2, Metadata: map[string]string{"title": "Outra"}},
		}}, "2 notas do Synapstor", true},
		{"uma nota", buscadorFalso{docs: []ai.UnknownDocument{
			{Content: "rede do postgres", Score: 0.9, Metadata: map[string]string{"title": "Postgres"}},
		}}, "1 nota do Synapstor", true},
		{"nada relevante", buscadorFalso{docs: []ai.UnknownDocument{{Content: "x", Score: 0.1}}}, "nenhuma nota relevante", false},
		{"erro na busca", buscadorFalso{err: errors.New("índice corrompido")}, "Synapstor indisponível", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := &provedorFalso{trechos: []string{"ok"}}
			f := fluxoDeTeste(p)
			f.Buscador = c.buscador
			eventos := coletar(t, f.Iniciar(context.Background(), 1, "o que houve?"), nil)

			if n := ultimaEtapa(t, eventos, EtapaContexto).Nota; n != c.nota {
				t.Errorf("nota esperada %q, veio %q", c.nota, n)
			}
			entrada := p.ultimaEntrada()
			if injeta := strings.Contains(entrada, "Memória Semântica"); injeta != c.injeta {
				t.Errorf("injeção do contexto = %v, esperado %v: %q", injeta, c.injeta, entrada)
			}
			if c.injeta && strings.Contains(entrada, "irrelevante") {
				t.Error("nota abaixo do limiar não deveria entrar no contexto")
			}
		})
	}
}

// O índice feito com outros embeddings não é "Synapstor indisponível": a etapa diz com
// o que ele foi feito e o comando que o refaz, e a pergunta segue sem notas.
func TestFluxo_IndiceDeOutroModelo(t *testing.T) {
	isolarHome(t)
	casos := []struct {
		nome  string
		err   error
		aviso string
	}{
		{"com a identidade do índice", fmt.Errorf("buscar: %w", &ai.IndiceDeOutroModeloError{
			Indice: ai.IdentidadeDoIndice{Provider: "ollama", Modelo: "nomic-embed-text", Dimensao: 768},
			Atual:  ai.IdentidadeDoIndice{Provider: "local", Modelo: "multilingual-e5-small", Dimensao: 384},
		}), "o índice foi feito com ollama/nomic-embed-text (768 dimensões), e os embeddings agora vêm de " +
			"local/multilingual-e5-small (384 dimensões).|rode yby synapstor index --full para refazer o índice"},
		{"índice antigo, sem identidade", &ai.IndiceDeOutroModeloError{
			Atual: ai.IdentidadeDoIndice{Provider: "local", Dimensao: 384},
		}, "o índice foi feito com um modelo de embeddings que não ficou registrado, e os embeddings agora vêm de " +
			"local (384 dimensões).|rode yby synapstor index --full para refazer o índice"},
		{"só o erro sentinela", fmt.Errorf("busca: %w", ai.ErrIndiceDeOutroModelo),
			"o índice foi feito com outros embeddings.|rode yby synapstor index --full para refazer o índice"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := &provedorFalso{trechos: []string{"ok"}}
			f := fluxoDeTeste(p)
			f.Buscador = buscadorFalso{err: c.err}
			eventos := coletar(t, f.Iniciar(context.Background(), 1, "o que houve?"), nil)

			etapa := ultimaEtapa(t, eventos, EtapaContexto)
			// A nota é curta e começa pelo que fazer: a linha da etapa corta o fim.
			if etapa.Nota != NotaIndiceDeOutroModelo {
				t.Errorf("nota esperada %q, veio %q", NotaIndiceDeOutroModelo, etapa.Nota)
			}
			// O comando vai numa linha só, para não se partir na quebra do texto.
			if aviso := strings.ReplaceAll(etapa.Aviso, "\n", "|"); aviso != c.aviso {
				t.Errorf("aviso esperado %q, veio %q", c.aviso, aviso)
			}
			if etapa.Estado != ui.EtapaFalha {
				t.Errorf("o índice de outro modelo é falha da etapa, veio %v", etapa.Estado)
			}
			if strings.Contains(p.ultimaEntrada(), "Memória Semântica") {
				t.Error("sem busca, nada de notas no prompt")
			}
			if fim := fimDe(t, eventos); fim.Err != nil {
				t.Errorf("a pergunta segue sem as notas: %v", fim.Err)
			}
		})
	}
}

// Os trechos do índice entram sob o caminho do arquivo e com o título e a seção que já
// trazem no começo (o Synapstor os põe em cada trecho), sem repeti-los; os do mesmo
// arquivo ficam juntos, sob um cabeçalho só, e a etapa conta arquivos, não trechos.
func TestFluxo_TrechosComCaminhoESecao(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	f.Buscador = buscadorFalso{docs: []ai.UnknownDocument{
		{ID: "decisoes/postgres.md#1", Content: "# Postgres\n## Rede\nrede do postgres", Score: 0.9,
			Metadata: map[string]string{"source": "decisoes/postgres.md", "title": "Postgres", "section": "Rede"}},
		{ID: "runbooks/worker.md#0", Content: "# Worker\ndeploy do worker", Score: 0.8,
			Metadata: map[string]string{"source": "runbooks/worker.md", "title": "Worker"}},
		{ID: "decisoes/postgres.md#2", Content: "# Postgres\n## Backup\nbackup do postgres", Score: 0.7,
			Metadata: map[string]string{"source": "decisoes/postgres.md", "title": "Postgres", "section": "Backup"}},
		{ID: "outra.md#0", Content: "# Outra\nirrelevante", Score: 0.1,
			Metadata: map[string]string{"source": "outra.md", "title": "Outra"}},
	}}
	eventos := coletar(t, f.Iniciar(context.Background(), 1, "como é o postgres?"), nil)

	entrada := p.ultimaEntrada()
	if n := strings.Count(entrada, "decisoes/postgres.md"); n != 1 {
		t.Errorf("o arquivo deveria aparecer uma vez, apareceu %d: %q", n, entrada)
	}
	for _, trecho := range []string{
		"--- Contexto: decisoes/postgres.md ---\n# Postgres\n## Rede\nrede do postgres",
		"# Postgres\n## Backup\nbackup do postgres",
		"--- Contexto: runbooks/worker.md ---\n# Worker\ndeploy do worker",
	} {
		if !strings.Contains(entrada, trecho) {
			t.Errorf("o prompt deveria ter %q: %q", trecho, entrada)
		}
	}
	// O título e a seção saem uma vez por trecho, do próprio trecho.
	if n := strings.Count(entrada, "Postgres"); n != 2 {
		t.Errorf("o título deveria sair só nos dois trechos, saiu %d vezes: %q", n, entrada)
	}
	for _, repetido := range []string{"[seção:", "Postgres (", "Worker ("} {
		if strings.Contains(entrada, repetido) {
			t.Errorf("o prompt não deveria repetir %q, que o trecho já traz: %q", repetido, entrada)
		}
	}
	// O trecho de menor pontuação do Postgres fica com o arquivo, antes do Worker.
	if strings.Index(entrada, "backup do postgres") > strings.Index(entrada, "deploy do worker") {
		t.Errorf("os trechos do mesmo arquivo deveriam ficar juntos: %q", entrada)
	}
	if strings.Contains(entrada, "irrelevante") {
		t.Error("o trecho abaixo do corte não deveria entrar")
	}
	if n := ultimaEtapa(t, eventos, EtapaContexto).Nota; n != "2 notas do Synapstor" {
		t.Errorf("a etapa deveria contar os arquivos: %q", n)
	}
}

// Trecho que não traz o título nem a seção no começo (sem o prefixo do Synapstor) entra
// com eles no cabeçalho e na marca de seção.
func TestFluxo_TrechoSemTituloNemSecaoNoTexto(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	f.Buscador = buscadorFalso{docs: []ai.UnknownDocument{
		{ID: "decisoes/postgres.md#1", Content: "rede do postgres", Score: 0.9,
			Metadata: map[string]string{"source": "decisoes/postgres.md", "title": "Postgres", "section": "Rede"}},
	}}
	coletar(t, f.Iniciar(context.Background(), 1, "como é o postgres?"), nil)

	if entrada := p.ultimaEntrada(); !strings.Contains(entrada, "--- Contexto: Postgres (decisoes/postgres.md) ---\n[seção: Rede]\nrede do postgres") {
		t.Errorf("o título e a seção deveriam vir de fora do trecho: %q", entrada)
	}
}

// O corte compara em float32, como o Synapstor: o trecho com a pontuação exata do corte
// (float32(ai.RelevanciaMinima)) entra, e o logo abaixo fica de fora.
func TestNotasPorArquivo_FronteiraDoCorte(t *testing.T) {
	corte := float32(ai.RelevanciaMinima)
	notas := notasPorArquivo([]ai.UnknownDocument{
		{ID: "a.md#0", Score: corte, Metadata: map[string]string{"source": "a.md"}},
		{ID: "b.md#0", Score: math.Nextafter32(corte, 0), Metadata: map[string]string{"source": "b.md"}},
	}, ai.RelevanciaMinima)
	if len(notas) != 1 || notas[0].caminho != "a.md" {
		t.Errorf("só o trecho no corte deveria passar: %+v", notas)
	}
}

// Muitos trechos do mesmo arquivo ficam no teto das notas.
func TestFluxo_TrechosDoMesmoArquivoNoTeto(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := fluxoDeTeste(p)
	var docs []ai.UnknownDocument
	for i := 0; i < 8; i++ {
		docs = append(docs, ai.UnknownDocument{
			ID: fmt.Sprintf("guia.md#%d", i), Content: linhasNumeradas(30, 100), Score: 0.9,
			Metadata: map[string]string{"source": "guia.md", "title": "Guia", "section": fmt.Sprintf("Parte %d", i)},
		})
	}
	f.Buscador = buscadorFalso{docs: docs}
	eventos := coletar(t, f.Iniciar(context.Background(), 1, "como faço?"), nil)

	if entrada := p.ultimaEntrada(); len(entrada) > TetoNotas+folgaMarca+folgaPrompt {
		t.Errorf("os trechos deveriam ficar no teto: %d bytes", len(entrada))
	}
	if n := ultimaEtapa(t, eventos, EtapaContexto).Nota; n != "1 nota do Synapstor" {
		t.Errorf("os trechos são de uma nota só: %q", n)
	}
}

func TestNovoFluxo_Padroes(t *testing.T) {
	f := NovoFluxo(nil, nil, Config{SystemPrompt: "s"})
	if f.Buscador != nil {
		t.Error("sem vector store o buscador deveria ser nil")
	}
	// O limiar 0 vale como veio (aceita todas as notas); o
	// padrão, ai.RelevanciaMinima, vem do loadBardConfig.
	if f.TopK != 5 || f.Limiar != 0 {
		t.Errorf("padrões inesperados: topK=%d limiar=%v", f.TopK, f.Limiar)
	}
	if f.Retry != retryPadrao {
		t.Error("o retry padrão deveria ser retryPadrao")
	}
	f = NovoFluxo(nil, nil, Config{TopK: 3, LimiarRelevancia: 0.8, Namespace: "payments"})
	if f.TopK != 3 || f.Limiar != 0.8 || f.Namespace != "payments" {
		t.Errorf("config ignorada: topK=%d limiar=%v ns=%q", f.TopK, f.Limiar, f.Namespace)
	}
	if f = NovoFluxo(nil, nil, Config{LimiarRelevancia: -1}); f.Limiar != ai.RelevanciaMinima {
		t.Errorf("limiar negativo deveria virar o corte do pkg/ai: %v", f.Limiar)
	}
}

func TestFluxo_LimiarZeroAceitaTodas(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{trechos: []string{"ok"}}
	f := NovoFluxo(p, nil, Config{SystemPrompt: "sistema", LimiarRelevancia: 0})
	f.Retry = retryRapido
	f.BuscarFerramenta = func(string) *tools.Tool { return nil }
	f.Buscador = buscadorFalso{docs: []ai.UnknownDocument{{Content: "pouco relevante", Score: 0.1, Metadata: map[string]string{"title": "X"}}}}
	eventos := coletar(t, f.Iniciar(context.Background(), 1, "oi"), nil)
	if n := ultimaEtapa(t, eventos, EtapaContexto).Nota; n != "1 nota do Synapstor" {
		t.Errorf("limiar 0 deveria aceitar a nota: %q", n)
	}
}

// provedorQuePanica entra em panic no StreamCompletion.
type provedorQuePanica struct{ *provedorFalso }

func (p provedorQuePanica) StreamCompletion(_ context.Context, _, _ string, out io.Writer) error {
	_, _ = out.Write([]byte("meio"))
	panic("índice fora do intervalo")
}

func TestFluxo_PanicViraErro(t *testing.T) {
	t.Run("no provider", func(t *testing.T) {
		isolarHome(t)
		f := fluxoDeTeste(&provedorFalso{})
		f.Provider = provedorQuePanica{&provedorFalso{}}
		eventos := coletar(t, f.Iniciar(context.Background(), 1, "oi"), nil)

		fim := fimDe(t, eventos)
		if fim.Err == nil || !strings.Contains(fim.Err.Error(), "índice fora do intervalo") || fim.Interrompida {
			t.Fatalf("o panic deveria virar erro da rodada: %#v", fim)
		}
		if final := ultimaEtapa(t, eventos, EtapaResposta); final.Estado != ui.EtapaFalha || final.Nota != "falha interna" {
			t.Errorf("a etapa em andamento deveria fechar como falha: %#v", final)
		}
	})
	t.Run("na ferramenta", func(t *testing.T) {
		isolarHome(t)
		p := &provedorFalso{intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`}
		f := fluxoDeTeste(p)
		f.BuscarFerramenta = func(string) *tools.Tool {
			return &tools.Tool{Name: "externa", Execute: func(context.Context, map[string]string) (string, error) {
				panic("ferramenta quebrada")
			}}
		}
		eventos := coletar(t, f.Iniciar(context.Background(), 1, "investiga"), nil)

		fim := fimDe(t, eventos)
		if fim.Err == nil || !strings.HasPrefix(fim.Err.Error(), "falha interna do Bard") {
			t.Fatalf("o panic da ferramenta deveria virar erro: %#v", fim)
		}
		if final := ultimaEtapa(t, eventos, EtapaFerramenta); final.Estado != ui.EtapaFalha {
			t.Errorf("a ferramenta deveria fechar como falha: %#v", final)
		}
		if p.totalChamadas() != 0 {
			t.Error("a resposta não deveria começar depois do panic")
		}
	})
}

func TestFluxo_CancelamentoNaEsperaDescartaTentativaFalha(t *testing.T) {
	isolarHome(t)
	falhas := make([]error, 10)
	for i := range falhas {
		falhas[i] = &ai.APIError{Provider: "falso", StatusCode: 429, Body: "rate limited"}
	}
	p := &provedorFalso{falhas: falhas, parcialPorFalha: "lixo"}
	f := fluxoDeTeste(p)
	f.Retry = retryPadrao // espera de segundos entre as tentativas
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventos := coletar(t, f.Iniciar(ctx, 1, "oi"), func(msg tea.Msg) {
		if e, ok := msg.(EtapaMsg); ok && e.Nota == "tentando de novo" {
			cancel()
		}
	})

	fim := fimDe(t, eventos)
	if !fim.Interrompida || fim.Err != nil || fim.Resposta != "" {
		t.Errorf("o texto da tentativa falha não deveria chegar ao fim: %#v", fim)
	}
	if p.totalChamadas() != 1 {
		t.Errorf("cancelado na espera, só a primeira chamada: %d", p.totalChamadas())
	}
}

func TestPedidoEntendido(t *testing.T) {
	casos := []struct {
		intent string
		tool   *tools.Tool
		quer   string
	}{
		{"investigate_pod", &tools.Tool{Description: "Investiga um pod"}, "investigar pod com falha"},
		{"pod_logs", &tools.Tool{}, "ver logs do pod"},
		{"argocd_status", &tools.Tool{Description: "Consulta o status do Argo CD.\nmais detalhes"}, "consulta o status do Argo CD"},
		{"argocd_sync", &tools.Tool{}, "argocd sync"},
	}
	for _, c := range casos {
		if got := pedidoEntendido(c.intent, c.tool); got != c.quer {
			t.Errorf("%s: esperava %q, veio %q", c.intent, c.quer, got)
		}
	}
}

func TestEmissor_DescartaDepoisDoFim(t *testing.T) {
	e := &emissor{ctx: context.Background(), ch: make(chan tea.Msg, 4), rodada: 1}
	e.fim(FimMsg{Rodada: 1})
	e.fim(FimMsg{Rodada: 1}) // segundo fim é ignorado
	if _, err := e.Write([]byte("atrasado")); err == nil {
		t.Error("escrever depois do fim deveria falhar")
	}
	e.etapa(EtapaMsg{Etapa: EtapaResposta})
	var n int
	for range e.ch {
		n++
	}
	if n != 1 {
		t.Errorf("esperava só o FimMsg no canal, vieram %d eventos", n)
	}
	if n, err := e.Write(nil); n != 0 || err != nil {
		t.Errorf("escrita vazia deveria ser neutra: %d, %v", n, err)
	}
}

func TestPrimeiraLinhaEAlvo(t *testing.T) {
	if got := primeiraLinha("  um\ndois "); got != "um" {
		t.Errorf("primeiraLinha: %q", got)
	}
	casos := []struct {
		nome   string
		params []tools.ToolParam
		valor  map[string]string
		quer   string
	}{
		{"obrigatórios em forma de comando", []tools.ToolParam{
			{Name: "resource", Required: true}, {Name: "name", Required: true}, {Name: "namespace"},
		}, map[string]string{"name": "worker-1", "resource": "pod", "namespace": "outro"}, "pod worker-1"},
		{"sem obrigatório, entram os opcionais", []tools.ToolParam{
			{Name: "namespace"}, {Name: "tail"},
		}, map[string]string{"namespace": "outro", "tail": "50", "x": "9"}, "outro 50"},
		{"namespace da sessão fica de fora", []tools.ToolParam{
			{Name: "namespace"},
		}, map[string]string{"namespace": "payments"}, ""},
		{"obrigatório vazio", []tools.ToolParam{
			{Name: "pod", Required: true}, {Name: "namespace"},
		}, map[string]string{"pod": " ", "namespace": "outro"}, "outro"},
	}
	for _, c := range casos {
		if got := alvoFerramenta(&tools.Tool{Parameters: c.params}, c.valor, "payments"); got != c.quer {
			t.Errorf("%s: esperava %q, veio %q", c.nome, c.quer, got)
		}
	}
}

// As notas do Synapstor e a saída da ferramenta vão juntas ao provider.
func TestFluxo_FerramentaMantemNotasDoSynapstor(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{
		intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`,
		trechos:  []string{"ok"},
	}
	chamadas := 0
	f := fluxoDeTeste(p)
	f.Buscador = buscadorFalso{docs: []ai.UnknownDocument{
		{Content: "NOTA-X", Metadata: map[string]string{"title": "runbook"}, Score: 0.9},
	}}
	f.TopK = 5
	f.Limiar = 0.6
	ferramenta := ferramentaDeTeste("SAIDA-Y", nil, &chamadas)
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	coletar(t, f.Iniciar(context.Background(), 1, "por que o worker reinicia?"), nil)

	entrada := p.ultimaEntrada()
	if !strings.Contains(entrada, "SAIDA-Y") || !strings.Contains(entrada, "NOTA-X") {
		t.Errorf("o prompt deveria ter a saída da ferramenta e as notas: %q", entrada)
	}
}

// A saída de toda ferramenta, e o erro dela, passa pela redação antes de ir ao
// provider e à tela ("↵ ver" mostra o que a IA recebeu).
func TestFluxo_SaidaDaFerramentaRedigida(t *testing.T) {
	isolarHome(t)
	casos := []struct {
		nome  string
		saida string
		err   error
	}{
		{"saída", "token=abcdefghijklmnopqrstuvwxyz123\nDB_PASSWORD: hunter2\nStatus: Running", nil},
		{"erro", "", errors.New("kubectl falhou\ntoken=abcdefghijklmnopqrstuvwxyz123\nDB_PASSWORD: hunter2")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := &provedorFalso{
				intencao: `{"intent":"investigate_pod","params":{"pod":"worker-1"},"direct":false}`,
				trechos:  []string{"ok"},
			}
			chamadas := 0
			f := fluxoDeTeste(p)
			ferramenta := ferramentaDeTeste(c.saida, c.err, &chamadas)
			f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

			eventos := coletar(t, f.Iniciar(context.Background(), 1, "investiga"), nil)

			final := ultimaEtapa(t, eventos, EtapaFerramenta)
			for _, texto := range []string{p.ultimaEntrada(), final.Saida, final.Nota} {
				if strings.Contains(texto, "abcdefghijklmnopqrstuvwxyz123") || strings.Contains(texto, "hunter2") {
					t.Errorf("segredo da ferramenta vazou: %q", texto)
				}
			}
			if !strings.Contains(p.ultimaEntrada(), "[REDIGIDO]") {
				t.Errorf("o prompt deveria levar a marca da redação: %q", p.ultimaEntrada())
			}
		})
	}
}

func TestFluxo_ObrigatorioFaltandoNaoExecuta(t *testing.T) {
	isolarHome(t)
	p := &provedorFalso{
		intencao: `{"intent":"investigate_pod","params":{"namespace":"payments"},"direct":false}`,
		trechos:  []string{"Qual pod?"},
	}
	chamadas := 0
	f := fluxoDeTeste(p)
	ferramenta := ferramentaDeTeste("saída", nil, &chamadas)
	f.BuscarFerramenta = func(string) *tools.Tool { return ferramenta }

	eventos := coletar(t, f.Iniciar(context.Background(), 1, "investiga"), nil)

	if chamadas != 0 {
		t.Fatalf("a ferramenta não deveria rodar sem o pod, rodou %d", chamadas)
	}
	e := ultimaEtapa(t, eventos, EtapaFerramenta)
	if e.Estado != ui.EtapaFalha || !strings.Contains(e.Saida, "pod") {
		t.Errorf("esperava falha clara citando o pod: %+v", e)
	}
}

// capturaClassificador guarda o que o classificador recebeu.
type capturaClassificador struct {
	*provedorFalso
	sistema, usuario string
}

func (c *capturaClassificador) Completion(_ context.Context, sistema, usuario string) (string, error) {
	c.sistema, c.usuario = sistema, usuario
	return `{"intent":"direct","params":{},"direct":true}`, nil
}

func TestClassifyIntent_PromptTemSessaoEEsquema(t *testing.T) {
	salvas := tools.All()
	tools.Reset()
	t.Cleanup(func() {
		tools.Reset()
		for _, s := range salvas {
			tools.Register(s)
		}
	})
	tools.Register(&tools.Tool{Name: "kubectl_logs", Description: "Logs", Intents: []string{"pod_logs", "view_logs"},
		Parameters: []tools.ToolParam{{Name: "pod", Required: true}, {Name: "tail"}}})
	c := &capturaClassificador{provedorFalso: &provedorFalso{}}

	classifyIntent(context.Background(), c, "logs do api", "", "prod", "payments")

	for _, esperado := range []string{"pod_logs", "view_logs", "pod*", "tail", "prod", "payments"} {
		if !strings.Contains(c.usuario, esperado) {
			t.Errorf("falta %q no prompt:\n%s", esperado, c.usuario)
		}
	}
	if strings.Contains(strings.ToLower(c.usuario+c.sistema), "default") {
		t.Errorf("o prompt não deve mandar usar default:\n%s\n%s", c.sistema, c.usuario)
	}
}

// Num terminal de 80 colunas, a etapa de contexto mostra a nota inteira; o aviso com o
// modelo do índice e o comando que o refaz fica na conversa depois da rodada, quebrado na
// largura da tela.
func TestModel_IndiceDeOutroModeloNaTela(t *testing.T) {
	semCores(t)
	m := modeloComFluxo(t, &provedorFalso{trechos: []string{"ok"}}, nil)
	atualizado, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = atualizado.(Model)
	m.fluxo.Buscador = buscadorFalso{err: &ai.IndiceDeOutroModeloError{
		Indice: ai.IdentidadeDoIndice{Provider: "ollama", Modelo: "nomic-embed-text", Dimensao: 768},
		Atual:  ai.IdentidadeDoIndice{Provider: "local", Modelo: "multilingual-e5-small", Dimensao: 384},
	}}
	m, cmd := enviar(t, m, "o que houve?")
	var etapas string
	m = rodar(t, m, cmd, func(m Model, msg tea.Msg) Model {
		if e, ok := msg.(EtapaMsg); ok && e.Etapa == EtapaContexto && e.Estado == ui.EtapaFalha {
			etapas = m.etapasVista(76)
		}
		return m
	})
	if !strings.Contains(etapas, NotaIndiceDeOutroModelo) {
		t.Errorf("a 76 colunas a etapa deveria mostrar a nota inteira:\n%s", etapas)
	}

	linhas := strings.Split(strings.TrimRight(m.historico, "\n"), "\n")
	for _, l := range linhas {
		if w := lipgloss.Width(l); w > m.viewport.Width {
			t.Errorf("linha com %d colunas numa conversa de %d: %q", w, m.viewport.Width, l)
		}
	}
	tela := strings.Join(strings.Fields(strings.Join(linhas, " ")), " ")
	if !strings.Contains(tela, "o índice foi feito com ollama/nomic-embed-text (768 dimensões)") {
		t.Errorf("depois da rodada a conversa deveria dizer com que modelo o índice foi feito:\n%s", m.historico)
	}
	comando := false
	for _, l := range linhas {
		comando = comando || strings.Contains(l, "rode yby synapstor index --full")
	}
	if !comando {
		t.Errorf("depois da rodada a conversa deveria guardar o comando numa linha só:\n%s", m.historico)
	}
}
