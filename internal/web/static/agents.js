// SmartClaw - Agents
(function() {
  'use strict';

  function createAgentCard(agent) {
    const el = document.createElement('div');
    el.className = 'agent-card';
    el.id = `agent-${agent.agent_id || agent.id}`;
    const shortId = (agent.agent_id || agent.id || '').slice(0, 8);
    const agentType = agent.type || 'explore';
    const status = agent.status || 'running';
    const isRunning = status === 'running' || status === 'starting';
    const isDone = status === 'completed' || status === 'failed' || status === 'stopped';
    const statusClass = isRunning ? 'agent-status-running' : isDone ? 'agent-status-done' : 'agent-status-idle';
    const pct = isDone ? 100 : isRunning ? 30 : 0;
    const elapsed = agent.started_at ? Math.round((Date.now() - new Date(agent.started_at).getTime()) / 1000) + 's' : '';

    let actions = '';
    if (isRunning) {
      actions += `<button class="agent-action-btn agent-stop-btn" data-agent-id="${SC.escapeHtml(agent.agent_id || agent.id || '')}" title="Stop agent">&#9632;</button>`;
    }
    if (isDone) {
      actions += `<button class="agent-action-btn agent-view-btn" data-agent-id="${SC.escapeHtml(agent.agent_id || agent.id || '')}" title="View output">&#9654;</button>`;
    }

    el.innerHTML = `
      <div class="agent-head">
        <span class="agent-name">${SC.escapeHtml(shortId)}</span>
        <span class="agent-type-badge">${SC.escapeHtml(agentType)}</span>
        <span class="agent-status ${statusClass}">${SC.escapeHtml(status)}</span>
      </div>
      ${elapsed ? `<div class="agent-elapsed">${SC.escapeHtml(elapsed)}</div>` : ''}
      <div class="prog-bar"><div class="prog-fill" style="width:${pct}%"></div></div>
      <div class="agent-actions">${actions}</div>
    `;

    el.querySelector('.agent-stop-btn')?.addEventListener('click', (e) => {
      e.stopPropagation();
      const agentId = (agent.agent_id || agent.id || '');
      if (agentId) SC.wsSend('agent_stop', { agent_id: agentId });
    });

    el.querySelector('.agent-view-btn')?.addEventListener('click', (e) => {
      e.stopPropagation();
      const agentId = (agent.agent_id || agent.id || '');
      if (agentId) SC.wsSend('agent_output', { agent_id: agentId });
    });

    return el;
  }

  function renderAgentListInto(container) {
    if (!container) return;
    container.innerHTML = '';
    if (!SC.state.agents || SC.state.agents.length === 0) {
      SC.showEmptyState(container,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>',
        'No active agents',
        'Agents will appear here when running tasks.'
      );
      return;
    }
    SC.state.agents.forEach(agent => {
      container.appendChild(createAgentCard(agent));
    });
    if (typeof SC.applyListStagger === 'function') SC.applyListStagger(container, '.agent-card');
  }

  function renderAgentList() {
    try {
    renderAgentListInto(SC.$('#agent-list'));
    renderAgentListInto(SC.$('#agent-list-view'));
    SC.updateStats();
    } catch (err) {
      console.error('[renderAgentList Error]', err);
      SC.showErrorBanner('Agent list error: ' + err.message, renderAgentList);
    }
  }

  function renderAgentOutput(data) {
    if (!data) return;
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const agentId = (data.agent_id || '').slice(0, 8);
    const status = data.status || 'unknown';
    const output = data.output || '(no output)';
    const exitCode = data.exit_code;
    const exitInfo = exitCode !== undefined && exitCode !== 0 ? ' (exit: ' + exitCode + ')' : '';
    var el = SC.renderMessageCard('cmd-result msg-group-start', SC.escapeHtml(output), ts, {
      roleLabel: 'Agent ' + agentId + ' \u2014 ' + status + exitInfo,
      style: 'font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px;max-height:400px;overflow-y:auto'
    });
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  SC.renderAgentList = renderAgentList;
  SC.renderAgentOutput = renderAgentOutput;

  function renderAgentSwitcher() {
    const container = SC.$('#agent-switcher');
    if (!container) return;

    const agents = [
      { id: 'build', label: 'Build', desc: 'Full-stack coding agent', color: '#3b82f6' },
      { id: 'plan', label: 'Plan', desc: 'Read-only planning & analysis', color: '#a78bfa' },
      { id: 'debug', label: 'Debug', desc: 'Error diagnosis & fixing', color: '#f87171' },
      { id: 'review', label: 'Review', desc: 'Code review & quality', color: '#34d399' },
      { id: 'refactor', label: 'Refactor', desc: 'Code restructuring & cleanup', color: '#fbbf24' },
      { id: 'ops', label: 'Ops', desc: 'DevOps & infrastructure', color: '#f97316' },
      { id: 'security', label: 'Security', desc: 'Vulnerability & compliance', color: '#ec4899' },
      { id: 'docs', label: 'Docs', desc: 'Documentation & comments', color: '#60a5fa' }
    ];

    const current = SC.state.currentAgent || 'build';

    container.innerHTML = `
      <div class="agent-switcher">
        <div class="agent-switcher-current" id="agent-switcher-toggle" title="Switch agent (Ctrl+T)">
          <span class="agent-dot" style="background:${agents.find(a => a.id === current)?.color || '#6b7280'}"></span>
          <span class="agent-switcher-name">${SC.escapeHtml(agents.find(a => a.id === current)?.label || current)}</span>
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 5l3 3 3-3"/></svg>
        </div>
        <div class="agent-switcher-dropdown" id="agent-switcher-dropdown" style="display:none">
          ${agents.map(a => `
            <div class="agent-switcher-option${a.id === current ? ' active' : ''}" data-agent="${a.id}">
              <span class="agent-dot" style="background:${a.color}"></span>
              <div class="agent-switcher-option-info">
                <span class="agent-switcher-option-name">${SC.escapeHtml(a.label)}</span>
                <span class="agent-switcher-option-desc">${SC.escapeHtml(a.desc)}</span>
              </div>
            </div>
          `).join('')}
        </div>
      </div>
    `;

    const toggle = SC.$('#agent-switcher-toggle');
    const dropdown = SC.$('#agent-switcher-dropdown');

    if (toggle && dropdown) {
      toggle.addEventListener('click', () => {
        const visible = dropdown.style.display !== 'none';
        dropdown.style.display = visible ? 'none' : 'block';
      });

      dropdown.querySelectorAll('.agent-switcher-option').forEach(opt => {
        opt.addEventListener('click', () => {
          const agentId = opt.dataset.agent;
          if (agentId && agentId !== current) {
            SC.wsSend('agent_switch', { agent_type: agentId });
            SC.state.currentAgent = agentId;
            dropdown.style.display = 'none';
            renderAgentSwitcher();
          }
        });
      });

      document.addEventListener('click', (e) => {
        if (!container.contains(e.target)) {
          dropdown.style.display = 'none';
        }
      });
    }
  }

  SC.renderAgentSwitcher = renderAgentSwitcher;
})();
