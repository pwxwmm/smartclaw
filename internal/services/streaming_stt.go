package services

import (
	"context"
	"sync"
	"time"
)

type StreamTranscription struct {
	Text       string
	Confidence float64
	IsFinal    bool
	StartTime  float64
	EndTime    float64
}

type StreamingSTT struct {
	provider  string
	streaming bool
	results   []StreamTranscription
	finalText string
	mu        sync.Mutex
}

func NewStreamingSTT(provider string) *StreamingSTT {
	return &StreamingSTT{
		provider:  provider,
		streaming: false,
		results:   make([]StreamTranscription, 0),
	}
}

func (s *StreamingSTT) StartStreaming(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.streaming {
		return nil
	}

	s.streaming = true
	return nil
}

func (s *StreamingSTT) StopStreaming() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.streaming = false
	return nil
}

func (s *StreamingSTT) ProcessChunk(ctx context.Context, chunk []byte) (string, error) {
	if !s.streaming {
		return "", nil
	}

	return "transcribed text", nil
}

func (s *StreamingSTT) AddResult(result StreamTranscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if result.IsFinal {
		s.finalText += result.Text + " "
	} else {
		s.results = append(s.results, result)
	}
}

func (s *StreamingSTT) GetCurrentText() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	text := s.finalText
	for _, r := range s.results {
		text += r.Text + " "
	}
	return text
}

func (s *StreamingSTT) GetFinalText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalText
}

func (s *StreamingSTT) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results = s.results[:0]
	s.finalText = ""
}

func (s *StreamingSTT) IsStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streaming
}

type VoiceDetector struct {
	threshold  float64
	sampleRate int
}

func NewVoiceDetector(threshold float64) *VoiceDetector {
	return &VoiceDetector{
		threshold:  threshold,
		sampleRate: 16000,
	}
}

func (d *VoiceDetector) DetectVoice(audio []byte) bool {
	if len(audio) == 0 {
		return false
	}

	sum := 0.0
	for i := 0; i < len(audio); i++ {
		sample := float64(int8(audio[i]))
		sum += sample * sample
	}

	rms := sum / float64(len(audio))
	return rms > d.threshold*d.threshold*128*128
}

func (d *VoiceDetector) SetThreshold(threshold float64) {
	d.threshold = threshold
}

func (d *VoiceDetector) GetThreshold() float64 {
	return d.threshold
}

type AudioBuffer struct {
	data     []byte
	maxSize  int
	duration time.Duration
	mu       sync.Mutex
}

func NewAudioBuffer(maxSize int, duration time.Duration) *AudioBuffer {
	return &AudioBuffer{
		data:     make([]byte, 0),
		maxSize:  maxSize,
		duration: duration,
	}
}

func (b *AudioBuffer) Write(data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data = append(b.data, data...)

	if b.maxSize > 0 && len(b.data) > b.maxSize {
		b.data = b.data[len(b.data)-b.maxSize:]
	}

	return nil
}

func (b *AudioBuffer) Read() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

func (b *AudioBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = b.data[:0]
}

func (b *AudioBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.data)
}
