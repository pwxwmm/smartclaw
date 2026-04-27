// SmartClaw - Files
(function() {
  'use strict';

  let dirCounter = 0;
  let gitStatusMap = {};
  let searchQuery = '';

  SC.fileIconMap = {
    go:   { color: '#00ADD8', label: 'Go' },
    js:   { color: '#F7DF1E', label: 'JS' },
    mjs:  { color: '#F7DF1E', label: 'JS' },
    cjs:  { color: '#F7DF1E', label: 'JS' },
    jsx:  { color: '#61DAFB', label: 'JSX' },
    ts:   { color: '#3178C6', label: 'TS' },
    tsx:  { color: '#3178C6', label: 'TSX' },
    py:   { color: '#3776AB', label: 'Py' },
    pyw:  { color: '#3776AB', label: 'Py' },
    md:   { color: '#E8E8EC', label: 'Md' },
    yaml: { color: '#CB171E', label: 'Ym' },
    yml:  { color: '#CB171E', label: 'Ym' },
    json: { color: '#5B9A4C', label: '{}' },
    html: { color: '#E34F26', label: 'Ht' },
    htm:  { color: '#E34F26', label: 'Ht' },
    css:  { color: '#A855F7', label: 'Cs' },
    scss: { color: '#A855F7', label: 'Sc' },
    less: { color: '#A855F7', label: 'Le' },
    rs:   { color: '#DEA584', label: 'Rs' },
    java: { color: '#ED8B00', label: 'Jv' },
    sh:   { color: '#4EAA25', label: 'Sh' },
    bash: { color: '#4EAA25', label: 'Sh' },
    zsh:  { color: '#4EAA25', label: 'Sh' },
    dockerfile: { color: '#2496ED', label: 'Dk' },
    mod:  { color: '#00ADD8', label: 'Md' },
    sum:  { color: '#00ADD8', label: 'Sm' },
    sql:  { color: '#E38C00', label: 'Db' },
    rb:   { color: '#CC342D', label: 'Rb' },
    c:    { color: '#A8B9CC', label: 'C' },
    h:    { color: '#A8B9CC', label: 'H' },
    cpp:  { color: '#00599C', label: 'C+' },
    toml: { color: '#9C4221', label: 'Tm' },
    xml:  { color: '#F26522', label: 'Xm' },
  };

  function getFileIcon(name) {
    const lower = name.toLowerCase();
    const basename = lower.split('/').pop();
    if (basename === 'dockerfile' || basename.startsWith('dockerfile.')) {
      return SC.fileIconMap.dockerfile;
    }
    if (basename === 'makefile' || basename === 'gnumakefile') {
      return { color: '#6D8086', label: 'Mk' };
    }
    if (basename === '.gitignore' || basename === '.dockerignore') {
      return { color: '#F54D27', label: 'Gi' };
    }
    if (basename === 'go.mod') return SC.fileIconMap.mod;
    if (basename === 'go.sum') return SC.fileIconMap.sum;
    const dotIdx = basename.lastIndexOf('.');
    if (dotIdx > 0) {
      const ext = basename.slice(dotIdx + 1);
      if (SC.fileIconMap[ext]) return SC.fileIconMap[ext];
    }
    return null;
  }

  function fileIconSvg(name) {
    const icon = getFileIcon(name);
    if (!icon) {
      return '<svg class="ficon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
    }
    return '<svg class="ficon file-type-icon" width="14" height="14" viewBox="0 0 24 24" fill="none"><rect x="4" y="2" width="16" height="20" rx="2" fill="' + icon.color + '" opacity="0.15" stroke="' + icon.color + '" stroke-width="1.2"/><text x="12" y="15" text-anchor="middle" fill="' + icon.color + '" font-size="7" font-weight="700" font-family="JetBrains Mono,monospace">' + icon.label + '</text></svg>';
  }

  function gitStatusDot(path) {
    const code = gitStatusMap[path];
    if (!code) return '';
    let color = '';
    if (code === '??' || code.startsWith('A')) color = 'var(--ok)';
    else if (code.includes('M')) color = 'var(--warn)';
    else if (code.includes('D')) color = 'var(--err)';
    else color = 'var(--info)';
    return '<span class="git-dot" style="background:' + color + '"></span>';
  }

  function highlightName(name, query) {
    if (!query) return SC.escapeHtml(name);
    const escaped = SC.escapeHtml(name);
    const q = SC.escapeHtml(query);
    const regex = new RegExp('(' + q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
    return escaped.replace(regex, '<mark>$1</mark>');
  }

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

  function matchesSearch(node, query, prefix) {
    if (!query) return true;
    const path = prefix ? prefix + '/' + node.name : node.name;
    const q = query.toLowerCase();
    if (node.name.toLowerCase().includes(q)) return true;
    if (node.type === 'dir' && node.children) {
      return node.children.some(child => matchesSearch(child, query, path));
    }
    return false;
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
    const prefix = parent ? getNodePath(parent) : '';
    const filteredNodes = searchQuery
      ? nodes.filter(node => matchesSearch(node, searchQuery, prefix))
      : nodes;

    filteredNodes.forEach(node => {
      const el = document.createElement('div');
      el.className = `file-node ${node.type === 'dir' ? 'dir' : ''}`;
      const path = prefix ? prefix + '/' + node.name : node.name;

      if (node.type === 'dir') {
        const iconSvg = '<svg class="ficon folder" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>';
        el.innerHTML = `${iconSvg}<span class="fname">${highlightName(node.name, searchQuery)}</span>`;
        const dirId = 'dir-' + (++dirCounter);
        el.dataset.dirId = dirId;
        const shouldExpand = searchQuery && matchesSearch(node, searchQuery, path);
        if (shouldExpand) {
          el.dataset.collapsed = 'false';
        }
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
          if (shouldExpand) {
            childContainer.style.display = '';
          }
          container.appendChild(childContainer);
        }
      } else {
        const iconSvg = fileIconSvg(node.name);
        const dot = gitStatusDot(path);
        el.innerHTML = `${iconSvg}<span class="fname">${highlightName(node.name, searchQuery)}</span>${dot}`;
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          SC.state.ui.currentFile = getNodePath(el);
          SC.wsSend('file_open', { path: SC.state.ui.currentFile });
        });
        el.draggable = true;
        el.addEventListener('dragstart', (e) => {
          SC.state.ui.currentFile = getNodePath(el);
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

  function initFileSearch() {
    const searchInput = SC.$('#file-search');
    const clearBtn = SC.$('#file-search-clear');
    if (!searchInput) return;

    searchInput.addEventListener('input', () => {
      searchQuery = searchInput.value.trim();
      clearBtn.classList.toggle('hidden', !searchQuery);
      dirCounter = 0;
      const tree = SC.state.fileTreeData || [];
      renderFileTree(tree);
    });

    clearBtn.addEventListener('click', () => {
      searchInput.value = '';
      searchQuery = '';
      clearBtn.classList.add('hidden');
      dirCounter = 0;
      const tree = SC.state.fileTreeData || [];
      renderFileTree(tree);
      searchInput.focus();
    });
  }

  function requestGitStatus() {
    SC.wsSend('git_status', {});
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
  SC.initFileSearch = initFileSearch;
  SC.requestGitStatus = requestGitStatus;

  SC.state.fileTreeData = [];
  SC.state.gitStatus = {};
})();
