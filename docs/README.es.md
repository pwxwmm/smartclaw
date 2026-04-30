[English](../README.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md)

> Esta es una traducción del [README en inglés](../README.md). La versión en inglés es la autoritativa.

# SmartClaw

Un agente de IA que se mejora a sí mismo y aprende tu forma de trabajar con el tiempo.

SmartClaw es un agente de programación autónomo que se vuelve más inteligente con cada tarea. Cuenta con un bucle de aprendizaje que evalúa las tareas completadas, extrae métodos reutilizables y crea habilidades automáticamente. Cuanto más lo usas, mejor entiende cómo trabajas.

## Filosofía Central

**"越来越懂你的工作方式"** — No "mejor en todo", sino "mejor en *tu* todo".

A diferencia de los asistentes de IA genéricos que lo olvidan todo entre sesiones, SmartClaw:

- **Aprende de las tareas completadas** — evalúa si un enfoque vale la pena reutilizar, y si es así, lo extrae como habilidad
- **Recuerda entre sesiones** — sistema de memoria de 4 capas con SQLite + búsqueda de texto completo FTS5
- **Se mejora con el tiempo** — los recordatorios periódicos activan la consolidación de memoria y el refinamiento de habilidades
- **Entiende tus preferencias** — rastrea pasivamente tu estilo de comunicación, conocimientos y flujos de trabajo habituales

## Características

### Capacidades del Agente

- **Bucle de Aprendizaje**: Evaluación post-tarea → extracción de métodos → creación de habilidades → auto-actualización de MEMORY.md
- **Memoria de 4 Capas**: Memoria de Prompt (MEMORY.md/USER.md), Búsqueda de Sesión (FTS5), Procedural de Habilidades (carga diferida), Modelado de Usuario
- **Recordatorio Periódico**: Auto-revisión activada por el sistema cada 10 turnos (configurable)
- **Compactación Inteligente**: Auto-compactación con umbrales configurables, protección de cabecera, poda de resultados de herramientas y resúmenes rastreables
- **Ejecución Especulativa**: Enrutamiento de modelo dual. Ejecuta modelos rápido y pesado en paralelo, acepta el resultado rápido si es similar, recurre al pesado si diverge
- **Enrutador de Modelo Adaptativo**: Selección de modelo basada en complejidad (fast/default/heavy) con estrategias de costo primero, calidad primero o equilibrada
- **Guardián de Costos**: Gestión de gastos con conocimiento de presupuesto. Límites diarios/sesión, umbrales de advertencia y degradación automática de modelo al acercarse a los límites

### Herramientas de Desarrollo

- **73+ Herramientas Integradas**: Operaciones de archivos, análisis de código, herramientas web, integración MCP, automatización de navegador, sandboxing con Docker y más
- **101 Comandos Slash**: Suite completa de comandos de productividad con gestión de agentes, sistema de plantillas e integración IDE
- **TUI Moderna**: Interfaz de usuario de terminal construida con Bubble Tea
- **REPL Interactivo**: Historial completo de conversación con respuestas en streaming
- **Integración MCP**: Conéctate a servidores MCP, descubre herramientas, lee recursos y autentícate vía OAuth
- **Servidor ACP**: Agent Communication Protocol para integración IDE (VS Code, Zed, JetBrains) vía stdio JSON-RPC
- **Extensión VS Code**: Extensión oficial con barra lateral de chat, explicación de código, corrección y generación de pruebas
- **Seguro**: Sistema de permisos con 4 modos, ejecución en sandbox en Linux, aislamiento con Docker
- **Seguimiento de Tokens**: Estimación de costo en tiempo real con auto-compactación en umbral

### Automatización de Navegador

- **Navegador Headless**: Navega, haz clic, escribe, captura pantallas, extrae contenido y rellena formularios usando Chromium (vía chromedp)
- **8 Herramientas de Navegador**: `browser_navigate`, `browser_click`, `browser_type`, `browser_screenshot`, `browser_extract`, `browser_wait`, `browser_select`, `browser_fill_form`

### Ejecución de Código y Sandboxing

