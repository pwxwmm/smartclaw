package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
)

type DAPClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
	stderr      bytes.Buffer
	mu          sync.Mutex
	seq         int
	initialized bool
	events      chan *DAPEvent
	done        chan struct{}
}

func Launch(programPath string, args ...string) (*DAPClient, error) {
	dlvArgs := append([]string{"dap"}, args...)
	cmd := exec.Command("dlv", dlvArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	client := &DAPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdoutPipe),
		events: make(chan *DAPEvent, 64),
		done:   make(chan struct{}),
	}

	cmd.Stderr = &client.stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dlv dap: %w", err)
	}

	go client.readEvents()

	return client, nil
}

func (c *DAPClient) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	args := map[string]any{
		"adapterID":    "smartclaw-dap",
		"pathFormat":   "path",
		"linesStartAt1": true,
		"columnsStartAt1": true,
		"supportsVariableType":       true,
		"supportsVariablePaging":     false,
		"supportsRunInTerminalRequest": false,
		"supportsMemoryReferences":   false,
		"supportsProgressReporting":  false,
	}

	resp, err := c.sendRequest("initialize", args)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("initialize failed: %s", resp.Message)
	}

	c.initialized = true
	return nil
}

func (c *DAPClient) LaunchRequest(programPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := map[string]any{
		"program": programPath,
		"mode":    "debug",
	}

	resp, err := c.sendRequest("launch", args)
	if err != nil {
		return fmt.Errorf("launch request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("launch failed: %s", resp.Message)
	}

	c.waitForInitializedEvent()
	return nil
}

func (c *DAPClient) waitForInitializedEvent() {
	for {
		select {
		case evt := <-c.events:
			if evt.Event == "initialized" {
				return
			}
		default:
			return
		}
	}
}

func (c *DAPClient) SetBreakpoints(source Source, lines []int) ([]Breakpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	breakpoints := make([]map[string]any, len(lines))
	for i, line := range lines {
		bp := map[string]any{"line": line}
		breakpoints[i] = bp
	}

	args := map[string]any{
		"source": map[string]any{
			"name": source.Name,
			"path": source.Path,
		},
		"breakpoints": breakpoints,
		"sourceModified": false,
	}

	resp, err := c.sendRequest("setBreakpoints", args)
	if err != nil {
		return nil, fmt.Errorf("setBreakpoints request failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("setBreakpoints failed: %s", resp.Message)
	}

	return c.parseBreakpoints(resp.Body), nil
}

func (c *DAPClient) Continue() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.sendRequest("continue", map[string]any{
		"threadId": 1,
	})
	if err != nil {
		return fmt.Errorf("continue request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("continue failed: %s", resp.Message)
	}

	return nil
}

func (c *DAPClient) Next() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.sendRequest("next", map[string]any{
		"threadId": 1,
	})
	if err != nil {
		return fmt.Errorf("next request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("next failed: %s", resp.Message)
	}

	return nil
}

func (c *DAPClient) StepIn() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.sendRequest("stepIn", map[string]any{
		"threadId": 1,
	})
	if err != nil {
		return fmt.Errorf("stepIn request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("stepIn failed: %s", resp.Message)
	}

	return nil
}

func (c *DAPClient) StepOut() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	resp, err := c.sendRequest("stepOut", map[string]any{
		"threadId": 1,
	})
	if err != nil {
		return fmt.Errorf("stepOut request failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("stepOut failed: %s", resp.Message)
	}

	return nil
}

func (c *DAPClient) GetStackTrace(threadID int) ([]StackFrame, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := map[string]any{
		"threadId": threadID,
		"levels":   20,
	}

	resp, err := c.sendRequest("stackTrace", args)
	if err != nil {
		return nil, fmt.Errorf("stackTrace request failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("stackTrace failed: %s", resp.Message)
	}

	return c.parseStackFrames(resp.Body), nil
}

func (c *DAPClient) GetScopes(frameID int) ([]Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := map[string]any{
		"frameId": frameID,
	}

	resp, err := c.sendRequest("scopes", args)
	if err != nil {
		return nil, fmt.Errorf("scopes request failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("scopes failed: %s", resp.Message)
	}

	return c.parseScopes(resp.Body), nil
}

func (c *DAPClient) GetVariables(variablesRef int) ([]Variable, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := map[string]any{
		"variablesReference": variablesRef,
	}

	resp, err := c.sendRequest("variables", args)
	if err != nil {
		return nil, fmt.Errorf("variables request failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("variables failed: %s", resp.Message)
	}

	return c.parseVariables(resp.Body), nil
}

func (c *DAPClient) Evaluate(expression string, frameID int) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := map[string]any{
		"expression": expression,
		"context":    "repl",
	}

	if frameID > 0 {
		args["frameId"] = frameID
	}

	resp, err := c.sendRequest("evaluate", args)
	if err != nil {
		return "", fmt.Errorf("evaluate request failed: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("evaluate failed: %s", resp.Message)
	}

	if resp.Body != nil {
		if result, ok := resp.Body["result"].(string); ok {
			return result, nil
		}
	}

	return "", nil
}

func (c *DAPClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, _ = c.sendRequest("disconnect", map[string]any{
		"restart": false,
		"terminateDebuggee": true,
	})

	close(c.done)

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}

	return nil
}

func (c *DAPClient) sendRequest(command string, args map[string]any) (*DAPResponse, error) {
	c.seq++
	req := DAPRequest{
		Seq:       c.seq,
		Type:      "request",
		Command:   command,
		Arguments: args,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(reqBytes))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return nil, fmt.Errorf("write header failed: %w", err)
	}
	if _, err := c.stdin.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("write body failed: %w", err)
	}

	return c.readResponse()
}

func (c *DAPClient) readResponse() (*DAPResponse, error) {
	for {
		msg, err := c.readMessage()
		if err != nil {
			return nil, err
		}

		if msg.Type == "response" {
			resp := &DAPResponse{
				Seq:        msg.Seq,
				Type:       msg.Type,
				Command:    msg.Command,
				RequestSeq: msg.RequestSeq,
				Success:    msg.Success,
				Body:       msg.Body,
				Message:    msg.Message,
			}
			return resp, nil
		}

		if msg.Type == "event" {
			evt := &DAPEvent{
				Seq:   msg.Seq,
				Type:  msg.Type,
				Event: msg.Event,
				Body:  msg.Body,
			}
			select {
			case c.events <- evt:
			default:
				slog.Warn("dap: event channel full, dropping event", "event", evt.Event)
			}
		}
	}
}

func (c *DAPClient) readMessage() (*DAPMessage, error) {
	var contentLength int
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header failed: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header in DAP message")
	}

	content := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, content); err != nil {
		return nil, fmt.Errorf("read content failed: %w", err)
	}

	var msg DAPMessage
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal DAP message failed: %w", err)
	}

	return &msg, nil
}

