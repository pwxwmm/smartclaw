# SmartClaw Hermes 化改造计划

## 项目概述

**目标**: 将 SmartClaw 从"通用 AI 开发工具"改造为"越来越懂你的工作方式"的智能 Agent

**参考**: Hermes 架构（单 Agent 持久循环 + 四层记忆 + 学习循环）

**核心转变**: 不是"在通用场景下更擅长工作"，而是"越来越懂你的工作方式"。目的不同，底层设计也就不同。

**现有代码基础**: 39,218 行 Go 代码，57 个工具，101 个命令，15 个核心子系统

---

## 一、架构差距总览

| 维度 | SmartClaw 现状 | Hermes 目标 | 差距等级 |
|------|--------------|-------------|---------|
| 设计哲学 | 通用工具，什么都行 | 越来越懂你的工作方式 | 根本性 |
| 核心循环 | 输入→API→输出→结束 | 输入→推理→工具→记忆→输出→**学习** | 🔴 完全缺失 |
| 记忆系统 | 碎片化JSON，线性扫描 | 四层分层，FTS5索引，按需加载 | 🔴 严重缺失 |
| 学习机制 | 无，任务结束即遗忘 | 评估→提炼→写skill→下次复用 | 🔴 完全缺失 |
| 自省机制 | 无 | 定期nudge触发Agent回顾整理 | 🔴 完全缺失 |
| 网关设计 | 传输层+模式切换 | 五职责一体化（消息/路由/交付/配对/定时） | 🟡 严重不足 |
| 上下文压缩 | 简单截断保留最后4条 | LLM摘要+溯源链 | 🟡 过于简陋 |
| 持久化 | JSON文件 | SQLite+WAL+FTS5 | 🟡 无索引 |

---

## 二、六大改造模块

### 模块 1: 学习循环 (Learning Loop)

**现状**: `QueryEngine.Query()` 流程是 输入→API→输出→结束，任务完成后什么都不做

**目标**: 任务完成→Agent自评估→提炼有效方法→写成skill→存入磁盘→下次直接复用

**新增文件**:

```
internal/learning/
├── loop.go           # 学习循环核心：协调 eval→extract→write 流程
├── evaluator.go      # 任务评估器：调用LLM判断"这次方法值不值得保留"
├── extractor.go      # 方法提炼器：从任务历史中提炼可复用步骤
├── skill_writer.go   # Skill写入器：将提炼结果写成SKILL.md
└── nudge.go          # Nudge引擎：定期触发Agent自省
```

**核心接口**:

```go
// loop.go
type LearningLoop struct {
    evaluator   *Evaluator
    extractor   *Extractor
    skillWriter *SkillWriter
    nudgeEngine *NudgeEngine
    memory      *layers.PromptMemory
}

// OnTaskComplete 在每次任务完成后触发
func (l *LearningLoop) OnTaskComplete(ctx context.Context, session *session.Session, result *runtime.QueryResult) error

// OnNudge 由nudge引擎定期触发
func (l *LearningLoop) OnNudge(ctx context.Context, session *session.Session) error
```

```go
// evaluator.go
type TaskEvaluation struct {
    WorthKeeping  bool     // 这套方法值得保留吗？
    Reason        string   // 为什么值得/不值得
    KeySteps      []string // 关键步骤摘要
    SkillName     string   // 建议的skill名称
    SkillCategory string   // skill分类
}

type Evaluator struct {
    client *api.Client // 用轻量模型做评估
}

func (e *Evaluator) Evaluate(ctx context.Context, messages []Message, result *QueryResult) (*TaskEvaluation, error)
```

```go
// extractor.go
type ExtractedSkill struct {
    Name        string   // skill名称
    Description string   // 一句话描述
    Triggers    []string // 触发条件
    Steps       []string // 执行步骤
    Tools       []string // 需要的工具
    Tags        []string // 标签
}

type Extractor struct {
    client *api.Client
}

func (ex *Extractor) Extract(ctx context.Context, messages []Message, evaluation *TaskEvaluation) (*ExtractedSkill, error)
```

```go
// skill_writer.go
type SkillWriter struct {
    skillsDir string // ~/.smartclaw/skills/
}

func (sw *SkillWriter) WriteSkill(skill *ExtractedSkill) error
// 写入 ~/.smartclaw/skills/{name}/SKILL.md
// 自动被 SkillManager.StartWatcher() 发现有新skill

func (sw *SkillWriter) UpdateMemoryMD(key, value string) error
// 更新 MEMORY.md 中的特定条目
```

```go
// nudge.go
type NudgeEngine struct {
    interval    int // 每N轮对话触发一次，默认5
    turnCounter int
}

// MaybeNudge 检查是否需要触发nudge
func (ne *NudgeEngine) MaybeNudge(currentTurn int) *NudgePrompt

type NudgePrompt struct {
    Content string // 系统级提示，不需要用户输入
}
```

**与现有系统的集成点**:

1. `runtime/engine.go` 的 `Query()` 方法末尾调用 `learningLoop.OnTaskComplete()`
2. `runtime/engine.go` 的 `CompactIfNeeded()` 前调用 `nudgeEngine.MaybeNudge()`
3. `skills/skills.go` 的 `StartWatcher()` 自动发现学习循环写入的新skill
4. 评估使用轻量模型（如 claude-3-haiku）控制成本

---

### 模块 2: 四层记忆系统 (Four-Layer Memory)

