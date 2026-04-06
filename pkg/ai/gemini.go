package ai

import (
	"bufio"
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
	APIKey  string
	Model   string
	BaseURL string
}

func NewGeminiProvider() *GeminiProvider {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil
	}

	// Precedência: config global > GEMINI_MODEL env var > default
	model := "gemini-2.5-flash"
	if override := getConfiguredModel(); override != "" {
		model = override
	} else if envModel := os.Getenv("GEMINI_MODEL"); envModel != "" {
		model = envModel
	}

	return &GeminiProvider{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://generativelanguage.googleapis.com",
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
	UsageMetadataResponse struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (p *GeminiProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.BaseURL, p.Model, p.APIKey)

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
		body, _ := io.ReadAll(resp.Body)
		return nil, NewAPIErrorFromResponse("gemini", resp, body)
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta do gemini: %w", err)
	}

	SetUsage(ctx, &UsageMetadata{
		PromptTokens:     gResp.UsageMetadataResponse.PromptTokenCount,
		CompletionTokens: gResp.UsageMetadataResponse.CandidatesTokenCount,
		TotalTokens:      gResp.UsageMetadataResponse.TotalTokenCount,
		Provider:         "gemini",
		Model:            p.Model,
		Operation:        "governance",
	})

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
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.BaseURL, p.Model, p.APIKey)

	// Context + User Prompt
	lang := GetLanguage()
	systemPrompt = fmt.Sprintf("%s\n\n(IMPORTANT: You MUST output your analysis/response entirely in %s language, maintaining the JSON structure if requested.)", systemPrompt, lang)
	fullPrompt := fmt.Sprintf("%s\n\nUSER PROMPT: %s", systemPrompt, userPrompt)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := http.Client{Timeout: 60 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("falha ao chamar gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", NewAPIErrorFromResponse("gemini", resp, bodyBytes)
	}

	var gResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return "", fmt.Errorf("falha ao decodificar resposta do gemini: %w", err)
	}

	SetUsage(ctx, &UsageMetadata{
		PromptTokens:     gResp.UsageMetadataResponse.PromptTokenCount,
		CompletionTokens: gResp.UsageMetadataResponse.CandidatesTokenCount,
		TotalTokens:      gResp.UsageMetadataResponse.TotalTokenCount,
		Provider:         "gemini",
		Model:            p.Model,
		Operation:        "completion",
	})

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia do gemini")
	}

	return gResp.Candidates[0].Content.Parts[0].Text, nil
}

func (p *GeminiProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	lang := GetLanguage()
	systemPrompt = fmt.Sprintf("%s\n\n(IMPORTANT: You MUST output your analysis/response entirely in %s language, maintaining the JSON structure if requested.)", systemPrompt, lang)
	fullPrompt := fmt.Sprintf("%s\n\nUSER PROMPT: %s", systemPrompt, userPrompt)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: fullPrompt}}},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("falha ao serializar request gemini: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		p.BaseURL, p.Model, p.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("falha ao criar request de streaming gemini: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao chamar streaming gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return NewAPIErrorFromResponse("gemini", resp, body)
	}

	// Parser SSE — Gemini retorna data: {json}\n\n (sem sentinel [DONE], termina com EOF)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk geminiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // pular linhas mal-formadas
		}
		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			text := chunk.Candidates[0].Content.Parts[0].Text
			if _, err := io.WriteString(out, text); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

type geminiEmbeddingRequest struct {
	Model   string                 `json:"model"`
	Content geminiEmbeddingContent `json:"content"`
}

type geminiEmbeddingContent struct {
	Parts []geminiPart `json:"parts"`
}

// geminiEmbeddingResponse was unused and removed

type geminiBatchEmbeddingRequest struct {
	Requests []geminiEmbeddingRequest `json:"requests"`
}

type geminiBatchEmbeddingResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

func (p *GeminiProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddingModel := "text-embedding-004"
	url := fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents?key=%s", p.BaseURL, embeddingModel, p.APIKey)

	const batchSize = 100
	var allEmbeddings [][]float32

	// Process in batches
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[i:end]
		requests := make([]geminiEmbeddingRequest, len(batchTexts))

		for j, text := range batchTexts {
			requests[j] = geminiEmbeddingRequest{
				Model: "models/" + embeddingModel,
				Content: geminiEmbeddingContent{
					Parts: []geminiPart{{Text: text}},
				},
			}
		}

		batchReq := geminiBatchEmbeddingRequest{Requests: requests}
		jsonBody, _ := json.Marshal(batchReq)

		client := http.Client{Timeout: 60 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("falha ao chamar gemini embeddings (batch %d): %w", i, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, NewAPIErrorFromResponse("gemini", resp, body)
		}

		var batchResp geminiBatchEmbeddingResponse
		if err := json.Unmarshal(body, &batchResp); err != nil {
			return nil, fmt.Errorf("falha ao decodificar batch %d: %w", i, err)
		}

		if len(batchResp.Embeddings) != len(batchTexts) {
			// In case of error or mismatch, we should probably error out or pad with zeros?
			// Returning error is safer to detect data loss.
			return nil, fmt.Errorf("mismatch in batch %d: sent %d, got %d", i, len(batchTexts), len(batchResp.Embeddings))
		}

		for _, emb := range batchResp.Embeddings {
			allEmbeddings = append(allEmbeddings, emb.Values)
		}
	}

	return allEmbeddings, nil
}
