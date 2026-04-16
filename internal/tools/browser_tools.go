package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

type browserContextEntry struct {
	ctx      context.Context
	cancel   context.CancelFunc
	lastUsed time.Time
}

var (
	browserCtxMu sync.RWMutex
	browserCtxs  map[string]*browserContextEntry
	browserOnce  sync.Once
)

func init() {
	browserCtxs = make(map[string]*browserContextEntry)
}

func startBrowserCtxCleanup() {
	browserOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cleanStaleBrowserContexts()
			}
		}()
	})
}

func cleanStaleBrowserContexts() {
	browserCtxMu.Lock()
	defer browserCtxMu.Unlock()

	now := time.Now()
	for id, entry := range browserCtxs {
		if now.Sub(entry.lastUsed) > 30*time.Minute {
			entry.cancel()
			delete(browserCtxs, id)
			slog.Debug("browser: cleaned up stale context", "session_id", id)
		}
	}
}

func getOrCreateBrowserContext(sessionID string) (*browserContextEntry, error) {
	startBrowserCtxCleanup()

	browserCtxMu.RLock()
	if entry, ok := browserCtxs[sessionID]; ok {
		entry.lastUsed = time.Now()
		browserCtxMu.RUnlock()
		return entry, nil
	}
	browserCtxMu.RUnlock()

	browserCtxMu.Lock()
	defer browserCtxMu.Unlock()

	if entry, ok := browserCtxs[sessionID]; ok {
		entry.lastUsed = time.Now()
		return entry, nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.WindowSize(1280, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	bctx, bCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(s string, i ...any) {
		slog.Debug("chromedp", "msg", fmt.Sprintf(s, i...))
	}))

	entry := &browserContextEntry{
		ctx: bctx,
		cancel: func() {
			bCancel()
			allocCancel()
		},
		lastUsed: time.Now(),
	}

	browserCtxs[sessionID] = entry
	slog.Debug("browser: created new persistent context", "session_id", sessionID)
	return entry, nil
}

func getSessionID(input map[string]any) string {
	if sid, ok := input["session_id"].(string); ok && sid != "" {
		return sid
	}
	return "default"
}

type BrowserNavigateTool struct{ BaseTool }

func (t *BrowserNavigateTool) Name() string { return "browser_navigate" }
func (t *BrowserNavigateTool) Description() string {
	return "Navigate to a URL in a headless browser. Returns the page title and URL after loading."
}

func (t *BrowserNavigateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to navigate to",
			},
			"wait": map[string]any{
				"type":        "string",
				"default":     "load",
				"description": "Wait condition: load, domcontentloaded, none",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"url"},
	}
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser navigate: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	var title string
	err = chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
	)

	currentURL := url
	if err != nil {
		return nil, fmt.Errorf("browser navigate failed: %w", err)
	}

	return map[string]any{
		"url":        currentURL,
		"title":      title,
		"session_id": sessionID,
	}, nil
}

type BrowserClickTool struct{ BaseTool }

func (t *BrowserClickTool) Name() string { return "browser_click" }
func (t *BrowserClickTool) Description() string {
	return "Click an element on the current page using a CSS selector."
}

func (t *BrowserClickTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the element to click",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"selector"},
	}
}

func (t *BrowserClickTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		return nil, ErrRequiredField("selector")
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser click: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	err = chromedp.Run(timeoutCtx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
	if err != nil {
		return nil, fmt.Errorf("browser click failed: %w", err)
	}

	return map[string]any{
		"clicked": selector,
	}, nil
}

type BrowserTypeTool struct{ BaseTool }

func (t *BrowserTypeTool) Name() string { return "browser_type" }
func (t *BrowserTypeTool) Description() string {
	return "Type text into an input field identified by CSS selector."
}

func (t *BrowserTypeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the input element",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text to type into the field",
			},
			"clear": map[string]any{
				"type":        "boolean",
				"default":     true,
				"description": "Clear existing text before typing",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"selector", "text"},
	}
}

