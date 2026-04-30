// SmartClaw - Status Ring
(function() {
  'use strict';

  var R = 10;
  var C = 2 * Math.PI * R;
  var GAP = 2;
  var SLOT = (C - 3 * GAP) / 3;
  var SW = 3;
  var CX = 14;
  var CY = 14;
  var COST_REF = 2.0;
  var ERR_REF = 5;
  var ANIM = '300ms cubic-bezier(0.4, 0, 0.2, 1)';

  var svgEl, centerText, tooltip;
  var segs = {};

  function ns(tag) {
    return document.createElementNS('http://www.w3.org/2000/svg', tag);
  }

  function makeSeg(color, startOffset) {
    var c = ns('circle');
    c.setAttribute('cx', CX);
    c.setAttribute('cy', CY);
    c.setAttribute('r', R);
    c.setAttribute('fill', 'none');
    c.setAttribute('stroke', color);
    c.setAttribute('stroke-width', SW);
    c.style.strokeDasharray = '0 ' + C;
    c.style.strokeDashoffset = String(-startOffset);
    c.style.transition = 'stroke-dasharray ' + ANIM + ', stroke-dashoffset ' + ANIM;
    return c;
  }

  function initStatusRing() {
    var target = SC.$('#stat-tokens');
    if (!target || SC.$('#status-ring')) return;

    var svg = ns('svg');
    svg.setAttribute('id', 'status-ring');
    svg.setAttribute('width', '28');
    svg.setAttribute('height', '28');
    svg.setAttribute('viewBox', '0 0 28 28');
    svg.style.cssText = 'flex-shrink:0;cursor:pointer;';

    var g = ns('g');
    g.setAttribute('transform', 'rotate(-90 ' + CX + ' ' + CY + ')');

    var bg = ns('circle');
    bg.setAttribute('cx', CX);
    bg.setAttribute('cy', CY);
    bg.setAttribute('r', R);
    bg.setAttribute('fill', 'none');
    bg.setAttribute('stroke', 'var(--bg-3)');
    bg.setAttribute('stroke-width', SW);
    g.appendChild(bg);

    segs.token = { el: makeSeg('#8b5cf6', 0), off: 0 };
    g.appendChild(segs.token.el);

    segs.cost = { el: makeSeg('hsl(45, 80%, 55%)', SLOT + GAP), off: SLOT + GAP };
    g.appendChild(segs.cost.el);

    segs.error = { el: makeSeg('hsl(0, 70%, 55%)', 2 * (SLOT + GAP)), off: 2 * (SLOT + GAP) };
    g.appendChild(segs.error.el);

    svg.appendChild(g);

    centerText = ns('text');
    centerText.setAttribute('x', CX);
    centerText.setAttribute('y', CY);
    centerText.setAttribute('text-anchor', 'middle');
    centerText.setAttribute('dominant-baseline', 'central');
    centerText.setAttribute('font-size', '6');
    centerText.setAttribute('font-weight', '700');
    centerText.setAttribute('fill', 'var(--tx-1)');
    centerText.setAttribute('font-family', 'var(--font-d)');
    centerText.textContent = '\u2014';
    svg.appendChild(centerText);

    svgEl = svg;
    target.parentNode.insertBefore(svg, target);

    tooltip = document.createElement('div');
    tooltip.id = 'status-ring-tooltip';
    tooltip.style.cssText = 'position:fixed;padding:6px 10px;font-size:11px;font-family:var(--font-d);background:var(--bg-2);color:var(--tx-0);border:1px solid var(--bd);border-radius:var(--r);box-shadow:var(--sh-m);pointer-events:none;opacity:0;transition:opacity 150ms;z-index:9999;white-space:pre;line-height:1.5;';
    document.body.appendChild(tooltip);

    svgEl.addEventListener('mouseenter', showTooltip);
    svgEl.addEventListener('mousemove', moveTooltip);
    svgEl.addEventListener('mouseleave', hideTooltip);

    updateStatusRing();
  }

  function updateStatusRing() {
    var tokens = SC.state.tokenCount || SC.state.tokens.used || 0;
    var limit = SC.state.tokens.limit || 200000;
    var cost = SC.state.estimatedCost || SC.state.cost || 0;
    var errors = SC.state.errorCount || 0;
    var hasData = tokens > 0 || cost > 0 || errors > 0;

    setArc('token', hasData ? Math.min(tokens / limit, 1) : 0);
    setArc('cost', hasData ? Math.min(cost / COST_REF, 1) : 0);
    setArc('error', hasData && errors > 0 ? Math.min(errors / ERR_REF, 1) : 0);

    if (centerText) {
      centerText.textContent = hasData ? '$' + cost.toFixed(2) : '\u2014';
    }
  }

  function setArc(name, ratio) {
    var seg = segs[name];
    if (!seg) return;
    var len = ratio * SLOT;
    if (len < 0.5) len = 0;
    seg.el.style.strokeDasharray = len + ' ' + (C - len);
  }

  function showTooltip() {
    var tokens = SC.state.tokenCount || SC.state.tokens.used || 0;
    var limit = SC.state.tokens.limit || 200000;
    var cost = SC.state.estimatedCost || SC.state.cost || 0;
    var errors = SC.state.errorCount || 0;
    var pct = limit > 0 ? ((tokens / limit) * 100).toFixed(1) : '0.0';
    tooltip.textContent = 'Tokens: ' + tokens.toLocaleString() + '/' + limit.toLocaleString() + ' (' + pct + '%)\nCost: $' + cost.toFixed(4) + '\nErrors: ' + errors;
    tooltip.style.opacity = '1';
  }

  function moveTooltip(e) {
    var tw = tooltip.offsetWidth;
    var th = tooltip.offsetHeight;
    tooltip.style.left = Math.max(4, Math.min(e.clientX - tw / 2, window.innerWidth - tw - 4)) + 'px';
    tooltip.style.top = Math.max(4, e.clientY - th - 10) + 'px';
  }

  function hideTooltip() {
    tooltip.style.opacity = '0';
  }

  SC.initStatusRing = initStatusRing;
  SC.updateStatusRing = updateStatusRing;
})();
