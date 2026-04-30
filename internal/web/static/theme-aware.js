// SmartClaw - Code-Aware Theme
(function() {
  'use strict';

  var LANG_HUES = {
    go: 215,
    rust: 0,
    python: 45,
    typescript: 230,
    javascript: 50,
    java: 15,
    'c++': 165,
    cpp: 165,
    c: 165
  };

  var DEFAULT_HUE = 265;

  var EXT_LANG = {
    go: 'go',
    rs: 'rust',
    py: 'python',
    pyw: 'python',
    ts: 'typescript',
    tsx: 'typescript',
    js: 'javascript',
    mjs: 'javascript',
    cjs: 'javascript',
    jsx: 'javascript',
    java: 'java',
    cpp: 'c++',
    cc: 'c++',
    cxx: 'c++',
    c: 'c',
    h: 'c',
    hpp: 'c++'
  };

  var currentHue = DEFAULT_HUE;
  var observer = null;

  function applyLanguageAccent(lang) {
    var hue = LANG_HUES[lang] !== undefined ? LANG_HUES[lang] : DEFAULT_HUE;
    if (hue === currentHue) return;
    currentHue = hue;
    var root = document.documentElement;
    root.style.setProperty('--accent-hue', hue);
    root.style.setProperty('--accent', 'hsl(' + hue + ', 70%, 60%)');
    root.style.setProperty('--accent-h', 'hsl(' + hue + ', 70%, 68%)');
    root.style.setProperty('--accent-bg', 'hsla(' + hue + ', 70%, 60%, 0.1)');
    root.style.setProperty('--accent-bd', 'hsla(' + hue + ', 70%, 60%, 0.2)');
  }

  function detectFromAttribute() {
    var lang = document.body.getAttribute('data-project-lang');
    if (lang) return lang;
    return null;
  }

  function detectFromTree() {
    var tree = SC.state && SC.state.fileTreeData;
    if (!tree || !tree.length) return null;
    var counts = {};
    countExtensions(tree, counts);
    var max = 0;
    var detected = null;
    for (var ext in counts) {
      var lang = EXT_LANG[ext];
      if (lang && counts[ext] > max) {
        max = counts[ext];
        detected = lang;
      }
    }
    return detected;
  }

  function countExtensions(nodes, counts) {
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      if (node.type === 'dir') {
        if (node.children) countExtensions(node.children, counts);
      } else {
        var dot = node.name.lastIndexOf('.');
        if (dot > 0) {
          var ext = node.name.slice(dot + 1).toLowerCase();
          counts[ext] = (counts[ext] || 0) + 1;
        }
      }
    }
  }

  function refresh() {
    var lang = detectFromAttribute() || detectFromTree();
    applyLanguageAccent(lang || 'default');
  }

  function initThemeAware() {
    document.body.style.transition = (document.body.style.transition ? document.body.style.transition + ', ' : '') +
      'color 500ms ease, background-color 500ms ease, border-color 500ms ease, box-shadow 500ms ease';

    var root = document.documentElement;
    root.style.setProperty('--accent-hue', DEFAULT_HUE);

    refresh();

    observer = new MutationObserver(function(mutations) {
      for (var i = 0; i < mutations.length; i++) {
        if (mutations[i].attributeName === 'data-project-lang') {
          refresh();
          return;
        }
      }
    });
    observer.observe(document.body, { attributes: true, attributeFilter: ['data-project-lang'] });

    var origWsHandler = SC._onFileTreeUpdate;
    SC._onFileTreeUpdate = function() {
      if (typeof origWsHandler === 'function') origWsHandler.apply(this, arguments);
      if (!detectFromAttribute()) refresh();
    };
  }

  SC.initThemeAware = initThemeAware;
  SC.applyLanguageAccent = applyLanguageAccent;
})();
