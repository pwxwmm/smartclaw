// SmartClaw - Memory
(function() {
  'use strict';

  function renderMemoryLayers() {
    const memEl = SC.$('#memory-content');
    const usrEl = SC.$('#user-content');
    if (!memEl || !usrEl) return;
    if (memEl.tagName === 'PRE') {
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
    if (usrEl.tagName === 'PRE') {
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
    const el = SC.$('#memory-stats');
    if (!el) return;
    const s = SC.state.memoryStats || {};
    el.textContent = `${s.memory_chars || 0} chars / MEMORY.md · ${s.user_chars || 0} chars / USER.md`;
  }

  function renderMemorySearchResults(results) {
    const container = SC.$('#memory-search-results');
    container.innerHTML = '';
    if (!results || results.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No results</div>';
      return;
    }
    results.forEach(frag => {
      const el = document.createElement('div');
      el.className = 'memory-frag';
      const title = frag.source || frag.key || frag.title || 'Memory';
      const text = frag.content || frag.text || frag.snippet || '';
      el.innerHTML = `
        <div class="memory-frag-title">${SC.escapeHtml(title)}</div>
        <div class="memory-frag-text">${SC.escapeHtml(text.slice(0, 300))}</div>
      `;
      container.appendChild(el);
    });
  }

  function renderMemoryRecall(data) {
    if (!data) return;
    const el = document.createElement('div');
    el.className = 'message cmd-result';
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const query = data.query || '';
    const context = data.context || 'No context found';
    el.innerHTML = `<div class="msg-role" style="color:var(--accent)">Memory Recall</div><div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px">${SC.escapeHtml(context)}</div><div class="msg-ts">${ts}</div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  function renderSessionFragments() {
    const container = SC.$('#session-fragments');
    if (!container) return;
    container.innerHTML = '';
    if (!SC.state.sessionFragments || SC.state.sessionFragments.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">Search session history above</div>';
      return;
    }
    SC.state.sessionFragments.forEach(frag => {
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
      container.appendChild(el);
    });
  }

  function renderUserObservations() {
    const container = SC.$('#user-observations');
    if (!container) return;
    container.innerHTML = '';
    if (!SC.state.userObservations || SC.state.userObservations.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No observations yet</div>';
      return;
    }
    SC.state.userObservations.forEach(obs => {
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
      container.appendChild(el);
    });
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
})();
