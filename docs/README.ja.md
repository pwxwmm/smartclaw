[English](../README.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md)

> これは[英語版 README](../README.md)の翻訳です。英語版が正となります。

# SmartClaw

使い続けるほどワークフローを学習する、自己改善型 AI エージェント。

SmartClaw は、タスクをこなすたびに賢くなる自律型コーディングエージェントです。完了したタスクを評価し、再利用可能な手法を抽出し、スキルを自動生成する学習ループを備えています。使えば使うほど、あなたの働き方を深く理解するようになります。

## コア思想

**"越来越懂你的工作方式"** — 「すべてにおいて優れている」のではなく、「あなたのすべてにおいて優れている」エージェントを目指す。

セッション間で何も記憶しない汎用 AI アシスタントとは異なり、SmartClaw は:

- **完了したタスクから学習する** — 手法が再利用可能かを評価し、そうであればスキルとして抽出
- **セッションをまたいで記憶する** — SQLite + FTS5 全文検索による 4 層メモリシステム
- **時間とともに自己改善する** — 定期的なナッジがエージェントにメモリの統合とスキルの洗練を促す
- **あなたの好みを理解する** — コミュニケーションスタイル、知識背景、よく使うワークフローを受動的に追跡

## 機能

### エージェント機能

- **学習ループ**: タスク完了後の評価 → 手法抽出 → スキル作成 → MEMORY.md 自動更新
- **4 層メモリ**: プロンプトメモリ (MEMORY.md/USER.md)、セッション検索 (FTS5)、スキル手順 (遅延読み込み)、ユーザーモデリング
- **定期ナッジ**: 10 ターンごとにシステムが自己レビューをトリガー（設定可能）
- **スマート圧縮**: 設定可能な閾値による自動圧縮、ヘッド保護、ツール結果のプルーニング、ソース追跡可能な要約
- **投機的実行**: デュアルモデルルーティング。高速モデルと重厚モデルを並列実行し、結果が類似していれば高速モデルを採用、乖離していれば重厚モデルにフォールバック
- **アダプティブモデルルーター**: 複雑度に基づくモデル選択（fast/default/heavy）、コスト優先 / 品質優先 / バランス戦略
- **コストガード**: 予算認識型の利用制限。1 日 / セッション単位の上限、警告閾値、制限接近時の自動モデルダウングレード

### 開発ツール

- **73 以上の組み込みツール**: ファイル操作、コード分析、Web ツール、MCP 統合、ブラウザ自動化、Docker サンドボックスなど
- **101 のスラッシュコマンド**: エージェント管理、テンプレートシステム、IDE 統合を備えた生産性コマンドスイート
- **モダン TUI**: Bubble Tea で構築されたターミナルユーザーインターフェース
- **インタラクティブ REPL**: ストリーミングレスポンスによる完全な会話履歴
- **MCP 統合**: MCP サーバーへの接続、ツールの発見、リソースの読み取り、OAuth 認証
- **ACP サーバー**: stdio JSON-RPC 経由の IDE 統合（VS Code、Zed、JetBrains）用 Agent Communication Protocol
- **VS Code 拡張機能**: チャットサイドバー、コード説明、修正、テスト生成コマンドを備えた公式拡張機能
- **セキュア**: 4 モードのパーミッションシステム、Linux でのサンドボックス実行、Docker 分離
- **トークン追跡**: リアルタイムコスト推定と閾値での自動圧縮

### ブラウザ自動化

- **ヘッドレスブラウザ**: Chromium (chromedp 経由) を使用したナビゲーション、クリック、入力、スクリーンショット、コンテンツ抽出、フォーム入力
- **8 つのブラウザツール**: `browser_navigate`、`browser_click`、`browser_type`、`browser_screenshot`、`browser_extract`、`browser_wait`、`browser_select`、`browser_fill_form`

### コード実行とサンドボックス

- **コード実行ツール**: RPC サンドボックスで Python コードを実行し、SmartClaw ツール (read_file, write_file, glob, grep, bash, web_search, web_fetch) に直接アクセス。マルチターンのワークフローをシングルターンに圧縮
- **Docker サンドボックス**: プロジェクトディレクトリを `/workspace` にマウントした分離コンテナ実行。ワンショットとセッション永続の両方をサポート
- **Linux 名前空間サンドボックス**: Linux 名前空間を使用したネイティブサンドボックス実行による安全な分離

