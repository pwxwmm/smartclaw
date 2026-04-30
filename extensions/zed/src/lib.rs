use zed_extension_api::{
    self as zed, Command, ContextServerConfiguration, ContextServerId, Extension, Project,
    Result, SlashCommand, SlashCommandArgumentCompletion, SlashCommandOutput,
    SlashCommandOutputSection, Worktree,
};
use serde::{Deserialize, Serialize};

#[derive(Serialize)]
struct JsonRpcRequest {
    jsonrpc: &'static str,
    id: u64,
    method: &'static str,
    params: serde_json::Value,
}

#[derive(Deserialize, Debug)]
struct JsonRpcResponse {
    #[allow(dead_code)]
    jsonrpc: String,
    #[allow(dead_code)]
    id: u64,
    result: Option<serde_json::Value>,
    error: Option<JsonRpcError>,
}

#[derive(Deserialize, Debug)]
struct JsonRpcError {
    #[allow(dead_code)]
    code: i64,
    message: String,
}

#[derive(Deserialize, Debug)]
#[allow(dead_code)]
struct InitializeResult {
    protocol_version: String,
    server_info: ServerInfo,
}

#[derive(Deserialize, Debug)]
#[allow(dead_code)]
struct ServerInfo {
    name: String,
    version: String,
}

#[derive(Deserialize, Debug)]
#[allow(dead_code)]
struct ListToolsResult {
    tools: Vec<ToolInfo>,
}

#[derive(Deserialize, Debug)]
#[allow(dead_code)]
struct ToolInfo {
    name: String,
    description: String,
}

#[derive(Deserialize, Debug)]
struct CallToolResult {
    content: Vec<ContentBlock>,
    #[serde(default)]
    is_error: bool,
}

#[derive(Deserialize, Debug)]
struct ContentBlock {
    #[serde(rename = "type")]
    _type: String,
    text: Option<String>,
}

fn extract_text(result: &CallToolResult) -> String {
    result
        .content
        .iter()
        .filter_map(|block| block.text.clone())
        .collect::<Vec<_>>()
        .join("\n")
}

/// Content-Length framing: `Content-Length: N\r\n\r\n{json}` (MCP/LSP/ACP)
#[allow(dead_code)]
fn frame_message(request: &JsonRpcRequest) -> Vec<u8> {
    let body =
        serde_json::to_vec(request).expect("JSON-RPC request serialization should not fail");
    let header = format!("Content-Length: {}\r\n\r\n", body.len());
    let mut framed = header.into_bytes();
    framed.extend_from_slice(&body);
    framed
}

#[allow(dead_code)]
fn parse_response(data: &[u8]) -> Option<JsonRpcResponse> {
    let s = std::str::from_utf8(data).ok()?;
    let header_end = s.find("\r\n\r\n")?;
    let header = &s[..header_end];
    let len: usize = header.split(':').nth(1)?.trim().parse().ok()?;
    let body_start = header_end + 4;
    let body_end = body_start + len;
    if body_end > s.len() {
        return None;
    }
    let body = &s[body_start..body_end];
    serde_json::from_str(body).ok()
}

/// ACP client for building JSON-RPC 2.0 requests targeting `smartclaw acp`.
///
/// Zed extensions run as WASM and cannot spawn subprocesses directly.
/// Instead, `context_server_command` tells Zed to spawn `smartclaw acp`
/// and manage the MCP/ACP stdio communication. This client encodes the
/// protocol request format so tests can verify framing correctness.
#[allow(dead_code)]
struct AcpClient {
    request_id: u64,
}

#[allow(dead_code)]
impl AcpClient {
    fn new() -> Self {
        Self { request_id: 0 }
    }

    fn next_id(&mut self) -> u64 {
        self.request_id += 1;
        self.request_id
    }

    fn initialize_request(&mut self) -> Vec<u8> {
        let id = self.next_id();
        frame_message(&JsonRpcRequest {
            jsonrpc: "2.0",
            id,
            method: "initialize",
            params: serde_json::json!({
                "protocolVersion": "2025-03-26",
                "clientInfo": { "name": "zed-smartclaw", "version": "0.1.0" },
                "capabilities": {},
            }),
        })
    }

    fn list_tools_request(&mut self) -> Vec<u8> {
        let id = self.next_id();
        frame_message(&JsonRpcRequest {
            jsonrpc: "2.0",
            id,
            method: "tools/list",
            params: serde_json::json!({}),
        })
    }

    fn call_tool_request(&mut self, tool_name: &str, arguments: serde_json::Value) -> Vec<u8> {
        let id = self.next_id();
        frame_message(&JsonRpcRequest {
            jsonrpc: "2.0",
            id,
            method: "tools/call",
            params: serde_json::json!({
                "name": tool_name,
                "arguments": arguments,
            }),
        })
    }

