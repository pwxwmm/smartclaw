[English](../README.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md)

> 이 문서는 [영문 README](../README.md)의 번역본입니다. 영문 버전이 권위 있는 원본입니다.

# SmartClaw

사용할수록 당신의 워크플로를 학습하는 자기 개선형 AI 에이전트.

SmartClaw은 작업을 수행할수록 더 똑똑해지는 자율형 코딩 에이전트입니다. 완료된 작업을 평가하고, 재사용 가능한 방법을 추출하며, 자동으로 스킬을 생성하는 학습 루프를 갖추고 있습니다. 많이 사용할수록 당신의 작업 방식을 더 깊이 이해합니다.

## 핵심 철학

**"越来越懂你的工作方式"** — "모든 것에 능한" 것이 아니라, "당신의 모든 것에 능한" 에이전트.

세션 사이에 모든 것을 잊어버리는 일반적인 AI 어시스턴트와 달리, SmartClaw은:

- **완료된 작업에서 학습** — 접근 방식의 재사용 가치를 평가하고, 가치가 있으면 스킬로 추출
- **세션 간 기억** — SQLite + FTS5 전문 검색 기반 4계층 메모리 시스템
- **시간이 지남에 따라 자기 개선** — 주기적인 넛지가 에이전트에게 메모리 통합과 스킬 정제를 유도
- **사용자 선호 이해** — 소통 스타일, 지식 배경, 일반적인 워크플로를 수동적으로 추적

## 기능

### 에이전트 역량

- **학습 루프**: 작업 완료 후 평가 → 방법 추출 → 스킬 생성 → MEMORY.md 자동 업데이트
- **4계층 메모리**: 프롬프트 메모리 (MEMORY.md/USER.md), 세션 검색 (FTS5), 스킬 절차 (지연 로딩), 사용자 모델링
- **주기적 넛지**: 10턴마다 시스템이 자체 리뷰를 트리거 (설정 가능)
- **스마트 압축**: 설정 가능한 임계값으로 자동 압축, 헤드 보호, 도구 결과 정리, 소스 추적 가능한 요약
- **추측 실행**: 듀얼 모델 라우팅. 빠른 모델과 무거운 모델을 병렬 실행하고, 결과가 유사하면 빠른 모델 채택, 다르면 무거운 모델로 폴백
- **적응형 모델 라우터**: 복잡도 기반 모델 선택 (fast/default/heavy), 비용 우선 / 품질 우선 / 균형 전략
- **비용 가드**: 예산 인식형 지출 관리. 일일/세션 한도, 경고 임계값, 한도 근접 시 자동 모델 다운그레이드

### 개발 도구

- **73개 이상의 내장 도구**: 파일 조작, 코드 분석, 웹 도구, MCP 통합, 브라우저 자동화, Docker 샌드박스 등
- **101개 슬래시 명령**: 에이전트 관리, 템플릿 시스템, IDE 통합을 갖춘 생산성 명령 모음
- **모던 TUI**: Bubble Tea로 구축된 터미널 사용자 인터페이스
- **대화형 REPL**: 스트리밍 응답이 포함된 전체 대화 기록
- **MCP 통합**: MCP 서버 연결, 도구 발견, 리소스 읽기, OAuth 인증
- **ACP 서버**: stdio JSON-RPC를 통한 IDE 통합 (VS Code, Zed, JetBrains)용 Agent Communication Protocol
- **VS Code 확장**: 채팅 사이드바, 코드 설명, 수정, 테스트 생성 명령을 갖춘 공식 확장
- **보안**: 4가지 모드의 권한 시스템, Linux에서 샌드박스 실행, Docker 격리
- **토큰 추적**: 실시간 비용 추정 및 임계값에서 자동 압축

### 브라우저 자동화

