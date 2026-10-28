// Package exporter implementa exportação de UKIs para múltiplos formatos.
package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// nomeManifesto é o arquivo, na raiz do destino, que lista o que o último export gravou.
// O próximo export apaga só o que está nele e não veio de novo; o que a pessoa pôs na
// mesma pasta (--output pode ser o site/docs dela) nunca é tocado.
const nomeManifesto = ".synapstor-export.json"

type manifesto struct {
	Arquivos []string `json:"arquivos"`
}

// limparObsoletos apaga do destino os arquivos do export anterior que o atual não gerou
// e grava o manifesto novo. Caminho do manifesto que sai da pasta é ignorado.
func limparObsoletos(raiz string, gerados []string) error {
	atuais := map[string]bool{}
	for _, g := range gerados {
		atuais[g] = true
	}
	// Toda a limpeza passa pelo os.Root: ele recusa qualquer resolução que saia da raiz,
	// symlink incluído (um repo clonado pode trazer um link e um manifesto adulterado).
	root, err := os.OpenRoot(raiz)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	if data, err := root.ReadFile(nomeManifesto); err == nil {
		var antigo manifesto
		if json.Unmarshal(data, &antigo) == nil {
			for _, rel := range antigo.Arquivos {
				if atuais[rel] || rel == "" || filepath.IsAbs(rel) || !filepath.IsLocal(filepath.FromSlash(rel)) {
					continue
				}
				loc := filepath.FromSlash(rel)
				if err := root.Remove(loc); err != nil {
					if os.IsNotExist(err) {
						continue
					}
					// Caminho que atravessa um link para fora da raiz: ignorado.
					if strings.Contains(err.Error(), "escapes") {
						continue
					}
					return fmt.Errorf("erro ao remover %s: %w", rel, err)
				}
				// Subpasta que ficou vazia sai também; com algo dentro, o Remove falha e fica.
				for d := filepath.Dir(loc); d != "." && d != string(filepath.Separator); d = filepath.Dir(d) {
					if root.Remove(d) != nil {
						break
					}
				}
			}
		}
	}
	sort.Strings(gerados)
	data, err := json.MarshalIndent(manifesto{Arquivos: gerados}, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFile(nomeManifesto, append(data, '\n'), 0644)
}

// relativoA devolve o caminho de saída relativo à raiz, com /.
func relativoA(raiz, caminho string) string {
	rel, err := filepath.Rel(raiz, caminho)
	if err != nil {
		return filepath.ToSlash(caminho)
	}
	return filepath.ToSlash(rel)
}

var (
	reCercaCodigo  = regexp.MustCompile("^\\s*(```|~~~)")
	reCodigoInline = regexp.MustCompile("`[^`\n]*`")
)

// escaparMDX protege `{` e `<` fora de blocos e trechos de código: o Docusaurus 3 lê .md
// como MDX, e `{{ }}`, `<pod>` ou `<https://…>` quebrariam o build do site.
func escaparMDX(corpo string) string {
	linhas := strings.Split(corpo, "\n")
	emCerca := false
	for i, l := range linhas {
		if reCercaCodigo.MatchString(l) {
			emCerca = !emCerca
			continue
		}
		if emCerca {
			continue
		}
		var sb strings.Builder
		pos := 0
		for _, loc := range reCodigoInline.FindAllStringIndex(l, -1) {
			sb.WriteString(trocarMDX(l[pos:loc[0]]))
			sb.WriteString(l[loc[0]:loc[1]])
			pos = loc[1]
		}
		sb.WriteString(trocarMDX(l[pos:]))
		linhas[i] = sb.String()
	}
	return strings.Join(linhas, "\n")
}

func trocarMDX(s string) string {
	s = strings.ReplaceAll(s, "{", "\\{")
	return strings.ReplaceAll(s, "<", "&lt;")
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

	var gerados []string
	for i, u := range ukis {
		frontmatter, corpo := cabecalho(u, fmt.Sprintf("sidebar_position: %d", i+1))
		corpo = escaparMDX(corpo)

		outPath, err := u.destino(docsDir)
		if err == nil {
			err = os.WriteFile(outPath, []byte(frontmatter+corpo), 0644)
		}
		if err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filepath.Base(u.Path), err)
		}
		gerados = append(gerados, relativoA(docsDir, outPath))
	}

	return limparObsoletos(docsDir, gerados)
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

	var gerados []string
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
		gerados = append(gerados, relativoA(outputDir, outPath))
	}

	return limparObsoletos(outputDir, gerados)
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
	var indexEntries, gerados []string
	for _, u := range ukis {
		outPath, err := u.destino(outputDir)
		if err == nil {
			err = os.WriteFile(outPath, []byte(u.Content), 0644)
		}
		if err != nil {
			return fmt.Errorf("erro ao exportar %s: %w", filepath.Base(u.Path), err)
		}
		rel := relativoA(outputDir, outPath)
		gerados = append(gerados, rel)
		indexEntries = append(indexEntries, fmt.Sprintf("- [%s](%s)", u.Title, rel))
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

	return limparObsoletos(outputDir, append(gerados, "README.md"))
}
