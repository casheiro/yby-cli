//go:build aws

package ai

import "context"

func init() {
	registerProvider("bedrock", func(ctx context.Context) Provider {
		p := NewBedrockProvider()
		if p != nil && p.IsAvailable(ctx) {
			return wrapProvider(p, p.Model)
		}
		return nil
	})
}