func (c *DAPClient) readEvent() (*DAPEvent, error) {
	msg, err := c.readMessage()
	if err != nil {
		return nil, err
	}

	if msg.Type != "event" {
		return nil, fmt.Errorf("expected event, got %s", msg.Type)
	}

	return &DAPEvent{
		Seq:   msg.Seq,
		Type:  msg.Type,
		Event: msg.Event,
		Body:  msg.Body,
	}, nil
}

func (c *DAPClient) readEvents() {
	defer close(c.events)
	for {
		select {
		case <-c.done:
			return
		default:
		}

		evt, err := c.readEvent()
		if err != nil {
			select {
			case <-c.done:
			default:
				slog.Warn("dap: event reader error", "error", err)
			}
			return
		}

		select {
		case c.events <- evt:
		case <-c.done:
			return
		}
	}
}

func (c *DAPClient) parseBreakpoints(body map[string]any) []Breakpoint {
	if body == nil {
		return nil
	}

	raw, ok := body["breakpoints"]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]Breakpoint, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		bp := Breakpoint{
			ID:       int(toFloat64(m["id"])),
			Line:     int(toFloat64(m["line"])),
			Verified: toBool(m["verified"]),
		}
		if src, ok := m["source"].(map[string]any); ok {
			bp.Source = &Source{
				Name: toString(src["name"]),
				Path: toString(src["path"]),
			}
		}
		result = append(result, bp)
	}

	return result
}

func (c *DAPClient) parseStackFrames(body map[string]any) []StackFrame {
	if body == nil {
		return nil
	}

	raw, ok := body["stackFrames"]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]StackFrame, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sf := StackFrame{
			ID:     int(toFloat64(m["id"])),
			Name:   toString(m["name"]),
			Line:   int(toFloat64(m["line"])),
			Column: int(toFloat64(m["column"])),
		}
		if src, ok := m["source"].(map[string]any); ok {
			sf.Source = &Source{
				Name: toString(src["name"]),
				Path: toString(src["path"]),
			}
		}
		result = append(result, sf)
	}

	return result
}

func (c *DAPClient) parseScopes(body map[string]any) []Scope {
	if body == nil {
		return nil
	}

	raw, ok := body["scopes"]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]Scope, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, Scope{
			Name:               toString(m["name"]),
			VariablesReference: int(toFloat64(m["variablesReference"])),
			Expensive:          toBool(m["expensive"]),
		})
	}

	return result
}

func (c *DAPClient) parseVariables(body map[string]any) []Variable {
	if body == nil {
		return nil
	}

	raw, ok := body["variables"]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]Variable, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, Variable{
			Name:               toString(m["name"]),
			Value:              toString(m["value"]),
			Type:               toString(m["type"]),
			VariablesReference: int(toFloat64(m["variablesReference"])),
		})
	}

	return result
}

func toFloat64(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
