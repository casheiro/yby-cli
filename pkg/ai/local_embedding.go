package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/backends"
	"github.com/knights-analytics/hugot/pipelines"
)

// LocalEmbeddingProvider gera embeddings localmente com o paraphrase-multilingual-MiniLM-L12-v2
// em int8, que entende português (o all-MiniLM-L6-v2 de antes só entendia inglês).
// Não requer API, não tem rate limit, funciona offline após o download inicial.
type LocalEmbeddingProvider struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
	ready    bool
	// contarTokens conta os tokens (com os especiais) que o pipeline gera para um texto,
	// com o mesmo tokenizador; limiteTokens é quantos o modelo aceita.
	contarTokens func(string) int
	limiteTokens int
}

// NewLocalEmbeddingProvider cria um provider de embeddings local.
func NewLocalEmbeddingProvider() *LocalEmbeddingProvider {
	return &LocalEmbeddingProvider{}
}

// Name retorna o identificador do provider.
func (p *LocalEmbeddingProvider) Name() string {
	return "Local Embedding (paraphrase-multilingual-MiniLM-L12-v2)"
}

// IdentidadeDosEmbeddings diz que os embeddings são locais e com que modelo.
func (p *LocalEmbeddingProvider) IdentidadeDosEmbeddings(context.Context) (IdentidadeDoIndice, error) {
	return IdentidadeDoIndice{Provider: "local", Modelo: localEmbeddingModelo}, nil
}

// IsAvailable sempre retorna true — embeddings locais estão sempre disponíveis.
func (p *LocalEmbeddingProvider) IsAvailable(_ context.Context) bool {
	return true
}

const (
	localEmbeddingModelID = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
	// localEmbeddingRevisao fixa a revisão do repositório: arquivos novos no main mudariam
	// os vetores sem mudar a identidade do índice.
	localEmbeddingRevisao = "e8f8c211226b894fcb81acc59f3b34ba3efd5f42"
	// localEmbeddingModelo é o modelo na identidade do índice: o int8 dá vetores um pouco
	// diferentes dos do fp32, então a quantização entra no nome.
	localEmbeddingModelo = "paraphrase-multilingual-MiniLM-L12-v2-int8"
	// localEmbeddingModelDir é a pasta do modelo em ~/.yby/models.
	localEmbeddingModelDir = "sentence-transformers_paraphrase-multilingual-MiniLM-L12-v2-int8"
	// localEmbeddingTamanho é o tamanho aproximado do download (ONNX int8 e tokenizador).
	localEmbeddingTamanho = "~120 MB"
)

// arquivoDoModelo é um arquivo do repositório do modelo e o nome com que ele fica na
// pasta do modelo.
type arquivoDoModelo struct {
	origem, destino string
}

// localEmbeddingFiles são os arquivos do repositório HuggingFace lidos pelo pipeline de
// feature extraction do hugot. Todos ficam na raiz da pasta do modelo; tokenizer.json vem
// por último e marca download completo. O modelo não tem vocab.txt: o tokenizador
// (Unigram) está no tokenizer.json.
var localEmbeddingFiles = []arquivoDoModelo{
	{"config.json", "config.json"},
	{"special_tokens_map.json", "special_tokens_map.json"},
	{"tokenizer_config.json", "tokenizer_config.json"},
	// O ONNX int8 (~118 MB) em vez do fp32 (~470 MB). Apesar do nome, é o mesmo arquivo
	// das variantes arm64 e avx512_vnni; o backend Go puro não depende da CPU. Fica como
	// model.onnx, o único .onnx da pasta, que é o que o hugot carrega.
	{"onnx/model_qint8_avx512.onnx", "model.onnx"},
	{"tokenizer.json", "tokenizer.json"},
}

// huggingFaceEndpoint retorna a URL base do HuggingFace Hub, respeitando
// HF_ENDPOINT (mirrors corporativos).
func huggingFaceEndpoint() string {
	if endpoint := os.Getenv("HF_ENDPOINT"); endpoint != "" {
		return strings.TrimSuffix(endpoint, "/")
	}
	return "https://huggingface.co"
}