- **Herramienta de Ejecución de Código**: Ejecuta código Python en un sandbox RPC con acceso directo a herramientas de SmartClaw (read_file, write_file, glob, grep, bash, web_search, web_fetch). Colapsa flujos de trabajo multi-turno en un solo turno
- **Sandbox Docker**: Ejecución aislada en contenedor con el directorio del proyecto montado en `/workspace`, soportando contenedores de un solo uso y persistentes por sesión
- **Sandbox de Namespaces Linux**: Ejecución nativa en sandbox usando namespaces de Linux para aislamiento seguro

### Gateway y Multiplataforma

- **Gateway Unificado**: Mensaje → Ruta → Memoria → Ejecutar → Aprender → Entregar
- **Adaptadores de Plataforma**: Terminal, Web UI, Telegram, extensible a Discord
- **Tareas Cron**: Tareas programadas como tareas de agente de primera clase con acceso completo a memoria
- **Enrutamiento de Sesión**: Enrutamiento basado en userID, no en plataforma. Cambia de dispositivo sin perder contexto
- **Grabación de Sesión**: Graba y reproduce sesiones completas para auditoría y revisión
- **Disparador Remoto**: Ejecuta comandos en hosts remotos vía SSH

### Colaboración en Equipo

- **Espacios de Trabajo en Equipo**: Crea espacios compartidos con sincronización de memoria cifrada con AES
- **Compartir Memoria de Equipo**: Comparte memorias, sesiones y conocimiento entre miembros del equipo
- **Herramientas de Equipo**: `team_create`, `team_delete`, `team_share_memory`, `team_get_memories`, `team_search_memories`, `team_sync`, `team_share_session`

### Observabilidad y Analítica

- **Panel de Métricas**: Conteo de consultas en tiempo real, tasa de acierto de caché, uso de tokens, estimación de costos, estadísticas de ejecución de herramientas y consultas por modelo
- **Trazado Distribuido**: Trazado a nivel de solicitud para depurar latencia y fallos
- **API de Telemetría**: Endpoint REST (`/api/telemetry`) que expone datos completos de observabilidad

### Evaluación Batch y RL

- **Ejecutor Batch**: Ejecuta el agente en cientos de prompts en paralelo, produce trayectorias de entrenamiento en formato ShareGPT
- **Evaluación RL**: Ejecuta bucles de evaluación basados en recompensa con métricas configurables (exact_match, code_quality, length_penalty)
- **Exportación de Trayectorias**: Exporta datos de episodios con recompensas paso a paso para investigación en aprendizaje por refuerzo

### Compatibilidad con OpenAI

- **Formato API OpenAI**: Soporte completo para endpoints de API compatibles con OpenAI vía flag `--openai` o configuración
- **URL Base Personalizada**: Apunta a cualquier proveedor compatible con OpenAI con la flag `--url`
- **Multi-Proveedor**: Cambia entre backends de Anthropic y compatibles con OpenAI sin interrupciones

## Arquitectura

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

### Sistema de Memoria de 4 Capas

| Capa | Nombre | Almacenamiento | Comportamiento |
|-------|------|---------|----------|
| L1 | Memoria de Prompt | `MEMORY.md` + `USER.md` | Carga automática en cada sesión, límite estricto de 3,575 caracteres |
| L2 | Búsqueda de Sesión | SQLite + FTS5 | El agente busca historial relevante, resumido por LLM antes de inyección |
| L3 | Procedural de Habilidades | `~/.smartclaw/skills/` | Solo carga nombre y descripción, contenido completo bajo demanda |
| L4 | Modelado de Usuario | tabla `user_observations` | Rastrea preferencias pasivamente, auto-actualiza USER.md |

### Bucle de Aprendizaje

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

### Ejecución Especulativa

```
User Query
    ├── Fast Model (Haiku) → result in ~1s
    └── Heavy Model (Opus) → result in ~5s
            ↓
    Compare: similarity > 0.7?
        ↓ Yes              ↓ No
    Use fast result    Use heavy result
```

### Enrutamiento de Modelo Adaptativo

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

## Inicio Rápido

### Requisitos

- Go 1.25+
- Clave API de Anthropic (o clave API compatible con OpenAI)

### Instalación

```bash
go build -o bin/smartclaw ./cmd/smartclaw/
```

### Uso Básico

