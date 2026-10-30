package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible Chat Completions API.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL:    normalizeBaseURL(baseURL),
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func normalizeBaseURL(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	if u == "" {
		return ""
	}
	lower := strings.ToLower(u)
	if strings.HasSuffix(lower, "/chat/completions") {
		u = strings.TrimSuffix(u, "/chat/completions")
		u = strings.TrimSuffix(u, "/Chat/Completions")
		u = strings.TrimRight(u, "/")
	}
	return u
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.model != ""
}

func (c *Client) WithModel(model string) *Client {
	if c == nil {
		return nil
	}
	out := *c
	out.model = strings.TrimSpace(model)
	return &out
}

type ModelInfo struct {
	ID string `json:"id"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if c == nil || c.baseURL == "" || c.apiKey == "" {
		return nil, fmt.Errorf("未配置 LLM 提供商")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("列出模型失败 HTTP %d: %s", res.StatusCode, truncate(string(body), 240))
	}
	var parsed modelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析模型列表失败: %w", err)
	}
	out := make([]ModelInfo, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			out = append(out, ModelInfo{ID: id})
		}
	}
	return out, nil
}

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("未配置 LLM（需要 base_url / api_key / model）")
	}
	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	var parsed chatResponse
	_ = json.Unmarshal(body, &parsed)
	if res.StatusCode >= 300 {
		msg := truncate(string(body), 320)
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return "", fmt.Errorf("LLM HTTP %d: %s", res.StatusCode, msg)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("LLM 返回空内容")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.baseURL == "" || c.apiKey == "" {
		return fmt.Errorf("未配置 LLM 提供商")
	}
	if c.model == "" {
		_, err := c.ListModels(ctx)
		return err
	}
	_, err := c.Chat(ctx, "You are a connectivity probe. Reply with exactly: ok", "ping")
	return err
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
