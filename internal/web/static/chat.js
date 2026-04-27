// SmartClaw - Chat
(function() {
  'use strict';

  let currentAssistantEl = null;
  let currentAssistantIndex = -1;
  let currentContent = '';
  let currentThinking = '';
  let thinkingBlock = null;
  let renderRAF = null;
  let doneTimeout = null;

  function forceFinishIfStale() {
    if (currentAssistantIndex >= 0 && currentContent) {
      finishMessage({ tokens: 0, cost: 0 });
    }
  }

  function initVirtualList() {
    var chatEl = SC.$('#chat');
    var messagesEl = SC.$('#messages');
    var welcomeEl = SC.$('#welcome');
    if (!chatEl || !messagesEl) return;
    SC.vl = new SC.VirtualList(messagesEl, {
      scrollContainer: chatEl,
      welcomeEl: welcomeEl,
      renderItem: createMessageElement
    });
  }

  function createMessageElement(item, index) {
    var el = document.createElement('div');
    var role = item.role;
    el.className = 'message ' + role;
    el.dataset.msgIndex = index;
    el.dataset.msgId = item.msgId || ('msg-' + index);
    var roleLabel = role === 'user' ? 'You' : role === 'cmd_result' ? '▶ Command' : 'SmartClaw';
    var ts = item.displayTs || '';

    var actionsHtml = '';
    if (role === 'user') {
      actionsHtml = '<div class="msg-actions">' +
        '<button class="msg-action-btn msg-edit-btn" title="Edit">' +
          '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>' +
        '</button>' +
        '<button class="msg-action-btn msg-retry-btn" title="Retry">' +
          '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/></svg>' +
        '</button>' +
      '</div>';
    }

    var bubbleContent;
    if (role === 'cmd_result') {
      bubbleContent = '<div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px">' + SC.escapeHtml(item.content) + '</div>';
    } else if (role === 'assistant') {
      if (item.isStreaming) {
        if (item.thinkingContent) {
          bubbleContent = '<div class="msg-bubble"><details class="thinking-block" open><summary>💭 Thinking...</summary><div class="thinking-content">' + SC.escapeHtml(item.thinkingContent) + '</div></details>' + (item.content ? renderPlainText(item.content) : '') + '</div>';
        } else if (!item.content) {
          bubbleContent = '<div class="msg-bubble"><div class="thinking"><div class="think-eyes"><svg width="32" height="16" viewBox="0 0 32 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle cx="24" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle class="pupil-l" cx="8" cy="8" r="2" fill="currentColor"/><circle class="pupil-r" cx="24" cy="8" r="2" fill="currentColor"/></svg></div><span class="think-label">Thinking<span class="think-dots"><span></span><span></span><span></span></span></span></div></div>';
        } else {
          bubbleContent = '<div class="msg-bubble">' + renderPlainText(item.content) + '</div>';
        }
      } else if (item.isRendered) {
        bubbleContent = '<div class="msg-bubble rendered">' + renderMarkdown(item.content) + '</div>';
      } else {
        bubbleContent = '<div class="msg-bubble">' + SC.escapeHtml(item.content) + '</div>';
      }
    } else {
      bubbleContent = '<div class="msg-bubble">' + SC.escapeHtml(item.content) + '</div>';
    }

    var roleStyle = role === 'cmd_result' ? ' style="color:var(--accent)"' : '';
    el.innerHTML = '<div class="msg-role"' + roleStyle + '>' + roleLabel + '</div>' + actionsHtml + bubbleContent + (ts ? '<div class="msg-ts">' + ts + '</div>' : '');

    if (role === 'user') {
      bindMsgActions(el, index);
    }

    bindMessageContextMenu(el);

    if (role === 'assistant' && item.isRendered && !item.isStreaming) {
      var bubble = el.querySelector('.msg-bubble');
      if (bubble) {
        if (item.thinkingContent) {
          var tb = bubble.querySelector('.thinking-block');
          if (tb) {
            tb.open = false;
            var summary = tb.querySelector('summary');
            if (summary) summary.textContent = '💭 Thought process (' + item.thinkingContent.length + ' chars)';
          }
        }
        bindCodeCopy(bubble);
        postRenderMarkdown(bubble);
      }
    }

    return el;
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
    currentAssistantIndex = SC.state.messages.length - 1;
    if (SC.vl) SC.vl.setStreaming(true);
    if (doneTimeout) clearTimeout(doneTimeout);
    doneTimeout = setTimeout(forceFinishIfStale, 30000);
    if (currentAssistantEl) {
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      if (bubble) bubble.innerHTML = '<div class="thinking"><div class="think-eyes"><svg width="32" height="16" viewBox="0 0 32 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle cx="24" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle class="pupil-l" cx="8" cy="8" r="2" fill="currentColor"/><circle class="pupil-r" cx="24" cy="8" r="2" fill="currentColor"/></svg></div><span class="think-label">Thinking<span class="think-dots"><span></span><span></span><span></span></span></span></div>';
    }
    SC.wsSend('chat', { content: text });
    } catch (err) {
      console.error('[sendMessage Error]', err);
      SC.showErrorBanner('Send error: ' + err.message, sendMessage);
    }
  }

  function addMessage(role, content) {
    var welcome = SC.$('#welcome');
    if (welcome) welcome.classList.add('hidden');

    var msgIndex = SC.state.messages.length;
    var msgId = 'msg-' + msgIndex;
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    var item = {
      role: role,
      content: content,
      ts: Date.now(),
      msgId: msgId,
      displayTs: ts,
      isStreaming: role === 'assistant',
      isRendered: role !== 'assistant',
      thinkingContent: ''
    };

    var el = null;

    if (SC.vl) {
      SC.vl.addItem(item);
      el = SC.vl.getItemElement(msgIndex);
    } else {
      el = createMessageElement(item, msgIndex);
      SC.$('#messages').appendChild(el);
    }

    SC.scrollChat();
    SC.state.messages.push({ role, content, ts: Date.now(), msgId });
    return el;
  }

  function bindMsgActions(el, msgIndex) {
    const editBtn = el.querySelector('.msg-edit-btn');
    const retryBtn = el.querySelector('.msg-retry-btn');

    if (editBtn) {
      editBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        startEditMessage(el, msgIndex);
      });
    }

    if (retryBtn) {
      retryBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        retryMessage(el, msgIndex);
      });
    }
  }

  function startEditMessage(el, msgIndex) {
    const bubble = el.querySelector('.msg-bubble');
    const originalContent = SC.state.messages[msgIndex] ? SC.state.messages[msgIndex].content : bubble.textContent;
    const originalHtml = bubble.innerHTML;

    bubble.innerHTML = '';
    const textarea = document.createElement('textarea');
    textarea.className = 'msg-edit-textarea';
    textarea.value = originalContent;
    textarea.rows = Math.min(Math.max(originalContent.split('\n').length + 1, 3), 12);

    const btnRow = document.createElement('div');
    btnRow.className = 'msg-edit-btns';
    const saveBtn = document.createElement('button');
    saveBtn.className = 'btn-primary sm';
    saveBtn.textContent = 'Save';
    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn-ghost sm';
    cancelBtn.textContent = 'Cancel';

    btnRow.appendChild(saveBtn);
    btnRow.appendChild(cancelBtn);
    bubble.appendChild(textarea);
    bubble.appendChild(btnRow);

    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);

    saveBtn.addEventListener('click', function() {
      const newContent = textarea.value.trim();
      if (!newContent) {
        SC.toast('Message cannot be empty', 'error');
        return;
      }
      if (SC.state.messages[msgIndex]) {
        SC.state.messages[msgIndex].content = newContent;
      }
      if (SC.vl && SC.vl.items[msgIndex]) {
        SC.vl.items[msgIndex].content = newContent;
        SC.vl.items[msgIndex].isRendered = true;
      }
      bubble.innerHTML = SC.escapeHtml(newContent);
      SC.wsSend('chat_edit', { content: newContent, msgIndex: msgIndex });
      SC.toast('Message edited', 'success');
      if (SC.vl) SC.vl.refreshItemHeight(msgIndex);
    });

    cancelBtn.addEventListener('click', function() {
      bubble.innerHTML = originalHtml;
    });

    textarea.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') {
        e.preventDefault();
        bubble.innerHTML = originalHtml;
      }
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        saveBtn.click();
      }
    });
  }

  function retryMessage(el, msgIndex) {
    const msgData = SC.state.messages[msgIndex];
    if (!msgData) return;
    const content = msgData.content;

    var removeCount = (SC.state.messages[msgIndex + 1] && SC.state.messages[msgIndex + 1].role === 'assistant') ? 2 : 1;

    if (SC.vl) {
      SC.vl.removeItemsAt(msgIndex, removeCount);
      for (var i = 0; i < SC.vl.items.length; i++) {
        SC.vl.items[i].msgId = 'msg-' + i;
      }
    } else {
      const msgElements = SC.$$('#messages .message');
      let remove = false;
      const toRemove = [];
      for (let i = 0; i < msgElements.length; i++) {
        const idx = parseInt(msgElements[i].dataset.msgIndex, 10);
        if (idx === msgIndex) {
          remove = true;
          toRemove.push(msgElements[i]);
          continue;
        }
        if (remove && msgElements[i].classList.contains('assistant')) {
          toRemove.push(msgElements[i]);
          break;
        }
        if (remove && msgElements[i].classList.contains('user')) {
          break;
        }
      }
      toRemove.forEach(e => e.remove());
    }

    SC.state.messages.splice(msgIndex, removeCount);
    reindexMessages();

    addMessage('user', content);
    currentContent = '';
    currentAssistantEl = addMessage('assistant', '');
    currentAssistantIndex = SC.state.messages.length - 1;
    if (SC.vl) SC.vl.setStreaming(true);
    if (doneTimeout) clearTimeout(doneTimeout);
    doneTimeout = setTimeout(forceFinishIfStale, 30000);
    if (currentAssistantEl) {
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      if (bubble) bubble.innerHTML = '<div class="thinking"><div class="think-eyes"><svg width="32" height="16" viewBox="0 0 32 16" fill="none"><circle cx="8" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle cx="24" cy="8" r="5.5" stroke="currentColor" stroke-width="1.2"/><circle class="pupil-l" cx="8" cy="8" r="2" fill="currentColor"/><circle class="pupil-r" cx="24" cy="8" r="2" fill="currentColor"/></svg></div><span class="think-label">Thinking<span class="think-dots"><span></span><span></span><span></span></span></span></div>';
    }
    SC.wsSend('chat', { content: content });
  }

  function reindexMessages() {
    if (SC.vl) {
      for (var i = 0; i < SC.vl.items.length; i++) {
        SC.vl.items[i].msgId = 'msg-' + i;
      }
      for (var j = 0; j < SC.state.messages.length; j++) {
        SC.state.messages[j].msgId = 'msg-' + j;
      }
      SC.vl._render();
      return;
    }
    const msgElements = SC.$$('#messages .message');
    msgElements.forEach(function(el, i) {
      el.dataset.msgIndex = i;
      el.dataset.msgId = 'msg-' + i;
      const editBtn = el.querySelector('.msg-edit-btn');
      const retryBtn = el.querySelector('.msg-retry-btn');
      if (editBtn) {
        editBtn.onclick = function(e) { e.stopPropagation(); startEditMessage(el, i); };
      }
      if (retryBtn) {
        retryBtn.onclick = function(e) { e.stopPropagation(); retryMessage(el, i); };
      }
    });
  }

  function addCmdResult(content) {
    var welcome = SC.$('#welcome');
    if (welcome) welcome.classList.add('hidden');
    var msgIndex = SC.state.messages.length;
    var msgId = 'msg-' + msgIndex;
    const now = new Date();
    const ts = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    var item = {
      role: 'cmd_result',
      content: content,
      ts: Date.now(),
      msgId: msgId,
      displayTs: ts,
      isStreaming: false,
      isRendered: true,
      thinkingContent: ''
    };

    if (SC.vl) {
      SC.vl.addItem(item);
    } else {
      const el = document.createElement('div');
      el.className = 'message cmd-result';
      el.innerHTML = `<div class="msg-role" style="color:var(--accent)">▶ Command</div><div class="msg-bubble" style="font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px">${SC.escapeHtml(content)}</div><div class="msg-ts">${ts}</div>`;
      SC.$('#messages').appendChild(el);
    }

    SC.scrollChat();
    SC.state.messages.push({ role: 'cmd_result', content, ts: Date.now() });
  }

  function appendToken(token) {
    if (currentAssistantIndex < 0 && !currentAssistantEl) return;
    if (!SC.state.isProcessing) {
      SC.setState('isProcessing', true);
      SC.updateStopBtn();
    }
    currentContent += token;
    if (renderRAF) return;
    renderRAF = requestAnimationFrame(() => {
      renderRAF = null;

      if (SC.vl && currentAssistantIndex >= 0 && currentAssistantIndex < SC.vl.items.length) {
        SC.vl.items[currentAssistantIndex].content = currentContent;
      }

      var el = SC.vl ? SC.vl.getItemElement(currentAssistantIndex) : currentAssistantEl;
      if (!el) return;
      const bubble = el.querySelector('.msg-bubble');
      if (!bubble) return;
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.remove();

      const thinkingBlockEl = bubble.querySelector('.thinking-block');

      bubble.innerHTML = renderPlainText(currentContent);

      if (thinkingBlockEl) {
        bubble.prepend(thinkingBlockEl);
      }
      SC.scrollChat();
      if (SC.vl) SC.vl.refreshItemHeight(currentAssistantIndex);
    });
  }

  function appendThinking(token) {
    if (currentAssistantIndex < 0 && !currentAssistantEl) return;
    var el = SC.vl ? SC.vl.getItemElement(currentAssistantIndex) : currentAssistantEl;
    if (!el) return;
    const bubble = el.querySelector('.msg-bubble');
    if (!bubble) return;
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
    if (SC.vl && currentAssistantIndex >= 0) {
      SC.vl.items[currentAssistantIndex].thinkingContent = currentThinking;
      SC.vl.refreshItemHeight(currentAssistantIndex);
    }
  }

  function finishMessage(msg) {
    if (renderRAF) { cancelAnimationFrame(renderRAF); renderRAF = null; }

    if (currentAssistantIndex >= 0 || currentAssistantEl) {
      var el = SC.vl ? SC.vl.getItemElement(currentAssistantIndex) : currentAssistantEl;
      if (el) {
        const bubble = el.querySelector('.msg-bubble');
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

      if (SC.vl && currentAssistantIndex >= 0 && currentAssistantIndex < SC.vl.items.length) {
        SC.vl.items[currentAssistantIndex].isStreaming = false;
        SC.vl.items[currentAssistantIndex].isRendered = true;
        SC.vl.items[currentAssistantIndex].content = currentContent;
        SC.vl.items[currentAssistantIndex].thinkingContent = currentThinking;
        SC.vl.refreshItemHeight(currentAssistantIndex);
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
    currentAssistantIndex = -1;
    currentContent = '';
    currentThinking = '';
    SC.setState('isProcessing', false);
    SC.updateStopBtn();
    if (SC.vl) SC.vl.setStreaming(false);

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
        const lines = highlighted.split('\n');
        const lineSpans = lines.map(function(line, i) {
          return '<span class="code-line" data-line="' + (i + 1) + '">' + (line || ' ') + '</span>';
        }).join('');
        return `<div class="code-block"><div class="code-header"><span>${SC.escapeHtml(langLabel)}</span><button class="code-copy" data-code-id="${codeId}">Copy</button></div><div class="code-content"><code id="${codeId}">${lineSpans}</code></div></div>`;
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
      text = text.replace(/@msg-(\d+)/g, '<a href="#msg-$1" class="msg-ref" data-msg-ref="msg-$1">@msg-$1</a>');
      var html = marked.parse(text);
      if (typeof DOMPurify !== 'undefined') {
        html = DOMPurify.sanitize(html, {
          ALLOWED_TAGS: ['h1','h2','h3','h4','h5','h6','p','a','ul','ol','li','blockquote','pre','code','em','strong','del','table','thead','tbody','tr','th','td','img','hr','br','div','span','details','summary','sup','sub','dl','dt','dd','s','mark','abbr','cite','kbd','var','samp'],
          ALLOWED_ATTR: ['href','src','alt','title','class','id','target','rel','data-code-id','data-msg-id','data-msg-ref','data-line'],
          FORBID_ATTR: ['onerror','onload','onclick','onmouseover'],
          FORBID_TAGS: ['script','iframe','object','embed','form','input','textarea','button','style'],
          ADD_ATTR: ['target']
        });
      }
      return html;
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

    container.querySelectorAll('.code-line:not([data-ln-bound])').forEach(line => {
      line.dataset.lnBound = '1';
      line.addEventListener('click', function(e) {
        if (e.target.closest('.code-copy')) return;
        line.classList.toggle('highlighted');
      });
    });
  }

  function bindMessageContextMenu(el) {
    el.addEventListener('contextmenu', function(e) {
      e.preventDefault();
      const msgId = el.dataset.msgId || '';
      const msgIndex = el.dataset.msgIndex;
      const bubble = el.querySelector('.msg-bubble');
      const fullText = bubble ? bubble.textContent : '';
      const items = [
        { label: 'Copy', action: function() { copyText(fullText); } },
        { label: 'Quote Reply', action: function() { insertQuoteReply(msgId, fullText); } },
        { label: 'Copy Message ID', action: function() { copyText(msgId); } }
      ];

      const codeBlock = e.target.closest('.code-block');
      if (codeBlock) {
        const codeEl = codeBlock.querySelector('code');
        if (codeEl) {
          items.splice(1, 0, { label: 'Copy Code', action: function() { copyText(codeEl.textContent); } });
        }
      }

      SC.showContextMenu(e.clientX, e.clientY, items);
    });
  }

  function copyText(text) {
    navigator.clipboard.writeText(text).then(function() {
      SC.toast('Copied to clipboard', 'success');
    }).catch(function() {
      var ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      SC.toast('Copied to clipboard', 'success');
    });
  }

  function insertQuoteReply(msgId, text) {
    var input = SC.$('#input');
    var excerpt = text.trim().replace(/\n/g, ' ').slice(0, 80);
    if (text.trim().length > 80) excerpt += '...';
    var quote = '> [@' + msgId + ' ' + excerpt + '] ';
    var current = input.value;
    if (current && !current.endsWith('\n') && !current.endsWith(' ')) current += '\n';
    input.value = current + quote;
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
    input.style.height = 'auto';
    input.style.height = Math.min(input.scrollHeight, 180) + 'px';
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
  SC.bindMessageContextMenu = bindMessageContextMenu;
  SC.reindexMessages = reindexMessages;
  SC.startEditMessage = startEditMessage;
  SC.retryMessage = retryMessage;
  SC.bindMsgActions = bindMsgActions;
  SC.initVirtualList = initVirtualList;
  SC.createMessageElement = createMessageElement;
})();
