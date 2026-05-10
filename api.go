package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Role       string                   `json:"role"`
	Content    interface{}              `json:"content"`
	Name       string                   `json:"name,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall               `json:"tool_calls,omitempty"`
	Images     []map[string]interface{} `json:"images,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Index    int    `json:"index,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ImageResult struct {
	Data     string // base64 without prefix
	MimeType string
	URL      string // if remote URL
}

type APIResult struct {
	Content      string
	ToolCalls    []ToolCall
	Tokens       int
	PromptTokens int
	Cost         float64
	Images       []ImageResult
}

type APIClient struct {
	client *http.Client
	apiURL string
	apiKey string
}

func NewAPIClient(apiURL, apiKey string) *APIClient {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &APIClient{
		client: &http.Client{Transport: transport},
		apiURL: apiURL,
		apiKey: apiKey,
	}
}

func (c *APIClient) buildBody(messages []Message, cfg *Config, tools []Tool, stream bool) ([]byte, error) {
	body := map[string]interface{}{
		"model":       cfg.Model,
		"messages":    messages,
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxTokens,
		"stream":      stream,
	}
	if strings.Contains(cfg.Model, "-image") {
		body["modalities"] = []string{"image", "text"}
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	return json.Marshal(body)
}

// Request performs a blocking (non-streaming) API call.
func (c *APIClient) Request(ctx context.Context, messages []Message, cfg *Config, tools []Tool) (*APIResult, error) {
	data, err := c.buildBody(messages, cfg, tools, false)
	if err != nil {
		return nil, fmt.Errorf("building request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	Log.Debug("api request", "model", cfg.Model, "messages", len(messages), "tools", len(tools), "bytes", len(data))
	resp, err := c.client.Do(req)
	if err != nil {
		setConnError()
		Log.Debug("api request failed", "err", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		setConnError()
		Log.Debug("api error response", "status", resp.StatusCode, "body", string(body[:min(len(body), 500)]))
		return nil, fmt.Errorf("API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body[:min(len(body), 500)])))
	}
	setConnOK()
	Log.Debug("api response", "status", resp.StatusCode, "bytes", len(body))

	var parsed struct {
		Choices []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}

	if strings.Contains(c.apiURL, "openrouter") && os.Getenv("NOOR_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] raw response: %s\n", string(body[:min(len(body), 2000)]))
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing API response: %w", err)
	}

	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, &APIError{Message: parsed.Error.Message}
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	msg := parsed.Choices[0].Message
	content, images := parseContent(msg.Content)

	// OpenRouter Gemini image models return images in message.images, not content
	for _, item := range msg.Images {
		if iu, ok := item["image_url"].(map[string]interface{}); ok {
			if u, ok := iu["url"].(string); ok {
				images = append(images, parseImageURL(u))
			}
		}
	}

	return &APIResult{
		Content:      content,
		ToolCalls:    msg.ToolCalls,
		Tokens:       parsed.Usage.CompletionTokens,
		PromptTokens: parsed.Usage.PromptTokens,
		Cost:         parsed.Usage.Cost,
		Images:       images,
	}, nil
}

// Stream performs a streaming API call, calling onChunk for each text delta.
func (c *APIClient) Stream(ctx context.Context, messages []Message, cfg *Config, tools []Tool, onChunk func(string)) (*APIResult, error) {
	data, err := c.buildBody(messages, cfg, tools, true)
	if err != nil {
		return nil, fmt.Errorf("building stream request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		setConnError()
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP-level errors before reading the body
	if resp.StatusCode >= 400 {
		setConnError()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	setConnOK()

	var (
		fullContent      strings.Builder
		toolCalls        = make(map[int]*ToolCall)
		completionTokens int
		streamCost       float64
		firstLine        = true
	)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024) // 16MB max line for image data

	for scanner.Scan() {
		line := scanner.Text()

		// Skip SSE comment lines (e.g. ": OPENROUTER PROCESSING") and blank lines
		if strings.HasPrefix(line, ":") || (firstLine && strings.TrimSpace(line) == "") {
			continue
		}

		// Non-streaming JSON fallback: first substantial line that doesn't
		// start with "data:" may be a non-streaming API response.
		if firstLine && !strings.HasPrefix(line, "data:") && line != "" {
			firstLine = false
			var parsed struct {
				Choices []struct {
					Message Message `json:"message"`
				} `json:"choices"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
				Usage struct {
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(line), &parsed); err == nil {
				if parsed.Error != nil && parsed.Error.Message != "" {
					return nil, &APIError{Message: parsed.Error.Message}
				}
				if len(parsed.Choices) > 0 {
					content, _ := parsed.Choices[0].Message.Content.(string)
					onChunk(content)
					return &APIResult{
						Content: content,
						Tokens:  parsed.Usage.CompletionTokens,
					}, nil
				}
			}
			return nil, fmt.Errorf("unexpected response: %s", line[:min(len(line), 300)])
		}
		firstLine = false

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		if payload == "" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				CompletionTokens int     `json:"completion_tokens"`
				Cost             float64 `json:"cost"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			completionTokens = chunk.Usage.CompletionTokens
			streamCost = chunk.Usage.Cost
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			onChunk(delta.Content)
			fullContent.WriteString(delta.Content)
		}

		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if _, ok := toolCalls[idx]; !ok {
				toolCalls[idx] = &ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
				}
			}
			existing := toolCalls[idx]
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			existing.Function.Arguments += tc.Function.Arguments
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("streaming error: %w", err)
	}

	result := &APIResult{
		Content: fullContent.String(),
		Tokens:  completionTokens,
		Cost:    streamCost,
	}
	for i := 0; i < len(toolCalls); i++ {
		if tc, ok := toolCalls[i]; ok {
			result.ToolCalls = append(result.ToolCalls, *tc)
		}
	}
	return result, nil
}

// RequestWithRetry wraps Request with a single silent retry on network errors.
func (c *APIClient) RequestWithRetry(ctx context.Context, messages []Message, cfg *Config, tools []Tool) (*APIResult, error) {
	result, err := c.Request(ctx, messages, cfg, tools)
	if err == nil {
		return result, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return nil, err
	}
	time.Sleep(2 * time.Second)
	return c.Request(ctx, messages, cfg, tools)
}

type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %s", e.Message)
}

// parseContent handles both string and array content (text + images).
func parseContent(raw interface{}) (string, []ImageResult) {
	switch c := raw.(type) {
	case string:
		return c, nil
	case []interface{}:
		var text string
		var images []ImageResult
		for _, item := range c {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if t, ok := m["text"].(string); ok {
					text += t
				}
			case "image_url":
				if iu, ok := m["image_url"].(map[string]interface{}); ok {
					if u, ok := iu["url"].(string); ok {
						images = append(images, parseImageURL(u))
					}
				}
			}
		}
		return text, images
	}
	return "", nil
}

// parseImageURL splits a data URL into base64+mime or keeps it as a remote URL.
func parseImageURL(u string) ImageResult {
	if strings.HasPrefix(u, "data:") {
		// data:image/png;base64,<data>
		rest := strings.TrimPrefix(u, "data:")
		parts := strings.SplitN(rest, ",", 2)
		if len(parts) == 2 {
			mime := strings.TrimSuffix(parts[0], ";base64")
			return ImageResult{Data: parts[1], MimeType: mime}
		}
	}
	return ImageResult{URL: u}
}
