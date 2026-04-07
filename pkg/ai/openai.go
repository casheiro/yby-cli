package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIProvider struct {
	APIKey  string
	Model   string
	BaseURL string
}

func NewOpenAIProvider() *OpenAIProvider {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil
	}

	model := "gpt-4o-mini" // Default: rápido e econômico
	if override := getConfiguredModel("openai"); override != "" {
		model = override
	}

	return &OpenAIProvider{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.openai.com/v1",
	}
}

func (p *OpenAIProvider) Name() string {
	return "OpenAI (Cloud)"
}

func (p *OpenAIProvider) IsAvailable(ctx context.Context) bool {
	return p.APIKey != ""
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *OpenAIProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	reqBody := openAIRequest{
		Model: p.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Descrição do Projeto: %s", description)},
		},
	}
	reqBody.ResponseFormat.Type = "json_object"

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, NewAPIErrorFromResponse("openai", resp, body)
	}

	var oResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta da openai: %w", err)
	}

	SetUsage(ctx, &UsageMetadata{
		PromptTokens:     oResp.Usage.PromptTokens,
		CompletionTokens: oResp.Usage.CompletionTokens,
		TotalTokens:      oResp.Usage.TotalTokens,
		Provider:         "openai",
		Model:            p.Model,
		Operation:        "governance",
	})

	if len(oResp.Choices) == 0 {
		return nil, fmt.Errorf("resposta vazia do openai")
	}

	cleanJSON := oResp.Choices[0].Message.Content
	// OpenAI with json_object usually returns clean JSON, but safeguard
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var blueprint GovernanceBlueprint
	if err := json.Unmarshal([]byte(cleanJSON), &blueprint); err != nil {
		return nil, fmt.Errorf("falha ao analisar json do blueprint: %w", err)
	}

	return &blueprint, nil
}

func (p *OpenAIProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := openAIRequest{
		Model: p.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	// No JSON format constraint for general completion

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha ao chamar openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", NewAPIErrorFromResponse("openai", resp, body)
	}

	var oResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return "", fmt.Errorf("falha ao decodificar resposta da openai: %w", err)
	}

	SetUsage(ctx, &UsageMetadata{
		PromptTokens:     oResp.Usage.PromptTokens,
		CompletionTokens: oResp.Usage.CompletionTokens,
		TotalTokens:      oResp.Usage.TotalTokens,
		Provider:         "openai",
		Model:            p.Model,
		Operation:        "completion",
	})

	if len(oResp.Choices) == 0 {
		return "", fmt.Errorf("resposta vazia da openai")
	}

	return oResp.Choices[0].Message.Content, nil
}

// StreamCompletion implements a simple SSE reader for OpenAI
func (p *OpenAIProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	type streamRequest struct {
		Model    string          `json:"model"`
		Messages []openAIMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}

	reqStruct := streamRequest{
		Model: p.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: true,
	}

	jsonBody, _ := json.Marshal(reqStruct)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao chamar openai stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return NewAPIErrorFromResponse("openai", resp, body)
	}

	// Simple SSE Parser
	// Reads line by line. Looks for "data: "
	// "data: [DONE]" -> finish
	// "data: {...}" -> parse choices[0].delta.content
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			lines := strings.Split(chunk, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					if data == "[DONE]" {
						return nil
					}
					// Parse JSON
					var steamResp struct {
						Choices []struct {
							Delta struct {
								Content string `json:"content"`
							} `json:"delta"`
						} `json:"choices"`
					}
					if err := json.Unmarshal([]byte(data), &steamResp); err == nil {
						if len(steamResp.Choices) > 0 {
							content := steamResp.Choices[0].Delta.Content
							if content != "" {
								if _, err := io.WriteString(out, content); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

type openAIEmbeddingRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func getEmbeddingModelForOpenAI() string {
	if configured := GetEmbeddingModel("openai"); configured != "" {
		return configured
	}
	return "text-embedding-3-small"
}

func (p *OpenAIProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := openAIEmbeddingRequest{
		Input:          texts,
		Model:          getEmbeddingModelForOpenAI(),
		EncodingFormat: "float",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/embeddings", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar openai embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, NewAPIErrorFromResponse("openai", resp, body)
	}

	var oResp openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, err
	}

	// Sort by index just in case, though usually ordered
	results := make([][]float32, len(texts))
	for _, data := range oResp.Data {
		if data.Index < len(results) {
			results[data.Index] = data.Embedding
		}
	}
	return results, nil
}