    fn shutdown_request(&mut self) -> Vec<u8> {
        let id = self.next_id();
        frame_message(&JsonRpcRequest {
            jsonrpc: "2.0",
            id,
            method: "shutdown",
            params: serde_json::json!({}),
        })
    }
}

enum SubCommand {
    Ask { prompt: String },
    Explain,
    Fix,
    GenerateTests,
    Raw { prompt: String },
}

impl SubCommand {
    fn parse(input: &str) -> Self {
        let input = input.trim();
        if let Some(rest) = input.strip_prefix("ask ") {
            SubCommand::Ask {
                prompt: rest.to_string(),
            }
        } else if input == "ask" {
            SubCommand::Ask {
                prompt: String::new(),
            }
        } else if input == "explain" {
            SubCommand::Explain
        } else if input == "fix" {
            SubCommand::Fix
        } else if input == "generate-tests" || input == "test" || input == "tests" {
            SubCommand::GenerateTests
        } else if input.is_empty() {
            SubCommand::Raw {
                prompt: String::new(),
            }
        } else {
            SubCommand::Raw {
                prompt: input.to_string(),
            }
        }
    }

    fn to_prompt(&self) -> String {
        match self {
            SubCommand::Ask { prompt } if prompt.is_empty() => {
                "Please ask a question after `/smartclaw ask`, e.g. `/smartclaw ask How do I read a file in Go?`".to_string()
            }
            SubCommand::Ask { prompt } => prompt.clone(),
            SubCommand::Explain => "Explain this code step by step, including what it does, how it works, and any notable patterns or potential issues.".to_string(),
            SubCommand::Fix => "Analyze this code for bugs, errors, or issues. Propose fixes with explanations for each change.".to_string(),
            SubCommand::GenerateTests => "Generate comprehensive tests for this code, including edge cases and error scenarios.".to_string(),
            SubCommand::Raw { prompt } if prompt.is_empty() => {
                "Usage: /smartclaw <ask|explain|fix|test> [prompt]\n\nExamples:\n  /smartclaw ask How do I parse JSON in Rust?\n  /smartclaw explain\n  /smartclaw fix\n  /smartclaw test".to_string()
            }
            SubCommand::Raw { prompt } => prompt.clone(),
        }
    }

    fn section_label(&self) -> String {
        match self {
            SubCommand::Ask { .. } => "SmartClaw: Ask".to_string(),
            SubCommand::Explain => "SmartClaw: Explain".to_string(),
            SubCommand::Fix => "SmartClaw: Fix".to_string(),
            SubCommand::GenerateTests => "SmartClaw: Generate Tests".to_string(),
            SubCommand::Raw { .. } => "SmartClaw".to_string(),
        }
    }
}

struct SmartClawExtension;

impl Extension for SmartClawExtension {
    fn new() -> Self {
        Self
    }

    /// Zed spawns `smartclaw acp` as a context server and communicates via
    /// MCP/ACP stdio JSON-RPC (Content-Length framing). All SmartClaw tools
    /// become available in Zed's assistant panel through this mechanism.
    fn context_server_command(
        &mut self,
        _context_server_id: &ContextServerId,
        _project: &Project,
    ) -> Result<Command> {
        Ok(Command {
            command: "smartclaw".to_string(),
            args: vec!["acp".to_string()],
            env: vec![],
        })
    }

    fn context_server_configuration(
        &mut self,
        _context_server_id: &ContextServerId,
        _project: &Project,
    ) -> Result<Option<ContextServerConfiguration>> {
        Ok(None)
    }

    /// `/smartclaw` slash command — formats user input as a prompt for
    /// Zed's assistant panel. ACP communication is handled by the
    /// context server above.
    ///
    ///   /smartclaw ask <question>
    ///   /smartclaw explain
    ///   /smartclaw fix
    ///   /smartclaw test
    ///   /smartclaw <any text>  (shorthand for ask)
    fn run_slash_command(
        &self,
        command: SlashCommand,
        args: Vec<String>,
        _worktree: Option<&Worktree>,
    ) -> Result<SlashCommandOutput> {
        if command.name != "smartclaw" {
            return Err("Unknown slash command".into());
        }

        let input = args.join(" ");
        let subcmd = SubCommand::parse(&input);
        let text = subcmd.to_prompt();
        let label = subcmd.section_label();

        Ok(SlashCommandOutput {
            text: text.clone(),
            sections: vec![SlashCommandOutputSection {
                range: (0..text.len()).into(),
                label,
            }],
        })
    }