### ゲートウェイとクロスプラットフォーム

- **統合ゲートウェイ**: メッセージ → ルーティング → メモリ → 実行 → 学習 → 配信
- **プラットフォームアダプター**: ターミナル、Web UI、Telegram、Discord への拡張可能
- **Cron タスク**: 完全なメモリアクセスを持つファーストクラスのエージェントタスクとしてのスケジュールタスク
- **セッションルーティング**: プラットフォームベースではなく userID ベースのルーティング。デバイスを切り替えてもコンテキストを維持
- **セッション録画**: 監査とレビューのためのフルセッションの録画と再生
- **リモートトリガー**: SSH 経由でリモートホスト上のコマンドを実行

### チームコラボレーション

- **チームワークスペース**: AES 暗号化メモリ同期による共有チームスペースの作成
- **チームメモリ共有**: チームメンバー間でメモリ、セッション、ナレッジを共有
- **チームツール**: `team_create`、`team_delete`、`team_share_memory`、`team_get_memories`、`team_search_memories`、`team_sync`、`team_share_session`

### オブザーバビリティと分析

- **メトリクスダッシュボード**: リアルタイムのクエリ数、キャッシュヒット率、トークン使用量、コスト推定、ツール実行統計、モデル別クエリ数
- **分散トレーシング**: レイテンシと障害のデバッグのためのリクエストレベルトレーシング
- **テレメトリ API**: 完全なオブザーバビリティデータを公開する REST エンドポイント (`/api/telemetry`)

### バッチと RL 評価

- **バッチランナー**: 数百のプロンプトに対してエージェントを並列実行し、ShareGPT 形式の学習軌跡を出力
- **RL 評価**: 設定可能なメトリクス (exact_match, code_quality, length_penalty) による報酬ベースの評価ループ
- **軌跡エクスポート**: 強化学習研究のためのステップバイステップ報酬付きエピソードデータのエクスポート

### OpenAI 互換

- **OpenAI API 形式**: `--openai` フラグまたは設定による OpenAI 互換 API エンドポイントの完全サポート
- **カスタムベース URL**: `--url` フラグで任意の OpenAI 互換プロバイダーを指定
- **マルチプロバイダー**: Anthropic と OpenAI 互換バックエンド間のシームレスな切り替え

## アーキテクチャ

```
Input → Reasoning → Tool Use → Memory → Output → Learning
                                                   ↓
                                          Evaluate: worth keeping?
                                                   ↓ Yes
                                          Extract: reusable method
                                                   ↓
                                          Write: skill to disk
                                                   ↓
                                     Next time: use saved skill
```

### 4 層メモリシステム

| 層 | 名前 | ストレージ | 動作 |
|-------|------|---------|----------|
| L1 | プロンプトメモリ | `MEMORY.md` + `USER.md` | セッションごとに自動ロード、3,575 文字のハードリミット |
| L2 | セッション検索 | SQLite + FTS5 | エージェントが関連履歴を検索、注入前に LLM で要約 |
| L3 | スキル手順 | `~/.smartclaw/skills/` | スキル名と説明のみロード、フルコンテンツはオンデマンド |
| L4 | ユーザーモデリング | `user_observations` テーブル | 好みを受動的に追跡、USER.md を自動更新 |

### 学習ループ

```
Task Complete
    ↓
Evaluator: "Was this approach worth reusing?" (LLM judgment)
    ↓ Yes
Extractor: "What's the reusable method?" (LLM extraction)
    ↓
SkillWriter: Write SKILL.md to ~/.smartclaw/skills/
    ↓
Update MEMORY.md with learned pattern
    ↓
Next similar task → discovered and used automatically
```

### 投機的実行

```
User Query
    ├── Fast Model (Haiku) → result in ~1s
    └── Heavy Model (Opus) → result in ~5s
            ↓
    Compare: similarity > 0.7?
        ↓ Yes              ↓ No
    Use fast result    Use heavy result
```

### アダプティブモデルルーティング

```
Query Complexity Signals:
  - Message length
  - Tool call count
  - History turn count
  - Code content detection
  - Retry count
  - Skill match
        ↓
  Complexity Score → Route to Tier
        ↓
  fast | default | heavy
```

## クイックスタート

### 必要要件

- Go 1.25+
- Anthropic API キー（または OpenAI 互換 API キー）

