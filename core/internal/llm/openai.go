package llm

import (
	"bytes"
	"context"
	"encoding/base64"
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
	Tools    []openaiTool    `json:"tools,omitempty"`
}

type openaiContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *openaiMediaURL `json:"image_url,omitempty"`
	VideoURL *openaiMediaURL `json:"video_url,omitempty"`
}

type openaiMediaURL struct {
	URL string `json:"url"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
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
	resp, err := o.ChatWithTools(ctx, systemPrompt, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (o *openaiCompatible) ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*ChatResponse, error) {
	var oaiMessages []openaiMessage

	if systemPrompt != "" {
		oaiMessages = append(oaiMessages, openaiMessage{Role: "system", Content: systemPrompt})
	}

	for _, msg := range messages {
		oaiMsg := openaiMessage{
			Role:       msg.Role,
			ToolCallID: msg.ToolCallID,
		}

		if len(msg.Media) > 0 {
			var parts []openaiContentPart
			for _, media := range msg.Media {
				dataURL := fmt.Sprintf("data:%s;base64,%s", media.MimeType, base64.StdEncoding.EncodeToString(media.Data))
				switch media.Type {
				case MediaTypeImage:
					parts = append(parts, openaiContentPart{Type: "image_url", ImageURL: &openaiMediaURL{URL: dataURL}})
				case MediaTypeVideo:
					parts = append(parts, openaiContentPart{Type: "video_url", VideoURL: &openaiMediaURL{URL: dataURL}})
				}
			}
			if msg.Content != "" {
				parts = append(parts, openaiContentPart{Type: "text", Text: msg.Content})
			}
			oaiMsg.Content = parts
		} else {
			oaiMsg.Content = msg.Content
		}

		for _, tc := range msg.ToolCalls {
			oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}

		oaiMessages = append(oaiMessages, oaiMsg)
	}

	reqBody := openaiRequest{
		Model:    o.model,
		Messages: oaiMessages,
	}

	if len(tools) > 0 {
		reqBody.Tools = o.convertTools(tools)
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var oaiResp openaiResponse

	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := oaiResp.Choices[0]
	result := &ChatResponse{
		Content:    choice.Message.Content,
		StopReason: choice.FinishReason,
	}

	if oaiResp.Usage != nil {
		result.Usage = &Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		}
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return result, nil
}

func (o *openaiCompatible) convertTools(tools []Tool) []openaiTool {
	result := make([]openaiTool, len(tools))

	for i, tool := range tools {
		params := tool.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}

		result[i] = openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return result
}
