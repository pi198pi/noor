package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tinyfishWebSearch hits TinyFish's Search API and returns a flat plain-text
// result the chat model can consume directly.
//
// Endpoint:  GET https://api.search.tinyfish.ai/?query=…&language=en
// Auth:      X-API-Key: <key>     (NOT Bearer, NOT query string)
// Response:  {query, results: [{position,title,url,snippet,site_name}], total_results}
//
// No "answer box" equivalent — only ranked organic results. The model
// synthesises the answer from the snippets.
func tinyfishWebSearch(query, apiKey string) string {
	fmt.Printf("\r\033[K  %s", styleInfo.Render("🔍 searching (tinyfish)..."))

	reqURL := "https://api.search.tinyfish.ai/?" + url.Values{
		"query":    {query},
		"language": {"en"},
	}.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return fmt.Sprintf("search error: building request: %v", err)
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("search error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("search error: reading response: %v", err)
	}

	if resp.StatusCode >= 400 {
		// Surface the upstream error in a way the LLM can act on (e.g. quota
		// exhausted → fall back to general knowledge).
		return fmt.Sprintf("search error: HTTP %d from tinyfish: %s",
			resp.StatusCode, truncate(string(body), 300))
	}

	var result struct {
		Query        string `json:"query"`
		TotalResults int    `json:"total_results"`
		Results      []struct {
			Position int    `json:"position"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Snippet  string `json:"snippet"`
			SiteName string `json:"site_name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "search error: invalid response from tinyfish"
	}

	if len(result.Results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	sb.WriteString("Search date: " + time.Now().Format("January 2, 2006") + "\n\n")
	for i, r := range result.Results {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\n%s\n\n",
			i+1, r.Title, r.URL, r.Snippet))
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
