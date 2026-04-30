// SmartClaw - Onboarding Interactive Tutorial
(function() {
  'use strict';

  var modal = null;
  var currentStep = 0;
  var onboardingState = null;

  function createModal() {
    if (modal) return modal;
    modal = document.createElement('div');
    modal.id = 'onboarding-modal';
    modal.className = 'onboarding-modal hidden';
    modal.innerHTML = [
      '<div class="onboarding-backdrop"></div>',
      '<div class="onboarding-content">',
      '  <div class="onboarding-header">',
      '    <span class="onboarding-title">🚀 SmartClaw Onboarding</span>',
      '    <button class="onboarding-close" aria-label="Close">&times;</button>',
      '  </div>',
      '  <div class="onboarding-progress">',
      '    <div class="onboarding-progress-bar" id="onboarding-progress-fill"></div>',
      '    <span class="onboarding-progress-text" id="onboarding-progress-text">0/3</span>',
      '  </div>',
      '  <div class="onboarding-body" id="onboarding-body"></div>',
      '  <div class="onboarding-footer" id="onboarding-footer"></div>',
      '</div>'
    ].join('');
    document.body.appendChild(modal);

    modal.querySelector('.onboarding-backdrop').addEventListener('click', hide);
    modal.querySelector('.onboarding-close').addEventListener('click', hide);

    addStyles();
    return modal;
  }

  function addStyles() {
    if (document.getElementById('onboarding-styles')) return;
    var style = document.createElement('style');
    style.id = 'onboarding-styles';
    style.textContent = [
      '.onboarding-modal { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 10000; display: flex; align-items: center; justify-content: center; }',
      '.onboarding-modal.hidden { display: none; }',
      '.onboarding-backdrop { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.6); }',
      '.onboarding-content { position: relative; background: var(--bg-secondary, #1e293b); border: 1px solid var(--border, #334155); border-radius: 12px; width: 520px; max-width: 90vw; max-height: 85vh; overflow-y: auto; box-shadow: 0 25px 50px rgba(0,0,0,0.5); }',
      '.onboarding-header { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border, #334155); }',
      '.onboarding-title { font-size: 16px; font-weight: 600; color: var(--text-primary, #e2e8f0); }',
      '.onboarding-close { background: none; border: none; color: var(--text-muted, #94a3b8); font-size: 20px; cursor: pointer; padding: 4px 8px; border-radius: 4px; }',
      '.onboarding-close:hover { background: var(--bg-hover, #334155); color: var(--text-primary, #e2e8f0); }',
      '.onboarding-progress { padding: 12px 20px; display: flex; align-items: center; gap: 10px; }',
      '.onboarding-progress-bar { flex: 1; height: 4px; background: var(--bg-hover, #334155); border-radius: 2px; overflow: hidden; }',
      '.onboarding-progress-bar::after { content: ""; display: block; height: 100%; background: var(--accent, #8b5cf6); border-radius: 2px; transition: width 0.3s ease; }',
      '.onboarding-progress-bar[data-fill="1"]::after { width: 33%; }',
      '.onboarding-progress-bar[data-fill="2"]::after { width: 66%; }',
      '.onboarding-progress-bar[data-fill="3"]::after { width: 100%; }',
      '.onboarding-progress-text { font-size: 12px; color: var(--text-muted, #94a3b8); min-width: 28px; text-align: right; }',
      '.onboarding-body { padding: 24px 20px; }',
      '.onboarding-step-title { font-size: 20px; font-weight: 700; color: var(--text-primary, #e2e8f0); margin-bottom: 8px; }',
      '.onboarding-step-desc { font-size: 14px; color: var(--text-secondary, #cbd5e1); margin-bottom: 20px; line-height: 1.5; }',
      '.onboarding-prompt-box { background: var(--bg-primary, #0f172a); border: 1px solid var(--border, #334155); border-radius: 8px; padding: 12px 16px; margin-bottom: 16px; display: flex; align-items: center; justify-content: space-between; cursor: pointer; transition: border-color 0.2s; }',
      '.onboarding-prompt-box:hover { border-color: var(--accent, #8b5cf6); }',
      '.onboarding-prompt-text { font-size: 14px; color: var(--text-primary, #e2e8f0); font-family: monospace; }',
      '.onboarding-prompt-send { background: var(--accent, #8b5cf6); color: #000; border: none; border-radius: 6px; padding: 6px 14px; font-size: 12px; font-weight: 600; cursor: pointer; white-space: nowrap; }',
      '.onboarding-prompt-send:hover { opacity: 0.9; }',
      '.onboarding-insight { background: rgba(139,92,246,0.1); border-left: 3px solid var(--accent, #8b5cf6); padding: 12px 16px; border-radius: 0 8px 8px 0; margin-top: 16px; }',
      '.onboarding-insight-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--accent, #8b5cf6); margin-bottom: 4px; font-weight: 600; }',
      '.onboarding-insight-text { font-size: 13px; color: var(--text-secondary, #cbd5e1); line-height: 1.5; }',
      '.onboarding-footer { padding: 16px 20px; border-top: 1px solid var(--border, #334155); display: flex; justify-content: flex-end; gap: 8px; }',
      '.onboarding-btn { border: none; border-radius: 6px; padding: 8px 18px; font-size: 13px; font-weight: 600; cursor: pointer; transition: opacity 0.2s; }',
      '.onboarding-btn:hover { opacity: 0.9; }',
      '.onboarding-btn-primary { background: var(--accent, #8b5cf6); color: #000; }',
      '.onboarding-btn-ghost { background: transparent; color: var(--text-muted, #94a3b8); border: 1px solid var(--border, #334155); }',
      '.onboarding-btn-ghost:hover { border-color: var(--text-muted, #94a3b8); }',
      '.onboarding-complete-icon { font-size: 48px; text-align: center; margin-bottom: 12px; }',
      '.onboarding-complete-title { font-size: 20px; font-weight: 700; color: var(--text-primary, #e2e8f0); text-align: center; margin-bottom: 8px; }',
      '.onboarding-complete-text { font-size: 14px; color: var(--text-secondary, #cbd5e1); text-align: center; line-height: 1.5; margin-bottom: 20px; }',
      '.onboarding-skills-list { display: flex; flex-direction: column; gap: 6px; margin-bottom: 16px; }',
      '.onboarding-skill-item { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text-secondary, #cbd5e1); padding: 6px 10px; background: var(--bg-primary, #0f172a); border-radius: 6px; }',
      '.onboarding-skill-item .skill-check { color: #10b981; font-weight: 700; }',
      '.onboarding-nav-btn { position: relative; display: flex; align-items: center; gap: 8px; }',
      '.onboarding-nav-btn .nav-badge { position: absolute; top: -4px; right: -4px; background: var(--accent, #8b5cf6); color: #000; font-size: 9px; font-weight: 700; border-radius: 50%; width: 16px; height: 16px; display: flex; align-items: center; justify-content: center; }',
    ].join('\n');
    document.head.appendChild(style);
  }

  function show() {
    createModal();
    modal.classList.remove('hidden');
    render();
  }

  function hide() {
    if (modal) modal.classList.add('hidden');
  }

  function render() {
    var body = document.getElementById('onboarding-body');
    var footer = document.getElementById('onboarding-footer');
    var progressBar = document.getElementById('onboarding-progress-fill');
    var progressText = document.getElementById('onboarding-progress-text');

    if (!body || !onboardingState) return;

    var step = onboardingState.step;

    if (step === 0) {
      progressBar.setAttribute('data-fill', '0');
      progressText.textContent = '0/3';
      body.innerHTML = [
        '<div class="onboarding-complete-icon">🧠</div>',
        '<div class="onboarding-complete-title">Learn how SmartClaw gets smarter</div>',
        '<div class="onboarding-complete-text">In 3 simple steps, you\'ll see how SmartClaw learns your workflow and creates reusable skills automatically. This takes about 5 minutes.</div>',
      ].join('');
      footer.innerHTML = '<button class="onboarding-btn onboarding-btn-primary" id="onboarding-start-btn">Get Started</button>';
      var startBtn = document.getElementById('onboarding-start-btn');
      if (startBtn) startBtn.addEventListener('click', startOnboarding);
      return;
    }

    if (step >= 4) {
      progressBar.setAttribute('data-fill', '3');
      progressText.textContent = '3/3';
      renderComplete();
      return;
    }

    progressBar.setAttribute('data-fill', String(step));
    progressText.textContent = step + '/3';

    var stepData = getStepData(step);
    if (!stepData) return;

    body.innerHTML = [
      '<div class="onboarding-step-title">Step ' + stepData.step + ': ' + SC.escapeHtml(stepData.title) + '</div>',
      '<div class="onboarding-step-desc">' + SC.escapeHtml(stepData.description) + '</div>',
      '<div class="onboarding-prompt-box" id="onboarding-prompt-box">',
      '  <span class="onboarding-prompt-text">💬 ' + SC.escapeHtml(stepData.prompt) + '</span>',
      '  <button class="onboarding-prompt-send" id="onboarding-try-btn">Try it</button>',
      '</div>',
      '<div class="onboarding-insight">',
      '  <div class="onboarding-insight-label">💡 What you\'ll learn</div>',
      '  <div class="onboarding-insight-text">' + SC.escapeHtml(stepData.insight) + '</div>',
      '</div>'
    ].join('');

    footer.innerHTML = '<button class="onboarding-btn onboarding-btn-primary" id="onboarding-next-btn">Complete Step</button>';

    var tryBtn = document.getElementById('onboarding-try-btn');
    if (tryBtn) tryBtn.addEventListener('click', function() {
      sendPrompt(stepData.prompt);
    });

    var promptBox = document.getElementById('onboarding-prompt-box');
    if (promptBox) promptBox.addEventListener('click', function() {
      sendPrompt(stepData.prompt);
    });

    var nextBtn = document.getElementById('onboarding-next-btn');
    if (nextBtn) nextBtn.addEventListener('click', function() {
      advanceStep(stepData.skill_name);
    });
  }

  function renderComplete() {
    var body = document.getElementById('onboarding-body');
    var footer = document.getElementById('onboarding-footer');

    body.innerHTML = [
      '<div class="onboarding-complete-icon">🎉</div>',
      '<div class="onboarding-complete-title">You\'re all set!</div>',
      '<div class="onboarding-complete-text">SmartClaw now knows 3 things about how you work. It will get smarter every session.</div>',
      '<div class="onboarding-skills-list">',
      '  <div class="onboarding-skill-item"><span class="skill-check">✓</span> bug-fix-workflow — Your debugging pattern</div>',
      '  <div class="onboarding-skill-item"><span class="skill-check">✓</span> code-explanation — Your preferred style</div>',
      '  <div class="onboarding-skill-item"><span class="skill-check">✓</span> test-runner — Your test workflow</div>',
      '</div>'
    ].join('');

    footer.innerHTML = '<button class="onboarding-btn onboarding-btn-primary" id="onboarding-done-btn">Done</button>';
    var doneBtn = document.getElementById('onboarding-done-btn');
    if (doneBtn) doneBtn.addEventListener('click', hide);
  }

  var stepsData = [
    null,
    { step: 1, title: 'Fix a Bug', description: 'See how SmartClaw learns your debugging pattern and creates a reusable skill.', prompt: 'Ask me to fix a simple bug', skill_name: 'bug-fix-workflow', insight: 'I noticed your debugging pattern and created a skill. Next time, I\'ll use it automatically.' },
    { step: 2, title: 'Explain Code', description: 'Watch SmartClaw adapt to your preferred explanation style.', prompt: 'Ask me to explain some code', skill_name: 'code-explanation', insight: 'I\'m learning you prefer concise explanations — I\'ll adjust my style.' },
    { step: 3, title: 'Run Tests', description: 'See how SmartClaw saves your test workflow for future reuse.', prompt: 'Ask me to run the tests', skill_name: 'test-runner', insight: 'I\'ve saved your test workflow. SmartClaw now knows 3 things about how you work.' },
  ];

  function getStepData(n) {
    if (n >= 1 && n < stepsData.length) return stepsData[n];
    return null;
  }

  function sendPrompt(text) {
    var input = SC.$('#input');
    if (input) {
      input.value = text;
      input.focus();
      input.style.height = 'auto';
      input.style.height = Math.min(input.scrollHeight, 200) + 'px';
    }
    hide();
  }

  function startOnboarding() {
    fetch('/api/onboarding/start', { method: 'POST', headers: { 'Content-Type': 'application/json' } })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.state) {
          onboardingState = data.state;
          render();
        }
      })
      .catch(function(err) {
        console.error('Onboarding start error:', err);
      });
  }

  function advanceStep(skillCreated) {
    fetch('/api/onboarding/step', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ skill_created: skillCreated })
    })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.state) {
          onboardingState = data.state;
          render();
          updateNavButton();
        }
      })
      .catch(function(err) {
        console.error('Onboarding step error:', err);
      });
  }

  function fetchStatus() {
    fetch('/api/onboarding/status')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        onboardingState = data;
        updateNavButton();
        if (data.step === 0) {
          showWelcomePrompt();
        }
      })
      .catch(function() {
        onboardingState = { step: 0 };
        updateNavButton();
      });
  }

  function showWelcomePrompt() {
    var welcome = SC.$('#welcome');
    if (!welcome) return;
    var existing = welcome.querySelector('.onboarding-welcome-cta');
    if (existing) return;

    var cta = document.createElement('div');
    cta.className = 'onboarding-welcome-cta';
    cta.style.cssText = 'margin-top: 16px; text-align: center;';
    cta.innerHTML = '<button class="onboarding-btn onboarding-btn-primary" style="font-size: 14px; padding: 10px 24px;">🚀 Take the 5-minute tour</button>';
    cta.querySelector('button').addEventListener('click', show);
    welcome.appendChild(cta);
  }

  function updateNavButton() {
    var btn = SC.$('#nav-onboarding');
    if (!btn) return;
    var badge = btn.querySelector('.nav-badge');
    if (onboardingState && onboardingState.step >= 1 && onboardingState.step < 4) {
      if (!badge) {
        badge = document.createElement('span');
        badge.className = 'nav-badge';
        btn.querySelector('span').parentNode.appendChild(badge);
      }
      badge.textContent = onboardingState.step;
    } else if (onboardingState && onboardingState.step >= 4) {
      btn.style.display = 'none';
    }
  }

  SC.onboarding = {
    init: fetchStatus,
    show: show,
    hide: hide,
    getState: function() { return onboardingState; }
  };
})();
