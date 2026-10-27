// Package exporter implementa exportação de UKIs para múltiplos formatos.
package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/uki"
)

// UKIFile representa um arquivo UKI para exportação.
type UKIFile struct {
	Path string `json:"path"`
	// Relativo é o caminho dentro da pasta dos UKIs, com /; o export repete as subpastas
	// de domínio no destino. Vazio, vale o nome do arquivo.
	Relativo string   `json:"relativo"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
}

// destino é o caminho do UKI na pasta de saída, com as subpastas de domínio já criadas.
func (u UKIFile) destino(pasta string) (string, error) {
	rel := u.Relativo
	if rel == "" {
		rel = filepath.Base(u.Path)
	}
	caminho := filepath.Join(pasta, filepath.FromSlash(rel))
	return caminho, os.MkdirAll(filepath.Dir(caminho), 0755)
}

// cabecalho devolve o frontmatter do export (o título, as linhas extras e as tags) e o
// corpo do UKI sem o frontmatter dele, para o arquivo exportado ter um bloco --- só.
func cabecalho(u UKIFile, extras ...string) (string, string) {
	meta, corpo := uki.Ler(u.Content)
	tags := u.Tags
	if len(tags) == 0 {
		tags = meta.Tags
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", u.Title))
	for _, e := range extras {
		sb.WriteString(e + "\n")
	}
	if len(tags) > 0 {
		sb.WriteString("tags:\n")
		for _, tag := range tags {
			sb.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
	}
	sb.WriteString("---\n\n")
	return sb.String(), corpo
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

// LoadUKIs carrega todos os UKIs de um diretório e das subpastas de domínio, com o
// título do corpo e as tags do frontmatter.
func LoadUKIs(ukiDir string) ([]UKIFile, error) {
	arquivos, err := uki.Listar(ukiDir)
	if err != nil {
		return nil, err
	}

	var ukis []UKIFile
	for _, a := range arquivos {
		data, err := os.ReadFile(a.Caminho)
		if err != nil {
			continue
		}

		content := string(data)
		meta, _ := uki.Ler(content)
		title := meta.Titulo
		if title == "" {
			title = filepath.Base(a.Caminho)
		}

		ukis = append(ukis, UKIFile{
			Path:     a.Caminho,
			Relativo: a.Relativo,
			Title:    title,
			Content:  content,
			Tags:     meta.Tags,
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

	for i, u := range ukis {
		frontmatter, corpo := cabecalho(u, fmt.Sprintf("sidebar_position: %d", i+1))

		outPath, err := u.destino(docsDir)
		if err == nil {
			err = os.WriteFile(outPath, []byte(frontmatter+corpo), 0644)
		}
		if err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filepath.Base(u.Path), err)
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

	for _, u := range ukis {
		var sb strings.Builder

		// Frontmatter YAML
		frontmatter, corpo := cabecalho(u)
		sb.WriteString(frontmatter)

		// Converter links markdown para wikilinks
		content := reMarkdownLink.ReplaceAllStringFunc(corpo, func(match string) string {
			submatches := reMarkdownLink.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			// Extrair nome do arquivo sem extensão
			name := strings.TrimSuffix(filepath.Base(submatches[2]), ".md")
			return fmt.Sprintf("[[%s]]", name)
		})
		sb.WriteString(content)

		outPath, err := u.destino(outputDir)
		if err == nil {
			err = os.WriteFile(outPath, []byte(sb.String()), 0644)
		}
		if err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filepath.Base(u.Path), err)
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
	for _, u := range ukis {
		outPath, err := u.destino(outputDir)
		if err == nil {
			err = os.WriteFile(outPath, []byte(u.Content), 0644)
		}
		if err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filepath.Base(u.Path), err)
		}
		rel, _ := filepath.Rel(outputDir, outPath)
		indexEntries = append(indexEntries, fmt.Sprintf("- [%s](%s)", u.Title, filepath.ToSlash(rel)))
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
