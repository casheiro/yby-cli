package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// Indexer manages the knowledge ingestion pipeline
type Indexer struct {
	Provider ai.Provider
	RootDir  string
}

func NewIndexer(provider ai.Provider, rootDir string) *Indexer {
	return &Indexer{
		Provider: provider,
		RootDir:  rootDir,
	}
}

// Run executes the indexing pipeline
func (i *Indexer) Run(ctx context.Context) error {
	fmt.Println("🚀 Iniciando Indexação Semântica...")

	// 1. Gather files
	files, err := i.scanFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("⚠️  Nenhum arquivo markdown encontrado para indexar.")
		return nil
	}

	fmt.Printf("📂 Encontrados %d arquivos. Processando chunks...\n", len(files))

	// 2. Process Files -> Chunks
	var allChunks []string
	var allMetadatas []map[string]string
	var allIDs []string

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️  Erro ao ler %s: %v\n", path, err)
			continue
		}

		chunks := splitMarkdown(string(content))
		baseName := filepath.Base(path)
		relPath, _ := filepath.Rel(i.RootDir, path)

		for idx, chunk := range chunks {
			chunkID := fmt.Sprintf("%s#%d", relPath, idx)

			meta := map[string]string{
				"source":   relPath,
				"filename": baseName,
			}

			// Extract title from chunk if possible
			// Simple heuristic: first line starting with #
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
	}

	// 3. Initialize Vector Store
	storePath := filepath.Join(i.RootDir, ".synapstor", ".index")
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		return fmt.Errorf("falha ao inicializar vector store: %w", err)
	}

	// 4. Index
	// Batch processing could be done here if needed, but VectorStore/Provider handles it
	if err := vs.AddDocuments(ctx, allChunks, allMetadatas, allIDs); err != nil {
		return err
	}

	fmt.Printf("✅ Indexação concluída! %d fragmentos de conhecimento salvos em %s\n", len(allChunks), storePath)
	return nil
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
