// SmartClaw - Multi-session Tab Browsing
(function() {
  'use strict';

  var MAX_TABS = 8;
  var tabIdCounter = 0;

  var Tabs = {
    tabs: [],
    activeTabId: null,

    init: function() {
      this._bindEvents();
      this._bindScrollButtons();
      this._updateScrollButtons();
    },

    create: function(sessionId, title) {
      if (this.tabs.length >= MAX_TABS) {
        SC.toast('Maximum ' + MAX_TABS + ' tabs allowed', 'warning');
        return null;
      }

      if (sessionId) {
        var existing = this.findBySessionId(sessionId);
        if (existing) {
          this.switch(existing.id);
          return existing;
        }
      }

      var id = 'tab-' + (++tabIdCounter);
      var tab = {
        id: id,
        sessionId: sessionId || null,
        title: title || 'New Chat',
        type: 'chat',
        messages: [],
        vlItems: [],
        scrollPos: 0,
        isActive: true
      };

      this.tabs.forEach(function(t) { t.isActive = false; });
      this.tabs.push(tab);
      this._saveCurrentTabState();

      this.activeTabId = id;
      SC.state.ui.currentSessionId = sessionId || null;
      this._syncStateToActiveTab();

      if (SC.vl) SC.vl.clear();

      var welcome = SC.$('#welcome');
      if (welcome) welcome.classList.remove('hidden');

      this.render();
      return tab;
    },

    switch: function(tabId) {
      if (this.activeTabId === tabId) return;
      var tab = this.findById(tabId);
      if (!tab) return;

      this._saveCurrentTabState();
      this.tabs.forEach(function(t) { t.isActive = false; });

      tab.isActive = true;
      this.activeTabId = tabId;

      if (tab.type === 'file') {
        SC.state.ui.currentSessionId = null;
        this._syncStateToActiveTab();
        this._showFileView(tab.filePath);
      } else {
        SC.state.ui.currentSessionId = tab.sessionId;
        this._syncStateToActiveTab();
        this._restoreTabMessages(tab);
        this._hideFileView();
        if (tab.sessionId) {
          try { localStorage.setItem('smartclaw-active-session', tab.sessionId); } catch {}
        }
      }

      this.render();
    },

    close: function(tabId) {
      var idx = this._indexOf(tabId);
      if (idx < 0) return;

      var closingTab = this.tabs[idx];

      if (closingTab.type === 'file' && closingTab.filePath && SC.state.fileTabs) {
        delete SC.state.fileTabs[closingTab.filePath];
      }

      if (this.tabs.length <= 1) {
        var onlyTab = this.tabs[0];
        onlyTab.messages = [];
        onlyTab.vlItems = [];
        onlyTab.scrollPos = 0;
        onlyTab.sessionId = null;
        onlyTab.title = 'New Chat';
        SC.state.messages = [];
        SC.state.ui.currentSessionId = null;
        if (SC.vl) SC.vl.clear();
        var welcome = SC.$('#welcome');
        if (welcome) welcome.classList.remove('hidden');
        try { localStorage.removeItem('smartclaw-active-session'); } catch {}
        this.render();
        return;
      }

      var wasActive = (this.activeTabId === tabId);

      this.tabs.splice(idx, 1);

      if (wasActive) {
        var newIdx = Math.min(idx, this.tabs.length - 1);
        this.activeTabId = this.tabs[newIdx].id;
        this.tabs[newIdx].isActive = true;

        var activeTab = this.tabs[newIdx];
        if (activeTab.type === 'file') {
          SC.state.ui.currentSessionId = null;
          this._syncStateToActiveTab();
          this._showFileView(activeTab.filePath);
        } else {
          SC.state.ui.currentSessionId = activeTab.sessionId;
          this._syncStateToActiveTab();
          this._restoreTabMessages(activeTab);
          this._hideFileView();
        }

        if (activeTab.sessionId) {
          try { localStorage.setItem('smartclaw-active-session', activeTab.sessionId); } catch {}
        }
      }

      this.render();
    },

    closeOthers: function(tabId) {
      var keepTab = this.findById(tabId);
      if (!keepTab) return;

      this.tabs = [keepTab];
      keepTab.isActive = true;
      this.activeTabId = tabId;

      SC.state.ui.currentSessionId = keepTab.sessionId;
      this._syncStateToActiveTab();
      this._restoreTabMessages(keepTab);

      this.render();
    },

    rename: function(tabId, title) {
      var tab = this.findById(tabId);
      if (!tab) return;
      tab.title = title || 'Untitled';
      this.render();
    },

    getActive: function() {
      return this.findById(this.activeTabId);
    },

    findBySessionId: function(sessionId) {
      if (!sessionId) return null;
      for (var i = 0; i < this.tabs.length; i++) {
        if (this.tabs[i].sessionId === sessionId) return this.tabs[i];
      }
      return null;
    },

    findById: function(tabId) {
      for (var i = 0; i < this.tabs.length; i++) {
        if (this.tabs[i].id === tabId) return this.tabs[i];
      }
      return null;
    },

    createOrSwitch: function(sessionId, title) {
      if (!sessionId) return this.create(null, title);
      var existing = this.findBySessionId(sessionId);
      if (existing) {
        this.switch(existing.id);
        if (title && existing.title !== title) {
          existing.title = title;
          this.render();
        }
        return existing;
      }
      return this.create(sessionId, title);
    },

    createFileTab: function(filename, filePath) {
      if (this.tabs.length >= MAX_TABS) {
        SC.toast('Maximum ' + MAX_TABS + ' tabs allowed', 'warning');
        return null;
      }

      var id = 'tab-' + (++tabIdCounter);
      var tab = {
        id: id,
        sessionId: null,
        title: filename,
        type: 'file',
        filePath: filePath,
        messages: [],
        vlItems: [],
        scrollPos: 0,
        isActive: true
      };

      this.tabs.forEach(function(t) { t.isActive = false; });
      this.tabs.push(tab);
      this._saveCurrentTabState();

      this.activeTabId = id;
      this._syncStateToActiveTab();
      this._showFileView(filePath);

      this.render();
      return tab;
    },

    findByFilePath: function(filePath) {
      if (!filePath) return null;
      for (var i = 0; i < this.tabs.length; i++) {
        if (this.tabs[i].type === 'file' && this.tabs[i].filePath === filePath) return this.tabs[i];
      }
      return null;
    },

    updateSessionTitle: function(sessionId, title) {
      var tab = this.findBySessionId(sessionId);
      if (tab) {
        tab.title = title;
        this.render();
      }
    },

    closeSessionTab: function(sessionId) {
      var tab = this.findBySessionId(sessionId);
      if (tab) {
        this.close(tab.id);
      }
    },

    render: function() {
      var container = SC.$('#tab-bar-scroll');
      if (!container) return;

      container.innerHTML = '';

      var self = this;

      this.tabs.forEach(function(tab) {
        var el = document.createElement('div');
        el.className = 'tab-item' + (tab.type === 'file' ? ' file-tab' : '') + (tab.id === self.activeTabId ? ' active' : '');
        el.dataset.tabId = tab.id;
        el.dataset.sessionId = tab.sessionId || '';
        if (tab.type === 'file') el.dataset.filePath = tab.filePath || '';
        el.draggable = true;

        if (tab.type === 'file') {
          var iconEl = document.createElement('span');
          iconEl.className = 'tab-icon';
          iconEl.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
          el.appendChild(iconEl);
        }

        var titleEl = document.createElement('span');
        titleEl.className = 'tab-title';
        titleEl.textContent = tab.title;
        titleEl.title = tab.title;

        var closeBtn = document.createElement('button');
        closeBtn.className = 'tab-close';
        closeBtn.innerHTML = '&times;';
        closeBtn.title = 'Close tab';

        el.appendChild(titleEl);
        el.appendChild(closeBtn);

        el.addEventListener('click', function(e) {
          if (e.target.closest('.tab-close')) return;
          self.switch(tab.id);
        });

        closeBtn.addEventListener('click', function(e) {
          e.stopPropagation();
          self.close(tab.id);
        });

        el.addEventListener('auxclick', function(e) {
          if (e.button === 1) { e.preventDefault(); self.close(tab.id); }
        });

        el.addEventListener('contextmenu', function(e) {
          e.preventDefault();
          self._showTabContextMenu(e.clientX, e.clientY, tab);
        });

        el.addEventListener('dragstart', function(e) {
          e.dataTransfer.setData('text/plain', tab.id);
          e.dataTransfer.effectAllowed = 'move';
          el.classList.add('dragging');
        });
        el.addEventListener('dragend', function() {
          el.classList.remove('dragging');
          SC.$$('.tab-item', container).forEach(function(t) { t.classList.remove('drag-over'); });
        });
        el.addEventListener('dragover', function(e) {
          e.preventDefault();
          e.dataTransfer.dropEffect = 'move';
          SC.$$('.tab-item', container).forEach(function(t) { t.classList.remove('drag-over'); });
          el.classList.add('drag-over');
        });
        el.addEventListener('dragleave', function() {
          el.classList.remove('drag-over');
        });
        el.addEventListener('drop', function(e) {
          e.preventDefault();
          var draggedId = e.dataTransfer.getData('text/plain');
          if (draggedId === tab.id) return;
          self._reorderTab(draggedId, tab.id);
        });

        container.appendChild(el);
      });

      this._updateScrollButtons();
    },

    _showFileView: function(filePath) {
      var chatView = SC.$('#view-chat');
      var chat = SC.$('#chat');
      var inputArea = SC.$('#input-area');

      SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
      chatView.classList.add('active');

      var existingRow = SC.$('.chat-main-row');
      if (existingRow) {
        chatView.insertBefore(chat, existingRow);
        chatView.insertBefore(inputArea, existingRow.nextSibling);
        existingRow.remove();
      }
      var existingFileView = SC.$('.file-view');
      if (existingFileView) existingFileView.remove();
      var existingDivider = SC.$('.file-chat-divider');
      if (existingDivider) existingDivider.remove();

      var splitPanel = SC.$('.split-code-panel');
      if (splitPanel) splitPanel.remove();
      var splitDivider = SC.$('.split-divider');
      if (splitDivider) splitDivider.remove();
      chatView.classList.remove('split');
      chatView.classList.remove('has-file-view');

      if (!SC.state.fileTabs || !SC.state.fileTabs[filePath]) return;

      var fileData = SC.state.fileTabs[filePath];
      var fileViewEl = SC.renderFileView(fileData.content, fileData.path);

      chatView.classList.add('has-side-panel');
      chat.classList.remove('hidden');
      inputArea.classList.remove('hidden');

      var mainRow = document.createElement('div');
      mainRow.className = 'chat-main-row';

      chatView.insertBefore(mainRow, chat);
      mainRow.appendChild(chat);

      var divider = document.createElement('div');
      divider.className = 'file-chat-divider';
      mainRow.appendChild(divider);

      mainRow.appendChild(fileViewEl);

      this._initDividerDrag(divider);
    },

    _hideFileView: function() {
      var chatView = SC.$('#view-chat');
      var chat = SC.$('#chat');
      var inputArea = SC.$('#input-area');

      var existingRow = SC.$('.chat-main-row');
      if (existingRow) {
        chatView.insertBefore(chat, existingRow);
        chatView.insertBefore(inputArea, existingRow.nextSibling);
        existingRow.remove();
      }

      var existingFileView = SC.$('.file-view');
      if (existingFileView) existingFileView.remove();

      var existingDivider = SC.$('.file-chat-divider');
      if (existingDivider) existingDivider.remove();

      var splitPanel = SC.$('.split-code-panel');
      if (splitPanel) splitPanel.remove();
      var splitDivider = SC.$('.split-divider');
      if (splitDivider) splitDivider.remove();

      chatView.classList.remove('has-file-view');
      chatView.classList.remove('has-side-panel');
      chatView.classList.remove('split');
      chat.classList.remove('hidden');
      inputArea.classList.remove('hidden');
    },

    _initDividerDrag: function(divider) {
      var isDragging = false;
      var startX = 0;
      var startFileWidth = 0;
      var chatView = SC.$('#view-chat');
      var fileView = SC.$('.file-view');

      divider.addEventListener('mousedown', function(e) {
        e.preventDefault();
        isDragging = true;
        startX = e.clientX;
        startFileWidth = fileView.offsetWidth;
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
        divider.classList.add('active');
      });

      document.addEventListener('mousemove', function(e) {
        if (!isDragging) return;
        var delta = startX - e.clientX;
        var newWidth = startFileWidth + delta;
        var minW = 250;
        var maxW = chatView.offsetWidth * 0.7;
        if (newWidth < minW) newWidth = minW;
        if (newWidth > maxW) newWidth = maxW;
        fileView.style.width = newWidth + 'px';
        fileView.style.flex = 'none';
      });

      document.addEventListener('mouseup', function() {
        if (!isDragging) return;
        isDragging = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        divider.classList.remove('active');
      });
    },

    _saveCurrentTabState: function() {
      var active = this.getActive();
      if (!active) return;

      active.messages = SC.state.messages.slice();

      if (SC.vl && SC.vl.items) {
        active.vlItems = SC.vl.items.slice();
      }

      var chatEl = SC.$('#chat');
      if (chatEl) {
        active.scrollPos = chatEl.scrollTop;
      }
    },

    _restoreTabMessages: function(tab) {
      if (SC.vl) {
        if (tab.vlItems && tab.vlItems.length > 0) {
          SC.vl.setItems(tab.vlItems.slice());
        } else {
          SC.vl.clear();
        }
      } else {
        var messagesEl = SC.$('#messages');
        if (messagesEl) messagesEl.innerHTML = '';
      }

      var welcome = SC.$('#welcome');
      if (tab.messages.length === 0) {
        if (welcome) welcome.classList.remove('hidden');
      } else {
        if (welcome) welcome.classList.add('hidden');
      }

      requestAnimationFrame(function() {
        var chatEl = SC.$('#chat');
        if (chatEl && tab.scrollPos > 0) {
          chatEl.scrollTop = tab.scrollPos;
        }
      });
    },

    _syncStateToActiveTab: function() {
      var active = this.getActive();
      if (active) {
        SC.state.messages = active.messages;
      } else {
        SC.state.messages = [];
      }
    },

    _indexOf: function(tabId) {
      for (var i = 0; i < this.tabs.length; i++) {
        if (this.tabs[i].id === tabId) return i;
      }
      return -1;
    },

    _reorderTab: function(draggedId, targetId) {
      var dragIdx = this._indexOf(draggedId);
      var targetIdx = this._indexOf(targetId);
      if (dragIdx < 0 || targetIdx < 0 || dragIdx === targetIdx) return;

      var dragged = this.tabs.splice(dragIdx, 1)[0];
      this.tabs.splice(targetIdx, 0, dragged);
      this.render();
    },

    _showTabContextMenu: function(x, y, tab) {
      var self = this;
      SC.showContextMenu(x, y, [
        { label: 'Close', action: function() { self.close(tab.id); } },
        { label: 'Close Others', action: function() { self.closeOthers(tab.id); } },
        { label: 'Rename', action: function() { self._startRename(tab); } }
      ]);
    },

    _startRename: function(tab) {
      var tabEl = SC.$('.tab-item[data-tab-id="' + tab.id + '"]');
      if (!tabEl) return;

      var titleEl = tabEl.querySelector('.tab-title');
      if (!titleEl) return;

      var input = document.createElement('input');
      input.type = 'text';
      input.className = 'tab-rename-input';
      input.value = tab.title;
      input.maxLength = 100;

      titleEl.replaceWith(input);
      input.focus();
      input.select();

      var self = this;
      function finishRename() {
        var newTitle = input.value.trim();
        if (newTitle && newTitle !== tab.title) {
          tab.title = newTitle;
          if (tab.sessionId) {
            SC.wsSend('session_rename', { id: tab.sessionId, title: newTitle });
          }
        }
        self.render();
      }

      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') { e.preventDefault(); finishRename(); }
        else if (e.key === 'Escape') { e.preventDefault(); self.render(); }
      });
      input.addEventListener('blur', finishRename);
    },

    _bindEvents: function() {
      var newBtn = SC.$('#tab-new');
      if (newBtn) {
        newBtn.addEventListener('click', function() {
          SC.wsSend('session_new', { model: SC.state.settings.model });
        });
      }
    },

    _bindScrollButtons: function() {
      var self = this;
      var scrollContainer = SC.$('#tab-bar-scroll');
      var leftBtn = SC.$('#tab-scroll-left');
      var rightBtn = SC.$('#tab-scroll-right');

      if (!scrollContainer || !leftBtn || !rightBtn) return;

      leftBtn.addEventListener('click', function() {
        scrollContainer.scrollLeft -= 150;
      });
      rightBtn.addEventListener('click', function() {
        scrollContainer.scrollLeft += 150;
      });

      scrollContainer.addEventListener('scroll', function() {
        self._updateScrollButtons();
      });

      window.addEventListener('resize', function() {
        self._updateScrollButtons();
      });
    },

    _updateScrollButtons: function() {
      var scrollContainer = SC.$('#tab-bar-scroll');
      var leftBtn = SC.$('#tab-scroll-left');
      var rightBtn = SC.$('#tab-scroll-right');

      if (!scrollContainer || !leftBtn || !rightBtn) return;

      var hasOverflow = scrollContainer.scrollWidth > scrollContainer.clientWidth;
      var atStart = scrollContainer.scrollLeft <= 2;
      var atEnd = scrollContainer.scrollLeft + scrollContainer.clientWidth >= scrollContainer.scrollWidth - 2;

      leftBtn.classList.toggle('hidden', !hasOverflow || atStart);
      rightBtn.classList.toggle('hidden', !hasOverflow || atEnd);
    }
  };

  SC.Tabs = Tabs;
})();