// downloadLocalEmbeddingModel baixa os arquivos do modelo direto para dest.
// Não usa o cache do HuggingFace (hugot.DownloadModel), que cria symlinks e
// falha no Windows sem privilégio de administrador ou Modo de Desenvolvedor.
func downloadLocalEmbeddingModel(ctx context.Context, endpoint, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório do modelo: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	for _, file := range localEmbeddingFiles {
		url := fmt.Sprintf("%s/%s/resolve/%s/%s", endpoint, localEmbeddingModelID, localEmbeddingRevisao, file.origem)
		target := filepath.Join(dest, file.destino)
		if err := downloadFile(ctx, client, url, target); err != nil {
			return fmt.Errorf("falha ao baixar %s: %w", file.origem, err)
		}
	}
	return nil
}

// localModelPath é a pasta do modelo local em ~/.yby/models.
func localModelPath() string {
	return filepath.Join(getModelDir(), localEmbeddingModelDir)
}

// modeloNoDisco diz se o download do modelo terminou (tokenizer.json é o último arquivo).
func modeloNoDisco(modelPath string) bool {
	_, err := os.Stat(filepath.Join(modelPath, "tokenizer.json"))
	return err == nil
}

// TamanhoABaixar devolve o tamanho aproximado do download do modelo, ou "" quando ele
// já está no disco.
func (p *LocalEmbeddingProvider) TamanhoABaixar() string {
	if modeloNoDisco(localModelPath()) {
		return ""
	}
	return localEmbeddingTamanho
}

// BaixarModelo baixa o modelo para ~/.yby/models; não faz nada se ele já está lá.
func (p *LocalEmbeddingProvider) BaixarModelo(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return baixarSeFaltar(ctx, localModelPath())
}

// ErrBaixarModelo é a falha no download do modelo de embeddings (rede, proxy, espelho).
var ErrBaixarModelo = errors.New("não deu para baixar o modelo de embeddings")

// baixarSeFaltar baixa o modelo para modelPath se o download ainda não terminou.
func baixarSeFaltar(ctx context.Context, modelPath string) error {
	if modeloNoDisco(modelPath) {
		return nil
	}
	if err := downloadLocalEmbeddingModel(ctx, huggingFaceEndpoint(), modelPath); err != nil {
		return fmt.Errorf("%w %s: %w", ErrBaixarModelo, localEmbeddingModelID, err)
	}
	return nil
}

// BaixaModelo é o provider de embeddings que baixa o modelo no primeiro uso (o local).
type BaixaModelo interface {
	// TamanhoABaixar devolve o tamanho aproximado do download (ex.: "~120 MB"), ou ""
	// quando o modelo já está no disco.
	TamanhoABaixar() string
	// BaixarModelo baixa o modelo; não faz nada se ele já está no disco.
	BaixarModelo(ctx context.Context) error
}

// PrepararEmbeddings baixa o modelo dos embeddings de p, se faltar, numa etapa própria
// ("baixar o modelo de embeddings (~120 MB)"): sem ela, o download ficava escondido na
// etapa de quem pediu os embeddings. Atravessa os decorators; sem nada a baixar, não
// abre etapa. A falha do download vem com ErrBaixarModelo.
func PrepararEmbeddings(ctx context.Context, p Provider, et shared.Etapas) error {
	for p != nil {
		if b, ok := p.(BaixaModelo); ok {
			tamanho := b.TamanhoABaixar()
			if tamanho == "" {
				return nil
			}
			et.Iniciar("baixar o modelo de embeddings (" + tamanho + ")")
			if err := b.BaixarModelo(ctx); err != nil {
				et.Falhar("")
				if !errors.Is(err, ErrBaixarModelo) {
					err = fmt.Errorf("%w: %w", ErrBaixarModelo, err)
				}
				return err
			}
			et.Concluir("")
			return nil
		}
		d, ok := p.(decorador)
		if !ok {
			return nil
		}
		p = d.Unwrap()
	}
	return nil
}

