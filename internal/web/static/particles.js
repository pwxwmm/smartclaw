// SmartClaw - Ambient Particles
(function() {
  'use strict';

  var CFG = {
    max: 40,
    radiusMin: 1,
    radiusMax: 3,
    opacityMin: 0.1,
    opacityMax: 0.25,
    speedMin: 0.1,
    speedMax: 0.4,
    colors: ['#8b5cf6', '#7c3aed', '#a78bfa'],
    idleDelay: 5000,
    fadeRate: 0.02,
    lifetimeMin: 300,
    lifetimeMax: 800
  };

  var canvas, ctx, particles, raf, running, idle, masterAlpha, idleTimer;
  var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function rand(a, b) { return a + Math.random() * (b - a); }

  function createParticle() {
    var life = Math.floor(rand(CFG.lifetimeMin, CFG.lifetimeMax));
    return {
      x: rand(0, canvas.width),
      y: rand(0, canvas.height),
      r: rand(CFG.radiusMin, CFG.radiusMax),
      vx: rand(-CFG.speedMax, CFG.speedMax),
      vy: rand(-CFG.speedMax, CFG.speedMax),
      color: CFG.colors[Math.floor(Math.random() * CFG.colors.length)],
      baseOpacity: rand(CFG.opacityMin, CFG.opacityMax),
      age: 0,
      life: life,
      fadeIn: Math.floor(life * 0.15),
      fadeOut: Math.floor(life * 0.25)
    };
  }

  function particleAlpha(p) {
    var a = p.baseOpacity;
    if (p.age < p.fadeIn) a *= p.age / p.fadeIn;
    else if (p.age > p.life - p.fadeOut) a *= (p.life - p.age) / p.fadeOut;
    return Math.max(0, a);
  }

  function init() {
    if (reducedMotion) return;
    canvas = document.createElement('canvas');
    canvas.style.cssText = 'position:fixed;top:0;left:0;width:100vw;height:100vh;pointer-events:none;z-index:0;';
    document.body.appendChild(canvas);
    ctx = canvas.getContext('2d');
    resize();
    particles = [];
    masterAlpha = 0;
    idle = false;
    running = true;

    for (var i = 0; i < CFG.max; i++) {
      var p = createParticle();
      p.age = Math.floor(rand(0, p.life * 0.8));
      particles.push(p);
    }

    window.addEventListener('resize', resize);
    document.addEventListener('mousemove', resetIdle);
    document.addEventListener('keydown', resetIdle);
    document.addEventListener('scroll', resetIdle, true);
    document.addEventListener('visibilitychange', onVisibility);
    resetIdle();
    loop();
  }

  function resize() {
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
  }

  function resetIdle() {
    clearTimeout(idleTimer);
    idle = false;
    idleTimer = setTimeout(function() { idle = true; }, CFG.idleDelay);
  }

  function onVisibility() {
    if (document.hidden) {
      running = false;
      cancelAnimationFrame(raf);
    } else {
      running = true;
      loop();
    }
  }

  function loop() {
    if (!running) return;
    raf = requestAnimationFrame(tick);
  }

  function tick() {
    if (!running) return;

    if (idle && masterAlpha < 1) {
      masterAlpha = Math.min(1, masterAlpha + CFG.fadeRate);
    } else if (!idle && masterAlpha > 0) {
      masterAlpha = Math.max(0, masterAlpha - CFG.fadeRate);
    }

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    if (masterAlpha > 0) {
      for (var i = 0; i < particles.length; i++) {
        var p = particles[i];
        p.age++;
        if (p.age >= p.life) {
          particles[i] = createParticle();
          continue;
        }
        p.x += p.vx;
        p.y += p.vy;
        if (p.x < -10) p.x = canvas.width + 10;
        if (p.x > canvas.width + 10) p.x = -10;
        if (p.y < -10) p.y = canvas.height + 10;
        if (p.y > canvas.height + 10) p.y = -10;

        var a = particleAlpha(p) * masterAlpha;
        if (a < 0.001) continue;

        ctx.beginPath();
        ctx.arc(p.x, p.y, p.r, 0, 6.2832);
        ctx.fillStyle = p.color;
        ctx.globalAlpha = a;
        ctx.fill();
      }
      ctx.globalAlpha = 1;
    }

    loop();
  }

  function destroy() {
    running = false;
    cancelAnimationFrame(raf);
    clearTimeout(idleTimer);
    window.removeEventListener('resize', resize);
    document.removeEventListener('mousemove', resetIdle);
    document.removeEventListener('keydown', resetIdle);
    document.removeEventListener('scroll', resetIdle, true);
    document.removeEventListener('visibilitychange', onVisibility);
    if (canvas && canvas.parentNode) canvas.parentNode.removeChild(canvas);
    canvas = null;
    ctx = null;
    particles = null;
  }

  SC.initParticles = function(opts) {
    if (opts) {
      for (var k in opts) {
        if (opts.hasOwnProperty(k) && CFG.hasOwnProperty(k)) CFG[k] = opts[k];
      }
    }
    if (canvas) destroy();
    init();
  };
})();
