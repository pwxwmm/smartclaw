// SmartClaw - Sessions
(function() {
  'use strict';

  var PINNED_KEY = 'smartclaw-pinned-sessions';
  var SESSION_ORDER_KEY = 'smartclaw-session-order';

  function getPinnedSessions() {
    try { return JSON.parse(localStorage.getItem(PINNED_KEY) || '[]'); } catch { return []; }
  }

  function togglePinSession(sessionId) {
    var pinned = getPinnedSessions();
    var idx = pinned.indexOf(sessionId);
    if (idx >= 0) {
      pinned.splice(idx, 1);
    } else {
      pinned.unshift(sessionId);
    }
    try { localStorage.setItem(PINNED_KEY, JSON.stringify(pinned)); } catch {}
    return idx < 0;
  }

  function setupSessionDragSort(container) {
    if (!container || container.dataset.dragSortSetup === 'true') return;
    container.dataset.dragSortSetup = 'true';

    SC.makeDraggable(container, {
      dropTarget: container,
      onDragStart: function(e) {
        var item = e.target.closest('.session-item');
        if (!item) return;
        e.dataTransfer.setData('text/plain', item.dataset.sessionId);
        item.classList.add('dragging');
      },
      onDragEnd: function() {
        container.querySelectorAll('.session-item.dragging').forEach(function(el) {
          el.classList.remove('dragging');
        });
        container.querySelectorAll('.session-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });
      },
      onDragOver: function(e) {
        var target = e.target.closest('.session-item');
        if (!target || target.classList.contains('dragging')) return;
        container.querySelectorAll('.session-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });
        target.classList.add('drag-over');
      },
      onDrop: function(e) {
        var target = e.target.closest('.session-item');
        if (!target) return;
        var draggedId = e.dataTransfer.getData('text/plain');
        var items = Array.from(container.querySelectorAll('.session-item'));
        var draggedItem = items.find(function(item) { return item.dataset.sessionId === draggedId; });
        if (!draggedItem || draggedItem === target) return;

        var allItems = Array.from(container.children);
        var draggedIdx = allItems.indexOf(draggedItem);
        var targetIdx = allItems.indexOf(target);

        if (draggedIdx < targetIdx) {
          container.insertBefore(draggedItem, target.nextSibling);
        } else {
          container.insertBefore(draggedItem, target);
        }

        container.querySelectorAll('.session-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });

        saveSessionOrder(container);
      }
    });
  }

  function saveSessionOrder(container) {
    var ids = Array.from(container.querySelectorAll('.session-item')).map(function(el) {
      return el.dataset.sessionId;
    });
    try { localStorage.setItem(SESSION_ORDER_KEY, JSON.stringify(ids)); } catch {}
  }

  function applySessionOrder(container, sessions) {
    try {
      var saved = localStorage.getItem(SESSION_ORDER_KEY);
      if (!saved) return sessions;
      var order = JSON.parse(saved);
      var sessionMap = {};
      sessions.forEach(function(s) { sessionMap[s.id] = s; });
      var ordered = [];
      order.forEach(function(id) {
        if (sessionMap[id]) {
          ordered.push(sessionMap[id]);
          delete sessionMap[id];
        }
      });
      Object.values(sessionMap).forEach(function(s) { ordered.push(s); });
      return ordered;
    } catch { return sessions; }
  }

  function renderSessionItem(container, s, isPinned, searchTerm) {
    var el = document.createElement('div');
    el.className = 'session-item' + (s.id === SC.state.ui.currentSessionId ? ' active' : '') + (isPinned ? ' pinned' : '');
    el.dataset.sessionId = s.id;
    el.draggable = true;
    el.innerHTML = `
      <span class="drag-handle">⋮⋮</span>
      <div class="stitle">${SC.escapeHtml(s.title || 'Untitled')}</div>
      <div class="smeta">${SC.escapeHtml(s.model)} / ${s.messageCount} msgs / ${new Date(s.updatedAt).toLocaleDateString()}</div>
      <div class="session-actions">
        <button class="session-pin" title="Pin session"><svg width="12" height="12" viewBox="0 0 24 24" fill="${isPinned ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="2"><path d="M12 2C9.24 2 7 4.24 7 7c0 3.31 5 11 5 11s5-7.69 5-11c0-2.76-2.24-5-5-5z"/></svg></button>
        <button class="session-rename" title="Rename session"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
        <button class="session-del" title="Delete session">&times;</button>
      </div>
      <div class="session-confirm hidden">
        <span>Delete this session? This cannot be undone.</span>
        <button class="confirm-delete">Delete</button>
        <button class="confirm-cancel">Cancel</button>
      </div>
    `;

    el.querySelector('.session-pin').addEventListener('click', function(e) {
      e.stopPropagation();
      var nowPinned = togglePinSession(s.id);
      SC.renderSessions(SC.state.sessions || []);
    });

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

    el.addEventListener('click', () => {
      if (SC.Tabs) {
        SC.Tabs.createOrSwitch(s.id, s.title || 'Untitled');
      }
      SC.wsSend('session_load', { id: s.id });
    });
    container.appendChild(el);
  }

  function renderSessionsInto(container, sessions, searchTerm) {
    container.innerHTML = '';
    if (sessions.length === 0) {
      SC.showEmptyState(container,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>',
        'No sessions yet',
        'Start a conversation to create your first session.'
      );
      return;
    }

    sessions = applySessionOrder(container, sessions);

    const filtered = searchTerm
      ? sessions.filter(s => (s.title || 'Untitled').toLowerCase().includes(searchTerm))
      : sessions;

    if (filtered.length === 0) {
      SC.showEmptyState(container,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>',
        'No sessions found',
        'Try a different search term.'
      );
      return;
    }

    var pinned = getPinnedSessions();
    var pinnedSessions = filtered.filter(function(s) { return pinned.indexOf(s.id) >= 0; });
    var regularSessions = filtered.filter(function(s) { return pinned.indexOf(s.id) < 0; });

    if (pinnedSessions.length > 0) {
      var pinnedHeader = document.createElement('div');
      pinnedHeader.className = 'session-pinned-header';
      pinnedHeader.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2C9.24 2 7 4.24 7 7c0 3.31 5 11 5 11s5-7.69 5-11c0-2.76-2.24-5-5-5z"/></svg> Pinned';
      container.appendChild(pinnedHeader);

      pinnedSessions.forEach(function(s) {
        renderSessionItem(container, s, true, searchTerm);
      });

      if (regularSessions.length > 0) {
        var divider = document.createElement('div');
        divider.className = 'session-list-divider';
        container.appendChild(divider);
      }
    }

    regularSessions.forEach(function(s) {
      renderSessionItem(container, s, false, searchTerm);
    });
    if (typeof SC.applyListStagger === 'function') SC.applyListStagger(container, '.session-item');
    setupSessionDragSort(container);
  }

  function renderSessions(sessions) {
    try {
    SC.renderToBoth('session-list', 'session-list-view', function(el) {
      var searchId = el.id === 'session-list-view' ? 'session-search-view' : 'session-search';
      var searchTerm = (SC.$('#' + searchId)?.value || '').toLowerCase();
      renderSessionsInto(el, sessions, searchTerm);
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

    if (SC.Tabs) {
      var tab = SC.Tabs.findBySessionId(msg.id);
      if (tab) {
        var session = (SC.state.sessions || []).find(s => s.id === msg.id);
        if (session && session.title) {
          SC.Tabs.rename(tab.id, session.title);
        }
      }
    }

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
      if (typeof SC.startBatchLoad === 'function') SC.startBatchLoad(items.length);
      SC.vl.setItems(items);
      if (typeof SC.endBatchLoad === 'function') SC.endBatchLoad();
      SC.scrollChat();
    } else {
      var container = SC.$('#messages');
      container.innerHTML = '';
      if (typeof SC.startBatchLoad === 'function') SC.startBatchLoad(messages.length);
      messages.forEach((m, i) => {
        var item = {
          role: m.role,
          content: m.content,
          ts: m.timestamp ? new Date(m.timestamp).getTime() : Date.now(),
          msgId: 'msg-' + i,
          displayTs: m.timestamp ? new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '',
          isStreaming: false,
          isRendered: m.role === 'assistant',
          thinkingContent: ''
        };
        var el = SC.createMessageElement ? SC.createMessageElement(item, i) : null;
        if (!el) {
          el = document.createElement('div');
          el.className = 'message ' + m.role;
          el.dataset.msgIndex = i;
          el.dataset.msgId = 'msg-' + i;
          var roleLabel = m.role === 'user' ? 'You' : 'SmartClaw';
          var ts = m.timestamp ? new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
          el.innerHTML = '<div class="msg-bubble">' + (m.role === 'assistant' ? (SC.renderMarkdown ? SC.renderMarkdown(m.content) : SC.escapeHtml(m.content)) : SC.escapeHtml(m.content)) + '</div>' + (ts ? '<div class="msg-ts">' + ts + '</div>' : '');
        }
        container.appendChild(el);

        if (m.role === 'user' && SC.bindMsgActions) {
          SC.bindMsgActions(el, i);
        }

        if (m.role === 'assistant') {
          const bubble = el.querySelector('.msg-bubble');
          if (bubble && SC.bindCodeCopy) SC.bindCodeCopy(bubble);
          if (bubble && SC.postRenderMarkdown) SC.postRenderMarkdown(bubble);
        }

        if (SC.bindMessageContextMenu) SC.bindMessageContextMenu(el);
        SC.state.messages.push({ role: m.role, content: m.content, ts: Date.now(), msgId: 'msg-' + i });
      });
      if (typeof SC.endBatchLoad === 'function') SC.endBatchLoad();
      SC.scrollChat();
    }

    if (SC.Tabs) {
      var loadedTab = SC.Tabs.findBySessionId(msg.id);
      if (loadedTab) {
        loadedTab.vlItems = SC.vl ? SC.vl.items.slice() : [];
      }
    }

    SC.wsSend('session_list', {});
  }

  function toggleSessionsPanel() {
    const sb = SC.$('#sidebar');
    if (sb.classList.contains('collapsed')) {
      sb.classList.remove('collapsed');
      SC.state.ui.sidebarOpen = true;
    }
    SC.$$('.sidebar-rail-btn').forEach(b => b.classList.remove('active'));
    var railBtn = SC.$('.sidebar-rail-btn[data-view="sessions"]');
    if (railBtn) railBtn.classList.add('active');
    SC.$$('.section').forEach(s => s.classList.remove('active'));
    SC.$('#section-sessions')?.classList.add('active');
    const searchInput = SC.$('#session-search');
    if (searchInput) setTimeout(() => searchInput.focus(), 100);
  }

  SC.renderSessions = renderSessions;
  SC.startRenameSession = startRenameSession;
  SC.loadSessionMessages = loadSessionMessages;
  SC.toggleSessionsPanel = toggleSessionsPanel;
})();
