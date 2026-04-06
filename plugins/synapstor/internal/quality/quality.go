// Package quality implementa scoring de qualidade para documentos UKI.
package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// QualityBreakdown detalha os critérios avaliados.
type QualityBreakdown struct {
	HasContext  bool `json:"has_context"`
	HasExamples bool `json:"has_examples"`
	HasHeaders  bool `json:"has_headers"`
	WordCount   int  `json:"word_count"`
	LinkCount   int  `json:"link_count"`
	HeaderCount int  `json:"header_count"`
	HasMetadata bool `json:"has_metadata"`
}

// QualityScore representa a pontuação de qualidade de um UKI.
type QualityScore struct {
	Path      string           `json:"path"`
	Title     string           `json:"title"`
	Score     int              `json:"score"`
	Breakdown QualityBreakdown `json:"breakdown"`
}

var reHeader = regexp.MustCompile(`(?m)^#{2,}\s+.+$`)
var reCodeBlock = regexp.MustCompile("(?s)```[\\s\\S]*?```")
var reLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var reTitle = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// ScoreUKI avalia a qualidade de um documento UKI.
func ScoreUKI(content string) QualityScore {
	score := QualityScore{}

	// Extrair título
	if matches := reTitle.FindStringSubmatch(content); len(matches) > 1 {
		score.Title = strings.TrimSpace(matches[1])
	}

	// Verificar contexto
	lower := strings.ToLower(content)
	score.Breakdown.HasContext = strings.Contains(lower, "## context") || strings.Contains(lower, "## contexto")
	if score.Breakdown.HasContext {
		score.Score += 15
	}

	// Verificar exemplos (blocos de código)
	codeBlocks := reCodeBlock.FindAllString(content, -1)
	score.Breakdown.HasExamples = len(codeBlocks) > 0
	if score.Breakdown.HasExamples {
		score.Score += 20
	}

	// Verificar headers (H2+)
	headers := reHeader.FindAllString(content, -1)
	score.Breakdown.HeaderCount = len(headers)
	score.Breakdown.HasHeaders = len(headers) > 0
	if len(headers) > 0 {
		pts := 10
		extra := len(headers) - 3
		if extra > 0 {
			pts += extra * 2
		}
		if pts > 20 {
			pts = 20
		}
		score.Score += pts
	}

	// Word count
	words := strings.Fields(content)
	score.Breakdown.WordCount = len(words)
	switch {
	case len(words) > 500:
		score.Score += 20
	case len(words) > 300:
		score.Score += 15
	case len(words) > 100:
		score.Score += 10
	}

	// Links/referências
	links := reLink.FindAllString(content, -1)
	score.Breakdown.LinkCount = len(links)
	if len(links) > 0 {
		score.Score += 10
	}

	// Metadata (ID, Type, Status)
	hasID := strings.Contains(content, "**ID:**")
	hasType := strings.Contains(content, "**Type:**") || strings.Contains(content, "**Tipo:**")
	hasStatus := strings.Contains(content, "**Status:**")
	score.Breakdown.HasMetadata = hasID && hasType && hasStatus
	if score.Breakdown.HasMetadata {
		score.Score += 15
	}

	return score
}

// ScoreAll avalia todos os UKIs em um diretório.
func ScoreAll(ukiDir string) ([]QualityScore, error) {
	entries, err := os.ReadDir(ukiDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório de UKIs: %w", err)
	}

	var scores []QualityScore
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(ukiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		qs := ScoreUKI(string(data))
		qs.Path = filePath
		scores = append(scores, qs)
	}

	return scores, nil
}

// FormatScore formata um QualityScore para exibição.
func FormatScore(qs QualityScore) string {
	return fmt.Sprintf("[%3d/100] %s (%s)", qs.Score, qs.Title, filepath.Base(qs.Path))
}
