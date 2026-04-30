// SmartClaw - Shared Utilities
(function() {
  'use strict';
  window.SC = window.SC || {};

  SC.$ = (s, p) => (p || document).querySelector(s);
  SC.$$ = (s, p) => [...(p || document).querySelectorAll(s)];

  SC.escapeHtml = function(str) {
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  };

  SC.getCSS = function(prop) {
    return getComputedStyle(document.documentElement).getPropertyValue(prop).trim();
  };

  SC.rgbToHex = function(rgb) {
    if (!rgb || rgb.startsWith('#')) return rgb || '#000000';
    const m = rgb.match(/\d+/g);
    if (!m || m.length < 3) return '#000000';
    return '#' + m.slice(0, 3).map(x => parseInt(x).toString(16).padStart(2, '0')).join('');
  };

  SC.showErrorBanner = function(message, retryFn) {
    let banner = document.getElementById('error-banner');
    if (!banner) {
      banner = document.createElement('div');
      banner.id = 'error-banner';
      banner.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:10000;background:#7f1d1d;color:#fca5a5;padding:10px 16px;display:flex;align-items:center;gap:12px;font-size:13px;box-shadow:0 2px 8px rgba(0,0,0,0.4);';
      document.body.prepend(banner);
    }
    banner.innerHTML = '';
    var msgSpan = document.createElement('span');
    msgSpan.style.flex = '1';
    msgSpan.textContent = '\u26A0 ' + message;
    banner.appendChild(msgSpan);
    if (retryFn) {
      var retryBtn = document.createElement('button');
      retryBtn.textContent = 'Retry';
      retryBtn.style.cssText = 'background:#991b1b;color:#fca5a5;border:1px solid #dc2626;padding:4px 14px;border-radius:4px;cursor:pointer;font-size:12px;font-weight:600;';
      retryBtn.addEventListener('click', function() { banner.remove(); retryFn(); });
      banner.appendChild(retryBtn);
    }
    var dismissBtn = document.createElement('button');
    dismissBtn.textContent = '\u2715';
    dismissBtn.style.cssText = 'background:none;border:none;color:#fca5a5;cursor:pointer;font-size:16px;padding:0 4px;';
    dismissBtn.addEventListener('click', function() { banner.remove(); });
    banner.appendChild(dismissBtn);
  };

  SC.errorBoundary = function(fn, sectionName) {
    return function() {
      try {
        return fn.apply(this, arguments);
      } catch (err) {
        console.error('[ErrorBoundary] ' + sectionName + ':', err);
        SC.showErrorBanner(sectionName + ' error: ' + err.message, function() { fn.apply(this, arguments); });
      }
    };
  };

  window.onerror = function(message, source, lineno, colno, error) {
    console.error('[Global Error]', message, source, lineno, colno, error);
    var msgText = typeof message === 'string' ? message.slice(0, 120) : 'Unknown error';
    var toastEl = document.createElement('div');
    toastEl.className = 'toast err';
    toastEl.innerHTML = '<span style="flex:1">' + SC.escapeHtml(msgText) + '</span><button style="background:none;border:none;color:#fca5a5;cursor:pointer;font-size:14px;margin-left:8px" aria-label="Dismiss">\u2715</button>';
    toastEl.style.display = 'flex';
    toastEl.style.alignItems = 'center';
    var container = document.getElementById('toast-container');
    if (container) {
      container.appendChild(toastEl);
      toastEl.querySelector('button').addEventListener('click', function() { toastEl.remove(); });
      setTimeout(function() { toastEl.style.opacity = '0'; setTimeout(function() { toastEl.remove(); }, 300); }, 6000);
    }
    return true;
  };

  window.addEventListener('unhandledrejection', function(event) {
    console.error('[Unhandled Promise]', event.reason);
    var msg = event.reason instanceof Error ? event.reason.message : String(event.reason);
    if (typeof SC.toast === 'function') {
      SC.toast('Async error: ' + msg.slice(0, 80), 'error');
    }
  });

  SC.showSkeleton = function(container, count) {
    count = count || 3;
    var html = '';
    for (var i = 0; i < count; i++) {
      html += '<div class="skeleton-block skeleton"><div class="skeleton-text skeleton w-75"></div><div class="skeleton-text skeleton w-50"></div><div class="skeleton-text skeleton w-33"></div></div>';
    }
    container.innerHTML = html;
  };

  SC.hideSkeleton = function(container) {
    var skeletons = container.querySelectorAll('.skeleton-block, .skeleton-text, .skeleton-circle');
    skeletons.forEach(function(el) { el.remove(); });
  };

  SC.showEmptyState = function(container, icon, title, desc) {
    container.innerHTML = '<div class="empty-state">' + (icon || '') + '<span class="empty-title">' + SC.escapeHtml(title) + '</span>' + (desc ? '<span class="empty-desc">' + SC.escapeHtml(desc) + '</span>' : '') + '</div>';
  };

  SC.showContextMenu = function(x, y, items) {
    var menu = document.getElementById('msg-context-menu');
    if (!menu) return;

    menu.innerHTML = '';
    items.forEach(function(item) {
      var menuItem = document.createElement('div');
      menuItem.className = 'ctx-menu-item';
      menuItem.textContent = item.label;
      menuItem.addEventListener('click', function() {
        hideContextMenu();
        if (item.action) item.action();
      });
      menu.appendChild(menuItem);
    });

    menu.classList.remove('hidden');
    var vw = window.innerWidth;
    var vh = window.innerHeight;
    var mw = menu.offsetWidth || 180;
    var mh = menu.offsetHeight || 200;
    var left = (x + mw > vw) ? x - mw : x;
    var top = (y + mh > vh) ? y - mh : y;
    menu.style.left = Math.max(0, left) + 'px';
    menu.style.top = Math.max(0, top) + 'px';
  };

  SC.fuzzyMatch = function(query, text) {
    if (!query) return { score: 1, matches: [] };
    query = query.toLowerCase();
    text = text.toLowerCase();
    var qi = 0, score = 0, matches = [], lastMatchIdx = -1;
    for (var ti = 0; ti < text.length && qi < query.length; ti++) {
      if (text[ti] === query[qi]) {
        var bonus = 0;
        if (ti === 0 || text[ti-1] === ' ' || text[ti-1] === '/' || text[ti-1] === '_') bonus = 2;
        if (ti === lastMatchIdx + 1) bonus += 1;
        score += 1 + bonus;
        matches.push(ti);
        lastMatchIdx = ti;
        qi++;
      }
    }
    if (qi < query.length) return null;
    return { score: score, matches: matches };
  };

  SC.applyListStagger = function(container, itemSelector) {
    var items = container.querySelectorAll(itemSelector || ':scope > *');
    items.forEach(function(el, i) {
      el.classList.add('list-enter');
      el.style.animationDelay = (i * 30) + 'ms';
    });
  };

  function hideContextMenu() {
    var menu = document.getElementById('msg-context-menu');
    if (menu) menu.classList.add('hidden');
  }

  document.addEventListener('click', function(e) {
    var menu = document.getElementById('msg-context-menu');
    if (menu && !menu.contains(e.target)) hideContextMenu();
  });

  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') hideContextMenu();
  });

  SC.renderToBoth = function(id1, id2, htmlOrFn) {
    var el1 = SC.$('#' + id1);
    var el2 = SC.$('#' + id2);
    if (typeof htmlOrFn === 'function') {
      if (el1) htmlOrFn(el1);
      if (el2) htmlOrFn(el2);
    } else {
      if (el1) el1.innerHTML = htmlOrFn;
      if (el2) el2.innerHTML = htmlOrFn;
    }
  };

  SC.renderMessageCard = function(role, contentHTML, timestamp, options) {
    options = options || {};
    var roleLabels = { user: 'You', assistant: 'SmartClaw', system: 'System' };
    var roleLabel = options.roleLabel || roleLabels[role] || role;
    var roleColor = role === 'user' ? 'var(--accent)' : role === 'assistant' ? 'var(--info)' : 'var(--tx-2)';
    var el = document.createElement('div');
    el.className = 'message ' + role;
    el.innerHTML = '<div class="msg-role" style="color:' + roleColor + '">' + SC.escapeHtml(roleLabel) + '</div>' +
      '<div class="msg-bubble"' + (options.style ? ' style="' + options.style + '"' : '') + '>' + contentHTML + '</div>' +
      (timestamp ? '<div class="msg-ts">' + SC.escapeHtml(timestamp) + '</div>' : '');
    return el;
  };

  SC.makeDraggable = function(container, opts) {
    opts = opts || {};
    container.addEventListener('dragstart', function(e) {
      if (opts.onDragStart) opts.onDragStart(e);
      e.dataTransfer.effectAllowed = opts.effect || 'move';
      if (opts.dragData) e.dataTransfer.setData('text/plain', JSON.stringify(opts.dragData(e)));
      container.classList.add('dragging');
    });
    container.addEventListener('dragend', function() {
      container.classList.remove('dragging');
      if (opts.onDragEnd) opts.onDragEnd();
    });
    if (opts.dropTarget) {
      opts.dropTarget.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = opts.effect || 'move';
        opts.dropTarget.classList.add('drag-over');
        if (opts.onDragOver) opts.onDragOver(e);
      });
      opts.dropTarget.addEventListener('dragleave', function() {
        opts.dropTarget.classList.remove('drag-over');
      });
      opts.dropTarget.addEventListener('drop', function(e) {
        e.preventDefault();
        opts.dropTarget.classList.remove('drag-over');
        if (opts.onDrop) opts.onDrop(e);
      });
    }
  };
})();