**现状**:
- L1: `EnhancedMemoryStore` **主动跳过** `MEMORY.md`，无自动加载，无字符限制
- L2: JSON文件+线性扫描，无FTS5，无语义检索
- L3: `SkillManager` 加载所有skill完整content，token浪费严重
- L4: 无用户建模

**目标**: 四层独立、按需加载、严格约束

**新增/重构文件**:

```
internal/memory/
├── memory.go              # 保留，作为基础层
├── enhanced_memory.go     # 保留，逐步迁移
├── layers/                # 新增：四层记忆
│   ├── prompt_memory.go   # L1: 常驻上下文
│   ├── session_search.go  # L2: 会话检索
│   ├── skill_memory.go    # L3: 技能程序性记忆
│   └── user_model.go      # L4: 用户建模
└── manager.go             # 新增：记忆管理器，协调四层
```

#### L1: Prompt Memory (常驻上下文)

```go
// layers/prompt_memory.go

const MaxPromptMemoryChars = 3575 // Hermes硬限制

type PromptMemory struct {
    memoryMD  *ManagedFile // ~/.smartclaw/MEMORY.md
    userMD    *ManagedFile // ~/.smartclaw/USER.md
    mu        sync.RWMutex
}

type ManagedFile struct {
    path    string
    content string
    modTime time.Time
}

// AutoLoad 每次session开始自动加载，拼入system prompt
func (pm *PromptMemory) AutoLoad() string {
    pm.mu.RLock()
    defer pm.mu.RUnlock()

    combined := pm.memoryMD.content + "\n" + pm.userMD.content
    if len(combined) > MaxPromptMemoryChars {
        combined = combined[:MaxPromptMemoryChars]
    }
    return combined
}

// UpdateMemory 写入MEMORY.md（由nudge或学习循环调用）
func (pm *PromptMemory) UpdateMemory(content string) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    return pm.memoryMD.Write(content)
}

// UpdateUserProfile 写入USER.md（由用户建模层调用）
func (pm *PromptMemory) UpdateUserProfile(profile string) error {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    return pm.userMD.Write(profile)
}

// EnforceLimit 检查并强制字符限制
func (pm *PromptMemory) EnforceLimit() error {
    // 如果超出3575字符，调用LLM压缩
}
```

**MEMORY.md 格式**:

```markdown
# System Memory

## User Preferences
- prefers concise responses
- works primarily in Go and TypeScript
- uses vim keybindings

## Learned Patterns
- always run tests after refactoring
- commit messages follow conventional commits
- prefers table-driven tests in Go

## Active Context
- current project: smartclaw
- architecture: single-agent with learning loop
```

**USER.md 格式**:

```markdown
# User Profile

## Communication Style
- terse, direct
- prefers code over explanation
- Chinese primary language

## Knowledge Background
- senior Go developer
- familiar with AI/ML concepts
- infrastructure background

## Common Workflows
- morning: review PRs, check CI
- afternoon: implement features
- evening: refactor and test
```

#### L2: Session Search (会话检索)

```go
// layers/session_search.go

type SessionSearch struct {
    db *sql.DB // ~/.smartclaw/state.db with FTS5
}

type SessionFragment struct {
    SessionID  string
    Timestamp  time.Time
    Role       string
    Content    string
    Relevance  float64
    SourceTurn int // 溯源到原始对话轮次
}

// Search 搜索与当前任务相关的历史上下文
func (ss *SessionSearch) Search(ctx context.Context, query string, limit int) ([]SessionFragment, error) {
    // 1. FTS5全文索引搜索
    rows, err := ss.db.Query(
        "SELECT session_id, role, content, timestamp, rank FROM messages_fts WHERE messages_fts MATCH ? ORDER BY rank LIMIT ?",
        query, limit,
    )
    // 2. 返回按相关度排序的片段
}

// SearchAndSummarize 搜索后用LLM摘要，只保留相关部分
func (ss *SessionSearch) SearchAndSummarize(ctx context.Context, query string, maxTokens int) (string, error) {
    // 1. FTS5搜索
    fragments := ss.Search(ctx, query, 10)
    // 2. LLM摘要，只保留与当前任务相关的部分
    summary := llm.Summarize(ctx, fragments, query, maxTokens)
    return summary, nil
}

// IndexSession 将session消息写入FTS5索引
func (ss *SessionSearch) IndexSession(session *session.Session) error
```

#### L3: Skill Procedural Memory (技能程序性记忆)

```go
// layers/skill_memory.go

type SkillSummary struct {
    Name        string
    Description string
    Tags        []string
    Triggers    []string
    Source      string // "bundled", "learned", "local"
}

type SkillProceduralMemory struct {
    index     map[string]*SkillSummary  // 只加载name+description
    fullCache map[string]*skills.Skill  // 需要时才读完整内容
    skillsDir string
    mu        sync.RWMutex
}

// LoadIndex 启动时只加载摘要，不加载完整内容
func (spm *SkillProceduralMemory) LoadIndex() error {
    // 遍历 ~/.smartclaw/skills/ 和 bundled skills
    // 只读 name, description, tags, triggers
    // 不读完整 SKILL.md 内容
}

// GetFullSkill 当前任务确实需要时才加载完整内容
func (spm *SkillProceduralMemory) GetFullSkill(name string) (*skills.Skill, error) {
    // 检查缓存
    if cached, ok := spm.fullCache[name]; ok {
        return cached, nil
    }
    // 从磁盘读取完整SKILL.md
    full, err := spm.loadFullSkill(name)
    spm.fullCache[name] = full
    return full, nil
}

// BuildSkillPrompt 构建skill提示：只有名字和摘要
func (spm *SkillProceduralMemory) BuildSkillPrompt() string {
    // 只列出 skill name + one-line description
    // 不包含完整skill内容，节省token
}
```

