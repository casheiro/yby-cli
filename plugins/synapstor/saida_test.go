package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/decay"
	"github.com/charmbracelet/lipgloss"
	"github.com/gofrs/flock"
	"github.com/muesli/termenv"
)

// atualizarGolden regrava os arquivos golden: go test ./ -run <Teste> -update (em plugins/synapstor)
var atualizarGolden = flag.Bool("update", false, "regrava os arquivos golden em testdata/")

func TestMain(m *testing.M) {
	// A home dos testes é temporária: o ~/.yby/config.yaml de verdade não muda o visual
	// nem o log.
	dir, err := os.MkdirTemp("", "synapstor-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	for _, k := range []string{"YBY_THEME", "YBY_ASCII", "NO_COLOR", "YBY_LOG_LEVEL", "YBY_LOG_FORMAT"} {
		os.Unsetenv(k)
	}
	codigo := m.Run()
	os.RemoveAll(dir)
	os.Exit(codigo)
}

// conferirGolden compara obtido com testdata/<nome>; com -update, regrava o arquivo.
func conferirGolden(t *testing.T, nome, obtido string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome)
	if *atualizarGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(obtido), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("golden %s ausente (rode com -update): %v", caminho, err)
	}
	if string(esperado) != obtido {
		t.Errorf("saída diferente do golden %s\n--- esperado ---\n%s\n--- obtido ---\n%s", caminho, esperado, obtido)
	}
}

// quandoGolden é o instante fixo das etapas nas goldens.
var quandoGolden = time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)

// relogioDePasso devolve um relógio que avança 1,5 s a cada leitura a partir de
// quandoGolden: a coluna de tempo das etapas entra nas goldens sempre igual.
func relogioDePasso() func() time.Time {
	var mu sync.Mutex
	agora := quandoGolden
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		agora = agora.Add(1500 * time.Millisecond)
		return agora
	}
}

// usarVisual fixa o modo visual e o perfil de cor, e os restaura no fim do teste.
func usarVisual(t *testing.T, o ui.Opcoes, perfil termenv.Profile) {
	t.Helper()
	perfilAnterior, opcoesAnteriores := lipgloss.ColorProfile(), ui.Atual()
	escuroAnterior := lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		ui.Configurar(opcoesAnteriores)
		lipgloss.SetColorProfile(perfilAnterior)
		lipgloss.SetHasDarkBackground(escuroAnterior)
	})
	ui.Configurar(o)
	lipgloss.SetColorProfile(perfil)
}

// provedorFalso gera embeddings por saco de palavras (cada palavra cai numa dimensão pelo
// hash), o que deixa a busca determinística, e responde sempre o mesmo texto no
// Completion. Nunca há IA de verdade nos testes.
type provedorFalso struct {
	ai.Provider
	resposta string
	err      error
	chamadas int
	// aviso, se não vazio, é logado com slog.Warn a cada EmbedDocuments.
	aviso string
	// nome, se não vazio, troca o nome "falso": outro nome são outros embeddings.
	nome string
}

func (p *provedorFalso) Name() string {
	if p.nome != "" {
		return p.nome
	}
	return "falso"
}

func (p *provedorFalso) Completion(context.Context, string, string) (string, error) {
	p.chamadas++
	return p.resposta, p.err
}

func (p *provedorFalso) EmbedDocuments(_ context.Context, textos []string) ([][]float32, error) {
	if p.aviso != "" {
		slog.Warn(p.aviso)
	}
	if p.err != nil {
		return nil, p.err
	}
	vetores := make([][]float32, len(textos))
	for i, texto := range textos {
		v := make([]float32, 128)
		for _, palavra := range strings.Fields(strings.ToLower(texto)) {
			h := fnv.New32a()
			_, _ = h.Write([]byte(strings.Trim(palavra, "#.,:*`")))
			v[h.Sum32()%128]++
		}
		var soma float64
		for _, x := range v {
			soma += float64(x * x)
		}
		norma := float32(math.Sqrt(soma))
		if norma == 0 {
			norma = 1
		}
		for j := range v {
			v[j] /= norma
		}
		vetores[i] = v
	}
	return vetores, nil
}

