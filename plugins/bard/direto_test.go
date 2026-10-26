package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/casheiro/yby-cli/plugins/bard/tui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// envProcessoTeste faz o binário de teste rodar como o plugin (TestMain), para os testes
// de processo exercitarem o stdin, o stdout e o stderr de verdade.
const envProcessoTeste = "YBY_BARD_TESTE_PROCESSO"

func TestMain(m *testing.M) {
	if os.Getenv(envProcessoTeste) == "1" {
		os.Exit(executar())
	}
	os.Exit(m.Run())
}

// iaFalsa responde a classificação com "direct" e escreve a resposta em trechos. As
// primeiras chamadas ao StreamCompletion falham com falhas[i], depois de escrever
// parcial.
type iaFalsa struct {
	mu       sync.Mutex
	trechos  func(pergunta string) []string
	falhas   []error
	parcial  string
	chamadas int
}

func (p *iaFalsa) Name() string                       { return "IA falsa" }
func (p *iaFalsa) IsAvailable(_ context.Context) bool { return true }
func (p *iaFalsa) GenerateGovernance(_ context.Context, _ string) (*ai.GovernanceBlueprint, error) {
	return nil, errors.New("não usado")
}
func (p *iaFalsa) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errors.New("não usado")
}
func (p *iaFalsa) Completion(_ context.Context, _, _ string) (string, error) {
	return `{"intent":"direct","params":{},"direct":true}`, nil
}
func (p *iaFalsa) StreamCompletion(_ context.Context, _, pergunta string, out io.Writer) error {
	p.mu.Lock()
	n := p.chamadas
	p.chamadas++
	p.mu.Unlock()
	if n < len(p.falhas) {
		if p.parcial != "" {
			_, _ = io.WriteString(out, p.parcial)
		}
		return p.falhas[n]
	}
	trechos := []string{"resposta ", "para ", pergunta}
	if p.trechos != nil {
		trechos = p.trechos(pergunta)
	}
	for _, t := range trechos {
		if _, err := io.WriteString(out, t); err != nil {
			return err
		}
	}
	return nil
}

// retryDeTeste mantém a forma da política padrão com intervalos curtos.
var retryDeTeste = retry.Options{
	InitialInterval:     time.Millisecond,
	MaxInterval:         2 * time.Millisecond,
	MaxElapsedTime:      200 * time.Millisecond,
	RandomizationFactor: 0,
	Multiplier:          1,
}

// semNovaTentativa desiste já na primeira falha (o tempo máximo acaba antes da espera).
var semNovaTentativa = retry.Options{
	InitialInterval: time.Millisecond,
	MaxInterval:     time.Millisecond,
	MaxElapsedTime:  time.Nanosecond,
	Multiplier:      1,
}

// novoDiretoDeTeste monta um direto com a IA falsa e saídas em buffers, sem cores.
func novoDiretoDeTeste(t *testing.T, p ai.Provider, etapas, continua bool) (*direto, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	isolarModoVisual(t, "")
	lipgloss.SetColorProfile(termenv.Ascii)
	f := tui.NovoFluxo(p, nil, tui.Config{SystemPrompt: "sistema", TopK: 5, LimiarRelevancia: 0.6})
	f.Retry = retryDeTeste
	f.BuscarFerramenta = func(string) *tools.Tool { return nil }
	var saida, erros bytes.Buffer
	return &direto{fluxo: f, saida: &saida, erros: &erros, etapas: etapas, continua: continua}, &saida, &erros
}

func semControle(t *testing.T, nome, s string) {
	t.Helper()
	if strings.ContainsRune(s, '\x1b') {
		t.Errorf("%s com códigos de controle: %q", nome, s)
	}
}

