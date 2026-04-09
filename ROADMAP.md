# SmartClaw 开发路线图

## 当前版本状态
- ✅ 基础 TUI 界面
- ✅ API 对接（Anthropic/OpenAI）
- ✅ 语法高亮
- ✅ 复制功能（鼠标选择 + 快捷键）
- ✅ 自动颜色检测
- ✅ Markdown 渲染

---

## 第一阶段：用户体验优化（高优先级）

### 1. 会话管理 ⭐⭐⭐⭐⭐
**优先级：最高**  
**实现难度：中等**  
**预期工作量：2-3天**

#### 功能描述
- 会话自动保存（每次对话后自动保存）
- 会话列表查看（`/sessions` 命令）
- 会话加载恢复（`/session load <id>`）
- 会话删除（`/session delete <id>`）
- 会话导出（导出为 Markdown/JSON）

#### 技术方案
```
数据存储：~/.smartclaw/sessions/
文件格式：JSON（包含消息、配置、时间戳）
命名规则：session_YYYYMMDD_HHMMSS.json
```

#### 命令设计
```
/session new           # 新建会话
/session list          # 列出所有会话
/session load <id>     # 加载会话
/session save          # 手动保存当前会话
/session delete <id>   # 删除会话
/session export <id>   # 导出会话
```

#### UI 展示
```
┌─ Sessions ─────────────────────────┐
│ ID          Date       Messages    │
│ sess_001    2025-04-08  15 msgs   │
│ sess_002    2025-04-07  8 msgs    │
│ sess_003    2025-04-06  23 msgs   │
└────────────────────────────────────┘
```

---

### 2. 多行输入支持 ⭐⭐⭐⭐⭐
**优先级：最高**  
**实现难度：中等**  
**预期工作量：1-2天**

#### 功能描述
- `Shift+Enter` 换行，不发送消息
- `Enter` 发送消息
- 多行文本编辑器（类似 textarea）
- 显示行号和当前行/列位置

#### 技术方案
- 使用 `bubbles/textarea` 替代 `textinput`
- 或自定义多行输入组件
- 状态栏显示：`Ln 3, Col 15`

#### 快捷键
```
Enter          # 发送消息
Shift+Enter    # 换行
Ctrl+Enter     # 强制发送（即使多行）
```

---

### 3. 错误处理优化 ⭐⭐⭐⭐
**优先级：高**  
**实现难度：简单**  
**预期工作量：1天**

#### 功能描述
- 错误分类（网络错误、API 错误、配置错误、权限错误）
- 友好的错误提示
- 建议解决方案
- 一键重试机制

#### 错误类型
```
网络错误：
  ✗ 无法连接到 API
  → 检查网络连接
  → [重试] [使用代理]

API 错误：
  ✗ API 密钥无效
  → 运行 /set-api-key 重新设置
  → [设置密钥]

配额错误：
  ✗ API 配额已用尽
  → 升级套餐或等待重置
  → [查看用量]

超时错误：
  ✗ 请求超时
  → [重试] [增加超时时间]
```

#### 重试机制
```
/retry              # 重试上一个请求
/retry-with-timeout # 带更长超时的重试
```

---

### 4. 快捷键系统 ⭐⭐⭐⭐
**优先级：高**  
**实现难度：简单**  
**预期工作量：1天**

#### 功能描述
- 全局快捷键支持
- 快捷键冲突检测
- 自定义快捷键映射
- 快捷键帮助面板

#### 默认快捷键
```
Ctrl+C          # 退出（已有）
Ctrl+H          # 帮助（已有）
Ctrl+S          # 保存会话
Ctrl+O          # 打开会话列表
Ctrl+K          # 命令面板
Ctrl+F          # 搜索历史
Ctrl+L          # 清屏
Ctrl+R          # 重试上一个请求
Ctrl+N          # 新建会话
Ctrl+P          # 切换模型
Tab             # 自动补全
Shift+Tab       # 反向补全
```

#### 命令面板（Ctrl+K）
```
┌─ Command Palette ──────────────────┐
│ >                                  │
│   New Session                      │
│   Open Session                     │
│   Switch Model                     │
│   Toggle Theme                     │
│   Export Conversation              │
└────────────────────────────────────┘
```

---

## 第二阶段：功能增强（中优先级）

### 5. 文件操作命令 ⭐⭐⭐⭐
**优先级：高**  
**实现难度：中等**  
**预期工作量：2-3天**

#### 功能描述
- 读取文件内容到对话
- AI 直接编辑文件
- 显示文件修改差异
- 项目文件浏览

#### 命令设计
```
/file read <path>         # 读取文件
/file preview <path>      # 预览文件（不加入上下文）
/file edit <path>         # AI 编辑文件模式
/file diff <path>         # 显示差异
/file create <path>       # 创建新文件
/file list [dir]          # 列出文件
```

