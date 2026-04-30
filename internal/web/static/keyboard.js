// SmartClaw - Keyboard-first Workflow
(function() {
  'use strict';

  var SHORTCUTS = [
    { key: 'Ctrl+J', action: 'toggleSessions', label: 'Sessions', desc: 'Toggle sessions panel' },
    { key: 'Ctrl+Shift+A', action: 'toggleAgents', label: 'Agents', desc: 'Toggle agents view' },
    { key: 'Ctrl+Shift+D', action: 'toggleDashboard', label: 'Dashboard', desc: 'Toggle dashboard panel' },
    { key: 'Ctrl+Shift+F', action: 'toggleSearch', label: 'Search', desc: 'Open message search' },
    { key: 'Ctrl+Shift+N', action: 'newSession', label: 'New Session', desc: 'Start a new chat session' },
    { key: 'Ctrl+Shift+P', action: 'togglePanels', label: 'Panels', desc: 'Toggle panel area' },
    { key: 'Ctrl+Shift+/', action: 'showOverlay', label: 'All Shortcuts', desc: 'Show full shortcuts reference' }
  ];

  var overlay = null;
  var overlayVisible = false;

  function matches(e, shortcut) {
    var parts = shortcut.key.split('+');
    var needCtrl = parts.indexOf('Ctrl') >= 0;
    var needShift = parts.indexOf('Shift') >= 0;
    var key = parts[parts.length - 1];

    var ctrlOk = needCtrl ? (e.ctrlKey || e.metaKey) : !(e.ctrlKey || e.metaKey);
    var shiftOk = needShift ? e.shiftKey : !e.shiftKey;
    var keyOk = e.key === key || e.key.toLowerCase() === key.toLowerCase();

    return ctrlOk && shiftOk && keyOk;
  }

  function executeAction(action) {
    switch (action) {
      case 'toggleSessions':
        SC.wsSend('session_list', {});
        var sb = SC.$('#sidebar');
        if (sb) {
          var sessionsNav = sb.querySelector('[data-section="sessions"]');
          if (sessionsNav) sessionsNav.click();
        }
        break;
      case 'toggleAgents':
        SC.wsSend('agent_list', {});
        var railBtn = SC.$('.sidebar-rail-btn[data-view="agents"]');
        if (railBtn) railBtn.click();
        break;
      case 'toggleDashboard':
        SC.openDashboard();
        break;
      case 'toggleSearch':
        if (SC.chatSearch) SC.chatSearch.open();
        break;
      case 'newSession':
        SC.wsSend('session_new', { model: SC.state.settings.model });
        break;
      case 'togglePanels':
        var pa = SC.$('#panel-area');
        if (pa) pa.classList.toggle('visible');
        break;
      case 'showOverlay':
        toggleOverlay();
        break;
    }
  }

  function createOverlay() {
    if (overlay) return overlay;

    overlay = document.createElement('div');
    overlay.id = 'keyboard-overlay';
    overlay.className = 'keyboard-overlay hidden';

    var inner = document.createElement('div');
    inner.className = 'keyboard-overlay-inner';

    var header = document.createElement('div');
    header.className = 'keyboard-overlay-header';
    header.innerHTML = '<span class="keyboard-overlay-title">Keyboard Shortcuts</span><button class="keyboard-overlay-close" aria-label="Close">&times;</button>';
    inner.appendChild(header);

    var grid = document.createElement('div');
    grid.className = 'keyboard-shortcut-grid';

    SHORTCUTS.forEach(function(s) {
      var row = document.createElement('div');
      row.className = 'keyboard-shortcut-row';

      var keys = document.createElement('span');
      keys.className = 'shortcut-keys';
      s.key.split('+').forEach(function(k, i) {
        if (i > 0) keys.appendChild(document.createTextNode(' + '));
        var kbd = document.createElement('kbd');
        kbd.textContent = k;
        keys.appendChild(kbd);
      });

      var label = document.createElement('span');
      label.className = 'shortcut-label';
      label.textContent = s.label;

      var desc = document.createElement('span');
      desc.className = 'shortcut-desc';
      desc.textContent = s.desc;

      row.appendChild(keys);
      row.appendChild(label);
      row.appendChild(desc);
      grid.appendChild(row);
    });

    inner.appendChild(grid);

    var extraSection = document.createElement('div');
    extraSection.className = 'keyboard-extra-section';
    extraSection.innerHTML = '<div class="keyboard-extra-title">Built-in</div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>Ctrl</kbd>+<kbd>S</kbd></span><span class="shortcut-label">Save</span><span class="shortcut-desc">Save file in editor</span></div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>Ctrl</kbd>+<kbd>N</kbd></span><span class="shortcut-label">New</span><span class="shortcut-desc">New session</span></div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>Ctrl</kbd>+<kbd>P</kbd></span><span class="shortcut-label">Model</span><span class="shortcut-desc">Model switcher</span></div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>Ctrl</kbd>+<kbd>O</kbd></span><span class="shortcut-label">Sessions</span><span class="shortcut-desc">Open sessions</span></div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>Esc</kbd></span><span class="shortcut-label">Close</span><span class="shortcut-desc">Close panels/modals</span></div>' +
      '<div class="keyboard-shortcut-row"><span class="shortcut-keys"><kbd>↑</kbd> / <kbd>↓</kbd></span><span class="shortcut-label">History</span><span class="shortcut-desc">Navigate command history</span></div>';
    inner.appendChild(extraSection);

    overlay.appendChild(inner);
    document.body.appendChild(overlay);

    overlay.querySelector('.keyboard-overlay-close').addEventListener('click', hideOverlay);
    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) hideOverlay();
    });

    return overlay;
  }

  function toggleOverlay() {
    if (overlayVisible) {
      hideOverlay();
    } else {
      showOverlay();
    }
  }

  function showOverlay() {
    createOverlay();
    overlay.classList.remove('hidden');
    overlayVisible = true;
  }

  function hideOverlay() {
    if (overlay) overlay.classList.add('hidden');
    overlayVisible = false;
  }

  function initKeyboard() {
    createOverlay();

    document.addEventListener('keydown', function(e) {
      if (isInInput(e)) return;

      for (var i = 0; i < SHORTCUTS.length; i++) {
        if (matches(e, SHORTCUTS[i])) {
          e.preventDefault();
          e.stopPropagation();
          executeAction(SHORTCUTS[i].action);
          return;
        }
      }

      if (e.key === 'Escape' && overlayVisible) {
        hideOverlay();
      }
    }, true);
  }

  function isInInput(e) {
    var tag = e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
    if (e.target.isContentEditable) return true;
    return false;
  }

  SC.initKeyboard = initKeyboard;
  SC.showKeyboardOverlay = showOverlay;
  SC.hideKeyboardOverlay = hideOverlay;
  SC.getShortcuts = function() { return SHORTCUTS; };
})();
