package mcp

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type McpServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
	OAuth     *McpOAuthConfig   `json:"oauth,omitempty"`
}

type McpOAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthorizeURL string   `json:"authorize_url"`
	TokenURL     string   `json:"token_url"`
	Scopes       []string `json:"scopes,omitempty"`
}

type McpTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	InputSchema any              `json:"inputSchema,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

type ToolAnnotations struct {
	ReadOnly    bool `json:"readOnly,omitempty"`
	Idempotent  bool `json:"idempotent,omitempty"`
	SideEffects bool `json:"sideEffects,omitempty"`
}

type McpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type McpResourceTemplate struct {
	URI         string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type McpPrompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type McpConnection struct {
	Config    *McpServerConfig
	Tools     []McpTool
	Resources []McpResource
	Prompts   []McpPrompt
}

type McpRegistry struct {
	connections map[string]*McpConnection
}

func NewRegistry() *McpRegistry {
	return &McpRegistry{
		connections: make(map[string]*McpConnection),
	}
}

func (r *McpRegistry) Add(name string, conn *McpConnection) {
	r.connections[name] = conn
}

func (r *McpRegistry) Get(name string) *McpConnection {
	return r.connections[name]
}

func (r *McpRegistry) Remove(name string) {
	delete(r.connections, name)
}

func (r *McpRegistry) List() []*McpConnection {
	conns := make([]*McpConnection, 0, len(r.connections))
	for _, conn := range r.connections {
		conns = append(conns, conn)
	}
	return conns
}

func convertSDKTools(sdkTools []*sdk.Tool) []McpTool {
	tools := make([]McpTool, 0, len(sdkTools))
	for _, t := range sdkTools {
		tool := McpTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
		if t.Annotations != nil {
			tool.Annotations = &ToolAnnotations{}
		}
		tools = append(tools, tool)
	}
	return tools
}

func convertSDKResources(sdkResources []*sdk.Resource) []McpResource {
	resources := make([]McpResource, 0, len(sdkResources))
	for _, r := range sdkResources {
		resources = append(resources, McpResource{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MIMEType,
		})
	}
	return resources
}