- **헤드리스 브라우저**: Chromium (chromedp 경유)을 사용한 탐색, 클릭, 입력, 스크린샷, 콘텐츠 추출, 폼 작성
- **8개 브라우저 도구**: `browser_navigate`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_extract`, `browser_wait`, `browser_select`, `browser_fill_form`

### 코드 실행 및 샌드박스

- **코드 실행 도구**: RPC 샌드박스에서 Python 코드를 실행하고 SmartClaw 도구 (read_file, write_file, glob, grep, bash, web_search, web_fetch)에 직접 접근. 멀티턴 워크플로를 싱글턴으로 압축
- **Docker 샌드박스**: 프로젝트 디렉토리를 `/workspace`에 마운트한 격리 컨테이너 실행. 원샷과 세션 지속형 모두 지원
- **Linux 네임스페이스 샌드박스**: Linux 네임스페이스를 사용한 네이티브 샌드박스 실행으로 안전한 격리

### 게이트웨이 및 크로스 플랫폼

- **통합 게이트웨이**: 메시지 → 라우팅 → 메모리 → 실행 → 학습 → 전달
- **플랫폼 어댑터**: 터미널, Web UI, Telegram, Discord로 확장 가능
- **Cron 작업**: 전체 메모리 접근이 가능한 일급 에이전트 작업으로서의 예약 작업
- **세션 라우팅**: 플랫폼이 아닌 userID 기반 라우팅. 기기를 전환해도 컨텍스트 유지
- **세션 녹화**: 감사 및 검토를 위한 전체 세션 녹화 및 재생
- **원격 트리거**: SSH를 통해 원격 호스트에서 명령 실행

### 팀 협업

- **팀 워크스페이스**: AES 암호화 메모리 동기화로 공유 팀 공간 생성
- **팀 메모리 공유**: 팀원 간 메모리, 세션, 지식 공유
- **팀 도구**: `team_create`, `team_delete`, `team_share_memory`, `team_get_memories`, `team_search_memories`, `team_sync`, `team_share_session`

### 관측 가능성 및 분석

- **메트릭 대시보드**: 실시간 쿼리 수, 캐시 적중률, 토큰 사용량, 비용 추정, 도구 실행 통계, 모델별 쿼리 수
- **분산 추적**: 레이턴시 및 장애 디버깅을 위한 요청 수준 추적
- **텔레메트리 API**: 전체 관측 데이터를 노출하는 REST 엔드포인트 (`/api/telemetry`)

### 배치 및 RL 평가

- **배치 러너**: 수백 개의 프롬프트에 대해 에이전트를 병렬 실행하고 ShareGPT 형식의 학습 궤적 출력
- **RL 평가**: 설정 가능한 메트릭 (exact_match, code_quality, length_penalty)으로 보상 기반 평가 루프 실행
- **궤적 내보내기**: 강화학습 연구를 위한 단계별 보상이 포함된 에피소드 데이터 내보내기

### OpenAI 호환

- **OpenAI API 형식**: `--openai` 플래그 또는 설정으로 OpenAI 호환 API 엔드포인트 완전 지원
- **커스텀 베이스 URL**: `--url` 플래그로 모든 OpenAI 호환 제공자 지정
- **멀티 프로바이더**: Anthropic과 OpenAI 호환 백엔드 간 원활한 전환

## 아키텍처

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

### 4계층 메모리 시스템

| 계층 | 이름 | 저장소 | 동작 |
|-------|------|---------|----------|
| L1 | 프롬프트 메모리 | `MEMORY.md` + `USER.md` | 세션마다 자동 로드, 3,575자 하드 리미트 |
| L2 | 세션 검색 | SQLite + FTS5 | 에이전트가 관련 기록 검색, 주입 전 LLM 요약 |
| L3 | 스킬 절차 | `~/.smartclaw/skills/` | 스킬 이름과 설명만 로드, 전체 콘텐츠는 온디맨드 |
| L4 | 사용자 모델링 | `user_observations` 테이블 | 선호도를 수동적으로 추적, USER.md 자동 업데이트 |

### 학습 루프

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

### 추측 실행

```
User Query
    ├── Fast Model (Haiku) → result in ~1s
    └── Heavy Model (Opus) → result in ~5s
            ↓
    Compare: similarity > 0.7?
        ↓ Yes              ↓ No
    Use fast result    Use heavy result
```

### 적응형 모델 라우팅

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

## 빠른 시작

### 요구 사항

- Go 1.25+
- Anthropic API 키 (또는 OpenAI 호환 API 키)

### 설치

```bash
go build -o bin/smartclaw ./cmd/smartclaw/
```

### 기본 사용법

```bash
# TUI 모드로 시작 (권장)
./bin/smartclaw tui

# 간단한 REPL 시작
./bin/smartclaw repl