```bash
# Iniciar modo TUI (recomendado)
./bin/smartclaw tui

# Iniciar REPL simple
./bin/smartclaw repl

# Enviar un prompt único
./bin/smartclaw prompt "Explain this code"

# Usar un modelo específico
./bin/smartclaw --model claude-opus-4-6 repl

# Iniciar servidor WebUI
./bin/smartclaw web --port 8080

# Iniciar servidor ACP para integración IDE
./bin/smartclaw acp

# Iniciar gateway multiplataforma
./bin/smartclaw gateway --adapters telegram,web --telegram-token <BOT_TOKEN>

# Ejecutar evaluación batch
./bin/smartclaw batch --prompts prompts.jsonl --output trajectories/

# Ejecutar bucle de evaluación RL
./bin/smartclaw rl-eval --tasks tasks.jsonl --metric code_quality --output rl-output/

# Usar API compatible con OpenAI
./bin/smartclaw --openai --url https://api.your-provider.com/v1 repl
```

### Configuración

Configura tu clave API de Anthropic:

```bash
export ANTHROPIC_API_KEY=your_key_here
```

O crea `~/.smartclaw/config.yaml`:

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

### Directorio de Datos

SmartClaw crea y gestiona automáticamente lo siguiente bajo `~/.smartclaw/`:

| Ruta | Descripción |
|------|-------------|
| `MEMORY.md` | Memoria del sistema, auto-actualizada por el bucle de aprendizaje |
| `USER.md` | Perfil de usuario, auto-evolucionado a partir de observaciones |
| `state.db` | Base de datos SQLite con índice FTS5 |
| `skills/` | Habilidades aprendidas e integradas |
| `cron/` | Definiciones de tareas programadas (JSON) |
| `recordings/` | Grabaciones de sesiones (JSONL) |
| `mcp/servers.json` | Configuraciones de servidores MCP |
| `exports/` | Sesiones exportadas |
| `outbox/` | Mensajes multiplataforma en cola |

`MEMORY.md` y `USER.md` se pueden editar directamente. SmartClaw los recargará en el próximo uso.

## Herramientas Disponibles (73+)

### Operaciones de Archivos

| Herramienta | Descripción |
|------|-------------|
| `bash` | Ejecutar comandos shell con soporte de timeout y segundo plano |
| `read_file` | Leer contenido de archivos |
| `write_file` | Escribir archivos |
| `edit_file` | Edición por reemplazo de cadenas |
| `glob` | Coincidencia de patrones de archivos |
| `grep` | Búsqueda de contenido con soporte de regex |
| `powershell` | Ejecutar comandos PowerShell (Windows) |

### Análisis de Código

| Herramienta | Descripción |
|------|-------------|
| `lsp` | Operaciones LSP (goto_definition, find_references, rename, diagnostics) |
| `ast_grep` | Búsqueda y reemplazo de patrones AST |
| `code_search` | Búsqueda semántica de código |
| `index` | Indexación de código para búsqueda |

### Web y Navegador

| Herramienta | Descripción |
|------|-------------|
| `web_fetch` | Obtener URLs y convertir a markdown |
| `web_search` | Búsqueda web |
| `browser_navigate` | Navegar a URL en navegador headless |
| `browser_click` | Hacer clic en elemento por selector CSS |
| `browser_type` | Escribir texto en elemento |
| `browser_screenshot` | Capturar pantalla de la página |
| `browser_extract` | Extraer contenido/texto de la página |
| `browser_wait` | Esperar elemento o condición |
| `browser_select` | Seleccionar opción en menú desplegable |
| `browser_fill_form` | Rellenar múltiples campos de formulario |

### Integración MCP

| Herramienta | Descripción |
|------|-------------|
| `mcp` | Ejecutar herramientas en servidores MCP conectados (transporte SSE/stdio) |
| `list_mcp_resources` | Listar recursos disponibles en un servidor MCP |
| `read_mcp_resource` | Leer un recurso de un servidor MCP conectado |
| `mcp_auth` | Autenticarse con servidores MCP vía flujo OAuth |

### Agente y Aprendizaje

