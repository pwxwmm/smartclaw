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
  let batchLoadCounter = 0;

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

  var OWL_AVATAR_SVG = '<svg width="28" height="28" viewBox="0 0 32 32" fill="none">' +
    '<circle cx="16" cy="16" r="15" fill="var(--bg-3)"/>' +
    '<ellipse cx="16" cy="19" rx="9" ry="10" fill="var(--tx-2)" opacity=".45"/>' +
    '<path d="M7 6l4 9M25 6l-4 9" stroke="var(--tx-2)" stroke-width="2.2" stroke-linecap="round" opacity=".45"/>' +
    '<circle cx="12" cy="16" r="3" fill="#c084fc"/>' +
    '<circle cx="20" cy="16" r="3" fill="#c084fc"/>' +
    '<circle cx="12" cy="16" r="1.5" fill="#0d0d1a"/>' +
    '<circle cx="20" cy="16" r="1.5" fill="#0d0d1a"/>' +
    '<circle cx="11.2" cy="15.2" r=".7" fill="#fff" opacity=".8"/>' +
    '<circle cx="19.2" cy="15.2" r=".7" fill="#fff" opacity=".8"/>' +
    '<path d="M15 20l1 3 1-3" fill="#c084fc"/>' +
    '</svg>';

  function createMessageElement(item, index) {
    var el = document.createElement('div');
    var role = item.role;
    el.className = 'message ' + role;
    el.dataset.msgIndex = index;
    el.dataset.msgId = item.msgId || ('msg-' + index);
    if (SC.isMessageBookmarked(item.msgId || ('msg-' + index))) {
      el.classList.add('bookmarked');
    }
    var ts = item.displayTs || '';

    var items = SC.vl ? SC.vl.items : SC.state.messages;
    var prevItem = index > 0 ? items[index - 1] : null;
    var nextItem = index < items.length - 1 ? items[index + 1] : null;
    var prevRole = prevItem ? prevItem.role : null;
    var nextRole = nextItem ? nextItem.role : null;

    var isFirstInGroup = prevRole !== role;
    var isLastInGroup = nextRole !== role;

    if (isFirstInGroup && !isLastInGroup) el.classList.add('msg-group-first');
    else if (!isFirstInGroup && !isLastInGroup) el.classList.add('msg-group-middle');
    else if (!isFirstInGroup && isLastInGroup) el.classList.add('msg-group-last');

    if (isFirstInGroup) el.classList.add('msg-group-start');

    if (batchLoadCounter > 0) {
      el.style.animationDelay = (batchLoadCounter * 50) + 'ms';
      batchLoadCounter++;
    }

    var timeDividerHtml = '';
    if (prevItem && item.ts && prevItem.ts) {
      var gap = item.ts - prevItem.ts;
      if (gap > 5 * 60 * 1000) {
        var dividerTime = new Date(item.ts);
        var timeStr = dividerTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        timeDividerHtml = '<div class="time-divider"><span>' + SC.escapeHtml(timeStr) + '</span></div>';
      }
    }

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
      var imagesHtml = '';
      if (item.images && item.images.length > 0) {
        imagesHtml = '<div class="msg-image-grid">';
        for (var imgIdx = 0; imgIdx < item.images.length; imgIdx++) {
          imagesHtml += '<img class="msg-image" src="data:' + SC.escapeHtml(item.images[imgIdx].type) + ';base64,' + item.images[imgIdx].data + '" alt="Image">';
        }
        imagesHtml += '</div>';
      }
      bubbleContent = '<div class="msg-bubble">' + imagesHtml + SC.escapeHtml(item.content) + '</div>';
    }

    if (role === 'assistant') {
      el.innerHTML = timeDividerHtml +
        '<div class="msg-row">' +
          '<div class="msg-avatar">' + OWL_AVATAR_SVG + '</div>' +
          '<div class="msg-content">' + bubbleContent + '</div>' +
        '</div>' +
        (ts ? '<div class="msg-ts">' + ts + '</div>' : '');
    } else if (role === 'user') {
      el.innerHTML = timeDividerHtml + actionsHtml + bubbleContent + (ts ? '<div class="msg-ts">' + ts + '</div>' : '');
    } else {
      var roleLabel = role === 'cmd_result' ? '▶ Command' : 'SmartClaw';
      var roleStyle = role === 'cmd_result' ? ' style="color:var(--accent)"' : '';
      el.innerHTML = timeDividerHtml + '<div class="msg-role"' + roleStyle + '>' + roleLabel + '</div>' + bubbleContent + (ts ? '<div class="msg-ts">' + ts + '</div>' : '');
    }

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
    if (!text && (!SC.state.pendingImages || SC.state.pendingImages.length === 0)) return;

    addMessage('user', text);
    SC.state.commandHistory.push(text);
    SC.state.historyIndex = SC.state.commandHistory.length;
    input.value = '';
    input.style.height = 'auto';

    if (text.startsWith('/')) {
      if (text === '/template' || text.startsWith('/template ')) {
        if (SC.templates) SC.templates.showPicker();
        return;
      }
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
      if (bubble) {
        bubble.innerHTML = '<span class="streaming-cursor">▍</span>';
        bubble.classList.add('streaming');
      }
    }
    var chatData = { content: text };
    if (SC.state.pendingImages && SC.state.pendingImages.length > 0) {
      chatData.images = SC.state.pendingImages.map(function(img) {
        return { data: img.data, type: img.type };
      });
      SC.clearImagePreviews();
    }
    SC.wsSend('chat', chatData);
    if (SC.audio && SC.audio.messageSent) SC.audio.messageSent();
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
    SC.state.messages.push({ role: role, content: content, ts: Date.now(), msgId: msgId, displayTs: ts, isStreaming: item.isStreaming, isRendered: item.isRendered, thinkingContent: '', images: (SC.state.pendingImages && SC.state.pendingImages.length > 0) ? SC.state.pendingImages.map(function(img) { return { data: img.data, type: img.type }; }) : undefined });
    if (typeof SC.emit === 'function') SC.emit('messages', SC.state.messages);

    if (SC.Tabs) {
      var activeTab = SC.Tabs.getActive();
      if (activeTab) {
        activeTab.vlItems = SC.vl ? SC.vl.items.slice() : [];
      }
    }

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

    var originalMsgs = SC.state.messages.slice(msgIndex);

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
    if (typeof SC.emit === 'function') SC.emit('messages', SC.state.messages);

    addMessage('user', content);
    currentContent = '';
    currentAssistantEl = addMessage('assistant', '');
    currentAssistantIndex = SC.state.messages.length - 1;
    if (SC.vl) SC.vl.setStreaming(true);
    if (doneTimeout) clearTimeout(doneTimeout);
    doneTimeout = setTimeout(forceFinishIfStale, 30000);
    if (currentAssistantEl) {
      const bubble = currentAssistantEl.querySelector('.msg-bubble');
      if (bubble) {
        bubble.innerHTML = '<span class="streaming-cursor">▍</span>';
        bubble.classList.add('streaming');
      }
    }
    SC.wsSend('chat', { content: content });
    if (typeof SC.recordBranch === 'function') SC.recordBranch(msgIndex, originalMsgs, SC.state.messages.slice(msgIndex));
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
      var el = SC.renderMessageCard('cmd-result msg-group-start', SC.escapeHtml(content), ts, {
        roleLabel: '\u25B6 Command',
        style: 'font-family:var(--font-d);background:var(--bg-2);border:1px solid var(--bd);border-radius:8px;white-space:pre-wrap;word-break:break-word;padding:10px 14px'
      });
      el.dataset.msgIndex = msgIndex;
      el.dataset.msgId = msgId;
      SC.$('#messages').appendChild(el);
    }

    SC.scrollChat();
    SC.state.messages.push({ role: 'cmd_result', content, ts: Date.now() });
    if (typeof SC.emit === 'function') SC.emit('messages', SC.state.messages);

    if (SC.Tabs) {
      var activeTab = SC.Tabs.getActive();
      if (activeTab) {
        activeTab.vlItems = SC.vl ? SC.vl.items.slice() : [];
      }
    }
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
      const skeleton = bubble.querySelector('.skeleton-code');
      if (skeleton) skeleton.remove();
      const thinking = bubble.querySelector('.thinking');
      if (thinking) thinking.remove();

      const thinkingBlockEl = bubble.querySelector('.thinking-block');

      var thinkingClone = thinkingBlockEl ? thinkingBlockEl.cloneNode(true) : null;
      var textEl = document.createElement('span');
      textEl.className = 'streaming-cursor';
      textEl.textContent = currentContent;
      bubble.innerHTML = '';
      if (thinkingClone) bubble.appendChild(thinkingClone);
      bubble.appendChild(textEl);
      bubble.classList.add('streaming');
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
      const skeleton = bubble.querySelector('.skeleton-code');
      if (skeleton) skeleton.remove();
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

    removePhaseIndicator();

    if (currentAssistantIndex >= 0 || currentAssistantEl) {
      var el = SC.vl ? SC.vl.getItemElement(currentAssistantIndex) : currentAssistantEl;
      if (el) {
        const bubble = el.querySelector('.msg-bubble');
        if (bubble) {
          const thinking = bubble.querySelector('.thinking');
          if (thinking) thinking.remove();
          const skeleton = bubble.querySelector('.skeleton-code');
          if (skeleton) skeleton.remove();
          bubble.classList.remove('streaming');
          var savedThinkingBlock = bubble.querySelector('.thinking-block');
          var savedThinkingContent = currentThinking;
          try {
            bubble.innerHTML = renderMarkdown(currentContent);
            bubble.classList.add('rendered');
          } catch (e) {
            console.error('renderMarkdown error:', e);
            bubble.innerHTML = renderPlainText(currentContent);
          }
          if (savedThinkingBlock && savedThinkingContent) {
            wrapReasoningAccordion(savedThinkingBlock, bubble);
          }
          bindCodeCopy(bubble);
          postRenderMarkdown(bubble);
          if (typeof SC.addPreviewButtons === 'function') {
            setTimeout(SC.addPreviewButtons, 200);
          }
        }
        if (el.classList.contains('assistant')) {
          addConfidenceIndicator(el, 0.7);
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

    if (SC.audio && SC.audio.messageReceived) SC.audio.messageReceived();

    if (SC.state.ui.currentSessionId && SC.state.messages.length > 0) {
      const currentSession = (SC.state.sessions || []).find(s => s.id === SC.state.ui.currentSessionId);
      const title = currentSession?.title || '';
      if (!title || title === 'Untitled' || title === '') {
        const firstUserMsg = SC.state.messages.find(m => m.role === 'user');
        if (firstUserMsg) {
          let autoTitle = firstUserMsg.content.trim().replace(/\n/g, ' ');
          if (autoTitle.length > 50) autoTitle = autoTitle.slice(0, 50) + '...';
          if (autoTitle) {
            SC.wsSend('session_rename', { id: SC.state.ui.currentSessionId, title: autoTitle });
            if (SC.Tabs) {
              SC.Tabs.updateSessionTitle(SC.state.ui.currentSessionId, autoTitle);
            }
          }
        }
      }
    }

    if (SC.Tabs) {
      var finishedTab = SC.Tabs.getActive();
      if (finishedTab) {
        finishedTab.vlItems = SC.vl ? SC.vl.items.slice() : [];
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
      } else {
        html = SC.escapeHtml(html);
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

      var isBookmarked = SC.isMessageBookmarked(msgId);
      items.push({
        label: isBookmarked ? 'Remove Bookmark' : 'Bookmark',
        action: function() {
          SC.toggleMessageBookmark(msgId, el);
        }
      });

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

  function initLightbox() {
    document.addEventListener('click', function(e) {
      if (e.target.classList.contains('msg-image')) {
        var overlay = document.createElement('div');
        overlay.className = 'lightbox';
        overlay.addEventListener('click', function() { overlay.remove(); });
        var img = document.createElement('img');
        img.src = e.target.src;
        img.style.maxWidth = '90vw';
        img.style.maxHeight = '90vh';
        img.style.borderRadius = '8px';
        overlay.appendChild(img);
        document.body.appendChild(overlay);
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initLightbox);
  } else {
    initLightbox();
  }

  SC.scrollChat = function() {
    var chat = SC.$('#chat');
    if (!chat) return;
    var isNearBottom = chat.scrollHeight - chat.scrollTop - chat.clientHeight < 120;
    if (isNearBottom) {
      chat.scrollTo({ top: chat.scrollHeight, behavior: 'smooth' });
    }
  };

  var PHASE_STEPS = [
    { id: 'scan', label: 'Scanning' },
    { id: 'analyze', label: 'Analyzing' },
    { id: 'generate', label: 'Generating' }
  ];

  function createPhaseIndicator() {
    var el = document.createElement('div');
    el.className = 'phase-indicator';
    el.id = 'current-phase';
    var html = '';
    PHASE_STEPS.forEach(function(step, i) {
      if (i > 0) html += '<span class="phase-sep">›</span>';
      html += '<span class="phase-step" data-phase="' + step.id + '"><span class="phase-dot"></span><span class="phase-label">' + step.label + '</span></span>';
    });
    el.innerHTML = html;
    return el;
  }

  function setPhase(phaseId) {
    var indicator = SC.$('#current-phase');
    if (!indicator) return;
    var steps = indicator.querySelectorAll('.phase-step');
    var found = false;
    steps.forEach(function(step) {
      if (step.dataset.phase === phaseId) {
        step.classList.add('active');
        step.classList.remove('done');
        found = true;
      } else if (!found) {
        step.classList.add('done');
        step.classList.remove('active');
      } else {
        step.classList.remove('done', 'active');
      }
    });
  }

  function removePhaseIndicator() {
    var el = SC.$('#current-phase');
    if (el) el.remove();
  }

  function wrapReasoningAccordion(thinkingEl, bubble) {
    var content = thinkingEl.querySelector('.thinking-content') || thinkingEl.querySelector('p');
    if (!content) return;
    var text = content.textContent || '';
    var filesMatch = text.match(/(?:reading|scanning|examining|analyzing)\s+[\w.\/\-]+/gi);
    var files = filesMatch ? filesMatch.map(function(f) { return f.replace(/^(reading|scanning|examining|analyzing)\s+/i, ''); }) : [];

    var block = document.createElement('div');
    block.className = 'reasoning-block';
    var header = document.createElement('div');
    header.className = 'reasoning-header';
    header.innerHTML = '<span class="reasoning-chevron">▸</span><span class="reasoning-summary">Reasoning</span>' + (files.length ? ' <span style="color:var(--tx-2);font-size:10px">(' + files.length + ' files)</span>' : '');
    var detail = document.createElement('div');
    detail.className = 'reasoning-detail';
    if (files.length) {
      var filesDiv = document.createElement('div');
      filesDiv.className = 'reasoning-files';
      files.forEach(function(f) {
        var badge = document.createElement('span');
        badge.className = 'reasoning-file';
        badge.textContent = f;
        filesDiv.appendChild(badge);
      });
      detail.appendChild(filesDiv);
    }
    var stepsDiv = document.createElement('div');
    stepsDiv.className = 'reasoning-steps';
    stepsDiv.textContent = text.slice(0, 300) + (text.length > 300 ? '...' : '');
    detail.appendChild(stepsDiv);
    block.appendChild(header);
    block.appendChild(detail);
    header.addEventListener('click', function() { block.classList.toggle('open'); });
    if (bubble) {
      bubble.prepend(block);
    } else if (thinkingEl.parentNode) {
      thinkingEl.parentNode.replaceChild(block, thinkingEl);
    }
  }

  function addConfidenceIndicator(msgEl, confidence) {
    if (!confidence || confidence <= 0) return;
    var bar = document.createElement('div');
    bar.className = 'confidence-bar';
    var level = confidence < 0.5 ? 'low' : confidence < 0.8 ? 'medium' : 'high';
    var label = level === 'low' ? 'Low confidence' : level === 'medium' ? 'Medium confidence' : 'High confidence';
    bar.innerHTML = '<div class="confidence-track"><div class="confidence-fill ' + level + '" style="width:' + Math.round(confidence * 100) + '%"></div></div><span class="confidence-label">' + label + '</span>';
    var bubble = msgEl.querySelector('.msg-bubble');
    if (bubble) bubble.appendChild(bar);
  }

  function createCodeSkeleton() {
    var el = document.createElement('div');
    el.className = 'skeleton-code';
    el.innerHTML = '<div class="skeleton-line"></div><div class="skeleton-line"></div><div class="skeleton-indent"><div class="skeleton-line"></div><div class="skeleton-line"></div></div><div class="skeleton-line"></div><div class="skeleton-line"></div>';
    return el;
  }

  function createTextSkeleton() {
    var el = document.createElement('div');
    el.className = 'skeleton-text';
    el.innerHTML = '<div class="skeleton-line"></div><div class="skeleton-line"></div><div class="skeleton-line"></div><div class="skeleton-line"></div>';
    return el;
  }

  function createChartSkeleton() {
    var el = document.createElement('div');
    el.className = 'skeleton-chart';
    var heights = [40, 65, 80, 55, 90, 70, 45];
    heights.forEach(function(h) {
      var bar = document.createElement('div');
      bar.className = 'skeleton-chart-bar';
      bar.style.height = h + '%';
      el.appendChild(bar);
    });
    return el;
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
  SC.createCodeSkeleton = createCodeSkeleton;
  SC.createTextSkeleton = createTextSkeleton;
  SC.createChartSkeleton = createChartSkeleton;
  SC.createPhaseIndicator = createPhaseIndicator;
  SC.setPhase = setPhase;
  SC.removePhaseIndicator = removePhaseIndicator;
  SC.wrapReasoningAccordion = wrapReasoningAccordion;
  SC.addConfidenceIndicator = addConfidenceIndicator;

  var BOOKMARK_KEY = 'smartclaw-msg-bookmarks';

  function getMessageBookmarks() {
    try { return JSON.parse(localStorage.getItem(BOOKMARK_KEY) || '[]'); } catch(e) { return []; }
  }

  function saveMessageBookmarks(bookmarks) {
    try { localStorage.setItem(BOOKMARK_KEY, JSON.stringify(bookmarks)); } catch(e) {}
  }

  function isMessageBookmarked(msgId) {
    return getMessageBookmarks().some(function(b) { return b.msgId === msgId; });
  }

  function toggleMessageBookmark(msgId, msgEl) {
    var bookmarks = getMessageBookmarks();
    var idx = bookmarks.findIndex(function(b) { return b.msgId === msgId; });

    if (idx >= 0) {
      bookmarks.splice(idx, 1);
      if (msgEl) msgEl.classList.remove('bookmarked');
      SC.toast('Bookmark removed', 'info');
    } else {
      var bubble = msgEl ? msgEl.querySelector('.msg-bubble') : null;
      var content = bubble ? bubble.textContent.slice(0, 200) : '';
      var role = msgEl ? (msgEl.classList.contains('user') ? 'user' : msgEl.classList.contains('assistant') ? 'assistant' : 'other') : 'unknown';
      bookmarks.unshift({
        msgId: msgId,
        content: content,
        role: role,
        timestamp: Date.now(),
        sessionId: SC.state.ui.currentSessionId || ''
      });
      if (msgEl) msgEl.classList.add('bookmarked');
      SC.toast('Message bookmarked', 'success');
    }

    saveMessageBookmarks(bookmarks);
    renderBookmarkBadge();
  }

  function renderBookmarkBadge() {
    var count = getMessageBookmarks().length;
    var badge = SC.$('#bookmark-count');
    if (badge) {
      badge.textContent = count;
      badge.style.display = count > 0 ? '' : 'none';
      badge.classList.remove('bounce');
      void badge.offsetWidth;
      badge.classList.add('bounce');
    }
  }

  SC.isMessageBookmarked = isMessageBookmarked;
  SC.toggleMessageBookmark = toggleMessageBookmark;
  SC.getMessageBookmarks = getMessageBookmarks;
  SC.renderBookmarkBadge = renderBookmarkBadge;

  SC.startBatchLoad = function(count) {
    batchLoadCounter = 1;
  };
  SC.endBatchLoad = function() {
    batchLoadCounter = 0;
  };
})();
