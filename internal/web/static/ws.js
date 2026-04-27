// SmartClaw - WebSocket Connection
(function() {
  'use strict';

  let reconnectTimer = null;

  function wsConnect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);

    ws.onopen = () => {
      SC.setState('connected', true);
      SC.$('#connection-status').dataset.status = 'on';
      const reconnecting = SC.$('.conn-reconnecting');
      if (reconnecting) reconnecting.remove();
      SC.toast('Connected', 'success');
      wsSend('file_tree', { path: '.' });
      wsSend('session_list', {});
      try {
        const savedSession = localStorage.getItem('smartclaw-active-session');
        if (savedSession) wsSend('session_load', { id: savedSession });
      } catch {}
    };

    ws.onclose = () => {
      SC.setState('connected', false);
      SC.$('#connection-status').dataset.status = 'off';
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
      try {
        const msg = JSON.parse(e.data);
        handleWSMessage(msg);
      } catch(err) {
        console.error('WS parse error:', err);
        if (typeof SC !== 'undefined' && SC.showErrorBanner) {
          SC.showErrorBanner('WS message error: ' + err.message, null);
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
        SC.appendThinking(msg.content);
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
        SC.renderFileTree(msg.tree || []);
        break;
      case 'file_content':
        SC.openFileDrawer(msg.content, SC.state.ui.currentFile);
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
        SC.$('#messages').innerHTML = '';
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
          SC.$('#messages').innerHTML = '';
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
    }
    } catch (err) {
      console.error('[WS Handler Error]', err);
      if (typeof SC !== 'undefined' && SC.showErrorBanner) {
        SC.showErrorBanner('WS handler error (' + msg.type + '): ' + err.message, function() { handleWSMessage(msg); });
      }
    }
  }

  SC.wsConnect = wsConnect;
  SC.wsSend = wsSend;
  SC.handleWSMessage = handleWSMessage;
})();
