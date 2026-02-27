package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Embedder struct {
	baseURL string
	model   string
}

type request struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type response struct {
	Embedding []float32 `json:"embedding"`
}

func NewEmbedder(baseURL, model string) *Embedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &Embedder{
		baseURL: baseURL,
		model:   model,
	}
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := request{
		Model:  e.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp response
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, err
	}

	return ollamaResp.Embedding, nil
}
