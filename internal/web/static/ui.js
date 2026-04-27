// SmartClaw - UI Utilities
(function() {
  'use strict';

  function toast(msg, type = 'info') {
    const el = document.createElement('div');
    el.className = `toast ${type === 'success' ? 'ok' : type === 'error' ? 'err' : type === 'warning' ? 'warn' : ''}`;
    el.textContent = msg;
    SC.$('#toast-container').appendChild(el);
    setTimeout(() => { el.style.opacity = '0'; setTimeout(() => el.remove(), 300); }, 3000);
  }

  function scrollChat() {
    const chat = SC.$('#chat');
    const isNearBottom = chat.scrollHeight - chat.scrollTop - chat.clientHeight < 100;
    if (isNearBottom) chat.scrollTop = chat.scrollHeight;
  }

  function updateStats() {
    const pct = Math.min(SC.state.tokens.used / SC.state.tokens.limit * 100, 100);
    SC.$('#token-fill').style.width = pct + '%';
    SC.$('#stat-tokens').textContent = `${(SC.state.tokens.used / 1000).toFixed(1)}k / ${(SC.state.tokens.limit / 1000)}k tokens`;
    SC.$('#stat-cost').textContent = `$${SC.state.cost.toFixed(2)}`;
    SC.$('#stat-agents').textContent = `${SC.state.agents.length} agents`;

    const tokenBar = SC.$('#token-bar');
    if (tokenBar && SC.state.lastCostBreakdown) {
      const b = SC.state.lastCostBreakdown;
      tokenBar.title = `${b.model}\nInput: $${b.inputCost.toFixed(4)} (${b.inputRate})\nOutput: $${b.outputCost.toFixed(4)} (${b.outputRate})\nTotal: $${(b.inputCost + b.outputCost).toFixed(4)}`;
    }

    const models = Object.entries(SC.state.costByModel);
    if (models.length > 0) {
      const breakdown = models.map(([m, c]) => `${m}: $${c.toFixed(4)}`).join('\n');
      const costEl = SC.$('#stat-cost');
      if (costEl) costEl.title = breakdown;
    }
  }

  function loadSettings() {
    try {
      const saved = localStorage.getItem('smartclaw-settings');
      if (saved) Object.assign(SC.state.settings, JSON.parse(saved));
      const theme = localStorage.getItem('smartclaw-theme');
      if (theme) document.documentElement.setAttribute('data-theme', theme);
    } catch {}
    applySettings();
  }

  function saveSettings() {
    try {
      localStorage.setItem('smartclaw-settings', JSON.stringify(SC.state.settings));
      localStorage.setItem('smartclaw-theme', SC.state.settings.theme);
    } catch {}
  }

  function applySettings() {
    document.documentElement.setAttribute('data-theme', SC.state.settings.theme);
    document.body.style.fontSize = SC.state.settings.fontSize + 'px';
    SC.$('#theme-select').value = SC.state.settings.theme;
    SC.$('#font-size').value = SC.state.settings.fontSize;
    SC.$('#current-model').textContent = SC.state.settings.model;
    SC.$('#model-select').value = SC.state.settings.model;
  }

  function clearChat() {
    SC.$('#messages').innerHTML = '';
    SC.state.messages = [];
    toast('Chat cleared', 'success');
  }

  function showHelpModal() {
    const modal = SC.$('#help-modal');
    if (!modal) return;
    modal.classList.remove('hidden');
  }

  function hideHelpModal() {
    const modal = SC.$('#help-modal');
    if (modal) modal.classList.add('hidden');
  }

  function showWelcome() {
    const messages = SC.$('#messages');
    if (messages.children.length > 0) return;
    messages.innerHTML = `
      <div class="welcome">
        <svg class="welcome-icon" width="64" height="64" viewBox="0 0 512 512" fill="none">
          <circle cx="256" cy="256" r="240" fill="#0f172a" opacity=".95"/>
          <path d="M190 170L175 110L210 155Z" fill="#2d3748"/>
          <path d="M322 170L337 110L302 155Z" fill="#2d3748"/>
          <path d="M120 220Q80 280 100 360Q130 340 160 300Q170 260 155 230Z" fill="#2d3748"/>
          <path d="M392 220Q432 280 412 360Q382 340 352 300Q342 260 357 230Z" fill="#2d3748"/>
          <ellipse cx="256" cy="310" rx="100" ry="120" fill="#2d3748"/>
          <ellipse cx="256" cy="195" rx="85" ry="70" fill="#2d3748"/>
          <circle cx="225" cy="190" r="24" fill="#ed8936"/>
          <circle cx="287" cy="190" r="24" fill="#ed8936"/>
          <circle cx="225" cy="190" r="14" fill="#0d0d1a"/>
          <circle cx="287" cy="190" r="14" fill="#0d0d1a"/>
          <circle cx="225" cy="190" r="7" fill="#000"/>
          <circle cx="287" cy="190" r="7" fill="#000"/>
          <circle cx="219" cy="184" r="3.5" fill="#fff" opacity=".85"/>
          <circle cx="281" cy="184" r="3.5" fill="#fff" opacity=".85"/>
          <path d="M248 208L256 228L264 208Z" fill="#ed8936"/>
        </svg>
        <h2>SmartClaw</h2>
        <p>Your AI coding assistant. Ask me anything about your codebase, write features, debug issues, or explore your project.</p>
        <div class="shortcuts">
          <span class="shortcut"><kbd>Enter</kbd> Send</span>
          <span class="shortcut"><kbd>Shift+Enter</kbd> New line</span>
          <span class="shortcut"><kbd>@</kbd> Add files</span>
          <span class="shortcut"><kbd>/</kbd> Commands</span>
          <span class="shortcut"><kbd>Ctrl+K</kbd> Focus input</span>
        </div>
      </div>`;
  }

  function loadSectionData(section) {
    if (section === 'skills') {
      SC.wsSend('skill_list', {});
    } else if (section === 'memory') {
      SC.wsSend('memory_layers', {});
      SC.wsSend('memory_stats', {});
    } else if (section === 'wiki') {
      SC.wsSend('wiki_pages', {});
    } else if (section === 'agents') {
      SC.wsSend('agent_list', {});
    }
  }

  function focusModelSwitcher() {
    const modelSelect = SC.$('#model-select');
    if (modelSelect) {
      modelSelect.focus();
      modelSelect.size = Math.min(modelSelect.options.length, 8);
      modelSelect.addEventListener('blur', function handler() {
        modelSelect.size = 1;
        modelSelect.removeEventListener('blur', handler);
      });
      modelSelect.addEventListener('change', function handler() {
        modelSelect.size = 1;
        modelSelect.removeEventListener('change', handler);
      });
    }
  }

  function initCmdPalette() {
    const input = SC.$('#input');
    const palette = SC.$('#cmd-palette');
    const cmdInput = SC.$('#cmd-input');
    const cmdList = SC.$('#cmd-list');

    input.addEventListener('input', () => {
      if (input.value.startsWith('/')) {
        palette.classList.remove('hidden');
        const query = input.value.slice(1).toLowerCase();
        const filtered = SC.commands.filter(c => c.name.includes(query) || c.desc.toLowerCase().includes(query));
        renderCmdList(filtered);
      } else {
        palette.classList.add('hidden');
      }
    });

    function renderCmdList(items) {
      cmdList.innerHTML = '';
      SC.state.cmdIndex = -1;
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
        const items = SC.$$('.cmd-item', cmdList);
        if (e.key === 'ArrowDown') { e.preventDefault(); SC.state.cmdIndex = Math.min(SC.state.cmdIndex + 1, items.length - 1); updateCmdSelection(items); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); SC.state.cmdIndex = Math.max(SC.state.cmdIndex - 1, 0); updateCmdSelection(items); }
        else if (e.key === 'Enter' && SC.state.cmdIndex >= 0) { e.preventDefault(); items[SC.state.cmdIndex]?.click(); }
        else if (e.key === 'Escape') { palette.classList.add('hidden'); }
      }
    });
  }

  function updateCmdSelection(items) {
    items.forEach((el, i) => el.classList.toggle('sel', i === SC.state.cmdIndex));
  }

  // Dashboard
  function openDashboard() {
    const panel = SC.$('#dashboard-panel');
    panel.classList.remove('hidden');
    requestAnimationFrame(() => panel.classList.add('visible'));
    drawCharts();
  }

  function drawCharts() {
    drawTokenChart();
    drawCostChart();
    drawAgentChart();
    SC.$('#s-total-msgs').textContent = SC.state.messages.length;
    SC.$('#s-total-tokens').textContent = SC.state.tokens.used.toLocaleString();
    SC.$('#s-total-cost').textContent = `$${SC.state.cost.toFixed(2)}`;
    const modelBreakdown = Object.entries(SC.state.costByModel)
      .map(([m, c]) => `${m}: $${c.toFixed(4)}`)
      .join('\n');
    const costEl = SC.$('#s-total-cost');
    if (costEl && modelBreakdown) costEl.title = modelBreakdown;
  }

  function drawTokenChart() {
    const canvas = SC.$('#chart-tokens');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = SC.getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    const data = SC.state.tokenHistory.slice(-20);
    if (data.length < 2) { ctx.fillStyle = SC.getCSS('--tx-2'); ctx.font = '12px Inter'; ctx.fillText('Waiting for data...', w/2-50, h/2); return; }
    const max = Math.max(...data.map(d => d.v), 1);
    ctx.strokeStyle = SC.getCSS('--accent');
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
    ctx.fillStyle = SC.getCSS('--accent-bg');
    ctx.fill();
  }

  function drawCostChart() {
    const canvas = SC.$('#chart-cost');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = SC.getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);

    const modelEntries = Object.entries(SC.state.costByModel);
    if (modelEntries.length > 0) {
      const modelColors = ['#3b82f6','#10b981','#f59e0b','#ef4444','#8b5cf6','#ec4899','#06b6d4','#f97316','#84cc16','#6366f1'];
      const maxCost = Math.max(...modelEntries.map(([,c]) => c), 0.01);
      const barW = Math.min(40, (w - 40) / modelEntries.length - 4);
      const totalBarW = modelEntries.length * (barW + 4);
      const startX = (w - totalBarW) / 2;

      modelEntries.forEach(([model, cost], i) => {
        const barH = (cost / maxCost) * (h - 60);
        const x = startX + i * (barW + 4);
        const y = h - 30 - barH;
        ctx.fillStyle = modelColors[i % modelColors.length];
        ctx.fillRect(x, y, barW, barH);

        ctx.fillStyle = SC.getCSS('--tx-2');
        ctx.font = '9px Inter';
        ctx.textAlign = 'center';
        const shortModel = model.replace(/^(claude|gpt|gemini|glm)-/, '').slice(0, 10);
        ctx.fillText(shortModel, x + barW / 2, h - 16);
        ctx.fillStyle = SC.getCSS('--tx-0');
        ctx.font = '9px JetBrains Mono';
        ctx.fillText(`$${cost.toFixed(3)}`, x + barW / 2, y - 4);
      });
    } else {
      const data = SC.state.costHistory.slice(-20);
      if (data.length < 2) { ctx.fillStyle = SC.getCSS('--tx-2'); ctx.font = '12px Inter'; ctx.fillText('Waiting for data...', w/2-50, h/2); return; }
      const max = Math.max(...data.map(d => d.v), 0.01);
      ctx.strokeStyle = SC.getCSS('--accent');
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
      ctx.fillStyle = SC.getCSS('--accent-bg');
      ctx.fill();
    }

    ctx.fillStyle = SC.getCSS('--tx-0');
    ctx.font = 'bold 12px JetBrains Mono';
    ctx.textAlign = 'center';
    ctx.fillText(`$${SC.state.cost.toFixed(3)}`, w/2, h/2);
  }

  function drawAgentChart() {
    const canvas = SC.$('#chart-agents');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = SC.getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    const data = SC.state.agentHistory.slice(-20);
    if (data.length < 2) { ctx.fillStyle = SC.getCSS('--tx-2'); ctx.font = '12px Inter'; ctx.fillText('No agent activity yet', w/2-70, h/2); return; }
    const barW = (w - 40) / data.length;
    const max = Math.max(...data.map(d => d.v), 1);
    data.forEach((d, i) => {
      const barH = (d.v / max) * (h - 40);
      ctx.fillStyle = d.color || SC.getCSS('--accent');
      ctx.fillRect(20 + i * barW + 2, h - 20 - barH, barW - 4, barH);
    });
  }

  // Theme Editor
  function initThemeEditor() {
    const vars = ['--bg-0','--bg-1','--bg-2','--bg-3','--tx-0','--tx-1','--accent','--accent-h','--bd','--ok','--warn','--err'];
    const container = SC.$('#theme-colors');
    container.innerHTML = '';
    vars.forEach(v => {
      const item = document.createElement('div');
      item.className = 'color-item';
      const current = SC.getCSS(v);
      item.innerHTML = `<label>${v}</label><input type="color" value="${SC.rgbToHex(current)}">`;
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

  // Provider Config
  function loadProviderConfig() {
    fetch('/api/config')
      .then(r => r.json())
      .then(cfg => {
        if (cfg.api_key) {
          const el = SC.$('#cfg-api-key');
          if (el) el.value = cfg.api_key;
        }
        if (cfg.base_url) {
          const el = SC.$('#cfg-base-url');
          if (el) el.value = cfg.base_url;
        }
        if (cfg.model) {
          const el = SC.$('#cfg-custom-model');
          if (el) el.value = cfg.model;
          const sel = SC.$('#model-select');
          if (sel) {
            let found = false;
            for (const opt of sel.options) {
              if (opt.value === cfg.model) { found = true; break; }
            }
            if (found) sel.value = cfg.model;
          }
          SC.$('#current-model').textContent = cfg.model;
          SC.state.settings.model = cfg.model;
        }
        if (cfg.openai !== undefined) {
          const el = SC.$('#cfg-openai');
          if (el) el.checked = cfg.openai;
        }
      })
      .catch(() => {});
  }

  function saveProviderConfig() {
    const apiKey = SC.$('#cfg-api-key')?.value?.trim() || '';
    const baseUrl = SC.$('#cfg-base-url')?.value?.trim() || '';
    const customModel = SC.$('#cfg-custom-model')?.value?.trim() || '';
    const openai = SC.$('#cfg-openai')?.checked ?? true;

    const model = customModel || SC.$('#model-select')?.value || 'sre-model';

    const config = {
      api_key: apiKey,
      base_url: baseUrl,
      model: model,
      openai: openai,
      show_thinking: true,
    };

    fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    })
    .then(r => r.json())
    .then(() => {
      SC.state.settings.model = model;
      SC.$('#current-model').textContent = model;
      SC.wsSend('model', { model: model });
      toast('Provider config saved & applied', 'success');
    })
    .catch(() => toast('Failed to save config', 'error'));
  }

  SC.toast = toast;
  SC.scrollChat = scrollChat;
  SC.updateStats = updateStats;
  SC.loadSettings = loadSettings;
  SC.saveSettings = saveSettings;
  SC.applySettings = applySettings;
  SC.clearChat = clearChat;
  SC.showHelpModal = showHelpModal;
  SC.hideHelpModal = hideHelpModal;
  SC.showWelcome = showWelcome;
  SC.loadSectionData = loadSectionData;
  SC.focusModelSwitcher = focusModelSwitcher;
  SC.initCmdPalette = initCmdPalette;
  SC.updateCmdSelection = updateCmdSelection;
  SC.openDashboard = openDashboard;
  SC.drawCharts = drawCharts;
  SC.initThemeEditor = initThemeEditor;
  SC.exportTheme = exportTheme;
  SC.importTheme = importTheme;
  SC.loadProviderConfig = loadProviderConfig;
  SC.saveProviderConfig = saveProviderConfig;

  function initSidebarDragSort() {
    const sidebarBody = SC.$('.sidebar-body');
    if (!sidebarBody) return;

    applySidebarOrder();
    applySidebarCollapsedState();
    addDragHandles();

    sidebarBody.addEventListener('dragstart', (e) => {
      const section = e.target.closest('.section');
      if (!section) return;
      const handle = e.target.closest('.drag-handle');
      if (!handle) {
        e.preventDefault();
        return;
      }
      e.dataTransfer.setData('text/plain', section.id);
      e.dataTransfer.effectAllowed = 'move';
      section.classList.add('dragging');
    });

    sidebarBody.addEventListener('dragend', (e) => {
      const section = e.target.closest('.section');
      if (section) section.classList.remove('dragging');
      SC.$$('.section', sidebarBody).forEach(s => s.classList.remove('drag-over'));
    });

    sidebarBody.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      const target = e.target.closest('.section');
      if (!target || target.classList.contains('dragging')) return;
      SC.$$('.section', sidebarBody).forEach(s => s.classList.remove('drag-over'));
      target.classList.add('drag-over');
    });

    sidebarBody.addEventListener('dragleave', (e) => {
      const target = e.target.closest('.section');
      if (target) target.classList.remove('drag-over');
    });

    sidebarBody.addEventListener('drop', (e) => {
      e.preventDefault();
      const target = e.target.closest('.section');
      if (!target) return;
      const draggedId = e.dataTransfer.getData('text/plain');
      const dragged = SC.$('#' + draggedId);
      if (!dragged || dragged === target) return;

      const sections = SC.$$('.section', sidebarBody);
      const draggedIdx = sections.indexOf(dragged);
      const targetIdx = sections.indexOf(target);

      if (draggedIdx < targetIdx) {
        sidebarBody.insertBefore(dragged, target.nextSibling);
      } else {
        sidebarBody.insertBefore(dragged, target);
      }

      SC.$$('.section', sidebarBody).forEach(s => s.classList.remove('drag-over'));
      saveSidebarOrder();
    });

    sidebarBody.addEventListener('click', (e) => {
      const head = e.target.closest('.section-head');
      if (!head) return;
      const toggle = e.target.closest('.collapse-toggle');
      if (!toggle) return;
      const section = head.closest('.section');
      if (!section) return;
      const isCollapsed = section.classList.toggle('section-collapsed');
      saveSidebarCollapsedState();
    });
  }

  function addDragHandles() {
    SC.$$('.section-head').forEach(head => {
      if (head.querySelector('.drag-handle')) return;
      const handle = document.createElement('span');
      handle.className = 'drag-handle';
      handle.textContent = '⋮⋮';
      handle.draggable = true;
      head.insertBefore(handle, head.firstChild);

      if (!head.querySelector('.collapse-toggle')) {
        const toggle = document.createElement('span');
        toggle.className = 'collapse-toggle';
        toggle.innerHTML = '▸';
        head.appendChild(toggle);
      }
    });
  }

  function saveSidebarOrder() {
    const sections = SC.$$('.section');
    const order = sections.map(s => s.id);
    try { localStorage.setItem('smartclaw-sidebar-order', JSON.stringify(order)); } catch {}
  }

  function applySidebarOrder() {
    try {
      const saved = localStorage.getItem('smartclaw-sidebar-order');
      if (!saved) return;
      const order = JSON.parse(saved);
      const sidebarBody = SC.$('.sidebar-body');
      if (!sidebarBody) return;
      order.forEach(id => {
        const section = SC.$('#' + id);
        if (section) sidebarBody.appendChild(section);
      });
    } catch {}
  }

  function saveSidebarCollapsedState() {
    const state = {};
    SC.$$('.section').forEach(s => {
      state[s.id] = s.classList.contains('section-collapsed');
    });
    try { localStorage.setItem('smartclaw-sidebar-collapsed', JSON.stringify(state)); } catch {}
  }

  function applySidebarCollapsedState() {
    try {
      const saved = localStorage.getItem('smartclaw-sidebar-collapsed');
      if (!saved) return;
      const state = JSON.parse(saved);
      Object.entries(state).forEach(([id, collapsed]) => {
        const section = SC.$('#' + id);
        if (section) {
          section.classList.toggle('section-collapsed', collapsed);
        }
      });
    } catch {}
  }

  SC.initSidebarDragSort = initSidebarDragSort;
})();
