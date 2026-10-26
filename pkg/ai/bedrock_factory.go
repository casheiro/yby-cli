//go:build aws

package ai

import (
	"context"
	"regexp"
)

// bedrockModelPattern aceita os IDs de modelo do Bedrock: fornecedor.modelo (com a versão
// depois de ":"), com ou sem o prefixo de região dos perfis de inferência ("us.").
var bedrockModelPattern = regexp.MustCompile(`^((us|eu|apac|global)\.)?(anthropic|amazon|meta|mistral|cohere|ai21)\.[a-z0-9][a-z0-9.:-]*$`)

func init() {
	registerProvider("bedrock", func(ctx context.Context, model string) Provider {
		p := NewBedrockProvider()
		if p != nil && model != "" {
			p.Model = model
		}
		if p != nil && p.IsAvailable(ctx) {
			return wrapProvider(p, p.Model)
		}
		return nil
	})
	// Os modelos que o código já conhece (tokencount.go); fora deles, aceita IDs no formato
	// fornecedor.modelo dos fornecedores do Bedrock, como os do console.
	registerModelSpec("bedrock", modelSpec{
		defaultModel: defaultBedrockModel,
		known:        []string{"anthropic.claude-3-5-sonnet-20241022-v2:0", "anthropic.claude-3-haiku-20240307-v1:0"},
		accepts:      bedrockModelPattern.MatchString,
	})
}
