package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 1 << 20 // 1MB
const maxResults = 50

// ScanResult holds the path and content (optional) of a file
type ScanResult struct {
	Path    string
	Content string
}

// Scan percorre o diretório e retorna arquivos que correspondem aos critérios.
// Se query for fornecida, faz uma verificação simples de "contains" no nome ou conteúdo.
// Resultados com match no nome têm prioridade sobre match no conteúdo.
func Scan(root string, query string) ([]ScanResult, error) {
	var nameMatches []ScanResult    // Prioridade alta
	var contentMatches []ScanResult // Prioridade baixa
	query = strings.ToLower(query)

	ignores := map[string]bool{
		".git":              true,
		"node_modules":      true,
		"vendor":            true,
		"dist":              true,
		".synapstor":        true,
		"go.sum":            true,
		"package-lock.json": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if ignores[d.Name()] {
				return filepath.SkipDir
			}
			// Pular diretórios ocultos
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Ignorar arquivos ocultos
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Verificar limite total
		if len(nameMatches)+len(contentMatches) >= maxResults {
			return filepath.SkipAll
		}

		// Pular arquivos grandes (> 1MB)
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)

		// Sem query = match tudo
		if query == "" {
			content, err := os.ReadFile(path)
			if err == nil && isText(content) {
				nameMatches = append(nameMatches, ScanResult{Path: relPath, Content: string(content)})
			}
			return nil
		}

		// Verificar match no nome (prioridade)
		if strings.Contains(strings.ToLower(relPath), query) {
			content, err := os.ReadFile(path)
			if err == nil && isText(content) {
				nameMatches = append(nameMatches, ScanResult{Path: relPath, Content: string(content)})
			}
			return nil
		}

		// Match no conteúdo (ler uma única vez)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !isText(content) {
			return nil
		}
		if strings.Contains(strings.ToLower(string(content)), query) {
			contentMatches = append(contentMatches, ScanResult{Path: relPath, Content: string(content)})
		}

		return nil
	})

	// Combinar resultados: nome primeiro, conteúdo depois, limitando ao total
	results := append(nameMatches, contentMatches...)
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, err
}

func isText(data []byte) bool {
	// Verificação simples: procurar null bytes
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
