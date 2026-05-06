// SmartClaw - UI Utilities
(function() {
  'use strict';

  function toast(msg, type, duration) {
    type = type || 'ok';
    duration = duration || 3000;
    var container = SC.$('#toast-container');
    if (!container) return;
    var el = document.createElement('div');
    el.className = 'toast ' + type;

    var iconSvg = '';
    if (type === 'ok' || type === 'success') iconSvg = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>';
    else if (type === 'err' || type === 'error') iconSvg = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>';
    else if (type === 'warn' || type === 'warning') iconSvg = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>';
    else iconSvg = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>';

    el.innerHTML = iconSvg + '<span style="flex:1">' + SC.escapeHtml(msg) + '</span><button class="toast-close" aria-label="Dismiss">\u2715</button><div class="toast-progress"></div>';

    if (type === 'success' || type === 'ok') { if (SC.audio && SC.audio.success) SC.audio.success(); }
    else if (type === 'error' || type === 'err') { if (SC.audio && SC.audio.error) SC.audio.error(); }

    container.appendChild(el);

    var progressEl = el.querySelector('.toast-progress');
    if (progressEl) {
      progressEl.style.animationDuration = duration + 'ms';
      progressEl.classList.add('active');
    }

    el.querySelector('.toast-close').addEventListener('click', function() {
      el.classList.add('toast-exit');
      setTimeout(function() { el.remove(); }, 200);
    });

    el._dismissTimer = setTimeout(function() {
      if (el.parentNode) {
        el.classList.add('toast-exit');
        setTimeout(function() { el.remove(); }, 200);
      }
    }, duration);

    el.addEventListener('mouseenter', function() {
      if (el._dismissTimer) {
        clearTimeout(el._dismissTimer);
        el._dismissTimer = null;
      }
    });
    el.addEventListener('mouseleave', function() {
      if (!el._dismissTimer) {
        el._dismissTimer = setTimeout(function() {
          if (el.parentNode) {
            el.classList.add('toast-exit');
            setTimeout(function() { el.remove(); }, 200);
          }
        }, 2000);
      }
    });
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

    if (typeof SC.updateStatusRing === 'function') SC.updateStatusRing();
  }

  function getSystemTheme() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  function resolveTheme(theme) {
    if (theme === 'system') return getSystemTheme();
    return theme || 'dark';
  }

  function loadSettings() {
    try {
      const saved = localStorage.getItem('smartclaw-settings');
      if (saved) Object.assign(SC.state.settings, JSON.parse(saved));
      const theme = localStorage.getItem('smartclaw-theme');
      if (theme) {
        SC.state.settings.theme = theme;
      } else if (!localStorage.getItem('smartclaw-theme-chosen')) {
        SC.state.settings.theme = 'system';
      }
      document.documentElement.setAttribute('data-theme', resolveTheme(SC.state.settings.theme));
    } catch {}
    applySettings();
    initSystemThemeListener();
  }

  function saveSettings() {
    try {
      localStorage.setItem('smartclaw-settings', JSON.stringify(SC.state.settings));
      localStorage.setItem('smartclaw-theme', SC.state.settings.theme);
    } catch {}
  }

  function applySettings() {
    document.documentElement.setAttribute('data-theme', resolveTheme(SC.state.settings.theme));
    document.body.style.fontSize = SC.state.settings.fontSize + 'px';
    var themeEl = SC.$('#theme-select-view') || SC.$('#theme-select');
    var fontEl = SC.$('#font-size-view') || SC.$('#font-size');
    var modelEl = SC.$('#model-select-view') || SC.$('#model-select');
    if (themeEl) themeEl.value = SC.state.settings.theme;
    if (fontEl) fontEl.value = SC.state.settings.fontSize;
    SC.$('#current-model').textContent = SC.state.settings.model;
    if (modelEl) modelEl.value = SC.state.settings.model;
  }

  function clearChat() {
    if (SC.vl) {
      SC.vl.clear();
    } else {
      SC.$('#messages').innerHTML = '';
    }
    SC.state.messages = [];
    var welcome = SC.$('#welcome');
    if (welcome) welcome.classList.remove('hidden');
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
    var welcome = SC.$('#welcome');
    if (welcome) {
      welcome.classList.remove('hidden');
      return;
    }
    var messages = SC.$('#messages');
    if (!SC.vl && messages.children.length > 0) return;
    if (SC.vl && SC.vl.items.length > 0) return;
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
          <circle cx="225" cy="190" r="24" fill="#c084fc"/>
          <circle cx="287" cy="190" r="24" fill="#c084fc"/>
          <circle cx="225" cy="190" r="14" fill="#0d0d1a"/>
          <circle cx="287" cy="190" r="14" fill="#0d0d1a"/>
          <circle cx="225" cy="190" r="7" fill="#000"/>
          <circle cx="287" cy="190" r="7" fill="#000"/>
          <circle cx="219" cy="184" r="3.5" fill="#fff" opacity=".85"/>
          <circle cx="281" cy="184" r="3.5" fill="#fff" opacity=".85"/>
          <path d="M248 208L256 228L264 208Z" fill="#c084fc"/>
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
    if (section === 'sessions') {
      SC.wsSend('session_list', {});
    } else if (section === 'skills') {
      SC.wsSend('skill_list', {});
    } else if (section === 'memory') {
      SC.wsSend('memory_layers', {});
      SC.wsSend('memory_stats', {});
    } else if (section === 'wiki') {
      SC.wsSend('wiki_pages', {});
    } else if (section === 'agents') {
      SC.wsSend('agent_list', {});
    } else if (section === 'mcp') {
      SC.wsSend('mcp_list', {});
      if (SC.renderMCPMarketplace) SC.renderMCPMarketplace();
    } else if (section === 'cost') {
      if (typeof SC.initCostDashboard === 'function') SC.initCostDashboard();
    } else if (section === 'profile') {
      if (SCProfile) { SCProfile.loadProfile(); SCProfile.loadObservations(); SCProfile.renderPrivacySection(); }
    } else if (section === 'workflows') {
      if (typeof SC.initWorkflows === 'function') SC.initWorkflows();
    } else if (section === 'files') {
      if (SC.state.fileTreeData && SC.state.fileTreeData.length > 0) {
        SC.renderFileTree(SC.state.fileTreeData);
      } else {
        SC.wsSend('file_tree', { path: '.' });
      }
    } else if (section === 'cron') {
      SC.renderCronPanel();
    } else if (section === 'warroom') {
      SC.wsSend('warroom_list', {});
      if (SC.warroom && SC.warroom.render) SC.warroom.render();
    } else if (section === 'settings') {
      if (typeof SC.syncSettingsToView === 'function') SC.syncSettingsToView();
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

  var CMD_CATEGORIES = {
    'Core': ['/help', '/status', '/exit', '/clear', '/version'],
    'Model': ['/model', '/model-list', '/fast', '/lazy'],
    'Session': ['/session', '/resume', '/save', '/export', '/import', '/rename', '/fork', '/rewind', '/share', '/summary'],
    'Memory': ['/memory', '/skills', '/observe'],
    'Agent': ['/agent', '/agent-list', '/agent-switch', '/agent-create', '/agent-delete', '/subagent', '/agents'],
    'MCP': ['/mcp', '/mcp-add', '/mcp-remove', '/mcp-list', '/mcp-start', '/mcp-stop'],
    'Git': ['/git-status', '/git-diff', '/git-commit', '/git-branch', '/git-log', '/diff', '/commit'],
    'Tools': ['/tools', '/tasks', '/lsp', '/read', '/write', '/exec', '/browse', '/web', '/ide', '/install'],
    'Config': ['/config', '/config-show', '/config-set', '/config-get', '/config-reset', '/config-export', '/config-import', '/set-api-key', '/env'],
    'Diagnostics': ['/doctor', '/cost', '/stats', '/usage', '/debug', '/inspect', '/cache', '/heapdump', '/reset-limits'],
    'UI': ['/theme', '/color', '/vim', '/keybindings', '/statusline'],
    'Planning': ['/plan', '/think', '/deepthink', '/ultraplan', '/thinkback'],
    'Compact': ['/compact']
  };

  function initCmdPalette() {
    var input = SC.$('#input');
    var palette = SC.$('#cmd-palette');
    var cmdInput = SC.$('#cmd-input');
    var cmdList = SC.$('#cmd-list');

    function getRecentCmds() {
      try { return JSON.parse(localStorage.getItem('smartclaw-recent-cmds') || '[]'); } catch(e) { return []; }
    }

    function addRecentCmd(name) {
      var recent = getRecentCmds().filter(function(c) { return c !== name; });
      recent.unshift(name);
      recent = recent.slice(0, 5);
      try { localStorage.setItem('smartclaw-recent-cmds', JSON.stringify(recent)); } catch(e) {}
    }

    function getCategorizedCommands() {
      var catMap = {};
      Object.keys(CMD_CATEGORIES).forEach(function(cat) {
        CMD_CATEGORIES[cat].forEach(function(cmd) {
          catMap[cmd] = cat;
        });
      });
      return catMap;
    }

    input.addEventListener('input', function() {
      if (input.value.startsWith('/')) {
        palette.classList.remove('hidden');
        var query = input.value.slice(1).toLowerCase().trim();
        renderCmdPalette(query);
      } else {
        palette.classList.add('hidden');
      }
    });

    function renderCmdPalette(query) {
      cmdList.innerHTML = '';
      SC.state.cmdIndex = -1;

      if (!query) {
        // Show recently used + all categorized
        var recent = getRecentCmds();
        if (recent.length > 0) {
          var recentLabel = document.createElement('li');
          recentLabel.className = 'cmd-recent-label';
          recentLabel.textContent = 'Recently Used';
          recentLabel.setAttribute('role', 'presentation');
          cmdList.appendChild(recentLabel);

          recent.forEach(function(cmdName) {
            var cmd = (SC.commands || []).find(function(c) { return c.name === cmdName; });
            if (cmd) addCmdItem(cmd, false);
          });

          var sep = document.createElement('li');
          sep.className = 'cmd-separator';
          sep.setAttribute('role', 'presentation');
          cmdList.appendChild(sep);
        }

        // Categorized
        var catMap = getCategorizedCommands();
        var categories = {};
        (SC.commands || []).forEach(function(cmd) {
          var cat = catMap[cmd.name] || 'Other';
          if (!categories[cat]) categories[cat] = [];
          categories[cat].push(cmd);
        });

        Object.keys(categories).sort().forEach(function(cat) {
          var catLabel = document.createElement('li');
          catLabel.className = 'cmd-category';
          catLabel.textContent = cat;
          catLabel.setAttribute('role', 'presentation');
          cmdList.appendChild(catLabel);

          categories[cat].forEach(function(cmd) {
            addCmdItem(cmd, false);
          });
        });
      } else {
        // Fuzzy search
        var results = [];
        (SC.commands || []).forEach(function(cmd) {
          var nameMatch = SC.fuzzyMatch(query, cmd.name);
          var descMatch = SC.fuzzyMatch(query, cmd.desc || '');
          var best = nameMatch || descMatch;
          if (best) {
            results.push({ cmd: cmd, score: best.score, nameMatch: nameMatch, descMatch: descMatch });
          }
        });
        results.sort(function(a, b) { return b.score - a.score; });

        // Group by category
        var catMap = getCategorizedCommands();
        var currentCat = null;
        results.forEach(function(r) {
          var cat = catMap[r.cmd.name] || 'Other';
          if (cat !== currentCat) {
            var catLabel = document.createElement('li');
            catLabel.className = 'cmd-category';
            catLabel.textContent = cat;
            catLabel.setAttribute('role', 'presentation');
            cmdList.appendChild(catLabel);
            currentCat = cat;
          }
          addCmdItem(r.cmd, false, r.nameMatch);
        });

        if (results.length === 0) {
          var empty = document.createElement('li');
          empty.className = 'cmd-item';
          empty.style.color = 'var(--tx-2)';
          empty.style.cursor = 'default';
          empty.textContent = 'No matching commands';
          cmdList.appendChild(empty);
        }
      }
    }

    function addCmdItem(cmd, showCategory, nameMatch) {
      var li = document.createElement('li');
      li.className = 'cmd-item';
      var nameHtml = SC.escapeHtml(cmd.name);
      if (nameMatch && nameMatch.matches) {
        // Highlight matched characters
        var chars = cmd.name.split('');
        var matchSet = {};
        nameMatch.matches.forEach(function(idx) { matchSet[idx] = true; });
        nameHtml = chars.map(function(ch, i) {
          return matchSet[i] ? '<span class="cmd-match">' + SC.escapeHtml(ch) + '</span>' : SC.escapeHtml(ch);
        }).join('');
      }
      li.innerHTML = '<span>' + nameHtml + '</span><span class="cdesc">' + SC.escapeHtml(cmd.desc || '') + '</span>';
      li.addEventListener('click', function() {
        input.value = cmd.name + ' ';
        palette.classList.add('hidden');
        input.focus();
        addRecentCmd(cmd.name);
      });
      cmdList.appendChild(li);
    }

    input.addEventListener('keydown', function(e) {
      if (!palette.classList.contains('hidden')) {
        var items = SC.$$('.cmd-item', cmdList);
        // Filter out non-interactive items
        var interactiveItems = Array.from(items).filter(function(item) {
          return item.style.cursor !== 'default';
        });
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          SC.state.cmdIndex = Math.min(SC.state.cmdIndex + 1, interactiveItems.length - 1);
          updateCmdSelection(interactiveItems);
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          SC.state.cmdIndex = Math.max(SC.state.cmdIndex - 1, 0);
          updateCmdSelection(interactiveItems);
        } else if (e.key === 'Enter' && SC.state.cmdIndex >= 0) {
          e.preventDefault();
          if (interactiveItems[SC.state.cmdIndex]) interactiveItems[SC.state.cmdIndex].click();
        } else if (e.key === 'Escape') {
          palette.classList.add('hidden');
        }
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

  function initSystemThemeListener() {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    mq.addEventListener('change', () => {
      if (SC.state.settings.theme === 'system') {
        document.documentElement.setAttribute('data-theme', getSystemTheme());
      }
    });
  }

  function applyThemeWithTransition(theme) {
    SC.state.settings.theme = theme;
    localStorage.setItem('smartclaw-theme-chosen', '1');
    SC.saveSettings();
    document.documentElement.classList.add('theme-transitioning');
    applySettings();
    setTimeout(() => {
      document.documentElement.classList.remove('theme-transitioning');
    }, 250);
    if (typeof SC.updateEditorTheme === 'function') SC.updateEditorTheme();
  }

  // --- Command Palette V2 (Ctrl+K overlay) ---
  function openCmdPalette() {
    var overlay = document.createElement('div');
    overlay.className = 'cmd-palette-overlay';

    var palette = document.createElement('div');
    palette.className = 'cmd-palette';

    var inputWrap = document.createElement('div');
    inputWrap.className = 'cmd-palette-input-wrap';
    inputWrap.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>';
    var input = document.createElement('input');
    input.className = 'cmd-palette-input';
    input.placeholder = 'Type a command or search...';
    input.autocomplete = 'off';
    inputWrap.appendChild(input);

    var results = document.createElement('div');
    results.className = 'cmd-palette-results';

    var footer = document.createElement('div');
    footer.className = 'cmd-palette-footer';
    footer.innerHTML = '<span><kbd>↑↓</kbd> Navigate</span><span><kbd>↵</kbd> Execute</span><span><kbd>Esc</kbd> Close</span>';

    palette.appendChild(inputWrap);
    palette.appendChild(results);
    palette.appendChild(footer);
    overlay.appendChild(palette);
    document.body.appendChild(overlay);

    setTimeout(function() { input.focus(); }, 50);

    var allCommands = [];
    Object.keys(CMD_CATEGORIES).forEach(function(catName) {
      CMD_CATEGORIES[catName].forEach(function(item) {
        var label = typeof item === 'string' ? item : (item.label || item);
        var action = typeof item === 'object' && item.action ? item.action : null;
        allCommands.push({ label: label, category: catName, action: action });
      });
    });

    var navCommands = [
      { label: 'Go to Chat', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'chat') b.click(); }); } },
      { label: 'Go to Sessions', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'sessions') b.click(); }); } },
      { label: 'Go to Agents', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'agents') b.click(); }); } },
      { label: 'Go to Skills', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'skills') b.click(); }); } },
      { label: 'Go to Memory', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'memory') b.click(); }); } },
      { label: 'Go to Settings', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'settings') b.click(); }); } },
      { label: 'Go to Files', category: 'Navigation', action: function() { SC.$$('.sidebar-rail-btn').forEach(function(b) { if (b.dataset.view === 'files') b.click(); }); } },
      { label: 'Toggle Focus Mode', category: 'Actions', action: function() { var btn = SC.$('#btn-focus-mode') || SC.$('#btn-focus-mode-alt'); if (btn) btn.click(); } },
      { label: 'New Session', category: 'Actions', shortcut: 'Ctrl+N', action: function() { var btn = SC.$('#tab-new'); if (btn) btn.click(); } },
      { label: 'Toggle Sidebar', category: 'Actions', shortcut: 'Ctrl+/', action: function() { var btn = SC.$('#sidebar-open') || SC.$('#sidebar-open-view'); if (btn) btn.click(); } }
    ];

    allCommands = allCommands.concat(navCommands);

    var selectedIndex = 0;
    var filteredCommands = allCommands;

    function renderResults(query) {
      filteredCommands = allCommands;
      if (query) {
        filteredCommands = allCommands.filter(function(cmd) {
          return cmd.label.toLowerCase().indexOf(query.toLowerCase()) >= 0;
        });
      }

      results.innerHTML = '';
      if (filteredCommands.length === 0) {
        results.innerHTML = '<div style="padding:16px;text-align:center;color:var(--tx-2);font-size:13px">No results found</div>';
        return;
      }

      filteredCommands.forEach(function(cmd, i) {
        var item = document.createElement('div');
        item.className = 'cmd-palette-item' + (i === selectedIndex ? ' selected' : '');
        var labelHtml = SC.escapeHtml(cmd.label);
        if (query) {
          var idx = cmd.label.toLowerCase().indexOf(query.toLowerCase());
          if (idx >= 0) {
            labelHtml = SC.escapeHtml(cmd.label.slice(0, idx)) + '<span class="cmd-match">' + SC.escapeHtml(cmd.label.slice(idx, idx + query.length)) + '</span>' + SC.escapeHtml(cmd.label.slice(idx + query.length));
          }
        }
        item.innerHTML = '<span class="cmd-palette-item-icon">⌘</span><span class="cmd-palette-item-label">' + labelHtml + '</span>' + (cmd.shortcut ? '<span class="cmd-palette-item-shortcut">' + SC.escapeHtml(cmd.shortcut) + '</span>' : '');
        item.addEventListener('click', function() {
          closePalette();
          if (cmd.action) cmd.action();
          else if (cmd.label.charAt(0) === '/') {
            var chatInput = SC.$('#input');
            if (chatInput) {
              chatInput.value = cmd.label + ' ';
              chatInput.focus();
            }
          }
        });
        item.addEventListener('mouseenter', function() {
          selectedIndex = i;
          updateSelection();
        });
        results.appendChild(item);
      });
    }

    function updateSelection() {
      var items = results.querySelectorAll('.cmd-palette-item');
      items.forEach(function(item, i) {
        item.classList.toggle('selected', i === selectedIndex);
      });
      if (items[selectedIndex]) items[selectedIndex].scrollIntoView({ block: 'nearest' });
    }

    function closePalette() {
      overlay.classList.add('hidden');
      setTimeout(function() { overlay.remove(); }, 200);
    }

    input.addEventListener('input', function() {
      selectedIndex = 0;
      renderResults(input.value.trim());
    });

    input.addEventListener('keydown', function(e) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, filteredCommands.length - 1);
        updateSelection();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        updateSelection();
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (filteredCommands[selectedIndex]) {
          closePalette();
          var cmd = filteredCommands[selectedIndex];
          if (cmd.action) cmd.action();
          else if (cmd.label.charAt(0) === '/') {
            var chatInput = SC.$('#input');
            if (chatInput) {
              chatInput.value = cmd.label + ' ';
              chatInput.focus();
            }
          }
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        closePalette();
      }
    });

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) closePalette();
    });

    renderResults('');
  }

  SC.toggleCmdPalette = function() {
    var existing = SC.$('.cmd-palette-overlay');
    if (existing) {
      existing.classList.add('hidden');
      return;
    }
    openCmdPalette();
  };

  // --- Ripple Effect ---
  function addRipple(element, event) {
    if (!element) return;
    element.classList.add('ripple-container');
    var rect = element.getBoundingClientRect();
    var x = event.clientX - rect.left;
    var y = event.clientY - rect.top;
    var size = Math.max(rect.width, rect.height) * 2;
    var wave = document.createElement('span');
    wave.className = 'ripple-wave';
    wave.style.width = size + 'px';
    wave.style.height = size + 'px';
    wave.style.left = (x - size / 2) + 'px';
    wave.style.top = (y - size / 2) + 'px';
    element.appendChild(wave);
    setTimeout(function() { wave.remove(); }, 500);
  }

  SC.addRipple = addRipple;

  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.icon-btn, .btn-primary, .btn-ghost:not(.sm)');
    if (btn) SC.addRipple(btn, e);
  });

  // --- Context Menu Shortcuts ---
  SC.CONTEXT_SHORTCUTS = {
    'Copy': 'Ctrl+C',
    'Quote Reply': 'Ctrl+Shift+R',
    'Bookmark': 'Ctrl+D',
    'Edit': 'Ctrl+E',
    'Retry': 'Ctrl+Shift+R'
  };

  (function() {
    var style = document.createElement('style');
    style.textContent = '.context-shortcut { margin-left: auto; padding: 1px 5px; font-size: 9px; font-family: var(--font-d); color: var(--tx-2); background: var(--bg-3); border-radius: 2px; opacity: 0.7; }';
    document.head.appendChild(style);
  })();

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
  SC.applyThemeWithTransition = applyThemeWithTransition;

  function initSidebarDragSort() {
    const sidebarBody = SC.$('.sidebar-body');
    if (!sidebarBody) return;

    applySidebarOrder();
    applySidebarCollapsedState();
    addDragHandles();

    SC.makeDraggable(sidebarBody, {
      dropTarget: sidebarBody,
      onDragStart: function(e) {
        const section = e.target.closest('.section');
        if (!section) return;
        const handle = e.target.closest('.drag-handle');
        if (!handle) {
          e.preventDefault();
          return;
        }
        e.dataTransfer.setData('text/plain', section.id);
        section.classList.add('dragging');
      },
      onDragEnd: function() {
        SC.$$('.section', sidebarBody).forEach(function(s) {
          s.classList.remove('dragging');
          s.classList.remove('drag-over');
        });
      },
      onDragOver: function(e) {
        const target = e.target.closest('.section');
        if (!target || target.classList.contains('dragging')) return;
        SC.$$('.section', sidebarBody).forEach(s => s.classList.remove('drag-over'));
        target.classList.add('drag-over');
      },
      onDrop: function(e) {
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
      }
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

  function isMobile() {
    return window.innerWidth <= 768;
  }

  function toggleSidebarDrawer(show) {
    var sidebar = SC.$('#sidebar');
    var overlay = SC.$('#sidebar-overlay');
    if (!sidebar) return;
    if (show === undefined) {
      show = !sidebar.classList.contains('visible');
    }
    if (show) {
      sidebar.classList.add('visible');
      sidebar.classList.remove('collapsed');
      if (overlay) overlay.classList.add('visible');
      SC.state.ui.sidebarOpen = true;
    } else {
      sidebar.classList.remove('visible');
      if (overlay) overlay.classList.remove('visible');
      SC.state.ui.sidebarOpen = false;
    }
  }

  function initMobileNav() {
    var mobileNav = SC.$('#mobile-nav');
    if (!mobileNav) return;

    mobileNav.addEventListener('click', function(e) {
      var tab = e.target.closest('.mobile-tab');
      if (!tab) return;

      var tabName = tab.dataset.tab;
      SC.$$('.mobile-tab').forEach(function(t) { t.classList.remove('active'); });
      tab.classList.add('active');

      if (tabName === 'chat') {
        SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
        SC.$('#view-chat').classList.add('active');
        SC.$$('.sidebar-rail-btn').forEach(function(b) { b.classList.remove('active'); });
        var chatBtn = SC.$('.sidebar-rail-btn[data-view="chat"]');
        if (chatBtn) chatBtn.classList.add('active');
        toggleSidebarDrawer(false);
      } else if (tabName === 'more') {
        SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
        SC.$('#view-settings').classList.add('active');
        SC.$$('.sidebar-rail-btn').forEach(function(b) { b.classList.remove('active'); });
        var settingsBtn = SC.$('.sidebar-rail-btn[data-view="settings"]');
        if (settingsBtn) settingsBtn.classList.add('active');
        SC.loadSectionData('settings');
        if (SC.reRenderActiveView) SC.reRenderActiveView('settings');
        toggleSidebarDrawer(false);
      } else {
        SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
        var view = SC.$('#view-' + tabName);
        if (view) view.classList.add('active');
        SC.$$('.sidebar-rail-btn').forEach(function(b) { b.classList.remove('active'); });
        var railBtn = SC.$('.sidebar-rail-btn[data-view="' + tabName + '"]');
        if (railBtn) railBtn.classList.add('active');
        SC.loadSectionData(tabName);
        if (SC.reRenderActiveView) SC.reRenderActiveView(tabName);
        toggleSidebarDrawer(false);
      }
    });

    var overlay = SC.$('#sidebar-overlay');
    if (overlay) {
      overlay.addEventListener('click', function() {
        toggleSidebarDrawer(false);
        SC.$$('.mobile-tab').forEach(function(t) {
          t.classList.toggle('active', t.dataset.tab === 'chat');
        });
        SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
        SC.$('#view-chat').classList.add('active');
      });
    }
  }

  function initSwipeGestures() {
    var chatEl = SC.$('#chat');
    var sidebar = SC.$('#sidebar');
    if (!chatEl || !sidebar) return;

    var startX = 0;
    var startY = 0;
    var SWIPE_MIN = 50;
    var VERTICAL_TOLERANCE = 100;

    chatEl.addEventListener('touchstart', function(e) {
      startX = e.touches[0].clientX;
      startY = e.touches[0].clientY;
    }, { passive: true });

    chatEl.addEventListener('touchend', function(e) {
      var dx = e.changedTouches[0].clientX - startX;
      var dy = Math.abs(e.changedTouches[0].clientY - startY);

      if (dy > VERTICAL_TOLERANCE) return;

      if (dx < -SWIPE_MIN && !sidebar.classList.contains('visible')) {
        toggleSidebarDrawer(true);
      }
    }, { passive: true });

    sidebar.addEventListener('touchstart', function(e) {
      startX = e.touches[0].clientX;
      startY = e.touches[0].clientY;
    }, { passive: true });

    sidebar.addEventListener('touchend', function(e) {
      var dx = e.changedTouches[0].clientX - startX;
      var dy = Math.abs(e.changedTouches[0].clientY - startY);

      if (dy > VERTICAL_TOLERANCE) return;

      if (dx > SWIPE_MIN && sidebar.classList.contains('visible')) {
        toggleSidebarDrawer(false);
        SC.$$('.mobile-tab').forEach(function(t) {
          t.classList.toggle('active', t.dataset.tab === 'chat');
        });
        SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
        SC.$('#view-chat').classList.add('active');
        SC.$$('.sidebar-rail-btn').forEach(function(b) { b.classList.remove('active'); });
        var chatBtn = SC.$('.sidebar-rail-btn[data-view="chat"]');
        if (chatBtn) chatBtn.classList.add('active');
      }
    }, { passive: true });
  }

  function initViewportHandling() {
    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', function() {
        if (document.activeElement === SC.$('#input')) {
          SC.scrollChat();
        }
      });
    }

    var mql = window.matchMedia('(max-width: 768px)');
    function handleBreakpoint(e) {
      var mobileNav = SC.$('#mobile-nav');
      var sidebar = SC.$('#sidebar');
      if (e.matches) {
        if (mobileNav) mobileNav.style.display = '';
        if (sidebar) {
          sidebar.classList.remove('collapsed');
          sidebar.classList.remove('visible');
        }
        var overlay = SC.$('#sidebar-overlay');
        if (overlay) overlay.classList.remove('visible');
      } else {
        if (mobileNav) mobileNav.style.display = 'none';
        if (sidebar) {
          sidebar.classList.remove('visible');
          sidebar.classList.remove('collapsed');
        }
        var overlay2 = SC.$('#sidebar-overlay');
        if (overlay2) overlay2.classList.remove('visible');
      }
    }
    if (mql.addEventListener) {
      mql.addEventListener('change', handleBreakpoint);
    } else {
      mql.addListener(handleBreakpoint);
    }
    handleBreakpoint(mql);
  }

  SC.isMobile = isMobile;
  SC.toggleSidebarDrawer = toggleSidebarDrawer;
  SC.initMobileNav = initMobileNav;
  SC.initSwipeGestures = initSwipeGestures;
  SC.initViewportHandling = initViewportHandling;

  SC.renderRecentProjects = function() {
    var list = SC.$('#project-recent-list');
    if (list) {
      list.innerHTML = '';
      var projects = SC.state.recentProjects || [];
      if (projects.length === 0) {
        list.innerHTML = '<div style="padding:12px;color:var(--tx-2);font-size:12px;text-align:center">No recent projects</div>';
      } else {
        projects.forEach(function(proj) {
          var item = document.createElement('div');
          var initial = (proj.name || proj.path.split('/').pop()).charAt(0).toUpperCase();
          item.className = 'project-recent-item' + (proj.path === SC.state.projectPath ? ' active' : '');
          item.innerHTML = '<div class="project-avatar-sm' + (proj.path === SC.state.projectPath ? ' project-avatar-active' : '') + '">' + SC.escapeHtml(initial) + '</div>' +
            '<div><div class="project-recent-item-name">' + SC.escapeHtml(proj.name || proj.path.split('/').pop()) + '</div>' +
            '<div class="project-recent-item-path" style="font-size:11px;color:var(--tx-2)">' + SC.escapeHtml(proj.path) + '</div></div>';
          list.appendChild(item);
        });
      }
    }

    SC.renderProjectSwitcher();
  };

  SC.renderProjectSwitcher = function() {
    SC.updateProjectAvatar();
    SC.renderRailProjects();
  };

  SC.renderRailProjects = function() {
    var container = SC.$('#sidebar-rail-projects');
    if (!container) return;
    container.innerHTML = '';
    var projects = SC.state.recentProjects || [];
    projects.forEach(function(proj) {
      var btn = document.createElement('button');
      var initial = (proj.name || proj.path.split('/').pop()).charAt(0).toUpperCase();
      var isActive = proj.path === SC.state.projectPath;
      btn.className = 'sidebar-rail-project' + (isActive ? ' active' : '');
      btn.dataset.name = proj.name || proj.path.split('/').pop();
      btn.title = (proj.name || proj.path.split('/').pop()) + '\n' + proj.path;
      btn.textContent = initial;
      btn.addEventListener('click', function() {
        if (!isActive) SC.wsSend('change_project', { path: proj.path });
      });
      container.appendChild(btn);
    });
  };

  SC.updateProjectAvatar = function() {
    var avatar = SC.$('#project-avatar');
    if (!avatar) return;
    var name = SC.state.projectName || SC.$('#project-name')?.textContent || 'P';
    var initial = name.charAt(0).toUpperCase();
    avatar.textContent = initial;
  };
})();
