// SmartClaw - Codebase Dependency Graph
(function() {
  'use strict';

  var svgEl = null;
  var nodes = [];
  var edges = [];
  var simulation = null;
  var animFrame = null;

  function generateGraphFromFiles(files) {
    nodes = [];
    edges = [];
    var nodeMap = {};

    if (!files || !files.length) {
      var sampleNodes = [
        { id: 'cmd/smartclaw', group: 'cmd', label: 'smartclaw' },
        { id: 'internal/api', group: 'api', label: 'api' },
        { id: 'internal/agents', group: 'core', label: 'agents' },
        { id: 'internal/memory', group: 'core', label: 'memory' },
        { id: 'internal/learning', group: 'core', label: 'learning' },
        { id: 'internal/runtime', group: 'core', label: 'runtime' },
        { id: 'internal/tools', group: 'tools', label: 'tools' },
        { id: 'internal/mcp', group: 'tools', label: 'mcp' },
        { id: 'internal/web', group: 'web', label: 'web' },
        { id: 'internal/config', group: 'config', label: 'config' },
        { id: 'internal/store', group: 'data', label: 'store' },
        { id: 'internal/session', group: 'core', label: 'session' },
        { id: 'internal/gateway', group: 'gateway', label: 'gateway' },
        { id: 'internal/cli', group: 'cli', label: 'cli' },
        { id: 'internal/skills', group: 'core', label: 'skills' },
        { id: 'internal/compact', group: 'core', label: 'compact' },
        { id: 'internal/costguard', group: 'guard', label: 'costguard' },
        { id: 'internal/permissions', group: 'security', label: 'permissions' },
        { id: 'internal/sandbox', group: 'security', label: 'sandbox' },
        { id: 'internal/hooks', group: 'core', label: 'hooks' }
      ];
      sampleNodes.forEach(function(n) {
        nodes.push(Object.assign({}, n, { x: 0, y: 0, vx: 0, vy: 0 }));
        nodeMap[n.id] = nodes[nodes.length - 1];
      });
      var sampleEdges = [
        ['cmd/smartclaw', 'internal/cli'], ['cmd/smartclaw', 'internal/config'],
        ['internal/cli', 'internal/api'], ['internal/cli', 'internal/runtime'],
        ['internal/api', 'internal/memory'], ['internal/api', 'internal/store'],
        ['internal/agents', 'internal/memory'], ['internal/agents', 'internal/tools'],
        ['internal/agents', 'internal/learning'], ['internal/agents', 'internal/hooks'],
        ['internal/learning', 'internal/memory'], ['internal/learning', 'internal/skills'],
        ['internal/runtime', 'internal/agents'], ['internal/runtime', 'internal/compact'],
        ['internal/runtime', 'internal/costguard'], ['internal/runtime', 'internal/tools'],
        ['internal/web', 'internal/mcp'], ['internal/web', 'internal/store'],
        ['internal/gateway', 'internal/agents'], ['internal/gateway', 'internal/session'],
        ['internal/session', 'internal/store'], ['internal/session', 'internal/memory'],
        ['internal/skills', 'internal/store'], ['internal/costguard', 'internal/config'],
        ['internal/compact', 'internal/memory'], ['internal/tools', 'internal/mcp'],
        ['internal/tools', 'internal/sandbox'], ['internal/tools', 'internal/permissions'],
        ['internal/mcp', 'internal/config']
      ];
      sampleEdges.forEach(function(e) {
        if (nodeMap[e[0]] && nodeMap[e[1]]) {
          edges.push({ source: nodeMap[e[0]], target: nodeMap[e[1]] });
        }
      });
      return;
    }

    files.forEach(function(f) {
      if (!f.path || !f.path.endsWith('.go')) return;
      var parts = f.path.split('/');
      if (parts.length < 2) return;
      var moduleId = parts.slice(0, 2).join('/');
      if (!nodeMap[moduleId]) {
        var group = parts[1] || 'other';
        nodes.push({ id: moduleId, group: group, label: parts[1] || moduleId, x: 0, y: 0, vx: 0, vy: 0 });
        nodeMap[moduleId] = nodes[nodes.length - 1];
      }
    });
  }

  var GROUP_COLORS = {
    cmd: '#f59e0b', api: '#3b82f6', core: '#8b5cf6', tools: '#10b981',
    web: '#ec4899', config: '#6a6a72', data: '#06b6d4', gateway: '#f97316',
    cli: '#f59e0b', guard: '#e05252', security: '#ef4444', other: '#6a6a72'
  };

  function simulate() {
    var alpha = 0.3;
    var repulsion = 800;
    var attraction = 0.005;
    var center = 0.01;
    var damping = 0.9;
    var width = 800;
    var height = 500;
    var cx = width / 2;
    var cy = height / 2;

    for (var iter = 0; iter < 300; iter++) {
      for (var i = 0; i < nodes.length; i++) {
        for (var j = i + 1; j < nodes.length; j++) {
          var dx = nodes[j].x - nodes[i].x;
          var dy = nodes[j].y - nodes[i].y;
          var dist = Math.sqrt(dx * dx + dy * dy) || 1;
          var force = repulsion / (dist * dist);
          var fx = dx / dist * force;
          var fy = dy / dist * force;
          nodes[i].vx -= fx; nodes[i].vy -= fy;
          nodes[j].vx += fx; nodes[j].vy += fy;
        }
      }

      edges.forEach(function(e) {
        var dx = e.target.x - e.source.x;
        var dy = e.target.y - e.source.y;
        var dist = Math.sqrt(dx * dx + dy * dy) || 1;
        var force = (dist - 120) * attraction;
        var fx = dx / dist * force;
        var fy = dy / dist * force;
        e.source.vx += fx; e.source.vy += fy;
        e.target.vx -= fx; e.target.vy -= fy;
      });

      nodes.forEach(function(n) {
        n.vx += (cx - n.x) * center;
        n.vy += (cy - n.y) * center;
        n.vx *= damping;
        n.vy *= damping;
        n.x += n.vx * alpha;
        n.y += n.vy * alpha;
      });
    }
  }

  function renderGraph() {
    var container = SC.$('#graph-canvas');
    if (!container) return;
    container.innerHTML = '';

    if (nodes.length === 0) {
      container.innerHTML = '<div class="empty-state"><span class="empty-desc">No graph data available</span></div>';
      return;
    }

    simulate();

    var width = container.clientWidth || 800;
    var height = container.clientHeight || 500;

    var minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    nodes.forEach(function(n) {
      if (n.x < minX) minX = n.x; if (n.x > maxX) maxX = n.x;
      if (n.y < minY) minY = n.y; if (n.y > maxY) maxY = n.y;
    });
    var scaleX = (width - 80) / (maxX - minX || 1);
    var scaleY = (height - 80) / (maxY - minY || 1);
    var scale = Math.min(scaleX, scaleY, 2);

    var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', width);
    svg.setAttribute('height', height);
    svg.setAttribute('viewBox', '0 0 ' + width + ' ' + height);

    edges.forEach(function(e) {
      var line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
      line.setAttribute('x1', (e.source.x - minX) * scale + 40);
      line.setAttribute('y1', (e.source.y - minY) * scale + 40);
      line.setAttribute('x2', (e.target.x - minX) * scale + 40);
      line.setAttribute('y2', (e.target.y - minY) * scale + 40);
      line.setAttribute('stroke', 'var(--bd)');
      line.setAttribute('stroke-width', '1');
      line.setAttribute('opacity', '0.4');
      svg.appendChild(line);
    });

    var tooltip = document.createElementNS('http://www.w3.org/2000/svg', 'text');
    tooltip.setAttribute('text-anchor', 'middle');
    tooltip.setAttribute('fill', 'var(--tx-0)');
    tooltip.setAttribute('font-size', '11');
    tooltip.setAttribute('font-weight', '600');
    tooltip.setAttribute('font-family', 'var(--font-d)');
    tooltip.style.display = 'none';
    svg.appendChild(tooltip);

    nodes.forEach(function(n) {
      var g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
      var cx = (n.x - minX) * scale + 40;
      var cy = (n.y - minY) * scale + 40;
      var color = GROUP_COLORS[n.group] || GROUP_COLORS.other;

      var circle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      circle.setAttribute('cx', cx);
      circle.setAttribute('cy', cy);
      circle.setAttribute('r', 8);
      circle.setAttribute('fill', color);
      circle.setAttribute('stroke', 'var(--bg-0)');
      circle.setAttribute('stroke-width', '2');
      circle.setAttribute('opacity', '0.8');
      circle.style.cursor = 'pointer';
      circle.style.transition = 'r 150ms ease, opacity 150ms ease';

      circle.addEventListener('mouseenter', function() {
        this.setAttribute('r', '12');
        this.setAttribute('opacity', '1');
        tooltip.textContent = n.id;
        tooltip.setAttribute('x', cx);
        tooltip.setAttribute('y', cy - 16);
        tooltip.style.display = '';
      });
      circle.addEventListener('mouseleave', function() {
        this.setAttribute('r', '8');
        this.setAttribute('opacity', '0.8');
        tooltip.style.display = 'none';
      });

      var text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      text.setAttribute('x', cx);
      text.setAttribute('y', cy + 22);
      text.setAttribute('text-anchor', 'middle');
      text.setAttribute('fill', 'var(--tx-2)');
      text.setAttribute('font-size', '9');
      text.setAttribute('font-family', 'var(--font-d)');
      text.textContent = n.label;

      g.appendChild(circle);
      g.appendChild(text);
      svg.appendChild(g);
    });

    container.appendChild(svg);
  }

  function initGraph() {
    generateGraphFromFiles(null);
    var refreshBtn = SC.$('#graph-refresh');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', function() {
        SC.wsSend('file_tree', {});
        renderGraph();
      });
    }
    renderGraph();

    var resizeObserver = new ResizeObserver(function() {
      if (SC.$('#graph-canvas')) renderGraph();
    });
    var graphCanvas = SC.$('#graph-canvas');
    if (graphCanvas) resizeObserver.observe(graphCanvas);
  }

  SC.initGraph = initGraph;
  SC.renderGraph = renderGraph;
})();
