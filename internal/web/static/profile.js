// SmartClaw - Profile / Personalization
(function() {
  'use strict';

  var cache = { profile: null, observations: null, ts: 0 };
  var CACHE_TTL = 30000;
  var obsPage = 0;
  var obsPerPage = 20;
  var obsFilter = '';

  function init() {
    loadProfile();
  }

  function isCacheValid() {
    return cache.ts > 0 && Date.now() - cache.ts < CACHE_TTL;
  }

  function loadProfile(force) {
    if (!force && isCacheValid() && cache.profile) {
      renderProfile(cache.profile);
      return;
    }
    fetch('/api/profile', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        cache.profile = data;
        cache.ts = Date.now();
        renderProfile(data);
      })
      .catch(function(err) {
        console.error('Profile load error:', err);
        var el = SC.$('#profile-content');
        if (el) el.innerHTML = '<div class="empty-state"><span class="empty-desc">Failed to load profile</span></div>';
      });
  }

  function loadObservations(force) {
    if (!force && isCacheValid() && cache.observations) {
      renderObservations(cache.observations);
      return;
    }
    fetch('/api/profile/observations', { credentials: 'same-origin' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        cache.observations = data;
        cache.ts = Date.now();
        renderObservations(data);
      })
      .catch(function(err) { console.error('Observations load error:', err); });
  }

  function renderProfile(data) {
    var el = SC.$('#profile-content');
    if (!el) return;

    var prefs = data.preferences || {};
    var prefCount = Object.keys(prefs).length;
    var patternCount = (data.top_patterns || []).length;
    var obsCount = (cache.observations || []).length;

    var h = '';
    h += '<div class="profile-header">';
    h += '<div class="profile-avatar">S</div>';
    h += '<div class="profile-header-info">';
    h += '<div class="profile-username">SmartClaw User</div>';
    h += '<div class="profile-member-since">Default User</div>';
    h += '</div>';
    h += '</div>';

    h += '<div class="profile-stats">';
    h += '<span class="profile-stat-val">' + prefCount + '</span> preferences';
    h += ' <span class="profile-stat-dot">&middot;</span> ';
    h += '<span class="profile-stat-val">' + patternCount + '</span> patterns';
    h += ' <span class="profile-stat-dot">&middot;</span> ';
    h += '<span class="profile-stat-val">' + obsCount + '</span> observations';
    h += '</div>';

    h += renderStyleSection(data.communication_style);
    h += renderPreferences(prefs);
    h += renderKnowledge(data.knowledge_background);
    h += renderPatterns(data.top_patterns);
    h += renderConflicts(data.conflicts);

    el.innerHTML = h;
    bindProfileEvents();
  }

  var STYLES = [
    { id: 'concise', icon: '\uD83C\uDFAF', label: 'Concise', desc: 'Short, to-the-point responses', color: '#3b82f6' },
    { id: 'verbose', icon: '\uD83D\uDCDA', label: 'Verbose', desc: 'Detailed explanations with context', color: '#8b5cf6' },
    { id: 'technical', icon: '\u2699\uFE0F', label: 'Technical', desc: 'Technical jargon OK, skip basics', color: '#06b6d4' },
    { id: 'plain', icon: '\uD83D\uDCAC', label: 'Plain', desc: 'Simple language, explain everything', color: '#22c55e' },
    { id: 'step-by-step', icon: '\uD83D\uDCCB', label: 'Step-by-step', desc: 'Numbered steps, one at a time', color: '#f59e0b' },
    { id: 'direct', icon: '\u26A1', label: 'Direct', desc: 'Just the answer, no preamble', color: '#ef4444' }
  ];

  function renderStyleSection(currentStyle) {
    var h = '<div class="profile-section">';
    h += '<div class="profile-section-title">Communication Style</div>';
    h += '<div class="style-current">Current: <strong>' + (currentStyle || 'not set') + '</strong></div>';
    h += '<div class="style-grid">';
    for (var i = 0; i < STYLES.length; i++) {
      var s = STYLES[i];
      var isActive = currentStyle === s.id;
      h += '<div class="style-card' + (isActive ? ' active' : '') + '" data-style="' + s.id + '" style="--style-color:' + s.color + '">';
      h += '<div class="style-card-icon">' + s.icon + '</div>';
      h += '<div class="style-card-label">' + s.label + '</div>';
      h += '<div class="style-card-desc">' + s.desc + '</div>';
      h += '</div>';
    }
    h += '</div>';
    h += '</div>';
    return h;
  }

  function renderPreferences(prefs) {
    var keys = Object.keys(prefs);
    if (keys.length === 0) return '';
    var grouped = {};
    for (var i = 0; i < keys.length; i++) {
      var k = keys[i];
      var category = guessPrefCategory(k);
      if (!grouped[category]) grouped[category] = [];
      grouped[category].push({ key: k, value: prefs[k] });
    }
    var h = '<div class="profile-section">';
    h += '<div class="profile-section-title">Preferences</div>';
    var catNames = Object.keys(grouped);
    for (var c = 0; c < catNames.length; c++) {
      var cat = catNames[c];
      h += '<div class="pref-category">' + cat + '</div>';
      var items = grouped[cat];
      for (var j = 0; j < items.length; j++) {
        var item = items[j];
        var obsId = findObsId(item.key, item.value);
        h += '<div class="preference-item">';
        h += '<span class="pref-key">' + SC.escapeHtml(item.key) + '</span>';
        h += '<span class="pref-arrow">&rarr;</span>';
        h += '<span class="pref-value">' + SC.escapeHtml(item.value) + '</span>';
        if (obsId) {
          h += '<button class="pref-unlearn-btn" data-obs-id="' + obsId + '" title="Unlearn">&times;</button>';
        }
        h += '</div>';
      }
    }
    h += '</div>';
    return h;
  }

  function guessPrefCategory(key) {
    var k = key.toLowerCase();
    if (k.indexOf('indent') >= 0 || k.indexOf('naming') >= 0 || k.indexOf('bracket') >= 0) return 'Code Style';
    if (k.indexOf('lang') >= 0 || k.indexOf('framework') >= 0) return 'Language';
    if (k.indexOf('tool') >= 0 || k.indexOf('editor') >= 0) return 'Tools';
    return 'General';
  }

  function findObsId(key, value) {
    var obs = cache.observations || [];
    for (var i = 0; i < obs.length; i++) {
      if (obs[i].key === key && obs[i].value === value) return obs[i].id;
    }
    return null;
  }

  function renderKnowledge(kb) {
    if (!kb || kb.length === 0) return '';
    var h = '<div class="profile-section">';
    h += '<div class="profile-section-title">Knowledge Background</div>';
    h += '<div class="knowledge-pills">';
    for (var i = 0; i < kb.length; i++) {
      var obsId = findKnowledgeObsId(kb[i]);
      h += '<span class="knowledge-pill">';
      h += SC.escapeHtml(kb[i]);
      if (obsId) h += '<button class="pill-remove" data-obs-id="' + obsId + '" title="Remove">&times;</button>';
      h += '</span>';
    }
    h += '</div>';
    h += '</div>';
    return h;
  }

  function findKnowledgeObsId(value) {
    var obs = cache.observations || [];
    for (var i = 0; i < obs.length; i++) {
      if (obs[i].category === 'knowledge' && obs[i].value === value) return obs[i].id;
    }
    return null;
  }

  function renderPatterns(patterns) {
    if (!patterns || patterns.length === 0) return '';
    var maxFreq = 1;
    for (var i = 0; i < patterns.length; i++) {
      if (patterns[i].frequency > maxFreq) maxFreq = patterns[i].frequency;
    }
    var h = '<div class="profile-section">';
    h += '<div class="profile-section-title">Work Patterns</div>';
    for (var i = 0; i < patterns.length; i++) {
      var p = patterns[i];
      var pct = Math.round((p.frequency / maxFreq) * 100);
      var obsId = findPatternObsId(p.pattern);
      h += '<div class="pattern-item">';
      h += '<div class="pattern-info">';
      h += '<span class="pattern-desc">' + SC.escapeHtml(p.pattern) + '</span>';
      h += '<span class="pattern-meta">' + p.frequency + 'x &middot; ' + relativeTime(p.last_seen) + '</span>';
      h += '</div>';
      h += '<div class="pattern-bar"><div class="pattern-bar-fill" style="width:' + pct + '%"></div></div>';
      if (obsId) {
        h += '<button class="pref-unlearn-btn" data-obs-id="' + obsId + '" title="Unlearn">&times;</button>';
      }
      h += '</div>';
    }
    h += '</div>';
    return h;
  }

  function findPatternObsId(pattern) {
    var obs = cache.observations || [];
    for (var i = 0; i < obs.length; i++) {
      if ((obs[i].category === 'pattern' || obs[i].category === 'workflow_pattern') && obs[i].value === pattern) return obs[i].id;
    }
    return null;
  }

  function renderConflicts(conflicts) {
    if (!conflicts || conflicts.length === 0) return '';
    var h = '<div class="profile-section">';
    h += '<div class="profile-section-title">Unresolved Conflicts</div>';
    for (var i = 0; i < conflicts.length; i++) {
      var c = conflicts[i];
      if (c.resolved) continue;
      h += '<div class="conflict-item">';
      h += '<div class="conflict-key">' + SC.escapeHtml(c.category) + ' / ' + SC.escapeHtml(c.key) + '</div>';
      h += '<div class="conflict-pair">';
      h += '<span class="conflict-thesis">' + SC.escapeHtml(c.thesis) + ' <small>(' + Math.round(c.thesis_confidence * 100) + '%)</small></span>';
      h += '<span class="conflict-vs">vs</span>';
      h += '<span class="conflict-antithesis">' + SC.escapeHtml(c.antithesis) + ' <small>(' + Math.round(c.antithesis_confidence * 100) + '%)</small></span>';
      h += '</div>';
      h += '</div>';
    }
    h += '</div>';
    return h;
  }

  function renderObservations(observations) {
    var el = SC.$('#profile-observations-timeline');
    if (!el) return;

    var filtered = observations;
    if (obsFilter) {
      filtered = observations.filter(function(o) { return o.category === obsFilter; });
    }

    var categories = {};
    for (var i = 0; i < observations.length; i++) {
      categories[observations[i].category] = true;
    }

    var h = '';
    h += '<div class="obs-filter-row">';
    h += '<select id="obs-category-filter" class="obs-filter-select">';
    h += '<option value="">All categories</option>';
    var catKeys = Object.keys(categories).sort();
    for (var c = 0; c < catKeys.length; c++) {
      var sel = obsFilter === catKeys[c] ? ' selected' : '';
      h += '<option value="' + catKeys[c] + '"' + sel + '>' + catKeys[c] + '</option>';
    }
    h += '</select>';
    h += '<span class="obs-count">' + filtered.length + ' observations</span>';
    h += '</div>';

    var start = obsPage * obsPerPage;
    var page = filtered.slice(start, start + obsPerPage);

    for (var i = 0; i < page.length; i++) {
      var o = page[i];
      var confPct = Math.round(o.confidence * 100);
      var confColor = confPct >= 80 ? 'var(--ok)' : confPct >= 50 ? 'var(--warn)' : 'var(--tx-2)';
      h += '<div class="observation-entry">';
      h += '<span class="observation-category-badge" style="background:' + categoryColor(o.category) + '">' + SC.escapeHtml(o.category) + '</span>';
      h += '<span class="obs-key-val"><strong>' + SC.escapeHtml(o.key) + '</strong> &rarr; ' + SC.escapeHtml(o.value) + '</span>';
      h += '<div class="confidence-bar"><div class="confidence-bar-fill" style="width:' + confPct + '%;background:' + confColor + '"></div></div>';
      h += '<span class="obs-time">' + relativeTime(o.observed_at) + '</span>';
      h += '<button class="obs-unlearn-btn" data-obs-id="' + o.id + '" title="Unlearn">&times;</button>';
      h += '</div>';
    }

    if (filtered.length > start + obsPerPage) {
      h += '<button class="obs-load-more" id="obs-load-more-btn">Load More</button>';
    }

    el.innerHTML = h;
    bindObsEvents();
  }

  function categoryColor(cat) {
    var colors = {
      'code_style': '#3b82f6',
      'communication_style': '#8b5cf6',
      'knowledge': '#06b6d4',
      'preference': '#22c55e',
      'pattern': '#f59e0b',
      'workflow_pattern': '#f59e0b'
    };
    return colors[cat] || 'var(--accent)';
  }

  function bindProfileEvents() {
    SC.$$('.style-card').forEach(function(card) {
      card.addEventListener('click', function() {
        var style = card.dataset.style;
        if (!style) return;
        setCommunicationStyle(style);
      });
    });

    SC.$$('.pref-unlearn-btn, .pill-remove').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var obsId = btn.dataset.obsId;
        if (obsId && confirm('Remove this observation?')) {
          deleteObservation(obsId);
        }
      });
    });
  }

  function bindObsEvents() {
    var filterEl = SC.$('#obs-category-filter');
    if (filterEl) {
      filterEl.addEventListener('change', function() {
        obsFilter = filterEl.value;
        obsPage = 0;
        renderObservations(cache.observations || []);
      });
    }

    var moreBtn = SC.$('#obs-load-more-btn');
    if (moreBtn) {
      moreBtn.addEventListener('click', function() {
        obsPage++;
        renderObservations(cache.observations || []);
      });
    }

    SC.$$('.obs-unlearn-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var obsId = btn.dataset.obsId;
        if (obsId && confirm('Remove this observation?')) {
          deleteObservation(obsId);
        }
      });
    });
  }

  function setCommunicationStyle(style) {
    fetch('/api/profile/style', {
      method: 'PUT',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ communication_style: style })
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.success) {
        showToast('Style updated to ' + style);
        loadProfile(true);
        loadObservations(true);
      } else {
        showToast('Error: ' + (data.error || 'unknown'));
      }
    })
    .catch(function(err) { showToast('Failed: ' + err.message); });
  }

  function deleteObservation(id) {
    fetch('/api/profile/observations/' + id, {
      method: 'DELETE',
      credentials: 'same-origin'
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.success) {
        showToast('Observation removed');
        cache.ts = 0;
        loadProfile(true);
        loadObservations(true);
      } else {
        showToast('Error: ' + (data.error || 'unknown'));
      }
    })
    .catch(function(err) { showToast('Failed: ' + err.message); });
  }

  function deleteAllObservations() {
    fetch('/api/profile/observations/delete-all', {
      method: 'DELETE',
      credentials: 'same-origin'
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.success) {
        showToast('All observations deleted');
        cache.ts = 0;
        loadProfile(true);
        loadObservations(true);
      } else {
        showToast('Error: ' + (data.error || 'unknown'));
      }
    })
    .catch(function(err) { showToast('Failed: ' + err.message); });
  }

  function exportData() {
    var data = cache.observations || [];
    var json = JSON.stringify(data, null, 2);
    var modal = SC.$('#export-data-modal');
    if (!modal) {
      modal = document.createElement('div');
      modal.id = 'export-data-modal';
      modal.className = 'modal';
      modal.innerHTML = '<div class="modal-backdrop"></div>' +
        '<div class="modal-content" style="max-width:640px">' +
        '<div class="modal-head"><span class="modal-title">Export Data</span>' +
        '<button class="icon-btn export-close-btn" aria-label="Close"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div>' +
        '<div class="modal-body"><pre id="export-json" style="max-height:400px;overflow:auto;font-size:12px;white-space:pre-wrap;word-break:break-all;padding:12px;background:var(--bg-2);border-radius:var(--radius-md)"></pre></div></div>';
      document.body.appendChild(modal);
    }
    SC.$('#export-json').textContent = json;
    modal.classList.remove('hidden');
    SC.$('.export-close-btn', modal).onclick = function() { modal.classList.add('hidden'); };
    SC.$('.modal-backdrop', modal).onclick = function() { modal.classList.add('hidden'); };
  }

  function showToast(msg) {
    var container = SC.$('#toast-container');
    if (!container) return;
    var toast = document.createElement('div');
    toast.className = 'toast';
    toast.textContent = msg;
    container.appendChild(toast);
    setTimeout(function() { toast.classList.add('show'); }, 10);
    setTimeout(function() {
      toast.classList.remove('show');
      setTimeout(function() { toast.remove(); }, 300);
    }, 2500);
  }

  function relativeTime(ts) {
    if (!ts) return '';
    var d = new Date(ts);
    var now = Date.now();
    var diff = now - d.getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
    if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
    if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
    return d.toLocaleDateString();
  }

  function renderPrivacySection() {
    var el = SC.$('#profile-privacy');
    if (!el) return;
    var obsCount = (cache.observations || []).length;
    var h = '<div class="privacy-section">';
    h += '<div class="profile-section-title">Privacy & Data</div>';
    h += '<p class="privacy-desc">SmartClaw has learned <strong>' + obsCount + '</strong> observations about your workflow. All data stays local.</p>';
    h += '<div class="privacy-actions">';
    h += '<button class="btn-ghost" id="btn-export-data">Export My Data</button>';
    h += '<button class="btn-ghost" style="color:var(--err)" id="btn-delete-all-obs">Delete All Observations</button>';
    h += '</div>';
    h += '</div>';
    el.innerHTML = h;

    SC.$('#btn-export-data').onclick = exportData;
    SC.$('#btn-delete-all-obs').onclick = function() {
      if (confirm('Delete ALL observations? This cannot be undone.')) {
        deleteAllObservations();
      }
    };
  }

  window.SCProfile = {
    init: init,
    loadProfile: loadProfile,
    loadObservations: loadObservations,
    renderPrivacySection: renderPrivacySection
  };
})();
