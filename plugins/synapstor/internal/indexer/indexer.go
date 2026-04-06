package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// IndexManifest rastreia o estado de indexação de cada arquivo.
type IndexManifest struct {
	Files map[string]IndexedFile `json:"files"`
}

// IndexedFile registra o hash e timestamp de um arquivo indexado.
type IndexedFile struct {
	SHA256    string    `json:"sha256"`
	IndexedAt time.Time `json:"indexed_at"`
}

const manifestFile = ".synapstor/.index_manifest.json"

// Indexer manages the knowledge ingestion pipeline
type Indexer struct {
	Provider    ai.Provider
	RootDir     string
	FullReindex bool
}

func NewIndexer(provider ai.Provider, rootDir string) *Indexer {
	return &Indexer{
		Provider: provider,
		RootDir:  rootDir,
	}
}

// loadManifest carrega o manifest de indexação do disco.
func (i *Indexer) loadManifest() *IndexManifest {
	data, err := os.ReadFile(filepath.Join(i.RootDir, manifestFile))
	if err != nil {
		return &IndexManifest{Files: make(map[string]IndexedFile)}
	}
	var m IndexManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &IndexManifest{Files: make(map[string]IndexedFile)}
	}
	if m.Files == nil {
		m.Files = make(map[string]IndexedFile)
	}
	return &m
}

// saveManifest persiste o manifest de indexação no disco.
func (i *Indexer) saveManifest(m *IndexManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Join(i.RootDir, manifestFile))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(i.RootDir, manifestFile), data, 0644)
}

// fileHash calcula o hash SHA-256 de um arquivo.
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// IndexReport contém métricas coletadas durante o processo de indexação.
type IndexReport struct {
	FilesScanned      int
	FilesSkipped      int
	ChunksGenerated   int
	EmbeddingsCreated int
	Duration          time.Duration
}

// Run executa o pipeline de indexação e retorna um relatório com métricas.
func (i *Indexer) Run(ctx context.Context) (*IndexReport, error) {
	start := time.Now()
	report := &IndexReport{}

	// 1. Coletar arquivos
	files, err := i.scanFiles()
	if err != nil {
		return nil, err
	}

	report.FilesScanned = len(files)

	if len(files) == 0 {
		report.Duration = time.Since(start)
		return report, nil
	}

	// 2. Carregar manifest para indexação incremental
	manifest := i.loadManifest()
	newManifest := &IndexManifest{Files: make(map[string]IndexedFile)}

	// 3. Processar arquivos -> chunks
	var allChunks []string
	var allMetadatas []map[string]string
	var allIDs []string

	for _, path := range files {
		relPath, _ := filepath.Rel(i.RootDir, path)
		hash, err := fileHash(path)
		if err != nil {
			continue
		}

		// Verificar se já indexado e não mudou
		if !i.FullReindex {
			if existing, ok := manifest.Files[relPath]; ok && existing.SHA256 == hash {
				newManifest.Files[relPath] = existing
				report.FilesSkipped++
				continue
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		chunks := splitMarkdown(string(content))
		baseName := filepath.Base(path)

		for idx, chunk := range chunks {
			chunkID := fmt.Sprintf("%s#%d", relPath, idx)

			meta := map[string]string{
				"source":   relPath,
				"filename": baseName,
			}

			lines := strings.Split(chunk, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# ") {
					meta["title"] = strings.TrimPrefix(line, "# ")
					break
				}
			}

			allChunks = append(allChunks, chunk)
			allMetadatas = append(allMetadatas, meta)
			allIDs = append(allIDs, chunkID)
		}

		newManifest.Files[relPath] = IndexedFile{
			SHA256:    hash,
			IndexedAt: time.Now(),
		}
	}

	report.ChunksGenerated = len(allChunks)

	// 3.5. Detectar arquivos removidos (presentes no manifest antigo, ausentes no novo)
	storePath := filepath.Join(i.RootDir, ".synapstor", ".index")
	var removedFiles []string
	for relPath := range manifest.Files {
		if _, exists := newManifest.Files[relPath]; !exists {
			removedFiles = append(removedFiles, relPath)
		}
	}
	if len(removedFiles) > 0 {
		vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
		if err == nil {
			for _, relPath := range removedFiles {
				_ = vs.DeleteByMetadata(ctx, map[string]string{"source": relPath})
			}
		}
	}

	// 4. Se não há chunks novos, salvar manifest e retornar
	if len(allChunks) == 0 {
		if err := i.saveManifest(newManifest); err != nil {
			return nil, fmt.Errorf("erro ao salvar manifest de indexação: %w", err)
		}
		report.Duration = time.Since(start)
		return report, nil
	}

	// 5. Inicializar Vector Store
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar vector store: %w", err)
	}

	// 6. Indexar
	if err := vs.AddDocuments(ctx, allChunks, allMetadatas, allIDs); err != nil {
		return nil, err
	}

	report.EmbeddingsCreated = len(allChunks)

	// 7. Salvar manifest atualizado
	if err := i.saveManifest(newManifest); err != nil {
		return nil, fmt.Errorf("erro ao salvar manifest de indexação: %w", err)
	}

	report.Duration = time.Since(start)
	return report, nil
}

func (i *Indexer) scanFiles() ([]string, error) {
	candidates := []string{}

	// Targets: .synapstor/.uki and docs/wiki
	targets := []string{
		filepath.Join(i.RootDir, ".synapstor", ".uki"),
		filepath.Join(i.RootDir, "docs", "wiki"),
	}

	for _, dir := range targets {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				candidates = append(candidates, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return candidates, nil
}

// splitMarkdown breaks markdown into logical chunks based on headers
func splitMarkdown(content string) []string {
	// Simple splitter: split by headers 1 and 2
	// Improves context retrieval by keeping sections together
	re := regexp.MustCompile(`(?m)^#{1,2}\s`)
	indexes := re.FindAllStringIndex(content, -1)

	if len(indexes) == 0 {
		return []string{content}
	}

	var chunks []string
	lastIdx := 0

	for i, idx := range indexes {
		start := idx[0]
		if i == 0 && start > 0 {
			// Content before first header
			chunks = append(chunks, content[0:start])
		}

		if i > 0 {
			chunks = append(chunks, content[lastIdx:start])
		}
		lastIdx = start
	}

	// Last chunk
	if lastIdx < len(content) {
		chunks = append(chunks, content[lastIdx:])
	}

	// Cleanup empty chunks
	var cleanChunks []string
	for _, c := range chunks {
		trimmed := strings.TrimSpace(c)
		if len(trimmed) > 10 { // Ignore very small chunks
			cleanChunks = append(cleanChunks, trimmed)
		}
	}

	return cleanChunks
}
