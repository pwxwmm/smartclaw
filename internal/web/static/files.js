// SmartClaw - Files
(function() {
  'use strict';

  let dirCounter = 0;

  function flattenFileTree(nodes, prefix) {
    let result = [];
    nodes.forEach(node => {
      const path = prefix ? prefix + '/' + node.name : node.name;
      if (node.type === 'dir') {
        if (node.children) result = result.concat(flattenFileTree(node.children, path));
      } else {
        result.push({ name: node.name, path: path });
      }
    });
    return result;
  }

  function renderFileTree(nodes, parent) {
    const container = parent || SC.$('#file-tree');
    container.innerHTML = '';
    if (!parent && nodes.length === 0) {
      SC.showEmptyState(container,
        '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>',
        'No files loaded',
        'Open a project directory to see its files here.'
      );
      return;
    }
    nodes.forEach(node => {
      const el = document.createElement('div');
      el.className = `file-node ${node.type === 'dir' ? 'dir' : ''}`;
      const iconSvg = node.type === 'dir'
        ? '<svg class="ficon folder" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>'
        : '<svg class="ficon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
      el.innerHTML = `${iconSvg}<span class="fname">${node.name}</span>`;
      if (node.type === 'dir') {
        const dirId = 'dir-' + (++dirCounter);
        el.dataset.dirId = dirId;
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          const children = container.querySelector(`[data-dir-children="${dirId}"]`);
          if (!children) return;
          const collapsed = el.dataset.collapsed === 'true';
          el.dataset.collapsed = collapsed ? 'false' : 'true';
          children.style.display = collapsed ? '' : 'none';
          el.querySelector('.folder').style.transform = collapsed ? '' : 'rotate(-90deg)';
        });
        container.appendChild(el);
        if (node.children && node.children.length > 0) {
          const childContainer = document.createElement('div');
          childContainer.className = 'file-children';
          childContainer.dataset.dirChildren = dirId;
          renderFileTree(node.children, childContainer);
          container.appendChild(childContainer);
        }
      } else {
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          SC.state.ui.currentFile = getNodePath(el);
          SC.wsSend('file_open', { path: SC.state.ui.currentFile });
        });
        el.draggable = true;
        el.addEventListener('dragstart', (e) => {
          e.dataTransfer.setData('text/plain', SC.state.ui.currentFile);
        });
        container.appendChild(el);
      }
    });
  }

  function getNodePath(el) {
    const parts = [];
    let node = el;
    while (node && node.id !== 'file-tree') {
      if (node.classList.contains('file-node')) {
        const name = node.querySelector('.fname')?.textContent;
        if (name) parts.unshift(name);
      } else if (node.classList.contains('file-children')) {
        const dirNode = node.previousElementSibling;
        if (dirNode && dirNode.classList.contains('file-node')) {
          const name = dirNode.querySelector('.fname')?.textContent;
          if (name) parts.unshift(name);
        }
      }
      node = node.parentElement;
    }
    return parts.join('/');
  }

  const extToLang = {
    go: 'go', js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
    ts: 'typescript', tsx: 'typescript', py: 'python', pyw: 'python',
    sh: 'bash', bash: 'bash', zsh: 'bash', json: 'json', yaml: 'yaml', yml: 'yaml',
    html: 'html', htm: 'html', css: 'css', scss: 'css', less: 'css',
    md: 'markdown', sql: 'sql', java: 'java', rs: 'rust', rb: 'ruby',
    c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
    xml: 'xml', toml: 'toml', ini: 'ini', cfg: 'ini',
    dockerfile: 'dockerfile', makefile: 'makefile',
  };

  function langFromPath(path) {
    if (!path) return null;
    const lower = path.toLowerCase();
    const basename = lower.split('/').pop();
    if (basename === 'dockerfile' || basename === 'dockerfile.') return 'dockerfile';
    if (basename === 'makefile' || basename === 'gnumakefile') return 'makefile';
    if (basename === '.gitignore' || basename === '.dockerignore') return 'bash';
    const ext = basename.split('.').pop();
    return extToLang[ext] || null;
  }

  function openFileDrawer(content, path) {
    try {
    const drawer = SC.$('#file-drawer');
    SC.$('#drawer-title').textContent = path || 'File Preview';
    const lines = content.split('\n');
    const lineCount = lines.length;
    const padWidth = String(lineCount).length;
    const lineNums = lines.map((_, i) => String(i + 1).padStart(padWidth, ' ')).join('\n');
    const container = SC.$('#drawer-content');
    container.innerHTML = '';
    const lineNumsEl = document.createElement('span');
    lineNumsEl.className = 'line-nums';
    lineNumsEl.textContent = lineNums;
    const codeEl = document.createElement('code');
    const lang = langFromPath(path);
    if (lang && typeof hljs !== 'undefined' && hljs.getLanguage(lang)) {
      try {
        codeEl.innerHTML = hljs.highlight(content, { language: lang }).value;
      } catch (e) {
        codeEl.textContent = content;
      }
    } else if (typeof hljs !== 'undefined') {
      try {
        codeEl.innerHTML = hljs.highlightAuto(content).value;
      } catch (e) {
        codeEl.textContent = content;
      }
    } else {
      codeEl.textContent = content;
    }
    container.appendChild(lineNumsEl);
    container.appendChild(codeEl);
    drawer.classList.remove('hidden');
    requestAnimationFrame(() => drawer.classList.add('visible'));
    } catch (err) {
      console.error('[openFileDrawer Error]', err);
      SC.showErrorBanner('File preview error: ' + err.message, function() { openFileDrawer(content, path); });
    }
  }

  function closeDrawer() {
    const drawer = SC.$('#file-drawer');
    drawer.classList.remove('visible');
    setTimeout(() => drawer.classList.add('hidden'), 340);
  }

  function openEditor(content, path) {
    SC.state.ui.editorFile = path;
    SC.$('#editor-title').textContent = path || 'New File';
    SC.$('#editor-content').value = content || '';
    const panel = SC.$('#editor-panel');
    panel.classList.remove('hidden');
    requestAnimationFrame(() => panel.classList.add('visible'));
    SC.$('#editor-content').focus();
  }

  function closeEditor() {
    const panel = SC.$('#editor-panel');
    panel.classList.remove('visible');
    setTimeout(() => panel.classList.add('hidden'), 220);
  }

  function saveEditor() {
    if (!SC.state.ui.editorFile) return;
    SC.wsSend('file_save', { path: SC.state.ui.editorFile, content: SC.$('#editor-content').value });
    SC.toast('File saved', 'success');
  }

  function initDragDrop() {
    const chat = SC.$('#chat');
    const overlay = SC.$('#drag-overlay');
    let dragCount = 0;

    chat.addEventListener('dragenter', (e) => { e.preventDefault(); dragCount++; overlay.classList.remove('hidden'); });
    chat.addEventListener('dragleave', (e) => { e.preventDefault(); dragCount--; if (dragCount <= 0) { overlay.classList.add('hidden'); dragCount = 0; } });
    chat.addEventListener('dragover', (e) => e.preventDefault());
    chat.addEventListener('drop', (e) => {
      e.preventDefault();
      dragCount = 0;
      overlay.classList.add('hidden');
      const files = e.dataTransfer.files;
      if (files.length > 0) {
        Array.from(files).forEach(f => {
          if (f.size > 51200) {
            SC.uploadFile(f);
            return;
          }
          const reader = new FileReader();
          reader.onload = (ev) => {
            const text = ev.target.result;
            const isText = typeof text === 'string' && text.length < 50000;
            if (isText) {
              const snippet = `\n\`\`\`${f.name}\n${text.slice(0, 10000)}${text.length > 10000 ? '\n... (truncated)' : ''}\n\`\`\`\n`;
              const input = SC.$('#input');
              input.value += snippet;
              input.style.height = 'auto';
              input.style.height = Math.min(input.scrollHeight, 200) + 'px';
              SC.toast(`Added ${f.name}`, 'success');
            } else {
              SC.uploadFile(f);
            }
          };
          reader.readAsText(f);
        });
      }
    });
  }

  function initFileMention() {
    const input = SC.$('#input');
    const mention = SC.$('#file-mention');
    const mentionList = SC.$('#file-mention-list');

    input.addEventListener('input', () => {
      const val = input.value;
      const cursorPos = input.selectionStart;
      const atIdx = val.lastIndexOf('@', cursorPos - 1);
      if (atIdx === -1 || (atIdx > 0 && val[atIdx - 1] !== ' ' && val[atIdx - 1] !== '\n')) {
        mention.classList.add('hidden');
        SC.state.mentionStart = -1;
        return;
      }
      const query = val.slice(atIdx + 1, cursorPos).toLowerCase();
      const filtered = SC.state.flatFiles.filter(f => f.path.toLowerCase().includes(query)).slice(0, 20);
      if (filtered.length === 0) {
        mention.classList.add('hidden');
        SC.state.mentionStart = -1;
        return;
      }
      SC.state.mentionStart = atIdx;
      SC.state.mentionIndex = -1;
      mentionList.innerHTML = '';
      filtered.forEach((f, i) => {
        const li = document.createElement('li');
        li.className = 'file-mention-item';
        li.dataset.index = i;
        const lastSlash = f.path.lastIndexOf('/');
        const dir = lastSlash > 0 ? f.path.slice(0, lastSlash + 1) : '';
        const name = lastSlash > 0 ? f.path.slice(lastSlash + 1) : f.path;
        li.innerHTML = `<svg class="fm-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg><span class="fm-path">${dir}<span class="fm-name">${name}</span></span>`;
        li.addEventListener('click', () => insertMention(f.path));
        li.addEventListener('mouseenter', () => {
          SC.state.mentionIndex = i;
          updateMentionSelection();
        });
        mentionList.appendChild(li);
      });
      mention.classList.remove('hidden');
    });

    input.addEventListener('keydown', (e) => {
      if (mention.classList.contains('hidden') || SC.state.mentionStart === -1) return;
      const items = SC.$$('.file-mention-item', mentionList);
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        SC.state.mentionIndex = Math.min(SC.state.mentionIndex + 1, items.length - 1);
        updateMentionSelection();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        SC.state.mentionIndex = Math.max(SC.state.mentionIndex - 1, 0);
        updateMentionSelection();
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        e.stopPropagation();
        if (SC.state.mentionIndex >= 0 && items[SC.state.mentionIndex]) {
          items[SC.state.mentionIndex].click();
        }
      } else if (e.key === 'Escape') {
        mention.classList.add('hidden');
        SC.state.mentionStart = -1;
      }
    }, true);

    function updateMentionSelection() {
      const items = SC.$$('.file-mention-item', mentionList);
      items.forEach((el, i) => el.classList.toggle('sel', i === SC.state.mentionIndex));
      if (items[SC.state.mentionIndex]) items[SC.state.mentionIndex].scrollIntoView({ block: 'nearest' });
    }

    function insertMention(path) {
      const before = input.value.slice(0, SC.state.mentionStart);
      const after = input.value.slice(input.selectionStart);
      input.value = before + '@' + path + ' ' + after;
      const newPos = SC.state.mentionStart + path.length + 2;
      input.selectionStart = input.selectionEnd = newPos;
      mention.classList.add('hidden');
      SC.state.mentionStart = -1;
      input.focus();
    }
  }

  SC.flattenFileTree = flattenFileTree;
  SC.renderFileTree = renderFileTree;
  SC.openFileDrawer = openFileDrawer;
  SC.closeDrawer = closeDrawer;
  SC.openEditor = openEditor;
  SC.closeEditor = closeEditor;
  SC.saveEditor = saveEditor;
  SC.initDragDrop = initDragDrop;
  SC.initFileMention = initFileMention;
})();