#### 示例流程
```
User: /file read main.go
AI: 已读取 main.go (234 行)，已添加到上下文

User: 优化这个文件的性能
AI: [分析并给出建议]

User: /file edit main.go
AI: 进入编辑模式，我会直接修改文件
    修改完成后使用 /file diff 查看
```

---

### 6. 上下文管理增强 ⭐⭐⭐⭐
**优先级：高**  
**实现难度：中等**  
**预期工作量：2天**

#### 功能描述
- 可视化上下文占用
- 手动添加/删除消息
- 上下文压缩（自动总结）
- 重要消息标记

#### 命令设计
```
/context                  # 显示上下文统计
/context list             # 列出所有消息
/context remove <id>      # 删除指定消息
/context keep <id>        # 标记为重要
/context compress         # 压缩旧消息
/context clear            # 清空上下文
```

#### 可视化展示
```
┌─ Context Usage ────────────────────┐
│ ████████████░░░░░░░  52.3k/200k    │
│                                    │
│ Messages: 15                       │
│ Estimated cost: $0.12              │
│                                    │
│ [Compress] [Export] [Clear]        │
└────────────────────────────────────┘
```

---

### 7. 输入历史搜索 ⭐⭐⭐
**优先级：中**  
**实现难度：中等**  
**预期工作量：1-2天**

#### 功能描述
- 类似 Shell 的 Ctrl+R 反向搜索
- 搜索所有历史输入
- 支持正则表达式
- 搜索结果高亮

#### 使用方式
```
Ctrl+R          # 进入搜索模式
输入关键词       # 实时搜索
Ctrl+R          # 下一个匹配
Enter           # 选择并使用
Esc             # 取消
```

---

### 8. 输出增强 ⭐⭐⭐
**优先级：中**  
**实现难度：中等**  
**预期工作量：2天**

#### 功能描述
- 长代码块折叠
- 消息时间戳
- 消息书签
- 输出过滤（只看代码/只看文本）

#### 折叠展示
```
◆ SmartClaw:
  这是一个 Python 脚本...
  
  ┌─ main.py (点击展开) ───┐
  │ [234 行代码]          │
  └───────────────────────┘
  
  详细说明...
```

#### 时间戳
```
▶ You [14:32]:
  写一个 HTTP 服务器

◆ SmartClaw [14:32]:
  [回复内容]
```

---

### 9. 配置管理系统 ⭐⭐⭐
**优先级：中**  
**实现难度：简单**  
**预期工作量：1天**

#### 功能描述
- 配置文件管理（~/.smartclaw/config.yaml）
- 配置项查看和修改
- 配置导入/导出
- 多配置切换

#### 命令设计
```
/config                   # 显示所有配置
/config show <key>        # 显示单个配置项
/config set <key> <value> # 设置配置项
/config reset             # 重置为默认
/config import <file>     # 导入配置
/config export <file>     # 导出配置
/config profiles          # 管理配置方案
```

#### 配置文件示例
```yaml
api:
  provider: anthropic
  model: claude-sonnet-4-5
  max_tokens: 4096
  timeout: 60
  
ui:
  theme: dark
  show_timestamps: true
  wrap_code: true
  
behavior:
  auto_save: true
  stream: true
  
shortcuts:
  new_session: Ctrl+N
  open_session: Ctrl+O
```

---

## 第三阶段：高级功能（低优先级）

### 10. 代码执行 ⭐⭐⭐
**优先级：中**  
**实现难度：高**  
**预期工作量：3-4天**

#### 功能描述
- 在沙箱中执行代码
- 支持多种语言（Python, JavaScript, Shell）
- 执行结果显示
- 安全限制（超时、文件访问）

#### 命令设计
```
/run <language> <code>    # 执行代码
/run file <path>          # 执行文件
/python                   # Python REPL
/shell                    # Shell 命令
```

#### 安全措施
- 超时限制（默认 10 秒）
- 内存限制
- 禁止网络访问
- 沙箱环境（Docker 可选）

---

### 11. 多模型支持 ⭐⭐⭐
**优先级：中**  
**实现难度：中等**  
**预期工作量：2天**

#### 功能描述
- 快速切换模型
- 模型对比模式
- 成本预估
- 模型能力说明

#### 命令设计
```
/model                    # 显示当前模型
/model list               # 列出可用模型
/model switch <name>      # 切换模型
/model compare            # 对比模式（两个模型回答同一问题）
/model info <name>        # 模型详细信息
```

#### 对比模式展示
```
┌─ claude-sonnet ──────┬─ gpt-4 ────────────┐
│ 回答内容...          │ 回答内容...         │
│                      │                     │
│ Tokens: 234          │ Tokens: 312         │
│ Cost: $0.02          │ Cost: $0.03         │
└──────────────────────┴─────────────────────┘
```