#### L4: User Modeling (用户建模)

```go
// layers/user_model.go

type UserModel struct {
    Preferences    map[string]string  // 用户偏好
    CommunicationStyle string         // 沟通风格
    KnowledgeBg    []string           // 知识背景
    CommonPatterns []WorkPattern      // 常见工作模式
    LastUpdated    time.Time
}

type WorkPattern struct {
    Pattern    string    // 模式描述
    Frequency  int       // 出现频率
    LastSeen   time.Time // 最后一次出现
}

type User ModelingLayer struct {
    model    *UserModel
    db       *sql.DB
    promptMem *PromptMemory
}

// TrackPassive 被动追踪用户偏好变化（不需要用户主动输入）
func (uml *UserModelingLayer) TrackPassive(session *session.Session) error {
    // 分析session中的用户行为：
    // - 常用的代码风格
    // - 偏好的工具和命令
    // - 沟通语言和风格
    // - 常见任务类型
}

// UpdateProfile 更新USER.md
func (uml *UserModelingLayer) UpdateProfile(ctx context.Context) error {
    // 根据积累的观察更新USER.md
    // 保持3,575字符限制
}
```

#### 记忆管理器

```go
// manager.go

type MemoryManager struct {
    promptMemory  *layers.PromptMemory
    sessionSearch *layers.SessionSearch
    skillMemory   *layers.SkillProceduralMemory
    userModel     *layers.UserModelingLayer
}

// BuildSystemContext 构建完整系统上下文
func (mm *MemoryManager) BuildSystemContext(ctx context.Context, currentQuery string) string {
    var parts []string

    // L1: 常驻上下文（每次都加载）
    parts = append(parts, mm.promptMemory.AutoLoad())

    // L2: 会话检索（按需搜索相关上下文）
    if currentQuery != "" {
        relevant := mm.sessionSearch.SearchAndSummarize(ctx, currentQuery, 1000)
        if relevant != "" {
            parts = append(parts, relevant)
        }
    }

    // L3: Skill摘要（只列名字和描述）
    parts = append(parts, mm.skillMemory.BuildSkillPrompt())

    // L4: 用户建模（通过L1的USER.md体现）

    return strings.Join(parts, "\n\n")
}
```

---

### 模块 3: SQLite 持久化层

**现状**: 全部JSON文件存储，无索引，线性扫描

**目标**: SQLite + WAL + FTS5，支持高效检索和并发访问

**新增文件**:

```
internal/store/
├── sqlite.go        # SQLite连接管理，WAL模式
├── schema.go        # 建表、索引、FTS5虚拟表
├── session_repo.go  # Session数据访问
├── message_repo.go  # Message数据访问+FTS5搜索
└── jsonl.go         # JSONL备份（与Hermes一致，额外写一份原始记录）
```

**Schema设计**:

```sql
-- schema.go

const SchemaSQL = `
-- 启用WAL模式（并发安全）
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;

-- Sessions表
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    platform   TEXT DEFAULT 'terminal',
    model      TEXT,
    title      TEXT,
    summary    TEXT,
    tokens     INTEGER DEFAULT 0,
    cost       REAL DEFAULT 0.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Messages表
CREATE TABLE IF NOT EXISTS messages (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role       TEXT NOT NULL,  -- user, assistant, system, tool
    content    TEXT NOT NULL,
    tokens     INTEGER DEFAULT 0,
    tool_name  TEXT,           -- 如果是tool调用
    tool_input TEXT,           -- tool输入
    tool_result TEXT,          -- tool输出
    timestamp  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- FTS5全文索引（只索引content字段）
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='rowid',
    tokenize='unicode61'
);

