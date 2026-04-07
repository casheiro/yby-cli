package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
)

// ClassifyIntent usa a IA para classificar a intenção do usuário.
// Chamada rápida e focada — retorna JSON com intent + params.
func ClassifyIntent(ctx context.Context, provider ai.Provider, input string) *tools.IntentResult {
	allTools := tools.All()
	var intentList strings.Builder
	for _, t := range allTools {
		if len(t.Intents) > 0 {
			intentList.WriteString(fmt.Sprintf("- %s: %s\n", t.Intents[0], t.Description))
		}
	}
	intentList.WriteString("- direct: responder diretamente sem executar ferramenta\n")

	classifyPrompt := prompts.Get("bard.classify")
	if classifyPrompt == "" {
		classifyPrompt = `Classifique a intencao. Responda APENAS JSON:
{"intent":"nome","params":{"chave":"valor"},"direct":false}
Se nao precisa de ferramenta: {"intent":"direct","params":{},"direct":true}`
	}

	userPrompt := fmt.Sprintf("Intencoes:\n%s\nUsuario: %s", intentList.String(), input)

	result, err := provider.Completion(ctx, classifyPrompt, userPrompt)
	if err != nil {
		return &tools.IntentResult{Direct: true}
	}

	var intent tools.IntentResult
	clean := strings.TrimSpace(result)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	if err := json.Unmarshal([]byte(clean), &intent); err != nil {
		return &tools.IntentResult{Direct: true}
	}

	return &intent
}