| Herramienta | Descripción |
|------|-------------|
| `agent` | Generar sub-agentes para tareas paralelas |
| `skill` | Cargar y gestionar habilidades |
| `session` | Gestión de sesiones |
| `todowrite` | Gestión de lista de tareas con recordatorio de verificación |
| `config` | Gestión de configuración |
| `memory` | Consulta y gestión de memoria de 4 capas (recall, search, store, layers, stats) |

### Ejecución de Código y Sandboxing

| Herramienta | Descripción |
|------|-------------|
| `execute_code` | Ejecutar código Python en sandbox RPC con acceso a herramientas. Colapsa multi-turno en un solo turno |
| `docker_exec` | Ejecutar comandos en contenedores Docker aislados (un solo uso o persistentes por sesión) |
| `repl` | Evaluar expresiones en JavaScript (Node.js) o Python con timeout en sandbox |

### Operaciones Git

| Herramienta | Descripción |
|------|-------------|
| `git_ai` | Mensajes de commit, revisión de código y descripciones de PR con IA |
| `git_status` | Estado de Git para el directorio de trabajo |
| `git_diff` | Git diff (con o sin stage) |
| `git_log` | Registro reciente de commits de Git |

### Batch y Paralelo

| Herramienta | Descripción |
|------|-------------|
| `batch` | Ejecutar múltiples llamadas de herramientas en batch |
| `parallel` | Ejecutar múltiples llamadas de herramientas en paralelo |
| `pipeline` | Encadenar llamadas de herramientas con pipe de salida |

### Colaboración en Equipo

| Herramienta | Descripción |
|------|-------------|
| `team_create` | Crear un espacio de trabajo de equipo para compartir memoria |
| `team_delete` | Eliminar un espacio de trabajo de equipo |
| `team_share_memory` | Compartir un item de memoria con el equipo |
| `team_get_memories` | Obtener memorias compartidas del equipo |
| `team_search_memories` | Buscar en todas las memorias del equipo |
| `team_sync` | Sincronizar estado del equipo entre miembros |
| `team_share_session` | Compartir una sesión con el equipo |

### Remoto y Mensajería

| Herramienta | Descripción |
|------|-------------|
| `remote_trigger` | Ejecutar comandos en hosts remotos vía SSH |
| `send_message` | Enviar mensajes a canales/usuarios entre plataformas (telegram, web, terminal) |

### Flujo de Trabajo y Planificación

| Herramienta | Descripción |
|------|-------------|
| `enter_worktree` | Crear un worktree de git para desarrollo paralelo |
| `exit_worktree` | Eliminar un worktree de git y limpiar |
| `enter_plan_mode` | Entrar en modo de planificación estructurada |
| `exit_plan_mode` | Salir del modo de planificación y reanudar ejecución |
| `schedule_cron` | Programar, listar y eliminar tareas cron |

### Multimedia y Documentos

| Herramienta | Descripción |
|------|-------------|
| `image` | Analizar y procesar imágenes |
| `pdf` | Extraer texto de documentos PDF |
| `audio` | Procesar y transcribir archivos de audio |

### Herramientas Cognitivas

| Herramienta | Descripción |
|------|-------------|
| `think` | Paso de pensamiento estructurado antes de actuar |
| `deep_think` | Razonamiento extendido para problemas complejos |
| `brief` | Resumen conciso de temas |
| `observe` | Modo de observación para análisis pasivo |
| `lazy` | Carga diferida de herramientas bajo demanda |
| `fork` | Bifurcar la sesión actual para exploración paralela |

### Utilidades

| Herramienta | Descripción |
|------|-------------|
| `tool_search` | Buscar herramientas disponibles por palabra clave |
| `cache` | Gestionar caché de resultados de herramientas |
| `attach` | Conectarse a un proceso en ejecución |
| `debug` | Activar/desactivar modo depuración |
| `env` | Mostrar variables de entorno |
| `sleep` | Dormir durante la duración especificada |

## Comandos Slash (101)

### Core

| Comando | Descripción |
|---------|-------------|
| `/help` | Mostrar comandos disponibles |
| `/status` | Estado de la sesión |
| `/exit` | Salir del REPL |
| `/clear` | Limpiar sesión |
| `/version` | Mostrar versión |

### Modelo y Config