-- FTS5同步触发器
CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES('delete', old.rowid, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;

-- Skills表（learned skills）
CREATE TABLE IF NOT EXISTS skills (
    name       TEXT PRIMARY KEY,
    description TEXT,
    content    TEXT,           -- 完整SKILL.md内容
    source     TEXT DEFAULT 'learned', -- bundled, learned, local
    use_count  INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User模型表
CREATE TABLE IF NOT EXISTS user_observations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    category   TEXT NOT NULL,  -- preference, style, knowledge, pattern
    key        TEXT NOT NULL,
    value      TEXT,
    confidence REAL DEFAULT 0.5,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_id TEXT
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_role ON messages(session_id, role);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source);
CREATE INDEX IF NOT EXISTS idx_observations_category ON user_observations(category, key);
`
```

**核心接口**:

```go
// sqlite.go
type Store struct {
    db       *sql.DB
    jsonlDir string // JSONL备份目录
}

func NewStore() (*Store, error) {
    // 打开 ~/.smartclaw/state.db
    // 执行SchemaSQL
    // 启动WAL模式
}

func (s *Store) Close() error {
    // checkpoint WAL
    // 关闭连接
}

// message_repo.go
func (s *Store) InsertMessage(msg *Message) error
func (s *Store) SearchMessages(query string, limit int) ([]*Message, error)
func (s *Store) GetSessionMessages(sessionID string) ([]*Message, error)

// session_repo.go
func (s *Store) UpsertSession(session *Session) error
func (s *Store) GetSession(id string) (*Session, error)
func (s *Store) ListSessions(userID string, limit int) ([]*Session, error)

// jsonl.go
func (s *Store) AppendJSONL(msg *Message) error
// 原始对话记录额外写入JSONL文件，与Hermes一致
```

---

### 模块 4: 智能上下文压缩

**现状**: `runtime/compact.go` 只有67行，简单截断保留最后4条

**目标**: LLM摘要+溯源链，压缩中间轮次但保留可追溯性

**重构文件**: `internal/runtime/compact.go`

```go
// compact.go (重构)

type Compactor struct {
    client     *api.Client // 用辅助模型做摘要
    store      *store.Store
    maxTokens  int
}

type CompactionResult struct {
    Summary      string    // LLM生成的摘要
    SourceRanges []Range   // 摘要对应的原始消息范围（溯源链）
    KeptMessages []Message // 保留的最近消息
}

type Range struct {
    StartMsgID string
    EndMsgID   string
    TurnStart  int
    TurnEnd    int
}

// Compact 执行智能压缩
func (c *Compactor) Compact(ctx context.Context, messages []Message, keepRecent int) ([]Message, error) {
    if len(messages) <= keepRecent {
        return messages, nil
    }

    // 1. 分离：要压缩的中间消息 vs 保留的最近消息
    toCompress := messages[:len(messages)-keepRecent]
    toKeep := messages[len(messages)-keepRecent:]

    // 2. LLM摘要：调用辅助模型生成总结
    summary, err := c.generateSummary(ctx, toCompress)
    if err != nil {
        // fallback到简单截断
        return c.simpleCompact(messages, keepRecent), nil
    }

    // 3. 构建溯源链：摘要指向原始消息
    summaryMsg := Message{
        Role:    "system",
        Content: summary,
        Metadata: map[string]interface{}{
            "compaction": true,
            "source_range": Range{
                StartMsgID: toCompress[0].ID,
                EndMsgID:   toCompress[len(toCompress)-1].ID,
            },
        },
    }

    // 4. 返回：摘要 + 最近消息
    return append([]Message{summaryMsg}, toKeep...), nil
}

// generateSummary 用LLM生成摘要
func (c *Compactor) generateSummary(ctx context.Context, messages []Message) (string, error) {
    prompt := fmt.Sprintf(
        "Summarize the following conversation, preserving:\n"+
            "1. Key decisions and their reasons\n"+
            "2. Important context about the project\n"+
            "3. Tool calls and their results\n"+
            "4. Any unresolved issues\n\n"+
            "Conversation:\n%s",
        formatMessages(messages),
    )
    // 调用轻量模型
    resp, err := c.client.CreateMessage(ctx, prompt, "claude-3-haiku-20240307")
    return resp.Content, err
}

// TraceBack 通过溯源链回溯原始对话
func (c *Compactor) TraceBack(ctx context.Context, summaryMsg Message) ([]Message, error) {
    range_ := summaryMsg.Metadata["source_range"].(Range)
    return c.store.GetMessagesRange(range_.StartMsgID, range_.EndMsgID)
}
```

---

### 模块 5: 网关一体化

**现状**: `transports/` 只有数据传输，`coordinator/` 只有模式切换

**目标**: 五职责一体化网关（消息传递/会话路由/交付/配对/定时触发）

**重构文件**:

```
internal/gateway/          # 新增（替代部分transports和coordinator功能）
├── gateway.go             # 网关核心，五个职责协调
├── router.go              # 会话路由（userID→session，跨平台）
├── delivery.go            # 消息交付
├── pairing.go             # 跨平台配对
├── cron_trigger.go        # 定时触发（定时任务作为一级Agent任务）
└── platform/              # 平台适配器
    ├── terminal.go        # 终端
    ├── web.go             # Web UI
    └── telegram.go        # Telegram（示例）
```

**核心接口**:

```go
// gateway.go

type Gateway struct {
    router      *SessionRouter
    delivery    *DeliveryManager
    pairing     *PairingManager
    cronTrigger *CronTrigger
    learning    *learning.LearningLoop
    memory      *memory.MemoryManager
}

// HandleMessage 处理来自任意平台的消息
func (g *Gateway) HandleMessage(ctx context.Context, userID, platform, content string) (*Response, error) {
    // 1. 路由：按userID找到session（不按平台）
    session := g.router.Route(userID)

    // 2. 加载记忆：L1自动加载 + L2按需检索
    context := g.memory.BuildSystemContext(ctx, content)

    // 3. 执行Agent循环
    result := g.executeAgentLoop(ctx, session, content, context)

    // 4. 学习循环：任务完成后评估
    g.learning.OnTaskComplete(ctx, session, result)

    // 5. 交付：推送到用户当前所在平台
    g.delivery.Deliver(userID, platform, result)

    return result, nil
}

// router.go

type SessionRouter struct {
    store *store.Store
}

// Route 按用户ID路由session，不按平台
// 用户在Telegram开启对话，切换到终端后继续，不丢失上下文
func (sr *SessionRouter) Route(userID string) *session.Session {
    // 查找用户最近的活跃session
    // 如果没有，创建新session
}
```

```go
// cron_trigger.go

type CronTrigger struct {
    store    *store.Store
    gateway  *Gateway
    cronDir  string // ~/.smartclaw/cron/
    stopChan chan struct{}
}

// ScheduleCron 安排定时任务（作为一级Agent任务）
func (ct *CronTrigger) ScheduleCron(taskID, instruction, schedule string) error {
    // 解析指令，存入 ~/.smartclaw/cron/{taskID}.json
}

// Start 启动定时任务循环
func (ct *CronTrigger) Start() {
    // 到点后，由网关的定时触发启动任务
    // Agent循环带着完整记忆和skill执行
    // 结果通过网关推送至指定平台
}
```

---

### 模块 6: 提示缓存优化

**现状**: 无提示缓存策略

**目标**: 利用系统提示的稳定性实现prompt caching，降低延迟和成本

**改造文件**: `internal/api/client.go`

```go
// 在client.go中增加缓存感知

type CacheAwareClient struct {
    client       *api.Client
    lastPrompt   string
    cacheValid   bool
}

// MarkCacheInvalid 标记缓存失效
// 仅在以下情况触发：
// 1. 切换模型
// 2. 修改MEMORY.md或USER.md
// 3. 修改上下文文件
func (cac *CacheAwareClient) MarkCacheInvalid() {
    cac.cacheValid = false
}

// 在API请求中利用prompt caching
// Anthropic等provider支持缓存系统提示
// 稳定的system prompt + 动态的user message
```

---

## 三、改造路线图

### Phase 1: 记忆基础 (P0, ~5天)

**目标**: 让Agent"有记忆"和"会学习"

| 天 | 任务 | 产出 |
|----|------|------|
| 1-2 | L1 Prompt Memory | `MEMORY.md` + `USER.md` + 自动加载 + 3,575字符限制 |
| 2-3 | 学习循环核心 | `internal/learning/loop.go` + `evaluator.go` + `extractor.go` + `skill_writer.go` |
| 3-4 | 学习循环集成 | 接入`runtime/engine.go`，任务完成后触发评估 |
| 4-5 | Nudge引擎 | `nudge.go`，定期触发自省+记忆整理 |
| 5 | 集成测试 | 端到端：任务完成→评估→提炼→写skill→下次复用 |

**验证标准**:
- [ ] 完成一次任务后，agent能自动判断方法是否值得保留
- [ ] 值得保留的方法被写成SKILL.md到~/.smartclaw/skills/
- [ ] 下次遇到类似任务时，能发现并使用已保存的skill
- [ ] MEMORY.md自动更新，保持3,575字符限制
- [ ] Nudge定期触发，不需要用户干预

### Phase 2: 存储升级 (P1, ~4天)

**目标**: 让记忆"能检索"和"不浪费token"

| 天 | 任务 | 产出 |
|----|------|------|
| 1-2 | SQLite + FTS5 | `internal/store/` 全部文件 |
| 2-3 | L2 Session Search | FTS5检索+LLM摘要+按需注入 |
| 3-4 | L3 Skill Procedural Memory | 只加载摘要，需要时才读完整内容 |
| 4 | 迁移+测试 | 现有JSON数据迁移到SQLite |

**验证标准**:
- [ ] FTS5搜索返回相关历史上下文
- [ ] 搜索结果经LLM摘要后注入，不超token预算
- [ ] Skill索引只占token预算的1/10（只列名字和描述）
- [ ] 需要skill时才加载完整内容
- [ ] SQLite WAL模式下并发安全

### Phase 3: 压缩+建模 (P2, ~5天)

**目标**: 让压缩"有溯源"和让Agent"懂用户"

| 天 | 任务 | 产出 |
|----|------|------|
| 1-2 | SmartCompactor | LLM摘要+溯源链，替代简单截断 |
| 2-3 | L4 User Modeling | 跨session被动追踪用户偏好+自动更新USER.md |
| 3-4 | 提示缓存优化 | 利用系统提示稳定性降低API成本 |
| 4-5 | 集成测试 | 四层记忆+学习循环+智能压缩 联调 |

**验证标准**:
- [ ] 上下文压缩保留溯源链，可回溯原始对话
- [ ] Agent被动学习用户偏好，无需用户主动配置
- [ ] USER.md随使用自动演化
- [ ] 系统提示缓存命中率高，API成本降低

### Phase 4: 网关一体化 (P3, ~8天)

**目标**: 让系统"跨平台"和"一体化"

| 天 | 任务 | 产出 |
|----|------|------|
| 1-3 | Gateway核心 | 五职责网关+会话路由（userID→session） |
| 3-5 | 跨平台配对 | Telegram↔终端↔Web 无缝切换 |
| 5-7 | 定时任务 | CronTrigger作为一级Agent任务 |
| 7-8 | 集成测试 | 跨平台连续性+定时任务执行 |

**验证标准**:
- [ ] 用户在Telegram开启对话，切换到终端后继续
- [ ] 定时任务带着完整记忆和skill执行
- [ ] 定时任务结果通过网关推送

---

## 四、关键设计决策

### D1: 单Agent vs 多Agent

**决策**: 保留多Agent能力，但学习循环只在主循环运行

**理由**:
- 子Agent是工具调用，不是学习主体
- 学习需要完整上下文，子Agent只有局部视角
- 多Agent协同仍有价值（并行探索），但不参与记忆积累

**实现**:
```
主Agent循环: 输入→推理→工具→记忆→输出→学习
                                          ↓
子Agent调用: ──→ spawn agent → result ──→  ↑（结果回传主循环）
```

### D2: 字符限制策略

**决策**: MEMORY.md + USER.md 合计 3,575 字符硬限制

**理由**:
- Hermes验证了这个约束的价值：逼迫系统只留真正重要的信息
- 常驻上下文太大会挤占对话空间
- 限制促使记忆持续精炼而非无限膨胀

### D3: Nudge频率

**决策**: 每5轮对话触发一次nudge

**理由**:
- 太频繁（每轮）会打断用户
- 太稀少（每20轮）会遗忘重要信息
- 5轮是一个平衡点：足够积累上下文，不会太打扰

### D4: 评估模型选择

**决策**: 评估和摘要使用轻量模型（claude-3-haiku），主循环使用主力模型

**理由**:
- 评估和摘要是辅助操作，不需要最强推理
- 控制成本：学习循环每次任务都会触发
- 主力模型专注于核心任务

### D5: SQLite选型

**决策**: 使用 `modernc.org/sqlite`（纯Go实现，无需CGO）

**理由**:
- 无CGO依赖，交叉编译简单
- 性能足够（FTS5搜索<10ms）
- 与Go生态集成好

### D6: 向后兼容

**决策**: 新系统启动时自动迁移现有JSON数据到SQLite

**实现**:
```go
func MigrateJSONToSQLite(jsonDir string, store *Store) error {
    // 1. 读取 ~/.smartclaw/sessions/*.json
    // 2. 写入 SQLite sessions + messages 表
    // 3. 重建 FTS5 索引
    // 4. 保留原JSON文件作为备份
}
```

---

## 五、与现有系统的集成映射

| 现有组件 | 改造动作 | 说明 |
|---------|---------|------|
| `internal/memory/memory.go` | 保留，降级为基础层 | MemoryStore仍可用于临时存储 |
| `internal/memory/enhanced_memory.go` | 逐步迁移到layers/ | EnhancedMemoryStore的BuildMemoryPrompt迁移到PromptMemory |
| `internal/skills/skills.go` | 修改SkillManager，接入L3懒加载 | 默认只加载索引，需要时才读完整内容 |
| `internal/skills/memory_integration.go` | 迁移到learning/ | LearnFromSession由学习循环统一管理 |
| `internal/runtime/engine.go` | 修改Query()，接入学习循环 | 任务完成后调用OnTaskComplete |
| `internal/runtime/compact.go` | 重构为SmartCompactor | LLM摘要+溯源链 |
| `internal/session/session.go` | 保留，接入SQLite持久化 | JSON仍作为导出格式 |
| `internal/services/session_memory.go` | 保留，接入L2 SessionSearch | GetContextForPrompt改为FTS5检索 |
| `internal/services/team_memory_sync.go` | 保留 | 团队协作是独立需求 |
| `internal/coordinator/coordinator.go` | 保留 | 协调模式不影响学习循环 |
| `internal/transports/` | 保留，gateway在其上层 | 传输层不变，网关增加路由逻辑 |
| `internal/tools/remaining_tools.go` | ScheduleCronTool接入CronTrigger | 定时任务走网关一体化路径 |

---

## 六、风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 学习循环增加API调用 | 成本上升 | 使用轻量模型(haiku)做评估/摘要，预估每次任务增加<$0.01 |
| SQLite引入新依赖 | 编译复杂度 | 使用纯Go实现 modernc.org/sqlite |
| MEMORY.md字符限制太紧 | 重要信息丢失 | 定期EnforceLimit用LLM压缩，保留最关键信息 |
| Nudge打断用户体验 | 用户反感 | Nudge是系统内部提示，用户不可见；且可配置频率 |
| 迁移数据丢失 | 历史session丢失 | 迁移保留原JSON文件作为备份，不删除 |

---

## 七、成功指标

| 指标 | 当前 | 目标 |
|------|------|------|
| 跨session记忆 | 无 | ✅ 相同用户新session能回忆之前的上下文 |
| 自动skill创建 | 无 | ✅ 10次任务后自动生成1-2个learned skill |
| token效率 | 所有skill全量加载 | ✅ skill索引只占1/10 token |
| 上下文压缩质量 | 简单截断 | ✅ LLM摘要+溯源链，可回溯 |
| 用户建模 | 无 | ✅ USER.md自动演化，无需手动配置 |
| FTS5检索延迟 | N/A | ✅ <10ms |
| 学习循环成本 | N/A | ✅ 每次任务增加<$0.01 |

---

## 八、Hermes 实际实现细节（来自源码分析）

> 以下来自 Hermes Agent (NousResearch/hermes-agent) 源码的直接分析，用于校准 SmartClaw 改造方案的精确度。

### 8.1 SQLite Schema — 来自 hermes_state.py

Hermes 的 SQLite 实现比我们方案的更细致，需要补充以下设计：

**Sessions 表**：
- `source` 字段区分 CLI session vs Gateway session
- `system_prompt` 字段缓存系统提示（配合 prompt caching）
- `parent_session_id` 字段支持压缩驱动的会话分裂——当上下文被压缩时，新 session 继承旧 session 的 lineage

```sql
-- Hermes 实际的 sessions 表
CREATE TABLE IF NOT EXISTS sessions (
    id                TEXT PRIMARY KEY,
    source            TEXT DEFAULT 'cli',    -- 'cli' 或 'gateway'
    model             TEXT,
    system_prompt     TEXT,                  -- 缓存的系统提示
    parent_session_id TEXT,                  -- 压缩分裂时指向原session
    started_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Messages 表**：
- `tool_calls` 字段存储完整的工具调用结构
- `finish_reason` 字段记录结束原因
- FTS5 使用 `content_rowid='id'` 而非 `rowid`

```sql
-- Hermes 实际的 messages 表
CREATE TABLE IF NOT EXISTS messages (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(id),
    role          TEXT NOT NULL,
    content       TEXT,
    tool_calls    TEXT,   -- JSON序列化的工具调用
    timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
    finish_reason TEXT
);

-- FTS5
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='id'
);
```

**并发写入安全**：Hermes 实现了 `_execute_write` 方法，使用 `BEGIN IMMEDIATE` + 重试+抖动（jitter），解决多进程（gateway + CLI）并发写入问题。

```python
# Hermes 的并发写入模式（Python伪代码，Go版需翻译）
def _execute_write(self, query, params, max_retries=3):
    for attempt in range(max_retries):
        try:
            conn = self._get_connection()
            conn.execute("BEGIN IMMEDIATE")
            conn.execute(query, params)
            conn.commit()
            return
        except sqlite3.OperationalError as e:
            if "locked" in str(e) and attempt < max_retries - 1:
                time.sleep(random.uniform(0.05, 0.2) * (2 ** attempt))
                continue
            raise
