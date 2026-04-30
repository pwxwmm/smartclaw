// SmartClaw - Entry Point
(function() {
  'use strict';

  function reRenderActiveView(viewName) {
    if (viewName === 'sessions' && SC.state.sessions) {
      SC.renderSessions(SC.state.sessions);
    } else if (viewName === 'agents') {
      SC.renderAgentList();
    } else if (viewName === 'skills') {
      SC.renderSkillList();
    } else if (viewName === 'memory') {
      if (SC.state.memoryLayers) SC.renderMemoryLayers();
      if (SC.state.memoryStats) SC.renderMemoryStats();
      SC.renderSkillMemoryList();
      SC.renderUserObservations();
    } else if (viewName === 'mcp') {
      if (typeof SC.renderMCPInstalled === 'function') SC.renderMCPInstalled();
      if (typeof SC.renderMCPCatalog === 'function') SC.renderMCPCatalog();
    } else if (viewName === 'wiki') {
      if (typeof SC.renderWikiPages === 'function') SC.renderWikiPages();
    } else if (viewName === 'files' && SC.state.fileTreeData) {
      SC.renderFileTree(SC.state.fileTreeData);
    } else if (viewName === 'cron') {
      SC.renderCronPanel();
    } else if (viewName === 'privacy') {
      if (typeof SC.initPrivacy === 'function') SC.initPrivacy();
    } else if (viewName === 'settings') {
      syncSettingsToView();
    }
  }

  function syncSettingsToView() {
    var pairs = [
      ['theme-select', 'theme-select-view'],
      ['model-select', 'model-select-view'],
      ['font-size', 'font-size-view'],
      ['lang-select', 'lang-select-view'],
      ['cfg-api-key', 'cfg-api-key-view'],
      ['cfg-base-url', 'cfg-base-url-view'],
      ['cfg-custom-model', 'cfg-custom-model-view'],
      ['cfg-openai', 'cfg-openai-view'],
      ['setting-sound', 'setting-sound-view'],
      ['setting-density', 'setting-density-view'],
      ['watchdog-toggle', 'watchdog-toggle-view']
    ];

    pairs.forEach(function(pair) {
      var src = SC.$('#' + pair[0]);
      var dst = SC.$('#' + pair[1]);
      if (src && dst) {
        if (src.type === 'checkbox') {
          dst.checked = src.checked;
        } else {
          dst.value = src.value;
        }
      } else if (!src && dst) {
        var stateVal = SC.state && SC.state.settings ? SC.state.settings[pair[0]] : undefined;
        if (stateVal !== undefined) {
          if (dst.type === 'checkbox') dst.checked = !!stateVal;
          else dst.value = stateVal;
        }
      }
    });

    var currentTheme = SC.state.settings.theme || 'dark';
    document.querySelectorAll('.theme-preview-card').forEach(function(c) {
      c.classList.toggle('active', c.dataset.theme === currentTheme);
    });

    var soundToggle = SC.$('#setting-sound-view') || SC.$('#setting-sound');
    if (soundToggle && SC.audio) soundToggle.checked = SC.audio.isSoundEnabled();

    var densitySelect = SC.$('#setting-density-view') || SC.$('#setting-density');
    if (densitySelect) densitySelect.value = localStorage.getItem('smartclaw-density') || 'comfortable';
  }

  SC.reRenderActiveView = reRenderActiveView;
  SC.syncSettingsToView = syncSettingsToView;

  // Desktop mode detection (Wails sets window.__WAILS__)
  SC.isDesktop = !!(window.__WAILS__);
  if (SC.isDesktop) {
    document.body.classList.add('desktop-mode');
  }

  function desktopWindowAction(action) {
    var rt = window.runtime;
    if (!rt) return;
    switch (action) {
      case 'minimize':
        if (rt.WindowMinimise) rt.WindowMinimise();
        break;
      case 'maximize':
        if (rt.WindowToggleMaximise) rt.WindowToggleMaximise();
        else if (rt.WindowMaximise) rt.WindowMaximise();
        break;
      case 'close':
        if (rt.WindowClose) rt.WindowClose();
        break;
    }
  }
  SC.desktopWindowAction = desktopWindowAction;

  var dirPickerCurrentPath = '';
  var dirPickerSelectedPath = '';
  var dirPickerEntries = [];

  function openDirPicker() {
    dirPickerSelectedPath = '';
    var overlay = SC.$('#dir-picker-overlay');
    if (!overlay) return;
    overlay.classList.remove('hidden');
    SC.$('#dir-picker-selected').textContent = '';
    SC.$('#dir-picker-select').disabled = false;
    SC.wsSend('browse_dirs', { path: '' });
    SC.wsSend('get_recent_projects', {});
  }
  SC.openDirPicker = openDirPicker;

  function closeDirPicker() {
    var overlay = SC.$('#dir-picker-overlay');
    if (overlay) overlay.classList.add('hidden');
    dirPickerSelectedPath = '';
  }

  function renderDirPickerEntries(data) {
    dirPickerCurrentPath = data.path || '';
    dirPickerEntries = data.entries || [];
    var listEl = SC.$('#dir-picker-list');
    var pathInput = SC.$('#dir-picker-path');
    var upBtn = SC.$('#dir-picker-up');
    if (!listEl) return;
    if (pathInput) pathInput.value = dirPickerCurrentPath;
    if (upBtn) upBtn.disabled = !data.parent;
    SC.$('#dir-picker-selected').textContent = dirPickerCurrentPath;

    listEl.innerHTML = '';
    if (dirPickerEntries.length === 0) {
      var empty = document.createElement('div');
      empty.className = 'dir-picker-empty';
      empty.textContent = 'No subdirectories';
      listEl.appendChild(empty);
      return;
    }
    dirPickerEntries.sort(function(a, b) { return a.name.localeCompare(b.name); });
    dirPickerEntries.forEach(function(entry, idx) {
      var item = document.createElement('div');
      item.className = 'dir-picker-item';
      item.dataset.idx = idx;
      if (dirPickerSelectedPath === entry.path) item.classList.add('selected');
      item.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg><span>' + entry.name + '</span>';
      item.addEventListener('click', function() {
        SC.wsSend('browse_dirs', { path: entry.path });
      });
      listEl.appendChild(item);
    });

    renderDirPickerRecent();
  }
  SC.renderDirPickerEntries = renderDirPickerEntries;

  function renderDirPickerRecent() {
    var section = SC.$('#dir-picker-recent');
    var listEl = SC.$('#dir-picker-recent-list');
    if (!section || !listEl) return;
    var projects = SC.state.recentProjects || [];
    if (projects.length === 0) {
      section.classList.add('hidden');
      return;
    }
    section.classList.remove('hidden');
    listEl.innerHTML = '';
    projects.slice(0, 5).forEach(function(proj) {
      var item = document.createElement('div');
      item.className = 'dir-picker-recent-item';
      var initial = (proj.name || proj.path.split('/').pop()).charAt(0).toUpperCase();
      item.innerHTML = '<div class="project-avatar-sm">' + SC.escapeHtml(initial) + '</div>' +
        '<div class="dir-picker-recent-info">' +
        '<div class="dir-picker-recent-name">' + SC.escapeHtml(proj.name || proj.path.split('/').pop()) + '</div>' +
        '<div class="dir-picker-recent-path">' + SC.escapeHtml(proj.path) + '</div></div>';
      item.addEventListener('click', function() {
        SC.wsSend('change_project', { path: proj.path });
        closeDirPicker();
      });
      listEl.appendChild(item);
    });
  }
  SC.renderDirPickerRecent = renderDirPickerRecent;

  function initDirPicker() {
    SC.$('#dir-picker-close')?.addEventListener('click', closeDirPicker);
    SC.$('#dir-picker-cancel')?.addEventListener('click', closeDirPicker);
    SC.$('#dir-picker-overlay')?.addEventListener('click', function(e) {
      if (e.target === this) closeDirPicker();
    });
    SC.$('#dir-picker-up')?.addEventListener('click', function() {
      if (dirPickerCurrentPath && dirPickerCurrentPath !== '/') {
        var parent = dirPickerCurrentPath.split('/').slice(0, -1).join('/') || '/';
        SC.wsSend('browse_dirs', { path: parent });
      }
    });
    SC.$('#dir-picker-go')?.addEventListener('click', function() {
      var path = (SC.$('#dir-picker-path')?.value || '').trim();
      if (path) SC.wsSend('browse_dirs', { path: path });
    });
    SC.$('#dir-picker-path')?.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        var path = (this.value || '').trim();
        if (path) SC.wsSend('browse_dirs', { path: path });
      }
    });
    SC.$('#dir-picker-select')?.addEventListener('click', function() {
      if (dirPickerCurrentPath) {
        SC.wsSend('change_project', { path: dirPickerCurrentPath });
        closeDirPicker();
      }
    });
    SC.$('#dir-picker-list')?.addEventListener('dblclick', function(e) {
      var item = e.target.closest('.dir-picker-item');
      if (item) {
        var idx = parseInt(item.dataset.idx, 10);
        var entry = dirPickerEntries[idx];
        if (entry) {
          SC.wsSend('change_project', { path: entry.path });
          closeDirPicker();
        }
      }
    });
  }

  function init() {
    if (SC.monitoring) SC.monitoring.init();
    if (SC.i18n) SC.i18n.init();
    SC.loadSettings();
    function tryMermaid() {
      if (typeof window.mermaid !== 'undefined') {
        mermaid.initialize({ startOnLoad: false, theme: 'dark' });
      } else {
        setTimeout(tryMermaid, 200);
      }
    }
    tryMermaid();
    var deferredInitsRan = false;
    function tryDeferredInits() {
      if (!SC.state.authenticated && !SC.state._noAuth) return;
      if (deferredInitsRan) return;
      deferredInitsRan = true;
      if (typeof SC.initPrivacy === 'function') SC.initPrivacy();
      if (typeof SC.initCanvas === 'function') SC.initCanvas();
      if (typeof SC.initArtifacts === 'function') SC.initArtifacts();
      if (typeof SC.initPanels === 'function') SC.initPanels();
      if (typeof SC.initGraph === 'function') SC.initGraph();
      if (typeof SC.initArena === 'function') SC.initArena();
      if (typeof SC.initWatchdog === 'function') SC.initWatchdog();
      if (typeof SC.initAdaptivePanels === 'function') SC.initAdaptivePanels();
      if (typeof SC.initIntentRibbon === 'function') SC.initIntentRibbon();
      if (typeof SC.initKeyboard === 'function') SC.initKeyboard();
      if (typeof SC.initBranches === 'function') SC.initBranches();
      if (typeof SC.initParticles === 'function') SC.initParticles();
      if (typeof SC.initSpotlight === 'function') SC.initSpotlight();
      if (typeof SC.initTimeline === 'function') SC.initTimeline();
      if (typeof SC.initThemeAware === 'function') SC.initThemeAware();
      if (typeof SC.initStatusRing === 'function') SC.initStatusRing();
      var soundToggle = SC.$('#setting-sound-view') || SC.$('#setting-sound');
      if (soundToggle && SC.audio) soundToggle.checked = SC.audio.isSoundEnabled();
    }
    SC.runDeferredInits = tryDeferredInits;
    if (document.readyState === 'complete') {
      tryDeferredInits();
    } else {
      window.addEventListener('load', tryDeferredInits);
    }
    if (typeof SC.initVirtualList === 'function') {
      SC.initVirtualList();
    }
    if (SC.Tabs) {
      SC.Tabs.init();
      SC.Tabs.create(null, 'New Chat');
    }
    SC.wsConnect();
    SC.initDragDrop();
    SC.initCmdPalette();
    SC.initFileMention();
    SC.initFileSearch();
    SC.initSidebarDragSort();
    SC.initNotifications();
    if (typeof SC.renderShortcutsEditor === 'function') SC.renderShortcutsEditor();
    SC.initThemeEditor();
    initDirPicker();
    if (typeof SC.renderAgentSwitcher === 'function') SC.renderAgentSwitcher();

    if (SC.isDesktop) {
      var winMin = SC.$('#win-minimize');
      var winMax = SC.$('#win-maximize');
      var winClose = SC.$('#win-close');
      if (winMin) winMin.addEventListener('click', function() { SC.desktopWindowAction('minimize'); });
      if (winMax) winMax.addEventListener('click', function() { SC.desktopWindowAction('maximize'); });
      if (winClose) winClose.addEventListener('click', function() { SC.desktopWindowAction('close'); });
    }

    var bookmarkBtn = SC.$('#btn-bookmarks');
    var bookmarkDropdown = SC.$('#bookmark-dropdown');
    if (bookmarkBtn && bookmarkDropdown) {
      bookmarkBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        var visible = bookmarkDropdown.classList.contains('hidden');
        bookmarkDropdown.classList.toggle('hidden', !visible);
        var notifDropdown = SC.$('#notification-dropdown');
        if (notifDropdown) notifDropdown.classList.add('hidden');
        if (visible) renderBookmarkDropdown();
      });

      var bookmarkClearAll = SC.$('#bookmark-clear-all');
      if (bookmarkClearAll) {
        bookmarkClearAll.addEventListener('click', function() {
          try { localStorage.removeItem('smartclaw-msg-bookmarks'); } catch(e) {}
          SC.renderBookmarkBadge();
          renderBookmarkDropdown();
          SC.toast('All bookmarks cleared', 'info');
        });
      }

      document.addEventListener('click', function(e) {
        if (!bookmarkBtn.contains(e.target) && !bookmarkDropdown.contains(e.target)) {
          bookmarkDropdown.classList.add('hidden');
        }
      });
    }

    var installBtn = SC.$('#btn-install-app');
    if (installBtn) installBtn.addEventListener('click', function() {
      if (typeof SC.installApp === 'function') SC.installApp();
    });

    function renderBookmarkDropdown() {
      var list = SC.$('#bookmark-list');
      if (!list) return;
      var bookmarks = SC.getMessageBookmarks();
      list.innerHTML = '';
      if (bookmarks.length === 0) {
        list.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2);padding:12px">No bookmarks yet</div>';
        return;
      }
      bookmarks.forEach(function(bm) {
        var item = document.createElement('div');
        item.className = 'bookmark-item';
        item.setAttribute('role', 'menuitem');
        var roleLabel = bm.role === 'user' ? 'You' : bm.role === 'assistant' ? 'SmartClaw' : 'System';
        var timeStr = bm.timestamp ? new Date(bm.timestamp).toLocaleString() : '';
        item.innerHTML = '<div class="bookmark-item-head"><span class="bookmark-role">' + SC.escapeHtml(roleLabel) + '</span>' + (timeStr ? '<span class="bookmark-time">' + SC.escapeHtml(timeStr) + '</span>' : '') + '</div><div class="bookmark-excerpt">' + SC.escapeHtml(bm.content.slice(0, 120)) + (bm.content.length > 120 ? '...' : '') + '</div>';
        list.appendChild(item);
      });
    }
    if (typeof SC.renderUserProfile === 'function') SC.renderUserProfile();
    if (typeof SC.renderCronPanel === 'function') SC.renderCronPanel();
    if (typeof SC.initMCP === 'function') SC.initMCP();

    var contextTabs = SC.$$('.context-view-tab');
    contextTabs.forEach(function(tab) {
      tab.addEventListener('click', function() {
        contextTabs.forEach(function(t) { t.classList.remove('active'); });
        this.classList.add('active');
        var section = this.dataset.section;
        SC.$('#context-files-section').classList.toggle('hidden', section !== 'context-files');
        SC.$('#dependency-graph-section').classList.toggle('hidden', section !== 'dependency-graph');
        if (section === 'dependency-graph' && SC.renderGraph) SC.renderGraph();
      });
    });

    SC.showWelcome();

    var dagToggle = SC.$('#dag-panel-toggle');
    if (dagToggle) {
      dagToggle.addEventListener('click', function() {
        var panel = SC.$('#dag-panel');
        if (panel) panel.classList.toggle('collapsed');
      });
    }

    setTimeout(() => {
      const splash = document.getElementById('splash');
      if (splash) {
        splash.classList.add('fade-out');
        setTimeout(() => splash.remove(), 500);
      }
    }, 800);

    var btnSend = SC.$('#btn-send'); if (btnSend) btnSend.addEventListener('click', SC.sendMessage);
    var inputEl = SC.$('#input');
    if (inputEl) {
      inputEl.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); SC.sendMessage(); }
        else if (e.key === 'ArrowUp' && e.target.value === '' && SC.state.commandHistory.length > 0) {
          e.preventDefault();
          SC.state.historyIndex = Math.max(0, SC.state.historyIndex - 1);
          e.target.value = SC.state.commandHistory[SC.state.historyIndex] || '';
          e.target.style.height = 'auto';
          e.target.style.height = Math.min(e.target.scrollHeight, 200) + 'px';
        }
        else if (e.key === 'ArrowDown' && e.target.value === '' && SC.state.commandHistory.length > 0) {
          e.preventDefault();
          SC.state.historyIndex = Math.min(SC.state.commandHistory.length, SC.state.historyIndex + 1);
          e.target.value = SC.state.commandHistory[SC.state.historyIndex] || '';
          e.target.style.height = 'auto';
          e.target.style.height = Math.min(e.target.scrollHeight, 200) + 'px';
        }
      });
      inputEl.addEventListener('input', function() {
        this.style.height = 'auto';
        this.style.height = Math.min(this.scrollHeight, 200) + 'px';
      });
    }

    SC.$('#sidebar-open')?.addEventListener('click', () => {
      if (SC.isMobile()) {
        SC.toggleSidebarDrawer();
      } else {
        const sb = SC.$('#sidebar');
        if (sb.classList.contains('hidden')) {
          sb.classList.remove('hidden');
          sb.classList.add('visible');
          sb.classList.remove('collapsed');
          SC.state.ui.sidebarOpen = true;
        } else {
          sb.classList.add('hidden');
          sb.classList.remove('visible');
          SC.state.ui.sidebarOpen = false;
        }
      }
    });

    SC.$('#sidebar-open-view')?.addEventListener('click', () => {
      if (SC.isMobile()) {
        SC.toggleSidebarDrawer();
      } else {
        const sb = SC.$('#sidebar');
        sb.classList.toggle('collapsed');
        SC.state.ui.sidebarOpen = !sb.classList.contains('collapsed');
      }
    });

    SC.$('#sidebar-toggle')?.addEventListener('click', () => {
      const sb = SC.$('#sidebar');
      sb.classList.toggle('collapsed');
      SC.state.ui.sidebarOpen = !sb.classList.contains('collapsed');
    });

    SC.$$('.sidebar-rail-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var viewName = btn.dataset.view;
        if (!viewName) return;

        SC.$$('.sidebar-rail-btn.active').forEach(function(b) { b.classList.remove('active'); b.setAttribute('aria-selected', 'false'); });
        btn.classList.add('active');
        btn.setAttribute('aria-selected', 'true');

        // Sidebar-only sections
        var sidebarSections = ['files'];
        if (sidebarSections.indexOf(viewName) >= 0) {
          var sb = SC.$('#sidebar');
          if (sb && sb.classList.contains('collapsed')) {
            sb.classList.remove('collapsed');
            SC.state.ui.sidebarOpen = true;
          }
          var oldSection = SC.$('.section.active');
          var newSection = SC.$('#section-' + viewName);
          if (oldSection && newSection && oldSection !== newSection) {
            oldSection.classList.remove('active');
            newSection.classList.add('active');
          }
          SC.loadSectionData(viewName);
          return;
        }

        // Full-page views
        var fullPageViews = ['sessions', 'settings', 'agents', 'skills', 'memory', 'mcp', 'wiki'];
        if (fullPageViews.indexOf(viewName) >= 0) {
          SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
          var targetView = SC.$('#view-' + viewName);
          if (targetView) targetView.classList.add('active');
          SC.loadSectionData(viewName);
          if (typeof SC.reRenderActiveView === 'function') SC.reRenderActiveView(viewName);
          return;
        }

        // Chat view
        if (viewName === 'chat') {
          var activeTab = SC.Tabs.getActive();
          if (activeTab && activeTab.type === 'file') {
            var chatTab = null;
            for (var i = 0; i < SC.Tabs.tabs.length; i++) {
              if (SC.Tabs.tabs[i].type !== 'file') { chatTab = SC.Tabs.tabs[i]; break; }
            }
            if (chatTab) SC.Tabs.switch(chatTab.id);
          }
          SC.$$('.view').forEach(function(v) { v.classList.remove('active'); });
          SC.$('#view-chat').classList.add('active');
        }
      });
    });

    SC.$('.sidebar-brand-btn')?.addEventListener('click', function() {
      var sb = SC.$('#sidebar');
      if (sb) {
        sb.classList.toggle('collapsed');
        SC.state.ui.sidebarOpen = !sb.classList.contains('collapsed');
      }
    });

    SC.$('#editor-close').addEventListener('click', SC.closeEditor);
    SC.$('#editor-save').addEventListener('click', SC.saveEditor);
    SC.$('#dashboard-close').addEventListener('click', () => SC.$('#dashboard-panel').classList.remove('visible'));
    SC.$('#btn-dashboard').addEventListener('click', SC.openDashboard);
    SC.$('#btn-editor').addEventListener('click', () => {
      SC.openEditor('', SC.state.ui.editorFile || 'untitled.go');
    });

    SC.$('#theme-editor-close').addEventListener('click', () => SC.$('#theme-editor-panel').classList.remove('visible'));
    SC.$('#open-theme-editor')?.addEventListener('click', () => {
      const panel = SC.$('#theme-editor-panel');
      panel.classList.remove('hidden');
      requestAnimationFrame(() => panel.classList.add('visible'));
    });
    SC.$('#theme-export')?.addEventListener('click', SC.exportTheme);
    SC.$('#theme-import')?.addEventListener('click', SC.importTheme);

    SC.$('#theme-select')?.addEventListener('change', (e) => {
      SC.applyThemeWithTransition(e.target.value);
      var viewEl = SC.$('#theme-select-view');
      if (viewEl) viewEl.value = e.target.value;
    });
    SC.$('#model-select')?.addEventListener('change', (e) => {
      SC.state.settings.model = e.target.value;
      SC.$('#current-model').textContent = e.target.value;
      detectProjectLanguage();
      SC.wsSend('model', { model: e.target.value });
      SC.saveSettings();
      var viewEl = SC.$('#model-select-view');
      if (viewEl) viewEl.value = e.target.value;
    });
    SC.$('#font-size')?.addEventListener('input', (e) => {
      SC.state.settings.fontSize = parseInt(e.target.value);
      SC.saveSettings();
      SC.applySettings();
      var viewEl = SC.$('#font-size-view');
      if (viewEl) viewEl.value = e.target.value;
    });

    var langSelect = SC.$('#lang-select');
    if (langSelect) {
      var savedLang = SC.i18n && SC.i18n.getLanguage();
      if (savedLang) langSelect.value = savedLang;
      langSelect.addEventListener('change', function() {
        if (SC.i18n) SC.i18n.setLanguage(langSelect.value);
        var viewEl = SC.$('#lang-select-view');
        if (viewEl) viewEl.value = langSelect.value;
      });
    }

    SC.$('#theme-select-view')?.addEventListener('change', (e) => {
      SC.applyThemeWithTransition(e.target.value);
      var sidebarEl = SC.$('#theme-select');
      if (sidebarEl) sidebarEl.value = e.target.value;
    });

    document.querySelectorAll('.theme-preview-card').forEach(function(card) {
      card.addEventListener('click', function() {
        var theme = card.dataset.theme;
        document.querySelectorAll('.theme-preview-card').forEach(function(c) { c.classList.remove('active'); });
        card.classList.add('active');
        if (SC.applyThemeWithTransition) SC.applyThemeWithTransition(theme);
        var sidebarSelect = SC.$('#theme-select');
        var viewSelect = SC.$('#theme-select-view');
        if (sidebarSelect) sidebarSelect.value = theme;
        if (viewSelect) viewSelect.value = theme;
      });
    });
    SC.$('#model-select-view')?.addEventListener('change', (e) => {
      SC.state.settings.model = e.target.value;
      SC.$('#current-model').textContent = e.target.value;
      detectProjectLanguage();
      SC.wsSend('model', { model: e.target.value });
      SC.saveSettings();
      var sidebarEl = SC.$('#model-select');
      if (sidebarEl) sidebarEl.value = e.target.value;
    });
    SC.$('#font-size-view')?.addEventListener('input', (e) => {
      SC.state.settings.fontSize = parseInt(e.target.value);
      SC.saveSettings();
      SC.applySettings();
      var sidebarEl = SC.$('#font-size');
      if (sidebarEl) sidebarEl.value = e.target.value;
    });
    var langSelectView = SC.$('#lang-select-view');
    if (langSelectView) {
      langSelectView.addEventListener('change', function() {
        if (SC.i18n) SC.i18n.setLanguage(langSelectView.value);
        var sidebarEl = SC.$('#lang-select');
        if (sidebarEl) sidebarEl.value = langSelectView.value;
      });
    }

    SC.loadProviderConfig();

    SC.$('#btn-save-provider')?.addEventListener('click', SC.saveProviderConfig);

    SC.$('#btn-save-provider-view')?.addEventListener('click', () => {
      var apiKey = SC.$('#cfg-api-key-view')?.value?.trim() || '';
      var baseUrl = SC.$('#cfg-base-url-view')?.value?.trim() || '';
      var customModel = SC.$('#cfg-custom-model-view')?.value?.trim() || '';
      var openai = SC.$('#cfg-openai-view')?.checked ?? true;
      var sidebarApiKey = SC.$('#cfg-api-key');
      var sidebarBaseUrl = SC.$('#cfg-base-url');
      var sidebarCustomModel = SC.$('#cfg-custom-model');
      var sidebarOpenai = SC.$('#cfg-openai');
      if (sidebarApiKey) sidebarApiKey.value = apiKey;
      if (sidebarBaseUrl) sidebarBaseUrl.value = baseUrl;
      if (sidebarCustomModel) sidebarCustomModel.value = customModel;
      if (sidebarOpenai) sidebarOpenai.checked = openai;
      SC.saveProviderConfig();
    });

    SC.$('#open-theme-editor-view')?.addEventListener('click', () => {
      const panel = SC.$('#theme-editor-panel');
      panel.classList.remove('hidden');
      requestAnimationFrame(() => panel.classList.add('visible'));
    });

    SC.$('#open-dashboard-view')?.addEventListener('click', SC.openDashboard);
    SC.$('#open-dashboard-view-2')?.addEventListener('click', SC.openDashboard);

    SC.$('#model-select')?.addEventListener('change', (e) => {
      const custom = SC.$('#cfg-custom-model');
      if (e.target.value !== '__custom__') {
        if (custom) custom.value = e.target.value;
      }
    });

    SC.$('#file-search-view')?.addEventListener('input', (e) => {
      var sidebarInput = SC.$('#file-search');
      if (sidebarInput) {
        sidebarInput.value = e.target.value;
        sidebarInput.dispatchEvent(new Event('input'));
      }
    });

    SC.$('#btn-voice').addEventListener('click', () => {
      if (SC.state.isRecording) SC.stopVoice();
      else SC.startVoice();
    });
    SC.$('#voice-stop').addEventListener('click', SC.stopVoice);

    SC.$('#btn-stop').addEventListener('click', () => {
      SC.wsSend('abort', {});
      SC.setState('isProcessing', false);
      SC.updateStopBtn();
    });

    SC.$('#refresh-files').addEventListener('click', () => SC.wsSend('file_tree', { path: '.' }));
    SC.$('#refresh-files-view')?.addEventListener('click', () => SC.wsSend('file_tree', { path: '.' }));
    SC.$('#refresh-files-view-2')?.addEventListener('click', () => SC.wsSend('file_tree', { path: '.' }));
    SC.$('#cost-refresh')?.addEventListener('click', () => { if (typeof SC.initCostDashboard === 'function') SC.initCostDashboard(); });
    SC.$('#cost-refresh-view')?.addEventListener('click', () => { if (typeof SC.initCostDashboard === 'function') SC.initCostDashboard(); });
    SC.$('#privacy-refresh-view')?.addEventListener('click', () => { if (typeof SC.initPrivacy === 'function') SC.initPrivacy(); });
    SC.$('#profile-refresh')?.addEventListener('click', () => { if (typeof SCProfile !== 'undefined') { SCProfile.loadProfile(true); SCProfile.loadObservations(true); SCProfile.renderPrivacySection(); } });
    SC.$('#new-session')?.addEventListener('click', () => SC.wsSend('session_new', { model: SC.state.settings.model }));
    SC.$('#new-session-view')?.addEventListener('click', () => SC.wsSend('session_new', { model: SC.state.settings.model }));
    SC.$('#refresh-sessions-view')?.addEventListener('click', () => SC.wsSend('session_list', {}));
    SC.$('#reload-context-view')?.addEventListener('click', () => SC.wsSend('context_reload', {}));

    let skillSearchTimer = null;
    SC.$('#skill-search')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (skillSearchTimer) clearTimeout(skillSearchTimer);
      if (!query) {
        SC.wsSend('skill_list', {});
        return;
      }
      skillSearchTimer = setTimeout(() => SC.wsSend('skill_search', { query }), 300);
    });

    SC.$('#skill-search-view')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (skillSearchTimer) clearTimeout(skillSearchTimer);
      if (!query) {
        SC.wsSend('skill_list', {});
        return;
      }
      skillSearchTimer = setTimeout(() => SC.wsSend('skill_search', { query }), 300);
    });

    SC.$('#btn-create-skill-view')?.addEventListener('click', () => {
      if (typeof SC.showSkillCreateForm === 'function') SC.showSkillCreateForm();
    });

    SC.$('#memory-search')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('memory_search', { query, limit: 5 });
      }
    });

    SC.$('#memory-search-view')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('memory_search', { query, limit: 5 });
      }
    });

    SC.$$('.memory-tab').forEach(tab => {
      tab.addEventListener('click', () => {
        var parent = tab.closest('#section-memory, #view-memory');
        var isView = parent && parent.id === 'view-memory';
        var suffix = isView ? '-view' : '';

        SC.$$('.memory-tab', parent).forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        SC.$$('.memory-tab-content', parent).forEach(c => c.classList.remove('active'));
        const tabId = tab.dataset.tab;
        const content = SC.$(`#memory-tab-${tabId}${suffix}`);
        if (content) content.classList.add('active');
        SC.state.memoryTab = tabId;
        if (tabId === 'l3') {
          SC.renderSkillMemoryList();
        } else if (tabId === 'l4') {
          SC.wsSend('memory_observations', {});
          if (typeof SC.renderUserProfile === 'function') SC.renderUserProfile();
        }
      });
    });

    SC.$('#session-fragment-search')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('session_fragments', { query, limit: 10 });
      }
    });

    SC.$('#session-fragment-search-view')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('session_fragments', { query, limit: 10 });
      }
    });

    SC.$('#refresh-observations')?.addEventListener('click', () => {
      SC.wsSend('memory_observations', {});
    });

    SC.$('#refresh-observations-view')?.addEventListener('click', () => {
      SC.wsSend('memory_observations', {});
    });

    SC.$('#skill-editor-save')?.addEventListener('click', () => {
      if (!SC.state.editingSkill) return;
      const content = SC.$('#skill-editor-content')?.value || '';
      SC.wsSend('skill_edit', { name: SC.state.editingSkill, content });
    });

    SC.$('#skill-editor-cancel')?.addEventListener('click', () => {
      SC.state.editingSkill = null;
      SC.$('#skill-editor')?.classList.add('hidden');
    });

    SC.$('#skill-editor-save-view')?.addEventListener('click', () => {
      if (!SC.state.editingSkill) return;
      const content = SC.$('#skill-editor-content-view')?.value || '';
      SC.wsSend('skill_edit', { name: SC.state.editingSkill, content });
    });

    SC.$('#skill-editor-cancel-view')?.addEventListener('click', () => {
      SC.state.editingSkill = null;
      SC.$('#skill-editor-view')?.classList.add('hidden');
    });

    SC.$('#btn-skill-marketplace')?.addEventListener('click', () => {
      if (typeof SC.wsSend === 'function') SC.wsSend('skill_marketplace', {});
    });

    let wikiSearchTimer = null;
    SC.$('#wiki-search')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (wikiSearchTimer) clearTimeout(wikiSearchTimer);
      if (!query) {
        SC.wsSend('wiki_pages', {});
        return;
      }
      wikiSearchTimer = setTimeout(() => SC.wsSend('wiki_search', { query, limit: 5 }), 300);
    });

    SC.$('#wiki-search-view')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (wikiSearchTimer) clearTimeout(wikiSearchTimer);
      if (!query) {
        SC.wsSend('wiki_pages', {});
        return;
      }
      wikiSearchTimer = setTimeout(() => SC.wsSend('wiki_search', { query, limit: 5 }), 300);
    });

    SC.$('#session-search')?.addEventListener('input', () => {
      SC.renderSessions(SC.state.sessions || []);
    });

    SC.$('#session-search-view')?.addEventListener('input', () => {
      SC.renderSessions(SC.state.sessions || []);
    });

    SC.$('#help-close')?.addEventListener('click', SC.hideHelpModal);
    SC.$('#help-modal .modal-backdrop')?.addEventListener('click', SC.hideHelpModal);

    document.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'F') {
        e.preventDefault();
        if (SC.chatSearch) SC.chatSearch.open();
      }
      if (e.ctrlKey || e.metaKey) {
        if (e.key === 'k') {
          e.preventDefault();
          if (typeof SC.toggleCmdPalette === 'function') {
            SC.toggleCmdPalette();
          } else {
            SC.$('#input').focus(); SC.$('#input').value = '/'; SC.$('#input').dispatchEvent(new Event('input'));
          }
        }
        else if (e.key === 's' && SC.state.ui.editorFile) { e.preventDefault(); SC.saveEditor(); }
        else if (e.key === 'n') { e.preventDefault(); SC.wsSend('session_new', { model: SC.state.settings.model }); }
        else if (e.key === '/') {
          e.preventDefault();
          if (SC.isMobile()) SC.toggleSidebarDrawer();
          else {
            const sb = SC.$('#sidebar');
            if (sb) {
              sb.classList.toggle('collapsed');
              SC.state.ui.sidebarOpen = !sb.classList.contains('collapsed');
            }
          }
        }
        else if (e.key === 'p') { e.preventDefault(); SC.focusModelSwitcher(); }
        else if (e.key === 'o') { e.preventDefault(); SC.toggleSessionsPanel(); }
        else if (e.key === 'l') { e.preventDefault(); SC.clearChat(); }
        else if (e.key === 'h') { e.preventDefault(); SC.showHelpModal(); }
        else if (e.key === '\\') {
          e.preventDefault();
          SC.openEditor('', SC.state.ui.editorFile || 'untitled.go');
        }
      }
      if (e.key === 'Escape') {
        SC.closeEditor();
        SC.closeSkillCreateForm();
        var dp = SC.$('#dashboard-panel'); if (dp) dp.classList.remove('visible');
        var tp = SC.$('#theme-editor-panel'); if (tp) tp.classList.remove('visible');
        var cp = SC.$('#cmd-palette'); if (cp) cp.classList.add('hidden');
        var cmdOverlay = SC.$('.cmd-palette-overlay');
        if (cmdOverlay) cmdOverlay.classList.add('hidden');
        var fm = SC.$('#file-mention'); if (fm) fm.classList.add('hidden');
        SC.$('#help-modal')?.classList.add('hidden');
        SC.$('#bookmark-dropdown')?.classList.add('hidden');
        SC.$('#project-switcher-dropdown')?.classList.add('hidden');
        SC.state.mentionStart = -1;
      }
    });

    SC.$('#btn-attach').addEventListener('click', () => {
      const input = document.createElement('input');
      input.type = 'file';
      input.multiple = true;
      input.onchange = (e) => {
        Array.from(e.target.files).forEach(f => {
          const reader = new FileReader();
          reader.onload = (ev) => {
            const text = ev.target.result;
            const isText = typeof text === 'string' && text.length < 50000 && f.size <= 51200;
            if (isText) {
              const snippet = `\n\`\`\`${f.name}\n${text.slice(0, 10000)}${text.length > 10000 ? '\n... (truncated)' : ''}\n\`\`\`\n`;
              const inputEl = SC.$('#input');
              inputEl.value += snippet;
              inputEl.style.height = 'auto';
              inputEl.style.height = Math.min(inputEl.scrollHeight, 200) + 'px';
              SC.toast(`Added ${f.name}`, 'success');
            } else {
              SC.uploadFile(f);
            }
          };
          reader.readAsText(f);
        });
      };
      input.click();
    });

    SC.initMobileNav();
    SC.initSwipeGestures();
    SC.initViewportHandling();

    if (SC.onboarding) SC.onboarding.init();

    var onboardingBtn = SC.$('#nav-onboarding');
    if (onboardingBtn) {
      onboardingBtn.addEventListener('click', function() {
        if (SC.onboarding) SC.onboarding.show();
      });
    }

    SC.$('#rail-project-add')?.addEventListener('click', async function() {
      if (SC.isDesktop && window.runtime && window.runtime.OpenDirectoryDialog) {
        try {
          var dir = await window.runtime.OpenDirectoryDialog('Select Project Directory');
          if (dir) SC.wsSend('change_project', { path: dir });
          return;
        } catch (err) { if (err.name === 'AbortError') return; }
      }
      SC.openDirPicker();
    });

    var soundToggle = SC.$('#setting-sound-view') || SC.$('#setting-sound');
    if (soundToggle) {
      soundToggle.checked = SC.audio ? SC.audio.isSoundEnabled() : true;
      soundToggle.addEventListener('change', function() {
        if (SC.audio) SC.audio.setSoundEnabled(soundToggle.checked);
      });
    }

    var densitySelect = SC.$('#setting-density-view') || SC.$('#setting-density');
    if (densitySelect) {
      var savedDensity = localStorage.getItem('smartclaw-density') || 'comfortable';
      densitySelect.value = savedDensity;
      document.body.classList.toggle('density-compact', savedDensity === 'compact');
      densitySelect.addEventListener('change', function() {
        var val = densitySelect.value;
        localStorage.setItem('smartclaw-density', val);
        document.body.classList.toggle('density-compact', val === 'compact');
        SC.toast('Density: ' + val, 'info');
      });
    }

    SC.$('#setting-sound-view')?.addEventListener('change', function() {
      if (SC.audio) SC.audio.setSoundEnabled(this.checked);
    });
    SC.$('#setting-density-view')?.addEventListener('change', function() {
      var val = this.value;
      localStorage.setItem('smartclaw-density', val);
      document.body.classList.toggle('density-compact', val === 'compact');
      SC.toast('Density: ' + val, 'info');
    });
    SC.$('#watchdog-toggle-view')?.addEventListener('change', function() {
      SC.state.settings.watchdog = this.checked;
      SC.saveSettings();
      SC.toast(this.checked ? 'Watchdog enabled' : 'Watchdog disabled', 'info');
    });
    SC.$('#shortcuts-reset-view')?.addEventListener('click', function() {
      if (typeof SC.initShortcuts === 'function') { SC.initShortcuts(); SC.toast('Shortcuts reset', 'info'); }
    });
    SC.$('#btn-focus-mode-view')?.addEventListener('click', function() {
      document.body.classList.toggle('focus-mode');
      var isFocus = document.body.classList.contains('focus-mode');
      localStorage.setItem('smartclaw-focus-mode', isFocus ? 'true' : 'false');
      SC.toast(isFocus ? 'Focus mode on' : 'Focus mode off', 'info');
    });

    var focusBtn = SC.$('#btn-focus-mode');
    var focusBtnAlt = SC.$('#btn-focus-mode-alt');
    var focusExit = SC.$('#focus-exit');
    var focusToggleHandler = function() {
        document.body.classList.toggle('focus-mode');
        var isFocus = document.body.classList.contains('focus-mode');
        localStorage.setItem('smartclaw-focus-mode', isFocus ? 'true' : 'false');
        SC.toast(isFocus ? 'Focus mode on' : 'Focus mode off', 'info');
    };
    if (focusBtn) {
      focusBtn.addEventListener('click', focusToggleHandler);
    }
    if (focusBtnAlt) {
      focusBtnAlt.addEventListener('click', focusToggleHandler);
    }
    if (focusExit) {
      focusExit.addEventListener('click', function() {
        document.body.classList.remove('focus-mode');
        localStorage.setItem('smartclaw-focus-mode', 'false');
        SC.toast('Focus mode off', 'info');
      });
    }

    if (localStorage.getItem('smartclaw-focus-mode') === 'true') {
      document.body.classList.add('focus-mode');
    }

    detectProjectLanguage();

    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/static/sw.js').then(function(reg) {
        reg.onupdatefound = function() {
          var newWorker = reg.installing;
          newWorker.onstatechange = function() {
            if (newWorker.state === 'activated') {
              SC.toast('SmartClaw updated — refresh for latest version', 'info');
            }
          };
        };
      }).catch(function() {});
    }

    var deferredPrompt = null;
    window.addEventListener('beforeinstallprompt', function(e) {
      e.preventDefault();
      deferredPrompt = e;
      var installBtn = SC.$('#btn-install-app');
      if (installBtn) installBtn.style.display = '';
    });

    SC.installApp = function() {
      if (!deferredPrompt) return;
      deferredPrompt.prompt();
      deferredPrompt.userChoice.then(function() {
        deferredPrompt = null;
        var installBtn = SC.$('#btn-install-app');
        if (installBtn) installBtn.style.display = 'none';
      });
    };
  }

  function detectProjectLanguage() {
    var lang = '';
    var modelTag = SC.$('#current-model');
    if (modelTag) {
      var text = modelTag.textContent.toLowerCase();
      if (text.indexOf('go') >= 0) lang = 'go';
      else if (text.indexOf('typescript') >= 0 || text.indexOf('ts') >= 0) lang = 'typescript';
      else if (text.indexOf('javascript') >= 0 || text.indexOf('js') >= 0) lang = 'javascript';
      else if (text.indexOf('rust') >= 0) lang = 'rust';
      else if (text.indexOf('python') >= 0 || text.indexOf('py') >= 0) lang = 'python';
      else if (text.indexOf('java') >= 0) lang = 'java';
    }
    if (lang) {
      document.body.setAttribute('data-project-lang', lang);
      document.body.classList.add('has-project-accent');
    } else {
      document.body.removeAttribute('data-project-lang');
      document.body.classList.remove('has-project-accent');
    }
  }

  function showErrorTroubleshooter(errorMsg, container) {
    var ts = document.createElement('div');
    ts.className = 'error-troubleshooter';
    ts.innerHTML = '<div class="error-ts-header"><span class="error-ts-chevron">▸</span> Troubleshoot Error</div>' +
      '<div class="error-ts-body">' +
      '<div class="error-ts-step">Check error message and stack trace</div>' +
      '<div class="error-ts-step">Verify dependencies and configuration</div>' +
      '<div class="error-ts-step">Search for similar issues</div>' +
      '<button class="btn-ai-debug"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align:-1px;margin-right:2px"><path d="M12 1a3 3 0 00-3 3v8a3 3 0 006 0V4a3 3 0 00-3-3z"/><path d="M19 10v2a7 7 0 01-14 0v-2"/></svg>Debug with AI</button>' +
      '</div>';
    var header = ts.querySelector('.error-ts-header');
    header.addEventListener('click', function() { ts.classList.toggle('open'); });
    var debugBtn = ts.querySelector('.btn-ai-debug');
    debugBtn.addEventListener('click', function() {
      var input = SC.$('#chat-input');
      if (input) {
        input.value = 'Debug this error:\n```\n' + errorMsg + '\n```';
        input.focus();
      }
    });
    container.appendChild(ts);
  }

  SC.showErrorTroubleshooter = showErrorTroubleshooter;

  // Telemetry Dashboard
  (function() {
    function fetchTelemetry() {
      fetch('/api/telemetry')
        .then(r => r.json())
        .then(data => {
          const el = id => document.getElementById(id);
          if (el('t-queries')) el('t-queries').textContent = data.query_count || 0;
          if (el('t-input-tokens')) el('t-input-tokens').textContent = (data.total_input_tokens || 0).toLocaleString();
          if (el('t-output-tokens')) el('t-output-tokens').textContent = (data.total_output_tokens || 0).toLocaleString();
          if (el('t-cost')) el('t-cost').textContent = '$' + (data.estimated_cost_usd || 0).toFixed(4);
          if (el('t-cache-rate')) el('t-cache-rate').textContent = ((data.cache_hit_rate || 0) * 100).toFixed(1) + '%';
          if (el('t-avg-latency')) {
            const qCount = data.query_count || 0;
            const totalMs = data.query_total_time_ms || 0;
            el('t-avg-latency').textContent = qCount > 0 ? Math.round(totalMs / qCount) + 'ms' : '0ms';
          }

          const toolList = el('tool-exec-list');
          if (toolList && data.tool_executions) {
            toolList.innerHTML = '';
            const tools = Object.entries(data.tool_executions).sort((a, b) => b[1].Count - a[1].Count);
            tools.slice(0, 15).forEach(([name, stats]) => {
              const item = document.createElement('div');
              item.className = 'tool-exec-item';
              item.innerHTML = `<span class="tool-name">${SC.escapeHtml(name)}</span><span class="tool-stats">${stats.Count}x · ${stats.Errors}err · ${Math.round(stats.Duration / 1e6)}ms</span>`;
              toolList.appendChild(item);
            });
          }
        })
        .catch(() => {});
    }

    document.addEventListener('click', e => {
      if (e.target && e.target.id === 'telemetry-refresh') fetchTelemetry();
    });

    let telemetryInterval = null;
    const observer = new MutationObserver(() => {
      const panel = document.getElementById('dashboard-panel');
      if (panel && panel.classList.contains('visible')) {
        if (!telemetryInterval) {
          fetchTelemetry();
          telemetryInterval = setInterval(fetchTelemetry, 5000);
        }
      } else {
        if (telemetryInterval) {
          clearInterval(telemetryInterval);
          telemetryInterval = null;
        }
      }
    });
    observer.observe(document.body, { attributes: true, subtree: true, attributeFilter: ['class'] });

    SC._telemetryObserver = observer;
    SC._telemetryInterval = function() { return telemetryInterval; };
    SC.destroyTelemetry = function() {
      if (observer) observer.disconnect();
      if (telemetryInterval) { clearInterval(telemetryInterval); telemetryInterval = null; }
    };
  })();

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
