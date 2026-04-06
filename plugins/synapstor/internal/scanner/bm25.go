package scanner

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Parâmetros BM25 padrão
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// ScoredResult representa um resultado com pontuação BM25.
type ScoredResult struct {
	Path    string
	Content string
	Score   float64
}

// tokenize divide o texto em tokens normalizados: lowercase, sem pontuação.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return words
}

// computeIDF calcula o Inverse Document Frequency.
// N = total de documentos, n = documentos contendo o termo.
func computeIDF(N, n int) float64 {
	return math.Log((float64(N)-float64(n)+0.5)/(float64(n)+0.5) + 1)
}

// computeBM25 calcula o score BM25 normalizado para um termo.
// tf = frequência do termo no documento, dl = tamanho do documento, avgdl = tamanho médio.
func computeBM25(tf, dl, avgdl float64) float64 {
	num := tf * (bm25K1 + 1)
	den := tf + bm25K1*(1-bm25B+bm25B*(dl/avgdl))
	return num / den
}

// ScoreDocuments calcula o score BM25 para cada documento dado uma query.
func ScoreDocuments(query string, docs []ScanResult) []ScoredResult {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 || len(docs) == 0 {
		results := make([]ScoredResult, len(docs))
		for i, d := range docs {
			results[i] = ScoredResult{Path: d.Path, Content: d.Content, Score: 0}
		}
		return results
	}

	N := len(docs)

	// Tokenizar todos os documentos
	docTokens := make([][]string, N)
	totalLen := 0
	for i, d := range docs {
		tokens := tokenize(d.Content)
		docTokens[i] = tokens
		totalLen += len(tokens)
	}

	avgdl := float64(totalLen) / float64(N)
	if avgdl == 0 {
		avgdl = 1
	}

	// Contar document frequency para cada termo da query
	df := make(map[string]int)
	for _, qt := range queryTokens {
		for i := range docs {
			for _, dt := range docTokens[i] {
				if dt == qt {
					df[qt]++
					break
				}
			}
		}
	}

	// Calcular score para cada documento
	results := make([]ScoredResult, N)
	for i, d := range docs {
		dl := float64(len(docTokens[i]))

		// Contar term frequency
		tfMap := make(map[string]int)
		for _, t := range docTokens[i] {
			tfMap[t]++
		}

		score := 0.0
		for _, qt := range queryTokens {
			tf := float64(tfMap[qt])
			if tf == 0 {
				continue
			}
			idf := computeIDF(N, df[qt])
			score += idf * computeBM25(tf, dl, avgdl)
		}

		results[i] = ScoredResult{
			Path:    d.Path,
			Content: d.Content,
			Score:   score,
		}
	}

	return results
}

// ScanWithScoring executa um scan e ranqueia os resultados usando BM25.
func ScanWithScoring(root, query string) ([]ScoredResult, error) {
	docs, err := Scan(root, query)
	if err != nil {
		return nil, err
	}

	// Fallback: se query tem apenas 1 token e poucos resultados, Contains já é suficiente
	queryTokens := tokenize(query)
	if len(queryTokens) <= 1 && len(docs) <= 5 {
		results := make([]ScoredResult, len(docs))
		for i, d := range docs {
			results[i] = ScoredResult{Path: d.Path, Content: d.Content, Score: 1.0}
		}
		return results, nil
	}

	scored := ScoreDocuments(query, docs)

	// Ordenar por score descendente
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Limitar a top-50
	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	return scored, nil
}
