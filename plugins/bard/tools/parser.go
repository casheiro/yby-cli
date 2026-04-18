package tools

import (
	"encoding/json"
	"regexp"
	"strings"
)

// jsonBlockPattern captura blocos JSON com campo "tool" em code fences ou inline.
var jsonBlockPattern = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{[^`]*?\"tool\"[^`]*?\\})\\s*\\n?```")

// inlineJSONPattern captura objetos JSON inline com campo "tool".
var inlineJSONPattern = regexp.MustCompile(`(\{"tool"\s*:\s*"[^"]+"\s*,\s*"params"\s*:\s*\{[^}]*\}\s*\})`)

// ParseToolCalls extrai tool calls da resposta da IA.
// Retorna os tool calls encontrados e o texto restante (sem os blocos JSON).
func ParseToolCalls(response string) ([]ToolCall, string) {
	var calls []ToolCall
	remaining := response

	// Primeiro: buscar em code fences (```json ... ```)
	matches := jsonBlockPattern.FindAllStringSubmatchIndex(remaining, -1)
	if len(matches) > 0 {
		// Processar de trás para frente para manter índices válidos
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			fullStart, fullEnd := match[0], match[1]
			jsonStart, jsonEnd := match[2], match[3]

			jsonStr := remaining[jsonStart:jsonEnd]
			var call ToolCall
			if err := json.Unmarshal([]byte(jsonStr), &call); err == nil && call.Name != "" {
				calls = append([]ToolCall{call}, calls...)
				remaining = remaining[:fullStart] + remaining[fullEnd:]
			}
		}
	}

	// Segundo: buscar JSON inline (sem code fence)
	inlineMatches := inlineJSONPattern.FindAllStringSubmatchIndex(remaining, -1)
	if len(inlineMatches) > 0 {
		for i := len(inlineMatches) - 1; i >= 0; i-- {
			match := inlineMatches[i]
			fullStart, fullEnd := match[0], match[1]
			jsonStart, jsonEnd := match[2], match[3]

			jsonStr := remaining[jsonStart:jsonEnd]
			var call ToolCall
			if err := json.Unmarshal([]byte(jsonStr), &call); err == nil && call.Name != "" {
				calls = append([]ToolCall{call}, calls...)
				remaining = remaining[:fullStart] + remaining[fullEnd:]
			}
		}
	}

	remaining = strings.TrimSpace(remaining)
	return calls, remaining
}