### インストール

```bash
go build -o bin/smartclaw ./cmd/smartclaw/
```

### 基本的な使い方

```bash
# TUI モードで起動（推奨）
./bin/smartclaw tui

# シンプルな REPL を起動
./bin/smartclaw repl

# 単一プロンプトを送信
./bin/smartclaw prompt "Explain this code"

# 特定のモデルを指定
./bin/smartclaw --model claude-opus-4-6 repl

# WebUI サーバーを起動
./bin/smartclaw web --port 8080

# IDE 統合用 ACP サーバーを起動
./bin/smartclaw acp

# マルチプラットフォームゲートウェイを起動
./bin/smartclaw gateway --adapters telegram,web --telegram-token <BOT_TOKEN>

# バッチ評価を実行
./bin/smartclaw batch --prompts prompts.jsonl --output trajectories/

# RL 評価ループを実行
./bin/smartclaw rl-eval --tasks tasks.jsonl --metric code_quality --output rl-output/

# OpenAI 互換 API を使用
./bin/smartclaw --openai --url https://api.your-provider.com/v1 repl
```

### 設定

Anthropic API キーを設定:

```bash
export ANTHROPIC_API_KEY=your_key_here
```

または `~/.smartclaw/config.yaml` を作成:

```yaml
api_key: your_api_key_here
model: claude-opus-4-6
max_tokens: 4096
permission: ask
log_level: info
openai: false
base_url: ""
show_thinking: true
```

### データディレクトリ

SmartClaw は `~/.smartclaw/` 以下に以下のファイルを自動作成・管理します:

| パス | 説明 |
|------|-------------|
| `MEMORY.md` | システムメモリ、学習ループにより自動更新 |
| `USER.md` | ユーザープロフィール、観察から自動進化 |
| `state.db` | FTS5 インデックス付き SQLite データベース |
| `skills/` | 学習済みおよびバンドルされたスキル |
| `cron/` | スケジュールタスク定義 (JSON) |
| `recordings/` | セッション録画 (JSONL) |
| `mcp/servers.json` | MCP サーバー設定 |
| `exports/` | エクスポートされたセッション |
| `outbox/` | クロスプラットフォームメッセージキュー |

`MEMORY.md` と `USER.md` は直接編集可能です。SmartClaw は次回使用時に再ロードします。

## 利用可能なツール (73+)

### ファイル操作

| ツール | 説明 |
|------|-------------|
| `bash` | タイムアウトとバックグラウンドサポート付きシェルコマンド実行 |
| `read_file` | ファイル内容の読み取り |
| `write_file` | ファイルの書き込み |
| `edit_file` | 文字列置換による編集 |
| `glob` | ファイルパターンマッチング |
| `grep` | 正規表現サポート付きコンテンツ検索 |
| `powershell` | PowerShell コマンドの実行 (Windows) |

### コード分析

| ツール | 説明 |
|------|-------------|
| `lsp` | LSP 操作 (goto_definition, find_references, rename, diagnostics) |
| `ast_grep` | AST パターンの検索と置換 |
| `code_search` | セマンティックコード検索 |
| `index` | 検索用コードインデックス |

### Web とブラウザ

| ツール | 説明 |
|------|-------------|
| `web_fetch` | URL を取得してマークダウンに変換 |
| `web_search` | Web 検索 |
| `browser_navigate` | ヘッドレスブラウザで URL にナビゲート |
| `browser_click` | CSS セレクタで要素をクリック |
| `browser_type` | 要素にテキストを入力 |
| `browser_screenshot` | ページのスクリーンショットを取得 |
| `browser_extract` | ページのコンテンツ/テキストを抽出 |
| `browser_wait` | 要素または条件を待機 |
| `browser_select` | ドロップダウンでオプションを選択 |
| `browser_fill_form` | 複数のフォームフィールドを入力 |

### MCP 統合

| ツール | 説明 |
|------|-------------|
| `mcp` | 接続された MCP サーバーでツールを実行 (SSE/stdio トランスポート) |
| `list_mcp_resources` | MCP サーバーで利用可能なリソースを一覧表示 |
| `read_mcp_resource` | 接続された MCP サーバーからリソースを読み取り |
| `mcp_auth` | OAuth フローで MCP サーバーに認証 |

### エージェントと学習