```

**SmartClaw 调整**：我们的 `internal/store/sqlite.go` 需要补充：
- [ ] `source` 字段区分 session 来源
- [ ] `parent_session_id` 支持压缩分裂
- [ ] `system_prompt` 缓存字段
- [ ] `tool_calls` JSON 字段
- [ ] `finish_reason` 字段
- [ ] `BEGIN IMMEDIATE` + 重试+抖动 的并发写入模式

### 8.2 Nudge 参数 — 来自 run_agent.py

Hermes 实际的 nudge 参数：

```python
self._memory_nudge_interval = int(mem_config.get("nudge_interval", 10))  # 默认每10轮
self._memory_flush_min_turns = int(mem_config.get("flush_min_turns", 6)) # 至少6轮后才flush
```

**SmartClaw 调整**：我们的 nudge 默认5轮可能太激进，调整为：
- `nudge_interval`: 默认 **10轮**（与Hermes一致）
- `flush_min_turns`: 至少 **6轮** 后才触发记忆写入
- 这两个参数可通过 `~/.smartclaw/config.yaml` 配置

### 8.3 上下文压缩 — 来自 agent/context_compressor.py

Hermes 的压缩比我们的设计更精细，分四个阶段：

1. **prune_old_tool_results**: 移除过大的工具输出，替换为占位符
2. **protect_head_messages**: 保护开头的系统消息
3. **choose_tail_by_budget**: 按token预算选择尾部消息
4. **summarize_middle_turns**: 用LLM总结中间轮次
5. **_sanitize_tool_pairs**: 确保压缩后tool_call/tool_result的配对完整性

**关键差异**：Hermes 不只是简单保留"最后N条"，而是：
- 保护头部的系统提示
- 保护尾部的最近对话
- 只压缩中间的轮次
- 特别处理工具调用配对（tool_call必须有对应的tool_result）

**SmartClaw 调整**：我们的 `SmartCompactor` 需要补充：
- [ ] 工具输出裁剪（过大的tool result替换为摘要占位符）
- [ ] 头部保护（系统消息不参与压缩）
- [ ] 工具配对完整性校验（_sanitize_tool_pairs）
- [ ] 增量摘要（update_previous_summary，而非每次重新生成）

### 8.4 网关实现 — 来自 gateway/run.py

Hermes 网关的关键实现模式：

```python
class GatewayRunner:
    def __init__(self, config):
        self.session_store = SessionStore(config.sessions_dir, config, ...)
        self.delivery_router = DeliveryRouter(config)
        self._session_db = None
        try:
            from hermes_state import SessionDB
            self._session_db = SessionDB()  # SQLite持久化
        except Exception as e:
            logger.debug("SQLite session store not available: %s", e)
