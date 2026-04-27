// SmartClaw - Code Editor (CodeMirror 5)
(function() {
  'use strict';

  var cmInstance = null;
  var currentFile = '';

  // Mode mapping: file extension → CodeMirror mode
  var modeMap = {
    '.go': 'go',
    '.js': 'javascript', '.jsx': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript',
    '.ts': 'javascript', '.tsx': 'javascript',
    '.py': 'python', '.pyw': 'python',
    '.md': 'markdown', '.markdown': 'markdown',
    '.yaml': 'yaml', '.yml': 'yaml',
    '.json': { name: 'javascript', json: true },
    '.html': 'htmlmixed', '.htm': 'htmlmixed',
    '.css': 'css', '.scss': 'css', '.less': 'css',
    '.sh': 'shell', '.bash': 'shell', '.zsh': 'shell',
    '.rs': 'rust',
    '.sql': 'sql',
    '.xml': 'xml',
    '.toml': 'toml',
    '.c': 'text/x-csrc', '.h': 'text/x-csrc',
    '.cpp': 'text/x-c++src', '.hpp': 'text/x-c++src', '.cc': 'text/x-c++src',
    '.java': 'text/x-java',
    '.rb': 'ruby',
  };

  function getModeForFile(filename) {
    if (!filename) return 'javascript';
    var lower = filename.toLowerCase();
    var basename = lower.split('/').pop();
    // Special filenames
    if (basename === 'dockerfile' || basename.startsWith('dockerfile.')) return 'shell';
    if (basename === 'makefile' || basename === 'gnumakefile') return 'shell';
    if (basename === 'go.mod' || basename === 'go.sum') return 'go';
    var ext = '.' + basename.split('.').pop();
    return modeMap[ext] || 'javascript';
  }

  function initEditor() {
    var textarea = document.getElementById('editor-content');
    if (!textarea) return;
    if (typeof CodeMirror === 'undefined') return;

    // Get current theme
    var theme = document.documentElement.dataset.theme || 'dark';
    var cmTheme = (theme === 'light') ? 'default' : 'dracula';

    cmInstance = CodeMirror.fromTextArea(textarea, {
      lineNumbers: true,
      mode: 'javascript',
      theme: cmTheme,
      indentUnit: 4,
      tabSize: 4,
      indentWithTabs: false,
      lineWrapping: false,
      matchBrackets: true,
      autoCloseBrackets: true,
      scrollbarStyle: 'native',
      extraKeys: {
        'Ctrl-S': function() { SC.saveEditor(); },
        'Cmd-S': function() { SC.saveEditor(); },
      }
    });

    cmInstance.setSize('100%', '100%');
  }

  function openInEditor(content, filename) {
    if (!cmInstance) {
      initEditor();
    }
    if (!cmInstance) {
      // Fallback: use plain textarea
      var textarea = document.getElementById('editor-content');
      if (textarea) textarea.value = content || '';
      return;
    }

    currentFile = filename || '';
    var mode = getModeForFile(filename);
    cmInstance.setOption('mode', mode);
    cmInstance.setValue(content || '');
    cmInstance.clearHistory();
    // Refresh after a frame to ensure proper rendering
    requestAnimationFrame(function() {
      cmInstance.refresh();
      cmInstance.focus();
    });
  }

  function getEditorContent() {
    if (cmInstance) return cmInstance.getValue();
    var textarea = document.getElementById('editor-content');
    return textarea ? textarea.value : '';
  }

  function updateEditorTheme() {
    if (!cmInstance) return;
    var theme = document.documentElement.dataset.theme || 'dark';
    cmInstance.setOption('theme', (theme === 'light') ? 'default' : 'dracula');
  }

  function refreshEditor() {
    if (cmInstance) cmInstance.refresh();
  }

  SC.initEditor = initEditor;
  SC.openInEditor = openInEditor;
  SC.getEditorContent = getEditorContent;
  SC.updateEditorTheme = updateEditorTheme;
  SC.refreshEditor = refreshEditor;
})();