// usarDependencias troca a IA e a análise do git por falsos, fixa o relógio, a largura
// (64) e a detecção de terminal (fora dele), e restaura tudo no fim.
func usarDependencias(t *testing.T, prov *provedorFalso) {
	t.Helper()
	p, e, d, r, te, l := provedorDeIA, provedorDeEmbeddings, analisarDecay, relogio, terminalEm, larguraSaida
	t.Cleanup(func() {
		provedorDeIA, provedorDeEmbeddings, analisarDecay, relogio, terminalEm, larguraSaida = p, e, d, r, te, l
	})
	devolver := func(context.Context) ai.Provider {
		if prov == nil {
			return nil
		}
		return prov
	}
	provedorDeIA, provedorDeEmbeddings = devolver, devolver
	relogio = relogioDePasso()
	terminalEm = func(io.Writer) bool { return false }
	larguraSaida = func(io.Writer) int { return larguraPadrao }
}

// capturarLog liga o slog do plugin como o main faz (configurarLog, nível info da home
// isolada) e manda o log para o buffer devolvido em vez do stderr do processo de teste;
// restaura tudo no fim. Assim um slog.Warn no caminho feliz aparece no teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	anterior := slog.Default()
	var log bytes.Buffer
	configurarLog()
	restaurar := logger.Desviar(&log)
	t.Cleanup(func() {
		restaurar()
		slog.SetDefault(anterior)
	})
	return &log
}

