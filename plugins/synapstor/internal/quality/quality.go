// Package quality implementa scoring de qualidade para documentos UKI.
package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/plugins/synapstor/internal/uki"
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

// reContexto acha a seção que abre um UKI na especificação (Contexto nas regras e
// métricas, Problema nas decisões, Quando usar nos padrões, Visão geral nas
// referências), em qualquer caixa, em qualquer nível de seção e com numeração.
var reContexto = regexp.MustCompile(`(?mi)^#{2,}\s+(?:\d+[.)]?\s*)?(contexto|context|problema|problem|quando usar|vis[aã]o geral|overview)\b`)

// ScoreUKI avalia a qualidade de um documento UKI. Os metadados saem do frontmatter (ou
// das linhas em negrito do formato antigo); o resto da nota olha só o corpo.
func ScoreUKI(content string) QualityScore {
	score := QualityScore{}
	meta, content := uki.Ler(content)
	score.Title = meta.Titulo

	// Verificar contexto
	score.Breakdown.HasContext = reContexto.MatchString(content)
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

	// Metadados (ID, tipo e status)
	score.Breakdown.HasMetadata = meta.Completos()
	if score.Breakdown.HasMetadata {
		score.Score += 15
	}

	return score
}

// ScoreAll avalia todos os UKIs de um diretório e das subpastas de domínio.
func ScoreAll(ukiDir string) ([]QualityScore, error) {
	arquivos, err := uki.Listar(ukiDir)
	if err != nil {
		return nil, err
	}

	var scores []QualityScore
	for _, a := range arquivos {
		data, err := os.ReadFile(a.Caminho)
		if err != nil {
			continue
		}

		qs := ScoreUKI(string(data))
		qs.Path = a.Caminho
		scores = append(scores, qs)
	}

	return scores, nil
}

// FormatScore formata um QualityScore para exibição.
func FormatScore(qs QualityScore) string {
	return fmt.Sprintf("[%3d/100] %s (%s)", qs.Score, qs.Title, filepath.Base(qs.Path))
}
