// SmartClaw - Arena Mode
(function() {
  'use strict';

  var arenaState = {
    results: null,
    revealed: false,
    voted: false,
    models: []
  };

  function initArena() {
    var arenaBtn = SC.$('#btn-arena');
    if (arenaBtn) {
      arenaBtn.addEventListener('click', function() {
        openArenaModal();
      });
    }

    var input = SC.$('#input');
    if (input) {
      input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && e.ctrlKey && e.shiftKey) {
          e.preventDefault();
          var text = input.value.trim();
          if (text) startArena(text);
        }
      });
    }

    var modalClose = SC.$('#arena-modal-close');
    if (modalClose) {
      modalClose.addEventListener('click', closeArenaModal);
    }

    var backdrop = SC.$('#arena-modal');
    if (backdrop) {
      backdrop.addEventListener('click', function(e) {
        if (e.target === backdrop) closeArenaModal();
      });
    }

    var revealBtn = SC.$('#arena-reveal-btn');
    if (revealBtn) {
      revealBtn.addEventListener('click', revealModels);
    }

    var voteA = SC.$('#arena-vote-a');
    var voteB = SC.$('#arena-vote-b');
    var voteTie = SC.$('#arena-vote-tie');
    var voteNeither = SC.$('#arena-vote-neither');

    if (voteA) voteA.addEventListener('click', function() { castVote('A'); });
    if (voteB) voteB.addEventListener('click', function() { castVote('B'); });
    if (voteTie) voteTie.addEventListener('click', function() { castVote('tie'); });
    if (voteNeither) voteNeither.addEventListener('click', function() { castVote('neither'); });
  }

  function openArenaModal() {
    var modal = SC.$('#arena-modal');
    if (!modal) return;

    resetArena();
    modal.classList.remove('hidden');

    var input = SC.$('#input');
    if (input) input.focus();
  }

  function closeArenaModal() {
    var modal = SC.$('#arena-modal');
    if (modal) modal.classList.add('hidden');
  }

  function resetArena() {
    arenaState.results = null;
    arenaState.revealed = false;
    arenaState.voted = false;
    arenaState.models = [];

    var panelA = SC.$('#arena-panel-a');
    var panelB = SC.$('#arena-panel-b');
    if (panelA) { panelA.querySelector('.arena-panel-content').innerHTML = '<div class="arena-loading"><span class="spin"></span>Waiting...</div>'; }
    if (panelB) { panelB.querySelector('.arena-panel-content').innerHTML = '<div class="arena-loading"><span class="spin"></span>Waiting...</div>'; }

    var headerA = SC.$('#arena-header-a');
    var headerB = SC.$('#arena-header-b');
    if (headerA) headerA.textContent = 'Model A';
    if (headerB) headerB.textContent = 'Model B';

    var votes = SC.$('.arena-vote-btn');
    if (votes) votes.forEach(function(b) { b.disabled = true; });

    var revealBtn = SC.$('#arena-reveal-btn');
    if (revealBtn) { revealBtn.disabled = true; revealBtn.classList.add('hidden'); }

    var statusEl = SC.$('#arena-status');
    if (statusEl) statusEl.textContent = '';
  }

  function startArena(prompt) {
    var modal = SC.$('#arena-modal');
    if (modal) modal.classList.remove('hidden');

    resetArena();

    var panelA = SC.$('#arena-panel-a');
    var panelB = SC.$('#arena-panel-b');
    if (panelA) panelA.querySelector('.arena-panel-content').innerHTML = '<div class="arena-loading"><span class="spin"></span>Generating...</div>';
    if (panelB) panelB.querySelector('.arena-panel-content').innerHTML = '<div class="arena-loading"><span class="spin"></span>Generating...</div>';

    var statusEl = SC.$('#arena-status');
    if (statusEl) statusEl.textContent = 'Running arena comparison...';

    var models = getArenaModels();
    SC.wsSend('arena_chat', { content: prompt, models: models });
  }

  function getArenaModels() {
    var select = SC.$('#model-select');
    var models = [];
    if (select) {
      var opts = select.querySelectorAll('option');
      opts.forEach(function(o) { models.push(o.value); });
    }
    if (models.length < 2) {
      models = ['sre-model', 'glm-4-plus'];
    }
    return models.slice(0, 2);
  }

  function handleArenaStart(msg) {
    var models = (msg.data && msg.data.models) || ['Model A', 'Model B'];
    arenaState.models = models;

    var headerA = SC.$('#arena-header-a');
    var headerB = SC.$('#arena-header-b');
    if (headerA && models[0]) headerA.textContent = models[0];
    if (headerB && models[1]) headerB.textContent = models[1];

    SC.toast('Arena started: ' + models.join(' vs '), 'info');
  }

  function handleArenaResult(msg) {
    var results = (msg.data && msg.data.results) || [];
    arenaState.results = results;

    var panelA = SC.$('#arena-panel-a');
    var panelB = SC.$('#arena-panel-b');

    results.forEach(function(r) {
      var panel = (r.label === 'Model A') ? panelA : panelB;
      if (!panel) return;

      var contentEl = panel.querySelector('.arena-panel-content');
      if (!contentEl) return;

      if (r.error) {
        contentEl.innerHTML = '<div class="arena-error">' + SC.escapeHtml(r.error) + '</div>';
      } else {
        contentEl.innerHTML = '<div class="arena-response">' + SC.renderMarkdown(r.content || '') + '</div>';
      }

      var metaEl = panel.querySelector('.arena-panel-meta');
      if (metaEl && r.duration > 0) {
        var tokInfo = '';
        if (r.tokens && (r.tokens.input_tokens || r.tokens.output_tokens)) {
          tokInfo = ' | In: ' + r.tokens.input_tokens + ' Out: ' + r.tokens.output_tokens;
        }
        metaEl.textContent = r.duration + 'ms' + tokInfo;
      }
    });

    var votes = SC.$$('.arena-vote-btn');
    votes.forEach(function(b) { b.disabled = false; });

    var revealBtn = SC.$('#arena-reveal-btn');
    if (revealBtn) { revealBtn.disabled = false; revealBtn.classList.remove('hidden'); }

    var statusEl = SC.$('#arena-status');
    if (statusEl) statusEl.textContent = 'Both models responded. Vote or reveal!';
  }

  function revealModels() {
    if (!arenaState.results) return;
    arenaState.revealed = true;

    var panelA = SC.$('#arena-panel-a');
    var panelB = SC.$('#arena-panel-b');

    arenaState.results.forEach(function(r) {
      var panel = (r.label === 'Model A') ? panelA : panelB;
      if (!panel) return;

      var header = panel.querySelector('.arena-panel-header');
      if (header) {
        var reveal = header.querySelector('.arena-model-reveal');
        if (!reveal) {
          reveal = document.createElement('span');
          reveal.className = 'arena-model-reveal';
          header.appendChild(reveal);
        }
        reveal.textContent = ' → ' + r.model;
      }
    });

    var revealBtn = SC.$('#arena-reveal-btn');
    if (revealBtn) { revealBtn.disabled = true; revealBtn.textContent = 'Revealed'; }
  }

  function castVote(choice) {
    if (arenaState.voted || !arenaState.results || arenaState.results.length < 2) return;
    arenaState.voted = true;

    var resultA = arenaState.results[0];
    var resultB = arenaState.results[1];
    var winner, loser;

    if (choice === 'A') {
      winner = resultA.model;
      loser = resultB.model;
    } else if (choice === 'B') {
      winner = resultB.model;
      loser = resultA.model;
    } else {
      // tie or neither — still record, but both as winner=first for tracking
      winner = resultA.model;
      loser = resultB.model;
    }

    SC.wsSend('arena_vote', {
      winner_model: winner,
      loser_model: loser,
      prompt: arenaState.prompt || '',
      choice: choice
    });

    revealModels();

    var votes = SC.$$('.arena-vote-btn');
    votes.forEach(function(b) { b.disabled = true; });

    var statusEl = SC.$('#arena-status');
    if (statusEl) {
      var labels = { A: 'Model A', B: 'Model B', tie: 'Tie', neither: 'Neither' };
      statusEl.textContent = 'Vote recorded: ' + (labels[choice] || choice);
    }

    SC.toast('Arena vote recorded!', 'success');
  }

  function handleArenaVoteRecorded(msg) {
    SC.toast(msg.message || 'Vote recorded', 'success');
  }

  SC.initArena = initArena;
  SC.startArena = startArena;
  SC.handleArenaStart = handleArenaStart;
  SC.handleArenaResult = handleArenaResult;
  SC.handleArenaVoteRecorded = handleArenaVoteRecorded;
  SC.closeArenaModal = closeArenaModal;
})();