// downloadFile baixa url para target via arquivo temporário, para que um
// download interrompido não deixe um arquivo truncado no lugar do final.
func downloadFile(ctx context.Context, client *http.Client, url, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status HTTP %d", resp.StatusCode)
	}

	tmp := target + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, target)
}

// getModelDir retorna o diretório onde o modelo é armazenado.
func getModelDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".yby", "models")
	}
	return filepath.Join(home, ".yby", "models")
}

// initPipeline inicializa o pipeline de embeddings (lazy loading). Quem não chamou
// PrepararEmbeddings antes (o Bard) baixa o modelo aqui, só com o aviso no log.
func (p *LocalEmbeddingProvider) initPipeline(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready {
		return nil
	}

	// Criar sessão hugot com backend Go puro (sem ONNX Runtime)
	session, err := hugot.NewGoSession()
	if err != nil {
		return fmt.Errorf("falha ao criar sessão hugot: %w", err)
	}

	modelPath := localModelPath()
	if !modeloNoDisco(modelPath) {
		slog.Info("baixando o modelo de embeddings local", "modelo", localEmbeddingModelID, "tamanho", localEmbeddingTamanho)
		if dlErr := baixarSeFaltar(ctx, modelPath); dlErr != nil {
			_ = session.Destroy()
			return dlErr
		}
	}

	// Criar pipeline de feature extraction
	pipelineConfig := backends.PipelineConfig[*pipelines.FeatureExtractionPipeline]{
		ModelPath: modelPath,
		Name:      "embedding",
	}

	pipeline, err := hugot.NewPipeline(session, pipelineConfig)
	if err != nil {
		_ = session.Destroy()
		return fmt.Errorf("falha ao criar pipeline de embedding: %w", err)
	}

	// O corte por tokens usa o tokenizador do próprio pipeline, para contar exatamente o
	// que vai ao modelo.
	tk := pipeline.Model.Tokenizer
	if tk == nil || tk.GoTokenizer == nil || tk.GoTokenizer.Tokenizer == nil {
		_ = session.Destroy()
		return fmt.Errorf("falha ao criar pipeline de embedding: o modelo em %s não trouxe o tokenizador", modelPath)
	}
	goTK := tk.GoTokenizer.Tokenizer
	p.contarTokens = func(s string) int { return len(goTK.Encode(s)) }
	p.limiteTokens = tk.MaxAllowedTokens
	if p.limiteTokens <= 0 {
		p.limiteTokens = limiteTokensPadrao
	}

	p.session = session
	p.pipeline = pipeline
	p.ready = true

	return nil
}

// GenerateGovernance não é suportado pelo provider local.
func (p *LocalEmbeddingProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, fmt.Errorf("GenerateGovernance nao suportado pelo %s", p.Name())
}

// Completion não é suportado pelo provider local.
func (p *LocalEmbeddingProvider) Completion(_ context.Context, _, _ string) (string, error) {
	return "", fmt.Errorf("Completion nao suportado pelo %s — use apenas para embeddings", p.Name())
}

// StreamCompletion não é suportado pelo provider local.
func (p *LocalEmbeddingProvider) StreamCompletion(_ context.Context, _, _ string, _ io.Writer) error {
	return fmt.Errorf("StreamCompletion nao suportado pelo %s", p.Name())
}

// EmbedDocuments gera embeddings localmente com o modelo multilíngue.
func (p *LocalEmbeddingProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if err := p.initPipeline(ctx); err != nil {
		return nil, err
	}

	// O modelo aceita no máximo 512 tokens e um texto acima disso derruba o lote inteiro:
	// o texto é cortado em tokens (não em bytes: um base64 vira quase um token por
	// caractere) antes de ir ao pipeline.
	prepared := make([]string, len(texts))
	for i, t := range texts {
		prepared[i] = prepararParaModelo(t, p.limiteTokens, p.contarTokens)
	}

	// Processar em batches de 16 (conservador pra evitar OOM)
	const batchSize = 16
	var allEmbeddings [][]float32

	for i := 0; i < len(prepared); i += batchSize {
		end := i + batchSize
		if end > len(prepared) {
			end = len(prepared)
		}
		batch := prepared[i:end]

		result, err := p.pipeline.RunPipeline(batch)
		if err != nil {
			return nil, fmt.Errorf("falha ao gerar embeddings (batch %d): %w", i/batchSize, err)
		}

		for _, embedding := range result.Embeddings {
			allEmbeddings = append(allEmbeddings, embedding)
		}
	}

	return allEmbeddings, nil
}

