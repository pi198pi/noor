package main

import (
	"errors"
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
}