---

### 12. AI 角色系统 ⭐⭐⭐
**优先级：中**  
**实现难度：中等**  
**预期工作量：2天**

#### 功能描述
- 预设 AI 角色（代码专家、翻译、写作助手）
- 自定义角色
- 角色切换
- 角色市场（分享角色）

#### 命令设计
```
/role                     # 显示当前角色
/role list                # 列出所有角色
/role use <name>          # 使用角色
/role create <name>       # 创建角色
/role edit <name>         # 编辑角色
```

#### 预设角色
```yaml
roles:
  - name: 代码专家
    system: 你是一位资深软件工程师...
    model: claude-sonnet
    
  - name: 翻译助手
    system: 你是专业翻译...
    model: claude-sonnet
    
  - name: 写作助手
    system: 你是创意写作专家...
    model: claude-sonnet
```

---

### 13. 提示词模板 ⭐⭐⭐
**优先级：中**  
**实现难度：简单**  
**预期工作量：1-2天**

#### 功能描述
- 保存常用提示词为模板
- 快速调用模板
- 变量替换
- 模板分享

#### 命令设计
```
/template                 # 列出模板
/template use <name>      # 使用模板
/template save <name>     # 保存为模板
/template edit <name>     # 编辑模板
```

#### 模板示例
```yaml
name: code-review
template: |
  请审查以下代码：
  
  文件：{{filename}}
  语言：{{language}}
  
  ```
  {{code}}
  ```
  
  关注点：
  - 代码质量
  - 性能优化
  - 安全问题
variables:
  - filename
  - language
  - code
```

---

### 14. Git 集成 ⭐⭐
**优先级：低**  
**实现难度：中等**  
**预期工作量：2-3天**

#### 功能描述
- Git 状态查看
- AI 自动提交
- PR 创建辅助
- 代码审查

#### 命令设计
```
/git status               # Git 状态
/git commit               # AI 生成提交信息并提交
/git review               # AI 审查变更
/git pr                   # 创建 PR 辅助
/git diff                 # 显示差异
```

---

### 15. 编辑器集成 ⭐⭐
**优先级：低**  
**实现难度：高**  
**预期工作量：5-7天**

#### 功能描述
- VSCode 插件
- JetBrains 插件
- LSP 协议支持
- 实时补全

---

### 16. 团队协作 ⭐
**优先级：低**  
**实现难度：高**  
**预期工作量：7-10天**

#### 功能描述
- 会话分享
- 团队工作区
- 权限管理
- 协作编辑

---

## 技术债务和优化

### 代码质量
- [ ] 单元测试覆盖率提升到 60%+
- [ ] 代码注释规范化
- [ ] 错误处理统一化
- [ ] 日志系统完善

### 性能优化
- [ ] 大文件懒加载
- [ ] 流式输出优化
- [ ] 内存使用优化
- [ ] 启动速度优化

### 文档完善
- [ ] 用户文档
- [ ] 开发者文档
- [ ] API 文档
- [ ] 架构设计文档

---

## 实现优先级总结

### P0（立即实现）
1. 会话管理
2. 多行输入支持
3. 错误处理优化
4. 快捷键系统

### P1（近期规划）
5. 文件操作命令
6. 上下文管理增强
7. 输入历史搜索
8. 输出增强

### P2（中期规划）
9. 配置管理系统
10. 代码执行
11. 多模型支持
12. AI 角色系统

### P3（长期规划）
13. 提示词模板
14. Git 集成
15. 编辑器集成
16. 团队协作

---

## 贡献指南

### 开发环境设置
```bash
# 克隆仓库
git clone <repo-url>
cd smartclaw

# 安装依赖
cd go
go mod download

# 运行测试
go test ./...

# 本地运行
go run ./cmd/smartclaw tui
```

### 代码规范
- 遵循 Go 官方代码规范
- 使用 gofmt 格式化代码
- 函数名使用驼峰命名
- 注释使用完整句子

### 提交规范
```
feat: 添加会话管理功能
fix: 修复鼠标选择复制问题
docs: 更新 README
refactor: 重构输出渲染逻辑
test: 添加单元测试
```

---

## 版本规划

### v0.2.0（下一版本）
- 会话管理
- 多行输入
- 错误处理优化

### v0.3.0
- 文件操作
- 上下文管理增强
- 配置系统

### v0.4.0
- 代码执行
- 多模型支持
- AI 角色

### v1.0.0
- 稳定版本
- 完整文档
- 测试覆盖
- 性能优化

---

## 反馈和建议

欢迎提交 Issue 和 Pull Request！

- GitHub Issues: [项目地址]
- 功能建议: [讨论区]
- Bug 报告: [Issues]
