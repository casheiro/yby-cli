package ai

import "context"

// GetBestProvider returns the first available AI provider (Local > Gemini > OpenAI)
func GetBestProvider(ctx context.Context) Provider {
	// 1. Prefer Local Inference (Privacy & Cost)
	ollama := NewOllamaProvider()
	if ollama.IsAvailable(ctx) {
		return ollama
	}

	// 2. Google Gemini (Fast & Generous Free Tier)
	gemini := NewGeminiProvider()
	if gemini != nil && gemini.IsAvailable(ctx) {
		return gemini
	}

	// 3. OpenAI (Standard)
	openai := NewOpenAIProvider()
	if openai != nil && openai.IsAvailable(ctx) {
		return openai
	}

	return nil
}
