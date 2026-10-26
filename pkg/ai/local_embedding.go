package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/backends"
	"github.com/knights-analytics/hugot/pipelines"
)

// LocalEmbeddingProvider gera embeddings localmente usando o modelo all-MiniLM-L6-v2.
// Não requer API, não tem rate limit, funciona offline após o download inicial.
type LocalEmbeddingProvider struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
	ready    bool
}

// NewLocalEmbeddingProvider cria um provider de embeddings local.
func NewLocalEmbeddingProvider() *LocalEmbeddingProvider {
	return &LocalEmbeddingProvider{}
}

// Name retorna o identificador do provider.
func (p *LocalEmbeddingProvider) Name() string {
	return "Local Embedding (all-MiniLM-L6-v2)"
}

// IdentidadeDosEmbeddings diz que os embeddings são locais e com que modelo.
func (p *LocalEmbeddingProvider) IdentidadeDosEmbeddings(context.Context) (IdentidadeDoIndice, error) {
	return IdentidadeDoIndice{Provider: "local", Modelo: localEmbeddingModelID}, nil
}

// IsAvailable sempre retorna true — embeddings locais estão sempre disponíveis.
func (p *LocalEmbeddingProvider) IsAvailable(_ context.Context) bool {
	return true
}

const (
	localEmbeddingModelID = "sentence-transformers/all-MiniLM-L6-v2"
	// localEmbeddingModelDir segue o nome usado pelo hugot.DownloadModel,
	// para reaproveitar modelos já baixados por versões anteriores.
	localEmbeddingModelDir = "sentence-transformers_all-MiniLM-L6-v2"
)

// localEmbeddingFiles são os arquivos do repositório HuggingFace lidos pelo
// pipeline de feature extraction do hugot. Todos são salvos na raiz do
// diretório do modelo; tokenizer.json vem por último e marca download completo.
var localEmbeddingFiles = []string{
	"config.json",
	"special_tokens_map.json",
	"tokenizer_config.json",
	"vocab.txt",
	"onnx/model.onnx",
	"tokenizer.json",
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
		url := fmt.Sprintf("%s/%s/resolve/main/%s", endpoint, localEmbeddingModelID, file)
		target := filepath.Join(dest, path.Base(file))
		if err := downloadFile(ctx, client, url, target); err != nil {
			return fmt.Errorf("falha ao baixar %s: %w", file, err)
		}
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

// initPipeline inicializa o pipeline de embeddings (lazy loading).
func (p *LocalEmbeddingProvider) initPipeline() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready {
		return nil
	}

	modelDir := getModelDir()
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de modelos: %w", err)
	}

	// Criar sessão hugot com backend Go puro (sem ONNX Runtime)
	session, err := hugot.NewGoSession()
	if err != nil {
		return fmt.Errorf("falha ao criar sessão hugot: %w", err)
	}

	// Verificar se modelo já foi baixado (tokenizer.json é o último arquivo baixado)
	modelPath := filepath.Join(modelDir, localEmbeddingModelDir)
	if _, err := os.Stat(filepath.Join(modelPath, "tokenizer.json")); os.IsNotExist(err) {
		slog.Info("Baixando modelo de embedding local (all-MiniLM-L6-v2, ~90MB)...")
		if dlErr := downloadLocalEmbeddingModel(context.Background(), huggingFaceEndpoint(), modelPath); dlErr != nil {
			_ = session.Destroy()
			return fmt.Errorf("falha ao baixar modelo: %w", dlErr)
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

// EmbedDocuments gera embeddings localmente usando all-MiniLM-L6-v2.
func (p *LocalEmbeddingProvider) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if err := p.initPipeline(); err != nil {
		return nil, err
	}

	// all-MiniLM-L6-v2 tem limite de 512 tokens (~2000 chars).
	// Textos maiores são truncados pois o modelo não suporta sequências mais longas.
	// Para UKIs, a maioria do conteúdo semântico relevante está no início (título, contexto).
	const maxChars = 2000
	prepared := make([]string, len(texts))
	for i, t := range texts {
		if len(t) > maxChars {
			prepared[i] = t[:maxChars]
		} else {
			prepared[i] = t
		}
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
