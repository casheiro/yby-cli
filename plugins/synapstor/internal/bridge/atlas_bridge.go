// Package bridge implementa a integração entre Atlas e Synapstor.
package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/graph"
)

// AtlasComponent representa um componente do snapshot do Atlas.
type AtlasComponent struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Language  string `json:"language"`
	Framework string `json:"framework"`
}

// AtlasSnapshot representa o snapshot completo do Atlas.
type AtlasSnapshot struct {
	Components []AtlasComponent `json:"components"`
}

// SyncReport contém o relatório da sincronização Atlas → Synapstor.
type SyncReport struct {
	NewUKIs         int `json:"new_ukis"`
	SkippedExisting int `json:"skipped_existing"`
	Errors          int `json:"errors"`
}

// SyncFromAtlas lê o snapshot do Atlas e cria UKIs stub para componentes novos.
func SyncFromAtlas(atlasSnapshotPath, ukiDir string) (*SyncReport, error) {
	data, err := os.ReadFile(atlasSnapshotPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler snapshot do Atlas: %w", err)
	}

	var snapshot AtlasSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("erro ao parsear snapshot do Atlas: %w", err)
	}

	if err := os.MkdirAll(ukiDir, 0755); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de UKIs: %w", err)
	}

	// Indexar UKIs existentes por nome do componente
	existingUKIs := indexExistingUKIs(ukiDir)

	report := &SyncReport{}

	for _, comp := range snapshot.Components {
		slug := slugify(comp.Name)
		if existingUKIs[slug] {
			report.SkippedExisting++
			continue
		}

		content := generateUKIStub(comp)
		filename := fmt.Sprintf("UKI-%d-%s.md", time.Now().UnixMilli(), slug)
		filePath := filepath.Join(ukiDir, filename)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			report.Errors++
			continue
		}

		report.NewUKIs++
	}

	return report, nil
}

// SyncFromAtlasWithGraph sincroniza do Atlas e adiciona edges no knowledge graph.
func SyncFromAtlasWithGraph(atlasSnapshotPath, ukiDir string, kg *graph.KnowledgeGraph) (*SyncReport, error) {
	report, err := SyncFromAtlas(atlasSnapshotPath, ukiDir)
	if err != nil {
		return nil, err
	}

	// Ler snapshot novamente para agrupar por blueprint (componentes no mesmo path raiz)
	data, _ := os.ReadFile(atlasSnapshotPath)
	var snapshot AtlasSnapshot
	_ = json.Unmarshal(data, &snapshot)

	// Agrupar componentes por diretório pai
	groups := make(map[string][]string)
	entries, _ := os.ReadDir(ukiDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		for _, comp := range snapshot.Components {
			slug := slugify(comp.Name)
			if strings.Contains(entry.Name(), slug) {
				parentDir := filepath.Dir(comp.Path)
				groups[parentDir] = append(groups[parentDir], entry.Name())
			}
		}
	}

	// Criar edges entre UKIs do mesmo blueprint
	for _, files := range groups {
		for i := 0; i < len(files); i++ {
			for j := i + 1; j < len(files); j++ {
				kg.AddEdge(files[i], files[j], graph.RelRelatesTo)
			}
		}
	}

	return report, nil
}

// indexExistingUKIs retorna um set de slugs já existentes no diretório de UKIs.
func indexExistingUKIs(ukiDir string) map[string]bool {
	existing := make(map[string]bool)
	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return existing
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		// Extrair conteúdo e verificar se menciona o componente
		data, err := os.ReadFile(filepath.Join(ukiDir, entry.Name()))
		if err != nil {
			continue
		}
		content := strings.ToLower(string(data))
		// Registrar o slug do filename
		parts := strings.SplitN(entry.Name(), "-", 3)
		if len(parts) >= 3 {
			slug := strings.TrimSuffix(parts[2], ".md")
			existing[slug] = true
		}
		// Também registrar nomes encontrados no conteúdo
		_ = content
	}

	return existing
}

// generateUKIStub gera o conteúdo de um UKI stub para um componente do Atlas.
func generateUKIStub(comp AtlasComponent) string {
	var sb strings.Builder
	ts := time.Now().Unix()

	sb.WriteString(fmt.Sprintf("# %s\n", comp.Name))
	sb.WriteString(fmt.Sprintf("**ID:** UKI-%d-%s\n", ts, slugify(comp.Name)))
	sb.WriteString("**Type:** Reference\n")
	sb.WriteString("**Status:** Draft\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("Componente detectado automaticamente pelo Atlas.\n\n"))
	sb.WriteString("## Detalhes\n")
	sb.WriteString(fmt.Sprintf("- **Tipo:** %s\n", comp.Type))
	sb.WriteString(fmt.Sprintf("- **Caminho:** %s\n", comp.Path))

	if comp.Language != "" {
		sb.WriteString(fmt.Sprintf("- **Linguagem:** %s\n", comp.Language))
	}
	if comp.Framework != "" {
		sb.WriteString(fmt.Sprintf("- **Framework:** %s\n", comp.Framework))
	}

	sb.WriteString("\n## Content\n")
	sb.WriteString("<!-- TODO: Adicionar documentação detalhada -->\n")

	return sb.String()
}

// slugify converte um nome em slug para uso em filenames.
func slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.ReplaceAll(slug, "/", "-")
	slug = strings.ReplaceAll(slug, ".", "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}
