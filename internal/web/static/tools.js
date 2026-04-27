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
    el.innerHTML = `<div class="tool-head"><span class="tool-dot running"></span><span class="tool-badge" style="background:${color};color:#000;font-size:11px;font-weight:600;padding:2px 8px;border-radius:4px;margin-right:6px">${SC.escapeHtml(msg.tool || 'tool')}</span></div>${detail}<div class="tool-body"></div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  function updateToolCard(msg) {
    const cardId = msg.id || SC.state.currentToolId;
    const body = SC.$(`#tool-${cardId} .tool-body`);
    if (!body) return;
    const toolName = (SC.$(`#tool-${cardId}`) || {}).dataset?.toolName || '';
    const output = msg.output || '';
    if (toolName === 'browser_screenshot' && output.length > 200 && /^[A-Za-z0-9+/=\s]+$/.test(output.slice(0, 500))) {
      const img = document.createElement('img');
      img.src = `data:image/png;base64,${output.replace(/\s/g, '')}`;
      img.className = 'screenshot-img';
      img.style.cssText = 'max-width:100%;border-radius:4px;border:1px solid var(--bd);margin:4px 0';
      body.innerHTML = '';
      body.appendChild(img);
    } else {
      body.textContent += output;
    }
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

  SC.addToolCard = addToolCard;
  SC.updateToolCard = updateToolCard;
  SC.finishToolCard = finishToolCard;
  SC.showApprovalCard = showApprovalCard;
  SC.updateAgentCard = updateAgentCard;
  SC.updateStopBtn = updateStopBtn;
})();
