package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type openaiCompatible struct {
	apiKey  string
	baseURL string
	model   string
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func newOpenAICompatible(apiKey, baseURL, model string) LLM {
	return &openaiCompatible{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}

func (o *openaiCompatible) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	var oaiMessages []openaiMessage

	if systemPrompt != "" {
		oaiMessages = append(oaiMessages, openaiMessage{Role: "system", Content: systemPrompt})
	}

	for _, msg := range messages {
		oaiMessages = append(oaiMessages, openaiMessage{Role: msg.Role, Content: msg.Content})
	}

	reqBody := openaiRequest{
		Model:    o.model,
		Messages: oaiMessages,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var oaiResp openaiResponse

	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	if oaiResp.Error != nil {
		return "", fmt.Errorf("api error: %s", oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return oaiResp.Choices[0].Message.Content, nil
}
