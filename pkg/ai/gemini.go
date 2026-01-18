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

type GeminiProvider struct {
	APIKey string
	Model  string
}

func NewGeminiProvider() *GeminiProvider {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash" // Fallback to stable v1.0 Pro if not specified
	}

	return &GeminiProvider{
		APIKey: apiKey,
		Model:  model,
	}
}

func (p *GeminiProvider) Name() string {
	return "Google Gemini (Cloud)"
}

func (p *GeminiProvider) IsAvailable(ctx context.Context) bool {
	return p.APIKey != ""
}

// Gemini Request Structure
type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiConfig    `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiConfig struct {
	ResponseMimeType string `json:"responseMimeType"`
}

// Gemini Response Structure
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (p *GeminiProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.Model, p.APIKey)

	fullPrompt := fmt.Sprintf("%s\n\nDESCRIÇÃO DO PROJETO: %s", SystemPrompt, description)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: fullPrompt},
				},
			},
		},
		GenerationConfig: geminiConfig{
			ResponseMimeType: "application/json",
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 60 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Read body for error details
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			return nil, fmt.Errorf("gemini retornou status: %d (falha ao ler corpo: %v)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("gemini retornou status: %d - %s", resp.StatusCode, buf.String())
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta do gemini: %w", err)
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("resposta vazia do gemini")
	}

	rawJSON := gResp.Candidates[0].Content.Parts[0].Text
	// Clean markdown fences if present (Gemini sometimes adds them even with mimeType set)
	cleanJSON := strings.TrimPrefix(rawJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```") // sometimes just ```
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var blueprint GovernanceBlueprint
	if err := json.Unmarshal([]byte(cleanJSON), &blueprint); err != nil {
		return nil, fmt.Errorf("falha ao analisar json do blueprint: %w", err)
	}

	return &blueprint, nil
}

func (p *GeminiProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.Model, p.APIKey)

	// Context + User Prompt
	fullPrompt := fmt.Sprintf("%s\n\nUSER PROMPT: %s", systemPrompt, userPrompt)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: fullPrompt},
				},
			},
		},
		// No specific response mime type force, let it be text
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 60 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("falha ao chamar gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gemini returned status: %d", resp.StatusCode)
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return "", fmt.Errorf("falha ao decodificar resposta do gemini: %w", err)
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia do gemini")
	}

	return gResp.Candidates[0].Content.Parts[0].Text, nil
}

func (p *GeminiProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	// Fallback to non-streaming for now to satisfy interface
	text, err := p.Completion(ctx, systemPrompt, userPrompt)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