| ツール | 説明 |
|------|-------------|
| `agent` | 並列タスク用のサブエージェントを起動 |
| `skill` | スキルのロードと管理 |
| `session` | セッション管理 |
| `todowrite` | 確認ナッジ付き Todo リスト管理 |
| `config` | 設定管理 |
| `memory` | 4 層メモリの照会と管理 (recall, search, store, layers, stats) |

### コード実行とサンドボックス

| ツール | 説明 |
|------|-------------|
| `execute_code` | ツールアクセス付き RPC サンドボックスで Python コードを実行。マルチターンをシングルターンに圧縮 |
| `docker_exec` | 分離された Docker コンテナでコマンドを実行（ワンショットまたはセッション永続） |
| `repl` | サンドボックスタイムアウト付きで JavaScript (Node.js) または Python の式を評価 |

### Git 操作

| ツール | 説明 |
|------|-------------|
| `git_ai` | AI 駆動のコミットメッセージ、コードレビュー、PR 説明 |
| `git_status` | ワーキングディレクトリの Git ステータス |
| `git_diff` | Git diff（ステージ済みまたは未ステージ） |
| `git_log` | 最近の Git コミットログ |

### バッチと並列

| ツール | 説明 |
|------|-------------|
| `batch` | 複数のツール呼び出しをバッチ実行 |
| `parallel` | 複数のツール呼び出しを並列実行 |
| `pipeline` | 出力パイプによるツール呼び出しのチェーン |

### チームコラボレーション

| ツール | 説明 |
|------|-------------|
| `team_create` | メモリ共有用のチームワークスペースを作成 |
| `team_delete` | チームワークスペースを削除 |
| `team_share_memory` | メモリアイテムをチームと共有 |
| `team_get_memories` | 共有されたチームメモリを取得 |
| `team_search_memories` | チームメモリ全体を検索 |
| `team_sync` | メンバー間でチーム状態を同期 |
| `team_share_session` | セッションをチームと共有 |

### リモートとメッセージング

| ツール | 説明 |
|------|-------------|
| `remote_trigger` | SSH 経由でリモートホスト上のコマンドを実行 |
| `send_message` | プラットフォーム横断でチャネル/ユーザーにメッセージを送信 (telegram, web, terminal) |

### ワークフローとプランニング

| ツール | 説明 |
|------|-------------|
| `enter_worktree` | 並列開発用の Git ワークツリーを作成 |
| `exit_worktree` | Git ワークツリーを削除してクリーンアップ |
| `enter_plan_mode` | 構造化プランニングモードに入る |
| `exit_plan_mode` | プランニングモードを終了して実行を再開 |
| `schedule_cron` | Cron ジョブのスケジュール、一覧表示、削除 |

### メディアとドキュメント

| ツール | 説明 |
|------|-------------|
| `image` | 画像の分析と処理 |
| `pdf` | PDF ドキュメントからテキストを抽出 |
| `audio` | 音声ファイルの処理と文字起こし |

### 認知ツール

| ツール | 説明 |
|------|-------------|
| `think` | 行動前の構造化思考ステップ |
| `deep_think` | 複雑な問題のための拡張推論 |
| `brief` | 簡潔なトピック要約 |
| `observe` | 受動的分析のための観察モード |
| `lazy` | オンデマンドの遅延ツールロード |
| `fork` | 並列探索のために現在のセッションをフォーク |

### ユーティリティ

| ツール | 説明 |
|------|-------------|
| `tool_search` | キーワードで利用可能なツールを検索 |
| `cache` | ツール結果キャッシュの管理 |
| `attach` | 実行中のプロセスにアタッチ |
| `debug` | デバッグモードの切り替え |
| `env` | 環境変数の表示 |
| `sleep` | 指定時間スリープ |

## スラッシュコマンド (101)

### コア

| コマンド | 説明 |
|---------|-------------|
| `/help` | 利用可能なコマンドを表示 |
| `/status` | セッションステータス |
| `/exit` | REPL を終了 |
| `/clear` | セッションをクリア |
| `/version` | バージョンを表示 |

### モデルと設定

| コマンド | 説明 |
|---------|-------------|
| `/model [name]` | モデルの表示または設定 |
| `/model-list` | 利用可能なモデルを一覧表示 |
| `/config` | 設定を表示 |
| `/config-show` | 完全な設定を表示 |
| `/config-set` | 設定値をセット |
| `/config-get` | 設定値を取得 |
| `/config-reset` | 設定をリセット |
| `/config-export` | 設定をエクスポート |
| `/config-import` | 設定をインポート |
| `/set-api-key <key>` | API キーを設定 |
| `/env` | 環境を表示 |

