// SmartClaw - Code Canvas (Split-Screen Editor)
(function() {
  'use strict';

  var cmInstance = null;
  var isCanvasOpen = false;
  var dragState = { onMouseMove: null, onMouseUp: null };
  var canvasObserver = null;

  function getModeForLang(lang) {
    var map = {
      'go': 'go', 'javascript': 'javascript', 'js': 'javascript', 'jsx': 'javascript',
      'typescript': 'javascript', 'ts': 'javascript', 'tsx': 'javascript',
      'python': 'python', 'py': 'python',
      'html': 'htmlmixed', 'htmlmixed': 'htmlmixed',
      'css': 'css', 'yaml': 'yaml', 'yml': 'yaml',
      'shell': 'shell', 'bash': 'shell', 'sh': 'shell',
      'markdown': 'markdown', 'md': 'markdown',
      'xml': 'xml', 'svg': 'xml', 'json': { name: 'javascript', json: true }
    };
    return map[(lang || '').toLowerCase().trim()] || 'javascript';
  }

  function openCanvas(code, lang, filename) {
    var container = SC.$('#canvas-panel');
    if (!container) return;

    container.classList.add('visible');
    isCanvasOpen = true;
    document.body.classList.add('canvas-active');

    var titleEl = SC.$('#canvas-filename');
    if (titleEl) titleEl.textContent = filename || (lang ? lang + ' code' : 'Code');

    var editorEl = SC.$('#canvas-editor');
    if (!editorEl) return;

    if (cmInstance) {
      cmInstance.setValue(code || '');
      cmInstance.setOption('mode', getModeForLang(lang));
    } else if (typeof CodeMirror !== 'undefined') {
      cmInstance = CodeMirror(editorEl, {
        value: code || '',
        mode: getModeForLang(lang),
        theme: 'dracula',
        lineNumbers: true,
        matchBrackets: true,
        autoCloseBrackets: true,
        indentUnit: 2,
        tabSize: 2,
        indentWithTabs: false,
        lineWrapping: false,
        readOnly: false,
        extraKeys: {
          'Ctrl-S': function() { saveCanvasContent(); }
        }
      });
      setTimeout(function() { if (cmInstance) cmInstance.refresh(); }, 100);
    } else {
      editorEl.innerHTML = '<pre style="padding:14px;color:var(--tx-1);font-family:var(--font-d);font-size:13px;white-space:pre-wrap;">' + SC.escapeHtml(code || '') + '</pre>';
    }

    if (SC.audio && SC.audio.click) SC.audio.click();
  }

  function closeCanvas() {
    var container = SC.$('#canvas-panel');
    if (container) container.classList.remove('visible');
    isCanvasOpen = false;
    document.body.classList.remove('canvas-active');
    if (dragState.onMouseMove) {
      document.removeEventListener('mousemove', dragState.onMouseMove);
      dragState.onMouseMove = null;
    }
    if (dragState.onMouseUp) {
      document.removeEventListener('mouseup', dragState.onMouseUp);
      dragState.onMouseUp = null;
    }
    if (canvasObserver) {
      canvasObserver.disconnect();
      canvasObserver = null;
    }
  }

  function saveCanvasContent() {
    if (!cmInstance) return;
    var code = cmInstance.getValue();
    var blob = new Blob([code], { type: 'text/plain' });
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = SC.$('#canvas-filename') ? SC.$('#canvas-filename').textContent : 'code.txt';
    a.click();
    URL.revokeObjectURL(a.href);
    SC.toast('Code saved', 'success');
  }

  function addCanvasButtons() {
    var codeBlocks = SC.$$('#messages pre code');
    codeBlocks.forEach(function(block) {
      if (block.dataset.canvasWired) return;
      block.dataset.canvasWired = 'true';

      var wrapper = block.parentElement ? block.parentElement.parentElement : null;
      if (!wrapper || !wrapper.classList.contains('code-block-wrapper')) return;

      var lang = '';
      var langMatch = block.className.match(/language-(\w+)/);
      if (langMatch) lang = langMatch[1];

      var btn = document.createElement('button');
      btn.className = 'code-action-btn canvas-open-btn';
      btn.innerHTML = '↗ Canvas';
      btn.title = 'Open in code canvas';
      btn.addEventListener('click', function() {
        var filename = lang ? 'untitled.' + lang : 'code';
        openCanvas(block.textContent, lang, filename);
      });

      var actionsBar = wrapper.querySelector('.code-actions');
      if (actionsBar) {
        actionsBar.appendChild(btn);
      } else {
        actionsBar = document.createElement('div');
        actionsBar.className = 'code-actions';
        actionsBar.appendChild(btn);
        wrapper.appendChild(actionsBar);
      }
    });
  }

  function initCanvas() {
    var closeBtn = SC.$('#canvas-close');
    var saveBtn = SC.$('#canvas-save');
    var divider = SC.$('#canvas-divider');

    if (closeBtn) closeBtn.addEventListener('click', closeCanvas);
    if (saveBtn) saveBtn.addEventListener('click', saveCanvasContent);

    if (divider) {
      var dragging = false;
      divider.addEventListener('mousedown', function(e) {
        dragging = true;
        e.preventDefault();
      });
      dragState.onMouseMove = function(e) {
        if (!dragging) return;
        var width = window.innerWidth - e.clientX;
        width = Math.max(300, Math.min(width, window.innerWidth * 0.7));
        var panel = SC.$('#canvas-panel');
        if (panel) panel.style.width = width + 'px';
        if (cmInstance) cmInstance.refresh();
      };
      dragState.onMouseUp = function() {
        dragging = false;
        document.removeEventListener('mousemove', dragState.onMouseMove);
        document.removeEventListener('mouseup', dragState.onMouseUp);
      };
      divider.addEventListener('mousedown', function(e) {
        dragging = true;
        e.preventDefault();
        document.addEventListener('mousemove', dragState.onMouseMove);
        document.addEventListener('mouseup', dragState.onMouseUp);
      });
    }

    canvasObserver = new MutationObserver(function() {
      setTimeout(addCanvasButtons, 150);
    });
    var messages = SC.$('#messages');
    if (messages) canvasObserver.observe(messages, { childList: true, subtree: true });

    addCanvasButtons();
  }

  SC.openCanvas = openCanvas;
  SC.closeCanvas = closeCanvas;
  SC.initCanvas = initCanvas;
  SC.addCanvasButtons = addCanvasButtons;
})();
