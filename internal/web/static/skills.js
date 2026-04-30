// SmartClaw - Skills
(function() {
  'use strict';

  var healthCache = { data: null, ts: 0 };
  var trendingCache = { data: null, ts: 0 };
  var CACHE_TTL = 60000;
  var SKILL_ORDER_KEY = 'smartclaw-skill-order';

  function fetchJSON(url) {
    return fetch(url).then(function(r) { return r.json(); });
  }

  function getCachedHealth() {
    if (healthCache.data && (Date.now() - healthCache.ts) < CACHE_TTL) {
      return Promise.resolve(healthCache.data);
    }
    return fetchJSON('/api/skills/health').then(function(data) {
      healthCache = { data: data, ts: Date.now() };
      return data;
    }).catch(function() { return null; });
  }

  function getCachedTrending() {
    if (trendingCache.data && (Date.now() - trendingCache.ts) < CACHE_TTL) {
      return Promise.resolve(trendingCache.data);
    }
    return fetchJSON('/api/skills/trending?limit=5').then(function(data) {
      trendingCache = { data: data, ts: Date.now() };
      return data;
    }).catch(function() { return []; });
  }

  function relativeTime(isoStr) {
    if (!isoStr) return '';
    var then = new Date(isoStr);
    var diff = (Date.now() - then.getTime()) / 1000;
    if (diff < 60) return 'just now';
    if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
    if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
    return Math.floor(diff / 86400) + 'd ago';
  }

  function healthColor(h) {
    if (h === 'healthy') return '#22c55e';
    if (h === 'degraded') return '#eab308';
    if (h === 'failing') return '#ef4444';
    return '#94a3b8';
  }

  function trendArrow(t) {
    if (t === 'improving') return '\u2197';
    if (t === 'declining') return '\u2198';
    return '\u2192';
  }

  function trendColor(t) {
    if (t === 'improving') return '#22c55e';
    if (t === 'declining') return '#ef4444';
    return '#94a3b8';
  }

  function renderHealthDashboard(report) {
    var container = SC.$('#skill-health-dashboard');
    var viewContainer = SC.$('#skill-health-dashboard-view');
    if (!container && !viewContainer) return;
    
    var targets = [container, viewContainer].filter(Boolean);
    targets.forEach(function(target) {
      target.innerHTML = '';

      if (!report) {
        target.innerHTML = '<div class="skill-health-empty">Health data unavailable</div>';
        return;
      }

      var healthy = report.healthy || 0;
      var degraded = report.degraded || 0;
      var failing = report.failing || 0;
      var unused = report.unused || 0;
      var total = healthy + degraded + failing + unused;

      var summaryEl = document.createElement('div');
      summaryEl.className = 'skill-health-summary';

      var badgesHtml = '<span class="skill-health-badge" style="--hc:#22c55e">' + healthy + ' healthy</span>';
      badgesHtml += '<span class="skill-health-badge" style="--hc:#eab308">' + degraded + ' degraded</span>';
      badgesHtml += '<span class="skill-health-badge" style="--hc:#ef4444">' + failing + ' failing</span>';
      badgesHtml += '<span class="skill-health-badge" style="--hc:#94a3b8">' + unused + ' unused</span>';

      var ringHtml = '';
      if (total > 0) {
        var pct = Math.round((healthy / total) * 100);
        ringHtml = '<div class="skill-health-ring" style="--ring-pct:' + pct + ';--ring-color:#22c55e">';
        ringHtml += '<span class="skill-health-ring-label">' + pct + '%</span>';
        ringHtml += '</div>';
      }

      summaryEl.innerHTML = ringHtml + '<div class="skill-health-badges">' + badgesHtml + '</div>';
      target.appendChild(summaryEl);
    });
  }

  function renderTrendingSection(trending) {
    var container = SC.$('#skill-trending-section');
    var viewContainer = SC.$('#skill-trending-section-view');
    if (!container && !viewContainer) return;

    var targets = [container, viewContainer].filter(Boolean);
    targets.forEach(function(target) {
      target.innerHTML = '';

      if (!trending || trending.length === 0) {
        target.innerHTML = '<div class="skill-trending-empty">No trending skills this week</div>';
        return;
      }

      trending.forEach(function(t) {
        var el = document.createElement('div');
        el.className = 'skill-trending-item';
        el.innerHTML =
          '<div class="skill-trending-info">' +
            '<span class="skill-trending-name">' + SC.escapeHtml(t.skill_id) + '</span>' +
            '<span class="skill-trending-count">' + t.usage_count + ' uses</span>' +
          '</div>' +
          '<span class="skill-trending-time">' + relativeTime(t.last_used) + '</span>';
        target.appendChild(el);
      });
    });
  }

  function setupSkillDragSort(container) {
    if (!container || container.dataset.dragSortSetup === 'true') return;
    container.dataset.dragSortSetup = 'true';

    SC.makeDraggable(container, {
      dropTarget: container,
      onDragStart: function(e) {
        var item = e.target.closest('.skill-item');
        if (!item) return;
        e.dataTransfer.setData('text/plain', item.dataset.skillName);
        item.classList.add('dragging');
      },
      onDragEnd: function() {
        container.querySelectorAll('.skill-item.dragging').forEach(function(el) {
          el.classList.remove('dragging');
        });
        container.querySelectorAll('.skill-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });
      },
      onDragOver: function(e) {
        var target = e.target.closest('.skill-item');
        if (!target || target.classList.contains('dragging')) return;
        container.querySelectorAll('.skill-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });
        target.classList.add('drag-over');
      },
      onDrop: function(e) {
        var target = e.target.closest('.skill-item');
        if (!target) return;
        var draggedName = e.dataTransfer.getData('text/plain');
        var items = Array.from(container.querySelectorAll('.skill-item'));
        var draggedItem = items.find(function(item) { return item.dataset.skillName === draggedName; });
        if (!draggedItem || draggedItem === target) return;

        var allItems = Array.from(container.children);
        var draggedIdx = allItems.indexOf(draggedItem);
        var targetIdx = allItems.indexOf(target);

        if (draggedIdx < targetIdx) {
          container.insertBefore(draggedItem, target.nextSibling);
        } else {
          container.insertBefore(draggedItem, target);
        }

        container.querySelectorAll('.skill-item.drag-over').forEach(function(el) {
          el.classList.remove('drag-over');
        });

        saveSkillOrder(container);
      }
    });
  }

  function saveSkillOrder(container) {
    var names = Array.from(container.querySelectorAll('.skill-item')).map(function(el) {
      return el.dataset.skillName;
    });
    try { localStorage.setItem(SKILL_ORDER_KEY, JSON.stringify(names)); } catch {}
  }

  function applySkillOrder(skills) {
    try {
      var saved = localStorage.getItem(SKILL_ORDER_KEY);
      if (!saved) return skills;
      var order = JSON.parse(saved);
      var skillMap = {};
      skills.forEach(function(s) { skillMap[s.name] = s; });
      var ordered = [];
      order.forEach(function(name) {
        if (skillMap[name]) {
          ordered.push(skillMap[name]);
          delete skillMap[name];
        }
      });
      Object.values(skillMap).forEach(function(s) { ordered.push(s); });
      return ordered;
    } catch { return skills; }
  }

  function renderSkillList() {
    var list = SC.$('#skill-list');
    if (list) list.innerHTML = '';

    if (!SC.$('#skill-health-dashboard-view')) {
      initHealthDashboard();
    }

    if (!SC.state.skills || SC.state.skills.length === 0) {
      SC.renderToBoth('skill-list', 'skill-list-view', '<div class="loading-placeholder" style="color:var(--tx-2)">No skills found</div>');
      return;
    }

    getCachedHealth().then(function(report) {
      var healthMap = {};
      if (report && report.skills) {
        report.skills.forEach(function(s) { healthMap[s.skill_id] = s; });
      }

      var orderedSkills = applySkillOrder(SC.state.skills);

      orderedSkills.forEach(function(skill) {
        if (list) list.appendChild(createSkillItem(skill, healthMap));
      });

      if (list && typeof SC.applyListStagger === 'function') SC.applyListStagger(list, '.skill-item');
      if (list) setupSkillDragSort(list);

      SC.renderToBoth('skill-list-view', null, function(viewList) {
        viewList.innerHTML = '';
        orderedSkills.forEach(function(skill) {
          viewList.appendChild(createSkillItem(skill, healthMap));
        });
        if (typeof SC.applyListStagger === 'function') SC.applyListStagger(viewList, '.skill-item');
        setupSkillDragSort(viewList);
      });
    });

    getCachedTrending().then(renderTrendingSection);
  }

  function createSkillItem(skill, healthMap) {
    var el = document.createElement('div');
    el.className = 'skill-item';
    el.dataset.skillName = skill.name;
    el.draggable = true;
    var desc = skill.description || '';
    var truncated = desc.length > 60 ? desc.slice(0, 60) + '...' : desc;
    var isOn = skill.enabled !== false;

    var h = healthMap[skill.name];
    var dotHtml = '<span class="skill-health-dot" style="--hd:' + (h ? healthColor(h.health) : '#94a3b8') + '"></span>';

    var metaHtml = '';
    if (h) {
      var pct = Math.round(h.success_rate * 100);
      metaHtml = '<div class="skill-meta">';
      metaHtml += '<span class="skill-success-rate" style="color:' + healthColor(h.health) + '">' + pct + '%</span>';
      metaHtml += '<span class="skill-trend-arrow" style="color:' + trendColor(h.trend) + '">' + trendArrow(h.trend) + '</span>';
      if (h.last_used) {
        metaHtml += '<span class="skill-last-used">' + relativeTime(h.last_used) + '</span>';
      }
      if (h.total_invocations) {
        metaHtml += '<span class="skill-invocations">' + h.total_invocations + '\u00d7</span>';
      }
      metaHtml += '</div>';
    }

    el.innerHTML =
      '<span class="drag-handle">⋮⋮</span>' +
      dotHtml +
      '<div class="skill-info">' +
        '<div class="skill-name">' + SC.escapeHtml(skill.name) + '</div>' +
        (truncated ? '<div class="skill-desc" title="' + SC.escapeHtml(desc) + '">' + SC.escapeHtml(truncated) + '</div>' : '') +
        metaHtml +
      '</div>' +
      '<div class="skill-toggle ' + (isOn ? 'on' : '') + '" data-skill="' + SC.escapeHtml(skill.name) + '" data-enabled="' + isOn + '"></div>';

    el.querySelector('.skill-name').addEventListener('click', function(e) {
      e.stopPropagation();
      toggleSkillDetail(skill.name, el, h);
    });

    el.querySelector('.skill-toggle').addEventListener('click', function(e) {
      e.stopPropagation();
      var action = isOn ? 'disable' : 'enable';
      SC.wsSend('skill_toggle', { name: skill.name, action: action });
    });

    return el;
  }

  function toggleSkillDetail(name, itemEl, healthEntry) {
    var existingPanel = itemEl.querySelector('.skill-detail-panel');
    if (existingPanel) {
      existingPanel.remove();
      return;
    }

    document.querySelectorAll('.skill-detail-panel').forEach(function(p) { p.remove(); });

    var panel = document.createElement('div');
    panel.className = 'skill-detail-panel';

    var html = '';

    if (healthEntry) {
      html += '<div class="skill-detail-health">';
      html += '<span class="skill-health-badge" style="--hc:' + healthColor(healthEntry.health) + '">' + SC.escapeHtml(healthEntry.health) + '</span>';
      html += '<span class="skill-detail-trend" style="color:' + trendColor(healthEntry.trend) + '">' + trendArrow(healthEntry.trend) + ' ' + SC.escapeHtml(healthEntry.trend) + '</span>';
      html += '</div>';

      html += '<div class="skill-detail-recommendation">' + SC.escapeHtml(healthEntry.recommendation || '') + '</div>';

      html += '<div class="skill-bar-chart">';
      var total = healthEntry.total_invocations || 1;
      var successW = Math.round((healthEntry.success_rate || 0) * 100);
      var failPct = total > 0 ? Math.round(((1 - healthEntry.success_rate) * 100)) : 0;
      html += '<div class="skill-bar-fill" style="width:' + successW + '%;background:#22c55e" title="Success"></div>';
      if (failPct > 0) {
        html += '<div class="skill-bar-fill" style="width:' + failPct + '%;background:#ef4444;left:' + successW + '%" title="Failures/Overrides"></div>';
      }
      html += '</div>';

      if (healthEntry.health === 'failing' || healthEntry.health === 'degraded') {
        html += '<button class="skill-improve-detail-btn" data-skill="' + SC.escapeHtml(name) + '">Improve Skill</button>';
      }
    } else {
      html += '<div class="skill-detail-recommendation">No health data available for this skill</div>';
    }

    panel.innerHTML = html;
    itemEl.appendChild(panel);

    var improveBtn = panel.querySelector('.skill-improve-detail-btn');
    if (improveBtn) {
      improveBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        SC.wsSend('skill_improve', { name: name, failure_messages: [] });
        improveBtn.textContent = 'Improving...';
        improveBtn.disabled = true;
      });
    }
  }

  function showSkillDetail(skill) {
    if (!skill) return;
    var now = new Date();
    var ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    var content = skill.content || skill.description || 'No content available';
    var el = SC.renderMessageCard('cmd-result msg-group-start', SC.escapeHtml(content), ts, {
      roleLabel: 'Skill: ' + skill.name,
      style: 'font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px'
    });
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

  function renderSkillHealth(data) {
    if (!data) return;
    renderHealthDashboard(data);

    var skills = data.skills || [];
    var healthy = data.healthy || 0;
    var degraded = data.degraded || 0;
    var failing = data.failing || 0;
    var unused = data.unused || 0;

    var now = new Date();
    var ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    let bubbleHtml = '';

    bubbleHtml += '<div style="display:flex;gap:12px;margin-bottom:10px;font-size:0.85em">';
    bubbleHtml += '<span style="color:#22c55e">&#10003; ' + healthy + ' healthy</span>';
    bubbleHtml += '<span style="color:#eab308">&#9888; ' + degraded + ' degraded</span>';
    bubbleHtml += '<span style="color:#ef4444">&#10007; ' + failing + ' failing</span>';
    bubbleHtml += '<span style="color:#94a3b8">&#9675; ' + unused + ' unused</span>';
    bubbleHtml += '</div>';

    if (skills.length === 0) {
      bubbleHtml += '<div style="color:var(--tx-2);font-size:0.9em">No tracked skills found. Skills appear here after being used.</div>';
    } else {
      bubbleHtml += '<table style="width:100%;font-size:0.85em;border-collapse:collapse">';
      bubbleHtml += '<tr style="border-bottom:1px solid var(--bd)"><th style="text-align:left;padding:4px 8px">Skill</th><th style="text-align:right;padding:4px 8px">Success</th><th style="text-align:center;padding:4px 8px">Trend</th><th style="text-align:left;padding:4px 8px">Status</th><th style="padding:4px 8px"></th></tr>';
      skills.forEach(skill => {
        const pct = Math.round(skill.success_rate * 100);
        const trendIcon = skill.trend === 'improving' ? '&#8599;' : skill.trend === 'declining' ? '&#8600;' : '&#8594;';
        const trendColorVal = skill.trend === 'improving' ? '#22c55e' : skill.trend === 'declining' ? '#ef4444' : '#94a3b8';
        const hc = healthColor(skill.health);

        bubbleHtml += '<tr style="border-bottom:1px solid var(--bd)">';
        bubbleHtml += '<td style="padding:4px 8px">' + SC.escapeHtml(skill.skill_id) + '</td>';
        bubbleHtml += '<td style="text-align:right;padding:4px 8px;color:' + hc + '">' + pct + '%</td>';
        bubbleHtml += '<td style="text-align:center;padding:4px 8px;color:' + trendColorVal + '">' + trendIcon + '</td>';
        bubbleHtml += '<td style="padding:4px 8px;color:' + hc + '">' + SC.escapeHtml(skill.health) + '</td>';
        if (skill.health === 'failing' || skill.health === 'degraded') {
          bubbleHtml += '<td style="padding:4px 8px"><button class="skill-improve-btn" data-skill="' + SC.escapeHtml(skill.skill_id) + '" style="font-size:0.8em;padding:2px 8px;border-radius:4px;border:1px solid var(--bd);background:var(--bg-2);cursor:pointer;color:var(--tx-1)">Improve</button></td>';
        } else {
          bubbleHtml += '<td style="padding:4px 8px"></td>';
        }
        bubbleHtml += '</tr>';
      });
      bubbleHtml += '</table>';
    }

    el = SC.renderMessageCard('cmd-result msg-group-start', bubbleHtml, ts, {
      roleLabel: 'Skill Health Report',
      style: 'font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;padding:10px 14px'
    });
    SC.$('#messages').appendChild(el);
    SC.scrollChat();

    el.querySelectorAll('.skill-improve-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const skillName = btn.getAttribute('data-skill');
        if (skillName) {
          SC.wsSend('skill_improve', { name: skillName, failure_messages: [] });
          btn.textContent = 'Improving...';
          btn.disabled = true;
        }
      });
    });
  }

  function createSkillMemoryItem(skill) {
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
    return el;
  }

  function renderSkillMemoryList() {
    var container = SC.$('#skill-memory-list');
    var emptyHtml = '<div class="loading-placeholder" style="color:var(--tx-2)">No skills found</div>';
    if (!SC.state.skills || SC.state.skills.length === 0) {
      if (container) container.innerHTML = emptyHtml;
      SC.renderToBoth('skill-memory-list-view', 'skill-memory-list-view-2', emptyHtml);
      return;
    }
    if (container) {
      container.innerHTML = '';
      SC.state.skills.forEach(skill => {
        container.appendChild(createSkillMemoryItem(skill));
      });
    }

    SC.renderToBoth('skill-memory-list-view', 'skill-memory-list-view-2', function(el) {
      el.innerHTML = '';
      SC.state.skills.forEach(skill => { el.appendChild(createSkillMemoryItem(skill)); });
    });
  }

  function openSkillEditor(name) {
    SC.state.editingSkill = name;
    var editor = SC.$('#skill-editor');
    var editorView = SC.$('#skill-editor-view');
    var nameEl = editor ? SC.$('#skill-editor-name') : null;
    var contentEl = editor ? SC.$('#skill-editor-content') : null;
    var nameViewEl = editorView ? SC.$('#skill-editor-name-view') : null;
    var contentViewEl = editorView ? SC.$('#skill-editor-content-view') : null;

    var skill = (SC.state.skills || []).find(s => s.name === name);
    var skillContent = skill?.content || '';

    if (nameEl) nameEl.textContent = name;
    if (contentEl) contentEl.value = skillContent;
    if (nameViewEl) nameViewEl.textContent = name;
    if (contentViewEl) contentViewEl.value = skillContent;

    SC.wsSend('skill_detail', { name });
    if (!SC.state._wsListeners) SC.state._wsListeners = {};
    if (!SC.state._wsListeners['skill_detail']) SC.state._wsListeners['skill_detail'] = [];
    SC.state._wsListeners['skill_detail'].push(function handler(data) {
      if (data.data && data.data.name === name) {
        if (contentEl) contentEl.value = data.data.content || '';
        if (contentViewEl) contentViewEl.value = data.data.content || '';
        if (editor) editor.classList.remove('hidden');
        if (editorView) editorView.classList.remove('hidden');
      }
      var idx = SC.state._wsListeners['skill_detail'].indexOf(handler);
      if (idx > -1) SC.state._wsListeners['skill_detail'].splice(idx, 1);
    });

    if (editor) editor.classList.remove('hidden');
    if (editorView) editorView.classList.remove('hidden');
  }

  function initHealthDashboard() {
    var viewDashEl = SC.$('#skill-health-dashboard-view');
    if (viewDashEl && !viewDashEl.dataset.initialized) {
      viewDashEl.dataset.initialized = 'true';
      viewDashEl.innerHTML = '<div class="skill-health-empty">Loading health data...</div>';

      if (!SC.$('#skill-trending-section-view')) {
        var trendingHeaderView = document.createElement('div');
        trendingHeaderView.className = 'skill-trending-header';
        trendingHeaderView.textContent = 'TRENDING';
        var trendingContainerView = document.createElement('div');
        trendingContainerView.id = 'skill-trending-section-view';
        trendingContainerView.className = 'skill-trending-section';

        var skillListView = SC.$('#skill-list-view');
        if (skillListView) {
          skillListView.parentElement.insertBefore(trendingHeaderView, skillListView);
          skillListView.parentElement.insertBefore(trendingContainerView, skillListView);
        }
      }

      getCachedHealth().then(renderHealthDashboard);
      getCachedTrending().then(renderTrendingSection);
    }
  }

  SC.renderSkillList = renderSkillList;
  SC.showSkillDetail = showSkillDetail;
  SC.showSkillCreateForm = showSkillCreateForm;
  SC.closeSkillCreateForm = closeSkillCreateForm;
  SC.submitSkillCreate = submitSkillCreate;
  SC.renderSkillMemoryList = renderSkillMemoryList;
  SC.openSkillEditor = openSkillEditor;
  SC.renderSkillHealth = renderSkillHealth;
  SC.initSkillHealthDashboard = initHealthDashboard;
})();