func (t *BrowserTypeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	text, _ := input["text"].(string)
	clear := true
	if c, ok := input["clear"].(bool); ok {
		clear = c
	}

	if selector == "" {
		return nil, ErrRequiredField("selector")
	}
	if text == "" {
		return nil, ErrRequiredField("text")
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser type: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	actions := []chromedp.Action{chromedp.WaitVisible(selector)}
	if clear {
		actions = append(actions, chromedp.Clear(selector))
	}
	actions = append(actions, chromedp.SendKeys(selector, text))

	err = chromedp.Run(timeoutCtx, actions...)
	if err != nil {
		return nil, fmt.Errorf("browser type failed: %w", err)
	}

	return map[string]any{
		"selector": selector,
		"typed":    text,
	}, nil
}

type BrowserScreenshotTool struct{ BaseTool }

func (t *BrowserScreenshotTool) Name() string { return "browser_screenshot" }
func (t *BrowserScreenshotTool) Description() string {
	return "Take a screenshot of the current page. Returns a base64-encoded PNG image."
}

func (t *BrowserScreenshotTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "Optional CSS selector to screenshot a specific element (defaults to full page)",
			},
			"quality": map[string]any{
				"type":        "integer",
				"default":     80,
				"description": "JPEG quality 1-100 (only for JPEG, not PNG)",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
	}
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser screenshot: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	var buf []byte

	if selector != "" {
		err = chromedp.Run(timeoutCtx,
			chromedp.WaitVisible(selector),
			chromedp.Screenshot(selector, &buf, chromedp.NodeVisible),
		)
	} else {
		err = chromedp.Run(timeoutCtx,
			chromedp.WaitReady("body"),
			chromedp.FullScreenshot(&buf, 100),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("browser screenshot failed: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf)

	return map[string]any{
		"image_base64": encoded,
		"format":       "png",
		"size_bytes":   len(buf),
	}, nil
}

type BrowserExtractTool struct{ BaseTool }

func (t *BrowserExtractTool) Name() string { return "browser_extract" }
func (t *BrowserExtractTool) Description() string {
	return "Extract text content from the current page or a specific CSS selector."
}

func (t *BrowserExtractTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector to extract text from (defaults to 'body')",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
	}
}

func (t *BrowserExtractTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		selector = "body"
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser extract: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	var text string
	err = chromedp.Run(timeoutCtx,
		chromedp.WaitReady(selector),
		chromedp.Text(selector, &text, chromedp.NodeVisible),
	)
	if err != nil {
		return nil, fmt.Errorf("browser extract failed: %w", err)
	}

	truncated := false
	maxLen := 10000
	if len(text) > maxLen {
		text = text[:maxLen] + "\n... (truncated)"
		truncated = true
	}

	return map[string]any{
		"text":        text,
		"selector":    selector,
		"truncated":   truncated,
		"full_length": len(text),
	}, nil
}

type BrowserWaitTool struct{ BaseTool }

func (t *BrowserWaitTool) Name() string { return "browser_wait" }
func (t *BrowserWaitTool) Description() string {
	return "Wait for an element to appear on the page. Useful for dynamic content that loads after navigation."
}

func (t *BrowserWaitTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector to wait for",
			},
			"state": map[string]any{
				"type":        "string",
				"default":     "visible",
				"description": "Wait state: visible, hidden, enabled, disabled",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"default":     10000,
				"description": "Maximum wait time in milliseconds",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"selector"},
	}
}

func (t *BrowserWaitTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		return nil, ErrRequiredField("selector")
	}

	state, _ := input["state"].(string)
	if state == "" {
		state = "visible"
	}

	timeoutMs := 10000
	if ts, ok := input["timeout"].(int); ok && ts > 0 {
		timeoutMs = ts
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser wait: failed to get context: %w", err)
	}

	waitTimeout := time.Duration(timeoutMs) * time.Millisecond
	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, waitTimeout)
	defer timeoutCancel()

	start := time.Now()

	switch state {
	case "visible":
		err = chromedp.Run(timeoutCtx, chromedp.WaitVisible(selector))
	case "hidden":
		err = chromedp.Run(timeoutCtx, chromedp.WaitNotVisible(selector))
	case "enabled":
		err = chromedp.Run(timeoutCtx, chromedp.WaitEnabled(selector))
	case "disabled":
		err = chromedp.Run(timeoutCtx, chromedp.WaitNotVisible(selector))
	default:
		err = chromedp.Run(timeoutCtx, chromedp.WaitVisible(selector))
	}

	if err != nil {
		return nil, fmt.Errorf("browser wait failed: %w", err)
	}

	return map[string]any{
		"selector":   selector,
		"state":      state,
		"waited_ms":  time.Since(start).Milliseconds(),
		"timeout_ms": timeoutMs,
	}, nil
}

