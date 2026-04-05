package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
)

const defaultTopK = 5

// searchResult representa um resultado de busca formatável.
type searchResult struct {
	Index   int
	Score   float32
	Title   string
	Preview string
}

// parseSearchArgs extrai a query e o valor de --top-k dos argumentos.
// Retorna a query concatenada e o topK (padrão: 5).
func parseSearchArgs(args []string) (string, int) {
	topK := defaultTopK
	var queryParts []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--top-k" && i+1 < len(args) {
			if v, err := strconv.Atoi(args[i+1]); err == nil && v > 0 {
				topK = v
			}
			i++ // pular o valor
			continue
		}
		// Ignorar --top-k=N formato inline
		if strings.HasPrefix(args[i], "--top-k=") {
			val := strings.TrimPrefix(args[i], "--top-k=")
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				topK = v
			}
			continue
		}
		queryParts = append(queryParts, args[i])
	}

	return strings.Join(queryParts, " "), topK
}

// runSearch inicializa o provider e vector store, executa a busca e imprime os resultados.
func runSearch(args []string) {
	query, topK := parseSearchArgs(args)
	if strings.TrimSpace(query) == "" {
		fmt.Println("❌ Uso: yby synapstor search \"sua consulta\" [--top-k N]")
		return
	}

	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA configurado.")
		return
	}

	cwd := "."
	storePath := filepath.Join(cwd, ".synapstor", ".index")
	vs, err := ai.NewVectorStore(ctx, storePath, provider)
	if err != nil {
		fmt.Printf("❌ Erro ao abrir índice: %v\n", err)
		fmt.Println("   Execute 'yby synapstor index' primeiro para criar o índice.")
		return
	}

	docs, err := vs.Search(ctx, query, topK)
	if err != nil {
		fmt.Printf("❌ Erro na busca: %v\n", err)
		return
	}

	results := make([]searchResult, len(docs))
	for i, doc := range docs {
		title := doc.Metadata["title"]
		if title == "" {
			title = doc.Metadata["filename"]
		}
		if title == "" {
			title = doc.ID
		}

		preview := truncatePreview(doc.Content, 200)

		results[i] = searchResult{
			Index:   i + 1,
			Score:   doc.Score,
			Title:   title,
			Preview: preview,
		}
	}

	fmt.Print(formatSearchResults(query, results))
}

// formatSearchResults formata os resultados de busca para exibição no terminal.
// Função pura para facilitar testes.
func formatSearchResults(query string, results []searchResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("🔍 Nenhum resultado encontrado para: %q\n", query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Resultados para: %q (%d encontrados)\n\n", query, len(results)))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] (%.2f) %s\n", r.Index, r.Score, r.Title))
		sb.WriteString(fmt.Sprintf("    %s\n\n", r.Preview))
	}

	return sb.String()
}

// truncatePreview corta o conteúdo no limite de caracteres, adicionando "..." se truncado.
func truncatePreview(content string, maxLen int) string {
	// Remover quebras de linha para preview compacto
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.TrimSpace(content)

	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
