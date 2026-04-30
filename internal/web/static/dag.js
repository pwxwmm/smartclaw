// SmartClaw - DAG Visualization
(function() {
  'use strict';

  function DAGLayout(nodes, edges, options) {
    this.nodes = nodes || [];
    this.edges = edges || [];
    this.options = Object.assign({
      nodeWidth: 140,
      nodeHeight: 40,
      layerGap: 60,
      nodeGap: 30,
      padding: 20
    }, options || {});
  }

  DAGLayout.prototype.layout = function() {
    var opts = this.options;
    var nodes = this.nodes;
    var edges = this.edges;

    if (nodes.length === 0) return nodes;

    var nodeMap = {};
    var inDegree = {};
    var adj = {};
    for (var i = 0; i < nodes.length; i++) {
      var id = nodes[i].id;
      nodeMap[id] = nodes[i];
      inDegree[id] = 0;
      adj[id] = [];
    }
    for (var j = 0; j < edges.length; j++) {
      var e = edges[j];
      if (adj[e.from]) adj[e.from].push(e.to);
      if (inDegree[e.to] !== undefined) inDegree[e.to]++;
    }

    var visited = {};
    var layers = [];
    var queue = [];
    for (var nid in inDegree) {
      if (inDegree[nid] === 0) queue.push(nid);
    }

    while (queue.length > 0) {
      var layer = [];
      var nextQueue = [];
      for (var k = 0; k < queue.length; k++) {
        if (!visited[queue[k]]) {
          visited[queue[k]] = true;
          layer.push(queue[k]);
          var neighbors = adj[queue[k]] || [];
          for (var m = 0; m < neighbors.length; m++) {
            inDegree[neighbors[m]]--;
            if (inDegree[neighbors[m]] === 0) {
              nextQueue.push(neighbors[m]);
            }
          }
        }
      }
      if (layer.length > 0) layers.push(layer);
      queue = nextQueue;
    }

    for (var nid2 in nodeMap) {
      if (!visited[nid2]) {
        layers.push([nid2]);
      }
    }

    for (var li = 0; li < layers.length; li++) {
      var lay = layers[li];
      for (var ni = 0; ni < lay.length; ni++) {
        var node = nodeMap[lay[ni]];
        if (node) {
          node.x = opts.padding + ni * (opts.nodeWidth + opts.nodeGap);
          node.y = opts.padding + li * (opts.nodeHeight + opts.layerGap);
          node.layer = li;
        }
      }
    }

    for (var ei = 0; ei < this.edges.length; ei++) {
      var ed = this.edges[ei];
      if (nodeMap[ed.from] && !nodeMap[ed.to]) {
        var dummy = { id: ed.to, label: ed.to, x: 0, y: 0, status: 'pending', layer: 0 };
        nodeMap[ed.to] = dummy;
        nodes.push(dummy);
      }
    }

    return nodes;
  };

  function DAGRenderer(container) {
    this.container = typeof container === 'string' ? document.querySelector(container) : container;
    this.svg = null;
    this.nodeElements = {};
  }

  DAGRenderer.prototype.render = function(nodes, edges) {
    if (!this.container) return;
    this.container.innerHTML = '';
    this.nodeElements = {};

    if (!nodes || nodes.length === 0) return;

    var opts = {
      nodeWidth: 140,
      nodeHeight: 40,
      padding: 20
    };

    var maxX = 0, maxY = 0;
    for (var i = 0; i < nodes.length; i++) {
      if (nodes[i].x + opts.nodeWidth > maxX) maxX = nodes[i].x + opts.nodeWidth;
      if (nodes[i].y + opts.nodeHeight > maxY) maxY = nodes[i].y + opts.nodeHeight;
    }

    var width = maxX + opts.padding;
    var height = maxY + opts.padding;

    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 ' + width + ' ' + height);
    svg.setAttribute('width', '100%');
    svg.style.overflow = 'visible';
    this.svg = svg;

    var defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
    var marker = document.createElementNS('http://www.w3.org/2000/svg', 'marker');
    marker.setAttribute('id', 'dag-arrowhead');
    marker.setAttribute('markerWidth', '10');
    marker.setAttribute('markerHeight', '7');
    marker.setAttribute('refX', '10');
    marker.setAttribute('refY', '3.5');
    marker.setAttribute('orient', 'auto');
    var polygon = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
    polygon.setAttribute('points', '0 0, 10 3.5, 0 7');
    polygon.setAttribute('fill', 'var(--tx-2)');
    marker.appendChild(polygon);
    defs.appendChild(marker);
    svg.appendChild(defs);

    var edgeGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    edgeGroup.setAttribute('class', 'dag-edges');

    if (edges) {
      for (var j = 0; j < edges.length; j++) {
        var fromNode = null, toNode = null;
        for (var n = 0; n < nodes.length; n++) {
          if (nodes[n].id === edges[j].from) fromNode = nodes[n];
          if (nodes[n].id === edges[j].to) toNode = nodes[n];
        }
        if (fromNode && toNode) {
          var x1 = fromNode.x + opts.nodeWidth;
          var y1 = fromNode.y + opts.nodeHeight / 2;
          var x2 = toNode.x;
          var y2 = toNode.y + opts.nodeHeight / 2;
          var cx1 = x1 + (x2 - x1) * 0.5;
          var cx2 = x2 - (x2 - x1) * 0.5;

          var path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
          path.setAttribute('d', 'M' + x1 + ',' + y1 + ' C' + cx1 + ',' + y1 + ' ' + cx2 + ',' + y2 + ' ' + x2 + ',' + y2);
          path.setAttribute('stroke', 'var(--tx-2)');
          path.setAttribute('stroke-width', '1.5');
          path.setAttribute('fill', 'none');
          path.setAttribute('marker-end', 'url(#dag-arrowhead)');
          path.setAttribute('opacity', '0.5');
          edgeGroup.appendChild(path);
        }
      }
    }
    svg.appendChild(edgeGroup);

    var nodeGroup = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    nodeGroup.setAttribute('class', 'dag-nodes');

    var statusColors = {
      pending: { fill: 'var(--bg-3)', stroke: 'var(--bd)', text: 'var(--tx-2)' },
      running: { fill: 'rgba(82,144,224,0.15)', stroke: 'var(--info)', text: 'var(--info)' },
      success: { fill: 'rgba(62,201,110,0.1)', stroke: 'var(--ok)', text: 'var(--ok)' },
      error: { fill: 'rgba(224,82,82,0.1)', stroke: 'var(--err)', text: 'var(--err)' }
    };

    for (var k = 0; k < nodes.length; k++) {
      var nd = nodes[k];
      var g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
      g.setAttribute('class', 'dag-node');
      g.setAttribute('data-node-id', nd.id);
      g.style.cursor = 'pointer';

      var colors = statusColors[nd.status || 'pending'] || statusColors.pending;

      var rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
      rect.setAttribute('x', nd.x);
      rect.setAttribute('y', nd.y);
      rect.setAttribute('width', opts.nodeWidth);
      rect.setAttribute('height', opts.nodeHeight);
      rect.setAttribute('rx', '6');
      rect.setAttribute('fill', colors.fill);
      rect.setAttribute('stroke', colors.stroke);
      rect.setAttribute('stroke-width', '1.5');
      if (nd.status === 'running') {
        rect.setAttribute('class', 'dag-node-running');
      }
      g.appendChild(rect);

      var label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      label.setAttribute('x', nd.x + opts.nodeWidth / 2);
      label.setAttribute('y', nd.y + opts.nodeHeight / 2 + 4);
      label.setAttribute('text-anchor', 'middle');
      label.setAttribute('fill', colors.text);
      label.setAttribute('font-size', '12');
      label.setAttribute('font-family', 'var(--font-d)');
      label.setAttribute('font-weight', '500');
      var text = nd.label || nd.id;
      if (text.length > 16) text = text.slice(0, 15) + '…';
      label.textContent = text;
      g.appendChild(label);

      var icons = { pending: '○', running: '◐', success: '●', error: '✕' };
      var icon = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      icon.setAttribute('x', nd.x + 10);
      icon.setAttribute('y', nd.y + opts.nodeHeight / 2 + 5);
      icon.setAttribute('fill', colors.text);
      icon.setAttribute('font-size', '12');
      icon.textContent = icons[nd.status || 'pending'] || '○';
      g.appendChild(icon);

      (function(nodeId) {
        g.addEventListener('click', function() {
          var evt = new CustomEvent('dag:nodeClick', { detail: { nodeId: nodeId } });
          document.dispatchEvent(evt);
        });
      })(nd.id);

      this.nodeElements[nd.id] = g;
      nodeGroup.appendChild(g);
    }

    svg.appendChild(nodeGroup);
    this.container.appendChild(svg);
  };

  DAGRenderer.prototype.updateNode = function(nodeId, status) {
    var g = this.nodeElements[nodeId];
    if (!g) return;

    var statusColors = {
      pending: { fill: 'var(--bg-3)', stroke: 'var(--bd)', text: 'var(--tx-2)' },
      running: { fill: 'rgba(82,144,224,0.15)', stroke: 'var(--info)', text: 'var(--info)' },
      success: { fill: 'rgba(62,201,110,0.1)', stroke: 'var(--ok)', text: 'var(--ok)' },
      error: { fill: 'rgba(224,82,82,0.1)', stroke: 'var(--err)', text: 'var(--err)' }
    };

    var colors = statusColors[status] || statusColors.pending;
    var rect = g.querySelector('rect');
    if (rect) {
      rect.setAttribute('fill', colors.fill);
      rect.setAttribute('stroke', colors.stroke);
      if (status === 'running') {
        rect.setAttribute('class', 'dag-node-running');
      } else {
        rect.removeAttribute('class');
      }
    }

    var icons = { pending: '○', running: '◐', success: '●', error: '✕' };
    var textEls = g.querySelectorAll('text');
    for (var i = 0; i < textEls.length; i++) {
      if (textEls[i].getAttribute('x') === '10' || (textEls[i].textContent === '○' || textEls[i].textContent === '◐' || textEls[i].textContent === '●' || textEls[i].textContent === '✕')) {
        textEls[i].setAttribute('fill', colors.text);
        textEls[i].textContent = icons[status] || '○';
      } else {
        textEls[i].setAttribute('fill', colors.text);
      }
    }
  };

  DAGRenderer.prototype.destroy = function() {
    if (this.container) this.container.innerHTML = '';
    this.svg = null;
    this.nodeElements = {};
  };

  if (!SC.state.agentToolCalls) {
    SC.state.agentToolCalls = [];
  }

  var dagRenderer = null;
  var dagPanelVisible = false;

  function trackToolCall(msg) {
    if (!SC.state.agentToolCalls) SC.state.agentToolCalls = [];
    var toolName = msg.tool || 'unknown';
    var toolId = msg.id || ('tool-' + Date.now());
    SC.state.agentToolCalls.push({
      id: toolId,
      name: toolName,
      status: 'running',
      input: msg.input || {}
    });
    maybeShowDAG();
    updateDAG();
  }

  function trackToolEnd(msg) {
    if (!SC.state.agentToolCalls) return;
    var toolId = msg.id;
    for (var i = 0; i < SC.state.agentToolCalls.length; i++) {
      if (SC.state.agentToolCalls[i].id === toolId) {
        SC.state.agentToolCalls[i].status = 'success';
        break;
      }
    }
    updateDAG();
  }

  function maybeShowDAG() {
    if (dagPanelVisible) return;
    if (SC.state.agentToolCalls && SC.state.agentToolCalls.length >= 2) {
      var panel = SC.$('#dag-panel');
      if (panel) {
        panel.classList.remove('hidden');
        dagPanelVisible = true;
      }
    }
  }

  function updateDAG() {
    var calls = SC.state.agentToolCalls;
    if (!calls || calls.length === 0) return;

    var nodes = [];
    var edges = [];
    for (var i = 0; i < calls.length; i++) {
      nodes.push({
        id: calls[i].id,
        label: calls[i].name,
        status: calls[i].status
      });
      if (i > 0) {
        edges.push({ from: calls[i - 1].id, to: calls[i].id });
      }
    }

    var layout = new DAGLayout(nodes, edges);
    var laidOut = layout.layout();

    var container = SC.$('#dag-container');
    if (!container) return;

    if (!dagRenderer) {
      dagRenderer = new DAGRenderer(container);
    }
    dagRenderer.render(laidOut, edges);
  }

  function resetDAG() {
    SC.state.agentToolCalls = [];
    dagPanelVisible = false;
    var panel = SC.$('#dag-panel');
    if (panel) panel.classList.add('hidden');
    if (dagRenderer) {
      dagRenderer.destroy();
      dagRenderer = null;
    }
  }

  document.addEventListener('dag:nodeClick', function(e) {
    var nodeId = e.detail.nodeId;
    if (SC.state.agentToolCalls) {
      for (var i = 0; i < SC.state.agentToolCalls.length; i++) {
        if (SC.state.agentToolCalls[i].id === nodeId) {
          var call = SC.state.agentToolCalls[i];
          SC.toast(call.name + ': ' + (call.status === 'running' ? 'Running...' : call.status), 'info');
          break;
        }
      }
    }
  });

  SC.DAGLayout = DAGLayout;
  SC.DAGRenderer = DAGRenderer;
  SC.trackToolCall = trackToolCall;
  SC.trackToolEnd = trackToolEnd;
  SC.resetDAG = resetDAG;
})();
