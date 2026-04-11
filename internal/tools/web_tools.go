package tools

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

type WebFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string { return "Fetch content from a URL" }

func (t *WebFetchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string"},
			"method":  map[string]any{"type": "string", "default": "GET"},
			"headers": map[string]any{"type": "object"},
			"body":    map[string]any{"type": "string"},
			"timeout": map[string]any{"type": "integer", "default": 30000},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}

	method, _ := input["method"].(string)
	if method == "" {
		method = "GET"
	}

	timeout := 30000
	if t, ok := input["timeout"].(int); ok && t > 0 {
		timeout = t
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	if headers, ok := input["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(body),
	}, nil
}

type WebSearchTool struct {
	apiKey string
	engine string
}

func NewWebSearchTool(apiKey, engine string) *WebSearchTool {
	return &WebSearchTool{
		apiKey: apiKey,
		engine: engine,
	}
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string { return "Search the web for information" }

func (t *WebSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":  map[string]any{"type": "string"},
			"limit":  map[string]any{"type": "integer", "default": 10},
			"offset": map[string]any{"type": "integer", "default": 0},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return nil, ErrRequiredField("query")
	}

	limit := 10
	if l, ok := input["limit"].(int); ok && l > 0 {
		limit = l
	}

	// If API key is configured, use the specified engine
	if t.apiKey != "" {
		switch t.engine {
		case "exa":
			return t.searchExa(ctx, query, limit)
		case "serper":
			return t.searchSerper(ctx, query, limit)
		case "tavily":
			return t.searchTavily(ctx, query, limit)
		case "duckduckgo":
			return t.searchDuckDuckGo(ctx, query, limit)
		default:
			return t.searchDuckDuckGo(ctx, query, limit)
		}
	}

	// Default to DuckDuckGo (free, no API key required)
	return t.searchDuckDuckGo(ctx, query, limit)
}

// searchDuckDuckGo uses DuckDuckGo's instant answer API
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, limit int) (any, error) {
	// DuckDuckGo Instant Answer API
	url := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1",
		strings.ReplaceAll(query, " ", "+"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ddgResponse struct {
		AbstractText   string `json:"AbstractText"`
		AbstractURL    string `json:"AbstractURL"`
		AbstractSource string `json:"AbstractSource"`
		Heading        string `json:"Heading"`
		Results        []struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &ddgResponse); err != nil {
		return nil, err
	}

	// Convert to standardized format
	results := make([]map[string]any, 0, limit)

	// Add abstract if available
	if ddgResponse.AbstractText != "" {
		results = append(results, map[string]any{
			"title":   ddgResponse.Heading,
			"url":     ddgResponse.AbstractURL,
			"snippet": ddgResponse.AbstractText,
			"source":  ddgResponse.AbstractSource,
		})
	}

	// Add results
	for i, r := range ddgResponse.Results {
		if i >= limit-len(results) {
			break
		}
		if r.Text != "" && r.URL != "" {
			results = append(results, map[string]any{
				"title":   r.Text,
				"url":     r.URL,
				"snippet": r.Text,
			})
		}
	}

	// Add related topics
	for i, r := range ddgResponse.RelatedTopics {
		if i >= limit-len(results) {
			break
		}
		if r.Text != "" && r.URL != "" {
			results = append(results, map[string]any{
				"title":   r.Text,
				"url":     r.URL,
				"snippet": r.Text,
			})
		}
	}

	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
		"engine":  "duckduckgo",
		"message": "Search completed successfully",
	}, nil
}

// searchExa uses Exa AI search API
func (t *WebSearchTool) searchExa(ctx context.Context, query string, limit int) (any, error) {
	url := "https://api.exa.ai/search"

	payload := map[string]any{
		"query":         query,
		"numResults":    limit,
		"useAutoprompt": true,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", t.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var exaResponse struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"text"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &exaResponse); err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0, len(exaResponse.Results))
	for _, r := range exaResponse.Results {
		results = append(results, map[string]any{
			"title":   r.Title,
			"url":     r.URL,
			"snippet": r.Snippet,
		})
	}

	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
		"engine":  "exa",
		"message": "Search completed successfully",
	}, nil
}

// searchSerper uses Serper.dev Google search API
func (t *WebSearchTool) searchSerper(ctx context.Context, query string, limit int) (any, error) {
	url := "https://google.serper.dev/search"

	payload := map[string]any{
		"q":   query,
		"num": limit,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", t.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var serperResponse struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}

	if err := json.Unmarshal(body, &serperResponse); err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0, len(serperResponse.Organic))
	for _, r := range serperResponse.Organic {
		results = append(results, map[string]any{
			"title":   r.Title,
			"url":     r.Link,
			"snippet": r.Snippet,
		})
	}

	return map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
		"engine":  "serper",
		"message": "Search completed successfully",
	}, nil
}