// rodar chama o subcomando em raiz como o core o chamaria e devolve stdout e stderr, com
// as barras do Windows trocadas para a golden ser a mesma em qualquer sistema.
func rodar(t *testing.T, raiz string, args ...string) (string, string, error) {
	t.Helper()
	var saida, erros bytes.Buffer
	err := comando(context.Background(), raiz, args, &saida, &erros)
	return strings.ReplaceAll(saida.String(), `\`, "/"), erros.String(), err
}

// ukisDeExemplo são os UKIs das goldens: um completo, um mediano e um rascunho.
var ukisDeExemplo = map[string]string{
	"UKI-1761390000-deploy-argo.md": "# Deploy com Argo CD\n\n**ID:** UKI-1\n**Tipo:** procedimento\n**Status:** ativo\n\n" +
		"## Contexto\n\nO payments sobe pelo Argo CD com sync automático a partir do repositório de GitOps.\n\n" +
		"## Passos\n\n```bash\nargocd app sync payments\n```\n\n## Rollback\n\nUse o histórico do Argo CD. " +
		"Veja [a documentação](https://argo-cd.readthedocs.io).\n\n## Observações\n\n" + strings.Repeat("deploy argo cd sync payments ", 30),
	"UKI-1761390100-helm-charts.md": "# Charts do Helm\n\n## Contexto\n\nOs charts ficam em charts/ e seguem a versão do app.\n\n" +
		"## Valores\n\nCada ambiente tem um values próprio ([exemplo](charts/api/values.yaml)). " + strings.Repeat("helm chart values ambiente ", 25),
	"UKI-1761390200-rascunho.md": "# Rascunho de redis\n\nanotar o redis depois",
}

// projetoDeExemplo cria um projeto temporário com os UKIs de exemplo.
func projetoDeExemplo(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	dir := filepath.Join(raiz, ".synapstor", ".uki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for nome, conteudo := range ukisDeExemplo {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

func TestAjuda_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	saida, _, err := rodar(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "ajuda.golden", saida)

	// a ajuda de cada subcomando sai pelo ui.EscreverAjuda, com o título dele
	for sub := range ajudas {
		saida, _, err := rodar(t, t.TempDir(), sub, "--help")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(saida, "YBY  SYNAPSTOR "+strings.ToUpper(sub)) || !strings.Contains(saida, "USO") {
			t.Errorf("ajuda de %s fora do formato:\n%s", sub, saida)
		}
	}
}

func TestComandoDesconhecido_ErroComHint(t *testing.T) {
	_, _, err := rodar(t, t.TempDir(), "inventado")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.GetHint(), "--help") {
		t.Fatalf("esperado ERR_VALIDATION com hint, obtido %v", err)
	}

	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	var b bytes.Buffer
	if codigo := sairComErro(&b, err); codigo != 1 {
		t.Errorf("código de saída %d, esperado 1", codigo)
	}
	if !strings.Contains(b.String(), "ERR_VALIDATION") || !strings.Contains(b.String(), "faça") {
		t.Errorf("erro fora do formato do ui.WriteError:\n%s", b.String())
	}
}

// TestSemProvider_ErroComHint: só capture e study pedem um provider de IA; index e search
// sempre têm embeddings (sem Ollama nem API, os locais).
func TestSemProvider_ErroComHint(t *testing.T) {
	usarDependencias(t, nil)
	for _, args := range [][]string{{"capture", "texto"}, {"study", "deploy"}} {
		_, _, err := rodar(t, t.TempDir(), args...)
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeConfig || !strings.Contains(ye.GetHint(), "ollama serve") {
			t.Errorf("%v: esperado ERR_CONFIG_INVALID com hint, obtido %v", args, err)
		}
	}
}

// campoEmbeddings é o campo "embeddings  falso" do resumo, com as cores no meio.
var campoEmbeddings = regexp.MustCompile(`embeddings +(\x1b\[[0-9;]*m)*falso`)

func TestIndexESearch_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	log := capturarLog(t)

	saida, erros, err := rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	if erros != "" || log.Len() > 0 {
		t.Errorf("nada no stderr nem no log fora do terminal, obtido %q e %q", erros, log.String())
	}
	conferirGolden(t, "index.golden", saida)
	if !campoEmbeddings.MatchString(saida) {
		t.Errorf("o resumo do index tem de dizer de onde vêm os embeddings:\n%s", saida)
	}

	// o manifest mora dentro da pasta do índice
	for _, p := range []string{filepath.Join(".index", "manifest.json"), ".index"} {
		if _, err := os.Stat(filepath.Join(raiz, ".synapstor", p)); err != nil {
			t.Errorf("%s não foi gravado: %v", p, err)
		}
	}

	saida, _, err = rodar(t, raiz, "search", "deploy", "argo", "cd", "helm", "chart", "values")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "search.golden", saida)
	if !campoEmbeddings.MatchString(saida) {
		t.Errorf("o resumo do search tem de dizer de onde vêm os embeddings:\n%s", saida)
	}

	// sem nada mudado e com um UKI apagado: 0 indexados, 1 removido
	if err := os.Remove(filepath.Join(raiz, ".synapstor", ".uki", "UKI-1761390200-rascunho.md")); err != nil {
		t.Fatal(err)
	}
	saida, _, err = rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "index-removido.golden", saida)
}

// provedorQueBaixa é o provedorFalso com um modelo a baixar no primeiro uso, como o local.
type provedorQueBaixa struct {
	*provedorFalso
	baixado bool
	err     error
}

func (p *provedorQueBaixa) TamanhoABaixar() string {
	if p.baixado {
		return ""
	}
	return "~120 MB"
}

func (p *provedorQueBaixa) BaixarModelo(context.Context) error {
	if p.err != nil {
		return p.err
	}
	p.baixado = true
	return nil
}

// TestIndexESearch_EtapaDeBaixarOModelo: o download do modelo local tinha só um slog.Info,
// dentro da etapa "gerar embeddings". Agora tem etapa própria, antes de gerar os embeddings
// e antes de buscar, e só no primeiro uso.
func TestIndexESearch_EtapaDeBaixarOModelo(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	prov := &provedorQueBaixa{provedorFalso: &provedorFalso{}}
	usarDependencias(t, prov.provedorFalso)
	provedorDeEmbeddings = func(context.Context) ai.Provider { return prov }
	raiz := projetoDeExemplo(t)

	saida, _, err := rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	baixar := strings.Index(saida, "baixar o modelo de embeddings (~120 MB)")
	if baixar < 0 || baixar > strings.Index(saida, "gerar embeddings") {
		t.Errorf("a etapa do download tem de vir antes de gerar os embeddings:\n%s", saida)
	}

	// Já baixado: a busca não abre a etapa.
	saida, _, err = rodar(t, raiz, "search", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saida, "baixar o modelo") {
		t.Errorf("com o modelo no disco não há etapa de download:\n%s", saida)
	}

	// Sem o modelo, a busca baixa antes de buscar; a falha do download para a busca.
	prov.baixado = false
	saida, _, err = rodar(t, raiz, "search", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	baixar = strings.Index(saida, "baixar o modelo de embeddings")
	if baixar < 0 || baixar > strings.Index(saida, "buscar no índice") {
		t.Errorf("a etapa do download tem de vir antes de buscar:\n%s", saida)
	}
	prov.baixado, prov.err = false, errors.New("status HTTP 503")
	_, _, err = rodar(t, raiz, "search", "deploy")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeUnreachable || !strings.Contains(ye.GetHint(), "HF_ENDPOINT") {
		t.Errorf("esperado erro de rede com hint do HF_ENDPOINT, obtido %v", err)
	}
}

func TestSearch_IndiceVazio(t *testing.T) {
	usarDependencias(t, &provedorFalso{})
	raiz := t.TempDir()
	_, _, err := rodar(t, raiz, "search", "deploy")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.GetHint(), "yby synapstor index") {
		t.Fatalf("esperado erro de índice vazio com hint, obtido %v", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, ".synapstor")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a busca sem índice não deve criar .synapstor")
	}
}

func TestSearchEIndex_TrocarDeEmbeddings(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	if _, _, err := rodar(t, raiz, "index"); err != nil {
		t.Fatal(err)
	}

	usarDependencias(t, &provedorFalso{nome: "outro"})
	_, _, err := rodar(t, raiz, "search", "deploy")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || !strings.Contains(ye.GetHint(), "yby synapstor index --full") ||
		!strings.Contains(ye.Message, "falso (128 dimensões)") || !strings.Contains(ye.Message, "outro (128 dimensões)") {
		t.Fatalf("a busca com outros embeddings diz com que o índice foi feito e como refazer, obtido %v", err)
	}

	saida, _, err := rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "apagar o índice antigo") || !strings.Contains(saida, "refeito do zero") || !strings.Contains(saida, "3 indexados") {
		t.Errorf("o index com outros embeddings refaz tudo e avisa:\n%s", saida)
	}
	if _, _, err := rodar(t, raiz, "search", "deploy"); err != nil {
		t.Errorf("depois de refeito, a busca funciona: %v", err)
	}
}

func TestIndex_SemUKIs(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{})
	saida, _, err := rodar(t, t.TempDir(), "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "nenhum UKI em .synapstor/.uki nem em docs/wiki") || !strings.Contains(saida, "PRÓXIMO PASSO") {
		t.Errorf("estado vazio fora do formato:\n%s", saida)
	}
}

func TestIndex_OutroIndexEmAndamento(t *testing.T) {
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	pasta := filepath.Join(raiz, ".synapstor", ".index")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	outro := flock.New(filepath.Join(pasta, "index.lock"))
	if ok, err := outro.TryLock(); err != nil || !ok {
		t.Fatalf("preparo: não deu para travar (%v)", err)
	}
	t.Cleanup(func() { _ = outro.Unlock() })

	_, _, err := rodar(t, raiz, "index")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || !strings.Contains(ye.Message, "outro yby synapstor index") || !strings.Contains(ye.GetHint(), "espere") {
		t.Fatalf("esperado o erro do index em andamento, com dica, obtido %v", err)
	}
}

func TestIndex_FalhaDoProvider(t *testing.T) {
	usarDependencias(t, &provedorFalso{err: errors.New("ollama fora do ar")})
	_, _, err := rodar(t, projetoDeExemplo(t), "index", "--full")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeExec || ye.Context["provider"] != "falso" {
		t.Fatalf("esperado ERR_EXEC_FAILED com o provider, obtido %v", err)
	}
}

func TestQuality_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	usarDependencias(t, nil)
	saida, _, err := rodar(t, projetoDeExemplo(t), "quality")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "quality.golden", saida)
}

func TestQuality_GoldenASCIISemCor(t *testing.T) {
	usarVisual(t, ui.Opcoes{}, termenv.TrueColor)
	configurarModoVisual(func(k string) string {
		return map[string]string{"YBY_ASCII": "1", "NO_COLOR": "1", "YBY_THEME": "dark"}[k]
	})
	if o := ui.Atual(); !o.ASCII || !o.SemCor {
		t.Fatalf("YBY_ASCII e NO_COLOR valem no plugin: %+v", o)
	}
	usarDependencias(t, nil)
	saida, _, err := rodar(t, projetoDeExemplo(t), "quality")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(saida, "◆✓✕▲○") || strings.Contains(saida, "\x1b[38;") {
		t.Errorf("YBY_ASCII=1 e NO_COLOR sem glifos Unicode nem cor:\n%s", saida)
	}
	conferirGolden(t, "quality-ascii.golden", saida)
}

func TestQuality_SemUKIs(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	saida, _, err := rodar(t, t.TempDir(), "quality")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "nenhum UKI em .synapstor/.uki") {
		t.Errorf("estado vazio fora do formato:\n%s", saida)
	}
}

func TestDecay_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	usarDependencias(t, nil)
	analisarDecay = func(string, string) ([]decay.DecayInfo, error) {
		return []decay.DecayInfo{
			{Title: "Deploy com Argo CD", DaysSinceActivity: 3},
			{Title: "Charts do Helm", DaysSinceActivity: 120, IsStale: true},
			{Path: "UKI-1761390200-rascunho.md", DaysSinceActivity: 1},
			{Title: "Rollback do payments", DaysSinceActivity: 5, IsStale: true, CodeCommits: 3},
		}, nil
	}
	saida, _, err := rodar(t, projetoDeExemplo(t), "decay")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "decay.golden", saida)
}

// respostaDoUKI é a resposta falsa da IA no capture e no study.
const respostaDoUKI = "```json\n{\"title\": \"Rollback do payments\", \"id\": \"deploy.argo.rollback\", \"tipo\": \"padrao\", " +
	"\"tags\": [\"argo-cd\", \"rollback\"], \"filename\": \"UKI-1761390300-rollback.md\", " +
	"\"content\": \"# Rollback do payments\\n\\nUse o histórico do Argo CD.\"}\n```"

func TestCapture_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	prov := &provedorFalso{resposta: respostaDoUKI}
	usarDependencias(t, prov)
	raiz := t.TempDir()

	saida, _, err := rodar(t, raiz, "capture", "o", "rollback", "usa", "o", "histórico", "do", "Argo", "CD")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "capture.golden", saida)

	// o UKI gravado é o content da IA com o frontmatter da especificação, montado pelo
	// código (status rascunho e as datas do dia)
	uki, err := os.ReadFile(filepath.Join(raiz, ".synapstor", ".uki", "UKI-1761390300-rollback.md"))
	if err != nil {
		t.Fatalf("UKI não gravado: %v", err)
	}
	hoje := time.Now().Format(time.DateOnly)
	esperado := "---\nid: deploy.argo.rollback\ntipo: padrao\nstatus: rascunho\ncriado_em: " + hoje +
		"\natualizado_em: " + hoje + "\ntags: [argo-cd, rollback]\n---\n\n# Rollback do payments\n\nUse o histórico do Argo CD."
	if string(uki) != esperado {
		t.Errorf("UKI fora do formato da especificação:\n%s", uki)
	}
	if prov.chamadas != 1 {
		t.Errorf("esperada 1 chamada à IA, houve %d", prov.chamadas)
	}
}

func TestCapture_SemTexto(t *testing.T) {
	usarDependencias(t, &provedorFalso{})
	_, _, err := rodar(t, t.TempDir(), "capture")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation {
		t.Fatalf("esperado ERR_VALIDATION, obtido %v", err)
	}
}

func TestStudy_SemArquivosNaoEErro(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	prov := &provedorFalso{resposta: respostaDoUKI}
	usarDependencias(t, prov)
	saida, _, err := rodar(t, t.TempDir(), "study", "assunto-que-não-existe")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "nenhum arquivo do projeto fala de") || prov.chamadas != 0 {
		t.Errorf("sem arquivos, avisa e não chama a IA (%d chamadas):\n%s", prov.chamadas, saida)
	}
}

func TestExport_Golden(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.TrueColor)
	usarDependencias(t, nil)
	raiz := projetoDeExemplo(t)
	saida, _, err := rodar(t, raiz, "export", "--format=obsidian")
	if err != nil {
		t.Fatal(err)
	}
	conferirGolden(t, "export.golden", saida)
	arquivos, err := os.ReadDir(filepath.Join(raiz, ".synapstor", "export", "obsidian"))
	if err != nil || len(arquivos) == 0 {
		t.Errorf("exportação não gravou nada: %v", err)
	}
}