| Comando | Descripción |
|---------|-------------|
| `/model [name]` | Mostrar o establecer modelo |
| `/model-list` | Listar modelos disponibles |
| `/config` | Mostrar configuración |
| `/config-show` | Mostrar configuración completa |
| `/config-set` | Establecer valor de configuración |
| `/config-get` | Obtener valor de configuración |
| `/config-reset` | Restablecer configuración |
| `/config-export` | Exportar configuración |
| `/config-import` | Importar configuración |
| `/set-api-key <key>` | Establecer clave API |
| `/env` | Mostrar entorno |

### Sesión

| Comando | Descripción |
|---------|-------------|
| `/session` | Listar sesiones |
| `/resume` | Reanudar una sesión |
| `/save` | Guardar sesión actual |
| `/export` | Exportar sesión (markdown/json) |
| `/import` | Importar sesión |
| `/rename` | Renombrar sesión |
| `/fork` | Bifurcar sesión para exploración paralela |
| `/rewind` | Rebobinar estado de sesión |
| `/share` | Compartir sesión |
| `/summary` | Resumen de sesión |
| `/attach` | Conectarse a proceso |

### Compactación

| Comando | Descripción |
|---------|-------------|
| `/compact` | Mostrar uso de contexto |
| `/compact now` | Compactar historial de conversación manualmente |
| `/compact auto` | Activar/desactivar auto-compactación |
| `/compact status` | Mostrar estadísticas de compactación |
| `/compact config` | Mostrar configuración de compactación |

### Sistema de Agentes

| Comando | Descripción |
|---------|-------------|
| `/agent` | Gestionar agentes de IA |
| `/agent-list` | Listar agentes disponibles |
| `/agent-switch` | Cambiar a un agente |
| `/agent-create` | Crear agente personalizado |
| `/agent-delete` | Eliminar agente personalizado |
| `/agent-info` | Mostrar información del agente |
| `/agent-export` | Exportar configuración del agente |
| `/agent-import` | Importar configuración del agente |
| `/subagent` | Generar subagente |
| `/agents` | Listar agentes disponibles |

### Sistema de Plantillas

| Comando | Descripción |
|---------|-------------|
| `/template` | Gestionar plantillas de prompt |
| `/template-list` | Listar plantillas |
| `/template-use` | Usar una plantilla |
| `/template-create` | Crear plantilla |
| `/template-delete` | Eliminar plantilla |
| `/template-info` | Mostrar información de plantilla |
| `/template-export` | Exportar plantilla |
| `/template-import` | Importar plantilla |

### Memoria y Aprendizaje

| Comando | Descripción |
|---------|-------------|
| `/memory` | Mostrar contexto de memoria |
| `/skills` | Listar habilidades disponibles |
| `/observe` | Modo observación |

### MCP

| Comando | Descripción |
|---------|-------------|
| `/mcp` | Gestionar servidores MCP |
| `/mcp-add` | Añadir servidor MCP |
| `/mcp-remove` | Eliminar servidor MCP |
| `/mcp-list` | Listar servidores MCP |
| `/mcp-start` | Iniciar servidor MCP |
| `/mcp-stop` | Detener servidor MCP |

### Git

| Comando | Descripción |
|---------|-------------|
| `/git-status` (`/gs`) | Mostrar estado de Git |
| `/git-diff` (`/gd`) | Mostrar diff de Git |
| `/git-commit` (`/gc`) | Hacer commit de cambios |
| `/git-branch` (`/gb`) | Listar ramas |
| `/git-log` (`/gl`) | Mostrar log de Git |
| `/diff` | Mostrar diff |
| `/commit` | Atajo de commit de Git |

### Herramientas y Desarrollo

| Comando | Descripción |
|---------|-------------|
| `/tools` | Listar herramientas disponibles |
| `/tasks` | Listar o gestionar tareas |
| `/lsp` | Operaciones LSP |
| `/read` | Leer archivo |
| `/write` | Escribir archivo |
| `/exec` | Ejecutar comando |
| `/browse` | Abrir navegador |
| `/web` | Operaciones web |
| `/ide` | Integración IDE |
| `/install` | Instalar paquete |

### Diagnóstico