type BrowserSelectTool struct{ BaseTool }

func (t *BrowserSelectTool) Name() string { return "browser_select" }
func (t *BrowserSelectTool) Description() string {
	return "Select an option in a <select> dropdown element by value or visible text."
}

func (t *BrowserSelectTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the <select> element",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "Value attribute of the option to select",
			},
			"visible_text": map[string]any{
				"type":        "string",
				"description": "Visible text of the option (alternative to value)",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"selector"},
	}
}

func (t *BrowserSelectTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		return nil, ErrRequiredField("selector")
	}

	value, _ := input["value"].(string)
	visibleText, _ := input["visible_text"].(string)

	if value == "" && visibleText == "" {
		return nil, fmt.Errorf("either 'value' or 'visible_text' is required")
	}

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser select: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	var jsExpr string
	if value != "" {
		jsExpr = fmt.Sprintf(
			`document.querySelector(%q).value = %q; document.querySelector(%q).dispatchEvent(new Event('change', {bubbles: true}))`,
			selector, value, selector,
		)
	} else {
		jsExpr = fmt.Sprintf(
			`(() => { const sel = document.querySelector(%q); for (const opt of sel.options) { if (opt.text === %q) { sel.value = opt.value; sel.dispatchEvent(new Event('change', {bubbles: true})); return; } } })()`,
			selector, visibleText,
		)
	}

	err = chromedp.Run(timeoutCtx,
		chromedp.WaitVisible(selector),
		chromedp.Evaluate(jsExpr, nil),
	)
	if err != nil {
		return nil, fmt.Errorf("browser select failed: %w", err)
	}

	selected := value
	if selected == "" {
		selected = visibleText
	}

	return map[string]any{
		"selector": selector,
		"selected": selected,
	}, nil
}

type BrowserFillFormTool struct{ BaseTool }

func (t *BrowserFillFormTool) Name() string { return "browser_fill_form" }
func (t *BrowserFillFormTool) Description() string {
	return "Fill multiple form fields at once. Provide a map of CSS selectors to values."
}

func (t *BrowserFillFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields": map[string]any{
				"type":        "object",
				"description": "Map of CSS selectors to values to fill in",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"submit": map[string]any{
				"type":        "string",
				"description": "Optional CSS selector of submit button to click after filling",
			},
			"session_id": map[string]any{
				"type":        "string",
				"default":     "default",
				"description": "Browser session ID to persist context across tool calls",
			},
		},
		"required": []string{"fields"},
	}
}

func (t *BrowserFillFormTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	fieldsRaw, ok := input["fields"].(map[string]any)
	if !ok || len(fieldsRaw) == 0 {
		return nil, ErrRequiredField("fields")
	}

	fields := make(map[string]string, len(fieldsRaw))
	for k, v := range fieldsRaw {
		if s, ok := v.(string); ok {
			fields[k] = s
		}
	}

	submit, _ := input["submit"].(string)

	sessionID := getSessionID(input)
	entry, err := getOrCreateBrowserContext(sessionID)
	if err != nil {
		return nil, fmt.Errorf("browser fill form: failed to get context: %w", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(entry.ctx, 30*time.Second)
	defer timeoutCancel()

	var actions []chromedp.Action

	for selector, value := range fields {
		sel := selector
		val := value
		actions = append(actions,
			chromedp.WaitVisible(sel),
			chromedp.Clear(sel),
			chromedp.SendKeys(sel, val),
		)
	}

	if submit != "" {
		actions = append(actions, chromedp.Click(submit))
	}

	err = chromedp.Run(timeoutCtx, actions...)
	if err != nil {
		return nil, fmt.Errorf("browser fill form failed: %w", err)
	}

	return map[string]any{
		"fields_filled": len(fields),
		"submitted":     submit != "",
	}, nil
}
