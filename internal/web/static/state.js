// SmartClaw - State Management
(function() {
  'use strict';

  const state = {
    messages: [], sessions: [], tools: [], agents: [], files: [],
    settings: { theme: 'dark', fontSize: 14, model: 'sre-model' },
    ui: { sidebarOpen: true, activeSection: 'files', currentFile: null, editorFile: null, currentSessionId: null },
    tokens: { used: 0, limit: 200000 },
    cost: 0,
    costByModel: {},
    lastCostBreakdown: null,
    ws: null,
    connected: false,
    currentToolId: null,
    tokenHistory: [],
    costHistory: [],
    agentHistory: [],
    isRecording: false,
    isProcessing: false,
    audioContext: null,
    analyser: null,
    mediaStream: null,
    animFrame: null,
    cmdIndex: -1,
    commandHistory: [],
    historyIndex: -1,
    flatFiles: [],
    mentionIndex: -1,
    mentionStart: -1,
    skills: [],
    memoryLayers: { memory: '', user: '' },
    memoryStats: { memory_chars: 0, user_chars: 0 },
    memoryTab: 'l1',
    userObservations: [],
    sessionFragments: [],
    editingSkill: null,
    wikiPages: [],
    wikiEnabled: false,
    uploads: [],
    pendingImages: [],
    gitStatus: {},
    fileTreeData: [],
    cronTasks: [],
    errorCount: 0,
    runningTools: 0,
    projectPath: '',
    projectName: 'Project',
    recentProjects: [],
    fileTabs: {},
  };

  const subscribers = {};
  function subscribe(key, fn) { (subscribers[key] = subscribers[key] || []).push(fn); }
  function emit(key, data) { (subscribers[key] || []).forEach(fn => fn(data)); }
  function setState(path, val) {
    const parts = path.split('.');
    let obj = state;
    for (let i = 0; i < parts.length - 1; i++) obj = obj[parts[i]];
    obj[parts[parts.length - 1]] = val;
    (subscribers[path] || []).forEach(fn => fn(val));
    (subscribers['*'] || []).forEach(fn => fn(path, val));
  }

  const commands = [
    { name: '/compact', desc: 'Compact context', shortcut: '' },
    { name: '/memory', desc: 'Memory management', shortcut: '' },
    { name: '/model', desc: 'Switch model', shortcut: 'Ctrl+P' },
    { name: '/session', desc: 'Session management', shortcut: 'Ctrl+O' },
    { name: '/voice', desc: 'Voice settings', shortcut: '' },
    { name: '/agent', desc: 'Agent management', shortcut: '' },
    { name: '/subagent', desc: 'Subagent tasks', shortcut: '' },
    { name: '/clear', desc: 'Clear chat', shortcut: 'Ctrl+L' },
    { name: '/help', desc: 'Show help', shortcut: 'Ctrl+H' },
    { name: '/schedule', desc: 'Manage cron tasks', shortcut: '' },
  ];

  const toolColors = {
    bash: '#f59e0b', read_file: '#3b82f6', write_file: '#10b981', edit_file: '#8b5cf6',
    glob: '#6366f1', grep: '#ec4899', lsp: '#14b8a6', ast_grep: '#f97316',
    browser_navigate: '#06b6d4', browser_click: '#06b6d4', browser_type: '#06b6d4',
    browser_screenshot: '#06b6d4', browser_extract: '#06b6d4', browser_wait: '#06b6d4',
    browser_select: '#06b6d4', browser_fill_form: '#06b6d4',
    sopa_list_nodes: '#f43f5e', sopa_get_node: '#f43f5e', sopa_node_logs: '#f43f5e',
    sopa_execute_task: '#f43f5e', sopa_execute_orchestration: '#f43f5e',
    sopa_list_faults: '#f43f5e', sopa_get_fault: '#f43f5e',
    sopa_list_audits: '#f43f5e', sopa_approve_audit: '#f43f5e', sopa_reject_audit: '#f43f5e',
    git_ai: '#f97316', git_status: '#f97316', git_diff: '#f97316', git_log: '#f97316',
    github_create_pr: '#a855f7', github_list_prs: '#a855f7', github_merge_pr: '#a855f7',
    github_create_issue: '#a855f7', github_list_issues: '#a855f7',
    docker_exec: '#0ea5e9', execute_code: '#0ea5e9',
    mcp: '#84cc16', list_mcp_resources: '#84cc16', read_mcp_resource: '#84cc16',
    investigate_incident: '#ef4444', incident_timeline: '#ef4444',
    audit_query: '#f59e0b', audit_stats: '#f59e0b',
    team_create: '#8b5cf6', team_share_memory: '#8b5cf6',
    worktree_create: '#14b8a6', worktree_list: '#14b8a6',
  };

  SC.state = state;
  SC.subscribe = subscribe;
  SC.emit = emit;
  SC.setState = setState;
  SC.commands = commands;
  SC.toolColors = toolColors;
})();
