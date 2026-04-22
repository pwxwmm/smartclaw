package autopilot

func defaultRules() []AutoRule {
	return []AutoRule{
		// Read-only tools — auto at TrustLevelRead
		{ToolName: "read_file", Action: ActionAllow, Condition: "read-only operation", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "glob", Action: ActionAllow, Condition: "read-only file listing", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "grep", Action: ActionAllow, Condition: "read-only content search", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "web_search", Action: ActionAllow, Condition: "read-only web search", TrustMin: TrustLevelRead, RiskScore: 0.05},
		{ToolName: "web_fetch", Action: ActionAllow, Condition: "read-only web fetch", TrustMin: TrustLevelRead, RiskScore: 0.05},
		{ToolName: "think", Action: ActionAllow, Condition: "internal reasoning", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "lsp", Action: ActionAllow, Condition: "read-only code analysis", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "ast_grep", Action: ActionAllow, Condition: "read-only AST search", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "tool_search", Action: ActionAllow, Condition: "read-only tool discovery", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "memory", Action: ActionAllow, Condition: "read-only memory access", TrustMin: TrustLevelRead, RiskScore: 0.05},
		{ToolName: "skill", Action: ActionAllow, Condition: "read-only skill loading", TrustMin: TrustLevelRead, RiskScore: 0.05},
		{ToolName: "git_status", Action: ActionAllow, Condition: "read-only git status", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "git_diff", Action: ActionAllow, Condition: "read-only git diff", TrustMin: TrustLevelRead, RiskScore: 0.0},
		{ToolName: "git_log", Action: ActionAllow, Condition: "read-only git log", TrustMin: TrustLevelRead, RiskScore: 0.0},

		// Write tools — auto at TrustLevelWrite
		{ToolName: "write_file", Action: ActionAllow, Condition: "file write within workspace", TrustMin: TrustLevelWrite, RiskScore: 0.3},
		{ToolName: "edit_file", Action: ActionAllow, Condition: "file edit within workspace", TrustMin: TrustLevelWrite, RiskScore: 0.3},
		{ToolName: "todowrite", Action: ActionAllow, Condition: "todo list management", TrustMin: TrustLevelWrite, RiskScore: 0.1},
		{ToolName: "session", Action: ActionAllow, Condition: "session management", TrustMin: TrustLevelWrite, RiskScore: 0.1},

		// Execute tools — auto at TrustLevelExecute
		{ToolName: "bash", Action: ActionAllow, Condition: "shell command execution", TrustMin: TrustLevelExecute, RiskScore: 0.5},
		{ToolName: "execute_code", Action: ActionAllow, Condition: "code execution", TrustMin: TrustLevelExecute, RiskScore: 0.5},
		{ToolName: "docker_exec", Action: ActionAllow, Condition: "docker execution", TrustMin: TrustLevelExecute, RiskScore: 0.6},
		{ToolName: "git_ai", Action: ActionAllow, Condition: "git AI operations (commit, review)", TrustMin: TrustLevelExecute, RiskScore: 0.5},

		// Dangerous tools — always ask unless TrustLevelFull
		{ToolName: "agent", Action: ActionAllow, Condition: "sub-agent spawning", TrustMin: TrustLevelFull, RiskScore: 0.7},
		{ToolName: "mcp", Action: ActionAllow, Condition: "MCP server execution", TrustMin: TrustLevelFull, RiskScore: 0.8},
		{ToolName: "powershell", Action: ActionAllow, Condition: "PowerShell execution", TrustMin: TrustLevelFull, RiskScore: 0.9},

		// Always-deny tools regardless of trust level
		{ToolName: "browser_navigate", Action: ActionDeny, Condition: "browser automation requires explicit approval", TrustMin: TrustLevelFull, RiskScore: 1.0},
	}
}
