(function() {
  'use strict';

  var branches = {};
  var BRANCH_KEY = 'smartclaw-branches';

  function saveBranches() {
    try {
      var sessionId = SC.state.ui.currentSessionId;
      if (!sessionId) return;
      var allBranches = JSON.parse(localStorage.getItem(BRANCH_KEY) || '{}');
      allBranches[sessionId] = branches;
      localStorage.setItem(BRANCH_KEY, JSON.stringify(allBranches));
    } catch(e) {}
  }

  function loadBranches() {
    try {
      var sessionId = SC.state.ui.currentSessionId;
      if (!sessionId) { branches = {}; return; }
      var allBranches = JSON.parse(localStorage.getItem(BRANCH_KEY) || '{}');
      branches = allBranches[sessionId] || {};
    } catch(e) { branches = {}; }
  }

  function recordBranch(forkIndex, originalMessages, newMessages) {
    var branchId = 'branch-' + Date.now();
    branches[branchId] = {
      forkIndex: forkIndex,
      timestamp: Date.now(),
      originalCount: originalMessages.length,
      newCount: newMessages.length,
      preview: (newMessages[0] || {}).content || ''
    };
    renderBranchIndicator(forkIndex, branchId);
    saveBranches();
  }

  function renderBranchIndicator(forkIndex, branchId) {
    if (SC.vl) {
      var el = SC.vl.getItemElement(forkIndex);
      if (!el) return;
    } else {
      var el = SC.$('#messages .message[data-msg-index="' + forkIndex + '"]');
    }
    if (!el) return;

    var existing = el.querySelector('.branch-indicator');
    if (existing) {
      var count = Object.keys(branches).length;
      existing.querySelector('.branch-count').textContent = count + ' branch' + (count > 1 ? 'es' : '');
      return;
    }

    var indicator = document.createElement('div');
    indicator.className = 'branch-indicator';
    indicator.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="2" x2="12" y2="6"/><path d="M12 6C12 6 6 10 6 16"/><path d="M12 6C12 6 18 10 18 16"/></svg><span class="branch-count">' + Object.keys(branches).length + ' branch' + (Object.keys(branches).length > 1 ? 'es' : '') + '</span>';
    indicator.addEventListener('click', function(e) {
      e.stopPropagation();
      showBranchMenu(forkIndex);
    });
    el.appendChild(indicator);
  }

  function showBranchMenu(forkIndex) {
    var menu = SC.$('#branch-menu');
    if (!menu) {
      menu = document.createElement('div');
      menu.id = 'branch-menu';
      menu.className = 'branch-menu';
      document.body.appendChild(menu);
    }
    var html = '<div class="branch-menu-header">Conversation Branches</div>';
    var keys = Object.keys(branches);
    keys.forEach(function(bid) {
      var b = branches[bid];
      var time = new Date(b.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      html += '<div class="branch-menu-item" data-branch="' + SC.escapeHtml(bid) + '"><span class="branch-menu-preview">' + SC.escapeHtml((b.preview || '').slice(0, 60)) + '</span><span class="branch-menu-time">' + time + '</span></div>';
    });
    html += '<div class="branch-menu-hint">Branches are tracked automatically on retry/edit</div>';
    menu.innerHTML = html;
    menu.style.display = 'block';

    var rect = SC.$('#messages').getBoundingClientRect();
    menu.style.top = (rect.top + 60) + 'px';
    menu.style.left = (rect.left + 20) + 'px';

    setTimeout(function() {
      document.addEventListener('click', function closeBranch(e) {
        if (!menu.contains(e.target)) {
          menu.style.display = 'none';
          document.removeEventListener('click', closeBranch);
        }
      });
    }, 100);
  }

  function initBranches() {
    loadBranches();
  }

  SC.recordBranch = recordBranch;
  SC.initBranches = initBranches;
})();
