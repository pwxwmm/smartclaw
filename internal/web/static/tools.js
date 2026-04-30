// SmartClaw - Tools
(function() {
  'use strict';

  function addToolCard(msg) {
    SC.state.currentToolId = msg.id || Date.now().toString();
    const el = document.createElement('div');
    el.className = 'tool-card';
    el.id = `tool-${SC.state.currentToolId}`;
    el.dataset.toolName = msg.tool || '';
    const color = SC.toolColors[msg.tool] || 'var(--accent)';
    let detail = '';
    const input = msg.input || {};
    if (msg.tool === 'bash' && input.command) {
      detail = `<div class="tool-cmd" style="font-family:var(--font-d);font-size:13px;color:var(--tx-1);background:var(--bg-0);padding:6px 10px;border-radius:4px;margin:6px 0 0;white-space:pre-wrap;word-break:break-all">$ ${SC.escapeHtml(input.command)}</div>`;
    } else if (msg.tool === 'edit_file' && input.path) {
      let diffHtml = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(input.path)}</div>`;
      if (input.old_string || input.new_string) {
        diffHtml += `<div style="margin:6px 0 0;font-family:var(--font-d);font-size:12px;line-height:1.6;border-radius:4px;overflow:hidden">`;
        if (input.old_string) {
          const oldLines = String(input.old_string).split('\n');
          for (const line of oldLines) {
            diffHtml += `<div class="diff-rm" style="padding:1px 8px;white-space:pre-wrap;word-break:break-all">- ${SC.escapeHtml(line)}</div>`;
          }
        }
        if (input.new_string) {
          const newLines = String(input.new_string).split('\n');
          for (const line of newLines) {
            diffHtml += `<div class="diff-add" style="padding:1px 8px;white-space:pre-wrap;word-break:break-all">+ ${SC.escapeHtml(line)}</div>`;
          }
        }
        diffHtml += `</div>`;
        if (msg.annotations && msg.annotations.length > 0) {
          for (const ann of msg.annotations) {
            diffHtml += `<div class="diff-annotation">💡 ${SC.escapeHtml(ann.reason)}</div>`;
          }
        }
      }
      detail = diffHtml;
    } else if (msg.tool === 'write_file' && input.path) {
      detail = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(input.path)}</div>`;
    } else if (msg.tool === 'read_file' && input.path) {
      detail = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(input.path)}</div>`;
    } else if (msg.tool === 'browser_navigate' && input.url) {
      detail = `<div style="font-family:var(--font-d);font-size:13px;color:#06b6d4;margin:4px 0 0">${SC.escapeHtml(input.url)}</div>`;
    } else if (msg.tool && msg.tool.startsWith('sopa_')) {
      if (msg.tool === 'sopa_execute_task' && input.scriptId) {
        detail = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">script: ${SC.escapeHtml(String(input.scriptId))}</div>`;
      } else if (msg.tool === 'sopa_list_nodes') {
        const filters = [];
        if (input.status) filters.push(`status=${input.status}`);
        if (input.hostname) filters.push(`host=${input.hostname}`);
        detail = filters.length ? `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(filters.join(' · '))}</div>` : '';
      }
    } else if (msg.tool === 'git_diff' || msg.tool === 'git_status' || msg.tool === 'git_log') {
      if (input.workdir || input.path) {
        detail = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(input.workdir || input.path)}</div>`;
      }
    } else if (msg.tool === 'docker_exec' && input.image) {
      detail = `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(String(input.image))}</div>`;
    } else if (msg.tool === 'investigate_incident') {
      const parts = [];
      if (input.incident_id) parts.push(`#${input.incident_id}`);
      if (input.hypothesis) parts.push(String(input.hypothesis).slice(0, 80));
      detail = parts.length ? `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(parts.join(' · '))}</div>` : '';
    } else if (msg.tool === 'incident_timeline') {
      const parts = [];
      if (input.action) parts.push(String(input.action));
      if (input.incident_id) parts.push(`#${input.incident_id}`);
      detail = parts.length ? `<div style="font-family:var(--font-d);font-size:13px;color:var(--tx-2);margin:4px 0 0">${SC.escapeHtml(parts.join(' · '))}</div>` : '';
    }
    el.classList.add('running');
    SC.state.runningTools = SC.state.runningTools || {};
    SC.state.runningTools[msg.id || Date.now()] = { tool: msg.tool, start: Date.now() };
    el.innerHTML = `<div class="tool-head"><span class="tool-dot running"></span><span class="tool-badge" style="background:${color};color:#000;font-size:11px;font-weight:600;padding:2px 8px;border-radius:4px;margin-right:6px">${SC.escapeHtml(msg.tool || 'tool')}</span></div>${detail}<div class="tool-body"></div>`;
    SC.$('#messages').appendChild(el);
    if (SC.vl) SC.vl.refreshItemHeight(SC.state.messages.length - 1);
    SC.scrollChat();
    renderToolProgress();
  }

  function updateToolCard(msg) {
    const cardId = msg.id || SC.state.currentToolId;
    const body = SC.$(`#tool-${cardId} .tool-body`);
    if (!body) return;
    const toolName = (SC.$(`#tool-${cardId}`) || {}).dataset?.toolName || '';
    const output = msg.output || '';

    var imageRendered = false;
    if (toolName === 'browser_screenshot' || toolName === 'image') {
      try {
        var parsed = JSON.parse(output);
        // Server contract: browser_screenshot→{image_base64,format}, image→{base64,data_url,path}
        var imgSrc = '';
        if (parsed.image_base64) {
          var fmt = parsed.format || 'png';
          imgSrc = 'data:image/' + fmt + ';base64,' + parsed.image_base64;
        } else if (parsed.data_url && parsed.data_url.indexOf('data:image/') === 0) {
          imgSrc = parsed.data_url;
        } else if (parsed.base64) {
          var mime = 'image/png';
          if (parsed.path) {
            var ext = parsed.path.split('.').pop().toLowerCase();
            if (ext === 'jpg' || ext === 'jpeg') mime = 'image/jpeg';
            else if (ext === 'gif') mime = 'image/gif';
            else if (ext === 'webp') mime = 'image/webp';
            else if (ext === 'svg') mime = 'image/svg+xml';
            else if (ext === 'bmp') mime = 'image/bmp';
          }
          imgSrc = 'data:' + mime + ';base64,' + parsed.base64;
        }
        if (imgSrc) {
          var img = document.createElement('img');
          img.src = imgSrc;
          img.className = 'screenshot-img';
          img.style.cssText = 'max-width:100%;border-radius:8px;border:1px solid var(--glass-border);margin:4px 0;cursor:pointer;transition:transform 0.2s';
          img.addEventListener('click', function() {
            if (this.style.transform) {
              this.style.transform = '';
              this.style.maxWidth = '100%';
            } else {
              this.style.transform = 'scale(1.5)';
              this.style.maxWidth = 'none';
              this.style.position = 'relative';
              this.style.zIndex = '100';
            }
          });
          body.innerHTML = '';
          body.appendChild(img);
          imageRendered = true;
        }
      } catch (e) {
      }
      if (!imageRendered && output.length > 200 && /^[A-Za-z0-9+/=\s]+$/.test(output.slice(0, 500))) {
        var img = document.createElement('img');
        img.src = 'data:image/png;base64,' + output.replace(/\s/g, '');
        img.className = 'screenshot-img';
        img.style.cssText = 'max-width:100%;border-radius:8px;border:1px solid var(--glass-border);margin:4px 0';
        body.innerHTML = '';
        body.appendChild(img);
        imageRendered = true;
      }
    }
    if (!imageRendered) {
      body.textContent += output;
    }
    var cardIdx = -1;
    var cardEl = SC.$(`#tool-${cardId}`);
    if (cardEl && cardEl.dataset.msgIndex) cardIdx = parseInt(cardEl.dataset.msgIndex, 10);
    if (cardIdx < 0) {
      var allCards = SC.$$('#messages .tool-card');
      for (var ci = 0; ci < allCards.length; ci++) {
        if (allCards[ci].id === `tool-${cardId}`) { cardIdx = ci; break; }
      }
    }
    if (SC.vl && cardIdx >= 0) SC.vl.refreshItemHeight(cardIdx);
    SC.scrollChat();
  }

  function formatDuration(ms) {
    if (!ms) return '0ms';
    if (ms < 1000) return ms + 'ms';
    return (ms / 1000).toFixed(1) + 's';
  }

  function finishToolCard(msg) {
    const card = SC.$(`#tool-${msg.id || SC.state.currentToolId}`);
    if (!card) return;
    card.classList.remove('running');
    card.classList.add('completed');
    const dot = card.querySelector('.tool-dot');
    dot.classList.remove('running');
    dot.classList.add('ok');
    const head = card.querySelector('.tool-head');
    const durationSpan = document.createElement('span');
    durationSpan.style.cssText = 'margin-left:auto;font-size:12px;color:var(--tx-2);font-family:var(--font-d)';
    durationSpan.textContent = formatDuration(msg.duration);
    head.appendChild(durationSpan);
    const toolName = card.dataset.toolName;
    if (toolName === 'write_file') {
      const input = msg.input || {};
      const path = input.path;
      if (path) {
        const viewBtn = document.createElement('button');
        viewBtn.textContent = 'View';
        viewBtn.style.cssText = 'margin-left:8px;font-size:11px;padding:2px 8px;border-radius:4px;background:var(--bg-2);color:var(--accent);border:1px solid var(--bd);cursor:pointer';
        viewBtn.addEventListener('click', () => {
          SC.state.ui.currentFile = path;
          SC.wsSend('file_open', { path });
        });
        head.appendChild(viewBtn);
      }
    }
    SC.state.runningTools = SC.state.runningTools || {};
    delete SC.state.runningTools[msg.id || SC.state.currentToolId];
    renderToolProgress();
    setTimeout(function() {
      if (card && card.parentNode && card.querySelector('.tool-body') && card.querySelector('.tool-body').scrollHeight > 200) {
        card.classList.add('collapsed');
        var expandBtn = document.createElement('button');
        expandBtn.className = 'tool-expand-btn';
        expandBtn.textContent = '▼';
        expandBtn.addEventListener('click', function(e) {
          e.stopPropagation();
          card.classList.toggle('collapsed');
          expandBtn.textContent = card.classList.contains('collapsed') ? '▼' : '▲';
        });
        head.appendChild(expandBtn);
      }
    }, 5000);
    if (SC.vl) {
      var msgIdx = -1;
      var msgs = SC.$$('#messages .tool-card');
      for (var fi = 0; fi < msgs.length; fi++) {
        if (msgs[fi] === card) { msgIdx = fi; break; }
      }
      if (msgIdx >= 0) SC.vl.refreshItemHeight(msgIdx);
    }
  }

  function updateStopBtn() {
    const btn = SC.$('#btn-stop');
    if (!btn) return;
    if (SC.state.isProcessing) {
      btn.classList.remove('hidden');
    } else {
      btn.classList.add('hidden');
    }
  }

  function showApprovalCard(msg) {
    const el = document.createElement('div');
    el.className = 'approval-card pending';
    el.id = `approval-${msg.id}`;
    el.dataset.toolName = msg.tool || '';

    const color = SC.toolColors[msg.tool] || 'var(--accent)';
    const input = msg.input || {};

    let preview = '';
    if (msg.tool === 'bash' && input.command) {
      preview = `<div class="approval-preview approval-cmd">$ ${SC.escapeHtml(input.command)}</div>`;
    } else if ((msg.tool === 'write_file' || msg.tool === 'edit_file' || msg.tool === 'read_file') && input.path) {
      preview = `<div class="approval-preview approval-file">${SC.escapeHtml(input.path)}</div>`;
    } else if (msg.tool && msg.tool.startsWith('sopa_')) {
      preview = `<div class="approval-preview approval-sopa">⚠ Affects production systems</div>`;
    } else {
      const jsonStr = JSON.stringify(input, null, 2);
      const truncated = jsonStr.length > 500 ? jsonStr.slice(0, 500) + '…' : jsonStr;
      preview = `<div class="approval-preview approval-generic"><pre>${SC.escapeHtml(truncated)}</pre></div>`;
    }

    let reasonHtml = '';
    if (input.reason) {
      reasonHtml = `<div class="approval-reason">${SC.escapeHtml(input.reason)}</div>`;
    }

    el.innerHTML = `
      <div class="approval-head">
        <span class="approval-icon">⚡</span>
        <span class="tool-badge" style="background:${color};color:#000;font-size:11px;font-weight:600;padding:2px 8px;border-radius:4px">${SC.escapeHtml(msg.tool || 'tool')}</span>
        <span class="approval-label">Requires Approval</span>
      </div>
      ${preview}
      ${reasonHtml}
      <div class="approval-actions">
        <button class="approval-btn always" data-action="always_approve">Always Approve</button>
        <button class="approval-btn approve" data-action="approve">Approve</button>
        <button class="approval-btn deny" data-action="deny">Deny</button>
      </div>
      <div class="approval-status pending-status">Waiting for your decision…</div>
    `;

    el.querySelectorAll('.approval-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const action = btn.dataset.action;
        SC.wsSend('tool_approval', { id: msg.id, name: msg.tool, content: action });
        el.classList.remove('pending');
        if (action === 'deny') {
          el.classList.add('denied');
          el.querySelector('.approval-status').className = 'approval-status denied-status';
          el.querySelector('.approval-status').textContent = '✗ Denied';
        } else {
          el.classList.add('approved');
          el.querySelector('.approval-status').className = 'approval-status approved-status';
          el.querySelector('.approval-status').textContent = action === 'always_approve' ? '✓ Always Approved' : '✓ Approved';
        }
        el.querySelectorAll('.approval-btn').forEach(b => b.disabled = true);
      });
    });

    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  function updateAgentCard(msg) {
    const agentId = msg.id || (msg.data && msg.data.agentId) || '';
    const status = msg.status || (msg.data && msg.data.status) || 'running';
    const progress = msg.progress !== undefined ? msg.progress : (msg.data && msg.data.progress) || 0;
    let card = SC.$(`#agent-${agentId}`);
    if (!card) {
      card = document.createElement('div');
      card.className = 'agent-card';
      card.id = `agent-${agentId}`;
      const list = SC.$('#agent-list');
      if (list) list.appendChild(card);
      if (!SC.state.agents.find(a => (a.id || a.agent_id) === agentId)) {
        SC.state.agents.push({ id: agentId, agent_id: agentId, status: status });
      }
    }
    const pct = Math.round(progress * 100);
    const shortId = agentId.slice(0, 8) || 'Agent';
    card.innerHTML = `
      <div class="agent-head"><span class="agent-name">${SC.escapeHtml(shortId)}</span><span class="agent-status">${SC.escapeHtml(status)}</span></div>
      <div class="prog-bar"><div class="prog-fill" style="width:${pct}%"></div></div>
    `;
    if (status === 'done' || status === 'error' || status === 'failed') {
      SC.state.agents = SC.state.agents.filter(a => (a.id || a.agent_id) !== agentId);
      setTimeout(() => { if (card.parentNode) card.remove(); }, 2000);
    }
    SC.updateStats();
  }

  function renderToolProgress() {
    SC.state.runningTools = SC.state.runningTools || {};
    var running = Object.keys(SC.state.runningTools);
    var bar = SC.$('#tool-progress-bar');
    if (running.length === 0) {
      if (bar) bar.remove();
      return;
    }
    if (!bar) {
      bar = document.createElement('div');
      bar.id = 'tool-progress-bar';
      bar.style.cssText = 'position:sticky;top:0;z-index:10;background:var(--accent-bg);border-bottom:1px solid var(--accent-bd);padding:4px 12px;font-size:11px;color:var(--accent);display:flex;align-items:center;gap:8px;';
      var chat = SC.$('#chat');
      if (chat) chat.prepend(bar);
    }
    var tools = running.map(function(id) {
      var t = SC.state.runningTools[id];
      return t ? t.tool : '';
    }).filter(Boolean);
    bar.innerHTML = '<span class="spin" style="width:12px;height:12px;border-width:1.5px;display:inline-block;border:1.5px solid var(--accent);border-top-color:transparent;border-radius:50%;animation:spin .8s linear infinite"></span>' + tools.length + ' tool' + (tools.length > 1 ? 's' : '') + ' running: ' + tools.slice(0, 3).join(', ') + (tools.length > 3 ? '...' : '');
  }

  SC.addToolCard = addToolCard;
  SC.updateToolCard = updateToolCard;
  SC.finishToolCard = finishToolCard;
  SC.showApprovalCard = showApprovalCard;
  SC.updateAgentCard = updateAgentCard;
  SC.updateStopBtn = updateStopBtn;

  var MCP_MARKET = [
    { name: 'filesystem', description: 'File system operations with access controls', category: 'Development', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem'], popular: true },
    { name: 'github', description: 'GitHub API - repos, issues, PRs, search', category: 'Development', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-github'], popular: true },
    { name: 'postgres', description: 'PostgreSQL database queries and schema', category: 'Data', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-postgres'], popular: true },
    { name: 'sqlite', description: 'SQLite database exploration and queries', category: 'Data', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-sqlite'] },
    { name: 'fetch', description: 'Web content fetching and search', category: 'Development', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-fetch'], popular: true },
    { name: 'brave-search', description: 'Web search via Brave Search API', category: 'AI', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-brave-search'] },
    { name: 'memory', description: 'Knowledge graph and persistent memory', category: 'AI', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-memory'], popular: true },
    { name: 'puppeteer', description: 'Browser automation via Puppeteer', category: 'Operations', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-puppeteer'] },
    { name: 'slack', description: 'Slack messaging and channel management', category: 'Communication', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-slack'] },
    { name: 'google-maps', description: 'Google Maps directions, places, geocoding', category: 'Productivity', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-google-maps'] },
    { name: 'sequential-thinking', description: 'Structured problem-solving and reasoning', category: 'AI', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-sequential-thinking'] },
    { name: 'everything', description: 'MCP test server with all features', category: 'Development', type: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-everything'] },
  ];

  var MCP_CATEGORY_COLORS = {
    Development: '#3b82f6',
    Data: '#06b6d4',
    AI: '#8b5cf6',
    Operations: '#f59e0b',
    Communication: '#ef4444',
    Productivity: '#10b981'
  };

  var MCP_CATEGORIES = ['All', 'Development', 'Data', 'AI', 'Operations', 'Communication', 'Productivity'];

  function renderMCPCatalog() {
    var installedNames = {};
    var servers = SC.state.mcpServers || [];
    for (var i = 0; i < servers.length; i++) {
      installedNames[servers[i].name] = true;
    }

    var catalogData = SC.state.mcpCatalog || MCP_MARKET;

    var chipsEl = SC.$('#mcp-category-chips');
    if (chipsEl) {
      var chipsHtml = '';
      var activeCategory = SC.state.mcpMarketCategory || 'All';
      for (var c = 0; c < MCP_CATEGORIES.length; c++) {
        var cat = MCP_CATEGORIES[c];
        chipsHtml += '<button class="mcp-category-chip' + (cat === activeCategory ? ' active' : '') + '" data-category="' + cat + '">' + cat + '</button>';
      }
      chipsEl.innerHTML = chipsHtml;
      chipsEl.querySelectorAll('.mcp-category-chip').forEach(function(chip) {
        chip.addEventListener('click', function() {
          SC.state.mcpMarketCategory = this.dataset.category;
          renderMCPCatalog();
        });
      });
    }

    var gridEl = SC.$('#mcp-market-grid');
    if (!gridEl) return;

    var searchTerm = (SC.$('#mcp-catalog-search-view') || {}).value || '';
    var activeCategory = SC.state.mcpMarketCategory || 'All';
    var filtered = catalogData.filter(function(item) {
      var matchSearch = !searchTerm || item.name.toLowerCase().indexOf(searchTerm.toLowerCase()) >= 0 || item.description.toLowerCase().indexOf(searchTerm.toLowerCase()) >= 0;
      var matchCategory = activeCategory === 'All' || item.category === activeCategory;
      return matchSearch && matchCategory;
    });

    var html = '';
    for (var j = 0; j < filtered.length; j++) {
      var item = filtered[j];
      var isInstalled = !!installedNames[item.name];
      var catColor = MCP_CATEGORY_COLORS[item.category] || '#6a6a72';
      html += '<div class="mcp-card' + (isInstalled ? ' installed' : '') + '">';
      html += '<div class="mcp-card-name">' + SC.escapeHtml(item.name) + '</div>';
      html += '<div class="mcp-card-desc">' + SC.escapeHtml(item.description) + '</div>';
      html += '<div class="mcp-card-footer">';
      html += '<span class="mcp-card-category" style="background:' + catColor + '">' + SC.escapeHtml(item.category) + '</span>';
      if (isInstalled) {
        html += '<span class="mcp-card-install">✓ Installed</span>';
      } else {
        html += '<button class="mcp-card-install" data-name="' + SC.escapeHtml(item.name) + '" data-type="' + SC.escapeHtml(item.type) + '" data-command="' + SC.escapeHtml(item.command) + '" data-args="' + SC.escapeHtml(JSON.stringify(item.args)) + '">Install</button>';
      }
      html += '</div></div>';
    }

    if (filtered.length === 0) {
      html = '<div class="empty-state"><span class="empty-desc">No servers found</span></div>';
    }

    gridEl.innerHTML = html;

    gridEl.querySelectorAll('.mcp-card-install[data-name]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var name = this.dataset.name;
        var type = this.dataset.type;
        var command = this.dataset.command;
        var args;
        try { args = JSON.parse(this.dataset.args); } catch(e) { args = []; }
        SC.wsSend('mcp_add', { name: name, type: type, command: command, args: args });
        SC.toast('Installing ' + name + '...', 'success');
      });
    });
  }

  function renderMCPMarketplace() {
    if (!SC.state.mcpServers) SC.state.mcpServers = [];
    if (!SC.state.mcpMarketCategory) SC.state.mcpMarketCategory = 'All';

    var installedEl = SC.$('#mcp-servers-panel');
    if (installedEl) {
      var servers = SC.state.mcpServers || [];
      if (servers.length === 0) {
        installedEl.innerHTML = '<div class="empty-state"><span class="empty-desc">No MCP servers installed</span></div>';
      } else {
        var instHtml = '<div class="mcp-market-grid">';
        for (var i = 0; i < servers.length; i++) {
          var s = servers[i];
          var catColor = '#6a6a72';
          for (var k = 0; k < MCP_MARKET.length; k++) {
            if (MCP_MARKET[k].name === s.name) {
              catColor = MCP_CATEGORY_COLORS[MCP_MARKET[k].category] || catColor;
              break;
            }
          }
          instHtml += '<div class="mcp-card installed">';
          instHtml += '<div class="mcp-card-name">' + SC.escapeHtml(s.name) + '</div>';
          instHtml += '<div class="mcp-card-desc">' + SC.escapeHtml(s.description || s.command + ' ' + (s.args || []).join(' ')) + '</div>';
          instHtml += '<div class="mcp-card-footer">';
          instHtml += '<span class="mcp-card-category" style="background:' + catColor + '">' + SC.escapeHtml(s.type || 'stdio') + '</span>';
          instHtml += '<button class="mcp-card-install mcp-remove-btn" data-name="' + SC.escapeHtml(s.name) + '">Remove</button>';
          instHtml += '</div></div>';
        }
        instHtml += '</div>';
        installedEl.innerHTML = instHtml;

        installedEl.querySelectorAll('.mcp-remove-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            SC.wsSend('mcp_remove', { name: this.dataset.name });
            SC.toast('Removing ' + this.dataset.name + '...', 'success');
          });
        });
      }
    }

    renderMCPCatalog();

    var searchEl = SC.$('#mcp-catalog-search-view');
    if (searchEl && !searchEl._mcpBound) {
      searchEl._mcpBound = true;
      searchEl.addEventListener('input', function() {
        renderMCPCatalog();
      });
    }

    var tabs = document.querySelectorAll('.mcp-view-tab');
    tabs.forEach(function(tab) {
      tab.addEventListener('click', function() {
        tabs.forEach(function(t) { t.classList.remove('active'); });
        this.classList.add('active');
        var view = this.dataset.view;
        var instView = SC.$('#mcp-servers-panel');
        var discView = SC.$('#mcp-catalog-panel');
        if (view === 'servers' || view === 'installed') {
          if (instView) instView.classList.remove('hidden');
          if (discView) discView.classList.add('hidden');
        } else {
          if (instView) instView.classList.add('hidden');
          if (discView) discView.classList.remove('hidden');
        }
      });
    });
  }

  SC.MCP_MARKET = MCP_MARKET;
  SC.renderMCPCatalog = renderMCPCatalog;
  SC.renderMCPMarketplace = renderMCPMarketplace;
})();