# 단일 프롬프트 전송
./bin/smartclaw prompt "Explain this code"

# 특정 모델 사용
./bin/smartclaw --model claude-opus-4-6 repl

# WebUI 서버 시작
./bin/smartclaw web --port 8080

# IDE 통합용 ACP 서버 시작
./bin/smartclaw acp

# 멀티 플랫폼 게이트웨이 시작
./bin/smartclaw gateway --adapters telegram,web --telegram-token <BOT_TOKEN>

# 배치 평가 실행
./bin/smartclaw batch --prompts prompts.jsonl --output trajectories/

# RL 평가 루프 실행
./bin/smartclaw rl-eval --tasks tasks.jsonl --metric code_quality --output rl-output/

# OpenAI 호환 API 사용
./bin/smartclaw --openai --url https://api.your-provider.com/v1 repl
```

### 설정

Anthropic API 키 설정:

```bash
export ANTHROPIC_API_KEY=your_key_here
```

또는 `~/.smartclaw/config.yaml` 생성:

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

### 데이터 디렉토리

SmartClaw은 `~/.smartclaw/` 아래에 다음 파일을 자동 생성하고 관리합니다:

| 경로 | 설명 |
|------|-------------|
| `MEMORY.md` | 시스템 메모리, 학습 루프에 의해 자동 업데이트 |
| `USER.md` | 사용자 프로필, 관찰에서 자동 진화 |
| `state.db` | FTS5 인덱스가 포함된 SQLite 데이터베이스 |
| `skills/` | 학습된 및 번들된 스킬 |
| `cron/` | 예약 작업 정의 (JSON) |
| `recordings/` | 세션 녹화 (JSONL) |
| `mcp/servers.json` | MCP 서버 설정 |
| `exports/` | 내보낸 세션 |
| `outbox/` | 크로스 플랫폼 메시지 대기열 |

`MEMORY.md`와 `USER.md`는 직접 편집할 수 있습니다. SmartClaw은 다음 사용 시 다시 로드합니다.

## 사용 가능한 도구 (73+)

### 파일 조작

| 도구 | 설명 |
|------|-------------|
| `bash` | 타임아웃 및 백그라운드 지원 셸 명령 실행 |
| `read_file` | 파일 내용 읽기 |
| `write_file` | 파일 쓰기 |
| `edit_file` | 문자열 교체 편집 |
| `glob` | 파일 패턴 매칭 |
| `grep` | 정규식 지원 콘텐츠 검색 |
| `powershell` | PowerShell 명령 실행 (Windows) |

### 코드 분석

| 도구 | 설명 |
|------|-------------|
| `lsp` | LSP 작업 (goto_definition, find_references, rename, diagnostics) |
| `ast_grep` | AST 패턴 검색 및 교체 |
| `code_search` | 시맨틱 코드 검색 |
| `index` | 검색용 코드 인덱스 |

### 웹 및 브라우저

| 도구 | 설명 |
|------|-------------|
| `web_fetch` | URL을 가져와 마크다운으로 변환 |
| `web_search` | 웹 검색 |
| `browser_navigate` | 헤드리스 브라우저에서 URL로 이동 |
| `browser_click` | CSS 선택자로 요소 클릭 |
| `browser_type` | 요소에 텍스트 입력 |
| `browser_screenshot` | 페이지 스크린샷 캡처 |
| `browser_extract` | 페이지 콘텐츠/텍스트 추출 |
| `browser_wait` | 요소 또는 조건 대기 |
| `browser_select` | 드롭다운에서 옵션 선택 |
| `browser_fill_form` | 여러 폼 필드 작성 |

### MCP 통합

| 도구 | 설명 |
|------|-------------|
| `mcp` | 연결된 MCP 서버에서 도구 실행 (SSE/stdio 전송) |
| `list_mcp_resources` | MCP 서버에서 사용 가능한 리소스 나열 |
| `read_mcp_resource` | 연결된 MCP 서버에서 리소스 읽기 |
| `mcp_auth` | OAuth 플로우로 MCP 서버 인증 |

### 에이전트 및 학습

| 도구 | 설명 |
|------|-------------|
| `agent` | 병렬 작업용 서브 에이전트 실행 |
| `skill` | 스킬 로드 및 관리 |
| `session` | 세션 관리 |
| `todowrite` | 확인 넛지가 있는 Todo 목록 관리 |
| `config` | 설정 관리 |
| `memory` | 4계층 메모리 조회 및 관리 (recall, search, store, layers, stats) |

### 코드 실행 및 샌드박스

| 도구 | 설명 |
|------|-------------|
| `execute_code` | 도구 접근이 가능한 RPC 샌드박스에서 Python 코드 실행. 멀티턴을 싱글턴으로 압축 |
| `docker_exec` | 격리된 Docker 컨테이너에서 명령 실행 (원샷 또는 세션 지속형) |
| `repl` | 샌드박스 타임아웃으로 JavaScript (Node.js) 또는 Python 표현식 평가 |

### Git 작업

| 도구 | 설명 |
|------|-------------|
| `git_ai` | AI 기반 커밋 메시지, 코드 리뷰, PR 설명 |
| `git_status` | 작업 디렉토리의 Git 상태 |
| `git_diff` | Git diff (스테이지됨 또는 스테이지 안 됨) |
| `git_log` | 최근 Git 커밋 로그 |

### 배치 및 병렬

| 도구 | 설명 |
|------|-------------|
| `batch` | 여러 도구 호출을 배치 실행 |
| `parallel` | 여러 도구 호출을 병렬 실행 |
| `pipeline` | 출력 파이핑으로 도구 호출 체인 |

### 팀 협업

| 도구 | 설명 |
|------|-------------|
| `team_create` | 메모리 공유용 팀 워크스페이스 생성 |
| `team_delete` | 팀 워크스페이스 삭제 |
| `team_share_memory` | 메모리 항목을 팀과 공유 |
| `team_get_memories` | 공유된 팀 메모리 가져오기 |
| `team_search_memories` | 팀 메모리 전체 검색 |
| `team_sync` | 멤버 간 팀 상태 동기화 |
| `team_share_session` | 세션을 팀과 공유 |

### 원격 및 메시징

| 도구 | 설명 |
|------|-------------|
| `remote_trigger` | SSH로 원격 호스트에서 명령 실행 |
| `send_message` | 플랫폼 간 채널/사용자에게 메시지 전송 (telegram, web, terminal) |

### 워크플로 및 기획

| 도구 | 설명 |
|------|-------------|
| `enter_worktree` | 병렬 개발용 Git 워크트리 생성 |
| `exit_worktree` | Git 워크트리 제거 및 정리 |
| `enter_plan_mode` | 구조화된 기획 모드 진입 |
| `exit_plan_mode` | 기획 모드 종료 및 실행 재개 |
| `schedule_cron` | Cron 작업 예약, 목록 조회, 삭제 |

### 미디어 및 문서

| 도구 | 설명 |
|------|-------------|
| `image` | 이미지 분석 및 처리 |
| `pdf` | PDF 문서에서 텍스트 추출 |
| `audio` | 오디오 파일 처리 및 전사 |

### 인지 도구

| 도구 | 설명 |
|------|-------------|
| `think` | 행동 전 구조화된 사고 단계 |
| `deep_think` | 복잡한 문제를 위한 확장 추론 |
| `brief` | 간결한 주제 요약 |
| `observe` | 수동적 분석을 위한 관찰 모드 |
| `lazy` | 온디맨드 지연 도구 로딩 |
| `fork` | 병렬 탐색을 위해 현재 세션 포크 |

### 유틸리티

| 도구 | 설명 |
|------|-------------|
| `tool_search` | 키워드로 사용 가능한 도구 검색 |
| `cache` | 도구 결과 캐시 관리 |
| `attach` | 실행 중인 프로세스에 연결 |
| `debug` | 디버그 모드 전환 |
| `env` | 환경 변수 표시 |
| `sleep` | 지정 시간 동안 대기 |

## 슬래시 명령 (101)

### 코어

| 명령 | 설명 |
|---------|-------------|
| `/help` | 사용 가능한 명령 표시 |
| `/status` | 세션 상태 |
| `/exit` | REPL 종료 |
| `/clear` | 세션 초기화 |
| `/version` | 버전 표시 |

### 모델 및 설정

| 명령 | 설명 |
|---------|-------------|
| `/model [name]` | 모델 표시 또는 설정 |
| `/model-list` | 사용 가능한 모델 나열 |
| `/config` | 설정 표시 |
| `/config-show` | 전체 설정 표시 |
| `/config-set` | 설정 값 지정 |
| `/config-get` | 설정 값 가져오기 |
| `/config-reset` | 설정 초기화 |
| `/config-export` | 설정 내보내기 |
| `/config-import` | 설정 가져오기 |
| `/set-api-key <key>` | API 키 설정 |
| `/env` | 환경 표시 |

### 세션

| 명령 | 설명 |
|---------|-------------|
| `/session` | 세션 나열 |
| `/resume` | 세션 재개 |
| `/save` | 현재 세션 저장 |
| `/export` | 세션 내보내기 (markdown/json) |
| `/import` | 세션 가져오기 |
| `/rename` | 세션 이름 변경 |
| `/fork` | 병렬 탐색을 위해 세션 포크 |
| `/rewind` | 세션 상태 되감기 |
| `/share` | 세션 공유 |
| `/summary` | 세션 요약 |
| `/attach` | 프로세스에 연결 |

### 압축

| 명령 | 설명 |
|---------|-------------|
| `/compact` | 컨텍스트 사용량 표시 |
| `/compact now` | 대화 기록 수동 압축 |
| `/compact auto` | 자동 압축 켜기/끄기 |
| `/compact status` | 압축 통계 표시 |
| `/compact config` | 압축 설정 표시 |

### 에이전트 시스템

| 명령 | 설명 |
|---------|-------------|
| `/agent` | AI 에이전트 관리 |
| `/agent-list` | 사용 가능한 에이전트 나열 |
| `/agent-switch` | 에이전트 전환 |
| `/agent-create` | 커스텀 에이전트 생성 |
| `/agent-delete` | 커스텀 에이전트 삭제 |
| `/agent-info` | 에이전트 정보 표시 |
| `/agent-export` | 에이전트 설정 내보내기 |
| `/agent-import` | 에이전트 설정 가져오기 |
| `/subagent` | 서브에이전트 실행 |
| `/agents` | 사용 가능한 에이전트 나열 |

### 템플릿 시스템

| 명령 | 설명 |
|---------|-------------|
| `/template` | 프롬프트 템플릿 관리 |
| `/template-list` | 템플릿 나열 |
| `/template-use` | 템플릿 사용 |
| `/template-create` | 템플릿 생성 |
| `/template-delete` | 템플릿 삭제 |
| `/template-info` | 템플릿 정보 표시 |
| `/template-export` | 템플릿 내보내기 |
| `/template-import` | 템플릿 가져오기 |

### 메모리 및 학습

| 명령 | 설명 |
|---------|-------------|
| `/memory` | 메모리 컨텍스트 표시 |
| `/skills` | 사용 가능한 스킬 나열 |
| `/observe` | 관찰 모드 |

### MCP

| 명령 | 설명 |
|---------|-------------|
| `/mcp` | MCP 서버 관리 |
| `/mcp-add` | MCP 서버 추가 |
| `/mcp-remove` | MCP 서버 제거 |
| `/mcp-list` | MCP 서버 나열 |
| `/mcp-start` | MCP 서버 시작 |
| `/mcp-stop` | MCP 서버 중지 |

### Git

| 명령 | 설명 |
|---------|-------------|
| `/git-status` (`/gs`) | Git 상태 표시 |
| `/git-diff` (`/gd`) | Git diff 표시 |
| `/git-commit` (`/gc`) | 변경사항 커밋 |
| `/git-branch` (`/gb`) | 브랜치 나열 |
| `/git-log` (`/gl`) | Git 로그 표시 |
| `/diff` | diff 표시 |
| `/commit` | Git 커밋 바로가기 |

### 도구 및 개발

| 명령 | 설명 |
|---------|-------------|
| `/tools` | 사용 가능한 도구 나열 |
| `/tasks` | 작업 나열 또는 관리 |
| `/lsp` | LSP 작업 |
| `/read` | 파일 읽기 |
| `/write` | 파일 쓰기 |
| `/exec` | 명령 실행 |
| `/browse` | 브라우저 열기 |
| `/web` | 웹 작업 |
| `/ide` | IDE 통합 |
| `/install` | 패키지 설치 |

### 진단

| 명령 | 설명 |
|---------|-------------|
| `/doctor` | 진단 실행 |
| `/cost` | 토큰 사용량 및 비용 표시 |
| `/stats` | 세션 통계 표시 |
| `/usage` | 사용 통계 |
| `/debug` | 디버그 모드 전환 |
| `/inspect` | 내부 상태 검사 |
| `/cache` | 캐시 관리 |
| `/heapdump` | 힙 덤프 |
| `/reset-limits` | 속도 제한 초기화 |

### 기획 및 사고

| 명령 | 설명 |
|---------|-------------|
| `/plan` | 기획 모드 |
| `/think` | 사고 모드 |
| `/deepthink` | 심층 사고 |
| `/ultraplan` | 울트라 기획 |
| `/thinkback` | 회고 사고 |

### 협업 및 소통

| 명령 | 설명 |
|---------|-------------|
| `/invite` | 협업 초대 |
| `/feedback` | 피드백 전송 |
| `/issue` | 이슈 트래커 |

### UI 및 개인화

| 명령 | 설명 |
|---------|-------------|
| `/theme` | 테마 관리 |
| `/color` | 색상 테마 |
| `/vim` | Vim 모드 |
| `/keybindings` | 키바인딩 관리 |
| `/statusline` | 상태 표시줄 |
| `/stickers` | 스티커 |

### 모드 전환

| 명령 | 설명 |
|---------|-------------|
| `/fast` | 빠른 모드 (가벼운 모델 사용) |
| `/lazy` | 지연 로딩 모드 |
| `/desktop` | 데스크톱 모드 |
| `/mobile` | 모바일 모드 |
| `/chrome` | Chrome 통합 |
| `/voice` | 음성 모드 제어 |

### 인증 및 업데이트

| 명령 | 설명 |
|---------|-------------|
| `/login` | 서비스 인증 |
| `/logout` | 인증 정보 삭제 |
| `/upgrade` | CLI 버전 업그레이드 |
| `/api` | API 작업 |

### 기타

| 명령 | 설명 |
|---------|-------------|
| `/init` | 새 프로젝트 초기화 |
| `/context` | 컨텍스트 관리 |
| `/permissions` | 권한 관리 |
| `/hooks` | 훅 관리 |
| `/plugin` | 플러그인 관리 |
| `/passes` | LSP 패스 |
| `/preview` | 변경사항 미리보기 |
| `/effort` | 에포트 추적 |
| `/tag` | 태그 관리 |
| `/copy` | 클립보드에 복사 |
| `/files` | 파일 나열 |
| `/advisor` | AI 어드바이저 |
| `/btw` | By the way |
| `/bughunter` | 버그 헌팅 모드 |
| `/insights` | 코드 인사이트 |
| `/onboarding` | 온보딩 |
| `/teleport` | 텔레포트 모드 |
| `/summary` | 세션 요약 |

## 프로젝트 구조

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

## VS Code 확장

SmartClaw은 ACP (Agent Communication Protocol)를 통해 연결되는 VS Code 확장을 제공합니다:

### 명령

| 명령 | 설명 |
|---------|-------------|
| `SmartClaw: Ask` | SmartClaw에게 질문 |
| `SmartClaw: Open Chat` | 채팅 사이드바 열기 |
| `SmartClaw: Explain Code` | 선택한 코드 설명 |
| `SmartClaw: Fix Code` | 선택한 코드의 문제 수정 |
| `SmartClaw: Generate Tests` | 선택한 코드의 테스트 생성 |

### 설치

1. SmartClaw 빌드: `go build -o bin/smartclaw ./cmd/smartclaw/`
2. `smartclaw`을 PATH에 추가
3. `extensions/vscode/`에서 확장 설치
4. 탐색기에서 SmartClaw 사이드바 열기

## API 사용법

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

## 환경 변수

| 변수 | 설명 |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic용 API 키 |
| `SMARTCLAW_MODEL` | 기본으로 사용할 모델 |
| `SMARTCLAW_CONFIG` | 설정 파일 경로 |
| `SMARTCLAW_SESSION_DIR` | 세션 저장 디렉토리 |
| `SMARTCLAW_LOG_LEVEL` | 로그 레벨 (debug, info, warn, error) |

## 테스트

```bash
# 모든 테스트 실행
go test ./...

# 특정 패키지 실행
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

# 커버리지 포함 실행
go test -cover ./...
```

## 라이선스

MIT License
