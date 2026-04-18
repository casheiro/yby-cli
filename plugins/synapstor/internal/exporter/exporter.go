// Package exporter implementa exportação de UKIs para múltiplos formatos.
package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// UKIFile representa um arquivo UKI para exportação.
type UKIFile struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// Exporter define a interface para exportadores de UKI.
type Exporter interface {
	Export(ukis []UKIFile, outputDir string) error
}

// NewExporter cria um exportador baseado no formato especificado.
func NewExporter(format string) (Exporter, error) {
	switch strings.ToLower(format) {
	case "docusaurus":
		return &DocusaurusExporter{}, nil
	case "obsidian":
		return &ObsidianExporter{}, nil
	case "markdown":
		return &MarkdownExporter{}, nil
	default:
		return nil, fmt.Errorf("formato desconhecido: %s (use docusaurus, obsidian ou markdown)", format)
	}
}

// LoadUKIs carrega todos os UKIs de um diretório.
func LoadUKIs(ukiDir string) ([]UKIFile, error) {
	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}

	reTitle := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	var ukis []UKIFile

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(ukiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		content := string(data)
		title := entry.Name()
		if matches := reTitle.FindStringSubmatch(content); len(matches) > 1 {
			title = strings.TrimSpace(matches[1])
		}

		ukis = append(ukis, UKIFile{
			Path:    filePath,
			Title:   title,
			Content: content,
		})
	}

	return ukis, nil
}

// --- DocusaurusExporter ---

// DocusaurusExporter exporta UKIs para formato Docusaurus com frontmatter YAML.
type DocusaurusExporter struct{}

// Export exporta UKIs para formato Docusaurus.
func (e *DocusaurusExporter) Export(ukis []UKIFile, outputDir string) error {
	docsDir := filepath.Join(outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório docs: %w", err)
	}

	for i, uki := range ukis {
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %q\n", uki.Title))
		sb.WriteString(fmt.Sprintf("sidebar_position: %d\n", i+1))
		if len(uki.Tags) > 0 {
			sb.WriteString("tags:\n")
			for _, tag := range uki.Tags {
				sb.WriteString(fmt.Sprintf("  - %s\n", tag))
			}
		}
		sb.WriteString("---\n\n")
		sb.WriteString(uki.Content)

		filename := filepath.Base(uki.Path)
		outPath := filepath.Join(docsDir, filename)
		if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filename, err)
		}
	}

	return nil
}

// --- ObsidianExporter ---

// ObsidianExporter exporta UKIs para formato Obsidian com wikilinks.
type ObsidianExporter struct{}

var reMarkdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*UKI-[^)]*\.md)\)`)

// Export exporta UKIs para formato Obsidian.
func (e *ObsidianExporter) Export(ukis []UKIFile, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de saída: %w", err)
	}

	for _, uki := range ukis {
		var sb strings.Builder

		// Frontmatter YAML
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %q\n", uki.Title))
		if len(uki.Tags) > 0 {
			sb.WriteString("tags:\n")
			for _, tag := range uki.Tags {
				sb.WriteString(fmt.Sprintf("  - %s\n", tag))
			}
		}
		sb.WriteString("---\n\n")

		// Converter links markdown para wikilinks
		content := reMarkdownLink.ReplaceAllStringFunc(uki.Content, func(match string) string {
			submatches := reMarkdownLink.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			// Extrair nome do arquivo sem extensão
			name := strings.TrimSuffix(filepath.Base(submatches[2]), ".md")
			return fmt.Sprintf("[[%s]]", name)
		})
		sb.WriteString(content)

		filename := filepath.Base(uki.Path)
		outPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filename, err)
		}
	}

	return nil
}

// --- MarkdownExporter ---

// MarkdownExporter exporta UKIs como markdown puro com índice.
type MarkdownExporter struct{}

// Export exporta UKIs como markdown com índice README.md.
func (e *MarkdownExporter) Export(ukis []UKIFile, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de saída: %w", err)
	}

	// Copiar arquivos sem modificação
	var indexEntries []string
	for _, uki := range ukis {
		filename := filepath.Base(uki.Path)
		outPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(outPath, []byte(uki.Content), 0644); err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filename, err)
		}
		indexEntries = append(indexEntries, fmt.Sprintf("- [%s](%s)", uki.Title, filename))
	}

	// Criar índice README.md
	var sb strings.Builder
	sb.WriteString("# Índice de Conhecimento (UKIs)\n\n")
	for _, entry := range indexEntries {
		sb.WriteString(entry + "\n")
	}

	readmePath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("erro ao criar índice: %w", err)
	}

	return nil
}
