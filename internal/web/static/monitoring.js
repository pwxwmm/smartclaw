// SmartClaw - Web Vitals & Error Tracking
(function() {
  'use strict';

  var metricsQueue = [];
  var flushInterval = null;
  var sampleRate = 1.0; // 100% in dev, 10% in prod

  function init() {
    // Detect if production (simplified heuristic)
    if (location.hostname !== 'localhost' && location.hostname !== '127.0.0.1') {
      sampleRate = 0.1;
    }

    if (Math.random() > sampleRate) return; // Skip based on sample rate

    // Collect Web Vitals
    collectWebVitals();

    // Error tracking
    setupErrorTracking();

    // Start periodic flush
    flushInterval = setInterval(flushMetrics, 30000);

    // Flush on page unload
    window.addEventListener('beforeunload', flushMetrics);
  }

  function collectWebVitals() {
    // Use PerformanceObserver for LCP, FID, CLS
    if (!window.PerformanceObserver) return;

    // LCP (Largest Contentful Paint)
    try {
      var lcpObserver = new PerformanceObserver(function(entryList) {
        var entries = entryList.getEntries();
        var lastEntry = entries[entries.length - 1];
        queueMetric('LCP', lastEntry.startTime);
      });
      lcpObserver.observe({ type: 'largest-contentful-paint', buffered: true });
    } catch(e) {}

    // FID (First Input Delay)
    try {
      var fidObserver = new PerformanceObserver(function(entryList) {
        var entries = entryList.getEntries();
        entries.forEach(function(entry) {
          queueMetric('FID', entry.processingStart - entry.startTime);
        });
      });
      fidObserver.observe({ type: 'first-input', buffered: true });
    } catch(e) {}

    // CLS (Cumulative Layout Shift)
    try {
      var clsValue = 0;
      var clsObserver = new PerformanceObserver(function(entryList) {
        entryList.getEntries().forEach(function(entry) {
          if (!entry.hadRecentInput) {
            clsValue += entry.value;
          }
        });
        queueMetric('CLS', clsValue);
      });
      clsObserver.observe({ type: 'layout-shift', buffered: true });
    } catch(e) {}

    // TTFB (Time to First Byte)
    try {
      var navEntry = performance.getEntriesByType('navigation')[0];
      if (navEntry) {
        queueMetric('TTFB', navEntry.responseStart - navEntry.requestStart);
      }
    } catch(e) {}

    // INP (Interaction to Next Paint) - modern replacement for FID
    try {
      var inpObserver = new PerformanceObserver(function(entryList) {
        var entries = entryList.getEntries();
        entries.forEach(function(entry) {
          if (entry.interactionId) {
            queueMetric('INP', entry.duration);
          }
        });
      });
      inpObserver.observe({ type: 'event', buffered: true });
    } catch(e) {}
  }

  function setupErrorTracking() {
    window.addEventListener('error', function(event) {
      queueMetric('JS_ERROR', {
        message: (event.error && event.error.message) || event.message || 'Unknown error',
        filename: event.filename || '',
        lineno: event.lineno || 0,
        colno: event.colno || 0,
      });
    });

    window.addEventListener('unhandledrejection', function(event) {
      queueMetric('PROMISE_ERROR', {
        message: (event.reason && event.reason.message) || String(event.reason) || 'Unhandled rejection',
      });
    });
  }

  function queueMetric(name, value) {
    metricsQueue.push({
      name: name,
      value: typeof value === 'object' ? JSON.stringify(value) : value,
      timestamp: Date.now(),
      url: location.pathname,
    });

    // Flush immediately if queue is large
    if (metricsQueue.length >= 20) {
      flushMetrics();
    }
  }

  function flushMetrics() {
    if (metricsQueue.length === 0) return;

    var batch = metricsQueue.splice(0, metricsQueue.length);

    // Send via sendBeacon (doesn't block)
    if (navigator.sendBeacon) {
      var blob = new Blob([JSON.stringify({ metrics: batch })], { type: 'application/json' });
      navigator.sendBeacon('/api/telemetry/frontend', blob);
    } else {
      // Fallback: fetch with keepalive
      try {
        fetch('/api/telemetry/frontend', {
          method: 'POST',
          body: JSON.stringify({ metrics: batch }),
          headers: { 'Content-Type': 'application/json' },
          keepalive: true,
        }).catch(function() {});
      } catch(e) {}
    }
  }

  SC.monitoring = {
    init: init,
    queueMetric: queueMetric,
    flushMetrics: flushMetrics,
  };
})();
