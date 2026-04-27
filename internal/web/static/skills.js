// SmartClaw - Skills
(function() {
  'use strict';

  function renderSkillList() {
    const list = SC.$('#skill-list');
    list.innerHTML = '';

    const headerEl = SC.$('#skill-section-header');
    if (headerEl && !headerEl.querySelector('.btn-create-skill')) {
      const btn = document.createElement('button');
      btn.className = 'btn-create-skill';
      btn.textContent = '+';
      btn.title = 'Create Skill';
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        showSkillCreateForm();
      });
      headerEl.appendChild(btn);
    }

    if (!SC.state.skills || SC.state.skills.length === 0) {
      list.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No skills found</div>';
      return;
    }
    SC.state.skills.forEach(skill => {
      const el = document.createElement('div');
      el.className = 'skill-item';
      const desc = skill.description || '';
      const truncated = desc.length > 60 ? desc.slice(0, 60) + '...' : desc;
      const isOn = skill.enabled !== false;
      el.innerHTML = `
        <div class="skill-info">
          <div class="skill-name">${SC.escapeHtml(skill.name)}</div>
          ${truncated ? `<div class="skill-desc" title="${SC.escapeHtml(desc)}">${SC.escapeHtml(truncated)}</div>` : ''}
        </div>
        <div class="skill-toggle ${isOn ? 'on' : ''}" data-skill="${SC.escapeHtml(skill.name)}" data-enabled="${isOn}"></div>
      `;
      el.querySelector('.skill-name').addEventListener('click', (e) => {
        e.stopPropagation();
        SC.wsSend('skill_detail', { name: skill.name });
      });
      el.querySelector('.skill-toggle').addEventListener('click', (e) => {
        e.stopPropagation();
        const action = isOn ? 'disable' : 'enable';
        SC.wsSend('skill_toggle', { name: skill.name, action: action });
      });
      list.appendChild(el);
    });
  }

  function showSkillDetail(skill) {
    if (!skill) return;
    const el = document.createElement('div');
    el.className = 'message cmd-result';
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const content = skill.content || skill.description || 'No content available';
    el.innerHTML = `<div class="msg-role" style="color:var(--accent)">Skill: ${SC.escapeHtml(skill.name)}</div><div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px">${SC.escapeHtml(content)}</div><div class="msg-ts">${ts}</div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
  }

  function showSkillCreateForm() {
    let overlay = SC.$('#skill-create-overlay');
    if (overlay) { overlay.classList.add('visible'); return; }

    overlay = document.createElement('div');
    overlay.id = 'skill-create-overlay';
    overlay.className = 'skill-create-overlay';
    overlay.innerHTML = `
      <div class="skill-create-form">
        <div class="skill-create-header">
          <span>Create Skill</span>
          <button class="skill-create-close" id="skill-create-close">&times;</button>
        </div>
        <div class="skill-create-body">
          <label class="skill-create-label">Name <span class="required">*</span></label>
          <input type="text" id="sc-name" class="skill-create-input" placeholder="my-skill" required>
          <label class="skill-create-label">Description <span class="required">*</span></label>
          <textarea id="sc-desc" class="skill-create-textarea" placeholder="What this skill does..." rows="2" required></textarea>
          <label class="skill-create-label">Version</label>
          <input type="text" id="sc-version" class="skill-create-input" placeholder="1.0" value="1.0">
          <label class="skill-create-label">Tags (comma-separated)</label>
          <input type="text" id="sc-tags" class="skill-create-input" placeholder="automation, code-review">
          <label class="skill-create-label">Tools (comma-separated)</label>
          <input type="text" id="sc-tools" class="skill-create-input" placeholder="bash, read_file, write_file">
          <label class="skill-create-label">Triggers (comma-separated)</label>
          <input type="text" id="sc-triggers" class="skill-create-input" placeholder="/my-skill">
          <label class="skill-create-label">Body (markdown content)</label>
          <textarea id="sc-body" class="skill-create-textarea skill-create-body-editor" placeholder="# My Skill\\n\\nInstructions for the skill..." rows="8"></textarea>
        </div>
        <div class="skill-create-actions">
          <button class="skill-create-btn skill-create-cancel" id="skill-create-cancel">Cancel</button>
          <button class="skill-create-btn skill-create-submit" id="skill-create-submit">Create</button>
        </div>
      </div>
    `;
    document.body.appendChild(overlay);
    requestAnimationFrame(() => overlay.classList.add('visible'));

    overlay.querySelector('#skill-create-close').addEventListener('click', closeSkillCreateForm);
    overlay.querySelector('#skill-create-cancel').addEventListener('click', closeSkillCreateForm);
    overlay.querySelector('#skill-create-submit').addEventListener('click', submitSkillCreate);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) closeSkillCreateForm(); });
  }

  function closeSkillCreateForm() {
    const overlay = SC.$('#skill-create-overlay');
    if (!overlay) return;
    overlay.classList.remove('visible');
    setTimeout(() => overlay.remove(), 200);
  }

  function submitSkillCreate() {
    const name = SC.$('#sc-name')?.value?.trim() || '';
    const description = SC.$('#sc-desc')?.value?.trim() || '';
    const version = SC.$('#sc-version')?.value?.trim() || '1.0';
    const tagsStr = SC.$('#sc-tags')?.value?.trim() || '';
    const toolsStr = SC.$('#sc-tools')?.value?.trim() || '';
    const triggersStr = SC.$('#sc-triggers')?.value?.trim() || '';
    const body = SC.$('#sc-body')?.value || '';

    if (!name || !description) {
      SC.toast('Name and description are required', 'error');
      return;
    }

    const tags = tagsStr ? tagsStr.split(',').map(s => s.trim()).filter(Boolean) : [];
    const tools = toolsStr ? toolsStr.split(',').map(s => s.trim()).filter(Boolean) : [];
    const triggers = triggersStr ? triggersStr.split(',').map(s => s.trim()).filter(Boolean) : [];

    SC.wsSend('skill_create', { name, description, version, tags, tools, triggers, body });
  }

  function renderSkillMemoryList() {
    const container = SC.$('#skill-memory-list');
    if (!container) return;
    container.innerHTML = '';
    if (!SC.state.skills || SC.state.skills.length === 0) {
      container.innerHTML = '<div class="loading-placeholder" style="color:var(--tx-2)">No skills found</div>';
      return;
    }
    SC.state.skills.forEach(skill => {
      const el = document.createElement('div');
      el.className = 'skill-memory-item';
      const desc = skill.description || '';
      const truncated = desc.length > 60 ? desc.slice(0, 60) + '...' : desc;
      const source = skill.source || 'local';
      el.innerHTML = `
        <div class="skill-memory-info">
          <div class="skill-memory-name">${SC.escapeHtml(skill.name)}</div>
          ${truncated ? `<div class="skill-memory-desc">${SC.escapeHtml(truncated)}</div>` : ''}
          <div class="skill-memory-meta">${SC.escapeHtml(source)}</div>
        </div>
        ${source !== 'bundled' ? '<button class="skill-memory-edit memory-edit-btn">Edit</button>' : ''}
      `;
      const editBtn = el.querySelector('.skill-memory-edit');
      if (editBtn) {
        editBtn.addEventListener('click', (e) => {
          e.stopPropagation();
          openSkillEditor(skill.name);
        });
      }
      container.appendChild(el);
    });
  }

  function openSkillEditor(name) {
    SC.state.editingSkill = name;
    const editor = SC.$('#skill-editor');
    const nameEl = SC.$('#skill-editor-name');
    const contentEl = SC.$('#skill-editor-content');
    if (!editor || !nameEl || !contentEl) return;

    nameEl.textContent = name;
    const skill = (SC.state.skills || []).find(s => s.name === name);
    contentEl.value = skill?.content || '';

    SC.wsSend('skill_detail', { name });
    const origHandler = SC.handleWSMessage;
    const tempHandler = (msg) => {
      if (msg.type === 'skill_detail' && msg.data?.name === name) {
        contentEl.value = msg.data.content || '';
        editor.classList.remove('hidden');
      }
    };
    const origOnMessage = SC.state.ws.onmessage;
    SC.state.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        tempHandler(msg);
      } catch {}
      if (origOnMessage) origOnMessage.call(SC.state.ws, e);
    };

    editor.classList.remove('hidden');
  }

  SC.renderSkillList = renderSkillList;
  SC.showSkillDetail = showSkillDetail;
  SC.showSkillCreateForm = showSkillCreateForm;
  SC.closeSkillCreateForm = closeSkillCreateForm;
  SC.submitSkillCreate = submitSkillCreate;
  SC.renderSkillMemoryList = renderSkillMemoryList;
  SC.openSkillEditor = openSkillEditor;
})();
