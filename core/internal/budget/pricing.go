package budget

import "strings"

type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

var pricing = map[string]ModelPricing{
	// Claude models (per million tokens)
	"claude-opus-4-5-20251101":   {15.00, 75.00},
	"claude-sonnet-4-20250514":   {3.00, 15.00},
	"claude-haiku-3-5-20241022":  {0.80, 4.00},

	// OpenAI models
	"gpt-4o":          {2.50, 10.00},
	"gpt-4o-mini":     {0.15, 0.60},
	"gpt-4-turbo":     {10.00, 30.00},
	"o1":              {15.00, 60.00},
	"o1-mini":         {3.00, 12.00},

	// Kimi models (estimated, adjust as needed)
	"kimi-k2-0711-preview": {1.00, 4.00},
	"kimi-k2.5:cloud":      {1.50, 6.00},

	// Ollama/local models (free)
	"ollama": {0, 0},
}

func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	p, ok := pricing[model]
	if !ok {
		// Try prefix matching for ollama models
		if strings.HasPrefix(model, "ollama/") || strings.Contains(model, ":") {
			return 0
		}
		// Unknown model, use conservative estimate
		p = ModelPricing{5.00, 15.00}
	}

	inputCost := float64(inputTokens) * p.InputPerMillion / 1_000_000
	outputCost := float64(outputTokens) * p.OutputPerMillion / 1_000_000

	return inputCost + outputCost
}

func GetPricing(model string) (input, output float64, found bool) {
	p, ok := pricing[model]
	if !ok {
		return 0, 0, false
	}
	return p.InputPerMillion, p.OutputPerMillion, true
}
