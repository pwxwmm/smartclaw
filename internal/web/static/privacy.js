// SmartClaw - Privacy Audit Trail
(function() {
  'use strict';

  var refreshTimer = null;

  var providerColors = {
    anthropic: '#a78bfa',
    openai: '#3ec96e',
    google: '#5290e0'
  };

  function init() {
    loadPrivacyAudit();
    startAutoRefresh();
  }

  function startAutoRefresh() {
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = setInterval(loadPrivacyAudit, 30000);
  }

  function loadPrivacyAudit() {
    fetch('/api/privacy/audit?limit=200', { credentials: 'same-origin' })
      .then(function(r) {
        if (r.status === 401) { SC.state.authenticated = false; return null; }
        return r.json();
      })
      .then(function(data) {
        if (!data) return;
        renderPrivacySummary(data.entries || []);
        renderPrivacyTable(data.entries || []);
      })
      .catch(function(err) {
        console.error('[Privacy Audit] load error:', err);
      });
  }

  function renderPrivacySummary(entries) {
    var today = new Date().toISOString().slice(0, 10);
    var todayEntries = entries.filter(function(e) {
      return e.timestamp && e.timestamp.slice(0, 10) === today;
    });

    var totalInput = 0;
    var totalOutput = 0;
    var providers = {};

    todayEntries.forEach(function(e) {
      totalInput += e.input_tokens || 0;
      totalOutput += e.output_tokens || 0;
      if (e.provider) providers[e.provider] = true;
    });

    var summary = {
      todayCount: todayEntries.length,
      totalInput: totalInput,
      totalOutput: totalOutput,
      activeProviders: Object.keys(providers).length
    };

    var html =
      '<div class="privacy-summary-card">' +
        '<div class="privacy-summary-value">' + summary.todayCount + '</div>' +
        '<div class="privacy-summary-label">Requests Today</div>' +
      '</div>' +
      '<div class="privacy-summary-card">' +
        '<div class="privacy-summary-value">' + summary.totalInput.toLocaleString() + '</div>' +
        '<div class="privacy-summary-label">Input Tokens</div>' +
      '</div>' +
      '<div class="privacy-summary-card">' +
        '<div class="privacy-summary-value">' + summary.totalOutput.toLocaleString() + '</div>' +
        '<div class="privacy-summary-label">Output Tokens</div>' +
      '</div>' +
      '<div class="privacy-summary-card">' +
        '<div class="privacy-summary-value">' + summary.activeProviders + '</div>' +
        '<div class="privacy-summary-label">Active Providers</div>' +
      '</div>';

    SC.renderToBoth('privacy-summary', 'privacy-summary-view', html);
  }

  function renderPrivacyTable(entries) {
    if (!entries || entries.length === 0) {
      SC.renderToBoth('privacy-table-body', 'privacy-table-body-view', '<tr><td colspan="8" class="privacy-empty">No outbound API calls recorded yet.</td></tr>');
      return;
    }

    var reversed = entries.slice().reverse();
    SC.renderToBoth('privacy-table-body', 'privacy-table-body-view', function(el) {
      el.innerHTML = '';
      reversed.forEach(function(entry) {
        var row = document.createElement('tr');
        row.className = 'privacy-row';

        var time = entry.timestamp ? new Date(entry.timestamp).toLocaleTimeString() : '';
        var providerColor = providerColors[entry.provider] || 'var(--tx-2)';
        var categories = (entry.data_categories || []).map(function(cat) {
          return '<span class="privacy-category-badge">' + SC.escapeHtml(cat) + '</span>';
        }).join('');

        row.innerHTML =
          '<td class="privacy-cell privacy-time">' + SC.escapeHtml(time) + '</td>' +
          '<td class="privacy-cell"><span class="privacy-provider-badge" style="background:' + providerColor + '20;color:' + providerColor + ';border:1px solid ' + providerColor + '40">' + SC.escapeHtml(entry.provider || '') + '</span></td>' +
          '<td class="privacy-cell">' + SC.escapeHtml(entry.model || '') + '</td>' +
          '<td class="privacy-cell privacy-num">' + (entry.message_count || 0) + '</td>' +
          '<td class="privacy-cell privacy-num">' + (entry.input_tokens || 0).toLocaleString() + '</td>' +
          '<td class="privacy-cell privacy-num">' + (entry.output_tokens || 0).toLocaleString() + '</td>' +
          '<td class="privacy-cell privacy-categories">' + categories + '</td>' +
          '<td class="privacy-cell privacy-num">' + (entry.duration_ms || 0) + 'ms</td>';

        el.appendChild(row);
      });
    });
  }

  SC.initPrivacy = init;
  SC.loadPrivacyAudit = loadPrivacyAudit;
})();
