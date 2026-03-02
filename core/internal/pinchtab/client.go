package pinchtab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

type Instance struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
	Status  string `json:"status"`
}

type Snapshot struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
}

type Element struct {
	Ref  string `json:"ref"`
	Role string `json:"role"`
	Name string `json:"name"`
	Text string `json:"text,omitempty"`
}

type Action struct {
	Type  string `json:"type"`
	Ref   string `json:"ref,omitempty"`
	Value string `json:"value,omitempty"`
	Key   string `json:"key,omitempty"`
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:9867"
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) CreateInstance(ctx context.Context, profile string) (*Instance, error) {
	body := map[string]string{"profile": profile}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/instances", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinchtab unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create instance failed: %s", string(body))
	}

	var instance Instance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, err
	}
	return &instance, nil
}

func (c *Client) Navigate(ctx context.Context, instanceID, url string) error {
	action := Action{Type: "navigate", Value: url}
	return c.doAction(ctx, instanceID, action)
}

func (c *Client) Click(ctx context.Context, instanceID, ref string) error {
	action := Action{Type: "click", Ref: ref}
	return c.doAction(ctx, instanceID, action)
}

func (c *Client) Fill(ctx context.Context, instanceID, ref, value string) error {
	action := Action{Type: "fill", Ref: ref, Value: value}
	return c.doAction(ctx, instanceID, action)
}

func (c *Client) Press(ctx context.Context, instanceID, key string) error {
	action := Action{Type: "press", Key: key}
	return c.doAction(ctx, instanceID, action)
}

func (c *Client) doAction(ctx context.Context, instanceID string, action Action) error {
	data, _ := json.Marshal(action)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/instances/"+instanceID+"/action", bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("action failed: %s", string(body))
	}
	return nil
}

func (c *Client) Snapshot(ctx context.Context, instanceID string) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/instances/"+instanceID+"/snapshot", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("snapshot failed: %s", string(body))
	}

	var snapshot Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (c *Client) Text(ctx context.Context, instanceID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/instances/"+instanceID+"/text", nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("text failed: %s", string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) CloseInstance(ctx context.Context, instanceID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/instances/"+instanceID, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("close instance failed: %s", string(body))
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
