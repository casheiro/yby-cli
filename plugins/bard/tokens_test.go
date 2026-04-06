package main

import (
	"strings"
	"testing"
)

// TestEstimateTokens_TextoVazio verifica que texto vazio retorna 0 tokens.
func TestEstimateTokens_TextoVazio(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("esperado 0, obtido %d", got)
	}
}

// TestEstimateTokens_TextoCurto verifica a estimativa para textos curtos.
func TestEstimateTokens_TextoCurto(t *testing.T) {
	// "abcd" = 4 chars = 1 token
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("esperado 1, obtido %d", got)
	}
	// "ab" = 2 chars ~ 1 token (arredondamento para cima)
	if got := EstimateTokens("ab"); got != 1 {
		t.Errorf("esperado 1, obtido %d", got)
	}
}

// TestEstimateTokens_TextoLongo verifica proporcionalidade da estimativa.
func TestEstimateTokens_TextoLongo(t *testing.T) {
	text := strings.Repeat("a", 400)
	got := EstimateTokens(text)
	if got != 100 {
		t.Errorf("esperado 100, obtido %d", got)
	}
}

// TestTruncateToFit_TudoCabe verifica que nada é truncado quando tudo cabe.
func TestTruncateToFit_TudoCabe(t *testing.T) {
	system := strings.Repeat("s", 100)  // 25 tokens
	user := strings.Repeat("u", 100)    // 25 tokens
	history := strings.Repeat("h", 100) // 25 tokens
	rag := strings.Repeat("r", 100)     // 25 tokens
	// Total: 100 tokens, budget: 200

	gotHistory, gotRAG := TruncateToFit(200, system, user, history, rag)

	if gotHistory != history {
		t.Error("histórico não deveria ser truncado")
	}
	if gotRAG != rag {
		t.Error("RAG não deveria ser truncado")
	}
}

// TestTruncateToFit_HistoricoTruncado verifica que o histórico é truncado primeiro.
func TestTruncateToFit_HistoricoTruncado(t *testing.T) {
	system := strings.Repeat("s", 200)  // 50 tokens
	user := strings.Repeat("u", 200)    // 50 tokens
	history := strings.Repeat("h", 400) // 100 tokens
	rag := strings.Repeat("r", 200)     // 50 tokens
	// Fixo: 100 tokens. Disponível: 100. RAG: 50. Histórico: 100 -> truncado para 50.

	gotHistory, gotRAG := TruncateToFit(200, system, user, history, rag)

	if gotRAG != rag {
		t.Error("RAG não deveria ser truncado quando só histórico precisa")
	}
	if len(gotHistory) >= len(history) {
		t.Error("histórico deveria ter sido truncado")
	}
	if len(gotHistory) == 0 {
		t.Error("histórico não deveria estar completamente vazio")
	}
}

// TestTruncateToFit_AmbosTruncados verifica truncamento de histórico e RAG.
func TestTruncateToFit_AmbosTruncados(t *testing.T) {
	system := strings.Repeat("s", 400)  // 100 tokens
	user := strings.Repeat("u", 200)    // 50 tokens
	history := strings.Repeat("h", 400) // 100 tokens
	rag := strings.Repeat("r", 400)     // 100 tokens
	// Fixo: 150 tokens. Disponível: 50. RAG sozinho já não cabe (100 > 50).

	gotHistory, gotRAG := TruncateToFit(200, system, user, history, rag)

	if gotHistory != "" {
		t.Error("histórico deveria estar vazio quando ambos são truncados")
	}
	if len(gotRAG) >= len(rag) {
		t.Error("RAG deveria ter sido truncado")
	}
	if len(gotRAG) == 0 {
		t.Error("RAG não deveria estar completamente vazio")
	}
}

// TestTruncateToFit_SemEspacoParaContexto verifica quando system+user já excedem o limite.
func TestTruncateToFit_SemEspacoParaContexto(t *testing.T) {
	system := strings.Repeat("s", 400) // 100 tokens
	user := strings.Repeat("u", 400)   // 100 tokens
	history := "histórico qualquer"
	rag := "rag qualquer"

	gotHistory, gotRAG := TruncateToFit(100, system, user, history, rag)

	if gotHistory != "" {
		t.Errorf("histórico deveria estar vazio, obtido %q", gotHistory)
	}
	if gotRAG != "" {
		t.Errorf("RAG deveria estar vazio, obtido %q", gotRAG)
	}
}

// TestTruncateToFit_ContextosVazios verifica com histórico e RAG vazios.
func TestTruncateToFit_ContextosVazios(t *testing.T) {
	system := strings.Repeat("s", 100)
	user := strings.Repeat("u", 100)

	gotHistory, gotRAG := TruncateToFit(200, system, user, "", "")

	if gotHistory != "" {
		t.Error("histórico deveria permanecer vazio")
	}
	if gotRAG != "" {
		t.Error("RAG deveria permanecer vazio")
	}
}

// TestTruncateToTokens_TextoCurto verifica que texto dentro do limite não é truncado.
func TestTruncateToTokens_TextoCurto(t *testing.T) {
	text := "curto"
	got := truncateToTokens(text, 100)
	if got != text {
		t.Errorf("esperado %q, obtido %q", text, got)
	}
}

// TestTruncateToTokens_Zero verifica que maxTokens zero retorna vazio.
func TestTruncateToTokens_Zero(t *testing.T) {
	got := truncateToTokens("texto qualquer", 0)
	if got != "" {
		t.Errorf("esperado vazio, obtido %q", got)
	}
}