```

**SmartClaw 调整**：我们的 Gateway 设计已覆盖此模式，但需补充：
- [ ] SQLite 不可用时的降级逻辑（fallback到JSON文件）
- [ ] DeliveryRouter 概念（消息路由到具体平台适配器）

### 8.5 Cron 实现 — 来自 cron/scheduler.py

Hermes 的 cron 实现使用文件锁确保单实例运行：

```python
def tick(self):
    # 1. 获取文件锁（确保只有一个scheduler在跑）
    # 2. get_due_jobs() — 查找到期的任务
    # 3. 对每个job：创建AIAgent实例，带着完整记忆执行
    # 4. save_job_output — 保存结果
    # 5. _deliver_result — 通过网关推送到目标平台
    # 6. session清理
```

**SmartClaw 调整**：我们的 CronTrigger 需要补充：
- [ ] 文件锁机制（防止多实例重复执行）
- [ ] Cron任务使用独立session（`cron_{taskID}_{timestamp}`格式）
- [ ] 结果交付走网关推送而非直接返回

### 8.6 记忆文件路径

Hermes 实际的记忆文件路径：
- `~/.hermes/memories/MEMORY.md` — 系统记忆
- `~/.hermes/memories/USER.md` — 用户画像
- `~/.hermes/skills/` — 技能目录
- `~/.hermes/state.db` — SQLite数据库
- `~/.hermes/cron/` — 定时任务定义

**SmartClaw 对应**：
- `~/.smartclaw/MEMORY.md` — 系统记忆（直接放根目录，更简洁）
- `~/.smartclaw/USER.md` — 用户画像
- `~/.smartclaw/skills/` — 技能目录（已有）
- `~/.smartclaw/state.db` — SQLite数据库
- `~/.smartclaw/cron/` — 定时任务定义

### 8.7 Prompt Assembly — 来自官方文档

Hermes 的系统提示组装顺序（从上到下）：

```
1. SOUL.md (Agent身份/人格)
2. AGENTS.md (项目上下文)
3. Frozen MEMORY snapshot (MEMORY.md)
4. Frozen USER profile snapshot (USER.md)
5. Honcho static block (用户建模层)
6. Skills index (只列skill名称+摘要)
7. Tool definitions
8. Memory nudges / periodic prompts
```

**关键发现**：
- MEMORY.md 和 USER.md 是 **frozen snapshot**（冻结快照），在session开始时加载，session期间不变
- Skills 只加载 index（名字+摘要），完整内容需要时才读
- Honcho 作为独立层注入
- Nudge prompts 放在系统提示末尾

**SmartClaw 调整**：我们的 `MemoryManager.BuildSystemContext()` 需要按此顺序组装。

---

## 九、修订后的模块接口更新

基于 Hermes 源码分析，以下接口需要调整：

### 9.1 store/sqlite.go — 增加字段和并发安全

```go
type Store struct {
    db       *sql.DB
    jsonlDir string
}

