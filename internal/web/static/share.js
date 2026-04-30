// SmartClaw - Share & Export
(function() {
  'use strict';

  function showModal() {
    var modal = SC.$('#share-modal');
    if (!modal) return;
    modal.classList.remove('hidden');
    document.addEventListener('keydown', onEscape);
  }

  function hideModal() {
    var modal = SC.$('#share-modal');
    if (!modal) return;
    modal.classList.add('hidden');
    document.removeEventListener('keydown', onEscape);
  }

  function onEscape(e) {
    if (e.key === 'Escape') hideModal();
  }

  function shareSession() {
    var sessionId = SC.state.ui.currentSessionId;
    if (!sessionId) {
      SC.toast('No active session to share', 'error');
      return;
    }
    var linkRow = SC.$('#share-link-row');
    if (linkRow) linkRow.innerHTML = '<span class="share-link-text" style="color:var(--tx-2);font-size:13px">Generating share link...</span>';
    showModal();
    fetch('/api/chat/share', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId })
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.error) {
        SC.toast(data.error, 'error');
        hideModal();
        return;
      }
      var shareUrl = window.location.origin + data.url;
      if (linkRow) {
        linkRow.innerHTML = '<input id="share-link-input" class="share-link-input" value="' + SC.escapeHtml(shareUrl) + '" readonly>' +
          '<button id="share-copy-btn" class="btn-primary sm" title="Copy link">Copy</button>';
        var copyBtn = SC.$('#share-copy-btn');
        if (copyBtn) {
          copyBtn.addEventListener('click', function() {
            var input = SC.$('#share-link-input');
            if (input) {
              navigator.clipboard.writeText(input.value).then(function() {
                SC.toast('Link copied!', 'success');
                copyBtn.textContent = 'Copied';
                setTimeout(function() { copyBtn.textContent = 'Copy'; }, 1500);
              }).catch(function() {
                input.select();
                document.execCommand('copy');
                SC.toast('Link copied!', 'success');
              });
            }
          });
        }
      }
    })
    .catch(function(err) {
      SC.toast('Share failed: ' + err.message, 'error');
      hideModal();
    });
  }

  function showResult(data) {
    if (!data) return;
    var shareUrl = window.location.origin + (data.url || '');
    showModal();
    var linkRow = SC.$('#share-link-row');
    if (linkRow) {
      linkRow.innerHTML = '<input id="share-link-input" class="share-link-input" value="' + SC.escapeHtml(shareUrl) + '" readonly>' +
        '<button id="share-copy-btn" class="btn-primary sm" title="Copy link">Copy</button>';
      var copyBtn = SC.$('#share-copy-btn');
      if (copyBtn) {
        copyBtn.addEventListener('click', function() {
          var input = SC.$('#share-link-input');
          if (input) {
            navigator.clipboard.writeText(input.value).then(function() {
              SC.toast('Link copied!', 'success');
            }).catch(function() {
              input.select();
              document.execCommand('copy');
              SC.toast('Link copied!', 'success');
            });
          }
        });
      }
    }
  }

  function exportMarkdown() {
    var sessionId = SC.state.ui.currentSessionId;
    if (!sessionId) {
      SC.toast('No active session to export', 'error');
      return;
    }
    window.open('/api/chat/export?id=' + encodeURIComponent(sessionId) + '&format=markdown', '_blank');
  }

  function exportPDF() {
    var chatEl = document.querySelector('.chat-messages');
    if (!chatEl) {
      window.print();
      return;
    }

    var sessionTitle = document.title || 'smartclaw-export';
    var htmlContent = '<!DOCTYPE html><html><head><meta charset="utf-8"><title>' +
      SC.escapeHtml(sessionTitle) +
      '</title><style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;' +
      'max-width:800px;margin:0 auto;padding:20px;color:#333}' +
      '.msg{margin:12px 0;padding:12px 16px;border-radius:8px}' +
      '.msg.user{background:#f0f0f0}' +
      '.msg.assistant{background:#e8f4fd}' +
      '.role{font-size:11px;font-weight:600;text-transform:uppercase;margin-bottom:4px;color:#666}' +
      '.content{line-height:1.6;white-space:pre-wrap;word-break:break-word}' +
      'pre{background:#f5f5f5;padding:12px;border-radius:6px;overflow-x:auto}' +
      'code{background:#f0f0f0;padding:2px 6px;border-radius:3px;font-size:13px}</style></head><body>';

    var messages = chatEl.querySelectorAll('.message');
    for (var i = 0; i < messages.length; i++) {
      var msg = messages[i];
      var role = msg.classList.contains('user') ? 'user' : 'assistant';
      var roleLabel = role === 'user' ? 'You' : 'SmartClaw';
      var content = msg.querySelector('.content') || msg.querySelector('.message-content');
      var contentText = content ? content.innerText : '';
      htmlContent += '<div class="msg ' + role + '"><div class="role">' + roleLabel +
        '</div><div class="content">' + SC.escapeHtml(contentText) + '</div></div>';
    }
    htmlContent += '</body></html>';

    SC.toast('Generating PDF...', 'info');

    fetch('/api/chat/export-pdf', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: htmlContent, title: sessionTitle })
    })
    .then(function(r) {
      if (!r.ok) {
        throw new Error('PDF generation failed');
      }
      return r.blob();
    })
    .then(function(blob) {
      var url = URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = sessionTitle + '.pdf';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      SC.toast('PDF exported!', 'success');
    })
    .catch(function(err) {
      SC.toast('Server PDF failed, using print fallback', 'error');
      window.print();
    });
  }

  function init() {
    var btn = SC.$('#btn-share');
    if (btn) {
      btn.addEventListener('click', shareSession);
    }
    var modal = SC.$('#share-modal');
    if (modal) {
      modal.addEventListener('click', function(e) {
        if (e.target === modal) hideModal();
      });
    }
    var closeBtn = SC.$('#share-modal-close');
    if (closeBtn) {
      closeBtn.addEventListener('click', hideModal);
    }
    var mdBtn = SC.$('#share-export-md');
    if (mdBtn) {
      mdBtn.addEventListener('click', exportMarkdown);
    }
    var pdfBtn = SC.$('#share-export-pdf');
    if (pdfBtn) {
      pdfBtn.addEventListener('click', exportPDF);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  SC.chatShare = {
    showModal: showModal,
    hideModal: hideModal,
    shareSession: shareSession,
    exportMarkdown: exportMarkdown,
    exportPDF: exportPDF,
    showResult: showResult
  };
})();
