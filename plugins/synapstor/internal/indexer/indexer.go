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

	// 2. Carregar manifest para indexação incremental
	manifest := i.loadManifest()
	newManifest := &IndexManifest{Files: make(map[string]IndexedFile)}

	// 3. Process Files -> Chunks
	var allChunks []string
	var allMetadatas []map[string]string
	var allIDs []string
	skipped := 0

	for _, path := range files {
		relPath, _ := filepath.Rel(i.RootDir, path)
		hash, err := fileHash(path)
		if err != nil {
			fmt.Printf("⚠️  Erro ao calcular hash de %s: %v\n", relPath, err)
			continue
		}

		// Verificar se já indexado e não mudou
		if !i.FullReindex {
			if existing, ok := manifest.Files[relPath]; ok && existing.SHA256 == hash {
				newManifest.Files[relPath] = existing // Manter no novo manifest
				skipped++
				continue // Pular - já indexado
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️  Erro ao ler %s: %v\n", path, err)
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

			// Extrair título do chunk se possível
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

	if skipped > 0 {
		fmt.Printf("⏭️  %d arquivos inalterados (pulados). Use --full para forçar reindexação.\n", skipped)
	}

	// 4. Se não há chunks novos, salvar manifest e retornar
	if len(allChunks) == 0 {
		if err := i.saveManifest(newManifest); err != nil {
			fmt.Printf("⚠️  Erro ao salvar manifest de indexação: %v\n", err)
		}
		fmt.Println("✅ Nenhum arquivo novo ou modificado para indexar.")
		return nil
	}

	// 5. Initialize Vector Store
	storePath := filepath.Join(i.RootDir, ".synapstor", ".index")
	vs, err := ai.NewVectorStore(ctx, storePath, i.Provider)
	if err != nil {
		return fmt.Errorf("falha ao inicializar vector store: %w", err)
	}

	// 6. Index
	if err := vs.AddDocuments(ctx, allChunks, allMetadatas, allIDs); err != nil {
		return err
	}

	// 7. Salvar manifest atualizado
	if err := i.saveManifest(newManifest); err != nil {
		fmt.Printf("⚠️  Erro ao salvar manifest de indexação: %v\n", err)
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
