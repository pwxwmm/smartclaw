package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LSPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  any `json:"params,omitempty"`
}

type LSPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  any `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    any `json:"data,omitempty"`
}

// LSPClient manages connection to an LSP server
type LSPClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	reader      *bufio.Reader
	requestID   int
	mu          sync.Mutex
	initialized bool
	fileURI     string
}

// LSPServerConfig defines LSP server configuration
type LSPServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}

// Language server mappings
var languageServers = map[string]*LSPServerConfig{
	".go": {
		Command: "gopls",
		Args:    []string{"serve"},
	},
	".ts": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	".tsx": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	".js": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	".jsx": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	".py": {
		Command: "pylsp",
		Args:    []string{},
	},
	".rs": {
		Command: "rust-analyzer",
		Args:    []string{},
	},
}

// NewLSPClient creates a new LSP client for the given file
func NewLSPClient(filePath string) (*LSPClient, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	config, ok := languageServers[ext]
	if !ok {
		return nil, fmt.Errorf("no LSP server configured for file type: %s", ext)
	}

	// Check if server is available
	if _, err := exec.LookPath(config.Command); err != nil {
		return nil, fmt.Errorf("LSP server not found: %s", config.Command)
	}

	cmd := exec.Command(config.Command, config.Args...)

	// Set environment
	if len(config.Env) > 0 {
		env := os.Environ()
		for k, v := range config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	client := &LSPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		reader:  bufio.NewReader(stdout),
		fileURI: "file://" + filePath,
	}

	return client, nil
}

// Initialize performs LSP initialization handshake
func (c *LSPClient) Initialize(ctx context.Context, rootPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   "file://" + rootPath,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"definition": map[string]any{
					"linkSupport": true,
				},
				"references": map[string]any{},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"rename": map[string]any{
					"prepareSupport": true,
				},
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification
	_, _ = c.sendRequest(ctx, "initialized", map[string]any{})

	c.initialized = true
	return nil
}

// DidOpen notifies the server that a file was opened
func (c *LSPClient) DidOpen(ctx context.Context, filePath, languageID, content string) error {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        "file://" + filePath,
			"languageId": languageID,
			"version":    1,
			"text":       content,
		},
	}

	_, err := c.sendNotification(ctx, "textDocument/didOpen", params)
	return err
}

// GotoDefinition requests definition location
func (c *LSPClient) GotoDefinition(ctx context.Context, filePath string, line, character int) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
		"position": map[string]any{
			"line":      line - 1, // LSP uses 0-based
			"character": character - 1,
		},
	}

	resp, err := c.sendRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("definition error: %s", resp.Error.Message)
	}

	return c.parseLocation(resp.Result), nil
}

// FindReferences finds all references to a symbol
func (c *LSPClient) FindReferences(ctx context.Context, filePath string, line, character int) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
		"position": map[string]any{
			"line":      line - 1,
			"character": character - 1,
		},
		"context": map[string]any{
			"includeDeclaration": true,
		},
	}

	resp, err := c.sendRequest(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("references error: %s", resp.Error.Message)
	}

	return c.parseLocations(resp.Result), nil
}

// DocumentSymbols gets all symbols in a document
func (c *LSPClient) DocumentSymbols(ctx context.Context, filePath string) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
	}

	resp, err := c.sendRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("symbols error: %s", resp.Error.Message)
	}

	return c.parseSymbols(resp.Result), nil
}

// Hover gets hover information
func (c *LSPClient) Hover(ctx context.Context, filePath string, line, character int) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
		"position": map[string]any{
			"line":      line - 1,
			"character": character - 1,
		},
	}

	resp, err := c.sendRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("hover error: %s", resp.Error.Message)
	}

	return c.parseHover(resp.Result), nil
}

// Rename renames a symbol
func (c *LSPClient) Rename(ctx context.Context, filePath string, line, character int, newName string) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
		"position": map[string]any{
			"line":      line - 1,
			"character": character - 1,
		},
		"newName": newName,
	}

	resp, err := c.sendRequest(ctx, "textDocument/rename", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rename error: %s", resp.Error.Message)
	}

	return resp.Result, nil
}

// sendRequest sends a JSON-RPC request
func (c *LSPClient) sendRequest(ctx context.Context, method string, params any) (*LSPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	req := LSPRequest{
		JSONRPC: "2.0",
		ID:      c.requestID,
		Method:  method,
		Params:  params,
	}

	// Marshal request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Write with Content-Length header
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(reqBytes))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(reqBytes); err != nil {
		return nil, err
	}

	// Read response
	return c.readResponse(ctx)
}

// sendNotification sends a JSON-RPC notification
func (c *LSPClient) sendNotification(ctx context.Context, method string, params any) (*LSPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := LSPRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(reqBytes))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(reqBytes); err != nil {
		return nil, err
	}

	return nil, nil
}

// readResponse reads a JSON-RPC response
func (c *LSPClient) readResponse(ctx context.Context) (*LSPResponse, error) {
	// Read Content-Length header
	var contentLength int
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("no Content-Length header")
	}

	// Read content
	content := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, content); err != nil {
		return nil, err
	}

	var resp LSPResponse
	if err := json.Unmarshal(content, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// parseLocation parses a location result
func (c *LSPClient) parseLocation(result any) any {
	if result == nil {
		return map[string]any{
			"locations": []any{},
		}
	}

	// Single location
	if loc, ok := result.(map[string]any); ok {
		return map[string]any{
			"locations": []any{c.formatLocation(loc)},
		}
	}

	// Array of locations
	if locs, ok := result.([]any); ok {
		locations := make([]any, 0, len(locs))
		for _, loc := range locs {
			if l, ok := loc.(map[string]any); ok {
				locations = append(locations, c.formatLocation(l))
			}
		}
		return map[string]any{
			"locations": locations,
		}
	}

	return map[string]any{
		"locations": []any{},
	}
}

// parseLocations parses multiple locations
func (c *LSPClient) parseLocations(result any) any {
	if result == nil {
		return map[string]any{
			"references": []any{},
		}
	}

	if locs, ok := result.([]any); ok {
		references := make([]any, 0, len(locs))
		for _, loc := range locs {
			if l, ok := loc.(map[string]any); ok {
				references = append(references, c.formatLocation(l))
			}
		}
		return map[string]any{
			"references": references,
			"count":      len(references),
		}
	}

	return map[string]any{
		"references": []any{},
	}
}

// parseSymbols parses document symbols
func (c *LSPClient) parseSymbols(result any) any {
	if result == nil {
		return map[string]any{
			"symbols": []any{},
		}
	}

	if symbols, ok := result.([]any); ok {
		parsed := make([]any, 0, len(symbols))
		for _, sym := range symbols {
			if s, ok := sym.(map[string]any); ok {
				parsed = append(parsed, map[string]any{
					"name":   s["name"],
					"kind":   s["kind"],
					"range":  s["range"],
					"detail": s["detail"],
				})
			}
		}
		return map[string]any{
			"symbols": parsed,
			"count":   len(parsed),
		}
	}

	return map[string]any{
		"symbols": []any{},
	}
}

// parseHover parses hover result
func (c *LSPClient) parseHover(result any) any {
	if result == nil {
		return map[string]any{
			"contents": nil,
		}
	}

	if hover, ok := result.(map[string]any); ok {
		return map[string]any{
			"contents": hover["contents"],
			"range":    hover["range"],
		}
	}

	return map[string]any{
		"contents": result,
	}
}

// formatLocation formats an LSP location
func (c *LSPClient) formatLocation(loc map[string]any) map[string]any {
	uri, _ := loc["uri"].(string)
	// Remove file:// prefix
	if strings.HasPrefix(uri, "file://") {
		uri = uri[7:]
	}

	result := map[string]any{
		"uri": uri,
	}

	if range_, ok := loc["range"].(map[string]any); ok {
		if start, ok := range_["start"].(map[string]any); ok {
			line, _ := start["line"].(float64)
			char, _ := start["character"].(float64)
			result["line"] = int(line) + 1 // Convert to 1-based
			result["character"] = int(char) + 1
		}
	}

	return result
}

// Close closes the LSP client
func (c *LSPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

// DidChange notifies the server that a file was modified
func (c *LSPClient) DidChange(ctx context.Context, filePath, content string) error {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":     "file://" + filePath,
			"version": time.Now().Unix(),
		},
		"contentChanges": []map[string]any{
			{
				"text": content,
			},
		},
	}

	_, err := c.sendNotification(ctx, "textDocument/didChange", params)
	return err
}

// Completion requests code completions at a position
func (c *LSPClient) Completion(ctx context.Context, filePath string, line, character int) (any, error) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file://" + filePath,
		},
		"position": map[string]any{
			"line":      line - 1,
			"character": character - 1,
		},
	}

	resp, err := c.sendRequest(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("completion error: %s", resp.Error.Message)
	}

	return c.parseCompletion(resp.Result), nil
}

// parseCompletion parses completion results
func (c *LSPClient) parseCompletion(result any) any {
	if result == nil {
		return map[string]any{
			"items": []any{},
		}
	}

	if list, ok := result.(map[string]any); ok {
		if items, ok := list["items"].([]any); ok {
			return map[string]any{
				"items": items,
				"count": len(items),
			}
		}
		return list
	}

	if items, ok := result.([]any); ok {
		return map[string]any{
			"items": items,
			"count": len(items),
		}
	}

	return map[string]any{
		"items": []any{},
	}
}

// CompletionItem resolves additional information for a completion item
func (c *LSPClient) ResolveCompletionItem(ctx context.Context, item any) (any, error) {
	params := map[string]any{
		"data": item,
	}

	resp, err := c.sendRequest(ctx, "completionItem/resolve", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resolve error: %s", resp.Error.Message)
	}

	return resp.Result, nil
}

// LSPDiagnostic represents a diagnostic from the LSP server
type LSPDiagnostic struct {
	Range    LSPRange `json:"range"`
	Severity int      `json:"severity"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

// LSPRange represents a range in a document
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPPosition represents a position in a document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// DiagnosticSeverity constants
const (
	DiagnosticError       = 1
	DiagnosticWarning     = 2
	DiagnosticInformation = 3
	DiagnosticHint        = 4
)

// LSPClientCache caches LSP clients by file extension
var lspClientCache = make(map[string]*LSPClient)
var lspClientMutex sync.Mutex

// GetOrCreateLSPClient gets or creates an LSP client for a file
func GetOrCreateLSPClient(filePath, rootPath string) (*LSPClient, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	lspClientMutex.Lock()
	defer lspClientMutex.Unlock()

	client, ok := lspClientCache[ext]
	if ok && client != nil {
		return client, nil
	}

	client, err := NewLSPClient(filePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Initialize(ctx, rootPath); err != nil {
		client.Close()
		return nil, err
	}

	lspClientCache[ext] = client
	return client, nil
}

// CloseAllLSPPClients closes all cached LSP clients
func CloseAllLSPPClients() {
	lspClientMutex.Lock()
	defer lspClientMutex.Unlock()

	for ext, client := range lspClientCache {
		if client != nil {
			client.Close()
		}
		delete(lspClientCache, ext)
	}
}
