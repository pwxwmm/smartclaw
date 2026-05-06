// SmartClaw - WebSocket Connection
(function() {
  'use strict';

  let reconnectTimer = null;
  let authCheckDone = false;

  function getStoredToken() {
    try { return localStorage.getItem('smartclaw-token'); } catch { return null; }
  }

  function storeToken(token) {
    try { localStorage.setItem('smartclaw-token', token); } catch {}
  }

  function clearToken() {
    try { localStorage.removeItem('smartclaw-token'); } catch {}
  }

  function showLoginScreen() {
    let overlay = SC.$('#auth-overlay');
    if (overlay) return;

    overlay = document.createElement('div');
    overlay.id = 'auth-overlay';
    overlay.style.cssText = 'position:fixed;inset:0;z-index:10000;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,0.85);';
    overlay.innerHTML =
      '<div style="background:var(--bg-1);border:1px solid var(--border);border-radius:12px;padding:32px;width:340px;display:flex;flex-direction:column;gap:16px">' +
        '<h2 style="margin:0;font-size:18px;color:var(--tx-1);text-align:center">SmartClaw Login</h2>' +
        '<input id="auth-api-key" type="password" placeholder="API Key" style="padding:10px 12px;border:1px solid var(--border);border-radius:6px;background:var(--bg-0);color:var(--tx-1);font-size:14px;outline:none" />' +
        '<button id="auth-submit" style="padding:10px;border:none;border-radius:6px;background:#8b5cf6;color:#fff;font-size:14px;font-weight:600;cursor:pointer">Connect</button>' +
        '<div id="auth-error" style="color:#f87171;font-size:13px;text-align:center;min-height:18px"></div>' +
      '</div>';
    document.body.appendChild(overlay);

    SC.$('#auth-submit').addEventListener('click', doLogin);
    SC.$('#auth-api-key').addEventListener('keydown', function(e) {
      if (e.key === 'Enter') doLogin();
    });
  }

  function hideLoginScreen() {
    const overlay = SC.$('#auth-overlay');
    if (overlay) overlay.remove();
  }

  async function doLogin() {
    const input = SC.$('#auth-api-key');
    const errEl = SC.$('#auth-error');
    const submitBtn = SC.$('#auth-submit');
    if (!input || !input.value.trim()) {
      if (errEl) errEl.textContent = 'Please enter an API key';
      return;
    }

    submitBtn.disabled = true;
    submitBtn.textContent = 'Connecting...';
    if (errEl) errEl.textContent = '';

    try {
      const resp = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ api_key: input.value.trim() })
      });
      const data = await resp.json();
      if (!resp.ok) {
        if (errEl) errEl.textContent = data.error || 'Login failed';
        submitBtn.disabled = false;
        submitBtn.textContent = 'Connect';
        return;
      }
      storeToken(data.token);
      hideLoginScreen();
      wsConnect();
    } catch (err) {
      if (errEl) errEl.textContent = 'Connection error';
      submitBtn.disabled = false;
      submitBtn.textContent = 'Connect';
    }
  }

  async function checkAuthAndConnect() {
    if (authCheckDone) { wsConnect(); return; }
    authCheckDone = true;

    try {
      const resp = await fetch('/api/auth/status', { credentials: 'same-origin' });
      const data = await resp.json();
      if (data.authenticated) {
        SC.state._noAuth = true;
        SC.state.authenticated = true;
        wsConnect();
        return;
      }
    } catch {}

    const token = getStoredToken();
    if (token) {
      wsConnect();
      return;
    }

    showLoginScreen();
  }

  function wsConnect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = getStoredToken();
    const wsUrl = token
      ? `${proto}//${location.host}/ws?token=${encodeURIComponent(token)}`
      : `${proto}//${location.host}/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      SC.setState('connected', true);
      SC.$('#connection-status').dataset.status = 'on';
      const reconnecting = SC.$('.conn-reconnecting');
      if (reconnecting) reconnecting.remove();
      const splash = SC.$('#splash');
      if (splash) splash.remove();
      SC.toast('Connected', 'success');
      wsSend('file_tree', { path: '.' });
      wsSend('session_list', {});
      try {
        const savedSession = localStorage.getItem('smartclaw-active-session');
        if (savedSession) wsSend('session_load', { id: savedSession });
      } catch {}
      SC.state.authenticated = true;
      if (typeof SC.runDeferredInits === 'function') SC.runDeferredInits();
      wsSend('get_recent_projects', {});
    };

    ws.onclose = (ev) => {
      SC.setState('connected', false);
      SC.$('#connection-status').dataset.status = 'off';

      if (ev.code === 1008 || (ev.code === 1006 && !SC.state.authenticated)) {
        SC.state.authenticated = false;
        clearToken();
        showLoginScreen();
        return;
      }

      let reconnecting = SC.$('.conn-reconnecting');
      if (!reconnecting) {
        reconnecting = document.createElement('span');
        reconnecting.className = 'conn-reconnecting';
        SC.$('.topbar-left').appendChild(reconnecting);
      }
      let countdown = 3;
      reconnecting.innerHTML = `<span class="spin"></span>Reconnecting ${countdown}s`;
      if (reconnectTimer) clearInterval(reconnectTimer);
      reconnectTimer = setInterval(() => {
        countdown--;
        if (countdown <= 0) {
          clearInterval(reconnectTimer);
          reconnectTimer = null;
          wsConnect();
        } else {
          reconnecting.innerHTML = `<span class="spin"></span>Reconnecting ${countdown}s`;
        }
      }, 1000);
    };

    ws.onerror = () => { ws.close(); };

    ws.onmessage = (e) => {
      var data = typeof e.data === 'string' ? e.data : '';
      var lines = data.split('\n');
      for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line) continue;
        try {
          var msg = JSON.parse(line);
          handleWSMessage(msg);
        } catch(err) {
          console.error('WS parse error:', err);
          if (typeof SC !== 'undefined' && SC.showErrorBanner) {
            SC.showErrorBanner('WS message error: ' + err.message, null);
          }
        }
      }
    };

    SC.state.ws = ws;
  }

  function wsSend(type, data) {
    if (SC.state.ws && SC.state.ws.readyState === WebSocket.OPEN) {
      SC.state.ws.send(JSON.stringify({ type, ...data }));
    }
  }

  function handleWSMessage(msg) {
    try {
    switch(msg.type) {
      case 'token':
        SC.appendToken(msg.content);
        break;
      case 'thinking':
        break;
      case 'tool_start':
        SC.addToolCard(msg);
        break;
      case 'tool_output':
        SC.updateToolCard(msg);
        break;
      case 'tool_end':
        SC.finishToolCard(msg);
        break;
      case 'tool_approval_request':
        SC.showApprovalCard(msg);
        break;
      case 'aborted':
        SC.setState('isProcessing', false);
        SC.updateStopBtn();
        SC.toast('Request aborted', 'warning');
        break;
      case 'agent_status':
        if (msg.data && msg.data.agentId) {
          SC.updateAgentCard({
            id: msg.data.agentId,
            status: msg.data.status,
            progress: msg.data.status === 'done' ? 1 : msg.data.status === 'error' ? 0 : 0.3
          });
        } else {
          SC.updateAgentCard(msg);
        }
        break;
      case 'agent_list':
        SC.state.agents = (msg.data && msg.data.agents) || [];
        SC.renderAgentList();
        break;
      case 'agent_stop':
        SC.toast('Agent stopped: ' + (msg.data?.agent_id || ''), 'success');
        wsSend('agent_list', {});
        break;
      case 'agent_output':
        SC.renderAgentOutput(msg.data);
        break;
      case 'warroom_list':
        if (SC.warroom) SC.warroom.handleList(msg.data || {});
        break;
      case 'warroom_started':
        if (SC.warroom) SC.warroom.handleStarted(msg.data || {});
        break;
      case 'warroom_status':
        if (SC.warroom) SC.warroom.handleStatus(msg.data || {});
        break;
      case 'warroom_stopped':
        if (SC.warroom) SC.warroom.handleStopped(msg.data || {});
        break;
      case 'warroom_agent_status':
        if (SC.warroom) SC.warroom.handleAgentStatus(msg.data || {});
        break;
      case 'warroom_findings':
        if (SC.warroom) SC.warroom.handleFindings(msg.data || {});
        break;
      case 'warroom_timeline':
        if (SC.warroom) SC.warroom.handleTimeline(msg.data || {});
        break;
      case 'warroom_update':
        if (SC.warroom) SC.warroom.handleUpdate(msg.data || {});
        break;
      case 'warroom_blackboard_update':
        if (SC.warroom) SC.warroom.handleBlackboardUpdate(msg.data || {});
        break;
      case 'warroom_handoff':
        if (SC.warroom) SC.warroom.handleHandoff(msg.data || {});
        break;
      case 'warroom_confidence_change':
        if (SC.warroom) SC.warroom.handleConfidenceChange(msg.data || {});
        break;
      case 'done':
        SC.finishMessage(msg);
        break;
      case 'error':
        SC.setState('isProcessing', false);
        SC.updateStopBtn();
        SC.toast(msg.message, 'error');
        break;
      case 'file_tree':
        SC.state.flatFiles = SC.flattenFileTree(msg.tree || [], '');
        SC.state.fileTreeData = msg.tree || [];
        SC.renderFileTree(msg.tree || []);
        SC.requestGitStatus();
        break;
      case 'file_content':
        SC.openFileTab(msg.content, SC.state.ui.currentFile);
        break;
      case 'git_status':
        if (msg.data && typeof msg.data === 'object') {
          SC.state.gitStatus = msg.data;
        } else {
          SC.state.gitStatus = {};
        }
        if (SC.state.fileTreeData && SC.state.fileTreeData.length > 0) {
          SC.renderFileTree(SC.state.fileTreeData);
        }
        break;
      case 'session_list':
        SC.renderSessions(msg.sessions || []);
        break;
      case 'session_active':
        SC.state.ui.currentSessionId = msg.id;
        try { localStorage.setItem('smartclaw-active-session', msg.id); } catch {}
        break;
      case 'session_created':
        SC.state.ui.currentSessionId = msg.id;
        try { localStorage.setItem('smartclaw-active-session', msg.id); } catch {}
        if (SC.vl) {
          SC.vl.clear();
        } else {
          SC.$('#messages').innerHTML = '';
        }
        SC.state.messages = [];
        SC.toast('Session created', 'success');
        wsSend('session_list', {});
        break;
      case 'session_loaded':
        SC.loadSessionMessages(msg);
        break;
      case 'session_deleted':
        SC.toast('Session deleted', 'success');
        if (SC.state.ui.currentSessionId === msg.id) {
          SC.state.ui.currentSessionId = null;
          try { localStorage.removeItem('smartclaw-active-session'); } catch {}
          if (SC.vl) {
            SC.vl.clear();
          } else {
            SC.$('#messages').innerHTML = '';
          }
          SC.state.messages = [];
        }
        wsSend('session_list', {});
        break;
      case 'model_changed':
        SC.toast(msg.message, 'success');
        SC.state.settings.model = msg.message.split(' ').pop();
        SC.$('#current-model').textContent = SC.state.settings.model;
        break;
      case 'voice_transcript':
        SC.$('#input').value = msg.text;
        break;
      case 'cmd_result':
        SC.addCmdResult(msg.content);
        break;
      case 'skill_list':
        SC.state.skills = msg.data || [];
        SC.renderSkillList();
        break;
      case 'skill_detail':
        SC.showSkillDetail(msg.data);
        break;
      case 'skill_toggle':
        const toggled = (SC.state.skills || []).find(s => s.name === msg.data?.name);
        if (toggled) { toggled.enabled = msg.data?.status === 'enabled'; SC.renderSkillList(); }
        SC.toast(`Skill ${msg.data?.name} ${msg.data?.status}`, 'success');
        break;
      case 'skill_search':
        SC.state.skills = msg.data || [];
        SC.renderSkillList();
        break;
      case 'skill_create':
        if (msg.data?.success) {
          SC.toast('Skill created: ' + msg.data.name, 'success');
          wsSend('skill_list', {});
          SC.closeSkillCreateForm();
        } else {
          SC.toast(msg.data?.error || 'Skill creation failed', 'error');
        }
        break;
      case 'memory_layers':
        SC.state.memoryLayers = msg.data || {};
        SC.renderMemoryLayers();
        break;
      case 'memory_search':
        SC.renderMemorySearchResults(msg.data || []);
        break;
      case 'memory_recall':
        SC.renderMemoryRecall(msg.data);
        break;
      case 'memory_store':
        SC.toast('Memory stored', 'success');
        break;
      case 'memory_update':
        if (msg.data?.success) SC.toast('Memory updated', 'success');
        else SC.toast('Memory update failed', 'error');
        break;
      case 'memory_stats':
        SC.state.memoryStats = msg.data || {};
        SC.renderMemoryStats();
        break;
      case 'memory_observations':
        SC.state.userObservations = msg.data || [];
        SC.renderUserObservations();
        break;
      case 'memory_observation_delete':
        if (msg.data?.success) SC.toast('Observation deleted', 'success');
        else SC.toast('Failed to delete observation', 'error');
        break;
      case 'skill_edit':
        if (msg.data?.success) {
          SC.toast('Skill updated: ' + msg.data.name, 'success');
          SC.state.editingSkill = null;
          SC.renderSkillMemoryList();
        } else {
          SC.toast('Skill edit failed', 'error');
        }
        break;
      case 'chat_edit':
        SC.toast(msg.message || 'Message edited on server', 'success');
        break;
      case 'session_fragments':
        SC.state.sessionFragments = msg.data || [];
        SC.renderSessionFragments();
        break;
      case 'wiki_search':
        SC.renderWikiResults(msg.data);
        break;
      case 'wiki_pages':
        SC.state.wikiEnabled = msg.data?.enabled || false;
        SC.state.wikiPages = msg.data?.pages || [];
        SC.renderWikiPages();
        break;
      case 'wiki_page_content':
        SC.renderWikiPageContent(msg.data);
        break;
      case 'project_changed':
        SC.state.projectPath = msg.path;
        SC.state.projectName = msg.message || msg.name;
        var nameEl = SC.$('#project-name');
        if (nameEl) nameEl.textContent = msg.message || msg.name;
        if (typeof SC.updateProjectAvatar === 'function') SC.updateProjectAvatar();
        if (typeof SC.renderRailProjects === 'function') SC.renderRailProjects();
        SC.toast('Project changed: ' + (msg.message || msg.name), 'success');
        break;
      case 'recent_projects':
        SC.state.recentProjects = msg.data || [];
        SC.renderRecentProjects();
        if (typeof SC.renderDirPickerRecent === 'function') SC.renderDirPickerRecent();
        break;
      case 'browse_dirs':
        if (typeof SC.renderDirPickerEntries === 'function') SC.renderDirPickerEntries(msg.data || {});
        break;
      case 'mcp_list':
        SC.state.mcpServers = msg.data || [];
        if (typeof SC.renderMCPServers === 'function') SC.renderMCPServers();
        if (typeof SC.renderMCPInstalled === 'function') SC.renderMCPInstalled();
        break;
      case 'mcp_catalog':
        SC.state.mcpCatalog = msg.data || [];
        if (typeof SC.renderMCPCatalogView === 'function') SC.renderMCPCatalogView();
        if (typeof SC.renderMCPSidebarCatalog === 'function') SC.renderMCPSidebarCatalog();
        break;
      case 'mcp_add':
        if (msg.data && msg.data.success) {
          SC.toast('Server added: ' + (msg.data.name || ''), 'success');
        } else {
          SC.toast(msg.data?.error || 'Failed to add server', 'error');
        }
        SC.wsSend('mcp_list', {});
        SC.wsSend('mcp_catalog', {});
        break;
      case 'mcp_remove':
        if (msg.data && msg.data.success) {
          SC.toast('Server removed', 'success');
        } else {
          SC.toast(msg.data?.error || 'Failed to remove server', 'error');
        }
        SC.wsSend('mcp_list', {});
        SC.wsSend('mcp_catalog', {});
        break;
      case 'mcp_start':
        if (msg.data && msg.data.success) {
          SC.toast('Server started: ' + (msg.data.name || ''), 'success');
        } else {
          SC.toast(msg.data?.error || 'Failed to start server', 'error');
        }
        SC.wsSend('mcp_list', {});
        break;
      case 'mcp_stop':
        if (msg.data && msg.data.success) {
          SC.toast('Server stopped: ' + (msg.data.name || ''), 'success');
        } else {
          SC.toast(msg.data?.error || 'Failed to stop server', 'error');
        }
        SC.wsSend('mcp_list', {});
        break;
      case 'mcp_tools':
        if (msg.data) {
          var tName = msg.data.name || '';
          if (!SC.state._mcpTools) SC.state._mcpTools = {};
          SC.state._mcpTools[tName] = msg.data.tools || [];
          if (typeof SC.renderMCPToolsData === 'function') SC.renderMCPToolsData(msg.data);
          if (typeof SC.renderMCPInstalled === 'function') SC.renderMCPInstalled();
        }
        break;
      case 'mcp_resources':
        if (msg.data) {
          var rName = msg.data.name || '';
          if (!SC.state._mcpResources) SC.state._mcpResources = {};
          SC.state._mcpResources[rName] = msg.data.resources || [];
          if (typeof SC.renderMCPResourcesData === 'function') SC.renderMCPResourcesData(msg.data);
          if (typeof SC.renderMCPInstalled === 'function') SC.renderMCPInstalled();
        }
        break;
    }
      var listeners = SC.state._wsListeners && SC.state._wsListeners[msg.type];
      if (listeners) {
        listeners.slice().forEach(function(fn) { try { fn(msg); } catch(e) { console.error(e); } });
      }
    } catch (err) {
      console.error('[WS Handler Error]', err);
      if (typeof SC !== 'undefined' && SC.showErrorBanner) {
        SC.showErrorBanner('WS handler error (' + msg.type + '): ' + err.message, function() { handleWSMessage(msg); });
      }
    }
  }

  SC.wsConnect = checkAuthAndConnect;
  SC.wsSend = wsSend;
  SC.handleWSMessage = handleWSMessage;
})();
