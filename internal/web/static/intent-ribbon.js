(function() {
  'use strict';

  var INTENTS = [
    { id: 'debug', label: '🔧 Debug', keywords: ['error', 'bug', 'fix', 'crash', 'panic', 'fail', 'broken', 'nil pointer', 'segfault', 'stack trace', 'traceback', 'exception'], prefix: 'Debug and fix this error: ' },
    { id: 'refactor', label: '♻️ Refactor', keywords: ['refactor', 'clean up', 'restructure', 'simplify', 'extract', 'optimize', 'improve', 'rewrite'], prefix: 'Refactor the following code: ' },
    { id: 'test', label: '🧪 Test', keywords: ['test', 'unit test', 'integration', 'coverage', 'spec', 'assert', 'mock'], prefix: 'Write tests for: ' },
    { id: 'explain', label: '📖 Explain', keywords: ['explain', 'how does', 'what is', 'why does', 'understand', 'describe', 'clarify', 'meaning'], prefix: 'Explain: ' },
    { id: 'architect', label: '🏗️ Architect', keywords: ['architect', 'design', 'structure', 'pattern', 'system', 'component', 'module', 'interface'], prefix: 'Design the architecture for: ' },
    { id: 'deploy', label: '🚀 Deploy', keywords: ['deploy', 'release', 'ci', 'cd', 'pipeline', 'docker', 'kubernetes', 'production', 'staging'], prefix: 'Help me deploy: ' },
    { id: 'security', label: '🔒 Security', keywords: ['security', 'vulnerability', 'cve', 'auth', 'encrypt', 'sanitize', 'xss', 'injection', 'csrf'], prefix: 'Security review: ' },
    { id: 'perf', label: '⚡ Performance', keywords: ['performance', 'slow', 'latency', 'memory', 'cpu', 'optimize', 'bottleneck', 'profile', 'benchmark'], prefix: 'Optimize performance of: ' }
  ];

  var MIN_LENGTH = 3;
  var debounceTimer = null;
  var DEBOUNCE_MS = 300;
  var lastIntents = [];

  function detectIntents(text) {
    if (!text || text.length < MIN_LENGTH) return [];
    var lower = text.toLowerCase();
    var scored = INTENTS.map(function(intent) {
      var score = 0;
      intent.keywords.forEach(function(kw) {
        if (lower.indexOf(kw) >= 0) score += kw.length;
      });
      return { intent: intent, score: score };
    }).filter(function(s) { return s.score > 0; });
    scored.sort(function(a, b) { return b.score - a.score; });
    return scored.slice(0, 3).map(function(s) { return s.intent; });
  }

  function render(intents) {
    var ribbon = SC.$('#intent-ribbon');
    if (!ribbon) return;

    if (intents.length === 0) {
      ribbon.innerHTML = '';
      ribbon.classList.add('hidden');
      return;
    }

    ribbon.classList.remove('hidden');
    var html = '';
    intents.forEach(function(intent) {
      html += '<button class="intent-chip" data-intent-id="' + SC.escapeHtml(intent.id) + '" data-prefix="' + SC.escapeHtml(intent.prefix) + '">' + intent.label + '</button>';
    });
    ribbon.innerHTML = html;

    ribbon.querySelectorAll('.intent-chip').forEach(function(chip) {
      chip.addEventListener('click', function() {
        var prefix = this.dataset.prefix;
        var input = SC.$('#input');
        if (!input) return;
        var currentText = input.value.trim();
        input.value = prefix + currentText;
        input.focus();
        input.setSelectionRange(input.value.length, input.value.length);
        input.style.height = 'auto';
        input.style.height = Math.min(input.scrollHeight, 180) + 'px';
        ribbon.innerHTML = '';
        ribbon.classList.add('hidden');
      });
    });
  }

  function onInputChange() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(function() {
      var input = SC.$('#input');
      if (!input) return;
      var text = input.value.trim();
      var intents = detectIntents(text);
      if (intents.length !== lastIntents.length || intents.some(function(i, idx) { return i.id !== lastIntents[idx].id; })) {
        lastIntents = intents;
        render(intents);
      }
    }, DEBOUNCE_MS);
  }

  function initIntentRibbon() {
    var input = SC.$('#input');
    if (input) {
      input.addEventListener('input', onInputChange);
      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
          var ribbon = SC.$('#intent-ribbon');
          if (ribbon) { ribbon.innerHTML = ''; ribbon.classList.add('hidden'); }
          lastIntents = [];
        }
      });
    }
  }

  SC.initIntentRibbon = initIntentRibbon;
})();
