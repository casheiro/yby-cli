package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

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

// IsAvailable sempre retorna true — embeddings locais estão sempre disponíveis.
func (p *LocalEmbeddingProvider) IsAvailable(_ context.Context) bool {
	return true
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

	// Verificar se modelo já foi baixado
	modelPath := filepath.Join(modelDir, "all-MiniLM-L6-v2")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		slog.Info("Baixando modelo de embedding local (all-MiniLM-L6-v2, ~22MB)...")
		opts := hugot.NewDownloadOptions()
		opts.OnnxFilePath = "onnx/model.onnx"
		downloadedPath, dlErr := hugot.DownloadModel(
			"sentence-transformers/all-MiniLM-L6-v2",
			modelDir,
			opts,
		)
		if dlErr != nil {
			_ = session.Destroy()
			return fmt.Errorf("falha ao baixar modelo: %w", dlErr)
		}
		modelPath = downloadedPath
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
