//go:build aws

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// mockBedrockClient implementa BedrockClient para testes.
type mockBedrockClient struct {
	converseFunc       func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
	converseStreamFunc func(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
	invokeModelFunc    func(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

func (m *mockBedrockClient) Converse(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	return m.converseFunc(ctx, params, optFns...)
}

func (m *mockBedrockClient) ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error) {
	return m.converseStreamFunc(ctx, params, optFns...)
}

func (m *mockBedrockClient) InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	return m.invokeModelFunc(ctx, params, optFns...)
}

func TestBedrockProvider_Name(t *testing.T) {
	p := newBedrockProviderWithClient(&mockBedrockClient{}, "test-model", "us-east-1")
	if got := p.Name(); got != "AWS Bedrock (Cloud)" {
		t.Errorf("Name() = %q, esperado %q", got, "AWS Bedrock (Cloud)")
	}
}

func TestBedrockProvider_IsAvailable(t *testing.T) {
	// IsAvailable tenta carregar config AWS — em ambiente de teste sem credenciais
	// deve funcionar (LoadDefaultConfig não falha sem credenciais, só falha na chamada real)
	p := newBedrockProviderWithClient(&mockBedrockClient{}, "test-model", "us-east-1")
	ctx := context.Background()
	// Não falha porque LoadDefaultConfig encontra config default
	_ = p.IsAvailable(ctx)
}

func TestBedrockProvider_Completion(t *testing.T) {
	mock := &mockBedrockClient{
		converseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			inputTokens := int32(10)
			outputTokens := int32(20)
			totalTokens := int32(30)
			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{Value: "resposta do bedrock"},
						},
					},
				},
				Usage: &types.TokenUsage{
					InputTokens:  &inputTokens,
					OutputTokens: &outputTokens,
					TotalTokens:  &totalTokens,
				},
			}, nil
		},
	}

	p := newBedrockProviderWithClient(mock, "anthropic.claude-3-5-sonnet-20241022-v2:0", "us-east-1")
	ctx := context.Background()

	result, err := p.Completion(ctx, "sistema", "usuario")
	if err != nil {
		t.Fatalf("Completion() erro inesperado: %v", err)
	}
	if result != "resposta do bedrock" {
		t.Errorf("Completion() = %q, esperado %q", result, "resposta do bedrock")
	}
}

func TestBedrockProvider_Completion_EmptyResponse(t *testing.T) {
	mock := &mockBedrockClient{
		converseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			return &bedrockruntime.ConverseOutput{
				Output: nil,
			}, nil
		},
	}

	p := newBedrockProviderWithClient(mock, "test-model", "us-east-1")
	ctx := context.Background()

	_, err := p.Completion(ctx, "sistema", "usuario")
	if err == nil {
		t.Fatal("Completion() deveria retornar erro para resposta vazia")
	}
}

func TestBedrockProvider_EmbedDocuments(t *testing.T) {
	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	mock := &mockBedrockClient{
		invokeModelFunc: func(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
			// Verificar que o request está correto
			var req titanEmbeddingRequest
			if err := json.Unmarshal(params.Body, &req); err != nil {
				t.Fatalf("falha ao decodificar request: %v", err)
			}
			if req.InputText == "" {
				t.Error("InputText não deveria estar vazio")
			}

			resp, _ := json.Marshal(titanEmbeddingResponse{Embedding: embedding})
			return &bedrockruntime.InvokeModelOutput{
				Body: resp,
			}, nil
		},
	}

	p := newBedrockProviderWithClient(mock, "test-model", "us-east-1")
	ctx := context.Background()

	results, err := p.EmbedDocuments(ctx, []string{"texto 1", "texto 2"})
	if err != nil {
		t.Fatalf("EmbedDocuments() erro inesperado: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("EmbedDocuments() retornou %d resultados, esperado 2", len(results))
	}
	if len(results[0]) != len(embedding) {
		t.Errorf("EmbedDocuments()[0] tem %d dimensões, esperado %d", len(results[0]), len(embedding))
	}
}

func TestBedrockProvider_GenerateGovernance(t *testing.T) {
	blueprint := GovernanceBlueprint{
		Domain:    "infraestrutura",
		RiskLevel: "alto",
		Summary:   "resumo do projeto",
		Files:     []GeneratedFile{{Path: "test.md", Content: "conteudo"}},
	}
	blueprintJSON, _ := json.Marshal(blueprint)

	mock := &mockBedrockClient{
		converseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{Value: string(blueprintJSON)},
						},
					},
				},
			}, nil
		},
	}

	p := newBedrockProviderWithClient(mock, "test-model", "us-east-1")
	ctx := context.Background()

	result, err := p.GenerateGovernance(ctx, "projeto de teste")
	if err != nil {
		t.Fatalf("GenerateGovernance() erro inesperado: %v", err)
	}
	if result.Domain != "infraestrutura" {
		t.Errorf("GenerateGovernance().Domain = %q, esperado %q", result.Domain, "infraestrutura")
	}
}

func TestBedrockProvider_StreamCompletion(t *testing.T) {
	// StreamCompletion precisa de um stream real que é difícil de mockar
	// sem a implementação interna do SDK. Verificamos apenas que o provider
	// chama o método correto.
	called := false
	mock := &mockBedrockClient{
		converseStreamFunc: func(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error) {
			called = true
			// Retornar erro controlado para verificar que foi chamado
			return nil, fmt.Errorf("mock: stream não implementado em teste")
		},
	}

	p := newBedrockProviderWithClient(mock, "test-model", "us-east-1")
	ctx := context.Background()
	var buf bytes.Buffer

	_ = p.StreamCompletion(ctx, "sistema", "usuario", &buf)
	if !called {
		t.Error("StreamCompletion() não chamou ConverseStream")
	}
}
