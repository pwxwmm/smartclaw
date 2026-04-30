// SmartClaw - Session Timeline
(function() {
  'use strict';

  var PANEL_WIDTH = 280;
  var SLIDE_MS = 250;
  var CHAPTER_SIZE = 6;
  var PREVIEW_MAX = 80;
  var SUMMARY_MAX = 60;
  var PURPLE = '#8b5cf6';

  var panel = null;
  var toggleBtn = null;
  var isOpen = false;
  var activeIdx = -1;
  var scrollObserver = null;

  function truncate(str, max) {
    if (!str) return '';
    var s = str.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
    return s.length > max ? s.slice(0, max) + '\u2026' : s;
  }

  function roleDotColor(role, toolName) {
    if (role === 'user') return PURPLE;
    if (role === 'tool' || role === 'cmd_result' || toolName) return 'var(--accent)';
    return 'var(--tx-2)';
  }

  function roleIcon(role, toolName) {
    if (role === 'user') return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="' + PURPLE + '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>';
    if (toolName) return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.77-3.77a6 6 0 01-7.94 7.94l-6.91 6.91a2.12 2.12 0 01-3-3l6.91-6.91a6 6 0 017.94-7.94l-3.76 3.76z"/></svg>';
    if (role === 'cmd_result') return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>';
    return '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--tx-2)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M12 2v4m0 12v4M2 12h4m12 0h4"/></svg>';
  }

  function formatTime(ts) {
    if (!ts) return '';
    var d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function buildChapters(messages) {
    var chapters = [];
    for (var i = 0; i < messages.length; i += CHAPTER_SIZE) {
      var group = messages.slice(i, i + CHAPTER_SIZE);
      var label = '';
      for (var j = 0; j < group.length; j++) {
        if (group[j].role === 'user' && group[j].content) {
          label = truncate(group[j].content, SUMMARY_MAX);
          break;
        }
      }
      if (!label) label = 'Chapter ' + (chapters.length + 1);
      chapters.push({ startIdx: i, label: label, count: group.length });
    }
    return chapters;
  }

  function createStyles() {
    var style = document.createElement('style');
    style.id = 'sc-timeline-styles';
    style.textContent =
      '.sc-tl-panel{position:fixed;top:0;right:0;width:' + PANEL_WIDTH + 'px;height:100vh;background:var(--bg-1);border-left:1px solid var(--bd);z-index:90;display:flex;flex-direction:column;transform:translateX(100%);transition:transform ' + SLIDE_MS + 'ms cubic-bezier(0.4,0,0.2,1);box-shadow:var(--sh-l);}' +
      '.sc-tl-panel.open{transform:translateX(0);}' +
      '.sc-tl-head{display:flex;align-items:center;justify-content:space-between;padding:10px 14px;border-bottom:1px solid var(--bd);background:var(--bg-2);flex-shrink:0;}' +
      '.sc-tl-title{font-family:var(--font-d);font-size:12px;font-weight:700;color:var(--tx-1);text-transform:uppercase;letter-spacing:0.8px;}' +
      '.sc-tl-close{display:flex;align-items:center;justify-content:center;width:26px;height:26px;background:none;border:1px solid var(--bd);color:var(--tx-2);cursor:pointer;border-radius:var(--r);transition:all var(--dur-s) var(--ease);font-size:14px;padding:0;}' +
      '.sc-tl-close:hover{color:var(--err);border-color:var(--err);background:rgba(224,82,82,0.1);}' +
      '.sc-tl-body{flex:1;overflow-y:auto;padding:12px 0;scroll-behavior:smooth;}' +
      '.sc-tl-body::-webkit-scrollbar{width:4px;}' +
      '.sc-tl-body::-webkit-scrollbar-thumb{background:var(--bd-h);border-radius:2px;}' +
      '.sc-tl-chapter{padding:6px 14px 6px 24px;}' +
      '.sc-tl-ch-label{font-family:var(--font-d);font-size:10px;font-weight:600;color:var(--accent);text-transform:uppercase;letter-spacing:0.6px;padding:3px 0;margin-bottom:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}' +
      '.sc-tl-node{position:relative;padding:6px 14px 6px 24px;cursor:pointer;transition:background var(--dur-s) var(--ease);}' +
      '.sc-tl-node:hover{background:var(--bg-hover);}' +
      '.sc-tl-node.active{background:var(--accent-bg);}' +
      '.sc-tl-node::before{content:"";position:absolute;left:14px;top:0;bottom:0;width:1px;background:var(--bd);}' +
      '.sc-tl-node:last-child::before{bottom:50%;}' +
      '.sc-tl-node:first-child::before{top:50%;}' +
      '.sc-tl-node:only-child::before{display:none;}' +
      '.sc-tl-dot{position:absolute;left:10px;top:14px;width:9px;height:9px;border-radius:50%;border:2px solid;transform:translateY(-50%);z-index:1;transition:box-shadow var(--dur-s) var(--ease);}' +
      '.sc-tl-node.active .sc-tl-dot{box-shadow:0 0 8px rgba(139,92,246,0.5);}' +
      '.sc-tl-row{display:flex;align-items:center;gap:6px;margin-bottom:2px;}' +
      '.sc-tl-time{font-family:var(--font-d);font-size:10px;color:var(--tx-2);flex-shrink:0;}' +
      '.sc-tl-icon{flex-shrink:0;display:flex;align-items:center;}' +
      '.sc-tl-tool{font-family:var(--font-d);font-size:10px;font-weight:600;color:var(--accent);background:var(--accent-bg);padding:1px 5px;border-radius:3px;border:1px solid var(--accent-bd);max-width:120px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}' +
      '.sc-tl-preview{font-size:12px;color:var(--tx-1);line-height:1.4;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:100%;}' +
      '.sc-tl-node.active .sc-tl-preview{color:var(--tx-0);}' +
      '.sc-tl-empty{padding:30px 20px;text-align:center;color:var(--tx-2);font-size:13px;}' +
      '.sc-tl-toggle{display:flex;align-items:center;justify-content:center;width:30px;height:30px;background:none;border:1px solid transparent;color:var(--tx-2);cursor:pointer;border-radius:var(--r);transition:all var(--dur-s) var(--ease);padding:0;}' +
      '.sc-tl-toggle:hover{color:var(--tx-0);background:var(--bg-hover);border-color:var(--bd);}' +
      '.sc-tl-toggle.active{color:' + PURPLE + ';border-color:' + PURPLE + '33;background:rgba(139,92,246,0.08);}' +
      '@media(max-width:768px){.sc-tl-panel{width:100%;}.sc-tl-toggle{display:none;}.sc-tl-mobile-toggle{display:flex !important;}}' +
      '.sc-tl-mobile-toggle{display:none;position:fixed;bottom:16px;right:16px;width:44px;height:44px;border-radius:50%;background:var(--bg-2);border:1px solid var(--bd);color:var(--tx-2);align-items:center;justify-content:center;cursor:pointer;box-shadow:var(--sh-m);z-index:89;transition:all var(--dur-s) var(--ease);}' +
      '.sc-tl-mobile-toggle:hover{color:' + PURPLE + ';border-color:' + PURPLE + '33;}';
    document.head.appendChild(style);
  }

  function createPanel() {
    panel = document.createElement('div');
    panel.className = 'sc-tl-panel';
    panel.innerHTML =
      '<div class="sc-tl-head">' +
        '<span class="sc-tl-title">Timeline</span>' +
        '<button class="sc-tl-close" aria-label="Close timeline">\u2715</button>' +
      '</div>' +
      '<div class="sc-tl-body"></div>';
    document.body.appendChild(panel);

    panel.querySelector('.sc-tl-close').addEventListener('click', close);
  }

  function createToggle() {
    var topbarRight = SC.$('.topbar-right');
    if (!topbarRight) return;

    toggleBtn = document.createElement('button');
    toggleBtn.className = 'sc-tl-toggle';
    toggleBtn.title = 'Toggle session timeline';
    toggleBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>';
    topbarRight.appendChild(toggleBtn);

    toggleBtn.addEventListener('click', function() {
      if (isOpen) close(); else open();
    });
  }

  function createMobileToggle() {
    var btn = document.createElement('button');
    btn.className = 'sc-tl-mobile-toggle';
    btn.innerHTML = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>';
    document.body.appendChild(btn);
    btn.addEventListener('click', function() {
      if (isOpen) close(); else open();
    });
  }

  function open() {
    isOpen = true;
    panel.classList.add('open');
    if (toggleBtn) toggleBtn.classList.add('active');
    renderTimeline();
    scrollToActive();
  }

  function close() {
    isOpen = false;
    panel.classList.remove('open');
    if (toggleBtn) toggleBtn.classList.remove('active');
  }

  function scrollToMessage(idx) {
    var chatEl = SC.$('#chat');
    if (!chatEl) return;
    var items = SC.vl ? SC.vl.items : SC.state.messages;
    if (idx < 0 || idx >= items.length) return;
    var msgEl = chatEl.querySelector('[data-msg-index="' + idx + '"]');
    if (msgEl) {
      msgEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
      msgEl.style.transition = 'background 0.3s';
      msgEl.style.background = 'var(--accent-bg)';
      setTimeout(function() { msgEl.style.background = ''; }, 1200);
    }
  }

  function scrollToActive() {
    setTimeout(function() {
      var body = panel.querySelector('.sc-tl-body');
      var activeNode = body.querySelector('.sc-tl-node.active');
      if (activeNode) activeNode.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }, SLIDE_MS + 50);
  }

  function updateActiveNode() {
    var chatEl = SC.$('#chat');
    if (!chatEl) return;
    var nodes = SC.$$('.sc-tl-node', panel);
    var items = SC.vl ? SC.vl.items : SC.state.messages;
    var chatRect = chatEl.getBoundingClientRect();
    var midY = chatRect.top + chatRect.height * 0.4;
    var bestIdx = -1;

    for (var i = items.length - 1; i >= 0; i--) {
      var msgEl = chatEl.querySelector('[data-msg-index="' + i + '"]');
      if (msgEl) {
        var r = msgEl.getBoundingClientRect();
        if (r.top <= midY) {
          bestIdx = i;
          break;
        }
      }
    }

    if (bestIdx === activeIdx) return;
    activeIdx = bestIdx;

    nodes.forEach(function(n) {
      var ni = parseInt(n.dataset.idx, 10);
      n.classList.toggle('active', ni === bestIdx);
    });
  }

  function renderTimeline() {
    if (!panel) return;
    var body = panel.querySelector('.sc-tl-body');
    var messages = SC.state.messages || [];
    if (!messages.length) {
      body.innerHTML = '<div class="sc-tl-empty">No messages yet</div>';
      return;
    }

    var chapters = buildChapters(messages);
    var html = '';
    var chapIdx = 0;

    for (var i = 0; i < messages.length; i++) {
      var msg = messages[i];
      var dotColor = roleDotColor(msg.role, msg.tool_name);
      var toolName = msg.tool_name || (msg.role === 'cmd_result' ? 'shell' : '');
      var isChapterStart = (i % CHAPTER_SIZE === 0);

      if (isChapterStart && chapIdx < chapters.length) {
        var chap = chapters[chapIdx];
        html += '<div class="sc-tl-chapter"><div class="sc-tl-ch-label">' + SC.escapeHtml(chap.label) + '</div></div>';
        chapIdx++;
      }

      var preview = truncate(msg.content, PREVIEW_MAX);
      var time = formatTime(msg.ts);

      html += '<div class="sc-tl-node' + (i === activeIdx ? ' active' : '') + '" data-idx="' + i + '">';
      html += '<div class="sc-tl-dot" style="background:' + dotColor + ';border-color:' + dotColor + ';"></div>';
      html += '<div class="sc-tl-row">';
      html += '<span class="sc-tl-time">' + SC.escapeHtml(time) + '</span>';
      html += '<span class="sc-tl-icon">' + roleIcon(msg.role, toolName) + '</span>';
      if (toolName) {
        html += '<span class="sc-tl-tool">' + SC.escapeHtml(toolName) + '</span>';
      }
      html += '</div>';
      html += '<div class="sc-tl-preview">' + SC.escapeHtml(preview) + '</div>';
      html += '</div>';
    }

    body.innerHTML = html;

    body.querySelectorAll('.sc-tl-node').forEach(function(node) {
      node.addEventListener('click', function() {
        var idx = parseInt(node.dataset.idx, 10);
        scrollToMessage(idx);
      });
    });
  }

  function setupScrollObserver() {
    var chatEl = SC.$('#chat');
    if (!chatEl) return;
    var ticking = false;
    chatEl.addEventListener('scroll', function() {
      if (!ticking) {
        requestAnimationFrame(function() {
          if (isOpen) updateActiveNode();
          ticking = false;
        });
        ticking = true;
      }
    });
  }

  function initTimeline() {
    if (panel) return;
    createStyles();
    createPanel();
    createToggle();
    createMobileToggle();
    setupScrollObserver();

    SC.subscribe('messages', function() {
      if (isOpen) renderTimeline();
    });

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && isOpen) close();
    });
  }

  SC.initTimeline = initTimeline;
  SC.renderTimeline = renderTimeline;
})();
