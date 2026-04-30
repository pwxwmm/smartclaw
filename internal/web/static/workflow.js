(function() {
  'use strict';

  var CATEGORY_COLORS = {
    file: '#3b82f6', code: '#8b5cf6', web: '#06b6d4',
    git: '#f59e0b', agent: '#22c55e', docker: '#ef4444', flow: '#94a3b8'
  };

  var GRID_SIZE = 20;
  var NODE_W = 180;
  var NODE_H = 60;
  var PORT_R = 6;
  var COND_SIZE = 70;

  function WorkflowBuilder(container) {
    this.container = typeof container === 'string' ? document.querySelector(container) : container;
    this.nodes = [];
    this.connections = [];
    this.tools = [];
    this.selectedNode = null;
    this.dragging = null;
    this.connecting = null;
    this.panX = 0;
    this.panY = 0;
    this.zoom = 1;
    this.isPanning = false;
    this.panStart = null;
    this.undoStack = [];
    this.redoStack = [];
    this.nextId = 1;
    this.currentWorkflow = null;
    this.svg = null;
    this.canvasGroup = null;
    this.nodeGroup = null;
    this.connGroup = null;
    this.tempConn = null;
  }

  WorkflowBuilder.prototype.init = function(tools) {
    this.tools = tools || [];
    this.buildDOM();
    this.bindEvents();
    this.loadWorkflowList();
  };

  WorkflowBuilder.prototype.buildDOM = function() {
    var self = this;
    this.container.innerHTML = '';

    var wrapper = document.createElement('div');
    wrapper.className = 'wf-wrapper';

    var palette = document.createElement('div');
    palette.className = 'wf-palette';
    this.buildPalette(palette);
    wrapper.appendChild(palette);

    var main = document.createElement('div');
    main.className = 'wf-main';

    var toolbar = document.createElement('div');
    toolbar.className = 'wf-toolbar';
    this.buildToolbar(toolbar);
    main.appendChild(toolbar);

    var canvasWrap = document.createElement('div');
    canvasWrap.className = 'wf-canvas-wrap';

    this.svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    this.svg.setAttribute('class', 'wf-canvas');
    this.svg.setAttribute('width', '100%');
    this.svg.setAttribute('height', '100%');

    var defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
    var marker = document.createElementNS('http://www.w3.org/2000/svg', 'marker');
    marker.setAttribute('id', 'wf-arrow');
    marker.setAttribute('markerWidth', '8');
    marker.setAttribute('markerHeight', '6');
    marker.setAttribute('refX', '8');
    marker.setAttribute('refY', '3');
    marker.setAttribute('orient', 'auto');
    var poly = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
    poly.setAttribute('points', '0 0, 8 3, 0 6');
    poly.setAttribute('fill', 'var(--tx-2, #6a6a72)');
    marker.appendChild(poly);
    defs.appendChild(marker);

    var gridPat = document.createElementNS('http://www.w3.org/2000/svg', 'pattern');
    gridPat.setAttribute('id', 'wf-grid');
    gridPat.setAttribute('width', String(GRID_SIZE));
    gridPat.setAttribute('height', String(GRID_SIZE));
    gridPat.setAttribute('patternUnits', 'userSpaceOnUse');
    var gridDot = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    gridDot.setAttribute('cx', '1');
    gridDot.setAttribute('cy', '1');
    gridDot.setAttribute('r', '0.8');
    gridDot.setAttribute('fill', 'var(--bd, #26262a)');
    gridPat.appendChild(gridDot);
    defs.appendChild(gridPat);

    var gridRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    gridRect.setAttribute('width', '100%');
    gridRect.setAttribute('height', '100%');
    gridRect.setAttribute('fill', 'url(#wf-grid)');
    defs.appendChild(gridRect);
    this.svg.appendChild(defs);

    this.canvasGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    this.canvasGroup.setAttribute('class', 'wf-canvas-group');

    var bgRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    bgRect.setAttribute('x', '-5000');
    bgRect.setAttribute('y', '-5000');
    bgRect.setAttribute('width', '10000');
    bgRect.setAttribute('height', '10000');
    bgRect.setAttribute('fill', 'url(#wf-grid)');
    this.canvasGroup.appendChild(bgRect);

    this.connGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    this.canvasGroup.appendChild(this.connGroup);

    this.nodeGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    this.canvasGroup.appendChild(this.nodeGroup);

    this.svg.appendChild(this.canvasGroup);
    canvasWrap.appendChild(this.svg);
    main.appendChild(canvasWrap);

    var props = document.createElement('div');
    props.className = 'wf-props';
    props.id = 'wf-props';
    props.innerHTML = '<div class="wf-props-empty">Select a node to edit</div>';
    main.appendChild(props);

    wrapper.appendChild(main);
    this.container.appendChild(wrapper);

    var list = document.createElement('div');
    list.className = 'wf-list';
    list.id = 'wf-list';
    this.container.appendChild(list);
  };

  WorkflowBuilder.prototype.buildPalette = function(palette) {
    var self = this;
    var categories = {};
    this.tools.forEach(function(t) {
      if (!categories[t.category]) categories[t.category] = [];
      categories[t.category].push(t);
    });

    var header = document.createElement('div');
    header.className = 'wf-palette-header';
    header.textContent = 'Tools';
    palette.appendChild(header);

    Object.keys(categories).forEach(function(cat) {
      var group = document.createElement('div');
      group.className = 'wf-palette-group';

      var label = document.createElement('div');
      label.className = 'wf-palette-label';
      label.textContent = cat.charAt(0).toUpperCase() + cat.slice(1);
      label.style.borderLeftColor = CATEGORY_COLORS[cat] || '#94a3b8';
      group.appendChild(label);

      categories[cat].forEach(function(tool) {
        var item = document.createElement('div');
        item.className = 'wf-palette-item';
        item.setAttribute('draggable', 'true');
        item.setAttribute('data-tool', tool.name);
        item.setAttribute('data-category', tool.category);
        item.textContent = tool.name;
        item.title = tool.description;

        item.addEventListener('dragstart', function(e) {
          e.dataTransfer.setData('text/plain', JSON.stringify(tool));
          e.dataTransfer.effectAllowed = 'copy';
        });

        group.appendChild(item);
      });

      palette.appendChild(group);
    });

    var flowGroup = document.createElement('div');
    flowGroup.className = 'wf-palette-group';
    var flowLabel = document.createElement('div');
    flowLabel.className = 'wf-palette-label';
    flowLabel.textContent = 'Flow';
    flowLabel.style.borderLeftColor = CATEGORY_COLORS.flow;
    flowGroup.appendChild(flowLabel);

    ['start', 'end', 'condition'].forEach(function(type) {
      var item = document.createElement('div');
      item.className = 'wf-palette-item wf-palette-flow';
      item.setAttribute('draggable', 'true');
      item.setAttribute('data-tool', type);
      item.setAttribute('data-category', 'flow');
      item.textContent = type;
      item.addEventListener('dragstart', function(e) {
        e.dataTransfer.setData('text/plain', JSON.stringify({
          name: type, category: 'flow', description: type + ' node', inputs: []
        }));
        e.dataTransfer.effectAllowed = 'copy';
      });
      flowGroup.appendChild(item);
    });
    palette.appendChild(flowGroup);
  };

  WorkflowBuilder.prototype.buildToolbar = function(toolbar) {
    var self = this;

    var btns = [
      { label: 'New', action: function() { self.newWorkflow(); } },
      { label: 'Save', action: function() { self.saveWorkflow(); } },
      { label: 'Save As', action: function() { self.saveAsWorkflow(); } },
      { label: 'Delete', action: function() { self.deleteCurrentWorkflow(); } },
      { label: '|', action: null },
      { label: 'Undo', action: function() { self.undo(); } },
      { label: 'Redo', action: function() { self.redo(); } },
      { label: '|', action: null },
      { label: 'Run', action: function() { self.runWorkflow(); }, cls: 'wf-btn-run' },
      { label: '|', action: null },
      { label: 'Zoom +', action: function() { self.setZoom(self.zoom + 0.1); } },
      { label: 'Zoom -', action: function() { self.setZoom(self.zoom - 0.1); } },
      { label: 'Fit', action: function() { self.fitView(); } },
    ];

    btns.forEach(function(b) {
      if (b.label === '|') {
        var sep = document.createElement('div');
        sep.className = 'wf-toolbar-sep';
        toolbar.appendChild(sep);
        return;
      }
      var btn = document.createElement('button');
      btn.className = 'wf-toolbar-btn' + (b.cls ? ' ' + b.cls : '');
      btn.textContent = b.label;
      btn.addEventListener('click', b.action);
      toolbar.appendChild(btn);
    });

    var nameSpan = document.createElement('span');
    nameSpan.className = 'wf-toolbar-name';
    nameSpan.id = 'wf-toolbar-name';
    nameSpan.textContent = '';
    toolbar.appendChild(nameSpan);
  };

  WorkflowBuilder.prototype.bindEvents = function() {
    var self = this;
    var svgEl = this.svg;

    svgEl.addEventListener('dragover', function(e) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
    });

    svgEl.addEventListener('drop', function(e) {
      e.preventDefault();
      try {
        var tool = JSON.parse(e.dataTransfer.getData('text/plain'));
        var pt = self.screenToCanvas(e.clientX, e.clientY);
        self.addNode(tool, pt.x, pt.y);
      } catch (ex) {}
    });

    svgEl.addEventListener('mousedown', function(e) {
      if (e.target === svgEl || e.target.tagName === 'rect') {
        if (e.button === 0) {
          self.isPanning = true;
          self.panStart = { x: e.clientX - self.panX, y: e.clientY - self.panY };
          svgEl.style.cursor = 'grabbing';
          window.addEventListener('mousemove', self._onMouseMove);
          window.addEventListener('mouseup', self._onMouseUp);
        }
      }
    });

    this._onMouseMove = function(e) {
      if (self.isPanning) {
        self.panX = e.clientX - self.panStart.x;
        self.panY = e.clientY - self.panStart.y;
        self.updateTransform();
      }
      if (self.dragging) {
        var pt = self.screenToCanvas(e.clientX, e.clientY);
        self.dragging.x = Math.round(pt.x / GRID_SIZE) * GRID_SIZE;
        self.dragging.y = Math.round(pt.y / GRID_SIZE) * GRID_SIZE;
        self.render();
      }
      if (self.connecting) {
        var pt2 = self.screenToCanvas(e.clientX, e.clientY);
        self.drawTempConnection(self.connecting, pt2);
      }
    };

    this._onMouseUp = function(e) {
      if (self.isPanning) {
        self.isPanning = false;
        svgEl.style.cursor = '';
      }
      if (self.dragging) {
        self.pushUndo();
        self.dragging = null;
      }
      if (self.connecting) {
        self.connecting = null;
        self.removeTempConnection();
      }
      window.removeEventListener('mousemove', self._onMouseMove);
      window.removeEventListener('mouseup', self._onMouseUp);
    };

    svgEl.addEventListener('wheel', function(e) {
      e.preventDefault();
      var delta = e.deltaY > 0 ? -0.05 : 0.05;
      self.setZoom(self.zoom + delta);
    }, { passive: false });

    svgEl.addEventListener('click', function(e) {
      var nodeEl = e.target.closest('[data-node-id]');
      if (nodeEl) {
        var nid = nodeEl.getAttribute('data-node-id');
        self.selectNode(nid);
      } else {
        self.selectNode(null);
      }
    });

    svgEl.addEventListener('contextmenu', function(e) {
      e.preventDefault();
      var connEl = e.target.closest('[data-conn-id]');
      if (connEl) {
        var cid = connEl.getAttribute('data-conn-id');
        self.deleteConnection(cid);
      }
    });
  };

  WorkflowBuilder.prototype.destroy = function() {
    if (this._onMouseMove) {
      window.removeEventListener('mousemove', this._onMouseMove);
      this._onMouseMove = null;
    }
    if (this._onMouseUp) {
      window.removeEventListener('mouseup', this._onMouseUp);
      this._onMouseUp = null;
    }
  };

  WorkflowBuilder.prototype.screenToCanvas = function(cx, cy) {
    var rect = this.svg.getBoundingClientRect();
    return {
      x: (cx - rect.left - this.panX) / this.zoom,
      y: (cy - rect.top - this.panY) / this.zoom
    };
  };

  WorkflowBuilder.prototype.updateTransform = function() {
    this.canvasGroup.setAttribute('transform',
      'translate(' + this.panX + ',' + this.panY + ') scale(' + this.zoom + ')');
  };

  WorkflowBuilder.prototype.setZoom = function(z) {
    this.zoom = Math.max(0.2, Math.min(2, z));
    this.updateTransform();
  };

  WorkflowBuilder.prototype.fitView = function() {
    if (this.nodes.length === 0) {
      this.panX = 0;
      this.panY = 0;
      this.zoom = 1;
      this.updateTransform();
      return;
    }
    var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    this.nodes.forEach(function(n) {
      var hw = n.type === 'condition' ? COND_SIZE / 2 : NODE_W / 2;
      var hh = n.type === 'condition' ? COND_SIZE / 2 : NODE_H / 2;
      if (n.x - hw < minX) minX = n.x - hw;
      if (n.y - hh < minY) minY = n.y - hh;
      if (n.x + hw > maxX) maxX = n.x + hw;
      if (n.y + hh > maxY) maxY = n.y + hh;
    });
    var rect = this.svg.getBoundingClientRect();
    var pad = 60;
    var w = maxX - minX + pad * 2;
    var h = maxY - minY + pad * 2;
    this.zoom = Math.min(rect.width / w, rect.height / h, 1.5);
    this.panX = rect.width / 2 - (minX + maxX) / 2 * this.zoom;
    this.panY = rect.height / 2 - (minY + maxY) / 2 * this.zoom;
    this.updateTransform();
  };

  WorkflowBuilder.prototype.addNode = function(tool, x, y) {
    this.pushUndo();
    var type = 'tool';
    if (tool.name === 'start') type = 'start';
    else if (tool.name === 'end') type = 'end';
    else if (tool.name === 'condition') type = 'condition';

    var node = {
      id: 'node-' + this.nextId++,
      type: type,
      tool: tool.name,
      category: tool.category || 'flow',
      description: tool.description || '',
      x: x,
      y: y,
      params: {},
      condition: '',
      maxRetries: 0,
      onFailure: 'abort',
      inputs: tool.inputs || []
    };

    if (type === 'tool') {
      (tool.inputs || []).forEach(function(inp) {
        node.params[inp] = '';
      });
    }

    this.nodes.push(node);
    this.render();
    this.selectNode(node.id);
    return node;
  };

  WorkflowBuilder.prototype.deleteNode = function(id) {
    this.pushUndo();
    this.nodes = this.nodes.filter(function(n) { return n.id !== id; });
    this.connections = this.connections.filter(function(c) {
      return c.from !== id && c.to !== id;
    });
    if (this.selectedNode === id) this.selectNode(null);
    this.render();
  };

  WorkflowBuilder.prototype.addConnection = function(fromId, fromPort, toId, toPort) {
    if (fromId === toId) return;
    var exists = this.connections.some(function(c) {
      return c.from === fromId && c.fromPort === fromPort && c.to === toId;
    });
    if (exists) return;

    this.pushUndo();
    this.connections.push({
      id: 'conn-' + Date.now(),
      from: fromId,
      fromPort: fromPort,
      to: toId,
      toPort: toPort
    });
    this.render();
  };

  WorkflowBuilder.prototype.deleteConnection = function(id) {
    this.pushUndo();
    this.connections = this.connections.filter(function(c) { return c.id !== id; });
    this.render();
  };

  WorkflowBuilder.prototype.selectNode = function(id) {
    this.selectedNode = id;
    this.render();
    this.showProperties(id);
  };

  WorkflowBuilder.prototype.showProperties = function(id) {
    var panel = document.getElementById('wf-props');
    if (!panel) return;
    if (!id) {
      panel.innerHTML = '<div class="wf-props-empty">Select a node to edit</div>';
      return;
    }
    var node = this.nodes.find(function(n) { return n.id === id; });
    if (!node) return;

    var html = '<div class="wf-props-header">' +
      '<span class="wf-props-title">' + SC.escapeHtml(node.tool) + '</span>' +
      '<button class="wf-props-delete" data-delete-node="' + node.id + '">Delete</button>' +
      '</div>';
    html += '<div class="wf-props-field"><label>Name</label><input type="text" data-field="tool" value="' +
      SC.escapeHtml(node.tool) + '"></div>';
    html += '<div class="wf-props-field"><label>Description</label><input type="text" data-field="description" value="' +
      SC.escapeHtml(node.description || '') + '"></div>';

    if (node.type === 'tool') {
      Object.keys(node.params).forEach(function(key) {
        html += '<div class="wf-props-field"><label>' + SC.escapeHtml(key) + '</label>' +
          '<input type="text" data-param="' + SC.escapeHtml(key) + '" value="' +
          SC.escapeHtml(node.params[key] || '') + '"></div>';
      });
    }

    if (node.type === 'condition') {
      html += '<div class="wf-props-field"><label>Condition</label>' +
        '<input type="text" data-field="condition" value="' +
        SC.escapeHtml(node.condition || '') + '" placeholder="e.g. {{.status}} == success"></div>';
    }

    html += '<div class="wf-props-field"><label>Max Retries</label>' +
      '<input type="number" data-field="maxRetries" value="' + (node.maxRetries || 0) + '" min="0" max="10"></div>';
    html += '<div class="wf-props-field"><label>On Failure</label>' +
      '<select data-field="onFailure"><option value="abort"' + (node.onFailure === 'abort' ? ' selected' : '') +
      '>Abort</option><option value="skip"' + (node.onFailure === 'skip' ? ' selected' : '') +
      '>Skip</option><option value="retry"' + (node.onFailure === 'retry' ? ' selected' : '') +
      '>Retry</option></select></div>';

    panel.innerHTML = html;

    var self = this;
    panel.querySelectorAll('[data-field]').forEach(function(el) {
      el.addEventListener('change', function() {
        var n = self.nodes.find(function(n) { return n.id === id; });
        if (!n) return;
        var f = el.getAttribute('data-field');
        if (f === 'maxRetries') n[f] = parseInt(el.value) || 0;
        else n[f] = el.value;
        self.render();
      });
    });

    panel.querySelectorAll('[data-param]').forEach(function(el) {
      el.addEventListener('change', function() {
        var n = self.nodes.find(function(n) { return n.id === id; });
        if (!n) return;
        n.params[el.getAttribute('data-param')] = el.value;
      });
    });

    var delBtn = panel.querySelector('[data-delete-node]');
    if (delBtn) {
      delBtn.addEventListener('click', function() {
        self.deleteNode(delBtn.getAttribute('data-delete-node'));
      });
    }
  };

  WorkflowBuilder.prototype.getPortPos = function(node, portType, portName) {
    if (node.type === 'start') {
      return { x: node.x, y: node.y + 20 };
    }
    if (node.type === 'end') {
      return { x: node.x, y: node.y - 20 };
    }
    if (node.type === 'condition') {
      var hs = COND_SIZE / 2;
      if (portType === 'in') return { x: node.x, y: node.y - hs };
      if (portName === 'true') return { x: node.x + hs, y: node.y };
      if (portName === 'false') return { x: node.x - hs, y: node.y };
      return { x: node.x, y: node.y + hs };
    }
    if (portType === 'in') return { x: node.x, y: node.y - NODE_H / 2 };
    return { x: node.x, y: node.y + NODE_H / 2 };
  };

  WorkflowBuilder.prototype.render = function() {
    var self = this;
    while (this.nodeGroup.firstChild) this.nodeGroup.removeChild(this.nodeGroup.firstChild);
    while (this.connGroup.firstChild) this.connGroup.removeChild(this.connGroup.firstChild);

    this.connections.forEach(function(conn) {
      var fromNode = self.nodes.find(function(n) { return n.id === conn.from; });
      var toNode = self.nodes.find(function(n) { return n.id === conn.to; });
      if (!fromNode || !toNode) return;

      var from = self.getPortPos(fromNode, 'out', conn.fromPort);
      var to = self.getPortPos(toNode, 'in', conn.toPort);

      var dx = Math.abs(to.x - from.x) * 0.5;
      var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', 'M' + from.x + ',' + from.y +
        ' C' + (from.x + dx) + ',' + from.y +
        ' ' + (to.x - dx) + ',' + to.y +
        ' ' + to.x + ',' + to.y);
      path.setAttribute('stroke', 'var(--tx-2, #6a6a72)');
      path.setAttribute('stroke-width', '2');
      path.setAttribute('fill', 'none');
      path.setAttribute('marker-end', 'url(#wf-arrow)');
      path.setAttribute('data-conn-id', conn.id);
      path.setAttribute('class', 'wf-connection');
      path.style.cursor = 'context-menu';
      self.connGroup.appendChild(path);

      if (conn.fromPort === 'true' || conn.fromPort === 'false') {
        var label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        var mx = (from.x + to.x) / 2;
        var my = (from.y + to.y) / 2 - 6;
        label.setAttribute('x', mx);
        label.setAttribute('y', my);
        label.setAttribute('text-anchor', 'middle');
        label.setAttribute('fill', conn.fromPort === 'true' ? 'var(--ok, #3ec96e)' : 'var(--err, #e05252)');
        label.setAttribute('font-size', '11');
        label.setAttribute('font-family', 'var(--font-d, monospace)');
        label.textContent = conn.fromPort;
        self.connGroup.appendChild(label);
      }
    });

    this.nodes.forEach(function(node) {
      var g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
      g.setAttribute('data-node-id', node.id);
      g.style.cursor = 'move';

      var isSelected = self.selectedNode === node.id;
      var color = CATEGORY_COLORS[node.category] || CATEGORY_COLORS.flow;

      if (node.type === 'start') {
        var circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        circle.setAttribute('cx', node.x);
        circle.setAttribute('cy', node.y);
        circle.setAttribute('r', '20');
        circle.setAttribute('fill', 'rgba(62,201,110,0.15)');
        circle.setAttribute('stroke', isSelected ? 'var(--accent, #8b5cf6)' : 'var(--ok, #3ec96e)');
        circle.setAttribute('stroke-width', '2');
        g.appendChild(circle);
        var t = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        t.setAttribute('x', node.x);
        t.setAttribute('y', node.y + 4);
        t.setAttribute('text-anchor', 'middle');
        t.setAttribute('fill', 'var(--ok, #3ec96e)');
        t.setAttribute('font-size', '11');
        t.setAttribute('font-weight', '600');
        t.textContent = 'START';
        g.appendChild(t);

        self.addPort(g, node, 'out', 'next', color);
      } else if (node.type === 'end') {
        var circle2 = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
        circle2.setAttribute('cx', node.x);
        circle2.setAttribute('cy', node.y);
        circle2.setAttribute('r', '20');
        circle2.setAttribute('fill', 'rgba(224,82,82,0.15)');
        circle2.setAttribute('stroke', isSelected ? 'var(--accent, #8b5cf6)' : 'var(--err, #e05252)');
        circle2.setAttribute('stroke-width', '2');
        g.appendChild(circle2);
        var t2 = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        t2.setAttribute('x', node.x);
        t2.setAttribute('y', node.y + 4);
        t2.setAttribute('text-anchor', 'middle');
        t2.setAttribute('fill', 'var(--err, #e05252)');
        t2.setAttribute('font-size', '11');
        t2.setAttribute('font-weight', '600');
        t2.textContent = 'END';
        g.appendChild(t2);

        self.addPort(g, node, 'in', 'in', color);
      } else if (node.type === 'condition') {
        var hs = COND_SIZE / 2;
        var diamond = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
        diamond.setAttribute('points',
          node.x + ',' + (node.y - hs) + ' ' +
          (node.x + hs) + ',' + node.y + ' ' +
          node.x + ',' + (node.y + hs) + ' ' +
          (node.x - hs) + ',' + node.y);
        diamond.setAttribute('fill', 'rgba(148,163,184,0.1)');
        diamond.setAttribute('stroke', isSelected ? 'var(--accent, #8b5cf6)' : color);
        diamond.setAttribute('stroke-width', '2');
        g.appendChild(diamond);
        var t3 = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        t3.setAttribute('x', node.x);
        t3.setAttribute('y', node.y + 4);
        t3.setAttribute('text-anchor', 'middle');
        t3.setAttribute('fill', color);
        t3.setAttribute('font-size', '10');
        t3.setAttribute('font-weight', '500');
        t3.setAttribute('font-family', 'var(--font-d, monospace)');
        var txt = node.condition || 'IF';
        if (txt.length > 8) txt = txt.slice(0, 7) + '\u2026';
        t3.textContent = txt;
        g.appendChild(t3);

        self.addPort(g, node, 'in', 'in', color);
        self.addPort(g, node, 'out', 'true', '#3ec96e');
        self.addPort(g, node, 'out', 'false', '#e05252');
      } else {
        var hw = NODE_W / 2;
        var hh = NODE_H / 2;
        var rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        rect.setAttribute('x', node.x - hw);
        rect.setAttribute('y', node.y - hh);
        rect.setAttribute('width', NODE_W);
        rect.setAttribute('height', NODE_H);
        rect.setAttribute('rx', '8');
        rect.setAttribute('fill', 'var(--bg-2, #1c1c1f)');
        rect.setAttribute('stroke', isSelected ? 'var(--accent, #8b5cf6)' : color);
        rect.setAttribute('stroke-width', '2');
        g.appendChild(rect);

        var catBar = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
        catBar.setAttribute('x', node.x - hw);
        catBar.setAttribute('y', node.y - hh);
        catBar.setAttribute('width', '4');
        catBar.setAttribute('height', NODE_H);
        catBar.setAttribute('rx', '2');
        catBar.setAttribute('fill', color);
        g.appendChild(catBar);

        var t4 = document.createElementNS('http://www.w3.org/2000/svg', 'text');
        t4.setAttribute('x', node.x);
        t4.setAttribute('y', node.y - 4);
        t4.setAttribute('text-anchor', 'middle');
        t4.setAttribute('fill', 'var(--tx-0, #e8e8ec)');
        t4.setAttribute('font-size', '13');
        t4.setAttribute('font-weight', '600');
        t4.setAttribute('font-family', 'var(--font-d, monospace)');
        var label = node.tool;
        if (label.length > 16) label = label.slice(0, 15) + '\u2026';
        t4.textContent = label;
        g.appendChild(t4);

        if (node.description) {
          var desc = document.createElementNS('http://www.w3.org/2000/svg', 'text');
          desc.setAttribute('x', node.x);
          desc.setAttribute('y', node.y + 12);
          desc.setAttribute('text-anchor', 'middle');
          desc.setAttribute('fill', 'var(--tx-2, #6a6a72)');
          desc.setAttribute('font-size', '10');
          var dtext = node.description;
          if (dtext.length > 22) dtext = dtext.slice(0, 21) + '\u2026';
          desc.textContent = dtext;
          g.appendChild(desc);
        }

        self.addPort(g, node, 'in', 'in', color);
        self.addPort(g, node, 'out', 'next', color);
      }

      g.addEventListener('mousedown', function(e) {
        if (e.target.classList.contains('wf-port')) return;
        e.stopPropagation();
        self.dragging = node;
      });

      g.addEventListener('dblclick', function(e) {
        e.stopPropagation();
        self.selectNode(node.id);
      });

      self.nodeGroup.appendChild(g);
    });
  };

  WorkflowBuilder.prototype.addPort = function(g, node, portType, portName, color) {
    var pos = this.getPortPos(node, portType, portName);
    var circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    circle.setAttribute('cx', pos.x);
    circle.setAttribute('cy', pos.y);
    circle.setAttribute('r', String(PORT_R));
    circle.setAttribute('fill', 'var(--bg-1, #141416)');
    circle.setAttribute('stroke', color);
    circle.setAttribute('stroke-width', '2');
    circle.setAttribute('class', 'wf-port');
    circle.setAttribute('data-port-type', portType);
    circle.setAttribute('data-port-name', portName);
    circle.setAttribute('data-node-id', node.id);
    circle.style.cursor = 'crosshair';

    var self = this;
    circle.addEventListener('mousedown', function(e) {
      e.stopPropagation();
      if (portType === 'out') {
        self.connecting = { nodeId: node.id, portName: portName, pos: pos };
      }
    });

    circle.addEventListener('mouseup', function(e) {
      e.stopPropagation();
      if (portType === 'in' && self.connecting && self.connecting.nodeId !== node.id) {
        self.addConnection(self.connecting.nodeId, self.connecting.portName, node.id, portName);
        self.connecting = null;
        self.removeTempConnection();
      }
    });

    g.appendChild(circle);
  };

  WorkflowBuilder.prototype.drawTempConnection = function(from, toPt) {
    this.removeTempConnection();
    var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    var dx = Math.abs(toPt.x - from.pos.x) * 0.5;
    path.setAttribute('d', 'M' + from.pos.x + ',' + from.pos.y +
      ' C' + (from.pos.x + dx) + ',' + from.pos.y +
      ' ' + (toPt.x - dx) + ',' + toPt.y +
      ' ' + toPt.x + ',' + toPt.y);
    path.setAttribute('stroke', 'var(--accent, #8b5cf6)');
    path.setAttribute('stroke-width', '2');
    path.setAttribute('stroke-dasharray', '6,3');
    path.setAttribute('fill', 'none');
    path.setAttribute('class', 'wf-temp-conn');
    this.connGroup.appendChild(path);
    this.tempConn = path;
  };

  WorkflowBuilder.prototype.removeTempConnection = function() {
    if (this.tempConn && this.tempConn.parentNode) {
      this.tempConn.parentNode.removeChild(this.tempConn);
    }
    this.tempConn = null;
  };

  WorkflowBuilder.prototype.pushUndo = function() {
    this.undoStack.push({
      nodes: JSON.parse(JSON.stringify(this.nodes)),
      connections: JSON.parse(JSON.stringify(this.connections))
    });
    this.redoStack = [];
    if (this.undoStack.length > 50) this.undoStack.shift();
  };

  WorkflowBuilder.prototype.undo = function() {
    if (this.undoStack.length === 0) return;
    this.redoStack.push({
      nodes: JSON.parse(JSON.stringify(this.nodes)),
      connections: JSON.parse(JSON.stringify(this.connections))
    });
    var state = this.undoStack.pop();
    this.nodes = state.nodes;
    this.connections = state.connections;
    this.render();
  };

  WorkflowBuilder.prototype.redo = function() {
    if (this.redoStack.length === 0) return;
    this.undoStack.push({
      nodes: JSON.parse(JSON.stringify(this.nodes)),
      connections: JSON.parse(JSON.stringify(this.connections))
    });
    var state = this.redoStack.pop();
    this.nodes = state.nodes;
    this.connections = state.connections;
    this.render();
  };

  WorkflowBuilder.prototype.serialize = function() {
    var name = this.currentWorkflow || 'untitled';
    var steps = [];
    var self = this;

    this.nodes.forEach(function(node) {
      if (node.type === 'start' || node.type === 'end') return;

      var step = {
        id: node.id,
        name: node.tool,
        description: node.description || ''
      };

      if (node.type === 'condition') {
        step.action = 'condition';
        step.condition = node.condition || '';
      } else {
        step.action = self.toolToAction(node.tool);
      }

      var outConns = self.connections.filter(function(c) { return c.from === node.id; });
      var trueConn = outConns.find(function(c) { return c.fromPort === 'true'; });
      var falseConn = outConns.find(function(c) { return c.fromPort === 'false'; });
      var nextConn = outConns.find(function(c) { return c.fromPort === 'next'; });

      if (trueConn) step.next_step = trueConn.to;
      else if (nextConn) step.next_step = nextConn.to;

      if (falseConn) step.on_failure = falseConn.to;
      else step.on_failure = node.onFailure || 'abort';

      if (node.maxRetries > 0) step.max_retries = node.maxRetries;

      if (node.type === 'tool') {
        var template = '';
        var command = '';
        var find = '';
        var append = '';
        var prompt = '';

        Object.keys(node.params).forEach(function(k) {
          var v = node.params[k];
          if (!v) return;
          if (node.tool === 'bash' && k === 'command') command = v;
          else if (node.tool === 'read_file' && k === 'path') find = v;
          else if (node.tool === 'write_file' && k === 'path') find = v;
          else if (node.tool === 'write_file' && k === 'content') template = v;
          else if (node.tool === 'edit_file' && k === 'path') find = v;
          else if (node.tool === 'edit_file' && k === 'replace') template = v;
          else if (node.tool === 'prompt' && k === 'text') prompt = v;
        });

        if (command) step.command = command;
        if (template) step.template = template;
        if (find) step.find = find;
        if (append) step.append = append;
        if (prompt) step.prompt = prompt;

        if (Object.keys(node.params).length > 0) {
          step.variables = {};
          Object.keys(node.params).forEach(function(k) {
            if (node.params[k]) step.variables[k] = node.params[k];
          });
          if (Object.keys(step.variables).length === 0) delete step.variables;
        }
      }

      steps.push(step);
    });

    var startNode = this.nodes.find(function(n) { return n.type === 'start'; });
    if (startNode) {
      var startConn = this.connections.find(function(c) { return c.from === startNode.id; });
      if (startConn) {
        var startTarget = this.nodes.find(function(n) { return n.id === startConn.to; });
        if (startTarget && startTarget.type !== 'end') {
          steps.unshift({
            id: 'start',
            name: 'start',
            action: 'condition',
            condition: 'on_success',
            next_step: startConn.to
          });
        }
      }
    }

    return {
      name: name,
      description: 'Workflow: ' + name,
      version: '1.0',
      params: [],
      steps: steps
    };
  };

  WorkflowBuilder.prototype.toolToAction = function(tool) {
    var map = {
      'read_file': 'edit_file',
      'write_file': 'create_file',
      'edit_file': 'edit_file',
      'bash': 'run_command',
      'condition': 'condition'
    };
    return map[tool] || 'run_command';
  };

  WorkflowBuilder.prototype.deserialize = function(pb) {
    this.nodes = [];
    this.connections = [];
    this.nextId = 1;
    this.currentWorkflow = pb.name || 'untitled';

    var startX = 100;
    var startY = 80;
    var stepGap = 120;

    this.addNode({ name: 'start', category: 'flow', description: '', inputs: [] }, startX, startY);

    (pb.steps || []).forEach(function(step, i) {
      var tool = step.action === 'condition' ? 'condition' :
        step.action === 'create_file' ? 'write_file' :
        step.action === 'edit_file' ? 'edit_file' :
        step.action === 'run_command' ? 'bash' :
        step.action === 'prompt' ? 'prompt' : step.name || step.action;

      var category = step.action === 'condition' ? 'flow' :
        (tool.startsWith('git_') ? 'git' :
        (tool.startsWith('web_') || tool.startsWith('browser_') ? 'web' :
        (tool.startsWith('docker_') || tool === 'execute_code' ? 'docker' :
        (tool === 'agent' || tool === 'think' || tool === 'skill' ? 'agent' : 'code'))));

      var y = startY + (i + 1) * stepGap;
      var node = this.addNode({
        name: tool,
        category: category,
        description: step.description || step.name || '',
        inputs: []
      }, startX, y);

      if (step.action === 'condition') {
        node.type = 'condition';
        node.condition = step.condition || '';
      }

      node.id = step.id || node.id;
      if (step.max_retries) node.maxRetries = step.max_retries;
      if (step.on_failure) node.on_failure = step.on_failure;

      if (step.command) node.params.command = step.command;
      if (step.template) node.params.content = step.template;
      if (step.find) node.params.path = step.find;
      if (step.prompt) node.params.text = step.prompt;

      if (step.variables) {
        Object.keys(step.variables).forEach(function(k) {
          node.params[k] = step.variables[k];
        });
      }
    }.bind(this));

    var endY = startY + (pb.steps || []).length * stepGap + stepGap;
    this.addNode({ name: 'end', category: 'flow', description: '', inputs: [] }, startX, endY);

    var allNodes = this.nodes;
    var startNode = allNodes[0];
    var endNode = allNodes[allNodes.length - 1];

    if (allNodes.length > 2) {
      this.connections.push({
        id: 'conn-init-0',
        from: startNode.id,
        fromPort: 'next',
        to: allNodes[1].id,
        toPort: 'in'
      });
    }

    (pb.steps || []).forEach(function(step, i) {
      var node = allNodes[i + 1];
      if (!node) return;

      if (step.next_step) {
        var target = allNodes.find(function(n) { return n.id === step.next_step; });
        if (target) {
          var portName = node.type === 'condition' ? 'true' : 'next';
          self.connections.push({
            id: 'conn-' + node.id + '-next',
            from: node.id,
            fromPort: portName,
            to: target.id,
            toPort: 'in'
          });
        }
      }

      if (step.on_failure && step.on_failure !== 'abort' && step.on_failure !== 'skip' && step.on_failure !== 'retry') {
        var failTarget = allNodes.find(function(n) { return n.id === step.on_failure; });
        if (failTarget) {
          var failPort = node.type === 'condition' ? 'false' : 'next';
          self.connections.push({
            id: 'conn-' + node.id + '-fail',
            from: node.id,
            fromPort: 'false',
            to: failTarget.id,
            toPort: 'in'
          });
        }
      }
    });

    var lastStepNode = allNodes[allNodes.length - 2];
    if (lastStepNode && lastStepNode.type !== 'end') {
      var hasOut = this.connections.some(function(c) { return c.from === lastStepNode.id; });
      if (!hasOut) {
        this.connections.push({
          id: 'conn-to-end',
          from: lastStepNode.id,
          fromPort: lastStepNode.type === 'condition' ? 'true' : 'next',
          to: endNode.id,
          toPort: 'in'
        });
      }
    }

    var self = this;
    this.render();
    this.fitView();
    this.updateName();
  };

  WorkflowBuilder.prototype.updateName = function() {
    var el = document.getElementById('wf-toolbar-name');
    if (el) el.textContent = this.currentWorkflow ? this.currentWorkflow : '';
  };

  WorkflowBuilder.prototype.newWorkflow = function() {
    this.nodes = [];
    this.connections = [];
    this.nextId = 1;
    this.currentWorkflow = null;
    this.undoStack = [];
    this.redoStack = [];

    this.addNode({ name: 'start', category: 'flow', description: '', inputs: [] }, 300, 60);
    this.addNode({ name: 'end', category: 'flow', description: '', inputs: [] }, 300, 500);

    this.render();
    this.fitView();
    this.updateName();
    this.showBuilder();
  };

  WorkflowBuilder.prototype.saveWorkflow = function() {
    var pb = this.serialize();
    if (!pb.name || pb.name === 'untitled') {
      var name = prompt('Workflow name:');
      if (!name) return;
      pb.name = name;
      this.currentWorkflow = name;
    }
    var self = this;
    fetch('/api/workflows', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(pb)
    }).then(function(r) { return r.json(); }).then(function(data) {
      if (data.error) {
        SC.toast('Save failed: ' + data.error, 'error');
      } else {
        SC.toast('Workflow saved: ' + pb.name, 'ok');
        self.currentWorkflow = pb.name;
        self.updateName();
        self.loadWorkflowList();
      }
    }).catch(function(err) {
      SC.toast('Save error: ' + err.message, 'error');
    });
  };

  WorkflowBuilder.prototype.saveAsWorkflow = function() {
    var name = prompt('Save as name:', this.currentWorkflow || '');
    if (!name) return;
    this.currentWorkflow = name;
    this.saveWorkflow();
  };

  WorkflowBuilder.prototype.deleteCurrentWorkflow = function() {
    if (!this.currentWorkflow) return;
    if (!confirm('Delete workflow "' + this.currentWorkflow + '"?')) return;
    var self = this;
    fetch('/api/workflows/' + encodeURIComponent(this.currentWorkflow), {
      method: 'DELETE'
    }).then(function(r) { return r.json(); }).then(function(data) {
      SC.toast('Deleted: ' + self.currentWorkflow, 'ok');
      self.currentWorkflow = null;
      self.updateName();
      self.loadWorkflowList();
    }).catch(function(err) {
      SC.toast('Delete error: ' + err.message, 'error');
    });
  };

  WorkflowBuilder.prototype.runWorkflow = function() {
    if (!this.currentWorkflow) {
      SC.toast('Save workflow before running', 'warn');
      return;
    }
    var self = this;
    SC.toast('Running workflow: ' + this.currentWorkflow, 'info');
    fetch('/api/workflows/' + encodeURIComponent(this.currentWorkflow) + '/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ params: {} })
    }).then(function(r) { return r.json(); }).then(function(data) {
      if (data.error) {
        SC.toast('Run failed: ' + data.error, 'error');
      } else {
        SC.toast('Workflow ' + data.status + ' (' + (data.results || []).length + ' steps)', 'ok');
      }
    }).catch(function(err) {
      SC.toast('Run error: ' + err.message, 'error');
    });
  };

  WorkflowBuilder.prototype.showBuilder = function() {
    var list = document.getElementById('wf-list');
    var wrapper = this.container.querySelector('.wf-wrapper');
    if (list) list.style.display = 'none';
    if (wrapper) wrapper.style.display = 'flex';
  };

  WorkflowBuilder.prototype.showList = function() {
    var list = document.getElementById('wf-list');
    var wrapper = this.container.querySelector('.wf-wrapper');
    if (list) list.style.display = 'block';
    if (wrapper) wrapper.style.display = 'none';
  };

  WorkflowBuilder.prototype.loadWorkflowList = function() {
    var self = this;
    fetch('/api/workflows').then(function(r) { return r.json(); }).then(function(workflows) {
      self.renderWorkflowList(workflows || []);
    }).catch(function(err) {
      self.renderWorkflowList([]);
    });
  };

  WorkflowBuilder.prototype.renderWorkflowList = function(workflows) {
    var list = document.getElementById('wf-list');
    if (!list) return;
    var self = this;

    var html = '<div class="wf-list-header"><span>Workflows</span>' +
      '<button class="wf-list-create" id="wf-create-btn">+ New</button></div>';

    if (workflows.length === 0) {
      html += '<div class="wf-list-empty">No workflows yet. Create one to get started.</div>';
    } else {
      workflows.forEach(function(wf) {
        var stepCount = (wf.steps || []).length;
        html += '<div class="wf-list-item" data-wf-name="' + SC.escapeHtml(wf.name) + '">' +
          '<div class="wf-list-item-name">' + SC.escapeHtml(wf.name) + '</div>' +
          '<div class="wf-list-item-meta">' + stepCount + ' steps</div>' +
          '<div class="wf-list-item-actions">' +
          '<button class="wf-list-run" data-wf-run="' + SC.escapeHtml(wf.name) + '">Run</button>' +
          '<button class="wf-list-del" data-wf-del="' + SC.escapeHtml(wf.name) + '">Delete</button>' +
          '</div></div>';
      });
    }

    list.innerHTML = html;

    var createBtn = list.querySelector('#wf-create-btn');
    if (createBtn) {
      createBtn.addEventListener('click', function() {
        self.newWorkflow();
      });
    }

    list.querySelectorAll('[data-wf-name]').forEach(function(el) {
      el.addEventListener('click', function(e) {
        if (e.target.closest('[data-wf-run]') || e.target.closest('[data-wf-del]')) return;
        var name = el.getAttribute('data-wf-name');
        self.openWorkflow(name);
      });
    });

    list.querySelectorAll('[data-wf-run]').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var name = btn.getAttribute('data-wf-run');
        SC.toast('Running: ' + name, 'info');
        fetch('/api/workflows/' + encodeURIComponent(name) + '/execute', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ params: {} })
        }).then(function(r) { return r.json(); }).then(function(data) {
          SC.toast('Workflow ' + (data.status || 'done'), data.error ? 'error' : 'ok');
        }).catch(function(err) {
          SC.toast('Run error: ' + err.message, 'error');
        });
      });
    });

    list.querySelectorAll('[data-wf-del]').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var name = btn.getAttribute('data-wf-del');
        if (!confirm('Delete "' + name + '"?')) return;
        fetch('/api/workflows/' + encodeURIComponent(name), {
          method: 'DELETE'
        }).then(function(r) { return r.json(); }).then(function(data) {
          SC.toast('Deleted: ' + name, 'ok');
          self.loadWorkflowList();
        }).catch(function(err) {
          SC.toast('Delete error: ' + err.message, 'error');
        });
      });
    });

    if (!this.currentWorkflow && workflows.length > 0) {
      this.showList();
    }
  };

  WorkflowBuilder.prototype.openWorkflow = function(name) {
    var self = this;
    fetch('/api/workflows/' + encodeURIComponent(name)).then(function(r) { return r.json(); }).then(function(pb) {
      if (pb.error) {
        SC.toast('Load failed: ' + pb.error, 'error');
        return;
      }
      self.deserialize(pb);
      self.showBuilder();
    }).catch(function(err) {
      SC.toast('Load error: ' + err.message, 'error');
    });
  };

  SC.WorkflowBuilder = WorkflowBuilder;

  var builderInstance = null;

  SC.initWorkflows = function() {
    var container = document.getElementById('section-workflows');
    if (!container) return;

    var inner = container.querySelector('.wf-container');
    if (!inner) {
      inner = document.createElement('div');
      inner.className = 'wf-container';
      container.appendChild(inner);
    }

    if (!builderInstance) {
      builderInstance = new WorkflowBuilder(inner);
      fetch('/api/workflow-tools').then(function(r) { return r.json(); }).then(function(tools) {
        builderInstance.init(tools || []);
      }).catch(function() {
        builderInstance.init([]);
      });
    } else {
      builderInstance.loadWorkflowList();
    }
  };

  document.addEventListener('section:workflows', function() {
    SC.initWorkflows();
  });
})();
