package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIError(t *testing.T) {
	err := &APIError{Message: "rate limited"}
	if err.Error() != "API error: rate limited" {
		t.Errorf("unexpected error string: %s", err.Error())
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Error("expected errors.As to match APIError")
	}
	if apiErr.Message != "rate limited" {
		t.Errorf("expected message 'rate limited', got %s", apiErr.Message)
	}
}

func TestParseImageURL(t *testing.T) {
	r := parseImageURL("data:image/png;base64,abc123")
	if r.Data != "abc123" || r.MimeType != "image/png" {
		t.Errorf("data URL: got data=%q mime=%q", r.Data, r.MimeType)
	}

	r = parseImageURL("https://example.com/image.png")
	if r.URL != "https://example.com/image.png" || r.Data != "" {
		t.Errorf("remote URL: got URL=%q data=%q", r.URL, r.Data)
	}

	// Data URL without explicit ;base64 still parses (TrimSuffix is no-op)
	r = parseImageURL("data:image/png,abc123")
	if r.Data != "abc123" || r.MimeType != "image/png" {
		t.Errorf("data URL no base64: got data=%q mime=%q", r.Data, r.MimeType)
	}
}

func TestParseContent(t *testing.T) {
	// String content
	text, imgs := parseContent("hello world")
	if text != "hello world" || len(imgs) != 0 {
		t.Errorf("string content: text=%q, imgs=%d", text, len(imgs))
	}

	// Array content with text
	arr := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "hello",
		},
	}
	text, imgs = parseContent(arr)
	if text != "hello" || len(imgs) != 0 {
		t.Errorf("array text: text=%q, imgs=%d", text, len(imgs))
	}

	// Array content with image
	arr = []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "look at this",
		},
		map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "https://example.com/img.png",
			},
		},
	}
	text, imgs = parseContent(arr)
	if text != "look at this" || len(imgs) != 1 {
		t.Errorf("array image: text=%q, imgs=%d", text, len(imgs))
	}
	if imgs[0].URL != "https://example.com/img.png" {
		t.Errorf("image URL: %q", imgs[0].URL)
	}

	// Unknown type
	text, imgs = parseContent(42)
	if text != "" || len(imgs) != 0 {
		t.Errorf("unknown type: text=%q, imgs=%d", text, len(imgs))
	}
}

func TestAPIClientRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("bad auth header: %s", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("bad content-type: %s", ct)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["stream"] != false {
			t.Errorf("expected stream=false, got %v", body["stream"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "hello"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "cost": 0.001}
		}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-key")
	cfg := defaultConfig()
	result, err := client.Request(context.Background(), []Message{}, &cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "hello" {
		t.Errorf("content = %q, want hello", result.Content)
	}
	if result.Tokens != 5 {
		t.Errorf("tokens = %d, want 5", result.Tokens)
	}
	if result.PromptTokens != 10 {
		t.Errorf("prompt tokens = %d, want 10", result.PromptTokens)
	}
	if result.Cost != 0.001 {
		t.Errorf("cost = %f, want 0.001", result.Cost)
	}
}

func TestAPIClientRequestError(t *testing.T) {
	// APIError is returned when HTTP is 200 but JSON body contains error field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": {"message": "invalid model"}, "choices": [], "usage": {}}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-key")
	cfg := defaultConfig()
	_, err := client.Request(context.Background(), []Message{}, &cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Message != "invalid model" {
		t.Errorf("message = %q, want 'invalid model'", apiErr.Message)
	}
}

func TestAPIClientRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-key")
	cfg := defaultConfig()
	_, err := client.Request(context.Background(), []Message{}, &cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "API returned HTTP 401") {
		t.Errorf("error = %q, want prefix 'API returned HTTP 401'", err.Error())
	}
}

func TestAPIClientRequestEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices": [], "usage": {}}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-key")
	cfg := defaultConfig()
	_, err := client.Request(context.Background(), []Message{}, &cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "empty response from API" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestAPIClientRequestWithRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"completion_tokens":1,"cost":0.0001}}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "key")
	cfg := defaultConfig()
	result, err := client.RequestWithRetry(context.Background(), []Message{}, &cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "ok" {
		t.Errorf("content = %q, want ok", result.Content)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestAPIClientRequestWithRetryDoesNotRetryAPIError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": {"message": "bad request"}, "choices": [], "usage": {}}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "key")
	cfg := defaultConfig()
	_, err := client.RequestWithRetry(context.Background(), []Message{}, &cfg, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry on API error), got %d", calls)
	}
}

func TestAPIClientStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected Flusher")
		}

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"usage\":{\"completion_tokens\":2,\"cost\":0.0002}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "key")
	cfg := defaultConfig()

	var chunks []string
	result, err := client.Stream(context.Background(), []Message{}, &cfg, nil, func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "hello world" {
		t.Errorf("content = %q, want 'hello world'", result.Content)
	}
	if result.Tokens != 2 {
		t.Errorf("tokens = %d, want 2", result.Tokens)
	}
	if result.Cost != 0.0002 {
		t.Errorf("cost = %f, want 0.0002", result.Cost)
	}
	if len(chunks) != 2 || chunks[0] != "hello" || chunks[1] != " world" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestAPIClientStreamNonStreamingFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"direct"}}],"usage":{"completion_tokens":1}}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "key")
	cfg := defaultConfig()

	var chunks []string
	result, err := client.Stream(context.Background(), []Message{}, &cfg, nil, func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "direct" {
		t.Errorf("content = %q, want direct", result.Content)
	}
	if len(chunks) != 1 || chunks[0] != "direct" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestAPIClientStreamHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "key")
	cfg := defaultConfig()
	_, err := client.Stream(context.Background(), []Message{}, &cfg, nil, func(string) {})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want 429 mention", err.Error())
	}
}