### セッション

| コマンド | 説明 |
|---------|-------------|
| `/session` | セッションを一覧表示 |
| `/resume` | セッションを再開 |
| `/save` | 現在のセッションを保存 |
| `/export` | セッションをエクスポート (markdown/json) |
| `/import` | セッションをインポート |
| `/rename` | セッション名を変更 |
| `/fork` | 並列探索のためにセッションをフォーク |
| `/rewind` | セッション状態を巻き戻し |
| `/share` | セッションを共有 |
| `/summary` | セッション要約 |
| `/attach` | プロセスにアタッチ |

### 圧縮

| コマンド | 説明 |
|---------|-------------|
| `/compact` | コンテキスト使用量を表示 |
| `/compact now` | 会話履歴を手動圧縮 |
| `/compact auto` | 自動圧縮のオン/オフ切り替え |
| `/compact status` | 圧縮統計を表示 |
| `/compact config` | 圧縮設定を表示 |

### エージェントシステム

| コマンド | 説明 |
|---------|-------------|
| `/agent` | AI エージェントを管理 |
| `/agent-list` | 利用可能なエージェントを一覧表示 |
| `/agent-switch` | エージェントを切り替え |
| `/agent-create` | カスタムエージェントを作成 |
| `/agent-delete` | カスタムエージェントを削除 |
| `/agent-info` | エージェント情報を表示 |
| `/agent-export` | エージェント設定をエクスポート |
| `/agent-import` | エージェント設定をインポート |
| `/subagent` | サブエージェントを起動 |
| `/agents` | 利用可能なエージェントを一覧表示 |

### テンプレートシステム

| コマンド | 説明 |
|---------|-------------|
| `/template` | プロンプトテンプレートを管理 |
| `/template-list` | テンプレートを一覧表示 |
| `/template-use` | テンプレートを使用 |
| `/template-create` | テンプレートを作成 |
| `/template-delete` | テンプレートを削除 |
| `/template-info` | テンプレート情報を表示 |
| `/template-export` | テンプレートをエクスポート |
| `/template-import` | テンプレートをインポート |

### メモリと学習

| コマンド | 説明 |
|---------|-------------|
| `/memory` | メモリコンテキストを表示 |
| `/skills` | 利用可能なスキルを一覧表示 |
| `/observe` | 観察モード |

### MCP

| コマンド | 説明 |
|---------|-------------|
| `/mcp` | MCP サーバーを管理 |
| `/mcp-add` | MCP サーバーを追加 |
| `/mcp-remove` | MCP サーバーを削除 |
| `/mcp-list` | MCP サーバーを一覧表示 |
| `/mcp-start` | MCP サーバーを起動 |
| `/mcp-stop` | MCP サーバーを停止 |

### Git

| コマンド | 説明 |
|---------|-------------|
| `/git-status` (`/gs`) | Git ステータスを表示 |
| `/git-diff` (`/gd`) | Git diff を表示 |
| `/git-commit` (`/gc`) | 変更をコミット |
| `/git-branch` (`/gb`) | ブランチを一覧表示 |
| `/git-log` (`/gl`) | Git ログを表示 |
| `/diff` | diff を表示 |
| `/commit` | Git コミットショートカット |

### ツールと開発

| コマンド | 説明 |
|---------|-------------|
| `/tools` | 利用可能なツールを一覧表示 |
| `/tasks` | タスクの一覧表示と管理 |
| `/lsp` | LSP 操作 |
| `/read` | ファイルを読み取り |
| `/write` | ファイルに書き込み |
| `/exec` | コマンドを実行 |
| `/browse` | ブラウザを開く |
| `/web` | Web 操作 |
| `/ide` | IDE 統合 |
| `/install` | パッケージをインストール |

### 診断

| コマンド | 説明 |
|---------|-------------|
| `/doctor` | 診断を実行 |
| `/cost` | トークン使用量とコストを表示 |
| `/stats` | セッション統計を表示 |
| `/usage` | 使用統計 |
| `/debug` | デバッグモードの切り替え |
| `/inspect` | 内部状態を検査 |
| `/cache` | キャッシュを管理 |
| `/heapdump` | ヒープダンプ |
| `/reset-limits` | レート制限をリセット |

