package ai

import "context"

// GetProvider returns the requested AI provider or defaults to the best available.
// preferred: "ollama", "gemini", "openai"
func GetProvider(ctx context.Context, preferred string) Provider {
	// 1. Explicit Preference
	if preferred != "" && preferred != "auto" {
		switch preferred {
		case "ollama":
			p := NewOllamaProvider()
			if p.IsAvailable(ctx) {
				return p
			}
		case "gemini":
			p := NewGeminiProvider()
			if p != nil && p.IsAvailable(ctx) {
				return p
			}
		case "openai":
			p := NewOpenAIProvider()
			if p != nil && p.IsAvailable(ctx) {
				return p
			}
		}
		// If explicit preference is not available, return nil (Strict)
		return nil
	}

	// 2. Auto-Detect: Prefer Local Inference (Privacy & Cost)
	ollama := NewOllamaProvider()
	if ollama.IsAvailable(ctx) {
		return ollama
	}

	// 3. Google Gemini (Fast & Generous Free Tier)
	gemini := NewGeminiProvider()
	if gemini != nil && gemini.IsAvailable(ctx) {
		return gemini
	}

	// 4. OpenAI (Standard)
	openai := NewOpenAIProvider()
	if openai != nil && openai.IsAvailable(ctx) {
		return openai
	}

	return nil
}
