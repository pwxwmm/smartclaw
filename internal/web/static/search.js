// SmartClaw - Chat Search
(function() {
  'use strict';

  var RECENT_KEY = 'smartclaw-recent-searches';
  var MAX_RECENT = 8;
  var searchTimer = null;
  var currentTab = 'messages';
  var isOpen = false;

  function getRecentSearches() {
    try {
      return JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
    } catch (e) { return []; }
  }

  function addRecentSearch(query) {
    if (!query) return;
    var recent = getRecentSearches();
    recent = recent.filter(function(r) { return r !== query; });
    recent.unshift(query);
    if (recent.length > MAX_RECENT) recent = recent.slice(0, MAX_RECENT);
    try { localStorage.setItem(RECENT_KEY, JSON.stringify(recent)); } catch (e) {}
  }

  function relativeTime(ts) {
    if (!ts) return '';
    var diff = Date.now() - ts;
    var sec = Math.floor(diff / 1000);
    if (sec < 60) return 'just now';
    var min = Math.floor(sec / 60);
    if (min < 60) return min + 'm ago';
    var hr = Math.floor(min / 60);
    if (hr < 24) return hr + 'h ago';
    var days = Math.floor(hr / 24);
    return days + 'd ago';
  }

  function excerpt(text, query, maxLen) {
    maxLen = maxLen || 200;
    if (!text) return '';
    var lower = text.toLowerCase();
    var qLower = (query || '').toLowerCase();
    var start = 0;
    if (qLower && lower.indexOf(qLower) !== -1) {
      start = Math.max(0, lower.indexOf(qLower) - 60);
    }
    var ex = text.slice(start, start + maxLen);
    if (start > 0) ex = '...' + ex;
    if (start + maxLen < text.length) ex = ex + '...';
    if (qLower) {
      var re = new RegExp('(' + qLower.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
      ex = SC.escapeHtml(ex).replace(re, '<mark>$1</mark>');
    } else {
      ex = SC.escapeHtml(ex);
    }
    return ex;
  }

  function open() {
    var panel = SC.$('#search-panel');
    if (!panel) return;
    panel.classList.remove('hidden');
    requestAnimationFrame(function() {
      panel.classList.add('open');
    });
    isOpen = true;
    var input = SC.$('#search-input');
    if (input) {
      input.value = '';
      input.focus();
    }
    renderRecent();
  }

  function close() {
    var panel = SC.$('#search-panel');
    if (!panel) return;
    panel.classList.remove('open');
    setTimeout(function() { panel.classList.add('hidden'); }, 200);
    isOpen = false;
  }

  function search(query) {
    if (!query || !query.trim()) {
      renderResults([]);
      return;
    }
    addRecentSearch(query.trim());
    var sinceEl = SC.$('#search-since');
    var untilEl = SC.$('#search-until');
    var roleEl = SC.$('#search-role');
    var data = { query: query.trim(), type: currentTab };
    if (sinceEl && sinceEl.value) data.since = sinceEl.value;
    if (untilEl && untilEl.value) data.until = untilEl.value;
    if (roleEl && roleEl.value) data.role = roleEl.value;
    SC.wsSend('chat_search', data);
  }

  function renderResults(results) {
    var container = SC.$('#search-results');
    if (!container) return;
    container.innerHTML = '';

    var query = (SC.$('#search-input') || {}).value || '';

    if (!results || results.length === 0) {
      container.innerHTML = '<div class="search-empty">' +
        '<svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>' +
        '<span>No results found</span>' +
      '</div>';
      return;
    }

    for (var i = 0; i < results.length; i++) {
      var r = results[i];
      var item = document.createElement('div');
      item.className = 'search-result-item';
      item.dataset.sessionId = r.session_id || r.sessionId || '';
      item.innerHTML =
        '<div class="search-result-excerpt">' + excerpt(r.content || r.text || '', query) + '</div>' +
        '<div class="search-result-meta">' +
          '<span class="search-result-session">' + SC.escapeHtml(r.session_title || r.title || 'Untitled') + '</span>' +
          '<span class="search-result-time">' + SC.escapeHtml(relativeTime(r.ts || r.timestamp)) + '</span>' +
        '</div>';
      item.addEventListener('click', function() {
        var sid = this.dataset.sessionId;
        if (sid) {
          SC.wsSend('session_load', { id: sid });
        }
        close();
      });
      container.appendChild(item);
    }
  }

  function renderRecent() {
    var container = SC.$('#search-recent');
    if (!container) return;
    var recent = getRecentSearches();
    if (recent.length === 0) {
      container.innerHTML = '';
      return;
    }
    var html = '<span class="search-recent-label">Recent</span>';
    for (var i = 0; i < recent.length; i++) {
      html += '<button class="search-recent-chip" data-query="' + SC.escapeHtml(recent[i]) + '">' + SC.escapeHtml(recent[i]) + '</button>';
    }
    container.innerHTML = html;

    var chips = container.querySelectorAll('.search-recent-chip');
    for (var j = 0; j < chips.length; j++) {
      chips[j].addEventListener('click', function() {
        var q = this.dataset.query;
        var input = SC.$('#search-input');
        if (input) input.value = q;
        search(q);
      });
    }
  }

  function init() {
    var panel = SC.$('#search-panel');
    if (!panel) return;

    var input = SC.$('#search-input');
    if (input) {
      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          search(this.value);
        }
        if (e.key === 'Escape') {
          e.preventDefault();
          close();
        }
      });
      input.addEventListener('input', function() {
        if (searchTimer) clearTimeout(searchTimer);
        var q = this.value.trim();
        if (!q) {
          renderResults([]);
          renderRecent();
          return;
        }
        searchTimer = setTimeout(function() { search(q); }, 300);
      });
    }

    var tabs = SC.$$('.search-tab');
    for (var i = 0; i < tabs.length; i++) {
      tabs[i].addEventListener('click', function() {
        SC.$$('.search-tab').forEach(function(t) { t.classList.remove('active'); });
        this.classList.add('active');
        currentTab = this.dataset.tab;
        var input2 = SC.$('#search-input');
        if (input2 && input2.value.trim()) search(input2.value.trim());
      });
    }

    var filterEls = ['#search-since', '#search-until', '#search-role'];
    for (var fi = 0; fi < filterEls.length; fi++) {
      var filterEl = SC.$(filterEls[fi]);
      if (filterEl) {
        filterEl.addEventListener('change', function() {
          var input2 = SC.$('#search-input');
          if (input2 && input2.value.trim()) search(input2.value.trim());
        });
      }
    }

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && isOpen) {
        e.preventDefault();
        close();
      }
    });
  }

  SC.chatSearch = {
    open: open,
    close: close,
    search: search,
    renderResults: renderResults
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
