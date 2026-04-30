(function() {
  'use strict';

  var TOOL_PANEL_MAP = {
    'edit_file': 'canvas',
    'write_file': 'canvas',
    'read_file': 'canvas',
    'bash': null,
    'browser_navigate': null,
    'browser_screenshot': null,
    'web_fetch': null,
    'web_search': null,
    'git_status': null,
    'git_diff': null,
    'git_log': null,
    'watchdog_start': 'watchdog',
    'watchdog_stop': 'watchdog',
    'watchdog_status': 'watchdog',
    'dap_start': 'watchdog',
    'mcp': 'mcp',
    'lsp': 'canvas',
    'ast_grep': 'canvas',
    'code_search': 'canvas'
  };

  var autoOpenedPanels = {};
  var AUTO_COLLAPSE_DELAY = 8000;

  function onToolStart(msg) {
    var toolName = msg.tool || '';
    var panelId = TOOL_PANEL_MAP[toolName];
    if (!panelId) return;

    if (typeof SC.createPanel === 'function') {
      SC.createPanel(panelId, getPanelTitle(panelId));
    }
    autoOpenedPanels[panelId] = true;
    SC.toast('Auto-opened ' + panelId + ' panel', 'info');
  }

  function onToolEnd(msg) {
    var toolName = '';
    var card = SC.$('#tool-' + (msg.id || SC.state.currentToolId));
    if (card) toolName = card.dataset.toolName || '';

    var panelId = TOOL_PANEL_MAP[toolName];
    if (!panelId || !autoOpenedPanels[panelId]) return;

    setTimeout(function() {
      if (autoOpenedPanels[panelId]) {
        delete autoOpenedPanels[panelId];
        if (typeof SC.showPanel === 'function') {
          var remaining = Object.keys(autoOpenedPanels);
          if (remaining.length > 0) {
            SC.showPanel(remaining[remaining.length - 1]);
          }
        }
      }
    }, AUTO_COLLAPSE_DELAY);
  }

  function getPanelTitle(id) {
    var titles = {
      canvas: 'Code Editor',
      watchdog: 'Watchdog',
      mcp: 'MCP',
      privacy: 'Privacy Audit',
      artifacts: 'Preview'
    };
    return titles[id] || id;
  }

  function initAdaptivePanels() {}

  SC.adaptiveOnToolStart = onToolStart;
  SC.adaptiveOnToolEnd = onToolEnd;
  SC.initAdaptivePanels = initAdaptivePanels;
})();
