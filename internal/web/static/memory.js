// SmartClaw - Memory
(function() {
  'use strict';

  function renderMemoryLayers() {
    const memEl = SC.$('#memory-content');
    const usrEl = SC.$('#user-content');
    const memViewEl = SC.$('#memory-content-view');
    const usrViewEl = SC.$('#user-content-view');
    if (!memEl && !memViewEl && !usrEl && !usrViewEl) return;

    if (memEl && memEl.tagName === 'PRE') {
      memEl.textContent = SC.state.memoryLayers.memory || '(empty)';
      const memBtns = memEl.parentElement.querySelector('.memory-edit-btns');
      if (memBtns) memBtns.remove();
      const memEditBtn = memEl.parentElement.querySelector('.memory-edit-btn');
      if (!memEditBtn) {
        const btn = document.createElement('button');
        btn.className = 'memory-edit-btn';
        btn.textContent = 'Edit';
        btn.onclick = () => editMemoryFile('memory');
        memEl.parentElement.insertBefore(btn, memEl);
      }
    }
    if (usrEl && usrEl.tagName === 'PRE') {
      usrEl.textContent = SC.state.memoryLayers.user || '(empty)';
      const usrBtns = usrEl.parentElement.querySelector('.memory-edit-btns');
      if (usrBtns) usrBtns.remove();
      const usrEditBtn = usrEl.parentElement.querySelector('.memory-edit-btn');
      if (!usrEditBtn) {
        const btn = document.createElement('button');
        btn.className = 'memory-edit-btn';
        btn.textContent = 'Edit';
        btn.onclick = () => editMemoryFile('user');
        usrEl.parentElement.insertBefore(btn, usrEl);
      }
    }

    if (memViewEl && memViewEl.tagName === 'PRE') memViewEl.textContent = SC.state.memoryLayers.memory || '(empty)';
    if (usrViewEl && usrViewEl.tagName === 'PRE') usrViewEl.textContent = SC.state.memoryLayers.user || '(empty)';
  }

  function editMemoryFile(fileType) {
    const id = fileType === 'memory' ? 'memory-content' : 'user-content';
    const el = document.getElementById(id);
    if (!el || el.tagName === 'TEXTAREA') return;
    const raw = fileType === 'memory' ? SC.state.memoryLayers.memory : SC.state.memoryLayers.user;
    const parent = el.parentElement;
    const editBtn = parent.querySelector('.memory-edit-btn');
    if (editBtn) editBtn.style.display = 'none';
    const textarea = document.createElement('textarea');
    textarea.id = id;
    textarea.className = 'memory-content memory-textarea';
    textarea.value = raw || '';
    el.replaceWith(textarea);
    const btns = document.createElement('div');
    btns.className = 'memory-edit-btns';
    const saveBtn = document.createElement('button');
    saveBtn.textContent = 'Save';
    saveBtn.className = 'memory-save-btn';
    saveBtn.onclick = () => saveMemoryFile(fileType);
    const cancelBtn = document.createElement('button');
    cancelBtn.textContent = 'Cancel';
    cancelBtn.className = 'memory-cancel-btn';
    cancelBtn.onclick = () => cancelMemoryEdit(fileType);
    btns.appendChild(saveBtn);
    btns.appendChild(cancelBtn);
    parent.appendChild(btns);
    textarea.focus();
  }

  function saveMemoryFile(fileType) {
    const id = fileType === 'memory' ? 'memory-content' : 'user-content';
    const textarea = document.getElementById(id);
    if (!textarea || textarea.tagName !== 'TEXTAREA') return;
    SC.wsSend('memory_update', { file: fileType, content: textarea.value });
    cancelMemoryEdit(fileType);
  }

  function cancelMemoryEdit(fileType) {
    const id = fileType === 'memory' ? 'memory-content' : 'user-content';
    const textarea = document.getElementById(id);
    if (!textarea || textarea.tagName !== 'TEXTAREA') return;
    const raw = fileType === 'memory' ? SC.state.memoryLayers.memory : SC.state.memoryLayers.user;
    const parent = textarea.parentElement;
    const btns = parent.querySelector('.memory-edit-btns');
    if (btns) btns.remove();
    const editBtn = parent.querySelector('.memory-edit-btn');
    if (editBtn) editBtn.style.display = '';
    const pre = document.createElement('pre');
    pre.id = id;
    pre.className = 'memory-content';
    pre.textContent = raw || '(empty)';
    textarea.replaceWith(pre);
  }

  function renderMemoryStats() {
    const s = SC.state.memoryStats || {};
    var text = `${s.memory_chars || 0} chars / MEMORY.md · ${s.user_chars || 0} chars / USER.md`;
    SC.renderToBoth('memory-stats', 'memory-stats-view', function(el) { el.textContent = text; });
  }

  function renderMemorySearchResults(results) {
    const emptyHtml = '<div class="loading-placeholder" style="color:var(--tx-2)">No results</div>';
    if (!results || results.length === 0) {
      SC.renderToBoth('memory-search-results', 'memory-search-results-view', emptyHtml);
      return;
    }
    SC.renderToBoth('memory-search-results', 'memory-search-results-view', function(el) {
      el.innerHTML = '';
      results.forEach(frag => { el.appendChild(createMemoryFragEl(frag)); });
    });
  }

  function createMemoryFragEl(frag) {
    const el = document.createElement('div');
    el.className = 'memory-frag';
    const title = frag.source || frag.key || frag.title || 'Memory';
    const text = frag.content || frag.text || frag.snippet || '';
    el.innerHTML = `
      <div class="memory-frag-title">${SC.escapeHtml(title)}</div>
      <div class="memory-frag-text">${SC.escapeHtml(text.slice(0, 300))}</div>
    `;
    return el;
  }

  function renderMemoryRecall(data) {
    if (!data) return;
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const context = data.context || 'No context found';
    var el = SC.renderMessageCard('cmd-result', SC.escapeHtml(context), ts, {
      roleLabel: 'Memory Recall',
      style: 'font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px'
    });
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  function renderSessionFragments() {
    const emptyHtml = '<div class="loading-placeholder" style="color:var(--tx-2)">Search session history above</div>';
    if (!SC.state.sessionFragments || SC.state.sessionFragments.length === 0) {
      SC.renderToBoth('session-fragments', 'session-fragments-view', emptyHtml);
      return;
    }
    SC.renderToBoth('session-fragments', 'session-fragments-view', function(el) {
      el.innerHTML = '';
      SC.state.sessionFragments.forEach(frag => { el.appendChild(createSessionFragEl(frag)); });
    });
  }

  function createSessionFragEl(frag) {
    const el = document.createElement('div');
    el.className = 'memory-frag';
    const role = frag.role || 'unknown';
    const timestamp = frag.timestamp ? new Date(frag.timestamp).toLocaleString() : '';
    const relevance = frag.relevance ? (frag.relevance * 100).toFixed(0) + '%' : '';
    const content = frag.content || '';
    el.innerHTML = `
      <div class="memory-frag-title">
        <span class="session-frag-role">${SC.escapeHtml(role)}</span>
        ${timestamp ? `<span class="session-frag-time">${SC.escapeHtml(timestamp)}</span>` : ''}
        ${relevance ? `<span class="session-frag-rel">${SC.escapeHtml(relevance)}</span>` : ''}
      </div>
      <div class="memory-frag-text">${SC.escapeHtml(content.slice(0, 300))}</div>
      <div class="session-frag-sid">session: ${SC.escapeHtml(frag.sessionId?.slice(0, 8) || '')}</div>
    `;
    return el;
  }

  function renderUserObservations() {
    const emptyHtml = '<div class="loading-placeholder" style="color:var(--tx-2)">No observations yet</div>';
    if (!SC.state.userObservations || SC.state.userObservations.length === 0) {
      SC.renderToBoth('user-observations', 'user-observations-view', emptyHtml);
      return;
    }
    SC.renderToBoth('user-observations', 'user-observations-view', function(el) {
      el.innerHTML = '';
      SC.state.userObservations.forEach(obs => { el.appendChild(createObservationItem(obs)); });
    });
  }

  function createObservationItem(obs) {
    const el = document.createElement('div');
    el.className = 'observation-item';
    const category = obs.category || '';
    const key = obs.key || '';
    const value = obs.value || '';
    const observedAt = obs.observedAt ? new Date(obs.observedAt).toLocaleString() : '';
    const confidence = obs.confidence ? (obs.confidence * 100).toFixed(0) + '%' : '';
    el.innerHTML = `
      <div class="observation-head">
        <span class="observation-category">${SC.escapeHtml(category)}</span>
        <span class="observation-key">${SC.escapeHtml(key)}</span>
        ${confidence ? `<span class="observation-confidence">${SC.escapeHtml(confidence)}</span>` : ''}
      </div>
      <div class="observation-value">${SC.escapeHtml(value.slice(0, 200))}</div>
      ${observedAt ? `<div class="observation-time">${SC.escapeHtml(observedAt)}</div>` : ''}
      <button class="observation-delete memory-edit-btn" data-id="${obs.id}">&times;</button>
    `;
    el.querySelector('.observation-delete').addEventListener('click', (e) => {
      e.stopPropagation();
      SC.wsSend('memory_observation_delete', { id: obs.id });
    });
    return el;
  }

  // --- User Profile Viewer ---

  var PROFILE_CATEGORIES = [
    { key: 'code_style', label: 'Code Style', icon: '{}' },
    { key: 'communication_style', label: 'Communication', icon: '>>' },
    { key: 'workflow_pattern', label: 'Work Patterns', icon: '||' }
  ];

  function renderUserProfile() {
    SC.renderToBoth('user-profile', 'user-profile-view', '<div class="loading-placeholder" style="color:var(--tx-2)">Loading profile...</div>');

    fetch('/api/memory/observations')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        var observations = data.observations || data || [];
        if (!Array.isArray(observations)) observations = [];

        SC.renderToBoth('user-profile', 'user-profile-view', function(el) {
          el.innerHTML = '';
          el.appendChild(buildProfileContent(observations));
        });
      })
      .catch(function() {
        SC.renderToBoth('user-profile', 'user-profile-view', '<div class="loading-placeholder" style="color:var(--tx-2)">Failed to load profile</div>');
      });
  }

  function buildProfileContent(observations) {
    var wrapper = document.createDocumentFragment();

    var header = document.createElement('div');
    header.className = 'profile-header';

    var title = document.createElement('span');
    title.className = 'profile-title';
    title.textContent = 'User Profile';
    header.appendChild(title);

    var actions = document.createElement('div');
    actions.className = 'profile-actions';

    var refreshBtn = document.createElement('button');
    refreshBtn.className = 'memory-edit-btn profile-refresh-btn';
    refreshBtn.textContent = 'Refresh';
    refreshBtn.addEventListener('click', function() { renderUserProfile(); });
    actions.appendChild(refreshBtn);

    var resetBtn = document.createElement('button');
    resetBtn.className = 'memory-edit-btn profile-reset-btn';
    resetBtn.textContent = 'Reset Profile';
    resetBtn.addEventListener('click', function() {
      resetBtn.disabled = true;
      resetBtn.textContent = 'Resetting...';
      fetch('/api/memory/observations', { method: 'DELETE' })
        .then(function() {
          renderUserProfile();
          if (SC.renderUserObservations) SC.renderUserObservations();
        })
        .catch(function() {
          resetBtn.disabled = false;
          resetBtn.textContent = 'Reset Profile';
        });
    });
    actions.appendChild(resetBtn);

    header.appendChild(actions);
    wrapper.appendChild(header);

    if (observations.length === 0) {
      var empty = document.createElement('div');
      empty.className = 'loading-placeholder';
      empty.style.color = 'var(--tx-2)';
      empty.textContent = 'No profile data yet';
      wrapper.appendChild(empty);
      return wrapper;
    }

    var grouped = {};
    observations.forEach(function(obs) {
      var cat = obs.category || 'other';
      if (!grouped[cat]) grouped[cat] = [];
      grouped[cat].push(obs);
    });

    PROFILE_CATEGORIES.forEach(function(catDef) {
      var items = grouped[catDef.key];
      if (!items || items.length === 0) return;
      delete grouped[catDef.key];
      wrapper.appendChild(buildProfileCategory(catDef.label, catDef.icon, items));
    });

    Object.keys(grouped).forEach(function(catKey) {
      if (grouped[catKey].length === 0) return;
      wrapper.appendChild(buildProfileCategory(catKey, '..', grouped[catKey]));
    });

    return wrapper;
  }

  function buildProfileCategory(label, icon, items) {
    var details = document.createElement('details');
    details.className = 'profile-category';
    details.open = true;

    var summary = document.createElement('summary');
    summary.className = 'profile-category-header';

    var iconSpan = document.createElement('span');
    iconSpan.className = 'profile-category-icon';
    iconSpan.textContent = icon;
    summary.appendChild(iconSpan);

    var labelSpan = document.createElement('span');
    labelSpan.className = 'profile-category-label';
    labelSpan.textContent = label;
    summary.appendChild(labelSpan);

    var countSpan = document.createElement('span');
    countSpan.className = 'profile-category-count';
    countSpan.textContent = items.length;
    summary.appendChild(countSpan);

    details.appendChild(summary);

    var body = document.createElement('div');
    body.className = 'profile-category-body';

    items.forEach(function(obs) {
      var row = document.createElement('div');
      row.className = 'profile-observation';

      var confidence = obs.confidence || 0;
      var confidencePct = Math.round(confidence * 100);

      var mainDiv = document.createElement('div');
      mainDiv.className = 'profile-observation-main';

      var keySpan = document.createElement('span');
      keySpan.className = 'profile-observation-key';
      keySpan.textContent = obs.key || '';
      mainDiv.appendChild(keySpan);

      var valueSpan = document.createElement('span');
      valueSpan.className = 'profile-observation-value';
      valueSpan.textContent = obs.value || '';
      mainDiv.appendChild(valueSpan);

      row.appendChild(mainDiv);

      var metaDiv = document.createElement('div');
      metaDiv.className = 'profile-observation-meta';

      var barWrap = document.createElement('div');
      barWrap.className = 'profile-confidence-bar';
      barWrap.title = 'Confidence: ' + confidencePct + '%';

      var barFill = document.createElement('div');
      barFill.className = 'profile-confidence-fill';
      barFill.style.width = confidencePct + '%';
      barWrap.appendChild(barFill);

      var pctSpan = document.createElement('span');
      pctSpan.className = 'profile-confidence-pct';
      pctSpan.textContent = confidencePct + '%';
      metaDiv.appendChild(barWrap);
      metaDiv.appendChild(pctSpan);

      if (obs.observedAt) {
        var d = new Date(obs.observedAt);
        var timeStr = d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) +
          ' ' + d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
        var timeSpan = document.createElement('span');
        timeSpan.className = 'profile-observation-time';
        timeSpan.textContent = timeStr;
        metaDiv.appendChild(timeSpan);
      }

      row.appendChild(metaDiv);
      body.appendChild(row);
    });

    details.appendChild(body);
    return details;
  }

  SC.renderMemoryLayers = renderMemoryLayers;
  SC.editMemoryFile = editMemoryFile;
  SC.saveMemoryFile = saveMemoryFile;
  SC.cancelMemoryEdit = cancelMemoryEdit;
  SC.renderMemoryStats = renderMemoryStats;
  SC.renderMemorySearchResults = renderMemorySearchResults;
  SC.renderMemoryRecall = renderMemoryRecall;
  SC.renderSessionFragments = renderSessionFragments;
  SC.renderUserObservations = renderUserObservations;
  SC.renderUserProfile = renderUserProfile;
})();
