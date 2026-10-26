//go:build aws

package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModels_Bedrock(t *testing.T) {
	isolarConfig(t)
	cat := ListModels(context.Background(), "bedrock")
	assert.Equal(t, "anthropic.claude-3-5-sonnet-20241022-v2:0", cat.Default)
	assert.Equal(t, []string{"anthropic.claude-3-5-sonnet-20241022-v2:0", "anthropic.claude-3-haiku-20240307-v1:0"}, cat.Models)
}

// A regra do Bedrock aceita só IDs fornecedor.modelo dos fornecedores dele; os nomes de
// outros providers voltam recusados, sem chamar a IA.
func TestValidarModelo_Bedrock(t *testing.T) {
	isolarConfig(t)
	ctx := context.Background()
	for _, aceito := range []string{
		"anthropic.claude-3-haiku-20240307-v1:0",
		"amazon.titan-text-express-v1",
		"us.anthropic.claude-3-5-sonnet-20241022-v2:0",
		"meta.llama3-8b-instruct-v1:0",
	} {
		modelo, err := validarModelo(ctx, "bedrock", aceito)
		require.NoError(t, err, aceito)
		assert.Equal(t, aceito, modelo)
	}
	for _, recusado := range []string{"gemini-2.5-flash", "gpt-4.1", "opus", "openai.gpt-4o", "anthropic."} {
		_, err := validarModelo(ctx, "bedrock", recusado)
		ye := codigoErro(t, err)
		assert.Equal(t, "o bedrock não tem o modelo "+recusado, ye.Message)
		assert.Contains(t, ye.Hint, "anthropic.claude-3-5-sonnet-20241022-v2:0")
	}
}