// WriteWithRetry 并发安全写入（翻译自Hermes的_execute_write）
func (s *Store) WriteWithRetry(ctx context.Context, query string, args ...interface{}) error {
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        _, err := s.db.ExecContext(ctx, "BEGIN IMMEDIATE")
        if err != nil {
            if isLockedError(err) && attempt < maxRetries-1 {
                // 抖动重试
                jitter := time.Duration(rand.Intn(200)) * time.Millisecond * time.Duration(1<<attempt)
                time.Sleep(jitter)
                continue
            }
            return err
        }
        _, err = s.db.ExecContext(ctx, query, args...)
        if err != nil {
            s.db.ExecContext(ctx, "ROLLBACK")
            return err
        }
        _, err = s.db.ExecContext(ctx, "COMMIT")
        return err
    }
    return fmt.Errorf("max retries exceeded")
}
```

### 9.2 learning/nudge.go — 调整参数

```go
type NudgeConfig struct {
    Interval      int `yaml:"nudge_interval"`       // 默认10轮
    FlushMinTurns int `yaml:"flush_min_turns"`       // 默认6轮
}

func DefaultNudgeConfig() NudgeConfig {
    return NudgeConfig{
        Interval:      10,
        FlushMinTurns: 6,
    }
}
```

### 9.3 runtime/compact.go — 四阶段压缩

```go
type SmartCompactor struct {
    client    *api.Client
    store     *store.Store
    maxTokens int
}

