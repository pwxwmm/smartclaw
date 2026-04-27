// SmartClaw - Sessions
(function() {
  'use strict';

  function renderSessions(sessions) {
    try {
    const list = SC.$('#session-list');
    list.innerHTML = '';
    if (sessions.length === 0) {
      SC.showEmptyState(list,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>',
        'No sessions yet',
        'Start a conversation to create your first session.'
      );
      SC.setState('sessions', sessions);
      SC.$('#s-total-sessions').textContent = '0';
      return;
    }

    const searchTerm = (SC.$('#session-search')?.value || '').toLowerCase();
    const filtered = searchTerm
      ? sessions.filter(s => (s.title || 'Untitled').toLowerCase().includes(searchTerm))
      : sessions;

    if (filtered.length === 0) {
      SC.showEmptyState(list,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>',
        'No sessions found',
        'Try a different search term.'
      );
      SC.setState('sessions', sessions);
      SC.$('#s-total-sessions').textContent = sessions.length;
      return;
    }

    filtered.forEach(s => {
      const el = document.createElement('div');
      el.className = 'session-item' + (s.id === SC.state.ui.currentSessionId ? ' active' : '');
      el.dataset.sessionId = s.id;
      el.innerHTML = `
        <div class="stitle">${SC.escapeHtml(s.title || 'Untitled')}</div>
        <div class="smeta">${SC.escapeHtml(s.model)} / ${s.messageCount} msgs / ${new Date(s.updatedAt).toLocaleDateString()}</div>
        <div class="session-actions">
          <button class="session-rename" title="Rename session"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
          <button class="session-del" title="Delete session">&times;</button>
        </div>
        <div class="session-confirm hidden">
          <span>Delete this session? This cannot be undone.</span>
          <button class="confirm-delete">Delete</button>
          <button class="confirm-cancel">Cancel</button>
        </div>
      `;

      el.querySelector('.session-rename').addEventListener('click', (e) => {
        e.stopPropagation();
        startRenameSession(el, s);
      });

      el.querySelector('.session-del').addEventListener('click', (e) => {
        e.stopPropagation();
        el.querySelector('.session-confirm').classList.remove('hidden');
      });

      el.querySelector('.confirm-delete').addEventListener('click', (e) => {
        e.stopPropagation();
        SC.wsSend('session_delete', { id: s.id });
      });

      el.querySelector('.confirm-cancel').addEventListener('click', (e) => {
        e.stopPropagation();
        el.querySelector('.session-confirm').classList.add('hidden');
      });

      el.addEventListener('click', () => SC.wsSend('session_load', { id: s.id }));
      list.appendChild(el);
    });
    SC.setState('sessions', sessions);
    SC.$('#s-total-sessions').textContent = sessions.length;
    } catch (err) {
      console.error('[renderSessions Error]', err);
      SC.showErrorBanner('Sessions render error: ' + err.message, function() { renderSessions(sessions); });
    }
  }

  function startRenameSession(el, session) {
    const titleEl = el.querySelector('.stitle');
    const currentTitle = session.title || 'Untitled';
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'rename-input';
    input.value = currentTitle;
    input.maxLength = 100;
    titleEl.replaceWith(input);
    input.focus();
    input.select();

    function finishRename() {
      const newTitle = input.value.trim();
      if (newTitle && newTitle !== currentTitle) {
        SC.wsSend('session_rename', { id: session.id, title: newTitle });
      } else {
        const span = document.createElement('div');
        span.className = 'stitle';
        span.textContent = currentTitle;
        input.replaceWith(span);
      }
    }

    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); finishRename(); }
      else if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        const span = document.createElement('div');
        span.className = 'stitle';
        span.textContent = currentTitle;
        input.replaceWith(span);
      }
    });

    input.addEventListener('blur', finishRename);
  }

  function loadSessionMessages(msg) {
    SC.state.ui.currentSessionId = msg.id;
    try { localStorage.setItem('smartclaw-active-session', msg.id); } catch {}
    SC.state.messages = [];
    var welcome = SC.$('#welcome');
    if (welcome) welcome.classList.add('hidden');

    var messages = msg.messages || [];

    if (SC.vl) {
      var items = [];
      messages.forEach((m, i) => {
        var ts = m.timestamp ? new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
        items.push({
          role: m.role,
          content: m.content,
          ts: m.timestamp ? new Date(m.timestamp).getTime() : Date.now(),
          msgId: 'msg-' + i,
          displayTs: ts,
          isStreaming: false,
          isRendered: m.role === 'assistant',
          thinkingContent: ''
        });
        SC.state.messages.push({ role: m.role, content: m.content, ts: Date.now(), msgId: 'msg-' + i });
      });
      SC.vl.setItems(items);
      SC.scrollChat();
    } else {
      var container = SC.$('#messages');
      container.innerHTML = '';
      messages.forEach((m, i) => {
        const el = document.createElement('div');
        el.className = `message ${m.role}`;
        el.dataset.msgIndex = i;
        el.dataset.msgId = 'msg-' + i;
        const roleLabel = m.role === 'user' ? 'You' : 'SmartClaw';
        const ts = m.timestamp ? new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';

        let actionsHtml = '';
        if (m.role === 'user') {
          actionsHtml = '<div class="msg-actions">' +
            '<button class="msg-action-btn msg-edit-btn" title="Edit">' +
              '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>' +
            '</button>' +
            '<button class="msg-action-btn msg-retry-btn" title="Retry">' +
              '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/></svg>' +
            '</button>' +
          '</div>';
        }

        el.innerHTML = `<div class="msg-role">${roleLabel}</div>${actionsHtml}<div class="msg-bubble">${m.role === 'assistant' ? SC.renderMarkdown(m.content) : SC.escapeHtml(m.content)}</div>${ts ? `<div class="msg-ts">${ts}</div>` : ''}`;
        container.appendChild(el);

        if (m.role === 'user') {
          SC.bindMsgActions ? (function(el, idx) {
            const editBtn = el.querySelector('.msg-edit-btn');
            const retryBtn = el.querySelector('.msg-retry-btn');
            if (editBtn) editBtn.addEventListener('click', function(e) { e.stopPropagation(); SC.startEditMessage(el, idx); });
            if (retryBtn) retryBtn.addEventListener('click', function(e) { e.stopPropagation(); SC.retryMessage(el, idx); });
          })(el, i) : null;
        }

        if (m.role === 'assistant') {
          const bubble = el.querySelector('.msg-bubble');
          SC.bindCodeCopy(bubble);
          SC.postRenderMarkdown(bubble);
        }

        if (SC.bindMessageContextMenu) SC.bindMessageContextMenu(el);
        SC.state.messages.push({ role: m.role, content: m.content, ts: Date.now(), msgId: 'msg-' + i });
      });
      SC.scrollChat();
    }

    SC.wsSend('session_list', {});
  }

  function toggleSessionsPanel() {
    const sb = SC.$('#sidebar');
    if (sb.classList.contains('collapsed')) {
      sb.classList.remove('collapsed');
      SC.state.ui.sidebarOpen = true;
    }
    SC.$$('.nav-btn').forEach(b => b.classList.remove('active'));
    SC.$$('.section').forEach(s => s.classList.remove('active'));
    const sessionsBtn = SC.$('[data-section="sessions"]');
    if (sessionsBtn) sessionsBtn.classList.add('active');
    SC.$('#section-sessions')?.classList.add('active');
    const searchInput = SC.$('#session-search');
    if (searchInput) setTimeout(() => searchInput.focus(), 100);
  }

  SC.renderSessions = renderSessions;
  SC.startRenameSession = startRenameSession;
  SC.loadSessionMessages = loadSessionMessages;
  SC.toggleSessionsPanel = toggleSessionsPanel;
})();