// Destroy libera os recursos da sessão.
func (p *LocalEmbeddingProvider) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		_ = p.session.Destroy()
		p.session = nil
		p.ready = false
	}
}

const (
	// limiteTokensPadrao é o limite de posições do modelo (max_position_embeddings do
	// config.json), usado quando o hugot não o informa. Passar dele derruba o lote inteiro
	// num pânico do grafo ("got shapes [1 627 384] vs [1 512 384]").
	limiteTokensPadrao = 512
	// maxBytesLocal é o teto, em bytes, do texto antes de tokenizar: só limita o trabalho
	// com textos enormes; o corte de verdade é em tokens. 512 tokens de texto real cabem
	// com folga.
	maxBytesLocal = 16 << 10
	// maxRunasPorPalavra é o tamanho máximo de uma palavra (sequência sem espaço) entregue
	// ao modelo. O tokenizador Unigram do hugot em Go puro custa da ordem do cubo do
	// tamanho da palavra (um base64 de 2000 caracteres leva ~6 s) e a transforma em quase
	// um token por caractere; base64, hex e afins não carregam sentido para a busca.
	maxRunasPorPalavra = 128
)

// prepararParaModelo devolve o texto que vai ao modelo local: as palavras longas demais
// encurtadas para maxRunasPorPalavra e o texto cortado no maior começo que o tokenizador
// (contar, que já soma os tokens especiais) transforma em no máximo limite tokens. O
// backend Go do hugot não trunca por tokens (ignora o MaxLen e o truncation do
// tokenizer.json), então o corte tem de vir daqui. Corta sempre numa divisa de caractere.
func prepararParaModelo(t string, limite int, contar func(string) int) string {
	t = encurtarPalavras(cortarEmBytes(t, maxBytesLocal), maxRunasPorPalavra)
	if contar(t) <= limite {
		return t
	}
	// Busca binária pelo maior começo que cabe, entre as divisas de caractere. O começo
	// vazio sempre cabe (só os tokens especiais) e o texto inteiro não: lo cabe, hi não.
	divisas := make([]int, 0, len(t))
	for i := range t {
		divisas = append(divisas, i)
	}
	lo, hi := 0, len(divisas)
	for hi-lo > 1 {
		meio := (lo + hi) / 2
		if contar(t[:divisas[meio]]) <= limite {
			lo = meio
		} else {
			hi = meio
		}
	}
	return t[:divisas[lo]]
}

// cortarEmBytes limita o texto a max bytes, recuando até a divisa do último caractere
// inteiro: cortar no meio de uma runa entregaria UTF-8 inválido ao tokenizador.
func cortarEmBytes(t string, max int) string {
	if len(t) <= max {
		return t
	}
	corte := max
	for corte > 0 && !utf8.RuneStart(t[corte]) {
		corte--
	}
	return t[:corte]
}

// encurtarPalavras deixa só as max primeiras runas de cada palavra (sequência sem
// espaço) mais longa que isso; os espaços ficam como estão.
func encurtarPalavras(t string, max int) string {
	var b strings.Builder
	runasNaPalavra := 0
	encurtou := false
	for i, r := range t {
		if unicode.IsSpace(r) {
			runasNaPalavra = 0
		} else {
			runasNaPalavra++
			if runasNaPalavra > max {
				if !encurtou {
					b.Grow(len(t))
					b.WriteString(t[:i])
					encurtou = true
				}
				continue
			}
		}
		if encurtou {
			b.WriteRune(r)
		}
	}
	if !encurtou {
		return t
	}
	return b.String()
}
