package main

// EstimateTokens estima o número de tokens em um texto.
// Usa a heurística de ~4 caracteres por token (padrão para modelos LLM).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4
}

// TokenBudget representa a alocação de tokens por componente do prompt.
type TokenBudget struct {
	MaxTokens    int
	SystemPrompt int
	UserInput    int
	HistoryCtx   int
	RAGCtx       int
}

// TruncateToFit ajusta os componentes do prompt para caber no orçamento de tokens.
// Prioridade de truncamento: histórico primeiro, depois RAG.
// O system prompt e user input nunca são truncados.
func TruncateToFit(maxTokens int, systemPrompt, userInput, historyCtx, ragCtx string) (string, string) {
	systemTokens := EstimateTokens(systemPrompt)
	userTokens := EstimateTokens(userInput)
	fixedTokens := systemTokens + userTokens

	// Se system + user já excedem o limite, retorna contextos vazios
	if fixedTokens >= maxTokens {
		return "", ""
	}

	available := maxTokens - fixedTokens
	historyTokens := EstimateTokens(historyCtx)
	ragTokens := EstimateTokens(ragCtx)

	// Tudo cabe
	if historyTokens+ragTokens <= available {
		return historyCtx, ragCtx
	}

	// Primeiro: truncar histórico, preservar RAG
	if ragTokens <= available {
		remaining := available - ragTokens
		truncatedHistory := truncateToTokens(historyCtx, remaining)
		return truncatedHistory, ragCtx
	}

	// Ambos precisam ser truncados: histórico zerado, RAG truncado
	truncatedRAG := truncateToTokens(ragCtx, available)
	return "", truncatedRAG
}

// truncateToTokens corta o texto para caber no número máximo de tokens estimado.
// Corta do início (mantém o final mais recente) para o histórico.
func truncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	currentTokens := EstimateTokens(text)
	if currentTokens <= maxTokens {
		return text
	}

	// Estimar o número de caracteres que cabem
	maxChars := maxTokens * 4
	if maxChars >= len(text) {
		return text
	}

	// Truncar do início (manter o final mais recente)
	return text[len(text)-maxChars:]
}
