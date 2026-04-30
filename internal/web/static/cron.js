// SmartClaw - Cron Panel
(function() {
  'use strict';

  function renderCronPanel() {
    renderCronPanelInto(SC.$('#cron-panel'));
    renderCronPanelInto(SC.$('#cron-panel-view'));
  }

  function renderCronPanelInto(container) {
    if (!container) return;
    container.innerHTML = '';

    const tasks = SC.state.cronTasks || [];

    if (tasks.length === 0) {
      SC.showEmptyState(container,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
        'No scheduled tasks',
        'Create a cron task to run instructions on a schedule.'
      );
      appendNewButton(container);
      return;
    }

    const list = document.createElement('div');
    list.className = 'cron-task-list';

    tasks.forEach(task => {
      const card = document.createElement('div');
      card.className = 'cron-task-card';
      card.id = `cron-task-${task.id}`;

      const shortId = (task.id || '').slice(0, 8);
      const schedule = task.schedule || '';
      const instruction = task.instruction || '';
      const enabled = task.enabled !== false;
      const lastRun = task.last_run_at ? formatCronTime(task.last_run_at) : 'Never';
      const statusLabel = enabled ? 'Enabled' : 'Disabled';
      const statusClass = enabled ? 'cron-status-enabled' : 'cron-status-disabled';

      card.innerHTML = `
        <div class="cron-task-head">
          <span class="cron-task-schedule" title="${SC.escapeHtml(schedule)}">${SC.escapeHtml(humanizeSchedule(schedule))}</span>
          <span class="cron-task-status ${statusClass}">${SC.escapeHtml(statusLabel)}</span>
        </div>
        <div class="cron-task-instruction" title="${SC.escapeHtml(instruction)}">${SC.escapeHtml(truncate(instruction, 60))}</div>
        <div class="cron-task-meta">
          <span class="cron-task-id">${SC.escapeHtml(shortId)}</span>
          <span class="cron-task-last-run">Last: ${SC.escapeHtml(lastRun)}</span>
        </div>
        <div class="cron-task-actions">
          <button class="cron-action-btn cron-toggle-btn" data-id="${SC.escapeHtml(task.id)}" title="${enabled ? 'Disable' : 'Enable'}">${enabled ? '&#9646;&#9646;' : '&#9654;'}</button>
          <button class="cron-action-btn cron-run-btn" data-id="${SC.escapeHtml(task.id)}" title="Run now">&#9654;</button>
          <button class="cron-action-btn cron-delete-btn" data-id="${SC.escapeHtml(task.id)}" title="Delete">&times;</button>
        </div>
      `;
      list.appendChild(card);
    });

    container.appendChild(list);
    appendNewButton(container);
    bindCronActions(container);
  }

  function appendNewButton(container) {
    const btn = document.createElement('button');
    btn.className = 'btn-primary cron-new-btn';
    btn.id = 'cron-new-btn';
    btn.textContent = '+ New Task';
    container.appendChild(btn);

    btn.addEventListener('click', showCronForm);
  }

  function showCronForm() {
    const container = SC.$('#cron-panel');
    if (!container) return;

    const existing = container.querySelector('.cron-form');
    if (existing) {
      existing.remove();
      return;
    }

    const form = document.createElement('div');
    form.className = 'cron-form';
    form.innerHTML = `
      <div class="cron-form-title">New Scheduled Task</div>
      <label class="cron-form-label">Schedule
        <input type="text" id="cron-schedule-input" class="cron-form-input" placeholder='e.g. "every day at 9am" or "0 9 * * *"'>
      </label>
      <div class="cron-schedule-preview" id="cron-schedule-preview"></div>
      <label class="cron-form-label">Instruction
        <textarea id="cron-instruction-input" class="cron-form-textarea" placeholder="What should the agent do?" rows="3"></textarea>
      </label>
      <div class="cron-form-actions">
        <button class="btn-ghost sm" id="cron-form-cancel">Cancel</button>
        <button class="btn-primary sm" id="cron-form-create">Create</button>
      </div>
    `;

    container.insertBefore(form, container.querySelector('.cron-new-btn'));

    SC.$('#cron-form-cancel').addEventListener('click', () => form.remove());
    SC.$('#cron-form-create').addEventListener('click', createCronTask);

    const scheduleInput = SC.$('#cron-schedule-input');
    scheduleInput.addEventListener('input', () => {
      const val = scheduleInput.value.trim();
      const preview = SC.$('#cron-schedule-preview');
      if (!val) {
        preview.textContent = '';
        return;
      }
      const parsed = previewSchedule(val);
      if (parsed) {
        preview.textContent = `Cron: ${parsed}`;
        preview.className = 'cron-schedule-preview cron-preview-valid';
      } else {
        preview.textContent = 'Will attempt natural language parsing';
        preview.className = 'cron-schedule-preview cron-preview-nl';
      }
    });

    scheduleInput.focus();
  }

  function createCronTask() {
    const schedule = SC.$('#cron-schedule-input')?.value?.trim();
    const instruction = SC.$('#cron-instruction-input')?.value?.trim();

    if (!schedule || !instruction) {
      SC.toast('Schedule and instruction are required', 'error');
      return;
    }

    SC.wsSend('cron_create', { schedule, instruction });
  }

  function bindCronActions(container) {
    container.querySelectorAll('.cron-toggle-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.dataset.id;
        if (id) SC.wsSend('cron_toggle', { id });
      });
    });

    container.querySelectorAll('.cron-run-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.dataset.id;
        if (id) SC.wsSend('cron_run', { id });
      });
    });

    container.querySelectorAll('.cron-delete-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const id = btn.dataset.id;
        if (id) SC.wsSend('cron_delete', { id });
      });
    });
  }

  function humanizeSchedule(schedule) {
    if (!schedule) return '(no schedule)';
    const parts = schedule.trim().split(/\s+/);
    if (parts.length === 5) {
      const [min, hour, dom, mon, dow] = parts;
      if (dom === '*' && mon === '*' && dow === '*') {
        if (min === '0' && hour !== '*') return `Every day at ${hour}:00`;
        if (min !== '*' && hour !== '*') return `Daily at ${hour}:${min.padStart(2, '0')}`;
      }
      if (dow !== '*' && dom === '*' && mon === '*') {
        const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        const dayName = days[parseInt(dow)] || dow;
        if (min !== '*' && hour !== '*') return `Every ${dayName} at ${hour}:${min.padStart(2, '0')}`;
      }
      if (dom !== '*' && mon === '*' && dow === '*') {
        if (min !== '*' && hour !== '*') return `Monthly on day ${dom} at ${hour}:${min.padStart(2, '0')}`;
      }
      return schedule;
    }
    return schedule;
  }

  function previewSchedule(input) {
    const parts = input.trim().split(/\s+/);
    if (parts.length === 5) {
      return input;
    }
    return null;
  }

  function formatCronTime(ts) {
    if (!ts) return 'Never';
    try {
      const d = new Date(ts);
      if (isNaN(d.getTime())) return 'Never';
      return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    } catch {
      return 'Never';
    }
  }

  function truncate(str, len) {
    if (!str) return '';
    return str.length > len ? str.slice(0, len) + '...' : str;
  }

  SC.renderCronPanel = renderCronPanel;
})();
