package services

import (
	"context"
	"sync"
)

type VoiceService struct {
	enabled bool
	muted   bool
	volume  float64
	mu      sync.RWMutex
	stt     SpeechToText
	tts     TextToSpeech
}

type SpeechToText interface {
	Transcribe(ctx context.Context, audio []byte) (string, error)
}

type TextToSpeech interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

func NewVoiceService() *VoiceService {
	return &VoiceService{
		enabled: false,
		muted:   false,
		volume:  1.0,
	}
}

func (v *VoiceService) Enable() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enabled = true
}

func (v *VoiceService) Disable() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enabled = false
}

func (v *VoiceService) IsEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.enabled
}

func (v *VoiceService) Mute() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.muted = true
}

func (v *VoiceService) Unmute() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.muted = false
}

func (v *VoiceService) IsMuted() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.muted
}

func (v *VoiceService) SetVolume(volume float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	v.volume = volume
}

func (v *VoiceService) GetVolume() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.volume
}

func (v *VoiceService) Transcribe(ctx context.Context, audio []byte) (string, error) {
	if !v.IsEnabled() {
		return "", nil
	}

	if v.stt == nil {
		return "", nil
	}

	return v.stt.Transcribe(ctx, audio)
}

func (v *VoiceService) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if !v.IsEnabled() || v.IsMuted() {
		return nil, nil
	}

	if v.tts == nil {
		return nil, nil
	}

	return v.tts.Synthesize(ctx, text)
}

func (v *VoiceService) SetSTT(stt SpeechToText) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.stt = stt
}

func (v *VoiceService) SetTTS(tts TextToSpeech) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.tts = tts
}

type VoiceKeyterms struct {
	keyterms []string
	mu       sync.RWMutex
}

func NewVoiceKeyterms() *VoiceKeyterms {
	return &VoiceKeyterms{
		keyterms: []string{},
	}
}

func (v *VoiceKeyterms) Add(keyterm string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keyterms = append(v.keyterms, keyterm)
}

func (v *VoiceKeyterms) Remove(keyterm string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for i, k := range v.keyterms {
		if k == keyterm {
			v.keyterms = append(v.keyterms[:i], v.keyterms[i+1:]...)
			break
		}
	}
}

func (v *VoiceKeyterms) GetAll() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]string, len(v.keyterms))
	copy(result, v.keyterms)
	return result
}
