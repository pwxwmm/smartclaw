// SmartClaw - Prompt Template Library
(function() {
  'use strict';

  var CATEGORY_COLORS = {
    Development: '#3b82f6',
    Data: '#06b6d4',
    AI: '#8b5cf6',
    Operations: '#f59e0b',
    Productivity: '#10b981'
  };

  var PRESET_TEMPLATES = [
    {
      id: 'preset-code-review',
      name: 'Code Review',
      description: 'Review code for bugs, style issues, and improvements',
      category: 'Development',
      content: 'Review the following {{language}} code from {{file}} for bugs, style issues, security vulnerabilities, and suggest improvements:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-bug-fix',
      name: 'Bug Fix',
      description: 'Analyze and fix a bug in the selected code',
      category: 'Development',
      content: 'Analyze the following {{language}} code from {{file}} and identify the bug. Provide a fix with explanation:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-doc-gen',
      name: 'Doc Generation',
      description: 'Generate documentation for the selected code',
      category: 'Productivity',
      content: 'Generate comprehensive documentation for the following {{language}} code from {{file}}:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-test-writing',
      name: 'Test Writing',
      description: 'Write unit tests for the selected code',
      category: 'Development',
      content: 'Write comprehensive unit tests for the following {{language}} code from {{file}}. Include edge cases:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-refactor',
      name: 'Refactor',
      description: 'Refactor code for better readability and performance',
      category: 'Development',
      content: 'Refactor the following {{language}} code from {{file}} for better readability, maintainability, and performance. Explain each change:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-explain-code',
      name: 'Explain Code',
      description: 'Explain what the selected code does',
      category: 'AI',
      content: 'Explain what the following {{language}} code from {{file}} does, step by step:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-security-audit',
      name: 'Security Audit',
      description: 'Audit code for security vulnerabilities',
      category: 'Operations',
      content: 'Perform a security audit on the following {{language}} code from {{file}}. Identify vulnerabilities, CVEs, and provide remediation:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-perf-review',
      name: 'Performance Review',
      description: 'Review code for performance issues',
      category: 'Operations',
      content: 'Review the following {{language}} code from {{file}} for performance issues. Identify bottlenecks and suggest optimizations:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-git-commit',
      name: 'Git Commit Message',
      description: 'Generate a conventional commit message',
      category: 'Productivity',
      content: 'Generate a conventional commit message for the following changes in {{file}}:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    },
    {
      id: 'preset-readme-gen',
      name: 'README Generator',
      description: 'Generate a README for the project or file',
      category: 'Productivity',
      content: 'Generate a README.md for {{file}}. Include description, usage, installation, and examples:\n\n{{selection}}',
      variables: ['file', 'selection', 'language'],
      isPreset: true
    }
  ];

  var allTemplates = [];
  var activeCategory = 'All';
  var pickerEl = null;

  function init() {
    SC.state.customTemplates = [];
    allTemplates = PRESET_TEMPLATES.slice();
    loadCachedTemplates();
    SC.wsSend('template_list', {});
  }

  function loadCachedTemplates() {
    try {
      var cached = localStorage.getItem('smartclaw-templates');
      if (cached) {
        SC.state.customTemplates = JSON.parse(cached);
        mergeTemplates();
      }
    } catch (e) {}
  }

  function cacheTemplates() {
    try {
      localStorage.setItem('smartclaw-templates', JSON.stringify(SC.state.customTemplates || []));
    } catch (e) {}
  }

  function mergeTemplates() {
    var custom = SC.state.customTemplates || [];
    allTemplates = PRESET_TEMPLATES.slice();
    for (var i = 0; i < custom.length; i++) {
      if (!custom[i].isPreset) {
        allTemplates.push(custom[i]);
      }
    }
  }

  function getLanguageFromFile(filePath) {
    if (!filePath) return 'text';
    var ext = filePath.split('.').pop().toLowerCase();
    var map = {
      js: 'javascript', ts: 'typescript', tsx: 'typescript', jsx: 'javascript',
      py: 'python', rb: 'ruby', go: 'go', rs: 'rust', java: 'java',
      kt: 'kotlin', cs: 'csharp', cpp: 'cpp', c: 'c', h: 'c',
      html: 'html', css: 'css', scss: 'scss', json: 'json', yaml: 'yaml',
      yml: 'yaml', md: 'markdown', sql: 'sql', sh: 'shell', bash: 'shell',
      php: 'php', swift: 'swift', dart: 'dart', lua: 'lua', r: 'r',
      zig: 'zig', nim: 'nim', ex: 'elixir', exs: 'elixir'
    };
    return map[ext] || ext;
  }

  function fillVariables(content, variables) {
    var result = content;
    var filePath = SC.state.ui.currentFile || '';
    var selection = '';
    try {
      selection = window.getSelection().toString().trim();
    } catch (e) {}
    var language = getLanguageFromFile(filePath);

    var varMap = {
      file: filePath,
      selection: selection,
      language: language
    };

    for (var i = 0; i < variables.length; i++) {
      var v = variables[i];
      var val = varMap[v] !== undefined ? varMap[v] : '';
      result = result.replace(new RegExp('\\{\\{' + v + '\\}\\}', 'g'), val);
    }
    return result;
  }

  function showPicker() {
    if (pickerEl) { pickerEl.remove(); pickerEl = null; }
    renderPicker();
  }

  function hidePicker() {
    if (pickerEl) {
      pickerEl.classList.remove('visible');
      setTimeout(function() { if (pickerEl) { pickerEl.remove(); pickerEl = null; } }, 200);
    }
  }

  function renderPicker() {
    pickerEl = document.createElement('div');
    pickerEl.className = 'template-picker';
    pickerEl.innerHTML =
      '<div class="template-picker-inner">' +
        '<div class="template-picker-head">' +
          '<span class="template-picker-title">Template Library</span>' +
          '<button class="template-picker-close" id="tpl-picker-close">&times;</button>' +
        '</div>' +
        '<div class="template-picker-search">' +
          '<input type="text" id="tpl-search" class="tpl-search-input" placeholder="Search templates..." autocomplete="off">' +
        '</div>' +
        '<div class="template-picker-tabs" id="tpl-category-tabs">' +
          '<button class="tpl-cat-tab active" data-cat="All">All</button>' +
          '<button class="tpl-cat-tab" data-cat="Development">Dev</button>' +
          '<button class="tpl-cat-tab" data-cat="Data">Data</button>' +
          '<button class="tpl-cat-tab" data-cat="AI">AI</button>' +
          '<button class="tpl-cat-tab" data-cat="Operations">Ops</button>' +
          '<button class="tpl-cat-tab" data-cat="Productivity">Prod</button>' +
        '</div>' +
        '<div class="template-picker-grid" id="tpl-grid"></div>' +
        '<div class="template-picker-footer">' +
          '<button class="tpl-create-btn" id="tpl-create-btn">+ Custom Template</button>' +
        '</div>' +
      '</div>';

    document.body.appendChild(pickerEl);
    requestAnimationFrame(function() { pickerEl.classList.add('visible'); });

    pickerEl.querySelector('#tpl-picker-close').addEventListener('click', hidePicker);
    pickerEl.addEventListener('click', function(e) {
      if (e.target === pickerEl) hidePicker();
    });

    var searchInput = pickerEl.querySelector('#tpl-search');
    searchInput.addEventListener('input', function() { renderGrid(); });
    searchInput.focus();

    var tabs = pickerEl.querySelectorAll('.tpl-cat-tab');
    for (var t = 0; t < tabs.length; t++) {
      tabs[t].addEventListener('click', function() {
        var allTabs = pickerEl.querySelectorAll('.tpl-cat-tab');
        for (var a = 0; a < allTabs.length; a++) allTabs[a].classList.remove('active');
        this.classList.add('active');
        activeCategory = this.dataset.cat;
        renderGrid();
      });
    }

    pickerEl.querySelector('#tpl-create-btn').addEventListener('click', showCreateForm);

    renderGrid();
  }

  function renderGrid() {
    var grid = pickerEl ? pickerEl.querySelector('#tpl-grid') : null;
    if (!grid) return;
    grid.innerHTML = '';

    var search = (pickerEl.querySelector('#tpl-search') || {}).value || '';
    search = search.toLowerCase().trim();

    var filtered = allTemplates.filter(function(tpl) {
      if (activeCategory !== 'All' && tpl.category !== activeCategory) return false;
      if (search && tpl.name.toLowerCase().indexOf(search) === -1 && tpl.description.toLowerCase().indexOf(search) === -1) return false;
      return true;
    });

    if (filtered.length === 0) {
      grid.innerHTML = '<div class="tpl-empty">No templates found</div>';
      return;
    }

    for (var i = 0; i < filtered.length; i++) {
      var tpl = filtered[i];
      var catColor = CATEGORY_COLORS[tpl.category] || '#3b82f6';
      var card = document.createElement('div');
      card.className = 'template-card';
      card.innerHTML =
        '<div class="template-card-head">' +
          '<span class="template-card-name">' + SC.escapeHtml(tpl.name) + '</span>' +
          '<span class="template-card-category" style="background:' + catColor + '22;color:' + catColor + ';border:1px solid ' + catColor + '44">' + SC.escapeHtml(tpl.category) + '</span>' +
        '</div>' +
        '<div class="template-card-desc">' + SC.escapeHtml(tpl.description) + '</div>' +
        (tpl.isPreset ? '' : '<button class="template-card-del" data-id="' + SC.escapeHtml(tpl.id) + '" title="Delete">&times;</button>');
      card.dataset.tplId = tpl.id;
      card.addEventListener('click', function(e) {
        if (e.target.classList.contains('template-card-del')) return;
        var id = this.dataset.tplId;
        selectTemplate(id);
      });
      var delBtn = card.querySelector('.template-card-del');
      if (delBtn) {
        delBtn.addEventListener('click', function(e) {
          e.stopPropagation();
          SC.wsSend('template_delete', { id: this.dataset.id });
        });
      }
      grid.appendChild(card);
    }
  }

  function selectTemplate(id) {
    var tpl = null;
    for (var i = 0; i < allTemplates.length; i++) {
      if (allTemplates[i].id === id) { tpl = allTemplates[i]; break; }
    }
    if (!tpl) return;

    if (tpl.variables && tpl.variables.length > 0) {
      showVariableForm(tpl);
    } else {
      var input = SC.$('#input');
      input.value = fillVariables(tpl.content, []);
      input.style.height = 'auto';
      input.style.height = Math.min(input.scrollHeight, 180) + 'px';
      input.focus();
      hidePicker();
    }
  }

  function showVariableForm(tpl) {
    var grid = pickerEl ? pickerEl.querySelector('#tpl-grid') : null;
    if (!grid) return;

    var html = '<div class="template-var-form">' +
      '<div class="template-var-title">' + SC.escapeHtml(tpl.name) + '</div>' +
      '<div class="template-var-desc">' + SC.escapeHtml(tpl.description) + '</div>';

    var autoVals = {
      file: SC.state.ui.currentFile || '',
      selection: '',
      language: getLanguageFromFile(SC.state.ui.currentFile || '')
    };

    for (var i = 0; i < tpl.variables.length; i++) {
      var v = tpl.variables[i];
      var prefill = autoVals[v] !== undefined ? autoVals[v] : '';
      html += '<label class="template-var-label">' + SC.escapeHtml(v) + '</label>' +
        '<input type="text" class="template-var-input" data-var="' + SC.escapeHtml(v) + '" value="' + SC.escapeHtml(prefill) + '" placeholder="' + SC.escapeHtml(v) + '">';
    }

    html += '<div class="template-var-actions">' +
      '<button class="tpl-var-cancel" id="tpl-var-cancel">Back</button>' +
      '<button class="tpl-var-insert" id="tpl-var-insert">Insert</button>' +
    '</div></div>';

    grid.innerHTML = html;

    var firstInput = grid.querySelector('.template-var-input');
    if (firstInput) firstInput.focus();

    grid.querySelector('#tpl-var-cancel').addEventListener('click', function() { renderGrid(); });
    grid.querySelector('#tpl-var-insert').addEventListener('click', function() {
      var vars = {};
      var inputs = grid.querySelectorAll('.template-var-input');
      for (var j = 0; j < inputs.length; j++) {
        vars[inputs[j].dataset.var] = inputs[j].value;
      }
      var content = tpl.content;
      for (var key in vars) {
        content = content.replace(new RegExp('\\{\\{' + key + '\\}\\}', 'g'), vars[key]);
      }
      var input = SC.$('#input');
      input.value = content;
      input.style.height = 'auto';
      input.style.height = Math.min(input.scrollHeight, 180) + 'px';
      input.focus();
      hidePicker();
    });
  }

  function showCreateForm() {
    var grid = pickerEl ? pickerEl.querySelector('#tpl-grid') : null;
    if (!grid) return;

    grid.innerHTML =
      '<div class="template-var-form">' +
        '<div class="template-var-title">Create Custom Template</div>' +
        '<label class="template-var-label">Name</label>' +
        '<input type="text" class="template-var-input" id="tpl-new-name" placeholder="My Template">' +
        '<label class="template-var-label">Description</label>' +
        '<input type="text" class="template-var-input" id="tpl-new-desc" placeholder="What this template does">' +
        '<label class="template-var-label">Category</label>' +
        '<select class="template-var-input" id="tpl-new-cat">' +
          '<option value="Development">Development</option>' +
          '<option value="Data">Data</option>' +
          '<option value="AI">AI</option>' +
          '<option value="Operations">Operations</option>' +
          '<option value="Productivity">Productivity</option>' +
        '</select>' +
        '<label class="template-var-label">Content (use {{file}}, {{selection}}, {{language}} for variables)</label>' +
        '<textarea class="template-var-textarea" id="tpl-new-content" rows="6" placeholder="Review {{language}} code from {{file}}:\n\n{{selection}}"></textarea>' +
        '<div class="template-var-actions">' +
          '<button class="tpl-var-cancel" id="tpl-create-cancel">Back</button>' +
          '<button class="tpl-var-insert" id="tpl-create-submit">Create</button>' +
        '</div>' +
      '</div>';

    grid.querySelector('#tpl-create-cancel').addEventListener('click', function() { renderGrid(); });
    grid.querySelector('#tpl-create-submit').addEventListener('click', function() {
      createCustom(
        grid.querySelector('#tpl-new-name').value.trim(),
        grid.querySelector('#tpl-new-desc').value.trim(),
        grid.querySelector('#tpl-new-content').value,
        grid.querySelector('#tpl-new-cat').value
      );
    });
  }

  function createCustom(name, desc, content, category) {
    if (!name || !content) {
      SC.toast('Name and content are required', 'error');
      return;
    }
    category = category || 'Development';
    var vars = [];
    var re = /\{\{(\w+)\}\}/g;
    var match;
    while ((match = re.exec(content)) !== null) {
      if (vars.indexOf(match[1]) === -1) vars.push(match[1]);
    }
    SC.wsSend('template_create', { name: name, description: desc, content: content, category: category, variables: vars });
  }

  function renderList() {
    mergeTemplates();
    cacheTemplates();
    if (pickerEl) renderGrid();
  }

  SC.templates = {
    init: init,
    showPicker: showPicker,
    hidePicker: hidePicker,
    selectTemplate: selectTemplate,
    fillVariables: fillVariables,
    createCustom: createCustom,
    renderList: renderList
  };
})();
