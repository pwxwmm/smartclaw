// SmartClaw - Artifacts Preview Panel
(function() {
  'use strict';

  var activeArtifact = null;
  var artifactHistory = [];

  function detectArtifactType(lang, code) {
    if (!lang) return null;
    var l = lang.toLowerCase().trim();
    if (l === 'html' || l === 'htmlmixed') return 'html';
    if (l === 'svg') return 'svg';
    if (l === 'mermaid') return 'mermaid';
    if (l === 'jsx' || l === 'tsx' || l === 'react') return 'react';
    if (l === 'markdown' || l === 'md') return 'markdown';
    return null;
  }

  function renderArtifact(type, code) {
    var preview = SC.$('#artifact-preview-content');
    if (!preview) return;

    if (type === 'html' || type === 'svg') {
      var iframe = document.createElement('iframe');
      iframe.sandbox = 'allow-scripts allow-same-origin';
      iframe.style.cssText = 'width:100%;height:100%;border:none;background:#fff;';
      preview.innerHTML = '';
      preview.appendChild(iframe);
      var doc = iframe.contentDocument || iframe.contentWindow.document;
      doc.open();
      if (type === 'svg') {
        doc.write('<!DOCTYPE html><html><head><style>body{margin:0;display:flex;align-items:center;justify-content:center;min-height:100vh;background:#f8f8f8;}</style></head><body>' + code + '</body></html>');
      } else {
        doc.write(code);
      }
      doc.close();
    } else if (type === 'mermaid') {
      preview.innerHTML = '<div id="artifact-mermaid-target" style="padding:20px;background:#fff;min-height:100%;display:flex;align-items:center;justify-content:center;"></div>';
      if (typeof mermaid !== 'undefined') {
        try {
          var target = SC.$('#artifact-mermaid-target');
          var id = 'mermaid-artifact-' + Date.now();
          mermaid.render(id, code).then(function(result) {
            target.innerHTML = result.svg;
          }).catch(function(err) {
            target.innerHTML = '<div style="color:#e05252;padding:20px;">Mermaid render error: ' + SC.escapeHtml(err.message || String(err)) + '</div>';
          });
        } catch(e) {
          SC.$('#artifact-mermaid-target').innerHTML = '<div style="color:#e05252;padding:20px;">Mermaid error: ' + SC.escapeHtml(String(e)) + '</div>';
        }
      }
    } else if (type === 'markdown') {
      if (typeof SC.renderMarkdown === 'function') {
        preview.innerHTML = '<div style="padding:20px;background:#fff;max-width:720px;margin:0 auto;">' + SC.renderMarkdown(code) + '</div>';
      } else {
        preview.innerHTML = '<pre style="padding:20px;white-space:pre-wrap;">' + SC.escapeHtml(code) + '</pre>';
      }
    } else if (type === 'react') {
      preview.innerHTML = '<iframe sandbox="allow-scripts" style="width:100%;height:100%;border:none;background:#fff;"></iframe>';
      var rIframe = preview.querySelector('iframe');
      var rDoc = rIframe.contentDocument || rIframe.contentWindow.document;
      rDoc.open();
      rDoc.write('<!DOCTYPE html><html><head><script src="https://unpkg.com/react@18/umd/react.development.js"><\/script><script src="https://unpkg.com/react-dom@18/umd/react-dom.development.js"><\/script><script src="https://unpkg.com/@babel/standalone/babel.min.js"><\/script><style>body{margin:0;font-family:sans-serif;}</style></head><body><div id="root"></div><script type="text/babel">' + code + '<\/script></body></html>');
      rDoc.close();
    }
  }

  function openArtifact(code, type, label) {
    var panel = SC.$('#artifact-panel');
    if (!panel) return;

    panel.classList.add('visible');
    activeArtifact = { code: code, type: type, label: label || type };
    artifactHistory.push(activeArtifact);

    var titleEl = SC.$('#artifact-title');
    if (titleEl) titleEl.textContent = label || type.toUpperCase() + ' Preview';

    renderArtifact(type, code);
  }

  function closeArtifact() {
    var panel = SC.$('#artifact-panel');
    if (panel) panel.classList.remove('visible');
    activeArtifact = null;
  }

  function addPreviewButtons() {
    var codeBlocks = SC.$$('#messages .code-block');
    codeBlocks.forEach(function(block) {
      if (block.dataset.artifactWired) return;
      block.dataset.artifactWired = 'true';

      var header = block.querySelector('.code-header');
      if (!header) return;

      var langSpan = header.querySelector('span');
      if (!langSpan) return;

      var lang = langSpan.textContent || '';
      var codeEl = block.querySelector('code');
      if (!codeEl) return;

      var type = detectArtifactType(lang, codeEl.textContent);
      if (!type) return;

      var btn = document.createElement('button');
      btn.className = 'code-copy artifact-preview-btn';
      btn.innerHTML = '▷ Preview';
      btn.title = 'Open live preview';
      btn.addEventListener('click', function() {
        openArtifact(codeEl.textContent, type, lang.toUpperCase() + ' Artifact');
        if (SC.audio && SC.audio.click) SC.audio.click();
      });

      header.appendChild(btn);
    });
  }

  function initArtifacts() {
    var closeBtn = SC.$('#artifact-close');
    var refreshBtn = SC.$('#artifact-refresh');
    var toggleCode = SC.$('#artifact-toggle-code');
    var codeView = SC.$('#artifact-code-view');

    if (closeBtn) closeBtn.addEventListener('click', closeArtifact);
    if (refreshBtn) {
      refreshBtn.addEventListener('click', function() {
        if (activeArtifact) renderArtifact(activeArtifact.type, activeArtifact.code);
      });
    }
    if (toggleCode && codeView) {
      toggleCode.addEventListener('click', function() {
        codeView.classList.toggle('visible');
        if (codeView.classList.contains('visible') && activeArtifact) {
          codeView.textContent = activeArtifact.code;
        }
      });
    }

    // Observe new messages for preview buttons
    var observer = new MutationObserver(function() {
      setTimeout(addPreviewButtons, 100);
    });
    var messages = SC.$('#messages');
    if (messages) {
      observer.observe(messages, { childList: true, subtree: true });
    }

    addPreviewButtons();
  }

  SC.openArtifact = openArtifact;
  SC.closeArtifact = closeArtifact;
  SC.initArtifacts = initArtifacts;
  SC.addPreviewButtons = addPreviewButtons;
})();
