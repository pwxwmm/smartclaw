// SmartClaw - Spotlight 2.0
(function() {
  'use strict';

  var STORAGE_KEY = 'smartclaw-spotlight-usage';
  var OVERLAY_Z = 9500;
  var SLIDE_MS = 200;

  var CATEGORY_META = {
    Actions:    { icon: '⚡', color: '#f59e0b' },
    Navigation: { icon: '◈', color: '#3b82f6' },
    Tools:      { icon: '⚒', color: '#8b5cf6' },
    Commands:   { icon: '⟩', color: '#10b981' }
  };

  var COMMANDS = [
    { id: 'act-new-session',  label: 'New Session',  cat: 'Actions',    shortcut: 'Ctrl+N',  action: function() { SC.wsSend('session_new', { model: SC.state.settings.model }); } },
    { id: 'act-clear',        label: 'Clear',         cat: 'Actions',    shortcut: 'Ctrl+L',  action: function() { SC.clearChat(); } },
    { id: 'act-focus',        label: 'Focus Mode',    cat: 'Actions',    shortcut: null,       action: function() { var b = SC.$('#btn-focus-mode') || SC.$('#btn-focus-mode-alt'); if (b) b.click(); } },

    { id: 'nav-chat',      label: 'Chat',      cat: 'Navigation', view: 'chat',      action: function() { clickRail('chat'); } },
    { id: 'nav-sessions',  label: 'Sessions',  cat: 'Navigation', view: 'sessions',  action: function() { clickRail('sessions'); } },
    { id: 'nav-agents',    label: 'Agents',    cat: 'Navigation', view: 'agents',    action: function() { clickRail('agents'); } },
    { id: 'nav-skills',    label: 'Skills',     cat: 'Navigation', view: 'skills',    action: function() { clickRail('skills'); } },
    { id: 'nav-memory',    label: 'Memory',     cat: 'Navigation', view: 'memory',    action: function() { clickRail('memory'); } },
    { id: 'nav-settings',  label: 'Settings',   cat: 'Navigation', view: 'settings',  action: function() { clickRail('settings'); } },
    { id: 'nav-files',     label: 'Files',      cat: 'Navigation', view: 'files',     action: function() { clickRail('files'); } },
    { id: 'nav-wiki',      label: 'Wiki',       cat: 'Navigation', view: 'wiki',      action: function() { clickRail('wiki'); } },
    { id: 'nav-mcp',       label: 'MCP',        cat: 'Navigation', view: 'mcp',       action: function() { clickRail('mcp'); } },
    { id: 'nav-context',   label: 'Context',    cat: 'Navigation', view: 'context',   action: function() { clickRail('context'); } },
    { id: 'nav-cron',      label: 'Schedule',   cat: 'Navigation', view: 'cron',      action: function() { clickRail('cron'); } },
    { id: 'nav-cost',      label: 'Cost',       cat: 'Navigation', view: 'cost',      action: function() { clickRail('cost'); } },
    { id: 'nav-privacy',   label: 'Privacy',    cat: 'Navigation', view: 'privacy',   action: function() { clickRail('privacy'); } },
    { id: 'nav-workflows', label: 'Workflows',  cat: 'Navigation', view: 'workflows', action: function() { clickRail('workflows'); } },

    { id: 'tool-bash',           label: 'bash',           cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec bash '); } },
    { id: 'tool-read_file',      label: 'read_file',      cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/read '); } },
    { id: 'tool-write_file',     label: 'write_file',     cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/write '); } },
    { id: 'tool-edit_file',      label: 'edit_file',      cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec edit_file '); } },
    { id: 'tool-glob',           label: 'glob',           cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec glob '); } },
    { id: 'tool-grep',           label: 'grep',           cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec grep '); } },
    { id: 'tool-web_fetch',      label: 'web_fetch',      cat: 'Tools', contextView: null,       action: function() { insertCommand('/web '); } },
    { id: 'tool-web_search',     label: 'web_search',     cat: 'Tools', contextView: null,       action: function() { insertCommand('/web search '); } },
    { id: 'tool-ast_grep',       label: 'ast_grep',       cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec ast_grep '); } },
    { id: 'tool-lsp',            label: 'LSP',            cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/lsp '); } },
    { id: 'tool-code_search',    label: 'Code Search',    cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/exec code_search '); } },
    { id: 'tool-browser',        label: 'Browser',        cat: 'Tools', contextView: null,       action: function() { insertCommand('/browse '); } },
    { id: 'tool-mcp',            label: 'MCP Tool',       cat: 'Tools', contextView: 'mcp',     action: function() { insertCommand('/mcp '); } },
    { id: 'tool-execute_code',   label: 'Execute Code',   cat: 'Tools', contextView: null,       action: function() { insertCommand('/exec execute_code '); } },
    { id: 'tool-docker_exec',    label: 'Docker Exec',    cat: 'Tools', contextView: null,       action: function() { insertCommand('/exec docker_exec '); } },
    { id: 'tool-git_ai',         label: 'Git AI',         cat: 'Tools', contextView: 'files',   action: function() { insertCommand('/gc '); } },
    { id: 'tool-agent',          label: 'Agent',          cat: 'Tools', contextView: 'agents',  action: function() { insertCommand('/subagent '); } },
    { id: 'tool-skill',          label: 'Skill',          cat: 'Tools', contextView: 'skills',  action: function() { insertCommand('/exec skill '); } },
    { id: 'tool-memory',         label: 'Memory Tool',    cat: 'Tools', contextView: 'memory',  action: function() { insertCommand('/exec memory '); } },

    { id: 'cmd-help',    label: '/help',    cat: 'Commands', action: function() { insertCommand('/help '); } },
    { id: 'cmd-model',   label: '/model',   cat: 'Commands', action: function() { insertCommand('/model '); } },
    { id: 'cmd-theme',   label: '/theme',   cat: 'Commands', action: function() { insertCommand('/theme '); } },
    { id: 'cmd-memory',  label: '/memory',  cat: 'Commands', action: function() { insertCommand('/memory '); } },
    { id: 'cmd-compact', label: '/compact', cat: 'Commands', action: function() { insertCommand('/compact '); } },
    { id: 'cmd-skills',  label: '/skills',  cat: 'Commands', action: function() { insertCommand('/skills '); } },
    { id: 'cmd-agents',  label: '/agents',  cat: 'Commands', action: function() { insertCommand('/agents '); } },
    { id: 'cmd-export',  label: '/export',  cat: 'Commands', action: function() { insertCommand('/export '); } },
    { id: 'cmd-cost',    label: '/cost',    cat: 'Commands', action: function() { insertCommand('/cost '); } },
    { id: 'cmd-debug',   label: '/debug',   cat: 'Commands', action: function() { insertCommand('/debug '); } }
  ];

  var overlay = null;
  var input = null;
  var resultsEl = null;
  var selectedIdx = 0;
  var filtered = [];
  var isOpen = false;

  function clickRail(viewName) {
    var btn = SC.$('.sidebar-rail-btn[data-view="' + viewName + '"]');
    if (btn) btn.click();
  }

  function insertCommand(text) {
    var chatInput = SC.$('#input');
    if (chatInput) {
      chatInput.value = text;
      chatInput.focus();
      chatInput.dispatchEvent(new Event('input'));
    }
  }

  function loadUsage() {
    try { return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}'); }
    catch(e) { return {}; }
  }

  function saveUsage(data) {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(data)); } catch(e) {}
  }

  function recordUsage(cmdId) {
    var data = loadUsage();
    if (!data[cmdId]) data[cmdId] = { count: 0, lastUsed: 0 };
    data[cmdId].count++;
    data[cmdId].lastUsed = Date.now();
    saveUsage(data);
  }

  function getActiveView() {
    var activeBtn = SC.$('.sidebar-rail-btn.active');
    return activeBtn ? activeBtn.dataset.view : 'chat';
  }

  function calcScore(cmd, usage) {
    var u = usage[cmd.id] || { count: 0, lastUsed: 0 };
    var score = u.count * 10;

    if (u.lastUsed) {
      var elapsed = Date.now() - u.lastUsed;
      var hour = 3600000;
      var day = 86400000;
      var week = 604800000;
      if (elapsed < hour) score += 50;
      else if (elapsed < day) score += 20;
      else if (elapsed < week) score += 5;
    }

    var activeView = getActiveView();
    if (cmd.cat === 'Navigation' && cmd.view === activeView) score += 20;
    if (cmd.cat === 'Tools' && cmd.contextView === activeView) score += 20;
    if (cmd.cat === 'Actions' && activeView === 'chat') score += 5;

    return score;
  }

  function fuzzyMatch(query, text) {
    if (!query) return { matched: true, score: 0, indices: [] };
    var q = query.toLowerCase();
    var t = text.toLowerCase();
    var qi = 0;
    var indices = [];
    var score = 0;
    var lastIdx = -2;

    for (var ti = 0; ti < t.length && qi < q.length; ti++) {
      if (t[ti] === q[qi]) {
        indices.push(ti);
        if (lastIdx === ti - 1) score += 10;
        if (ti === 0) score += 5;
        score += 1;
        lastIdx = ti;
        qi++;
      }
    }

    if (qi < q.length) return { matched: false, score: 0, indices: [] };
    return { matched: true, score: score, indices: indices };
  }

  function highlightMatch(text, indices) {
    if (!indices || indices.length === 0) return SC.escapeHtml(text);
    var result = '';
    var idxSet = {};
    for (var i = 0; i < indices.length; i++) idxSet[indices[i]] = true;
    for (var j = 0; j < text.length; j++) {
      var ch = SC.escapeHtml(text[j]);
      result += idxSet[j] ? '<span class="sp-match">' + ch + '</span>' : ch;
    }
    return result;
  }

  function sortCommands(query) {
    var usage = loadUsage();
    var scored = [];

    for (var i = 0; i < COMMANDS.length; i++) {
      var cmd = COMMANDS[i];
      var nameMatch = fuzzyMatch(query, cmd.label);
      var catMatch = fuzzyMatch(query, cmd.cat);
      var best = nameMatch.matched ? nameMatch : (catMatch.matched ? catMatch : null);

      if (!query || best) {
        var usageScore = calcScore(cmd, usage);
        var fuzzyScore = best ? best.score : 0;
        var alphaBonus = (1000 - i) * 0.001;
        scored.push({
          cmd: cmd,
          totalScore: usageScore + fuzzyScore + alphaBonus,
          matchIndices: best && best === nameMatch ? best.indices : []
        });
      }
    }

    scored.sort(function(a, b) { return b.totalScore - a.totalScore; });
    return scored;
  }

  function injectStyles() {
    if (document.getElementById('spotlight-styles')) return;
    var s = document.createElement('style');
    s.id = 'spotlight-styles';
    s.textContent =
      '.sp-overlay{' +
        'position:fixed;inset:0;z-index:' + OVERLAY_Z + ';' +
        'background:rgba(0,0,0,0.55);' +
        'backdrop-filter:blur(6px);-webkit-backdrop-filter:blur(6px);' +
        'display:flex;justify-content:center;padding-top:min(18vh,140px);' +
        'opacity:0;transition:opacity ' + SLIDE_MS + 'ms cubic-bezier(0.4,0,0.2,1);' +
      '}' +
      '.sp-overlay.visible{opacity:1}' +
      '.sp-overlay.closing{opacity:0}' +
      '.sp-modal{' +
        'width:min(580px,90vw);max-height:min(460px,70vh);' +
        'background:var(--bg-1);border:1px solid var(--bd);border-radius:var(--r-l);' +
        'box-shadow:0 24px 80px rgba(0,0,0,0.6),0 0 0 1px rgba(139,92,246,0.1);' +
        'display:flex;flex-direction:column;overflow:hidden;' +
        'transform:translateY(-12px) scale(0.97);' +
        'transition:transform ' + SLIDE_MS + 'ms cubic-bezier(0.4,0,0.2,1);' +
      '}' +
      '.sp-overlay.visible .sp-modal{transform:translateY(0) scale(1)}' +
      '.sp-overlay.closing .sp-modal{transform:translateY(-8px) scale(0.98)}' +
      '.sp-input-row{' +
        'display:flex;align-items:center;gap:10px;' +
        'padding:14px 16px;border-bottom:1px solid var(--bd);' +
      '}' +
      '.sp-input-row svg{color:var(--tx-2);flex-shrink:0}' +
      '.sp-input{' +
        'flex:1;background:none;border:none;outline:none;' +
        'color:var(--tx-0);font-family:var(--font-b);font-size:15px;' +
        'caret-color:#8b5cf6;' +
      '}' +
      '.sp-input::placeholder{color:var(--tx-2)}' +
      '.sp-results{' +
        'flex:1;overflow-y:auto;padding:6px 0;' +
        'scrollbar-width:thin;scrollbar-color:var(--bd-h) transparent;' +
      '}' +
      '.sp-results::-webkit-scrollbar{width:4px}' +
      '.sp-results::-webkit-scrollbar-thumb{background:var(--bd-h);border-radius:2px}' +
      '.sp-cat-label{' +
        'padding:8px 16px 4px;font-size:10px;font-weight:700;' +
        'text-transform:uppercase;letter-spacing:0.8px;color:var(--tx-2);' +
      '}' +
      '.sp-item{' +
        'display:flex;align-items:center;gap:10px;' +
        'padding:8px 16px;cursor:pointer;' +
        'transition:background 80ms ease;' +
      '}' +
      '.sp-item:hover,.sp-item.selected{background:var(--bg-hover)}' +
      '.sp-item.selected{background:rgba(139,92,246,0.08)}' +
      '.sp-item-icon{' +
        'width:28px;height:28px;border-radius:6px;' +
        'display:flex;align-items:center;justify-content:center;' +
        'font-size:14px;flex-shrink:0;' +
      '}' +
      '.sp-item-icon.act{background:rgba(245,158,11,0.12);color:#f59e0b}' +
      '.sp-item-icon.nav{background:rgba(59,130,246,0.12);color:#3b82f6}' +
      '.sp-item-icon.tool{background:rgba(139,92,246,0.12);color:#8b5cf6}' +
      '.sp-item-icon.cmd{background:rgba(16,185,129,0.12);color:#10b981}' +
      '.sp-item-body{flex:1;min-width:0}' +
      '.sp-item-label{font-size:13px;color:var(--tx-0);font-weight:500}' +
      '.sp-item-sub{font-size:11px;color:var(--tx-2);margin-top:1px}' +
      '.sp-match{color:#8b5cf6;font-weight:700}' +
      '.sp-badge{' +
        'font-size:9px;font-weight:700;text-transform:uppercase;letter-spacing:0.5px;' +
        'padding:2px 6px;border-radius:3px;flex-shrink:0;' +
      '}' +
      '.sp-badge.act{background:rgba(245,158,11,0.1);color:#f59e0b;border:1px solid rgba(245,158,11,0.2)}' +
      '.sp-badge.nav{background:rgba(59,130,246,0.1);color:#3b82f6;border:1px solid rgba(59,130,246,0.2)}' +
      '.sp-badge.tool{background:rgba(139,92,246,0.1);color:#8b5cf6;border:1px solid rgba(139,92,246,0.2)}' +
      '.sp-badge.cmd{background:rgba(16,185,129,0.1);color:#10b981;border:1px solid rgba(16,185,129,0.2)}' +
      '.sp-shortcut{' +
        'font-family:var(--font-d);font-size:10px;color:var(--tx-2);' +
        'padding:2px 5px;background:var(--bg-3);border-radius:3px;flex-shrink:0;' +
      '}' +
      '.sp-footer{' +
        'display:flex;gap:16px;padding:8px 16px;' +
        'border-top:1px solid var(--bd);font-size:11px;color:var(--tx-2);' +
      '}' +
      '.sp-footer kbd{' +
        'font-family:var(--font-d);font-size:10px;' +
        'padding:1px 4px;background:var(--bg-3);border-radius:2px;' +
      '}' +
      '.sp-empty{' +
        'padding:24px 16px;text-align:center;color:var(--tx-2);font-size:13px;' +
      '}';
    document.head.appendChild(s);
  }

  function catClass(cat) {
    if (cat === 'Actions') return 'act';
    if (cat === 'Navigation') return 'nav';
    if (cat === 'Tools') return 'tool';
    if (cat === 'Commands') return 'cmd';
    return 'act';
  }

  function renderResults(query) {
    var scored = sortCommands(query);
    filtered = scored.map(function(s) { return s.cmd; });
    resultsEl.innerHTML = '';

    if (filtered.length === 0) {
      resultsEl.innerHTML = '<div class="sp-empty">No matching results</div>';
      selectedIdx = -1;
      return;
    }

    selectedIdx = 0;
    var currentCat = null;

    for (var i = 0; i < scored.length; i++) {
      var cmd = scored[i].cmd;
      var indices = scored[i].matchIndices;

      if (cmd.cat !== currentCat) {
        currentCat = cmd.cat;
        var catEl = document.createElement('div');
        catEl.className = 'sp-cat-label';
        catEl.textContent = currentCat;
        resultsEl.appendChild(catEl);
      }

      var item = document.createElement('div');
      item.className = 'sp-item' + (i === selectedIdx ? ' selected' : '');
      item.dataset.index = i;

      var cls = catClass(cmd.cat);
      var labelHtml = highlightMatch(cmd.label, indices);

      item.innerHTML =
        '<div class="sp-item-icon ' + cls + '">' + CATEGORY_META[cmd.cat].icon + '</div>' +
        '<div class="sp-item-body">' +
          '<div class="sp-item-label">' + labelHtml + '</div>' +
        '</div>' +
        '<span class="sp-badge ' + cls + '">' + SC.escapeHtml(cmd.cat) + '</span>' +
        (cmd.shortcut ? '<span class="sp-shortcut">' + SC.escapeHtml(cmd.shortcut) + '</span>' : '');

      item.addEventListener('click', onSelect.bind(null, i));
      item.addEventListener('mouseenter', onHover.bind(null, i));
      resultsEl.appendChild(item);
    }

    scrollIntoView();
  }

  function onSelect(idx) {
    var cmd = filtered[idx];
    if (!cmd) return;
    recordUsage(cmd.id);
    close();
    cmd.action();
  }

  function onHover(idx) {
    selectedIdx = idx;
    updateSelection();
  }

  function updateSelection() {
    var items = resultsEl.querySelectorAll('.sp-item');
    for (var i = 0; i < items.length; i++) {
      var itemIdx = parseInt(items[i].dataset.index, 10);
      items[i].classList.toggle('selected', itemIdx === selectedIdx);
    }
    scrollIntoView();
  }

  function scrollIntoView() {
    var sel = resultsEl.querySelector('.sp-item.selected');
    if (sel) sel.scrollIntoView({ block: 'nearest' });
  }

  function open() {
    if (isOpen) return;
    injectStyles();

    var cp = SC.$('#cmd-palette');
    if (cp) cp.classList.add('hidden');
    var cpo = SC.$('.cmd-palette-overlay');
    if (cpo) cpo.classList.add('hidden');

    overlay = document.createElement('div');
    overlay.className = 'sp-overlay';
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-label', 'Spotlight');

    var modal = document.createElement('div');
    modal.className = 'sp-modal';

    var inputRow = document.createElement('div');
    inputRow.className = 'sp-input-row';
    inputRow.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>';
    input = document.createElement('input');
    input.className = 'sp-input';
    input.placeholder = 'Search commands, tools, navigation...';
    input.autocomplete = 'off';
    input.setAttribute('aria-label', 'Spotlight search');
    inputRow.appendChild(input);

    resultsEl = document.createElement('div');
    resultsEl.className = 'sp-results';

    var footer = document.createElement('div');
    footer.className = 'sp-footer';
    footer.innerHTML = '<span><kbd>↑↓</kbd> Navigate</span><span><kbd>↵</kbd> Select</span><span><kbd>Esc</kbd> Close</span>';

    modal.appendChild(inputRow);
    modal.appendChild(resultsEl);
    modal.appendChild(footer);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    renderResults('');

    requestAnimationFrame(function() {
      overlay.classList.add('visible');
      input.focus();
    });

    isOpen = true;

    input.addEventListener('input', function() {
      renderResults(input.value.trim());
    });

    input.addEventListener('keydown', function(e) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (filtered.length > 0) {
          selectedIdx = (selectedIdx + 1) % filtered.length;
          updateSelection();
        }
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (filtered.length > 0) {
          selectedIdx = (selectedIdx - 1 + filtered.length) % filtered.length;
          updateSelection();
        }
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (selectedIdx >= 0 && filtered[selectedIdx]) {
          onSelect(selectedIdx);
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        close();
      }
    });

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) close();
    });
  }

  function close() {
    if (!overlay || !isOpen) return;
    isOpen = false;
    overlay.classList.add('closing');
    overlay.classList.remove('visible');
    setTimeout(function() {
      if (overlay && overlay.parentNode) overlay.parentNode.removeChild(overlay);
      overlay = null;
      input = null;
      resultsEl = null;
      filtered = [];
      selectedIdx = 0;
    }, SLIDE_MS);
  }

  function toggle() {
    if (isOpen) close();
    else open();
  }

  function interceptCtrlK(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      e.stopPropagation();
      toggle();
    }
  }

  function handleGlobalEscape(e) {
    if (e.key === 'Escape' && isOpen) {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  }

  function initSpotlight() {
    document.addEventListener('keydown', interceptCtrlK, true);
    document.addEventListener('keydown', handleGlobalEscape, true);
  }

  SC.initSpotlight = initSpotlight;
  SC.toggleSpotlight = toggle;
})();