func TestExport_FormatoDesconhecido(t *testing.T) {
	usarDependencias(t, nil)
	for _, args := range [][]string{{"export", "--format", "pdf"}, {"export", "--format"}, {"export", "--formato", "x"}} {
		_, _, err := rodar(t, projetoDeExemplo(t), args...)
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeValidation || ye.GetHint() == "" {
			t.Errorf("%v: esperado ERR_VALIDATION com hint, obtido %v", args, err)
		}
	}
}

func TestContextHook_JSONNaoMuda(t *testing.T) {
	var b bytes.Buffer
	t.Chdir(t.TempDir())
	if err := handlePluginRequest(plugin.PluginRequest{Hook: "context"}, &b, io.Discard); err != nil {
		t.Fatal(err)
	}
	esperado := `{"data":{"synapstor_indexed_files":0,"synapstor_last_indexed":"","synapstor_status":"not_indexed"}}` + "\n"
	if b.String() != esperado {
		t.Errorf("JSON do context mudou:\n%s", b.String())
	}
}

func TestSearch_GoldenEstreito(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	if _, _, err := rodar(t, raiz, "index"); err != nil {
		t.Fatal(err)
	}
	for _, larg := range []int{40, 20} {
		larguraSaida = func(io.Writer) int { return larg }
		saida, _, err := rodar(t, raiz, "search", "deploy", "argo", "cd", "helm", "chart", "values")
		if err != nil {
			t.Fatal(err)
		}
		if larg == 40 {
			conferirGolden(t, "search-estreito.golden", saida)
		}
		// da seção RESULTADOS até o RESUMO, nenhuma linha passa da largura
		resultados := saida[strings.Index(saida, "RESULTADOS"):strings.Index(saida, "RESUMO")]
		for _, l := range strings.Split(resultados, "\n") {
			if w := lipgloss.Width(l); w > larg {
				t.Errorf("a %d colunas, linha com %d: %q", larg, w, l)
			}
		}
	}
}

