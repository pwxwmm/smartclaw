// SmartClaw - Audio Feedback
(function() {
  'use strict';

  var AUDIO_KEY = 'smartclaw-sound-enabled';
  var audioCtx = null;

  function getAudioCtx() {
    if (!audioCtx) {
      audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    }
    return audioCtx;
  }

  function isSoundEnabled() {
    return localStorage.getItem(AUDIO_KEY) !== 'false';
  }

  function setSoundEnabled(enabled) {
    localStorage.setItem(AUDIO_KEY, enabled ? 'true' : 'false');
  }

  function playTone(freq, duration, type, volume) {
    if (!isSoundEnabled()) return;
    try {
      var ctx = getAudioCtx();
      if (ctx.state === 'suspended') ctx.resume();
      var osc = ctx.createOscillator();
      var gain = ctx.createGain();
      osc.type = type || 'sine';
      osc.frequency.value = freq;
      gain.gain.value = volume || 0.08;
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + (duration || 0.1));
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(ctx.currentTime);
      osc.stop(ctx.currentTime + (duration || 0.1));
    } catch {}
  }

  function playMessageSent() {
    playTone(880, 0.08, 'sine', 0.06);
  }

  function playMessageReceived() {
    playTone(523, 0.08, 'sine', 0.05);
    setTimeout(function() { playTone(659, 0.1, 'sine', 0.05); }, 80);
  }

  function playSuccess() {
    playTone(523, 0.08, 'sine', 0.06);
    setTimeout(function() { playTone(659, 0.08, 'sine', 0.06); }, 80);
    setTimeout(function() { playTone(784, 0.12, 'sine', 0.06); }, 160);
  }

  function playError() {
    playTone(330, 0.15, 'square', 0.04);
    setTimeout(function() { playTone(262, 0.2, 'square', 0.04); }, 150);
  }

  function playClick() {
    playTone(1200, 0.03, 'sine', 0.03);
  }

  function playNotification() {
    playTone(784, 0.1, 'sine', 0.05);
    setTimeout(function() { playTone(988, 0.12, 'sine', 0.05); }, 100);
  }

  SC.audio = {
    isSoundEnabled: isSoundEnabled,
    setSoundEnabled: setSoundEnabled,
    messageSent: playMessageSent,
    messageReceived: playMessageReceived,
    success: playSuccess,
    error: playError,
    click: playClick,
    notification: playNotification
  };
})();
