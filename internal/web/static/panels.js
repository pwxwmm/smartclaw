// SmartClaw - Multi-Panel Split View
(function() {
  'use strict';

  var activePanel = null;
  var panels = {};
  var panelWidth = 380;
  var snapPosition = 'snap-right';
  var LAYOUT_KEY = 'smartclaw-panel-layout';
  var indicator = null;
  var dragState = null;
  var panelHandlers = { dragMouseMove: null, dragMouseUp: null, divMouseMove: null, divMouseUp: null };

  function detectSnap(x, y, w, h) {
    var margin = 80;
    if (x > w - margin) return 'snap-right';
    if (x < margin + 48) return 'snap-left';
    if (y > h - margin) return 'snap-bottom';
    return null;
  }

  function saveLayout() {
    try {
      var layout = { snap: snapPosition, panels: Object.keys(panels) };
      localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout));
    } catch(e) {}
  }

  function loadLayout() {
    try {
      var saved = JSON.parse(localStorage.getItem(LAYOUT_KEY) || '{}');
      if (saved.snap) snapPosition = saved.snap;
      return saved;
    } catch(e) { return {}; }
  }

  function applySnapPosition(panelArea, snap) {
    if (!panelArea) return;
    panelArea.classList.remove('snap-right', 'snap-left', 'snap-bottom');
    panelArea.classList.add(snap);
    snapPosition = snap;
  }

  function createPanel(id, title, iconSvg) {
    if (panels[id]) {
      togglePanel(id);
      return;
    }

    var panelArea = SC.$('#panel-area');
    if (!panelArea) return;

    var panel = document.createElement('div');
    panel.className = 'split-panel';
    panel.dataset.panelId = id;

    var header = document.createElement('div');
    header.className = 'split-panel-header';

    var dragHandle = document.createElement('span');
    dragHandle.className = 'panel-drag-handle';
    dragHandle.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="9" cy="5" r="1"/><circle cx="15" cy="5" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="19" r="1"/><circle cx="15" cy="19" r="1"/></svg>';
    header.appendChild(dragHandle);

    var titleSpan = document.createElement('span');
    titleSpan.className = 'split-panel-title';
    titleSpan.textContent = title;
    header.appendChild(titleSpan);

    var closeBtn = document.createElement('button');
    closeBtn.className = 'icon-btn sm split-panel-close';
    closeBtn.dataset.panelId = id;
    closeBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';
    header.appendChild(closeBtn);

    var body = document.createElement('div');
    body.className = 'split-panel-body';
    body.id = 'panel-body-' + id;

    panel.appendChild(header);
    panel.appendChild(body);
    panelArea.appendChild(panel);

    panels[id] = { el: panel, title: title };
    activePanel = id;

    applySnapPosition(panelArea, snapPosition);
    panelArea.classList.add('visible');
    document.body.classList.add('panel-active');
    updatePanelTabs();

    closeBtn.addEventListener('click', function() {
      closePanel(this.dataset.panelId);
    });

    dragHandle.addEventListener('mousedown', function(e) {
      e.preventDefault();
      var startX = e.clientX;
      var startY = e.clientY;
      var startSnap = snapPosition;
      dragState = { startX: startX, startY: startY, startSnap: startSnap, moved: false };
      document.addEventListener('mousemove', panelHandlers.dragMouseMove);
      document.addEventListener('mouseup', panelHandlers.dragMouseUp);
    });

    if (SC.audio && SC.audio.click) SC.audio.click();
    saveLayout();
  }

  function closePanel(id) {
    if (!panels[id]) return;
    panels[id].el.remove();
    delete panels[id];

    var remaining = Object.keys(panels);
    if (remaining.length === 0) {
      var panelArea = SC.$('#panel-area');
      if (panelArea) panelArea.classList.remove('visible');
      document.body.classList.remove('panel-active');
      activePanel = null;
    } else {
      activePanel = remaining[remaining.length - 1];
      showPanel(activePanel);
    }
    updatePanelTabs();
    saveLayout();
  }

  function togglePanel(id) {
    if (panels[id]) {
      if (activePanel === id) {
        closePanel(id);
      } else {
        showPanel(id);
      }
    }
  }

  function showPanel(id) {
    if (!panels[id]) return;
    activePanel = id;
    Object.keys(panels).forEach(function(key) {
      panels[key].el.classList.toggle('active', key === id);
    });
    updatePanelTabs();
  }

  function updatePanelTabs() {
    var tabBar = SC.$('#panel-tabs');
    if (!tabBar) return;
    var html = '';
    Object.keys(panels).forEach(function(id) {
      html += '<button class="panel-tab' + (id === activePanel ? ' active' : '') + '" data-panel-id="' + SC.escapeHtml(id) + '">' + SC.escapeHtml(panels[id].title) + '</button>';
    });
    tabBar.innerHTML = html;
    tabBar.querySelectorAll('.panel-tab').forEach(function(tab) {
      tab.addEventListener('click', function() {
        showPanel(this.dataset.panelId);
      });
    });
  }

  function initPanels() {
    var saved = loadLayout();

    indicator = document.createElement('div');
    indicator.className = 'panel-snap-indicator';
    indicator.id = 'panel-snap-indicator';
    document.body.appendChild(indicator);

    var panelArea = SC.$('#panel-area');
    if (panelArea && saved.snap) {
      applySnapPosition(panelArea, saved.snap);
    }

    panelHandlers.dragMouseMove = function(e) {
      if (!dragState) return;
      var dx = e.clientX - dragState.startX;
      var dy = e.clientY - dragState.startY;
      if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
        dragState.moved = true;
      }
      if (!dragState.moved) return;

      var w = window.innerWidth;
      var h = window.innerHeight;
      var snap = detectSnap(e.clientX, e.clientY, w, h);

      if (indicator) {
        indicator.classList.remove('snap-right', 'snap-left', 'snap-bottom');
        if (snap) {
          indicator.classList.add(snap);
          indicator.classList.add('active');
        } else {
          indicator.classList.remove('active');
        }
      }
    };

    panelHandlers.dragMouseUp = function(e) {
      document.removeEventListener('mousemove', panelHandlers.dragMouseMove);
      document.removeEventListener('mouseup', panelHandlers.dragMouseUp);
      if (!dragState) return;
      if (dragState.moved) {
        var w = window.innerWidth;
        var h = window.innerHeight;
        var snap = detectSnap(e.clientX, e.clientY, w, h);
        var panelArea = SC.$('#panel-area');
        if (snap && panelArea) {
          applySnapPosition(panelArea, snap);
          saveLayout();
        }
        if (indicator) {
          indicator.classList.remove('active', 'snap-right', 'snap-left', 'snap-bottom');
        }
      }
      dragState = null;
    };

    var divider = SC.$('#panel-divider');
    if (divider) {
      var dragging = false;
      divider.addEventListener('mousedown', function(e) {
        dragging = true;
        e.preventDefault();
        document.addEventListener('mousemove', panelHandlers.divMouseMove);
        document.addEventListener('mouseup', panelHandlers.divMouseUp);
      });
      panelHandlers.divMouseMove = function(e) {
        if (!dragging) return;
        var pa = SC.$('#panel-area');
        if (!pa) return;
        if (snapPosition === 'snap-right') {
          var w = window.innerWidth;
          var width = w - e.clientX;
          width = Math.max(280, Math.min(width, w * 0.6));
          pa.style.width = width + 'px';
        } else if (snapPosition === 'snap-left') {
          var width = e.clientX - 48;
          width = Math.max(280, Math.min(width, window.innerWidth * 0.6));
          pa.style.width = width + 'px';
        } else if (snapPosition === 'snap-bottom') {
          var height = window.innerHeight - e.clientY;
          height = Math.max(180, Math.min(height, window.innerHeight * 0.6));
          pa.style.height = height + 'px';
        }
      };
      panelHandlers.divMouseUp = function() {
        dragging = false;
        document.removeEventListener('mousemove', panelHandlers.divMouseMove);
        document.removeEventListener('mouseup', panelHandlers.divMouseUp);
      };
    }

    SC.$$('.panel-toggle-btn').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var id = this.dataset.panel;
        var title = this.dataset.panelTitle || id;
        createPanel(id, title);
      });
    });
  }

  SC.createPanel = createPanel;
  SC.closePanel = closePanel;
  SC.togglePanel = togglePanel;
  SC.showPanel = showPanel;
  SC.initPanels = initPanels;
  SC.destroyPanels = function() {
    if (panelHandlers.dragMouseMove) document.removeEventListener('mousemove', panelHandlers.dragMouseMove);
    if (panelHandlers.dragMouseUp) document.removeEventListener('mouseup', panelHandlers.dragMouseUp);
    if (panelHandlers.divMouseMove) document.removeEventListener('mousemove', panelHandlers.divMouseMove);
    if (panelHandlers.divMouseUp) document.removeEventListener('mouseup', panelHandlers.divMouseUp);
  };
})();
