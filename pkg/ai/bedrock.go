//go:build aws

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
)

// BedrockClient define a interface do cliente Bedrock para facilitar testes com mock.
type BedrockClient interface {
	Converse(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
	ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
	InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// BedrockProvider implementa a interface Provider usando Amazon Bedrock.
type BedrockProvider struct {
	client BedrockClient
	Model  string
	Region string
}

// NewBedrockProvider cria um BedrockProvider configurado com credenciais AWS.
// Retorna nil se não for possível carregar a configuração AWS.
func NewBedrockProvider() *BedrockProvider {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	model := "anthropic.claude-3-5-sonnet-20241022-v2:0"
	if override := getConfiguredModel("bedrock"); override != "" {
		model = override
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil
	}

	client := bedrockruntime.NewFromConfig(cfg)

	return &BedrockProvider{
		client: client,
		Model:  model,
		Region: region,
	}
}

// newBedrockProviderWithClient cria um BedrockProvider com um client customizado (para testes).
func newBedrockProviderWithClient(client BedrockClient, model, region string) *BedrockProvider {
	return &BedrockProvider{
		client: client,
		Model:  model,
		Region: region,
	}
}

func (p *BedrockProvider) Name() string {
	return "AWS Bedrock (Cloud)"
}

func (p *BedrockProvider) IsAvailable(ctx context.Context) bool {
	_, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	return err == nil
}

func (p *BedrockProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(p.Model),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		},
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: userPrompt},
				},
			},
		},
	}

	output, err := p.client.Converse(ctx, input)
	if err != nil {
		return "", fmt.Errorf("falha ao chamar bedrock converse: %w", err)
	}

	if output.Usage != nil {
		SetUsage(ctx, &UsageMetadata{
			PromptTokens:     int(aws.ToInt32(output.Usage.InputTokens)),
			CompletionTokens: int(aws.ToInt32(output.Usage.OutputTokens)),
			TotalTokens:      int(aws.ToInt32(output.Usage.TotalTokens)),
			Provider:         "bedrock",
			Model:            p.Model,
			Operation:        "completion",
		})
	}

	if output.Output == nil {
		return "", fmt.Errorf("resposta vazia do bedrock")
	}

	msg, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok || len(msg.Value.Content) == 0 {
		return "", fmt.Errorf("resposta vazia do bedrock")
	}

	textBlock, ok := msg.Value.Content[0].(*types.ContentBlockMemberText)
	if !ok {
		return "", fmt.Errorf("resposta do bedrock não contém texto")
	}

	return textBlock.Value, nil
}

func (p *BedrockProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	input := &bedrockruntime.ConverseStreamInput{
		ModelId: aws.String(p.Model),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		},
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: userPrompt},
				},
			},
		},
	}

	output, err := p.client.ConverseStream(ctx, input)
	if err != nil {
		return fmt.Errorf("falha ao chamar bedrock converse stream: %w", err)
	}

	stream := output.GetStream()
	defer stream.Close()

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if delta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				if _, err := io.WriteString(out, delta.Value); err != nil {
					return err
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("erro no stream do bedrock: %w", err)
	}

	return nil
}

// titanEmbeddingRequest é o payload para o modelo Titan Embeddings.
type titanEmbeddingRequest struct {
	InputText string `json:"inputText"`
}

// titanEmbeddingResponse é a resposta do modelo Titan Embeddings.
type titanEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func getEmbeddingModelForBedrock() string {
	if configured := GetEmbeddingModel("bedrock"); configured != "" {
		return configured
	}
	return "amazon.titan-embed-text-v2:0"
}

func (p *BedrockProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	embeddingModel := getEmbeddingModelForBedrock()
	results := make([][]float32, len(texts))

	for i, text := range texts {
		reqBody, err := json.Marshal(titanEmbeddingRequest{InputText: text})
		if err != nil {
			return nil, fmt.Errorf("falha ao serializar request de embedding: %w", err)
		}

		input := &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(embeddingModel),
			ContentType: aws.String("application/json"),
			Body:        reqBody,
		}

		output, err := p.client.InvokeModel(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("falha ao chamar bedrock invoke model para embedding: %w", err)
		}

		var resp titanEmbeddingResponse
		if err := json.Unmarshal(output.Body, &resp); err != nil {
			return nil, fmt.Errorf("falha ao decodificar resposta de embedding do bedrock: %w", err)
		}

		results[i] = resp.Embedding
	}

	return results, nil
}

func (p *BedrockProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	result, err := p.Completion(ctx, prompts.Get("governance.system"), fmt.Sprintf("Descrição do Projeto: %s", description))
	if err != nil {
		return nil, err
	}

	var blueprint GovernanceBlueprint
	if err := json.Unmarshal([]byte(result), &blueprint); err != nil {
		return nil, fmt.Errorf("falha ao analisar json do blueprint: %w", err)
	}

	return &blueprint, nil
}
