package main

import (
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// rotulosProvider liga o começo do Name() de cada provider ao nome curto que o
// usuário escreve em ai.provider no config.yaml. A ordem importa: "Gemini CLI"
// vem antes de qualquer outro Gemini.
var rotulosProvider = []struct{ prefixo, rotulo string }{
	{"claude code cli", "claude-cli"},
	{"gemini cli", "gemini-cli"},
	{"ollama", "ollama"},
	{"google gemini", "gemini"},
	{"openai", "openai"},
	{"aws bedrock", "bedrock"},
}

// rotuloProvider devolve o nome curto do provider para o cabeçalho do Bard
// (ex.: "claude-cli"). Um provider desconhecido aparece pelo próprio nome, em
// minúsculas; nil vira vazio.
func rotuloProvider(p ai.Provider) string {
	if p == nil {
		return ""
	}
	nome := strings.ToLower(strings.TrimSpace(p.Name()))
	for _, r := range rotulosProvider {
		if strings.HasPrefix(nome, r.prefixo) {
			return r.rotulo
		}
	}
	return nome
}
