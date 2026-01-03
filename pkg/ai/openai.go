package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	// Support Anthropic/Other via Compatible Base URL if needed, but defaults to OpenAI
	return &OpenAIProvider{
		APIKey:  apiKey,
		Model:   "gpt-4o-mini", // Smart and fast
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
}

func (p *OpenAIProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	reqBody := openAIRequest{
		Model: p.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Project Description: %s", description)},
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
		return nil, fmt.Errorf("failed to call openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("openai returned status: %d - %s", resp.StatusCode, buf.String())
	}

	var oResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("failed to decode openai response: %w", err)
	}

	if len(oResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from openai")
	}

	cleanJSON := oResp.Choices[0].Message.Content
	// OpenAI with json_object usually returns clean JSON, but safeguard
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var blueprint GovernanceBlueprint
	if err := json.Unmarshal([]byte(cleanJSON), &blueprint); err != nil {
		return nil, fmt.Errorf("failed to parse blueprint json: %w", err)
	}

	return &blueprint, nil
}
