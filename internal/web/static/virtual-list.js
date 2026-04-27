// SmartClaw - Virtual List
// Renders only visible chat messages in the DOM for 500+ message performance.
(function() {
  'use strict';
  
  var ITEM_GAP = 20; // px gap between messages
  
  function VirtualList(container, options) {
    this.container = container;                                      // #messages
    this.scrollContainer = options.scrollContainer || container.parentElement; // #chat
    this.items = [];
    this.itemHeights = [];
    this.estimatedHeight = 100;  // includes gap
    this.overscan = 5;
    this.renderItem = options.renderItem || function() { return null; };
    this.onScrollTop = options.onScrollTop || null;
    this.welcomeEl = options.welcomeEl || null;
    
    this.scrollTop = 0;
    this.viewportHeight = 0;
    this.totalHeight = 0;
    this._renderScheduled = false;
    this._streaming = false;  // suppress scroll-triggered re-renders during streaming
    
    // Create spacer elements
    this.topSpacer = document.createElement('div');
    this.topSpacer.className = 'vl-top-spacer';
    this.topSpacer.setAttribute('aria-hidden', 'true');
    this.bottomSpacer = document.createElement('div');
    this.bottomSpacer.className = 'vl-bottom-spacer';
    this.bottomSpacer.setAttribute('aria-hidden', 'true');
    
    // Clear container and add spacers
    this.container.innerHTML = '';
    this.container.appendChild(this.topSpacer);
    this.container.appendChild(this.bottomSpacer);
    
    // Bind scroll
    this._onScroll = this._handleScroll.bind(this);
    this.scrollContainer.addEventListener('scroll', this._onScroll, { passive: true });
    
    // Resize observer
    this._resizeObserver = new ResizeObserver(this._handleResize.bind(this));
    this._resizeObserver.observe(this.scrollContainer);
    
    this.viewportHeight = this.scrollContainer.clientHeight;
    this._updateWelcome();
  }
  
  // --- Item management ---
  
  VirtualList.prototype.setItems = function(items) {
    this.items = items;
    this.itemHeights = new Array(items.length).fill(this.estimatedHeight);
    this.totalHeight = this.itemHeights.reduce(function(a, b) { return a + b; }, 0);
    this._updateWelcome();
    this._render();
  };
  
  VirtualList.prototype.addItem = function(item) {
    var wasNearBottom = this.isNearBottom(150);
    this.items.push(item);
    this.itemHeights.push(this.estimatedHeight);
    this.totalHeight += this.estimatedHeight;
    this._updateWelcome();
    this._render();
    if (wasNearBottom) {
      this.scrollToBottom();
    }
  };
  
  VirtualList.prototype.removeItemAt = function(index) {
    if (index < 0 || index >= this.items.length) return;
    var removedHeight = this.itemHeights[index];
    this.items.splice(index, 1);
    this.itemHeights.splice(index, 1);
    this.totalHeight -= removedHeight;
    if (this.totalHeight < 0) this.totalHeight = 0;
    this._updateWelcome();
    this._render();
  };
  
  VirtualList.prototype.removeItemsAt = function(index, count) {
    if (index < 0 || index >= this.items.length) return;
    count = Math.min(count, this.items.length - index);
    if (count <= 0) return;
    var removedHeights = this.itemHeights.splice(index, count);
    var removedTotal = removedHeights.reduce(function(a, b) { return a + b; }, 0);
    this.items.splice(index, count);
    this.totalHeight -= removedTotal;
    if (this.totalHeight < 0) this.totalHeight = 0;
    this._updateWelcome();
    this._render();
  };
  
  VirtualList.prototype.removeItemsFrom = function(index) {
    if (index < 0 || index >= this.items.length) return;
    var removedHeights = this.itemHeights.splice(index);
    var removedTotal = removedHeights.reduce(function(a, b) { return a + b; }, 0);
    this.items.splice(index);
    this.totalHeight -= removedTotal;
    if (this.totalHeight < 0) this.totalHeight = 0;
    this._updateWelcome();
    this._render();
  };
  
  VirtualList.prototype.updateItem = function(index, item) {
    if (index < 0 || index >= this.items.length) return;
    this.items[index] = item;
    // Re-render only if the element is currently in the DOM
    var el = this._getItemElement(index);
    if (el) {
      var newEl = this.renderItem(item, index);
      if (newEl) {
        newEl.dataset.vlIndex = index;
        newEl.style.marginBottom = ITEM_GAP + 'px';
        // Preserve cached height info from old element
        if (el.offsetHeight > 0) {
          this.itemHeights[index] = el.offsetHeight + ITEM_GAP;
        }
        this.container.replaceChild(newEl, el);
        requestAnimationFrame(this._updateSingleHeight.bind(this, index, newEl));
      }
    }
    // If not in DOM, data is updated; next _render will create correct element
  };
  
  VirtualList.prototype.clear = function() {
    this.items = [];
    this.itemHeights = [];
    this.totalHeight = 0;
    this._streaming = false;
    this._updateWelcome();
    this._render();
  };
  
  // --- Streaming support ---
  
  VirtualList.prototype.setStreaming = function(streaming) {
    this._streaming = !!streaming;
  };
  
  VirtualList.prototype.refreshItemHeight = function(index) {
    var el = this._getItemElement(index);
    if (el && el.offsetHeight > 0) {
      var oldH = this.itemHeights[index];
      var newH = el.offsetHeight + ITEM_GAP;
      if (oldH !== newH) {
        this.itemHeights[index] = newH;
        this.totalHeight += (newH - oldH);
      }
    }
  };
  
  // --- DOM queries ---
  
  VirtualList.prototype._getItemElement = function(index) {
    return this.container.querySelector('[data-vl-index="' + index + '"]');
  };
  
  VirtualList.prototype.getItemElement = function(index) {
    return this._getItemElement(index);
  };
  
  // --- Scroll ---
  
  VirtualList.prototype.isNearBottom = function(threshold) {
    threshold = threshold || 100;
    var sc = this.scrollContainer;
    return sc.scrollHeight - sc.scrollTop - sc.clientHeight < threshold;
  };
  
  VirtualList.prototype.scrollToBottom = function() {
    var sc = this.scrollContainer;
    sc.scrollTop = sc.scrollHeight;
  };
  
  VirtualList.prototype.scrollToItem = function(index) {
    var offset = 0;
    for (var i = 0; i < index; i++) {
      offset += this.itemHeights[i];
    }
    this.scrollContainer.scrollTop = offset;
  };
  
  // --- Internal ---
  
  VirtualList.prototype._updateWelcome = function() {
    if (this.welcomeEl) {
      if (this.items.length === 0) {
        this.welcomeEl.classList.remove('hidden');
      } else {
        this.welcomeEl.classList.add('hidden');
      }
    }
  };
  
  VirtualList.prototype._handleScroll = function() {
    this.scrollTop = this.scrollContainer.scrollTop;
    if (this.scrollTop < 50 && this.onScrollTop) {
      this.onScrollTop();
    }
    // Throttle re-renders during streaming to avoid destroying the streaming element
    if (!this._streaming) {
      this._scheduleRender();
    }
  };
  
  VirtualList.prototype._handleResize = function() {
    this.viewportHeight = this.scrollContainer.clientHeight;
    this._scheduleRender();
  };
  
  VirtualList.prototype._scheduleRender = function() {
    if (this._renderScheduled) return;
    this._renderScheduled = true;
    var self = this;
    requestAnimationFrame(function() {
      self._renderScheduled = false;
      self._render();
    });
  };
  
  VirtualList.prototype._getVisibleRange = function() {
    if (this.items.length === 0) return { start: 0, end: -1 };
    
    var scrollTop = this.scrollTop;
    var viewportBottom = scrollTop + this.viewportHeight;
    
    // Binary-ish search for start index
    var startIndex = 0;
    var offset = 0;
    var found = false;
    for (var i = 0; i < this.items.length; i++) {
      if (offset + this.itemHeights[i] > scrollTop) {
        startIndex = i;
        found = true;
        break;
      }
      offset += this.itemHeights[i];
    }
    if (!found) startIndex = this.items.length - 1;
    
    // Find end index
    var endIndex = startIndex;
    for (var j = startIndex; j < this.items.length; j++) {
      if (offset >= viewportBottom) {
        endIndex = j - 1;
        break;
      }
      offset += this.itemHeights[j];
      endIndex = j;
    }
    
    // Add overscan
    startIndex = Math.max(0, startIndex - this.overscan);
    endIndex = Math.min(this.items.length - 1, endIndex + this.overscan);
    
    return { start: startIndex, end: endIndex };
  };
  
  VirtualList.prototype._render = function() {
    if (this.items.length === 0) {
      this.topSpacer.style.height = '0px';
      this.bottomSpacer.style.height = '0px';
      this._removeAllRenderedItems();
      return;
    }
    
    var range = this._getVisibleRange();
    if (range.start > range.end) {
      this.topSpacer.style.height = '0px';
      this.bottomSpacer.style.height = this.totalHeight + 'px';
      this._removeAllRenderedItems();
      return;
    }
    
    // Calculate top offset
    var topOffset = 0;
    for (var i = 0; i < range.start; i++) {
      topOffset += this.itemHeights[i];
    }
    
    // Calculate bottom offset
    var bottomOffset = 0;
    for (var j = range.end + 1; j < this.items.length; j++) {
      bottomOffset += this.itemHeights[j];
    }
    
    this.topSpacer.style.height = topOffset + 'px';
    this.bottomSpacer.style.height = bottomOffset + 'px';
    
    // Collect currently rendered indices
    var rendered = {};
    var children = this.container.children;
    for (var k = 0; k < children.length; k++) {
      var child = children[k];
      if (child === this.topSpacer || child === this.bottomSpacer) continue;
      var idx = parseInt(child.dataset.vlIndex);
      if (!isNaN(idx)) {
        rendered[idx] = child;
      }
    }
    
    // Determine which indices should be visible
    var visibleSet = {};
    for (var m = range.start; m <= range.end; m++) {
      visibleSet[m] = true;
    }
    
    // Remove elements no longer in range
    var renderedKeys = Object.keys(rendered);
    for (var r = 0; r < renderedKeys.length; r++) {
      var rIdx = parseInt(renderedKeys[r]);
      if (!visibleSet[rIdx]) {
        var oldEl = rendered[rIdx];
        // Cache height before removing
        if (oldEl.offsetHeight > 0) {
          this.itemHeights[rIdx] = oldEl.offsetHeight + ITEM_GAP;
        }
        this.container.removeChild(oldEl);
        delete rendered[rIdx];
      }
    }
    
    // Create and insert new elements in correct order
    var insertBefore = this.bottomSpacer;
    for (var n = range.end; n >= range.start; n--) {
      if (rendered[n]) {
        // Element already exists; ensure it's before insertBefore
        var existingEl = rendered[n];
        if (existingEl.nextSibling !== insertBefore) {
          this.container.insertBefore(existingEl, insertBefore);
        }
        insertBefore = existingEl;
      } else {
        // Create new element
        var newEl = this.renderItem(this.items[n], n);
        if (newEl) {
          newEl.dataset.vlIndex = n;
          newEl.style.marginBottom = ITEM_GAP + 'px';
          this.container.insertBefore(newEl, insertBefore);
          insertBefore = newEl;
        }
      }
    }
    
    // Update cached heights after render
    requestAnimationFrame(this._updateHeights.bind(this));
  };
  
  VirtualList.prototype._removeAllRenderedItems = function() {
    var children = this.container.children;
    for (var k = children.length - 1; k >= 0; k--) {
      var child = children[k];
      if (child !== this.topSpacer && child !== this.bottomSpacer) {
        var idx = parseInt(child.dataset.vlIndex);
        if (!isNaN(idx) && child.offsetHeight > 0) {
          this.itemHeights[idx] = child.offsetHeight + ITEM_GAP;
        }
        this.container.removeChild(child);
      }
    }
  };
  
  VirtualList.prototype._updateSingleHeight = function(index, el) {
    if (el && el.offsetHeight > 0) {
      var oldH = this.itemHeights[index];
      var newH = el.offsetHeight + ITEM_GAP;
      if (oldH !== newH) {
        this.itemHeights[index] = newH;
        this.totalHeight += (newH - oldH);
      }
    }
  };
  
  VirtualList.prototype._updateHeights = function() {
    var children = this.container.children;
    var changed = false;
    for (var i = 0; i < children.length; i++) {
      var child = children[i];
      if (child === this.topSpacer || child === this.bottomSpacer) continue;
      var idx = parseInt(child.dataset.vlIndex);
      if (!isNaN(idx) && child.offsetHeight > 0) {
        var oldH = this.itemHeights[idx];
        var newH = child.offsetHeight + ITEM_GAP;
        if (oldH !== newH) {
          this.itemHeights[idx] = newH;
          this.totalHeight += (newH - oldH);
          changed = true;
        }
      }
    }
  };
  
  VirtualList.prototype.destroy = function() {
    this.scrollContainer.removeEventListener('scroll', this._onScroll);
    this._resizeObserver.disconnect();
  };
  
  SC.VirtualList = VirtualList;
})();
