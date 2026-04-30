// SmartClaw - Keyboard Shortcuts Editor
(function() {
  'use strict';

  var SHORTCUTS_KEY = 'smartclaw-shortcuts';

  var DEFAULT_SHORTCUTS = [
    { id: 'focus_input', label: 'Focus Input', keys: 'Ctrl+K', category: 'Core' },
    { id: 'save_file', label: 'Save File', keys: 'Ctrl+S', category: 'Core' },
    { id: 'new_session', label: 'New Session', keys: 'Ctrl+N', category: 'Session' },
    { id: 'toggle_sidebar', label: 'Toggle Sidebar', keys: 'Ctrl+/', category: 'UI' },
    { id: 'model_switch', label: 'Switch Model', keys: 'Ctrl+P', category: 'Model' },
    { id: 'sessions_panel', label: 'Sessions Panel', keys: 'Ctrl+O', category: 'Session' },
    { id: 'clear_chat', label: 'Clear Chat', keys: 'Ctrl+L', category: 'Core' },
    { id: 'help', label: 'Help', keys: 'Ctrl+H', category: 'UI' },
    { id: 'search', label: 'Search Messages', keys: 'Ctrl+Shift+F', category: 'Search' },
  ];

  function getShortcuts() {
    try {
      var saved = localStorage.getItem(SHORTCUTS_KEY);
      if (saved) {
        var custom = JSON.parse(saved);
        var map = {};
        DEFAULT_SHORTCUTS.forEach(function(s) { map[s.id] = s; });
        custom.forEach(function(s) { if (map[s.id]) map[s.id] = s; });
        return Object.values(map);
      }
    } catch {}
    return DEFAULT_SHORTCUTS.slice();
  }

  function saveShortcuts(shortcuts) {
    var custom = shortcuts.filter(function(s) {
      var def = DEFAULT_SHORTCUTS.find(function(d) { return d.id === s.id; });
      return def && def.keys !== s.keys;
    });
    try { localStorage.setItem(SHORTCUTS_KEY, JSON.stringify(custom)); } catch {}
  }

  function resetShortcuts() {
    try { localStorage.removeItem(SHORTCUTS_KEY); } catch {}
    SC.toast('Shortcuts reset to defaults', 'success');
    renderShortcutsEditor();
  }

  function renderShortcutsEditor() {
    var container = SC.$('#shortcuts-editor');
    if (!container) return;

    var shortcuts = getShortcuts();
    var categories = {};
    shortcuts.forEach(function(s) {
      if (!categories[s.category]) categories[s.category] = [];
      categories[s.category].push(s);
    });

    var html = '';
    Object.keys(categories).sort().forEach(function(cat) {
      html += '<div class="shortcut-category">' + SC.escapeHtml(cat) + '</div>';
      categories[cat].forEach(function(s) {
        var keys = s.keys.split('+');
        var keysHtml = keys.map(function(k) { return '<kbd>' + SC.escapeHtml(k.trim()) + '</kbd>'; }).join('<span class="shortcut-plus">+</span>');
        html += '<div class="shortcut-row" data-shortcut-id="' + SC.escapeHtml(s.id) + '">' +
          '<span class="shortcut-label">' + SC.escapeHtml(s.label) + '</span>' +
          '<span class="shortcut-keys">' + keysHtml + '</span>' +
          '<button class="shortcut-edit-btn" data-id="' + SC.escapeHtml(s.id) + '" title="Edit shortcut">✎</button>' +
        '</div>';
      });
    });

    html += '<div class="shortcut-actions">' +
      '<button class="btn-ghost sm" id="shortcuts-reset">Reset to Defaults</button>' +
    '</div>';

    container.innerHTML = html;

    container.querySelectorAll('.shortcut-edit-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        startEditShortcut(btn.dataset.id);
      });
    });

    var resetBtn = SC.$('#shortcuts-reset');
    if (resetBtn) resetBtn.addEventListener('click', resetShortcuts);
  }

  function startEditShortcut(shortcutId) {
    var shortcuts = getShortcuts();
    var shortcut = shortcuts.find(function(s) { return s.id === shortcutId; });
    if (!shortcut) return;

    var row = SC.$('.shortcut-row[data-shortcut-id="' + shortcutId + '"]');
    if (!row) return;

    var keysSpan = row.querySelector('.shortcut-keys');
    if (!keysSpan) return;

    var originalHtml = keysSpan.innerHTML;
    keysSpan.innerHTML = '<span class="shortcut-recording">Press keys…</span>';
    keysSpan.classList.add('recording');

    function onKeyDown(e) {
      e.preventDefault();
      e.stopPropagation();

      var parts = [];
      if (e.ctrlKey || e.metaKey) parts.push('Ctrl');
      if (e.shiftKey) parts.push('Shift');
      if (e.altKey) parts.push('Alt');

      var key = e.key;
      if (key === 'Control' || key === 'Shift' || key === 'Alt' || key === 'Meta') return;

      if (key === ' ') key = 'Space';
      else if (key === 'Escape') { cancelEdit(); return; }
      else key = key.length === 1 ? key.toUpperCase() : key;

      parts.push(key);
      var combo = parts.join('+');

      shortcut.keys = combo;
      saveShortcuts(shortcuts);
      SC.toast(shortcut.label + ': ' + combo, 'success');

      cleanup();
      renderShortcutsEditor();
    }

    function cancelEdit() {
      keysSpan.innerHTML = originalHtml;
      keysSpan.classList.remove('recording');
      cleanup();
    }

    function cleanup() {
      document.removeEventListener('keydown', onKeyDown, true);
    }

    document.addEventListener('keydown', onKeyDown, true);

    setTimeout(function() {
      if (keysSpan.classList.contains('recording')) cancelEdit();
    }, 5000);
  }

  SC.getShortcuts = getShortcuts;
  SC.renderShortcutsEditor = renderShortcutsEditor;
})();
