// SmartClaw - Entry Point
(function() {
  'use strict';

  function init() {
    SC.loadSettings();
    if (typeof window.mermaid !== 'undefined') {
      mermaid.initialize({ startOnLoad: false, theme: 'dark' });
    }
    if (typeof SC.initVirtualList === 'function') {
      SC.initVirtualList();
    }
    SC.wsConnect();
    SC.initDragDrop();
    SC.initCmdPalette();
    SC.initFileMention();
    SC.initFileSearch();
    SC.initSidebarDragSort();
    SC.initNotifications();
    SC.initThemeEditor();
    SC.showWelcome();

    SC.$('#btn-send').addEventListener('click', SC.sendMessage);
    SC.$('#input').addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); SC.sendMessage(); }
      else if (e.key === 'ArrowUp' && e.target.value === '' && SC.state.commandHistory.length > 0) {
        e.preventDefault();
        SC.state.historyIndex = Math.max(0, SC.state.historyIndex - 1);
        e.target.value = SC.state.commandHistory[SC.state.historyIndex] || '';
        e.target.style.height = 'auto';
        e.target.style.height = Math.min(e.target.scrollHeight, 200) + 'px';
      }
      else if (e.key === 'ArrowDown' && e.target.value === '' && SC.state.commandHistory.length > 0) {
        e.preventDefault();
        SC.state.historyIndex = Math.min(SC.state.commandHistory.length, SC.state.historyIndex + 1);
        e.target.value = SC.state.commandHistory[SC.state.historyIndex] || '';
        e.target.style.height = 'auto';
        e.target.style.height = Math.min(e.target.scrollHeight, 200) + 'px';
      }
    });
    SC.$('#input').addEventListener('input', function() {
      this.style.height = 'auto';
      this.style.height = Math.min(this.scrollHeight, 200) + 'px';
    });

    SC.$('#sidebar-open').addEventListener('click', () => {
      const sb = SC.$('#sidebar');
      if (sb.classList.contains('collapsed')) {
        sb.classList.remove('collapsed');
        SC.state.ui.sidebarOpen = true;
      } else {
        sb.classList.add('collapsed');
        SC.state.ui.sidebarOpen = false;
      }
    });

    SC.$$('.nav-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        SC.$$('.nav-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        SC.$$('.section').forEach(s => s.classList.remove('active'));
        const section = btn.dataset.section;
        SC.$(`#section-${section}`)?.classList.add('active');
        SC.loadSectionData(section);
      });
    });

    SC.$('#drawer-close').addEventListener('click', SC.closeDrawer);
    SC.$('#drawer-add-context').addEventListener('click', () => {
      const selection = window.getSelection();
      const selectedText = selection.toString().trim();
      const path = SC.state.ui.currentFile || 'file';
      const input = SC.$('#input');
      let snippet;
      if (selectedText) {
        snippet = `\n\`\`\`${path}\n${selectedText}\n\`\`\`\n`;
      } else {
        const content = SC.$('#drawer-content code').textContent;
        snippet = `\n\`\`\`${path}\n${content}\n\`\`\`\n`;
      }
      input.value += snippet;
      input.style.height = 'auto';
      input.style.height = Math.min(input.scrollHeight, 200) + 'px';
      input.focus();
      SC.closeDrawer();
    });
    SC.$('#drawer-edit').addEventListener('click', () => {
      const content = SC.$('#drawer-content code').textContent;
      SC.openEditor(content, SC.state.ui.currentFile);
      SC.closeDrawer();
    });

    SC.$('#editor-close').addEventListener('click', SC.closeEditor);
    SC.$('#editor-save').addEventListener('click', SC.saveEditor);
    SC.$('#dashboard-close').addEventListener('click', () => SC.$('#dashboard-panel').classList.remove('visible'));
    SC.$('#btn-dashboard').addEventListener('click', SC.openDashboard);
    SC.$('#btn-editor').addEventListener('click', () => {
      SC.openEditor('', SC.state.ui.editorFile || 'untitled.go');
    });

    SC.$('#theme-editor-close').addEventListener('click', () => SC.$('#theme-editor-panel').classList.remove('visible'));
    SC.$('#open-theme-editor').addEventListener('click', () => {
      const panel = SC.$('#theme-editor-panel');
      panel.classList.remove('hidden');
      requestAnimationFrame(() => panel.classList.add('visible'));
    });
    SC.$('#theme-export').addEventListener('click', SC.exportTheme);
    SC.$('#theme-import').addEventListener('click', SC.importTheme);

    SC.$('#theme-select').addEventListener('change', (e) => {
      SC.state.settings.theme = e.target.value;
      SC.saveSettings();
      SC.applySettings();
      if (typeof SC.updateEditorTheme === 'function') SC.updateEditorTheme();
    });
    SC.$('#model-select').addEventListener('change', (e) => {
      SC.state.settings.model = e.target.value;
      SC.$('#current-model').textContent = e.target.value;
      SC.wsSend('model', { model: e.target.value });
      SC.saveSettings();
    });
    SC.$('#font-size').addEventListener('input', (e) => {
      SC.state.settings.fontSize = parseInt(e.target.value);
      SC.saveSettings();
      SC.applySettings();
    });

    SC.loadProviderConfig();

    SC.$('#btn-save-provider').addEventListener('click', SC.saveProviderConfig);

    SC.$('#model-select').addEventListener('change', (e) => {
      const custom = SC.$('#cfg-custom-model');
      if (e.target.value !== '__custom__') {
        if (custom) custom.value = e.target.value;
      }
    });

    SC.$('#btn-voice').addEventListener('click', () => {
      if (SC.state.isRecording) SC.stopVoice();
      else SC.startVoice();
    });
    SC.$('#voice-stop').addEventListener('click', SC.stopVoice);

    SC.$('#btn-stop').addEventListener('click', () => {
      SC.wsSend('abort', {});
      SC.setState('isProcessing', false);
      SC.updateStopBtn();
    });

    SC.$('#refresh-files').addEventListener('click', () => SC.wsSend('file_tree', { path: '.' }));
    SC.$('#new-session').addEventListener('click', () => SC.wsSend('session_new', { model: SC.state.settings.model }));

    let skillSearchTimer = null;
    SC.$('#skill-search')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (skillSearchTimer) clearTimeout(skillSearchTimer);
      if (!query) {
        SC.wsSend('skill_list', {});
        return;
      }
      skillSearchTimer = setTimeout(() => SC.wsSend('skill_search', { query }), 300);
    });

    SC.$('#memory-search')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('memory_search', { query, limit: 5 });
      }
    });

    SC.$$('.memory-tab').forEach(tab => {
      tab.addEventListener('click', () => {
        SC.$$('.memory-tab').forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        SC.$$('.memory-tab-content').forEach(c => c.classList.remove('active'));
        const tabId = tab.dataset.tab;
        const content = SC.$(`#memory-tab-${tabId}`);
        if (content) content.classList.add('active');
        SC.state.memoryTab = tabId;
        if (tabId === 'l3') {
          SC.renderSkillMemoryList();
        } else if (tabId === 'l4') {
          SC.wsSend('memory_observations', {});
        }
      });
    });

    SC.$('#session-fragment-search')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const query = e.target.value.trim();
        if (query) SC.wsSend('session_fragments', { query, limit: 10 });
      }
    });

    SC.$('#refresh-observations')?.addEventListener('click', () => {
      SC.wsSend('memory_observations', {});
    });

    SC.$('#skill-editor-save')?.addEventListener('click', () => {
      if (!SC.state.editingSkill) return;
      const content = SC.$('#skill-editor-content')?.value || '';
      SC.wsSend('skill_edit', { name: SC.state.editingSkill, content });
    });

    SC.$('#skill-editor-cancel')?.addEventListener('click', () => {
      SC.state.editingSkill = null;
      SC.$('#skill-editor')?.classList.add('hidden');
    });

    let wikiSearchTimer = null;
    SC.$('#wiki-search')?.addEventListener('input', (e) => {
      const query = e.target.value.trim();
      if (wikiSearchTimer) clearTimeout(wikiSearchTimer);
      if (!query) {
        SC.wsSend('wiki_pages', {});
        return;
      }
      wikiSearchTimer = setTimeout(() => SC.wsSend('wiki_search', { query, limit: 5 }), 300);
    });

    SC.$('#session-search')?.addEventListener('input', () => {
      SC.renderSessions(SC.state.sessions || []);
    });

    SC.$('#help-close')?.addEventListener('click', SC.hideHelpModal);
    SC.$('#help-modal .modal-backdrop')?.addEventListener('click', SC.hideHelpModal);

    document.addEventListener('keydown', (e) => {
      if (e.ctrlKey || e.metaKey) {
        if (e.key === 'k') { e.preventDefault(); SC.$('#input').focus(); SC.$('#input').value = '/'; SC.$('#input').dispatchEvent(new Event('input')); }
        else if (e.key === 's' && SC.state.ui.editorFile) { e.preventDefault(); SC.saveEditor(); }
        else if (e.key === 'n') { e.preventDefault(); SC.wsSend('session_new', { model: SC.state.settings.model }); }
        else if (e.key === '/') { e.preventDefault(); SC.$('#sidebar').classList.toggle('collapsed'); }
        else if (e.key === 'p') { e.preventDefault(); SC.focusModelSwitcher(); }
        else if (e.key === 'o') { e.preventDefault(); SC.toggleSessionsPanel(); }
        else if (e.key === 'l') { e.preventDefault(); SC.clearChat(); }
        else if (e.key === 'h') { e.preventDefault(); SC.showHelpModal(); }
      }
      if (e.key === 'Escape') {
        SC.closeDrawer();
        SC.closeEditor();
        SC.closeSkillCreateForm();
        SC.$('#dashboard-panel').classList.remove('visible');
        SC.$('#theme-editor-panel').classList.remove('visible');
        SC.$('#cmd-palette').classList.add('hidden');
        SC.$('#file-mention').classList.add('hidden');
        SC.$('#help-modal')?.classList.add('hidden');
        SC.state.mentionStart = -1;
      }
    });

    SC.$('#btn-attach').addEventListener('click', () => {
      const input = document.createElement('input');
      input.type = 'file';
      input.multiple = true;
      input.onchange = (e) => {
        Array.from(e.target.files).forEach(f => {
          const reader = new FileReader();
          reader.onload = (ev) => {
            const text = ev.target.result;
            const isText = typeof text === 'string' && text.length < 50000 && f.size <= 51200;
            if (isText) {
              const snippet = `\n\`\`\`${f.name}\n${text.slice(0, 10000)}${text.length > 10000 ? '\n... (truncated)' : ''}\n\`\`\`\n`;
              const inputEl = SC.$('#input');
              inputEl.value += snippet;
              inputEl.style.height = 'auto';
              inputEl.style.height = Math.min(inputEl.scrollHeight, 200) + 'px';
              SC.toast(`Added ${f.name}`, 'success');
            } else {
              SC.uploadFile(f);
            }
          };
          reader.readAsText(f);
        });
      };
      input.click();
    });
  }

  // Telemetry Dashboard
  (function() {
    function fetchTelemetry() {
      fetch('/api/telemetry')
        .then(r => r.json())
        .then(data => {
          const el = id => document.getElementById(id);
          if (el('t-queries')) el('t-queries').textContent = data.query_count || 0;
          if (el('t-input-tokens')) el('t-input-tokens').textContent = (data.total_input_tokens || 0).toLocaleString();
          if (el('t-output-tokens')) el('t-output-tokens').textContent = (data.total_output_tokens || 0).toLocaleString();
          if (el('t-cost')) el('t-cost').textContent = '$' + (data.estimated_cost_usd || 0).toFixed(4);
          if (el('t-cache-rate')) el('t-cache-rate').textContent = ((data.cache_hit_rate || 0) * 100).toFixed(1) + '%';
          if (el('t-avg-latency')) {
            const qCount = data.query_count || 0;
            const totalMs = data.query_total_time_ms || 0;
            el('t-avg-latency').textContent = qCount > 0 ? Math.round(totalMs / qCount) + 'ms' : '0ms';
          }

          const toolList = el('tool-exec-list');
          if (toolList && data.tool_executions) {
            toolList.innerHTML = '';
            const tools = Object.entries(data.tool_executions).sort((a, b) => b[1].Count - a[1].Count);
            tools.slice(0, 15).forEach(([name, stats]) => {
              const item = document.createElement('div');
              item.className = 'tool-exec-item';
              item.innerHTML = `<span class="tool-name">${name}</span><span class="tool-stats">${stats.Count}x · ${stats.Errors}err · ${Math.round(stats.Duration / 1e6)}ms</span>`;
              toolList.appendChild(item);
            });
          }
        })
        .catch(() => {});
    }

    document.addEventListener('click', e => {
      if (e.target && e.target.id === 'telemetry-refresh') fetchTelemetry();
    });

    let telemetryInterval = null;
    const observer = new MutationObserver(() => {
      const panel = document.getElementById('dashboard-panel');
      if (panel && panel.classList.contains('visible')) {
        if (!telemetryInterval) {
          fetchTelemetry();
          telemetryInterval = setInterval(fetchTelemetry, 5000);
        }
      } else {
        if (telemetryInterval) {
          clearInterval(telemetryInterval);
          telemetryInterval = null;
        }
      }
    });
    observer.observe(document.body, { attributes: true, subtree: true, attributeFilter: ['class'] });
  })();

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