func (t *WebSearchTool) searchTavily(ctx context.Context, query string, limit int) (any, error) {
	url := "https://api.tavily.com/search"

	payload := map[string]any{
		"query":               query,
		"max_results":         limit,
		"include_answer":      true,
		"include_raw_content": false,
		"search_depth":        "advanced",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tavilyResponse struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &tavilyResponse); err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0, len(tavilyResponse.Results))
	for _, r := range tavilyResponse.Results {
		results = append(results, map[string]any{
			"title":   r.Title,
			"url":     r.URL,
			"snippet": r.Content,
			"score":   r.Score,
		})
	}

	response := map[string]any{
		"query":   query,
		"results": results,
		"count":   len(results),
		"engine":  "tavily",
		"message": "Search completed successfully",
	}

	if tavilyResponse.Answer != "" {
		response["answer"] = tavilyResponse.Answer
	}

	return response, nil
}

type HTTPRequestTool struct {
	client *http.Client
}

func NewHTTPRequestTool() *HTTPRequestTool {
	return &HTTPRequestTool{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (t *HTTPRequestTool) Name() string        { return "http_request" }
func (t *HTTPRequestTool) Description() string { return "Make an HTTP request" }

func (t *HTTPRequestTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string"},
			"method":  map[string]any{"type": "string", "default": "GET"},
			"headers": map[string]any{"type": "object"},
			"body":    map[string]any{"type": "string"},
			"timeout": map[string]any{"type": "integer"},
		},
		"required": []string{"url"},
	}
}

func (t *HTTPRequestTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	method, _ := input["method"].(string)
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	if headers, ok := input["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    resp.Header,
		"body":       string(body),
	}, nil
}

type JSONParserTool struct{}

func NewJSONParserTool() *JSONParserTool {
	return &JSONParserTool{}
}

func (t *JSONParserTool) Name() string        { return "json_parse" }
func (t *JSONParserTool) Description() string { return "Parse JSON string to object" }

func (t *JSONParserTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string"},
		},
		"required": []string{"json"},
	}
}

func (t *JSONParserTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	jsonStr, _ := input["json"].(string)
	if jsonStr == "" {
		return nil, ErrRequiredField("json")
	}

	var result any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

type JSONStringifyTool struct{}

func NewJSONStringifyTool() *JSONStringifyTool {
	return &JSONStringifyTool{}
}

func (t *JSONStringifyTool) Name() string        { return "json_stringify" }
func (t *JSONStringifyTool) Description() string { return "Convert object to JSON string" }

func (t *JSONStringifyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"object":  map[string]any{"type": "object"},
			"prettty": map[string]any{"type": "boolean", "default": false},
		},
		"required": []string{"object"},
	}
}

func (t *JSONStringifyTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	obj := input["object"]
	pretty, _ := input["pretty"].(bool)

	if pretty {
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return nil, err
		}
		return string(data), nil
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

type URLParserTool struct{}

func NewURLParserTool() *URLParserTool {
	return &URLParserTool{}
}

func (t *URLParserTool) Name() string        { return "url_parse" }
func (t *URLParserTool) Description() string { return "Parse URL into components" }

func (t *URLParserTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string"},
		},
		"required": []string{"url"},
	}
}

func (t *URLParserTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}

	parts := strings.SplitN(url, "://", 2)
	scheme := ""
	if len(parts) == 2 {
		scheme = parts[0]
		url = parts[1]
	}

	hostEnd := strings.Index(url, "/")
	host := ""
	path := ""
	if hostEnd > 0 {
		host = url[:hostEnd]
		path = url[hostEnd:]
	} else {
		host = url
	}

	return map[string]any{
		"scheme": scheme,
		"host":   host,
		"path":   path,
	}, nil
}

type TextTransformTool struct{}

func NewTextTransformTool() *TextTransformTool {
	return &TextTransformTool{}
}

func (t *TextTransformTool) Name() string { return "text_transform" }
func (t *TextTransformTool) Description() string {
	return "Transform text (uppercase, lowercase, etc.)"
}

func (t *TextTransformTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string"},
			"transform": map[string]any{"type": "string", "enum": []string{"upper", "lower", "title", "reverse"}},
		},
		"required": []string{"text", "transform"},
	}
}

func (t *TextTransformTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	text, _ := input["text"].(string)
	transform, _ := input["transform"].(string)

	switch transform {
	case "upper":
		return strings.ToUpper(text), nil
	case "lower":
		return strings.ToLower(text), nil
	case "title":
		return strings.Title(text), nil
	case "reverse":
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil
	default:
		return text, nil
	}
}
