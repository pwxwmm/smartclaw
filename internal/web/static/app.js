(function() {
  'use strict';

  const $ = (s, p) => (p || document).querySelector(s);
  const $$ = (s, p) => [...(p || document).querySelectorAll(s)];

  const state = {
    messages: [], sessions: [], tools: [], agents: [], files: [],
    settings: { theme: 'dark', fontSize: 14, model: 'glm-5' },
    ui: { sidebarOpen: true, activeSection: 'files', currentFile: null, editorFile: null, currentSessionId: null },
    tokens: { used: 0, limit: 200000 },
    cost: 0,
    ws: null,
    connected: false,
    currentToolId: null,
    tokenHistory: [],
    costHistory: [],
    agentHistory: [],
    isRecording: false,
    audioContext: null,
    analyser: null,
    mediaStream: null,
    animFrame: null,
    cmdIndex: -1,
    flatFiles: [],
    mentionIndex: -1,
    mentionStart: -1,
  };

  const subscribers = {};
  function subscribe(key, fn) { (subscribers[key] = subscribers[key] || []).push(fn); }
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
  ];

  function wsConnect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);

    ws.onopen = () => {
      setState('connected', true);
      $('#connection-status').dataset.status = 'on';
      toast('Connected', 'success');
      wsSend('file_tree', { path: '.' });
      wsSend('session_list', {});
    };

    ws.onclose = () => {
      setState('connected', false);
      $('#connection-status').dataset.status = 'off';
      setTimeout(wsConnect, 3000);
    };

    ws.onerror = () => { ws.close(); };

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        handleWSMessage(msg);
      } catch(err) { console.error('WS parse error:', err); }
    };

    state.ws = ws;
  }

  function wsSend(type, data) {
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type, ...data }));
    }
  }

  function handleWSMessage(msg) {
    switch(msg.type) {
      case 'token':
        appendToken(msg.content);
        break;
      case 'thinking':
        appendThinking(msg.content);
        break;
      case 'tool_start':
        addToolCard(msg);
        break;
      case 'tool_output':
        updateToolCard(msg);
        break;
      case 'tool_end':
        finishToolCard(msg);
        break;
      case 'agent_status':
        updateAgentCard(msg);
        break;
      case 'done':
        finishMessage(msg);
        break;
      case 'error':
        toast(msg.message, 'error');
        break;
      case 'file_tree':
        state.flatFiles = flattenFileTree(msg.tree || [], '');
        renderFileTree(msg.tree || []);
        break;
      case 'file_content':
        openFileDrawer(msg.content, state.ui.currentFile);
        break;
      case 'session_list':
        renderSessions(msg.sessions || []);
        break;
      case 'session_active':
        state.ui.currentSessionId = msg.id;
        break;
      case 'session_created':
        state.ui.currentSessionId = msg.id;
        $('#messages').innerHTML = '';
        state.messages = [];
        toast('Session created', 'success');
        wsSend('session_list', {});
        break;
      case 'session_loaded':
        loadSessionMessages(msg);
        break;
      case 'session_deleted':
        toast('Session deleted', 'success');
        if (state.ui.currentSessionId === msg.id) {
          state.ui.currentSessionId = null;
          $('#messages').innerHTML = '';
          state.messages = [];
        }
        wsSend('session_list', {});
        break;
      case 'model_changed':
        toast(msg.message, 'success');
        state.settings.model = msg.message.split(' ').pop();
        $('#current-model').textContent = state.settings.model;
        break;
      case 'voice_transcript':
        $('#input').value = msg.text;
        break;
    }
  }

  let currentAssistantEl = null;
  let currentContent = '';
  let currentThinking = '';
  let thinkingBlock = null;
  let renderRAF = null;
  let doneTimeout = null;

  function forceFinishIfStale() {
    if (currentAssistantEl && currentContent) {
      finishMessage({ tokens: 0, cost: 0 });
    }
  }

  function sendMessage() {
    const input = $('#input');
    const mention = $('#file-mention');
    if (mention && !mention.classList.contains('hidden')) return;
    const text = input.value.trim();
    if (!text) return;

    addMessage('user', text);
    input.value = '';
    input.style.height = 'auto';

    if (text.startsWith('/')) {
      const parts = text.split(' ');
      wsSend('cmd', { name: parts[0], args: parts.slice(1) });
      return;
    }

    currentContent = '';
    currentAssistantEl = addMessage('assistant', '');
    if (doneTimeout) clearTimeout(doneTimeout);
    doneTimeout = setTimeout(forceFinishIfStale, 30000);
    const bubble = currentAssistantEl.querySelector('.msg-bubble');
    bubble.innerHTML = '<div class="thinking"><div class="think-eyes"><svg width="32" height="16" viewBox="0 0 32 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle cx="24" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle class="pupil-l" cx="8" cy="8" r="2" fill="currentColor"/><circle class="pupil-r" cx="24" cy="8" r="2" fill="currentColor"/></svg></div><span class="think-label">Thinking<span class="think-dots"><span></span><span></span><span></span></span></span></div>';
    wsSend('chat', { content: text });
  }

  function addMessage(role, content) {
    const el = document.createElement('div');
    el.className = `message ${role}`;
    const roleLabel = role === 'user' ? 'You' : 'SmartClaw';
    el.innerHTML = `<div class="msg-role">${roleLabel}</div><div class="msg-bubble">${escapeHtml(content)}</div>`;
    $('#messages').appendChild(el);
    scrollChat();
    state.messages.push({ role, content, ts: Date.now() });
    return el;
  }

  function appendToken(token) {
    if (!currentAssistantEl) return;
    currentContent += token;
    if (renderRAF) return;
    renderRAF = requestAnimationFrame(() => {
      renderRAF = null;
      if (!currentAssistantEl) return;
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.remove();
      bubble.innerHTML = renderPlainText(currentContent);
      scrollChat();
    });
  }

  function appendThinking(token) {
    if (!currentAssistantEl) return;
    const bubble = currentAssistantEl.querySelector('.msg-bubble');
    currentThinking += token;
    if (!thinkingBlock) {
      thinkingBlock = document.createElement('details');
      thinkingBlock.className = 'thinking-block';
      thinkingBlock.open = true;
      thinkingBlock.innerHTML = '<summary>Thinking...</summary><div class="thinking-content"></div>';
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.replaceWith(thinkingBlock);
      else bubble.prepend(thinkingBlock);
    }
    const content = thinkingBlock.querySelector('.thinking-content');
    content.textContent = currentThinking;
    thinkingBlock.querySelector('summary').textContent = 'Thinking...';
    scrollChat();
  }

  function finishMessage(msg) {
    if (renderRAF) { cancelAnimationFrame(renderRAF); renderRAF = null; }
    if (currentAssistantEl && currentContent) {
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      if (bubble) {
        const thinking = bubble.querySelector('.thinking');
        if (thinking) thinking.remove();
        try {
          bubble.innerHTML = renderMarkdown(currentContent);
          bubble.classList.add('rendered');
        } catch (e) {
          console.error('renderMarkdown error:', e);
          bubble.innerHTML = renderPlainText(currentContent);
        }
        bindCodeCopy(bubble);
      }
    }
    if (msg.tokens) {
      state.tokens.used = msg.tokens;
      state.cost += msg.cost || 0;
      updateStats();
      state.tokenHistory.push({ t: Date.now(), v: msg.tokens });
      state.costHistory.push({ t: Date.now(), v: state.cost });
    }
    if (thinkingBlock) {
      thinkingBlock.open = false;
      thinkingBlock.querySelector('summary').textContent = 'Thought process (' + currentThinking.length + ' chars)';
      thinkingBlock = null;
    }
    if (doneTimeout) { clearTimeout(doneTimeout); doneTimeout = null; }
    currentAssistantEl = null;
    currentContent = '';
    currentThinking = '';
  }

  function addToolCard(msg) {
    state.currentToolId = msg.id || Date.now().toString();
    const el = document.createElement('div');
    el.className = 'tool-card';
    el.id = `tool-${state.currentToolId}`;
    el.innerHTML = `<div class="tool-head"><span class="tool-dot running"></span><span>${msg.tool}</span></div><div class="tool-body"></div>`;
    $('#messages').appendChild(el);
    scrollChat();
  }

  function updateToolCard(msg) {
    const body = $(`#tool-${msg.id || state.currentToolId} .tool-body`);
    if (body) body.textContent += msg.output || '';
    scrollChat();
  }

  function finishToolCard(msg) {
    const card = $(`#tool-${msg.id || state.currentToolId}`);
    if (card) {
      const dot = card.querySelector('.tool-dot');
      dot.classList.remove('running');
      dot.classList.add('ok');
      const head = card.querySelector('.tool-head span:last-child');
      head.textContent += ` (${msg.duration || 0}ms)`;
    }
  }

  function updateAgentCard(msg) {
    let card = $(`#agent-${msg.id}`);
    if (!card) {
      card = document.createElement('div');
      card.className = 'agent-card';
      card.id = `agent-${msg.id}`;
      $('#agent-list').appendChild(card);
    }
    const pct = Math.round((msg.progress || 0) * 100);
    card.innerHTML = `
      <div class="agent-head"><span class="agent-name">${msg.id?.slice(0,8) || 'Agent'}</span><span class="agent-status">${msg.status || 'running'}</span></div>
      <div class="prog-bar"><div class="prog-fill" style="width:${pct}%"></div></div>
    `;
  }

  let dirCounter = 0;

  function flattenFileTree(nodes, prefix) {
    let result = [];
    nodes.forEach(node => {
      const path = prefix ? prefix + '/' + node.name : node.name;
      if (node.type === 'dir') {
        if (node.children) result = result.concat(flattenFileTree(node.children, path));
      } else {
        result.push({ name: node.name, path: path });
      }
    });
    return result;
  }

  function renderFileTree(nodes, parent) {
    const container = parent || $('#file-tree');
    container.innerHTML = '';
    nodes.forEach(node => {
      const el = document.createElement('div');
      el.className = `file-node ${node.type === 'dir' ? 'dir' : ''}`;
      const iconSvg = node.type === 'dir'
        ? '<svg class="ficon folder" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>'
        : '<svg class="ficon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
      el.innerHTML = `${iconSvg}<span class="fname">${node.name}</span>`;
      if (node.type === 'dir') {
        const dirId = 'dir-' + (++dirCounter);
        el.dataset.dirId = dirId;
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          const children = container.querySelector(`[data-dir-children="${dirId}"]`);
          if (!children) return;
          const collapsed = el.dataset.collapsed === 'true';
          el.dataset.collapsed = collapsed ? 'false' : 'true';
          children.style.display = collapsed ? '' : 'none';
          el.querySelector('.folder').style.transform = collapsed ? '' : 'rotate(-90deg)';
        });
        container.appendChild(el);
        if (node.children && node.children.length > 0) {
          const childContainer = document.createElement('div');
          childContainer.className = 'file-children';
          childContainer.dataset.dirChildren = dirId;
          renderFileTree(node.children, childContainer);
          container.appendChild(childContainer);
        }
      } else {
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          state.ui.currentFile = getNodePath(el);
          wsSend('file_open', { path: state.ui.currentFile });
        });
        el.draggable = true;
        el.addEventListener('dragstart', (e) => {
          e.dataTransfer.setData('text/plain', state.ui.currentFile);
        });
        container.appendChild(el);
      }
    });
  }

  function getNodePath(el) {
    const parts = [];
    let node = el;
    while (node && node.id !== 'file-tree') {
      if (node.classList.contains('file-node')) {
        const name = node.querySelector('.fname')?.textContent;
        if (name) parts.unshift(name);
      } else if (node.classList.contains('file-children')) {
        const dirNode = node.previousElementSibling;
        if (dirNode && dirNode.classList.contains('file-node')) {
          const name = dirNode.querySelector('.fname')?.textContent;
          if (name) parts.unshift(name);
        }
      }
      node = node.parentElement;
    }
    return parts.join('/');
  }

  function renderSessions(sessions) {
    const list = $('#session-list');
    list.innerHTML = '';
    sessions.forEach(s => {
      const el = document.createElement('div');
      el.className = 'session-item' + (s.id === state.ui.currentSessionId ? ' active' : '');
      el.innerHTML = `<div class="stitle">${s.title || 'Untitled'}</div><div class="smeta">${s.model} / ${s.messageCount} msgs / ${new Date(s.updatedAt).toLocaleDateString()}</div><button class="session-del" title="Delete session">&times;</button>`;
      el.querySelector('.session-del').addEventListener('click', (e) => {
        e.stopPropagation();
        if (confirm('Delete this session?')) wsSend('session_delete', { id: s.id });
      });
      el.addEventListener('click', () => wsSend('session_load', { id: s.id }));
      list.appendChild(el);
    });
    setState('sessions', sessions);
    $('#s-total-sessions').textContent = sessions.length;
  }

  function loadSessionMessages(msg) {
    state.ui.currentSessionId = msg.id;
    const container = $('#messages');
    container.innerHTML = '';
    state.messages = [];
    const messages = msg.messages || [];
    messages.forEach(m => {
      const el = document.createElement('div');
      el.className = `message ${m.role}`;
      const roleLabel = m.role === 'user' ? 'You' : 'SmartClaw';
      el.innerHTML = `<div class="msg-role">${roleLabel}</div><div class="msg-bubble">${m.role === 'assistant' ? renderMarkdown(m.content) : escapeHtml(m.content)}</div>`;
      container.appendChild(el);
      if (m.role === 'assistant') {
        bindCodeCopy(el.querySelector('.msg-bubble'));
      }
      state.messages.push({ role: m.role, content: m.content, ts: Date.now() });
    });
    scrollChat();
    wsSend('session_list', {});
  }

  const extToLang = {
    go: 'go', js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
    ts: 'typescript', tsx: 'typescript', py: 'python', pyw: 'python',
    sh: 'bash', bash: 'bash', zsh: 'bash', json: 'json', yaml: 'yaml', yml: 'yaml',
    html: 'html', htm: 'html', css: 'css', scss: 'css', less: 'css',
    md: 'markdown', sql: 'sql', java: 'java', rs: 'rust', rb: 'ruby',
    c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
    xml: 'xml', toml: 'toml', ini: 'ini', cfg: 'ini',
    dockerfile: 'dockerfile', makefile: 'makefile',
  };

  function langFromPath(path) {
    if (!path) return null;
    const lower = path.toLowerCase();
    const basename = lower.split('/').pop();
    if (basename === 'dockerfile' || basename === 'dockerfile.') return 'dockerfile';
    if (basename === 'makefile' || basename === 'gnumakefile') return 'makefile';
    if (basename === '.gitignore' || basename === '.dockerignore') return 'bash';
    const ext = basename.split('.').pop();
    return extToLang[ext] || null;
  }

  function openFileDrawer(content, path) {
    const drawer = $('#file-drawer');
    $('#drawer-title').textContent = path || 'File Preview';
    const lines = content.split('\n');
    const lineCount = lines.length;
    const padWidth = String(lineCount).length;
    const lineNums = lines.map((_, i) => String(i + 1).padStart(padWidth, ' ')).join('\n');
    const container = $('#drawer-content');
    container.innerHTML = '';
    const lineNumsEl = document.createElement('span');
    lineNumsEl.className = 'line-nums';
    lineNumsEl.textContent = lineNums;
    const codeEl = document.createElement('code');
    const lang = langFromPath(path);
    if (lang) {
      codeEl.innerHTML = simpleHighlight(content, lang);
    } else {
      codeEl.textContent = content;
    }
    container.appendChild(lineNumsEl);
    container.appendChild(codeEl);
    drawer.classList.remove('hidden');
    requestAnimationFrame(() => drawer.classList.add('visible'));
  }

  function closeDrawer() {
    const drawer = $('#file-drawer');
    drawer.classList.remove('visible');
    setTimeout(() => drawer.classList.add('hidden'), 340);
  }

  function openEditor(content, path) {
    state.ui.editorFile = path;
    $('#editor-title').textContent = path || 'New File';
    $('#editor-content').value = content || '';
    const panel = $('#editor-panel');
    panel.classList.remove('hidden');
    requestAnimationFrame(() => panel.classList.add('visible'));
    $('#editor-content').focus();
  }

  function closeEditor() {
    const panel = $('#editor-panel');
    panel.classList.remove('visible');
    setTimeout(() => panel.classList.add('hidden'), 220);
  }

  function saveEditor() {
    if (!state.ui.editorFile) return;
    wsSend('file_save', { path: state.ui.editorFile, content: $('#editor-content').value });
    toast('File saved', 'success');
  }

  function openDashboard() {
    const panel = $('#dashboard-panel');
    panel.classList.remove('hidden');
    requestAnimationFrame(() => panel.classList.add('visible'));
    drawCharts();
  }

  function drawCharts() {
    drawTokenChart();
    drawCostChart();
    drawAgentChart();
    $('#s-total-msgs').textContent = state.messages.length;
    $('#s-total-tokens').textContent = state.tokens.used.toLocaleString();
    $('#s-total-cost').textContent = `$${state.cost.toFixed(2)}`;
  }

  function drawTokenChart() {
    const canvas = $('#chart-tokens');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    const data = state.tokenHistory.slice(-20);
    if (data.length < 2) {     ctx.fillStyle = getCSS('--tx-2'); ctx.font = '12px Inter'; ctx.fillText('Waiting for data...', w/2-50, h/2); return; }
    const max = Math.max(...data.map(d => d.v), 1);
    ctx.strokeStyle = getCSS('--accent');
    ctx.lineWidth = 2;
    ctx.beginPath();
    data.forEach((d, i) => {
      const x = (i / (data.length - 1)) * (w - 40) + 20;
      const y = h - 20 - ((d.v / max) * (h - 40));
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
    });
    ctx.stroke();
    ctx.lineTo((w - 20), h - 20);
    ctx.lineTo(20, h - 20);
    ctx.closePath();
    ctx.fillStyle = getCSS('--accent-bg');
    ctx.fill();
  }

  function drawCostChart() {
    const canvas = $('#chart-cost');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    const segments = [
      { label: 'Input', value: state.cost * 0.3, color: getCSS('--info') },
      { label: 'Output', value: state.cost * 0.6, color: getCSS('--accent') },
      { label: 'Tools', value: state.cost * 0.1, color: getCSS('--ok') },
    ];
    const total = segments.reduce((s, d) => s + d.value, 0) || 1;
    const cx = w / 2, cy = h / 2, r = Math.min(w, h) / 2 - 30;
    let start = -Math.PI / 2;
    segments.forEach(seg => {
      const angle = (seg.value / total) * Math.PI * 2;
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.arc(cx, cy, r, start, start + angle);
      ctx.closePath();
      ctx.fillStyle = seg.color;
      ctx.fill();
      start += angle;
    });
    ctx.beginPath();
    ctx.arc(cx, cy, r * 0.5, 0, Math.PI * 2);
    ctx.fillStyle = getCSS('--bg-0');
    ctx.fill();
    ctx.fillStyle = getCSS('--tx-0');
    ctx.font = 'bold 14px IBM Plex Mono';
    ctx.textAlign = 'center';
    ctx.fillText(`$${state.cost.toFixed(2)}`, cx, cy + 5);
  }

  function drawAgentChart() {
    const canvas = $('#chart-agents');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    const data = state.agentHistory.slice(-20);
    if (data.length < 2) {     ctx.fillStyle = getCSS('--tx-2'); ctx.font = '12px Inter'; ctx.fillText('No agent activity yet', w/2-70, h/2); return; }
    const barW = (w - 40) / data.length;
    const max = Math.max(...data.map(d => d.v), 1);
    data.forEach((d, i) => {
      const barH = (d.v / max) * (h - 40);
      ctx.fillStyle = d.color || getCSS('--accent');
      ctx.fillRect(20 + i * barW + 2, h - 20 - barH, barW - 4, barH);
    });
  }

  function initThemeEditor() {
    const vars = ['--bg-0','--bg-1','--bg-2','--bg-3','--tx-0','--tx-1','--accent','--accent-h','--bd','--ok','--warn','--err'];
    const container = $('#theme-colors');
    container.innerHTML = '';
    vars.forEach(v => {
      const item = document.createElement('div');
      item.className = 'color-item';
      const current = getCSS(v);
      item.innerHTML = `<label>${v}</label><input type="color" value="${rgbToHex(current)}">`;
      item.querySelector('input').addEventListener('input', (e) => {
        document.documentElement.style.setProperty(v, e.target.value);
      });
      container.appendChild(item);
    });
  }

  function exportTheme() {
    const theme = {};
    document.documentElement.style.cssText.split(';').forEach(rule => {
      const [k, v] = rule.split(':').map(s => s.trim());
      if (k && v) theme[k.replace('--', '')] = v;
    });
    const blob = new Blob([JSON.stringify(theme, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'smartclaw-theme.json';
    a.click();
  }

  function importTheme() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = (e) => {
      const file = e.target.files[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = (ev) => {
        try {
          const theme = JSON.parse(ev.target.result);
          Object.entries(theme).forEach(([k, v]) => {
            document.documentElement.style.setProperty(`--${k}`, v);
          });
          toast('Theme imported', 'success');
        } catch { toast('Invalid theme file', 'error'); }
      };
      reader.readAsText(file);
    };
    input.click();
  }

  let recognition = null;
  let sttFailed = false;

  function startVoice() {
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;

    setState('isRecording', true);
    sttFailed = false;
    $('#voice-bar').classList.remove('hidden');

    navigator.mediaDevices.getUserMedia({ audio: true }).then(stream => {
      state.mediaStream = stream;
      state.audioContext = new (window.AudioContext || window.webkitAudioContext)();
      const source = state.audioContext.createMediaStreamSource(stream);
      state.analyser = state.audioContext.createAnalyser();
      state.analyser.fftSize = 2048;
      source.connect(state.analyser);
      drawWaveform();

      if (SpeechRecognition) {
        recognition = new SpeechRecognition();
        recognition.continuous = true;
        recognition.interimResults = true;
        recognition.lang = navigator.language || 'en-US';

        let finalTranscript = '';
        recognition.onresult = (e) => {
          let interim = '';
          for (let i = e.resultIndex; i < e.results.length; i++) {
            if (e.results[i].isFinal) {
              finalTranscript += e.results[i][0].transcript;
            } else {
              interim += e.results[i][0].transcript;
            }
          }
          $('#voice-status').textContent = finalTranscript + interim || 'Listening...';
        };

        recognition.onerror = () => {
          sttFailed = true;
          $('#voice-status').textContent = 'Recording... (STT unavailable)';
        };

        recognition.onend = () => {
          if (sttFailed) return;
          if (!state.isRecording) return;
          const text = finalTranscript.trim();
          if (text) {
            const input = $('#input');
            input.value += (input.value ? ' ' : '') + text;
            input.style.height = 'auto';
            input.style.height = Math.min(input.scrollHeight, 200) + 'px';
          }
          stopVoice();
        };

        try { recognition.start(); } catch(e) {}
        $('#voice-status').textContent = 'Listening...';
      } else {
        $('#voice-status').textContent = 'Recording... (no STT support)';
      }
    }).catch(() => {
      toast('Microphone access denied', 'error');
      stopVoice();
    });
  }

  function stopVoice() {
    if (recognition) { try { recognition.stop(); } catch(e) {} recognition = null; }
    if (state.mediaStream) state.mediaStream.getTracks().forEach(t => t.stop());
    if (state.audioContext) { try { state.audioContext.close(); } catch(e) {} }
    if (state.animFrame) cancelAnimationFrame(state.animFrame);
    setState('isRecording', false);
    $('#voice-bar').classList.add('hidden');
  }

  function drawWaveform() {
    if (!state.isRecording || !state.analyser) return;
    const canvas = $('#waveform');
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    const data = new Uint8Array(state.analyser.frequencyBinCount);
    state.analyser.getByteTimeDomainData(data);
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    ctx.lineWidth = 2;
    ctx.strokeStyle = getCSS('--accent');
    ctx.beginPath();
    const sliceW = w / data.length;
    let x = 0;
    for (let i = 0; i < data.length; i++) {
      const v = data[i] / 128.0;
      const y = (v * h) / 2;
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
      x += sliceW;
    }
    ctx.lineTo(w, h / 2);
    ctx.stroke();
    state.animFrame = requestAnimationFrame(drawWaveform);
  }

  function initDragDrop() {
    const chat = $('#chat');
    const overlay = $('#drag-overlay');
    let dragCount = 0;

    chat.addEventListener('dragenter', (e) => { e.preventDefault(); dragCount++; overlay.classList.remove('hidden'); });
    chat.addEventListener('dragleave', (e) => { e.preventDefault(); dragCount--; if (dragCount <= 0) { overlay.classList.add('hidden'); dragCount = 0; } });
    chat.addEventListener('dragover', (e) => e.preventDefault());
    chat.addEventListener('drop', (e) => {
      e.preventDefault();
      dragCount = 0;
      overlay.classList.add('hidden');
      const files = e.dataTransfer.files;
      if (files.length > 0) {
        Array.from(files).forEach(f => {
          const reader = new FileReader();
          reader.onload = (ev) => {
            addMessage('user', `Attached: ${f.name} (${(f.size / 1024).toFixed(1)}KB)`);
          };
          reader.readAsArrayBuffer(f);
        });
      }
    });
  }

  function initCmdPalette() {
    const input = $('#input');
    const palette = $('#cmd-palette');
    const cmdInput = $('#cmd-input');
    const cmdList = $('#cmd-list');

    input.addEventListener('input', () => {
      if (input.value.startsWith('/')) {
        palette.classList.remove('hidden');
        const query = input.value.slice(1).toLowerCase();
        const filtered = commands.filter(c => c.name.includes(query) || c.desc.toLowerCase().includes(query));
        renderCmdList(filtered);
      } else {
        palette.classList.add('hidden');
      }
    });

  function renderCmdList(items) {
    cmdList.innerHTML = '';
    state.cmdIndex = -1;
    items.forEach((cmd, i) => {
      const li = document.createElement('li');
      li.className = 'cmd-item';
      li.innerHTML = `<span>${cmd.name}</span><span class="cdesc">${cmd.desc}</span>`;
      li.addEventListener('click', () => { input.value = cmd.name + ' '; palette.classList.add('hidden'); input.focus(); });
      cmdList.appendChild(li);
    });
  }

    input.addEventListener('keydown', (e) => {
      if (!palette.classList.contains('hidden')) {
        const items = $$('.cmd-item', cmdList);
        if (e.key === 'ArrowDown') { e.preventDefault(); state.cmdIndex = Math.min(state.cmdIndex + 1, items.length - 1); updateCmdSelection(items); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); state.cmdIndex = Math.max(state.cmdIndex - 1, 0); updateCmdSelection(items); }
        else if (e.key === 'Enter' && state.cmdIndex >= 0) { e.preventDefault(); items[state.cmdIndex]?.click(); }
        else if (e.key === 'Escape') { palette.classList.add('hidden'); }
      }
    });
  }

  function updateCmdSelection(items) {
    items.forEach((el, i) => el.classList.toggle('sel', i === state.cmdIndex));
  }

  function scrollChat() {
    const chat = $('#chat');
    const isNearBottom = chat.scrollHeight - chat.scrollTop - chat.clientHeight < 100;
    if (isNearBottom) chat.scrollTop = chat.scrollHeight;
  }

  function updateStats() {
    const pct = Math.min(state.tokens.used / state.tokens.limit * 100, 100);
    $('#token-fill').style.width = pct + '%';
    $('#stat-tokens').textContent = `${(state.tokens.used / 1000).toFixed(1)}k / ${(state.tokens.limit / 1000)}k tokens`;
    $('#stat-cost').textContent = `$${state.cost.toFixed(2)}`;
    $('#stat-agents').textContent = `${state.agents.length} agents`;
  }

  function toast(msg, type = 'info') {
    const el = document.createElement('div');
    el.className = `toast ${type === 'success' ? 'ok' : type === 'error' ? 'err' : type === 'warning' ? 'warn' : ''}`;
    el.textContent = msg;
    $('#toast-container').appendChild(el);
    setTimeout(() => { el.style.opacity = '0'; setTimeout(() => el.remove(), 300); }, 3000);
  }

  marked.use({
    renderer: {
      code({ text, lang }) {
        const codeId = 'code-' + Math.random().toString(36).slice(2, 8);
        const langLabel = lang || 'code';
        let highlighted;
        try {
          if (typeof hljs !== 'undefined' && lang && hljs.getLanguage(lang)) {
            highlighted = hljs.highlight(text, { language: lang }).value;
          } else if (typeof hljs !== 'undefined') {
            highlighted = hljs.highlightAuto(text).value;
          } else {
            highlighted = escapeHtml(text);
          }
        } catch (e) {
          highlighted = escapeHtml(text);
        }
        return `<div class="code-block"><div class="code-header"><span>${escapeHtml(langLabel)}</span><button class="code-copy" data-code-id="${codeId}">Copy</button></div><div class="code-content"><code id="${codeId}">${highlighted}</code></div></div>`;
      },
      codespan({ text }) {
        return `<code style="background:var(--bg-2);padding:2px 6px;border-radius:3px;font-family:var(--font-d);font-size:13px">${text}</code>`;
      }
    },
    breaks: true,
    gfm: true
  });

  function renderMarkdown(text) {
    try {
      return marked.parse(text);
    } catch (e) {
      return escapeHtml(text).replace(/\n/g, '<br>');
    }
  }

  function renderPlainText(text) {
    return `<div class="streaming-text">${escapeHtml(text).replace(/\n/g, '<br>')}</div>`;
  }

  function bindCodeCopy(container) {
    container.querySelectorAll('.code-copy:not([data-bound])').forEach(btn => {
      btn.dataset.bound = '1';
      btn.addEventListener('click', () => {
        const codeId = btn.dataset.codeId;
        const codeEl = codeId ? document.getElementById(codeId) : btn.closest('.code-block')?.querySelector('code');
        if (!codeEl) return;
        const text = codeEl.textContent;
        navigator.clipboard.writeText(text).then(() => {
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
        }).catch(() => {
          const ta = document.createElement('textarea');
          ta.value = text;
          ta.style.position = 'fixed';
          ta.style.left = '-9999px';
          document.body.appendChild(ta);
          ta.select();
          document.execCommand('copy');
          document.body.removeChild(ta);
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
        });
      });
    });
  }

  function escapeHtml(str) {
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  function getCSS(prop) {
    return getComputedStyle(document.documentElement).getPropertyValue(prop).trim();
  }

  function rgbToHex(rgb) {
    if (!rgb || rgb.startsWith('#')) return rgb || '#000000';
    const m = rgb.match(/\d+/g);
    if (!m || m.length < 3) return '#000000';
    return '#' + m.slice(0, 3).map(x => parseInt(x).toString(16).padStart(2, '0')).join('');
  }

  function loadSettings() {
    try {
      const saved = localStorage.getItem('smartclaw-settings');
      if (saved) Object.assign(state.settings, JSON.parse(saved));
      const theme = localStorage.getItem('smartclaw-theme');
      if (theme) document.documentElement.setAttribute('data-theme', theme);
    } catch {}
    applySettings();
  }

  function saveSettings() {
    try {
      localStorage.setItem('smartclaw-settings', JSON.stringify(state.settings));
      localStorage.setItem('smartclaw-theme', state.settings.theme);
    } catch {}
  }

  function applySettings() {
    document.documentElement.setAttribute('data-theme', state.settings.theme);
    document.body.style.fontSize = state.settings.fontSize + 'px';
    $('#theme-select').value = state.settings.theme;
    $('#font-size').value = state.settings.fontSize;
    $('#current-model').textContent = state.settings.model;
    $('#model-select').value = state.settings.model;
  }

  function initFileMention() {
    const input = $('#input');
    const mention = $('#file-mention');
    const mentionList = $('#file-mention-list');

    input.addEventListener('input', () => {
      const val = input.value;
      const cursorPos = input.selectionStart;
      const atIdx = val.lastIndexOf('@', cursorPos - 1);
      if (atIdx === -1 || (atIdx > 0 && val[atIdx - 1] !== ' ' && val[atIdx - 1] !== '\n')) {
        mention.classList.add('hidden');
        state.mentionStart = -1;
        return;
      }
      const query = val.slice(atIdx + 1, cursorPos).toLowerCase();
      const filtered = state.flatFiles.filter(f => f.path.toLowerCase().includes(query)).slice(0, 20);
      if (filtered.length === 0) {
        mention.classList.add('hidden');
        state.mentionStart = -1;
        return;
      }
      state.mentionStart = atIdx;
      state.mentionIndex = -1;
      mentionList.innerHTML = '';
      filtered.forEach((f, i) => {
        const li = document.createElement('li');
        li.className = 'file-mention-item';
        li.dataset.index = i;
        const lastSlash = f.path.lastIndexOf('/');
        const dir = lastSlash > 0 ? f.path.slice(0, lastSlash + 1) : '';
        const name = lastSlash > 0 ? f.path.slice(lastSlash + 1) : f.path;
        li.innerHTML = `<svg class="fm-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg><span class="fm-path">${dir}<span class="fm-name">${name}</span></span>`;
        li.addEventListener('click', () => insertMention(f.path));
        li.addEventListener('mouseenter', () => {
          state.mentionIndex = i;
          updateMentionSelection();
        });
        mentionList.appendChild(li);
      });
      mention.classList.remove('hidden');
    });

    input.addEventListener('keydown', (e) => {
      if (mention.classList.contains('hidden') || state.mentionStart === -1) return;
      const items = $$('.file-mention-item', mentionList);
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        state.mentionIndex = Math.min(state.mentionIndex + 1, items.length - 1);
        updateMentionSelection();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        state.mentionIndex = Math.max(state.mentionIndex - 1, 0);
        updateMentionSelection();
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        e.stopPropagation();
        if (state.mentionIndex >= 0 && items[state.mentionIndex]) {
          items[state.mentionIndex].click();
        }
      } else if (e.key === 'Escape') {
        mention.classList.add('hidden');
        state.mentionStart = -1;
      }
    }, true);

    function updateMentionSelection() {
      const items = $$('.file-mention-item', mentionList);
      items.forEach((el, i) => el.classList.toggle('sel', i === state.mentionIndex));
      if (items[state.mentionIndex]) items[state.mentionIndex].scrollIntoView({ block: 'nearest' });
    }

    function insertMention(path) {
      const before = input.value.slice(0, state.mentionStart);
      const after = input.value.slice(input.selectionStart);
      input.value = before + '@' + path + ' ' + after;
      const newPos = state.mentionStart + path.length + 2;
      input.selectionStart = input.selectionEnd = newPos;
      mention.classList.add('hidden');
      state.mentionStart = -1;
      input.focus();
    }
  }

  function init() {
    loadSettings();
    wsConnect();
    initDragDrop();
    initCmdPalette();
    initFileMention();
    initThemeEditor();

    $('#btn-send').addEventListener('click', sendMessage);
    $('#input').addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
    });
    $('#input').addEventListener('input', function() {
      this.style.height = 'auto';
      this.style.height = Math.min(this.scrollHeight, 200) + 'px';
    });

    $('#sidebar-open').addEventListener('click', () => {
      const sb = $('#sidebar');
      if (sb.classList.contains('collapsed')) {
        sb.classList.remove('collapsed');
        state.ui.sidebarOpen = true;
      } else {
        sb.classList.add('collapsed');
        state.ui.sidebarOpen = false;
      }
    });

    $$('.nav-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        $$('.nav-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        $$('.section').forEach(s => s.classList.remove('active'));
        $(`#section-${btn.dataset.section}`)?.classList.add('active');
      });
    });

    $('#drawer-close').addEventListener('click', closeDrawer);
    $('#drawer-add-context').addEventListener('click', () => {
      const selection = window.getSelection();
      const selectedText = selection.toString().trim();
      const path = state.ui.currentFile || 'file';
      const input = $('#input');
      let snippet;
      if (selectedText) {
        snippet = `\n\`\`\`${path}\n${selectedText}\n\`\`\`\n`;
      } else {
        const content = $('#drawer-content code').textContent;
        snippet = `\n\`\`\`${path}\n${content}\n\`\`\`\n`;
      }
      input.value += snippet;
      input.style.height = 'auto';
      input.style.height = Math.min(input.scrollHeight, 200) + 'px';
      input.focus();
      closeDrawer();
    });
    $('#drawer-edit').addEventListener('click', () => {
      const content = $('#drawer-content code').textContent;
      openEditor(content, state.ui.currentFile);
      closeDrawer();
    });

    $('#editor-close').addEventListener('click', closeEditor);
    $('#editor-save').addEventListener('click', saveEditor);
    $('#dashboard-close').addEventListener('click', () => $('#dashboard-panel').classList.remove('visible'));
    $('#btn-dashboard').addEventListener('click', openDashboard);
    $('#btn-editor').addEventListener('click', () => {
      openEditor('', state.ui.editorFile || 'untitled.go');
    });

    $('#theme-editor-close').addEventListener('click', () => $('#theme-editor-panel').classList.remove('visible'));
    $('#open-theme-editor').addEventListener('click', () => {
      const panel = $('#theme-editor-panel');
      panel.classList.remove('hidden');
      requestAnimationFrame(() => panel.classList.add('visible'));
    });
    $('#theme-export').addEventListener('click', exportTheme);
    $('#theme-import').addEventListener('click', importTheme);

    $('#theme-select').addEventListener('change', (e) => {
      state.settings.theme = e.target.value;
      saveSettings();
      applySettings();
    });
    $('#model-select').addEventListener('change', (e) => {
      state.settings.model = e.target.value;
      $('#current-model').textContent = e.target.value;
      wsSend('model', { model: e.target.value });
      saveSettings();
    });
    $('#font-size').addEventListener('input', (e) => {
      state.settings.fontSize = parseInt(e.target.value);
      saveSettings();
      applySettings();
    });

    $('#btn-voice').addEventListener('click', () => {
      if (state.isRecording) stopVoice();
      else startVoice();
    });
    $('#voice-stop').addEventListener('click', stopVoice);

    $('#refresh-files').addEventListener('click', () => wsSend('file_tree', { path: '.' }));
    $('#new-session').addEventListener('click', () => wsSend('session_new', { model: state.settings.model }));

    document.addEventListener('keydown', (e) => {
      if (e.ctrlKey || e.metaKey) {
        if (e.key === 'k') { e.preventDefault(); $('#input').focus(); $('#input').value = '/'; $('#input').dispatchEvent(new Event('input')); }
        else if (e.key === 's' && state.ui.editorFile) { e.preventDefault(); saveEditor(); }
        else if (e.key === 'n') { e.preventDefault(); wsSend('session_new', { model: state.settings.model }); }
        else if (e.key === '/') { e.preventDefault(); $('#sidebar').classList.toggle('collapsed'); }
      }
      if (e.key === 'Escape') {
        closeDrawer();
        closeEditor();
        $('#dashboard-panel').classList.remove('visible');
        $('#theme-editor-panel').classList.remove('visible');
        $('#cmd-palette').classList.add('hidden');
        $('#file-mention').classList.add('hidden');
        state.mentionStart = -1;
      }
    });

    $('#btn-attach').addEventListener('click', () => {
      const input = document.createElement('input');
      input.type = 'file';
      input.multiple = true;
      input.onchange = (e) => {
        Array.from(e.target.files).forEach(f => {
          addMessage('user', `Attached: ${f.name} (${(f.size / 1024).toFixed(1)}KB)`);
        });
      };
      input.click();
    });
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
