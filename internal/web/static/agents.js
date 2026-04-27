// SmartClaw - Agents
(function() {
  'use strict';

  function renderAgentList() {
    try {
    const list = SC.$('#agent-list');
    if (!list) return;
    list.innerHTML = '';
    if (!SC.state.agents || SC.state.agents.length === 0) {
      list.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No active agents</div>';
      SC.updateStats();
      return;
    }
    SC.state.agents.forEach(agent => {
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
      list.appendChild(el);
    });

    list.querySelectorAll('.agent-stop-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const agentId = btn.dataset.agentId;
        if (agentId) SC.wsSend('agent_stop', { agent_id: agentId });
      });
    });

    list.querySelectorAll('.agent-view-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const agentId = btn.dataset.agentId;
        if (agentId) SC.wsSend('agent_output', { agent_id: agentId });
      });
    });

    SC.updateStats();
    } catch (err) {
      console.error('[renderAgentList Error]', err);
      SC.showErrorBanner('Agent list error: ' + err.message, renderAgentList);
    }
  }

  function renderAgentOutput(data) {
    if (!data) return;
    const el = document.createElement('div');
    el.className = 'message cmd-result';
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const agentId = (data.agent_id || '').slice(0, 8);
    const status = data.status || 'unknown';
    const output = data.output || '(no output)';
    const exitCode = data.exit_code;
    const exitInfo = exitCode !== undefined && exitCode !== 0 ? ` (exit: ${exitCode})` : '';
    el.innerHTML = `<div class="msg-role" style="color:var(--accent)">Agent ${SC.escapeHtml(agentId)} — ${SC.escapeHtml(status)}${exitInfo}</div><div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px;max-height:400px;overflow-y:auto">${SC.escapeHtml(output)}</div><div class="msg-ts">${ts}</div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  SC.renderAgentList = renderAgentList;
  SC.renderAgentOutput = renderAgentOutput;
})();
