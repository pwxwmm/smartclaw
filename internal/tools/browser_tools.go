package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/chromedp/chromedp"
)

type BrowserNavigateTool struct{}

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
		},
		"required": []string{"url"},
	}
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	var title string
	err := chromedp.Run(bctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
	)

	currentURL := url
	if err != nil {
		return nil, fmt.Errorf("browser navigate failed: %w", err)
	}

	return map[string]any{
		"url":   currentURL,
		"title": title,
	}, nil
}

type BrowserClickTool struct{}

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
		},
		"required": []string{"selector"},
	}
}

func (t *BrowserClickTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		return nil, ErrRequiredField("selector")
	}

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	err := chromedp.Run(bctx,
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

type BrowserTypeTool struct{}

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

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	actions := []chromedp.Action{chromedp.WaitVisible(selector)}
	if clear {
		actions = append(actions, chromedp.Clear(selector))
	}
	actions = append(actions, chromedp.SendKeys(selector, text))

	err := chromedp.Run(bctx, actions...)
	if err != nil {
		return nil, fmt.Errorf("browser type failed: %w", err)
	}

	return map[string]any{
		"selector": selector,
		"typed":    text,
	}, nil
}

type BrowserScreenshotTool struct{}

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
		},
	}
}

func (t *BrowserScreenshotTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	var buf []byte
	var err error

	if selector != "" {
		err = chromedp.Run(bctx,
			chromedp.WaitVisible(selector),
			chromedp.Screenshot(selector, &buf, chromedp.NodeVisible),
		)
	} else {
		err = chromedp.Run(bctx,
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

type BrowserExtractTool struct{}

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
		},
	}
}

func (t *BrowserExtractTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	selector, _ := input["selector"].(string)
	if selector == "" {
		selector = "body"
	}

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	var text string
	err := chromedp.Run(bctx,
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

func newBrowserContext(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.WindowSize(1280, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)

	bctx, bCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(s string, i ...any) {
		slog.Debug("chromedp", "msg", fmt.Sprintf(s, i...))
	}))

	timeoutCtx, timeoutCancel := context.WithTimeout(bctx, 30*time.Second)

	combinedCancel := func() {
		timeoutCancel()
		bCancel()
		allocCancel()
	}

	return timeoutCtx, combinedCancel
}

type BrowserWaitTool struct{}

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

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

	start := time.Now()

	var err error
	switch state {
	case "visible":
		err = chromedp.Run(bctx, chromedp.WaitVisible(selector))
	case "hidden":
		err = chromedp.Run(bctx, chromedp.WaitNotVisible(selector))
	case "enabled":
		err = chromedp.Run(bctx, chromedp.WaitEnabled(selector))
	case "disabled":
		err = chromedp.Run(bctx, chromedp.WaitNotVisible(selector))
	default:
		err = chromedp.Run(bctx, chromedp.WaitVisible(selector))
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

type BrowserSelectTool struct{}

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

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

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

	err := chromedp.Run(bctx,
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

type BrowserFillFormTool struct{}

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

	bctx, cancel := newBrowserContext(ctx)
	defer cancel()

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

	err := chromedp.Run(bctx, actions...)
	if err != nil {
		return nil, fmt.Errorf("browser fill form failed: %w", err)
	}

	return map[string]any{
		"fields_filled": len(fields),
		"submitted":     submit != "",
	}, nil
}
