// SmartClaw - Chat
(function() {
  'use strict';

  let currentAssistantEl = null;
  let currentContent = '';
  let currentThinking = '';
  let thinkingBlock = null;
  let renderRAF = null;
  let doneTimeout = null;

  function forceFinishIfStale() {
    if (currentAssistantEl && currentContent) {
      finishMessage({ tokens: 0, cost: 0 });
    }
  }

  function sendMessage() {
    try {
    const input = SC.$('#input');
    const mention = SC.$('#file-mention');
    if (mention && !mention.classList.contains('hidden')) return;
    const text = input.value.trim();
    if (!text) return;

    addMessage('user', text);
    SC.state.commandHistory.push(text);
    SC.state.historyIndex = SC.state.commandHistory.length;
    input.value = '';
    input.style.height = 'auto';

    if (text.startsWith('/')) {
      const parts = text.split(' ');
      SC.wsSend('cmd', { name: parts[0], args: parts.slice(1) });
      return;
    }

    currentContent = '';
    currentAssistantEl = addMessage('assistant', '');
    if (doneTimeout) clearTimeout(doneTimeout);
    doneTimeout = setTimeout(forceFinishIfStale, 30000);
    const bubble = currentAssistantEl.querySelector('.msg-bubble');
    bubble.innerHTML = '<div class="thinking"><div class="think-eyes"><svg width="32" height="16" viewBox="0 0 32 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle cx="24" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle class="pupil-l" cx="8" cy="8" r="2" fill="currentColor"/><circle class="pupil-r" cx="24" cy="8" r="2" fill="currentColor"/></svg></div><span class="think-label">Thinking<span class="think-dots"><span></span><span></span><span></span></span></span></div>';
    SC.wsSend('chat', { content: text });
    } catch (err) {
      console.error('[sendMessage Error]', err);
      SC.showErrorBanner('Send error: ' + err.message, sendMessage);
    }
  }

  function addMessage(role, content) {
    const welcome = SC.$('.welcome');
    if (welcome) welcome.remove();
    const el = document.createElement('div');
    el.className = `message ${role}`;
    const roleLabel = role === 'user' ? 'You' : 'SmartClaw';
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    el.innerHTML = `<div class="msg-role">${roleLabel}</div><div class="msg-bubble">${SC.escapeHtml(content)}</div><div class="msg-ts">${ts}</div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
    SC.state.messages.push({ role, content, ts: Date.now() });
    return el;
  }

  function addCmdResult(content) {
    const welcome = SC.$('.welcome');
    if (welcome) welcome.remove();
    const el = document.createElement('div');
    el.className = 'message cmd-result';
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    el.innerHTML = `<div class="msg-role" style="color:var(--accent)">▶ Command</div><div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px">${SC.escapeHtml(content)}</div><div class="msg-ts">${ts}</div>`;
    SC.$('#messages').appendChild(el);
    SC.scrollChat();
    SC.state.messages.push({ role: 'cmd_result', content, ts: Date.now() });
    return el;
  }

  function appendToken(token) {
    if (!currentAssistantEl) return;
    if (!SC.state.isProcessing) {
      SC.setState('isProcessing', true);
      SC.updateStopBtn();
    }
    currentContent += token;
    if (renderRAF) return;
    renderRAF = requestAnimationFrame(() => {
      renderRAF = null;
      if (!currentAssistantEl) return;
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.remove();

      const thinkingBlockEl = bubble.querySelector('.thinking-block');

      bubble.innerHTML = renderPlainText(currentContent);

      if (thinkingBlockEl) {
        bubble.prepend(thinkingBlockEl);
      }
      SC.scrollChat();
    });
  }

  function appendThinking(token) {
    if (!currentAssistantEl) return;
    const bubble = currentAssistantEl.querySelector('.msg-bubble');
    currentThinking += token;
    if (!thinkingBlock) {
      thinkingBlock = document.createElement('details');
      thinkingBlock.className = 'thinking-block';
      thinkingBlock.open = true;
      thinkingBlock.innerHTML = '<summary>💭 Thinking...</summary><div class="thinking-content"></div>';
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.replaceWith(thinkingBlock);
      else bubble.prepend(thinkingBlock);
    }
    const content = thinkingBlock.querySelector('.thinking-content');
    content.textContent = currentThinking;
    thinkingBlock.querySelector('summary').textContent = '💭 Thinking...';
    SC.scrollChat();
  }

  function finishMessage(msg) {
    if (renderRAF) { cancelAnimationFrame(renderRAF); renderRAF = null; }
    if (currentAssistantEl && currentContent) {
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      if (bubble) {
        const thinking = bubble.querySelector('.thinking');
        if (thinking) thinking.remove();
        const thinkingBlockEl = bubble.querySelector('.thinking-block');
        try {
          bubble.innerHTML = renderMarkdown(currentContent);
          bubble.classList.add('rendered');
        } catch (e) {
          console.error('renderMarkdown error:', e);
          bubble.innerHTML = renderPlainText(currentContent);
        }
        if (thinkingBlockEl) {
          thinkingBlockEl.open = false;
          thinkingBlockEl.querySelector('summary').textContent = '💭 Thought process (' + currentThinking.length + ' chars)';
          bubble.prepend(thinkingBlockEl);
          thinkingBlock = null;
        }
        bindCodeCopy(bubble);
        postRenderMarkdown(bubble);
      }
    }
    if (msg.tokens) {
      SC.state.tokens.used = msg.tokens;
      SC.state.cost += msg.cost || 0;
      if (msg.costBreakdown) {
        SC.state.lastCostBreakdown = msg.costBreakdown;
        const model = msg.costBreakdown.model || msg.model || 'unknown';
        if (!SC.state.costByModel[model]) SC.state.costByModel[model] = 0;
        SC.state.costByModel[model] += msg.cost || 0;
      }
      SC.updateStats();
      SC.state.tokenHistory.push({ t: Date.now(), v: msg.tokens });
      SC.state.costHistory.push({ t: Date.now(), v: SC.state.cost, model: msg.costBreakdown?.model || msg.model });
    }
    if (thinkingBlock) {
      thinkingBlock.open = false;
      thinkingBlock.querySelector('summary').textContent = '💭 Thought process (' + currentThinking.length + ' chars)';
      thinkingBlock = null;
    }
    if (doneTimeout) { clearTimeout(doneTimeout); doneTimeout = null; }
    currentAssistantEl = null;
    currentContent = '';
    currentThinking = '';
    SC.setState('isProcessing', false);
    SC.updateStopBtn();

    if (SC.state.ui.currentSessionId && SC.state.messages.length > 0) {
      const currentSession = (SC.state.sessions || []).find(s => s.id === SC.state.ui.currentSessionId);
      const title = currentSession?.title || '';
      if (!title || title === 'Untitled' || title === '') {
        const firstUserMsg = SC.state.messages.find(m => m.role === 'user');
        if (firstUserMsg) {
          let autoTitle = firstUserMsg.content.trim().replace(/\n/g, ' ');
          if (autoTitle.length > 50) autoTitle = autoTitle.slice(0, 50) + '...';
          if (autoTitle) SC.wsSend('session_rename', { id: SC.state.ui.currentSessionId, title: autoTitle });
        }
      }
    }
  }

  marked.use({
    renderer: {
      code({ text, lang }) {
        if (lang === 'mermaid') {
          const mermaidId = 'mermaid-' + Math.random().toString(36).slice(2, 8);
          return `<div class="mermaid" id="${mermaidId}">${SC.escapeHtml(text)}</div>`;
        }
        const codeId = 'code-' + Math.random().toString(36).slice(2, 8);
        const langLabel = lang || 'code';
        let highlighted;
        try {
          if (typeof hljs !== 'undefined' && lang && hljs.getLanguage(lang)) {
            highlighted = hljs.highlight(text, { language: lang }).value;
          } else if (typeof hljs !== 'undefined') {
            highlighted = hljs.highlightAuto(text).value;
          } else {
            highlighted = SC.escapeHtml(text);
          }
        } catch (e) {
          highlighted = SC.escapeHtml(text);
        }
        return `<div class="code-block"><div class="code-header"><span>${SC.escapeHtml(langLabel)}</span><button class="code-copy" data-code-id="${codeId}">Copy</button></div><div class="code-content"><code id="${codeId}">${highlighted}</code></div></div>`;
      },
      codespan({ text }) {
        return `<code style="background:var(--bg-2);padding:2px 6px;border-radius:3px;font-family:var(--font-d);font-size:13px">${text}</code>`;
      }
    },
    breaks: true,
    gfm: true
  });

  function renderMarkdown(text) {
    try {
      return marked.parse(text);
    } catch (e) {
      return SC.escapeHtml(text).replace(/\n/g, '<br>');
    }
  }

  function renderKatex(element) {
    if (typeof window.renderMathInElement === 'function') {
      try {
        renderMathInElement(element, {
          delimiters: [
            { left: '$$', right: '$$', display: true },
            { left: '$', right: '$', display: false }
          ],
          throwOnError: false
        });
      } catch (e) {
        console.warn('KaTeX render error:', e);
      }
    }
  }

  function renderMermaidInElement(element) {
    const mermaidNodes = element.querySelectorAll('.mermaid');
    if (mermaidNodes.length === 0) return;
    if (typeof window.mermaid !== 'undefined') {
      try {
        mermaid.run({ nodes: mermaidNodes });
      } catch (e) {
        console.warn('Mermaid render error:', e);
        mermaidNodes.forEach(node => {
          if (!node.querySelector('svg')) {
            const raw = node.textContent;
            node.innerHTML = `<pre style="margin:0;white-space:pre-wrap;font-size:12px;color:var(--tx-2)">${SC.escapeHtml(raw)}</pre>`;
          }
        });
      }
    }
  }

  function postRenderMarkdown(element) {
    renderKatex(element);
    renderMermaidInElement(element);
  }

  function renderPlainText(text) {
    return `<div class="streaming-text">${SC.escapeHtml(text).replace(/\n/g, '<br>')}</div>`;
  }

  function bindCodeCopy(container) {
    container.querySelectorAll('.code-copy:not([data-bound])').forEach(btn => {
      btn.dataset.bound = '1';
      btn.addEventListener('click', () => {
        const codeId = btn.dataset.codeId;
        const codeEl = codeId ? document.getElementById(codeId) : btn.closest('.code-block')?.querySelector('code');
        if (!codeEl) return;
        const text = codeEl.textContent;
        navigator.clipboard.writeText(text).then(() => {
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
        }).catch(() => {
          const ta = document.createElement('textarea');
          ta.value = text;
          ta.style.position = 'fixed';
          ta.style.left = '-9999px';
          document.body.appendChild(ta);
          ta.select();
          document.execCommand('copy');
          document.body.removeChild(ta);
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
        });
      });
    });
  }

  SC.sendMessage = sendMessage;
  SC.addMessage = addMessage;
  SC.addCmdResult = addCmdResult;
  SC.appendToken = appendToken;
  SC.appendThinking = appendThinking;
  SC.finishMessage = finishMessage;
  SC.renderMarkdown = renderMarkdown;
  SC.postRenderMarkdown = postRenderMarkdown;
  SC.renderPlainText = renderPlainText;
  SC.bindCodeCopy = bindCodeCopy;
})();
