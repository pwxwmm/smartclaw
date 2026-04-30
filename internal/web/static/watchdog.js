(function() {
  'use strict';

  var refreshInterval = null;
  var REFRESH_MS = 15000;

  function initWatchdog() {
    if (SC.$('#panel-body-watchdog')) return;
    SC.createPanel('watchdog', 'Watchdog');
    fetchStatus();
    if (refreshInterval) clearInterval(refreshInterval);
    refreshInterval = setInterval(fetchStatus, REFRESH_MS);
  }

  function fetchStatus() {
    fetch('/api/watchdog/status')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        SC.state.watchdogStatus = data;
        renderPanel(data);
      })
      .catch(function() {});
  }

  function renderPanel(data) {
    var body = SC.$('#panel-body-watchdog');
    if (!body) return;

    var enabled = data && data.enabled;
    var errors = (data && data.recent_errors) || [];
    var watches = (data && data.active_watches) || [];
    var errorCount = (data && data.error_count_today) || 0;

    var html = '';

    html += '<div class="watchdog-status">';
    html += '<div class="watchdog-header-row">';
    html += '<span class="watchdog-indicator ' + (enabled ? 'on' : 'off') + '"></span>';
    html += '<span class="watchdog-label">' + (enabled ? 'Monitoring Active' : 'Monitoring Off') + '</span>';
    html += '<button class="btn-sm ' + (enabled ? 'btn-secondary' : 'btn-primary') + '" id="watchdog-toggle">' + (enabled ? 'Stop' : 'Start') + '</button>';
    html += '</div>';
    html += '<div class="watchdog-stats">';
    html += '<span class="watchdog-stat"><strong>' + errorCount + '</strong> errors today</span>';
    html += '<span class="watchdog-stat"><strong>' + watches.length + '</strong> watches</span>';
    html += '</div>';
    html += '</div>';

    if (watches.length > 0) {
      html += '<div class="watchdog-section">';
      html += '<h4 class="watchdog-section-title">Active Watches</h4>';
      watches.forEach(function(w) {
        html += '<div class="watchdog-watch-item">';
        html += '<span class="watchdog-watch-id">' + SC.escapeHtml(w.id || '') + '</span>';
        html += '<span class="watchdog-watch-time">since ' + SC.escapeHtml((w.started_at || '').substring(11, 19)) + '</span>';
        if (w.last_error) {
          html += '<span class="watchdog-watch-error" title="' + SC.escapeHtml(w.last_error) + '">' + SC.escapeHtml(w.last_error.substring(0, 60)) + '</span>';
        }
        html += '</div>';
      });
      html += '</div>';
    }

    if (errors.length > 0) {
      html += '<div class="watchdog-section">';
      html += '<h4 class="watchdog-section-title">Recent Errors</h4>';
      errors.reverse().forEach(function(err) {
        var sevClass = 'severity-' + (err.severity || 'info');
        var fileLine = '';
        if (err.file) {
          fileLine = err.file;
          if (err.line_num) fileLine += ':' + err.line_num;
        }
        html += '<div class="watchdog-error-row" data-error-msg="' + SC.escapeHtml(err.message || err.line || '') + '">';
        html += '<span class="watchdog-severity-badge ' + sevClass + '">' + SC.escapeHtml(err.severity || 'info') + '</span>';
        if (fileLine) {
          html += '<span class="watchdog-error-file">' + SC.escapeHtml(fileLine) + '</span>';
        }
        html += '<span class="watchdog-error-msg">' + SC.escapeHtml(err.message || err.line || '') + '</span>';
        if (err.debug_suggestion) {
          html += '<button class="btn-sm btn-primary watchdog-debug-btn" data-debug-cmd="' + SC.escapeHtml(err.debug_suggestion.command) + '">🐛 Debug</button>';
        }
        html += '<span class="watchdog-error-source">' + SC.escapeHtml(err.source || '') + '</span>';
        html += '</div>';
      });
      html += '</div>';
    } else if (enabled) {
      html += '<div class="watchdog-empty">No errors detected</div>';
    }

    body.innerHTML = html;

    var toggleBtn = SC.$('#watchdog-toggle');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', function() {
        if (enabled) {
          SC.wsSend('cmd', { content: '/tools watchdog_stop' });
        } else {
          SC.wsSend('cmd', { content: '/tools watchdog_start' });
        }
        setTimeout(fetchStatus, 500);
      });
    }

    body.querySelectorAll('.watchdog-error-row').forEach(function(row) {
      row.addEventListener('click', function() {
        var msg = this.dataset.errorMsg;
        if (msg && SC.$('#input')) {
          SC.$('#input').value = 'Debug this error: ' + msg;
          SC.$('#input').focus();
        }
      });
    });

    body.querySelectorAll('.watchdog-debug-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var cmd = this.dataset.debugCmd;
        if (cmd && SC.$('#input')) {
          SC.$('#input').value = cmd;
          SC.$('#input').focus();
        }
      });
    });
  }

  SC.initWatchdog = initWatchdog;
  SC.fetchWatchdogStatus = fetchStatus;
})();