func TestIndex_NoTerminalRedesenhaEDesviaOLog(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{aviso: "o modelo de embeddings está lento"})
	log := capturarLog(t)
	terminalEm = func(io.Writer) bool { return true }

	saida, erros, err := rodar(t, projetoDeExemplo(t), "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "\x1b[1A\r\x1b[J") {
		t.Errorf("no terminal, a etapa atual é redesenhada no lugar:\n%q", saida)
	}
	for _, etapa := range []string{"✓ procurar documentos", "✓ gerar embeddings"} {
		if n := strings.Count(saida, etapa); n != 1 {
			t.Errorf("%q sai uma vez só, de vez, obtido %d:\n%q", etapa, n, saida)
		}
	}
	// a barra da etapa de embeddings avança com o AoAvancar do VectorStore
	for _, pct := range []string{"25%", "50%", "100%"} {
		if !strings.Contains(saida, pct) {
			t.Errorf("a barra de gerar embeddings não passou por %s:\n%q", pct, saida)
		}
	}
	// o log passa pelo escritor das etapas e vai para o stderr, fora do trecho redesenhado
	if !strings.Contains(erros, "WARN") || !strings.Contains(erros, "o modelo de embeddings está lento") {
		t.Errorf("o aviso do log vai para o stderr pelo slog do yby:\n%q", erros)
	}
	if strings.Contains(saida, "lento") || log.Len() > 0 {
		t.Errorf("o log não cai no meio das etapas nem fora do escritor delas:\n%q\n%q", saida, log.String())
	}
}

func TestIndex_TodosApagadosLimpaOIndice(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	if _, _, err := rodar(t, raiz, "index"); err != nil {
		t.Fatal(err)
	}
	for nome := range ukisDeExemplo {
		if err := os.Remove(filepath.Join(raiz, ".synapstor", ".uki", nome)); err != nil {
			t.Fatal(err)
		}
	}
	saida, _, err := rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "tirar os removidos do índice") || !strings.Contains(saida, "3 removidos") {
		t.Errorf("os 3 UKIs apagados saem do índice:\n%s", saida)
	}
	manifest, err := os.ReadFile(filepath.Join(raiz, ".synapstor", ".index", "manifest.json"))
	if err != nil || strings.Contains(string(manifest), "UKI-") {
		t.Errorf("o manifest fica vazio, obtido %s (%v)", manifest, err)
	}
	_, _, err = rodar(t, raiz, "search", "deploy", "argo", "cd")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || !strings.Contains(ye.GetHint(), "yby synapstor index") {
		t.Errorf("a busca não acha mais os UKIs apagados, obtido %v", err)
	}
}