func (sc *SmartCompactor) Compact(ctx context.Context, messages []Message, maxTokens int) ([]Message, error) {
    // Phase 1: 裁剪过大的工具输出
    messages = sc.pruneOldToolResults(messages)

    // Phase 2: 保护头部系统消息
    headEnd := sc.findHeadBoundary(messages)
    head := messages[:headEnd]
    rest := messages[headEnd:]

    // Phase 3: 按token预算选择尾部
    tailStart := sc.findTailBoundary(rest, maxTokens/3)
    tail := rest[tailStart:]
    middle := rest[:tailStart]

    // Phase 4: LLM摘要中间轮次
    summary, err := sc.summarizeMiddleTurns(ctx, middle)
    if err != nil {
        // fallback: 简单截断
    }

    // Phase 5: 确保tool_call/tool_result配对完整
    result := append(head, summary)
    result = append(result, tail...)
    result = sc.sanitizeToolPairs(result)

    return result, nil
}
```

---

## 十、参考资源

| 资源 | 链接 | 用途 |
|------|------|------|
| Hermes 源码 | https://github.com/NousResearch/hermes-agent | 核心参考 |
| hermes_state.py | commit 268ee6bd | SQLite schema + FTS5 + 并发写入 |
| run_agent.py | main branch | Agent循环 + nudge参数 + 记忆flush |
| gateway/run.py | commit 268ee6bd | 网关实现 + SessionStore + DeliveryRouter |
| agent/context_compressor.py | main branch | 四阶段压缩 + tool pair完整性 |
| cron/scheduler.py | main branch | 文件锁 + 独立session + 结果交付 |
| 官方架构文档 | https://hermes-agent.nousresearch.com/docs/developer-guide/architecture/ | 架构全貌 |
| Prompt Assembly | https://hermes-agent.nousresearch.com/docs/developer-guide/prompt-assembly | 系统提示组装顺序 |
| Session Storage | https://hermes-agent.nousresearch.com/docs/developer-guide/session-storage | SQLite + FTS5 设计 |
| Mr. Ånand 深度分析 | https://generativeai.pub/inside-hermes-agent-how-a-self-improving-ai-agent-actually-works-1aed9c529c0b | 学习循环+记忆分层设计思路 |