### プランニングと思考

| コマンド | 説明 |
|---------|-------------|
| `/plan` | プランモード |
| `/think` | 思考モード |
| `/deepthink` | ディープシンキング |
| `/ultraplan` | ウルトラプランニング |
| `/thinkback` | 振り返り思考 |

### コラボレーションとコミュニケーション

| コマンド | 説明 |
|---------|-------------|
| `/invite` | コラボレーションに招待 |
| `/feedback` | フィードバックを送信 |
| `/issue` | Issue トラッカー |

### UI とパーソナライゼーション

| コマンド | 説明 |
|---------|-------------|
| `/theme` | テーマを管理 |
| `/color` | カラーテーマ |
| `/vim` | Vim モード |
| `/keybindings` | キーバインドを管理 |
| `/statusline` | ステータスライン |
| `/stickers` | ステッカー |

### モード切り替え

| コマンド | 説明 |
|---------|-------------|
| `/fast` | 高速モード（軽量モデルを使用） |
| `/lazy` | 遅延ロードモード |
| `/desktop` | デスクトップモード |
| `/mobile` | モバイルモード |
| `/chrome` | Chrome 統合 |
| `/voice` | 音声モード制御 |

### 認証とアップデート

| コマンド | 説明 |
|---------|-------------|
| `/login` | サービスに認証 |
| `/logout` | 認証をクリア |
| `/upgrade` | CLI バージョンをアップグレード |
| `/api` | API 操作 |

### その他

| コマンド | 説明 |
|---------|-------------|
| `/init` | 新しいプロジェクトを初期化 |
| `/context` | コンテキストを管理 |
| `/permissions` | パーミッションを管理 |
| `/hooks` | フックを管理 |
| `/plugin` | プラグインを管理 |
| `/passes` | LSP パス |
| `/preview` | 変更をプレビュー |
| `/effort` | エフォートトラッキング |
| `/tag` | タグ管理 |
| `/copy` | クリップボードにコピー |
| `/files` | ファイルを一覧表示 |
| `/advisor` | AI アドバイザー |
| `/btw` | By the way |
| `/bughunter` | バグハントモード |
| `/insights` | コードインサイト |
| `/onboarding` | オンボーディング |
| `/teleport` | テレポートモード |
| `/summary` | セッション要約 |

## プロジェクト構造

```
cmd/
└── smartclaw/              # Application entrypoint

internal/
├── acp/                    # Agent Communication Protocol (IDE integration via JSON-RPC)
├── analytics/              # Usage analytics and reporting
├── api/                    # API client with prompt caching + OpenAI support
├── assistant/              # Assistant personality and behavior
├── auth/                   # OAuth authentication
├── batch/                  # Batch runner for parallel prompt execution
├── bootstrap/              # Bootstrap and first-run
├── bridge/                 # Bridge adapters
├── buddy/                  # Buddy system for guided assistance
├── cache/                  # Caching system with dependency tracking
├── cli/                    # CLI commands (repl, tui, web, acp, batch, rl-eval, gateway)
├── commands/               # 101 Slash commands
├── compact/                # Compaction service (auto, micro, time-based)
├── components/             # Reusable TUI components
├── config/                 # Configuration management
├── constants/              # Shared constants
├── coordinator/            # Task coordination
├── costguard/              # Budget-aware spending guard with model downgrade
├── entrypoints/            # Application entrypoint variants
├── gateway/                # Unified gateway (router, delivery, cron)
│   └── platform/           # Platform adapters (terminal, web, telegram)
├── git/                    # Git context and operations
├── history/                # Command history
├── hooks/                  # Hook system
├── keybindings/            # Keybinding configuration
├── learning/               # Learning loop (evaluator, extractor, skill writer, nudge)
├── logger/                 # Structured logging
├── mcp/                    # MCP protocol (client, transport, auth, registry, enhanced)
├── memdir/                 # Memory directory management
├── memory/                 # Memory manager (4-layer coordination)
│   └── layers/             # L1 Prompt, L2 Session Search, L3 Skill, L4 User Model
├── migrations/             # Database migrations
├── models/                 # Data models
├── native/                 # Native platform bindings
├── native_ts/              # TypeScript native bindings
├── observability/          # Metrics, tracing, and telemetry
├── outputstyles/           # Output formatting styles
├── permissions/            # Permission engine (4 modes)
├── plugins/                # Plugin system
├── process/                # Process management
├── provider/               # Multi-provider API abstraction
├── query/                  # Query engine
├── remote/                 # Remote execution
├── rl/                     # Reinforcement learning evaluation environment
├── routing/                # Adaptive model routing + speculative execution
├── runtime/                # Query engine, compaction, session
├── sandbox/                # Sandboxed execution (Linux namespaces, RPC)
├── schemas/                # JSON schemas for tool inputs
├── screens/                # Screen layout management
├── server/                 # Direct connect server
├── services/               # Shared services (recorder, playback, sync, LSP, OAuth, voice, compact, analytics, rate limit)
├── session/                # Session management
├── skills/                 # Skills system (bundled + learned)
├── state/                  # Application state
├── store/                  # SQLite persistence (WAL, FTS5, JSONL backup)
├── template/               # Prompt template engine
├── tools/                  # Tool implementations (73+ tools)
├── transports/             # Transport layer abstractions
├── tui/                    # Terminal UI (Bubble Tea)
├── types/                  # Shared type definitions
├── upstreamproxy/          # Upstream API proxy
├── utils/                  # Utility functions
├── vim/                    # Vim mode support
├── voice/                  # Voice input/output
├── watcher/                # File system watcher
└── web/                    # Web UI + WebSocket server

pkg/
├── output/                 # Shared output formatting
└── progress/               # Progress bar utilities

extensions/
└── vscode/                 # VS Code extension (chat sidebar, code actions)
```