func TestIndex_FalhaAoTirarRemovidosAvisaEMantemNoManifest(t *testing.T) {
	usarVisual(t, ui.Opcoes{Tema: ui.TemaEscuro, SemCor: true}, termenv.Ascii)
	usarDependencias(t, &provedorFalso{})
	raiz := projetoDeExemplo(t)
	if _, _, err := rodar(t, raiz, "index"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(raiz, ".synapstor", ".uki", "UKI-1761390200-rascunho.md")); err != nil {
		t.Fatal(err)
	}
	// o índice ganha uma coleção quebrada (sem metadados): o vector store não abre e o
	// removido não sai; o manifest, dentro da pasta, continua lá
	indice := filepath.Join(raiz, ".synapstor", ".index")
	entradas, err := os.ReadDir(indice)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		if e.Name() != "manifest.json" {
			if err := os.RemoveAll(filepath.Join(indice, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(indice, "quebrada"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(indice, "quebrada", "00000000.gob"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	saida, _, err := rodar(t, raiz, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saida, "não deu para tirar do índice 1 arquivo apagado") || !strings.Contains(saida, "0 removidos") {
		t.Errorf("a falha ao tirar o removido aparece como aviso e não conta como removido:\n%s", saida)
	}
	manifest, err := os.ReadFile(filepath.Join(raiz, ".synapstor", ".index", "manifest.json"))
	if err != nil || !strings.Contains(string(manifest), "UKI-1761390200-rascunho.md") {
		t.Errorf("o removido que ficou no índice continua no manifest, para a próxima tentativa: %s (%v)", manifest, err)
	}
}
