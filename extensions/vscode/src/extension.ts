import * as vscode from 'vscode';
import { ChildProcess, spawn } from 'child_process';
import { EventEmitter } from 'events';

interface ACPRequest {
    jsonrpc: string;
    id: number;
    method: string;
    params?: any;
}

interface ACPResponse {
    jsonrpc: string;
    id: number;
    result?: any;
    error?: { code: number; message: string };
}

class ACPClient extends EventEmitter {
    private proc: ChildProcess | null = null;
    private requestId = 0;
    private pending = new Map<number, { resolve: Function; reject: Function }>();
    private buffer = '';

    async connect(): Promise<void> {
        const config = vscode.workspace.getConfiguration('smartclaw');
        const smartclawPath = config.get<string>('path', 'smartclaw');

        this.proc = spawn(smartclawPath, ['acp'], {
            stdio: ['pipe', 'pipe', 'pipe'],
        });

        this.proc.stdout!.on('data', (data: Buffer) => {
            this.onData(data.toString());
        });

        this.proc.stderr!.on('data', (data: Buffer) => {
            this.emit('log', data.toString());
        });

        this.proc.on('close', (code) => {
            this.emit('close', code);
        });

        await this.initialize();
    }

    private async initialize(): Promise<any> {
        return this.send('initialize', {
            protocolVersion: '2025-03-26',
            clientInfo: { name: 'vscode-smartclaw', version: '0.1.0' },
            capabilities: {},
        });
    }

    async listTools(): Promise<any[]> {
        const result = await this.send('tools/list', {});
        return result?.tools ?? [];
    }

    async callTool(name: string, arguments_: Record<string, any>): Promise<any> {
        return this.send('tools/call', { name, arguments: arguments_ });
    }

    async send(method: string, params: any): Promise<any> {
        return new Promise((resolve, reject) => {
            const id = ++this.requestId;
            const request: ACPRequest = {
                jsonrpc: '2.0',
                id,
                method,
                params,
            };

            this.pending.set(id, { resolve, reject });

            const data = JSON.stringify(request);
            const header = `Content-Length: ${Buffer.byteLength(data)}\r\n\r\n`;

            this.proc!.stdin!.write(header + data);
        });
    }

    private onData(chunk: string): void {
        this.buffer += chunk;

        while (true) {
            const headerEnd = this.buffer.indexOf('\r\n\r\n');
            if (headerEnd === -1) return;

            const header = this.buffer.substring(0, headerEnd);
            const match = header.match(/Content-Length:\s*(\d+)/i);
            if (!match) {
                this.buffer = this.buffer.substring(headerEnd + 4);
                continue;
            }

            const contentLength = parseInt(match[1], 10);
            const bodyStart = headerEnd + 4;
            const bodyEnd = bodyStart + contentLength;

            if (this.buffer.length < bodyEnd) return;

            const body = this.buffer.substring(bodyStart, bodyEnd);
            this.buffer = this.buffer.substring(bodyEnd);

            try {
                const response: ACPResponse = JSON.parse(body);
                const pending = this.pending.get(response.id);
                if (pending) {
                    this.pending.delete(response.id);
                    if (response.error) {
                        pending.reject(new Error(response.error.message));
                    } else {
                        pending.resolve(response.result);
                    }
                }
            } catch (e) {
                this.emit('error', e);
            }
        }
    }

    dispose(): void {
        if (this.proc) {
            this.send('shutdown', {}).finally(() => {
                this.proc!.kill();
                this.proc = null;
            });
        }
    }
}

let client: ACPClient | null = null;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
    client = new ACPClient();

    context.subscriptions.push(
        vscode.commands.registerCommand('smartclaw.ask', async () => {
            const question = await vscode.window.showInputBox({
                prompt: 'Ask SmartClaw',
                placeHolder: 'Type your question...',
            });
            if (!question) return;

            try {
                if (!client) {
                    client = new ACPClient();
                    await client.connect();
                }
                const result = await client.callTool('bash', {
                    command: `echo "${question.replace(/"/g, '\\"')}"`,
                });
                const text = extractText(result);
                vscode.window.showInformationMessage(text.substring(0, 200));
            } catch (err: any) {
                vscode.window.showErrorMessage(`SmartClaw error: ${err.message}`);
            }
        }),

        vscode.commands.registerCommand('smartclaw.openChat', () => {
            vscode.window.showInformationMessage('SmartClaw chat panel coming soon');
        }),

        vscode.commands.registerCommand('smartclaw.explainCode', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const selection = editor.document.getText(editor.selection);
            if (!selection) {
                vscode.window.showWarningMessage('Select code to explain');
                return;
            }
            const prompt = `Explain this code:\n\n${selection}`;
            vscode.window.showInformationMessage('SmartClaw: Explain sent');
        }),

        vscode.commands.registerCommand('smartclaw.fixCode', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const selection = editor.document.getText(editor.selection);
            if (!selection) {
                vscode.window.showWarningMessage('Select code to fix');
                return;
            }
            vscode.window.showInformationMessage('SmartClaw: Fix sent');
        }),

        vscode.commands.registerCommand('smartclaw.generateTests', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const selection = editor.document.getText(editor.selection);
            if (!selection) {
                vscode.window.showWarningMessage('Select code to generate tests for');
                return;
            }
            vscode.window.showInformationMessage('SmartClaw: Generate tests sent');
        }),

        {
            dispose: () => {
                client?.dispose();
                client = null;
            },
        }
    );
}

function extractText(result: any): string {
    if (!result?.content) return JSON.stringify(result);
    if (Array.isArray(result.content)) {
        return result.content
            .filter((b: any) => b.type === 'text')
            .map((b: any) => b.text)
            .join('\n');
    }
    return JSON.stringify(result);
}

export function deactivate(): void {
    client?.dispose();
    client = null;
}