func TestDireto_PipeSoARespostaNoStdout(t *testing.T) {
	d, saida, erros := novoDiretoDeTeste(t, &iaFalsa{}, false, false)
	if err := d.perguntar(context.Background(), "quantos pods?"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "resposta para quantos pods?\n"; got != want {
		t.Errorf("stdout = %q, esperado %q", got, want)
	}
	if erros.Len() != 0 {
		t.Errorf("fora do terminal o stderr deveria ficar vazio: %q", erros.String())
	}
	semControle(t, "stdout", saida.String())
}

func TestDireto_TerminalEscreveARespostaPorLinha(t *testing.T) {
	ia := &iaFalsa{trechos: func(string) []string { return []string{"primeira ", "linha\nsegunda", " linha"} }}
	d, saida, _ := novoDiretoDeTeste(t, ia, false, true)
	if err := d.perguntar(context.Background(), "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "primeira linha\nsegunda linha\n"; got != want {
		t.Errorf("stdout = %q, esperado %q", got, want)
	}
}

func TestDireto_EtapasNoStderr(t *testing.T) {
	d, saida, erros := novoDiretoDeTeste(t, &iaFalsa{}, true, false)
	d.fluxo.BuscarFerramenta = func(string) *tools.Tool {
		return &tools.Tool{
			Name:       "kubectl_get",
			Parameters: []tools.ToolParam{{Name: "resource", Required: true}},
			Execute:    func(context.Context, map[string]string) (string, error) { return "NAME READY\napi 1/1", nil },
		}
	}
	d.fluxo.Provider = &iaClassifica{iaFalsa: &iaFalsa{}, json: `{"intent":"list_pods","params":{"resource":"pods"},"direct":false}`}
	if err := d.perguntar(context.Background(), "lista os pods"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.HasPrefix(saida.String(), "resposta para Pergunta do usuario: lista os pods") {
		t.Errorf("stdout inesperado: %q", saida.String())
	}
	tela := ansi.Strip(erros.String())
	for _, trecho := range []string{"✓ contexto do projeto", "sem Synapstor", "✓ entender o pedido", "listar pods",
		"✓ kubectl get", "pods", "✓ gerar resposta"} {
		if !strings.Contains(tela, trecho) {
			t.Errorf("stderr sem %q:\n%s", trecho, tela)
		}
	}
	semControle(t, "stdout", saida.String())
}

// iaClassifica é a IA falsa com outra resposta para a classificação.
type iaClassifica struct {
	*iaFalsa
	json string
}

func (p *iaClassifica) Completion(_ context.Context, _, _ string) (string, error) { return p.json, nil }

func TestDireto_RespostaQueRecomecaNoTerminal(t *testing.T) {
	ia := &iaFalsa{falhas: []error{&ai.APIError{Provider: "falso", StatusCode: 503, Body: "indisponível"}}, parcial: "começo"}
	d, saida, erros := novoDiretoDeTeste(t, ia, true, true)
	if err := d.perguntar(context.Background(), "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "começo\nresposta para oi\n"; got != want {
		t.Errorf("stdout = %q, esperado %q", got, want)
	}
	if !strings.Contains(ansi.Strip(erros.String()), "▲ a resposta recomeçou depois de uma falha do provider") {
		t.Errorf("faltou o aviso do recomeço:\n%s", ansi.Strip(erros.String()))
	}
}

func TestDireto_RespostaQueRecomecaNoPipe(t *testing.T) {
	ia := &iaFalsa{falhas: []error{&ai.APIError{Provider: "falso", StatusCode: 503, Body: "indisponível"}}, parcial: "começo"}
	d, saida, _ := novoDiretoDeTeste(t, ia, false, false)
	if err := d.perguntar(context.Background(), "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "resposta para oi\n"; got != want {
		t.Errorf("no pipe só a última tentativa deveria sair: %q", got)
	}
}

func TestDireto_FalhaDoProvider(t *testing.T) {
	falha := errors.New("401 chave inválida")
	d, saida, _ := novoDiretoDeTeste(t, &iaFalsa{falhas: []error{falha}}, false, false)
	d.fluxo.Retry = semNovaTentativa
	err := d.perguntar(context.Background(), "oi")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeExec || !errors.Is(err, falha) {
		t.Fatalf("esperava ERR_EXEC_FAILED com a causa, veio %v", err)
	}
	if saida.Len() != 0 {
		t.Errorf("sem resposta, o stdout deveria ficar vazio: %q", saida.String())
	}
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 1 {
		t.Errorf("código = %d, esperado 1", codigo)
	}
	texto := b.String()
	if !strings.HasPrefix(texto, "\n✕ a IA não respondeu") {
		t.Errorf("o erro deveria vir depois de uma linha em branco:\n%q", texto)
	}
	for _, trecho := range []string{"ERR_EXEC_FAILED", "401 chave inválida", "confira o provider de IA com yby doctor",
		"YBY_LOG_LEVEL=debug", "provider=ia falsa"} {
		if !strings.Contains(texto, trecho) {
			t.Errorf("erro sem %q:\n%s", trecho, texto)
		}
	}
	// o --log-level do core não chega ao plugin: o erro não pode mandar usá-lo
	if strings.Contains(texto, "--log-level") {
		t.Errorf("o erro aponta um --log-level que não funciona no Bard:\n%s", texto)
	}
	semControle(t, "stderr", texto)
}

// A falha da última tentativa, com parte da resposta já na tela, não anuncia um recomeço
// que não vem; entre a etapa que falhou e o erro fica uma linha em branco.
func TestDireto_FalhaFinalComRespostaParcial(t *testing.T) {
	ia := &iaFalsa{falhas: []error{errors.New("401 chave inválida")}, parcial: "começo"}
	d, saida, erros := novoDiretoDeTeste(t, ia, true, true)
	d.fluxo.Retry = semNovaTentativa
	err := d.perguntar(context.Background(), "oi")
	if err == nil {
		t.Fatal("esperava a falha do provider")
	}
	sairComErro(d.erros, err)
	if got := saida.String(); got != "começo\n" {
		t.Errorf("stdout = %q, esperado %q", got, "começo\n")
	}
	tela := ansi.Strip(erros.String())
	if strings.Contains(tela, "recomeçou") {
		t.Errorf("sem nova tentativa, nada recomeça:\n%s", tela)
	}
	// a etapa pode trazer o tempo no fim da linha ("0,0 s"), conforme a resolução do relógio
	if !strings.Contains(tela, "✕ gerar resposta") || !regexp.MustCompile(`401 chave inválida[^\n]*\n\n✕ a IA não respondeu`).MatchString(tela) {
		t.Errorf("faltou a etapa que falhou e a linha em branco antes do erro:\n%s", tela)
	}
}

// A nova tentativa que começa sem nada da resposta na tela aparece como detalhe da
// etapa, como a nota "tentativa 2" da TUI.
func TestDireto_NovaTentativaSemTextoMostraANota(t *testing.T) {
	ia := &iaFalsa{falhas: []error{&ai.APIError{Provider: "falso", StatusCode: 503, Body: "indisponível"}}}
	d, saida, erros := novoDiretoDeTeste(t, ia, true, true)
	if err := d.perguntar(context.Background(), "oi"); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "resposta para oi\n"; got != want {
		t.Errorf("stdout = %q, esperado %q", got, want)
	}
	tela := ansi.Strip(erros.String())
	if !strings.Contains(tela, "tentativa 2") || strings.Contains(tela, "recomeçou") {
		t.Errorf("esperava a nota da tentativa 2 sem aviso de recomeço:\n%s", tela)
	}
}

func TestDireto_Interrompida(t *testing.T) {
	d, saida, _ := novoDiretoDeTeste(t, &iaFalsa{}, false, false)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	err := d.perguntar(ctx, "oi")
	var s saidaSilenciosa
	if !errors.As(err, &s) || s.codigo != 130 {
		t.Fatalf("esperava saída silenciosa 130, veio %v", err)
	}
	if saida.Len() != 0 {
		t.Errorf("interrompida, nada deveria ir para o stdout: %q", saida.String())
	}
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 130 || b.Len() != 0 {
		t.Errorf("saída silenciosa escreveu %q com código %d", b.String(), codigo)
	}
}

func TestLote_UmaPerguntaPorLinha(t *testing.T) {
	d, saida, erros := novoDiretoDeTeste(t, &iaFalsa{}, false, false)
	if err := d.lote(context.Background(), strings.NewReader("primeira\n\n  segunda  \n")); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got, want := saida.String(), "resposta para primeira\n"+separadorLote+"resposta para segunda\n"; got != want {
		t.Errorf("stdout = %q, esperado %q", got, want)
	}
	if erros.Len() != 0 {
		t.Errorf("stderr deveria ficar vazio: %q", erros.String())
	}
}

func TestLote_SemPerguntas(t *testing.T) {
	d, _, _ := novoDiretoDeTeste(t, &iaFalsa{}, false, false)
	err := d.lote(context.Background(), strings.NewReader("\n  \n"))
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.GetHint(), "yby bard -p") {
		t.Fatalf("esperava ERR_VALIDATION com a dica do -p, veio %v", err)
	}
}

func TestLote_FalhaDeUmaPerguntaNaoParaOLote(t *testing.T) {
	// Só a primeira chamada falha e, sem nova tentativa, a primeira pergunta fica sem
	// resposta e a segunda responde.
	ia := &iaFalsa{falhas: []error{errors.New("401 chave inválida")}}
	d, saida, erros := novoDiretoDeTeste(t, ia, false, false)
	d.fluxo.Retry = semNovaTentativa
	err := d.lote(context.Background(), strings.NewReader("primeira\nsegunda\n"))
	var s saidaSilenciosa
	if !errors.As(err, &s) || s.codigo != 1 {
		t.Fatalf("esperava saída silenciosa 1, veio %v", err)
	}
	if !strings.Contains(saida.String(), "resposta para segunda") {
		t.Errorf("a segunda pergunta deveria ter resposta: %q", saida.String())
	}
	if texto := erros.String(); !strings.Contains(texto, "a IA não respondeu") || !strings.Contains(texto, "pergunta=primeira") {
		t.Errorf("o erro deveria dizer qual pergunta falhou:\n%s", texto)
	}
}

func TestLote_Interrompido(t *testing.T) {
	d, _, _ := novoDiretoDeTeste(t, &iaFalsa{}, false, false)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	err := d.lote(ctx, strings.NewReader("primeira\n"))
	var s saidaSilenciosa
	if !errors.As(err, &s) || s.codigo != 130 {
		t.Fatalf("esperava saída silenciosa 130, veio %v", err)
	}
}

func TestNovoDireto_SemProvider(t *testing.T) {
	isolarModoVisual(t, "")
	anterior := obterProvider
	obterProvider = func(context.Context) ai.Provider { return nil }
	t.Cleanup(func() { obterProvider = anterior })
	_, err := novoDireto(context.Background(), nil, opcoesBard{})
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeConfig {
		t.Fatalf("esperava ERR_CONFIG, veio %v", err)
	}
}

func TestSairComErro_ErroComumViraErrPlugin(t *testing.T) {
	isolarModoVisual(t, "")
	lipgloss.SetColorProfile(termenv.Ascii)
	var b bytes.Buffer
	if codigo := sairComErro(&b, errors.New("algo quebrou")); codigo != 1 {
		t.Errorf("código = %d, esperado 1", codigo)
	}
	if texto := b.String(); !strings.Contains(texto, "✕ o Bard falhou") || !strings.Contains(texto, "ERR_PLUGIN") || !strings.Contains(texto, "algo quebrou") {
		t.Errorf("erro inesperado:\n%s", texto)
	}
}

func TestPorLinha_FecharSoComResto(t *testing.T) {
	var b bytes.Buffer
	l := &porLinha{destino: &b}
	l.fechar()
	_, _ = l.Write([]byte("a\nb"))
	if b.String() != "a\n" {
		t.Fatalf("só a linha inteira deveria sair: %q", b.String())
	}
	l.fechar()
	l.fechar()
	if b.String() != "a\nb\n" {
		t.Errorf("fechar deveria escrever o resto uma vez: %q", b.String())
	}
}

// --- Testes de processo: o plugin sem terminal, com um Ollama falso ---

// ollamaFalso imita o Ollama: /api/tags lista um modelo, /api/generate sem stream
// classifica como "direct" e com stream responde "resposta para <pergunta>". Qualquer
// outro caminho conta como inesperado.
type ollamaFalso struct {
	mu          sync.Mutex
	classificou int
	respondeu   []string
	inesperados []string
}

func (o *ollamaFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	switch r.URL.Path {
	case "/api/tags":
		_, _ = io.WriteString(w, `{"models":[{"name":"falso"}]}`)
	case "/api/generate":
		var req struct {
			Prompt string `json:"prompt"`
			Stream bool   `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		enc := json.NewEncoder(w)
		if !req.Stream {
			o.classificou++
			_ = enc.Encode(map[string]string{"response": `{"intent":"direct","params":{},"direct":true}`})
			return
		}
		o.respondeu = append(o.respondeu, req.Prompt)
		for _, t := range []string{"resposta ", "para ", req.Prompt} {
			_ = enc.Encode(map[string]interface{}{"response": t, "done": false})
		}
	default:
		o.inesperados = append(o.inesperados, r.Method+" "+r.URL.Path)
		http.NotFound(w, r)
	}
}

// rodarPlugin roda o binário de teste como o plugin, sem terminal em nenhuma ponta, com
// a home isolada e o Ollama falso como único provider; falha se passar do prazo.
func rodarPlugin(t *testing.T, ollama string, args []string, entrada string, extras ...string) (saida, erros string, codigo int) {
	t.Helper()
	pedido, err := json.Marshal(map[string]interface{}{"hook": "command", "args": args})
	if err != nil {
		t.Fatal(err)
	}
	return rodarProcesso(t, ollama, entrada, append([]string{"YBY_PLUGIN_REQUEST=" + string(pedido)}, extras...)...)
}

// rodarProcesso é rodarPlugin com as variáveis de ambiente extras dadas, sem o pedido
// implícito: sem YBY_PLUGIN_REQUEST, o plugin roda no modo dev.
func rodarProcesso(t *testing.T, ollama, entrada string, extras ...string) (saida, erros string, codigo int) {
	t.Helper()
	home := t.TempDir()
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Dir = t.TempDir()
	var env []string
	for _, kv := range os.Environ() {
		chave := strings.ToUpper(strings.SplitN(kv, "=", 2)[0])
		if strings.HasPrefix(chave, "YBY_") || strings.HasPrefix(chave, "OLLAMA") || strings.Contains(chave, "API_KEY") ||
			chave == "HOME" || chave == "USERPROFILE" || chave == "NO_COLOR" || strings.HasPrefix(chave, "CLICOLOR") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env,
		envProcessoTeste+"=1",
		"YBY_AI_PROVIDER=ollama",
		"OLLAMA_HOST="+ollama,
		"HOME="+home,
		"USERPROFILE="+home,
	)
	cmd.Env = append(cmd.Env, extras...)
	if entrada != "" {
		cmd.Stdin = strings.NewReader(entrada)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("o plugin travou: passou de 60 s\nstdout: %s\nstderr: %s", out.String(), errb.String())
	}
	var saidaErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &saidaErr):
		codigo = saidaErr.ExitCode()
	default:
		t.Fatalf("o plugin não rodou: %v", err)
	}
	return out.String(), errb.String(), codigo
}

func iniciarOllamaFalso(t *testing.T) (*ollamaFalso, string) {
	t.Helper()
	o := &ollamaFalso{}
	srv := httptest.NewServer(o)
	t.Cleanup(srv.Close)
	return o, srv.URL
}

func TestProcesso_BatchSemTerminal(t *testing.T) {
	o, url := iniciarOllamaFalso(t)
	saida, erros, codigo := rodarPlugin(t, url, nil, "quantos pods?\nqual o namespace?\n")
	if codigo != 0 {
		t.Fatalf("código %d\nstderr: %s", codigo, erros)
	}
	if want := "resposta para quantos pods?\n" + separadorLote + "resposta para qual o namespace?\n"; saida != want {
		t.Errorf("stdout = %q, esperado %q", saida, want)
	}
	semControle(t, "stdout", saida)
	semControle(t, "stderr", erros)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.classificou != 2 || len(o.respondeu) != 2 || len(o.inesperados) != 0 {
		t.Errorf("chamadas ao Ollama falso: classificou=%d respondeu=%v inesperados=%v", o.classificou, o.respondeu, o.inesperados)
	}
}

func TestProcesso_OneShotSemTerminal(t *testing.T) {
	o, url := iniciarOllamaFalso(t)
	saida, erros, codigo := rodarPlugin(t, url, []string{"-p", "lista", "os", "pods"}, "")
	if codigo != 0 {
		t.Fatalf("código %d\nstderr: %s", codigo, erros)
	}
	if saida != "resposta para lista os pods\n" {
		t.Errorf("stdout = %q", saida)
	}
	semControle(t, "stdout", saida)
	semControle(t, "stderr", erros)
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.respondeu) != 1 || len(o.inesperados) != 0 {
		t.Errorf("chamadas ao Ollama falso: respondeu=%v inesperados=%v", o.respondeu, o.inesperados)
	}
}

func TestProcesso_StdinVazioNaoTrava(t *testing.T) {
	o, url := iniciarOllamaFalso(t)
	saida, erros, codigo := rodarPlugin(t, url, nil, "")
	if codigo != 1 {
		t.Fatalf("código %d, esperado 1\nstderr: %s", codigo, erros)
	}
	if saida != "" {
		t.Errorf("stdout deveria ficar vazio: %q", saida)
	}
	if !strings.Contains(erros, "✕ nenhuma pergunta no stdin") || !strings.Contains(erros, "ERR_VALIDATION") {
		t.Errorf("stderr inesperado:\n%s", erros)
	}
	semControle(t, "stderr", erros)
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.respondeu) != 0 {
		t.Errorf("sem perguntas, a IA não deveria responder: %v", o.respondeu)
	}
}

func TestProcesso_PromptSemPergunta(t *testing.T) {
	o, url := iniciarOllamaFalso(t)
	saida, erros, codigo := rodarPlugin(t, url, []string{"-p"}, "")
	if codigo != 1 {
		t.Fatalf("código %d, esperado 1\nstderr: %s", codigo, erros)
	}
	if saida != "" {
		t.Errorf("stdout deveria ficar vazio: %q", saida)
	}
	if !strings.HasPrefix(erros, "\n✕ falta a pergunta do -p") || !strings.Contains(erros, "ERR_VALIDATION") ||
		!strings.Contains(erros, `yby bard -p "pergunta"`) {
		t.Errorf("stderr inesperado:\n%q", erros)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.classificou != 0 || len(o.respondeu) != 0 {
		t.Errorf("sem pergunta, a IA não deveria ser chamada: classificou=%d respondeu=%v", o.classificou, o.respondeu)
	}
}

// Sem YBY_PLUGIN_REQUEST (modo dev), o stdin que não é um pedido JSON segue inteiro
// para o batch.
func TestProcesso_ModoDevBatch(t *testing.T) {
	_, url := iniciarOllamaFalso(t)
	saida, erros, codigo := rodarProcesso(t, url, "quantos pods?\nqual o namespace?\n")
	if codigo != 0 {
		t.Fatalf("código %d\nstderr: %s", codigo, erros)
	}
	if want := "resposta para quantos pods?\n" + separadorLote + "resposta para qual o namespace?\n"; saida != want {
		t.Errorf("stdout = %q, esperado %q", saida, want)
	}
}

// O erro de antes da detecção do provider já segue o stderr: num pipe, sai sem códigos
// de cor mesmo com a cor forçada pelo ambiente (como se o stdout fosse um terminal).
func TestProcesso_SemProviderSemCorNoPipe(t *testing.T) {
	pedido := `{"hook":"command","args":["-p","oi"]}`
	saida, erros, codigo := rodarProcesso(t, "http://127.0.0.1:1", "", "YBY_PLUGIN_REQUEST="+pedido, "CLICOLOR_FORCE=1")
	if codigo != 1 {
		t.Fatalf("código %d, esperado 1\nstderr: %s", codigo, erros)
	}
	if saida != "" {
		t.Errorf("stdout deveria ficar vazio: %q", saida)
	}
	if !strings.Contains(erros, "✕ nenhum provider de IA disponível") {
		t.Errorf("stderr inesperado:\n%s", erros)
	}
	semControle(t, "stderr", erros)
}

func TestNovoDireto_ClusterENamespaceDaSessao(t *testing.T) {
	isolarModoVisual(t, "")
	antProvider, antCluster := obterProvider, enriquecerCluster
	obterProvider = func(context.Context) ai.Provider { return &iaFalsa{} }
	enriquecerCluster = func(_ context.Context, contexto, namespace string) *ClusterContext {
		return &ClusterContext{Cluster: "atual", Namespace: "default"}
	}
	t.Cleanup(func() { obterProvider, enriquecerCluster = antProvider, antCluster })

	d, err := novoDireto(context.Background(), nil, opcoesBard{contexto: "prod", namespace: "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if d.fluxo.Cluster != "prod" || d.fluxo.Namespace != "payments" {
		t.Errorf("fluxo com %q/%q, esperava prod/payments", d.fluxo.Cluster, d.fluxo.Namespace)
	}

	d, err = novoDireto(context.Background(), nil, opcoesBard{})
	if err != nil {
		t.Fatal(err)
	}
	if d.fluxo.Cluster != "atual" || d.fluxo.Namespace != "default" {
		t.Errorf("sem flags, fluxo com %q/%q, esperava atual/default", d.fluxo.Cluster, d.fluxo.Namespace)
	}
}