    fn complete_slash_command_argument(
        &self,
        _command: SlashCommand,
        args: Vec<String>,
    ) -> Result<Vec<SlashCommandArgumentCompletion>> {
        let current = args.join(" ");

        if current.is_empty() {
            return Ok(vec![
                SlashCommandArgumentCompletion {
                    label: "ask".to_string(),
                    new_text: "ask ".to_string(),
                    run_command: false,
                },
                SlashCommandArgumentCompletion {
                    label: "explain".to_string(),
                    new_text: "explain".to_string(),
                    run_command: true,
                },
                SlashCommandArgumentCompletion {
                    label: "fix".to_string(),
                    new_text: "fix".to_string(),
                    run_command: true,
                },
                SlashCommandArgumentCompletion {
                    label: "test".to_string(),
                    new_text: "test".to_string(),
                    run_command: true,
                },
            ]);
        }

        let subcommands = ["ask", "explain", "fix", "test", "generate-tests"];
        let matching: Vec<_> = subcommands
            .iter()
            .filter(|sc| sc.starts_with(&current))
            .map(|sc| {
                let new_text = if *sc == "ask" {
                    "ask ".to_string()
                } else {
                    sc.to_string()
                };
                let run_command = *sc != "ask";
                SlashCommandArgumentCompletion {
                    label: sc.to_string(),
                    new_text,
                    run_command,
                }
            })
            .collect();

        Ok(matching)
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed_extension_api::LanguageServerId,
        _worktree: &Worktree,
    ) -> Result<Command> {
        Err("SmartClaw is not a language server".into())
    }
}

zed::register_extension!(SmartClawExtension);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_subcommand_parse_ask() {
        let sc = SubCommand::parse("ask How do I read a file?");
        match sc {
            SubCommand::Ask { prompt } => assert_eq!(prompt, "How do I read a file?"),
            _ => panic!("Expected Ask variant"),
        }
    }

    #[test]
    fn test_subcommand_parse_explain() {
        let sc = SubCommand::parse("explain");
        match sc {
            SubCommand::Explain => {}
            _ => panic!("Expected Explain variant"),
        }
    }

    #[test]
    fn test_subcommand_parse_fix() {
        let sc = SubCommand::parse("fix");
        match sc {
            SubCommand::Fix => {}
            _ => panic!("Expected Fix variant"),
        }
    }

    #[test]
    fn test_subcommand_parse_test() {
        assert!(matches!(
            SubCommand::parse("test"),
            SubCommand::GenerateTests
        ));
        assert!(matches!(
            SubCommand::parse("tests"),
            SubCommand::GenerateTests
        ));
        assert!(matches!(
            SubCommand::parse("generate-tests"),
            SubCommand::GenerateTests
        ));
    }

    #[test]
    fn test_subcommand_parse_raw() {
        let sc = SubCommand::parse("How do I parse JSON?");
        match sc {
            SubCommand::Raw { prompt } => assert_eq!(prompt, "How do I parse JSON?"),
            _ => panic!("Expected Raw variant"),
        }
    }

    #[test]
    fn test_subcommand_to_prompt() {
        let sc = SubCommand::parse("explain");
        let prompt = sc.to_prompt();
        assert!(prompt.contains("Explain this code"));
    }

    #[test]
    fn test_frame_message() {
        let mut client = AcpClient::new();
        let framed = client.initialize_request();
        let s = std::str::from_utf8(&framed).unwrap();
        assert!(s.starts_with("Content-Length:"));
        assert!(s.contains("\"method\":\"initialize\""));
    }

    #[test]
    fn test_parse_response() {
        let body = r#"{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","serverInfo":{"name":"smartclaw","version":"1.0.0"}}}"#;
        let framed = format!("Content-Length: {}\r\n\r\n{}", body.len(), body);
        let resp = parse_response(framed.as_bytes()).unwrap();
        assert_eq!(resp.id, 1);
        assert!(resp.result.is_some());
    }

    #[test]
    fn test_extract_text() {
        let result = CallToolResult {
            content: vec![
                ContentBlock {
                    _type: "text".to_string(),
                    text: Some("Hello".to_string()),
                },
                ContentBlock {
                    _type: "text".to_string(),
                    text: Some("World".to_string()),
                },
            ],
            is_error: false,
        };
        assert_eq!(extract_text(&result), "Hello\nWorld");
    }

    #[test]
    fn test_acp_client_tools_call() {
        let mut client = AcpClient::new();
        let framed = client.call_tool_request("bash", serde_json::json!({ "command": "echo hello" }));
        let s = std::str::from_utf8(&framed).unwrap();
        assert!(s.contains("\"method\":\"tools/call\""));
        assert!(s.contains("\"name\":\"bash\""));
    }
}