## VS Code 拡張機能

SmartClaw は ACP (Agent Communication Protocol) 経由で接続する VS Code 拡張機能を同梱しています:

### コマンド

| コマンド | 説明 |
|---------|-------------|
| `SmartClaw: Ask` | SmartClaw に質問 |
| `SmartClaw: Open Chat` | チャットサイドバーを開く |
| `SmartClaw: Explain Code` | 選択したコードを説明 |
| `SmartClaw: Fix Code` | 選択したコードの問題を修正 |
| `SmartClaw: Generate Tests` | 選択したコードのテストを生成 |

### インストール

1. SmartClaw をビルド: `go build -o bin/smartclaw ./cmd/smartclaw/`
2. `smartclaw` を PATH に追加
3. `extensions/vscode/` から拡張機能をインストール
4. エクスプローラーで SmartClaw サイドバーを開く

## API 使用例

```go
package main

import (
    "context"
    "fmt"

    "github.com/instructkr/smartclaw/internal/api"
    "github.com/instructkr/smartclaw/internal/gateway"
    "github.com/instructkr/smartclaw/internal/learning"
    "github.com/instructkr/smartclaw/internal/memory"
    "github.com/instructkr/smartclaw/internal/runtime"
)

func main() {
    client := api.NewClient("your-api-key")
    memManager, _ := memory.NewMemoryManager()
    learningLoop := learning.NewLearningLoop(nil, memManager.GetPromptMemory(), "")

    engineFactory := func() *runtime.QueryEngine {
        return runtime.NewQueryEngine(client, runtime.QueryConfig{})
    }

    gw := gateway.NewGateway(engineFactory, memManager, learningLoop)
    defer gw.Close()

    resp, err := gw.HandleMessage(context.Background(), "user-1", "terminal", "Hello!")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Content)
}
```

## 環境変数

| 変数 | 説明 |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic 用 API キー |
| `SMARTCLAW_MODEL` | デフォルトで使用するモデル |
| `SMARTCLAW_CONFIG` | 設定ファイルのパス |
| `SMARTCLAW_SESSION_DIR` | セッションストレージのディレクトリ |
| `SMARTCLAW_LOG_LEVEL` | ログレベル (debug, info, warn, error) |

## テスト

```bash
# すべてのテストを実行
go test ./...

# 特定のパッケージを実行
go test ./internal/learning/...
go test ./internal/store/...
go test ./internal/memory/layers/...
go test ./internal/tools/...
go test ./internal/services/...
go test ./internal/sandbox/...
go test ./internal/compact/...
go test ./internal/routing/...
go test ./internal/costguard/...
go test ./internal/acp/...
go test ./internal/observability/...

# カバレッジ付きで実行
go test -cover ./...
```

## ライセンス

MIT License
