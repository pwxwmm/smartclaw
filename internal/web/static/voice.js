(function() {
  'use strict';

  let recognition = null;
  let sttFailed = false;

  function startVoice() {
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;

    SC.setState('isRecording', true);
    sttFailed = false;
    SC.$('#voice-bar').classList.remove('hidden');

    navigator.mediaDevices.getUserMedia({ audio: true }).then(stream => {
      SC.state.mediaStream = stream;
      SC.state.audioContext = new (window.AudioContext || window.webkitAudioContext)();
      const source = SC.state.audioContext.createMediaStreamSource(stream);
      SC.state.analyser = SC.state.audioContext.createAnalyser();
      SC.state.analyser.fftSize = 2048;
      source.connect(SC.state.analyser);
      drawWaveform();

      if (SpeechRecognition) {
        recognition = new SpeechRecognition();
        recognition.continuous = true;
        recognition.interimResults = true;
        recognition.lang = navigator.language || 'en-US';

        let finalTranscript = '';
        recognition.onresult = (e) => {
          let interim = '';
          for (let i = e.resultIndex; i < e.results.length; i++) {
            if (e.results[i].isFinal) {
              finalTranscript += e.results[i][0].transcript;
            } else {
              interim += e.results[i][0].transcript;
            }
          }
          SC.$('#voice-status').textContent = finalTranscript + interim || 'Listening...';
        };

        recognition.onerror = () => {
          sttFailed = true;
          SC.$('#voice-status').textContent = 'Recording... (STT unavailable)';
        };

        recognition.onend = () => {
          if (sttFailed) return;
          if (!SC.state.isRecording) return;
          const text = finalTranscript.trim();
          if (text) {
            const input = SC.$('#input');
            input.value += (input.value ? ' ' : '') + text;
            input.style.height = 'auto';
            input.style.height = Math.min(input.scrollHeight, 200) + 'px';
          }
          stopVoice();
        };

        try { recognition.start(); } catch(e) {}
        SC.$('#voice-status').textContent = 'Listening...';
      } else {
        SC.$('#voice-status').textContent = 'Recording... (no STT support)';
      }
    }).catch(() => {
      SC.toast('Microphone access denied', 'error');
      stopVoice();
    });
  }

  function stopVoice() {
    if (recognition) { try { recognition.stop(); } catch(e) {} recognition = null; }
    if (SC.state.mediaStream) SC.state.mediaStream.getTracks().forEach(t => t.stop());
    if (SC.state.audioContext) { try { SC.state.audioContext.close(); } catch(e) {} }
    if (SC.state.animFrame) cancelAnimationFrame(SC.state.animFrame);
    SC.setState('isRecording', false);
    SC.$('#voice-bar').classList.add('hidden');
  }

  function drawWaveform() {
    if (!SC.state.isRecording || !SC.state.analyser) return;
    const canvas = SC.$('#waveform');
    const ctx = canvas.getContext('2d');
    const w = canvas.width, h = canvas.height;
    const data = new Uint8Array(SC.state.analyser.frequencyBinCount);
    SC.state.analyser.getByteTimeDomainData(data);
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = SC.getCSS('--bg-0');
    ctx.fillRect(0, 0, w, h);
    ctx.lineWidth = 2;
    ctx.strokeStyle = SC.getCSS('--accent');
    ctx.beginPath();
    const sliceW = w / data.length;
    let x = 0;
    for (let i = 0; i < data.length; i++) {
      const v = data[i] / 128.0;
      const y = (v * h) / 2;
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
      x += sliceW;
    }
    ctx.lineTo(w, h / 2);
    ctx.stroke();
    SC.state.animFrame = requestAnimationFrame(drawWaveform);
  }

  SC.startVoice = startVoice;
  SC.stopVoice = stopVoice;
})();