| Comando | Descripción |
|---------|-------------|
| `/doctor` | Ejecutar diagnósticos |
| `/cost` | Mostrar uso de tokens y costo |
| `/stats` | Mostrar estadísticas de sesión |
| `/usage` | Estadísticas de uso |
| `/debug` | Activar/desactivar modo depuración |
| `/inspect` | Inspeccionar estado interno |
| `/cache` | Gestionar caché |
| `/heapdump` | Volcado de heap |
| `/reset-limits` | Restablecer límites de tasa |

### Planificación y Pensamiento

| Comando | Descripción |
|---------|-------------|
| `/plan` | Modo de planificación |
| `/think` | Modo de pensamiento |
| `/deepthink` | Pensamiento profundo |
| `/ultraplan` | Planificación ultra |
| `/thinkback` | Pensamiento retrospectivo |

### Colaboración y Comunicación

| Comando | Descripción |
|---------|-------------|
| `/invite` | Invitar a colaboración |
| `/feedback` | Enviar comentarios |
| `/issue` | Rastreador de issues |

### UI y Personalización

| Comando | Descripción |
|---------|-------------|
| `/theme` | Gestionar temas |
| `/color` | Tema de color |
| `/vim` | Modo Vim |
| `/keybindings` | Gestionar atajos de teclado |
| `/statusline` | Línea de estado |
| `/stickers` | Stickers |

### Cambio de Modo

| Comando | Descripción |
|---------|-------------|
| `/fast` | Modo rápido (usar modelo más ligero) |
| `/lazy` | Modo de carga diferida |
| `/desktop` | Modo escritorio |
| `/mobile` | Modo móvil |
| `/chrome` | Integración con Chrome |
| `/voice` | Control de modo de voz |

### Autenticación y Actualizaciones

| Comando | Descripción |
|---------|-------------|
| `/login` | Autenticarse con servicio |
| `/logout` | Limpiar autenticación |
| `/upgrade` | Actualizar versión del CLI |
| `/api` | Operaciones de API |

### Varios

| Comando | Descripción |
|---------|-------------|
| `/init` | Inicializar nuevo proyecto |
| `/context` | Gestionar contexto |
| `/permissions` | Gestionar permisos |
| `/hooks` | Gestionar hooks |
| `/plugin` | Gestionar plugins |
| `/passes` | Pases LSP |
| `/preview` | Previsualizar cambios |
| `/effort` | Seguimiento de esfuerzo |
| `/tag` | Gestión de etiquetas |
| `/copy` | Copiar al portapapeles |
| `/files` | Listar archivos |
| `/advisor` | Asesor de IA |
| `/btw` | Por cierto |
| `/bughunter` | Modo de caza de bugs |
| `/insights` | Insights de código |
| `/onboarding` | Onboarding |
| `/teleport` | Modo teletransporte |
| `/summary` | Resumen de sesión |

## Estructura del Proyecto

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

## Extensión VS Code

SmartClaw incluye una extensión de VS Code que se conecta vía ACP (Agent Communication Protocol):

### Comandos

| Comando | Descripción |
|---------|-------------|
| `SmartClaw: Ask` | Hacer una pregunta a SmartClaw |
| `SmartClaw: Open Chat` | Abrir la barra lateral de chat |
| `SmartClaw: Explain Code` | Explicar el código seleccionado |
| `SmartClaw: Fix Code` | Corregir problemas en el código seleccionado |
| `SmartClaw: Generate Tests` | Generar pruebas para el código seleccionado |

### Instalación

1. Compila SmartClaw: `go build -o bin/smartclaw ./cmd/smartclaw/`
2. Añade `smartclaw` a tu PATH
3. Instala la extensión desde `extensions/vscode/`
4. Abre la barra lateral de SmartClaw en el Explorador

## Uso de la API

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

## Variables de Entorno

| Variable | Descripción |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Clave API para Anthropic |
| `SMARTCLAW_MODEL` | Modelo predeterminado a usar |
| `SMARTCLAW_CONFIG` | Ruta al archivo de configuración |
| `SMARTCLAW_SESSION_DIR` | Directorio para almacenamiento de sesiones |
| `SMARTCLAW_LOG_LEVEL` | Nivel de log (debug, info, warn, error) |

## Pruebas

```bash
# Ejecutar todas las pruebas
go test ./...

# Ejecutar paquetes específicos
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

# Ejecutar con cobertura
go test -cover ./...
```

## Licencia

MIT License
