package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type VoiceMode int

const (
	VoiceModeDisabled VoiceMode = iota
	VoiceModePushToTalk
	VoiceModeAlwaysOn
)

type VoiceConfig struct {
	Mode             VoiceMode `json:"mode"`
	Language         string    `json:"language"`
	Model            string    `json:"model"`
	ApiKey           string    `json:"api_key"`
	ApiEndpoint      string    `json:"api_endpoint"`
	SampleRate       int       `json:"sample_rate"`
	EnableKeyterms   bool      `json:"enable_keyterms"`
	VadEnabled       bool      `json:"vad_enabled"`
	VadThreshold     float64   `json:"vad_threshold"`
	RecordingTimeout int       `json:"recording_timeout"`
	SilenceThreshold int       `json:"silence_threshold"`
}

type VoiceActivityDetector struct {
	threshold  float64
	sampleRate int
	silenceMs  int
}

func NewVAD(threshold float64, sampleRate int, silenceMs int) *VoiceActivityDetector {
	if threshold == 0 {
		threshold = 0.5
	}
	if sampleRate == 0 {
		sampleRate = 16000
	}
	if silenceMs == 0 {
		silenceMs = 2000
	}
	return &VoiceActivityDetector{
		threshold:  threshold,
		sampleRate: sampleRate,
		silenceMs:  silenceMs,
	}
}

func (vad *VoiceActivityDetector) Detect(audioData []byte) bool {
	if len(audioData) < 1024 {
		return true
	}

	var sum int64
	samples := len(audioData) / 2
	for i := 0; i < samples; i++ {
		sample := int16(audioData[i*2]) | int16(audioData[i*2+1])<<8
		if sample < 0 {
			sample = -sample
		}
		sum += int64(sample)
	}

	avg := float64(sum) / float64(samples)
	normalized := avg / 32768.0

	return normalized > vad.threshold
}

func (vad *VoiceActivityDetector) DetectWithCallback(audioData []byte, onVoiceStart, onSilence func()) bool {
	hasVoice := vad.Detect(audioData)

	if hasVoice && onVoiceStart != nil {
		onVoiceStart()
	} else if !hasVoice && onSilence != nil {
		onSilence()
	}

	return hasVoice
}

type TranscriptionResult struct {
	Text       string    `json:"text"`
	Language   string    `json:"language"`
	Duration   float64   `json:"duration"`
	Confidence float64   `json:"confidence"`
	Timestamp  time.Time `json:"timestamp"`
}

type VoiceRecorder struct {
	config      VoiceConfig
	running     bool
	audioBuffer *bytes.Buffer
	mu          sync.Mutex
	cmd         *exec.Cmd
}

type VoiceTranscriber struct {
	config   VoiceConfig
	client   *http.Client
	endpoint string
}

type VoiceManager struct {
	config          VoiceConfig
	recorder        *VoiceRecorder
	transcriber     *VoiceTranscriber
	keyterms        []string
	dialogueSession *VoiceDialogueSession
	mu              sync.RWMutex
}

func NewVoiceManager(config VoiceConfig) *VoiceManager {
	if config.SampleRate == 0 {
		config.SampleRate = 16000
	}
	if config.Language == "" {
		config.Language = "en"
	}
	if config.RecordingTimeout == 0 {
		config.RecordingTimeout = 30
	}
	if config.SilenceThreshold == 0 {
		config.SilenceThreshold = 3
	}

	vm := &VoiceManager{
		config: config,
		recorder: &VoiceRecorder{
			config:      config,
			audioBuffer: bytes.NewBuffer(nil),
		},
		transcriber: &VoiceTranscriber{
			config:   config,
			client:   &http.Client{Timeout: 60 * time.Second},
			endpoint: config.ApiEndpoint,
		},
		keyterms: make([]string, 0),
	}

	if vm.transcriber.endpoint == "" {
		vm.transcriber.endpoint = "https://api.openai.com/v1/audio/transcriptions"
	}

	return vm
}

func (vm *VoiceManager) SetMode(mode VoiceMode) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.config.Mode = mode
}

func (vm *VoiceManager) GetMode() VoiceMode {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.config.Mode
}

func (vm *VoiceManager) AddKeyterm(term string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.keyterms = append(vm.keyterms, term)
}

func (vm *VoiceManager) SetKeyterms(terms []string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.keyterms = terms
}

func (vm *VoiceManager) GetKeyterms() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return append([]string{}, vm.keyterms...)
}

func (vr *VoiceRecorder) StartRecording(ctx context.Context) error {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if vr.running {
		return fmt.Errorf("recording already in progress")
	}

	vr.audioBuffer.Reset()

	platform := detectPlatform()
	var cmd *exec.Cmd

	switch platform {
	case "darwin":
		cmd = exec.CommandContext(ctx, "rec", "-q", "-r",
			fmt.Sprintf("%d", vr.config.SampleRate),
			"-c", "1", "-", "trim", "0",
			fmt.Sprintf("%d", vr.config.RecordingTimeout))
	case "linux":
		cmd = exec.CommandContext(ctx, "arecord", "-q", "-r",
			fmt.Sprintf("%d", vr.config.SampleRate),
			"-c", "1", "-f", "S16_LE", "-t", "wav", "-d",
			fmt.Sprintf("%d", vr.config.RecordingTimeout))
	default:
		return fmt.Errorf("unsupported platform for voice recording: %s", platform)
	}

	var stderr bytes.Buffer
	cmd.Stdout = vr.audioBuffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}

	vr.cmd = cmd
	vr.running = true

	return nil
}

func (vr *VoiceRecorder) StopRecording() ([]byte, error) {
	vr.mu.Lock()
	defer vr.mu.Unlock()

	if !vr.running || vr.cmd == nil {
		return nil, fmt.Errorf("no recording in progress")
	}

	if vr.cmd.Process != nil {
		vr.cmd.Process.Kill()
		vr.cmd.Wait()
	}

	vr.running = false
	audioData := make([]byte, vr.audioBuffer.Len())
	copy(audioData, vr.audioBuffer.Bytes())

	return audioData, nil
}

func (vr *VoiceRecorder) IsRecording() bool {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	return vr.running
}

func (vt *VoiceTranscriber) Transcribe(ctx context.Context, audioData []byte) (*TranscriptionResult, error) {
	if vt.config.ApiKey == "" {
		return nil, fmt.Errorf("API key not configured for voice transcription")
	}

	tmpFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(audioData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}
	tmpFile.Close()

	file, err := os.Open(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := createMultipartWriter(&body, file, vt.config)

	req, err := http.NewRequestWithContext(ctx, "POST", vt.endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+vt.config.ApiKey)

	resp, err := vt.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcription request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcription failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Text string  `json:"text"`
		Lang string  `json:"language"`
		Dur  float64 `json:"duration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &TranscriptionResult{
		Text:      result.Text,
		Language:  result.Lang,
		Duration:  result.Dur,
		Timestamp: time.Now(),
	}, nil
}

func (vt *VoiceTranscriber) TranscribeLocal(ctx context.Context, audioData []byte) (*TranscriptionResult, error) {
	whisperPath := findWhisperBinary()
	if whisperPath == "" {
		return nil, fmt.Errorf("whisper binary not found, please install whisper or configure API key")
	}

	tmpFile, err := os.CreateTemp("", "voice-*.wav")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.Write(audioData)
	tmpFile.Close()

	cmd := exec.CommandContext(ctx, whisperPath,
		tmpFile.Name(),
		"--model", vt.config.Model,
		"--language", vt.config.Language,
		"--output_format", "json",
		"--output_dir", filepath.Dir(tmpFile.Name()),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper failed: %w, stderr: %s", err, stderr.String())
	}

	jsonFile := strings.TrimSuffix(tmpFile.Name(), ".wav") + ".json"
	defer os.Remove(jsonFile)

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper output: %w", err)
	}

	var result struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse whisper output: %w", err)
	}

	return &TranscriptionResult{
		Text:      strings.TrimSpace(result.Text),
		Language:  vt.config.Language,
		Timestamp: time.Now(),
	}, nil
}

func (vm *VoiceManager) StartPushToTalk(ctx context.Context) error {
	if vm.GetMode() != VoiceModePushToTalk {
		return fmt.Errorf("voice mode is not set to push-to-talk")
	}

	return vm.recorder.StartRecording(ctx)
}

func (vm *VoiceManager) StopPushToTalk(ctx context.Context) (*TranscriptionResult, error) {
	audioData, err := vm.recorder.StopRecording()
	if err != nil {
		return nil, err
	}

	if len(audioData) == 0 {
		return nil, fmt.Errorf("no audio data recorded")
	}

	if vm.config.ApiKey != "" {
		return vm.transcriber.Transcribe(ctx, audioData)
	}

	return vm.transcriber.TranscribeLocal(ctx, audioData)
}

func (vm *VoiceManager) TranscribeFile(ctx context.Context, filePath string) (*TranscriptionResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	if vm.config.ApiKey != "" {
		return vm.transcriber.Transcribe(ctx, data)
	}

	return vm.transcriber.TranscribeLocal(ctx, data)
}

func (vm *VoiceManager) CheckKeyterms(text string) []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	matched := make([]string, 0)
	lowerText := strings.ToLower(text)

	for _, keyterm := range vm.keyterms {
		if strings.Contains(lowerText, strings.ToLower(keyterm)) {
			matched = append(matched, keyterm)
		}
	}

	return matched
}

func detectPlatform() string {
	goos := os.Getenv("GOOS")
	if goos != "" {
		return goos
	}

	if _, err := exec.LookPath("sw_vers"); err == nil {
		return "darwin"
	}
	if _, err := exec.LookPath("lsb_release"); err == nil {
		return "linux"
	}
	if _, err := exec.LookPath("ver"); err == nil {
		return "windows"
	}

	return "unknown"
}

func findWhisperBinary() string {
	candidates := []string{"whisper", "whisper.cpp", "whisper-cli"}

	for _, cmd := range candidates {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}

	home, _ := os.UserHomeDir()
	localPaths := []string{
		filepath.Join(home, ".local", "bin", "whisper"),
		"/usr/local/bin/whisper",
		"/usr/bin/whisper",
	}

	for _, path := range localPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func createMultipartWriter(body *bytes.Buffer, file *os.File, config VoiceConfig) *multipart.Writer {
	writer := multipart.NewWriter(body)

	writer.WriteField("model", config.Model)
	writer.WriteField("language", config.Language)

	fileWriter, _ := writer.CreateFormFile("file", "audio.wav")
	io.Copy(fileWriter, file)

	writer.Close()
	return writer
}

// VoiceDialogueSession manages a continuous voice dialogue with VAD-based
// speech boundary detection. It records audio in short chunks, runs VAD on
// each chunk, and when an utterance boundary is detected (silence after
// speech), it transcribes the accumulated speech and invokes the callback.
type VoiceDialogueSession struct {
	vm             *VoiceManager
	vad            *VoiceActivityDetector
	onTranscription func(text string)
	cancel         context.CancelFunc
	done           chan struct{}
	mu             sync.Mutex
	active         bool
}

// chunkDurationSec is the duration of each recorded audio chunk for VAD analysis.
const chunkDurationSec = 0.5

// StartDialogue begins the continuous VAD-based dialogue loop.
// It records audio in short chunks, detects speech boundaries via VAD,
// and calls onTranscription whenever a complete utterance is detected.
func (s *VoiceDialogueSession) StartDialogue(ctx context.Context, onTranscription func(text string)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return fmt.Errorf("dialogue session already active")
	}

	if onTranscription == nil {
		return fmt.Errorf("onTranscription callback is required")
	}

	s.onTranscription = onTranscription
	s.active = true

	dialogueCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})

	go s.dialogueLoop(dialogueCtx)

	return nil
}

// StopDialogue stops the dialogue session and waits for the loop goroutine to finish.
func (s *VoiceDialogueSession) StopDialogue() error {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return fmt.Errorf("dialogue session not active")
	}
	s.active = false
	s.cancel()
	s.mu.Unlock()

	<-s.done
	return nil
}

// IsActive returns whether the dialogue session is currently running.
func (s *VoiceDialogueSession) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// dialogueLoop is the main loop for the always-on dialogue mode.
// It continuously records short audio chunks, runs VAD on each chunk,
// and manages speech buffering and utterance boundary detection.
func (s *VoiceDialogueSession) dialogueLoop(ctx context.Context) {
	defer close(s.done)

	silenceThreshold := s.vm.config.SilenceThreshold
	if silenceThreshold <= 0 {
		silenceThreshold = 3
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		chunkData, err := s.recordChunk(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		if len(chunkData) == 0 {
			continue
		}

		hasVoice := s.vad.Detect(chunkData)

		s.mu.Lock()
		isActive := s.active
		s.mu.Unlock()
		if !isActive {
			return
		}

		if hasVoice {
			s.processVoiceChunk(ctx, chunkData, silenceThreshold)
		}
	}
}

// processVoiceChunk accumulates voice chunks and tracks silence to detect
// utterance boundaries. When silenceThreshold consecutive non-voice chunks
// pass after voice was detected, the utterance is transcribed.
func (s *VoiceDialogueSession) processVoiceChunk(ctx context.Context, chunkData []byte, silenceThreshold int) {
	speechBuffer := new(bytes.Buffer)
	speechBuffer.Write(chunkData)

	consecutiveSilence := 0

	for consecutiveSilence < silenceThreshold {
		select {
		case <-ctx.Done():
			return
		default:
		}

		nextChunk, err := s.recordChunk(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		if s.vad.Detect(nextChunk) {
			speechBuffer.Write(nextChunk)
			consecutiveSilence = 0
		} else {
			consecutiveSilence++
		}

		s.mu.Lock()
		stillActive := s.active
		s.mu.Unlock()
		if !stillActive {
			return
		}
	}

	audioData := speechBuffer.Bytes()
	if len(audioData) == 0 {
		return
	}

	result, err := s.vm.transcribe(ctx, audioData)
	if err != nil {
		return
	}

	text := strings.TrimSpace(result.Text)
	if text != "" && s.onTranscription != nil {
		s.onTranscription(text)
	}
}

// recordChunk records a short audio chunk of chunkDurationSec duration.
// It spawns a platform-appropriate recording command, captures its output,
// and returns the raw audio bytes.
func (s *VoiceDialogueSession) recordChunk(ctx context.Context) ([]byte, error) {
	platform := detectPlatform()
	var cmd *exec.Cmd

	sampleRate := s.vm.config.SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}

	switch platform {
	case "darwin":
		cmd = exec.CommandContext(ctx, "rec", "-q", "-r",
			fmt.Sprintf("%d", sampleRate),
			"-c", "1", "-", "trim", "0",
			fmt.Sprintf("%.1f", chunkDurationSec))
	case "linux":
		cmd = exec.CommandContext(ctx, "arecord", "-q", "-r",
			fmt.Sprintf("%d", sampleRate),
			"-c", "1", "-f", "S16_LE", "-t", "wav", "-d",
			fmt.Sprintf("%.1f", chunkDurationSec))
	default:
		return nil, fmt.Errorf("unsupported platform for voice recording: %s", platform)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chunk recording failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// transcribe is a helper that uses the VoiceManager's transcriber,
// selecting API or local whisper based on configuration.
func (vm *VoiceManager) transcribe(ctx context.Context, audioData []byte) (*TranscriptionResult, error) {
	if vm.config.ApiKey != "" {
		return vm.transcriber.Transcribe(ctx, audioData)
	}
	return vm.transcriber.TranscribeLocal(ctx, audioData)
}

// StartAlwaysOn starts the continuous dialogue mode on the VoiceManager.
// It creates a VoiceDialogueSession and begins the VAD-based recording loop.
// Returns an error if already in always-on mode.
func (vm *VoiceManager) StartAlwaysOn(ctx context.Context, onTranscription func(text string)) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.dialogueSession != nil && vm.dialogueSession.IsActive() {
		return fmt.Errorf("always-on dialogue already active")
	}

	vad := NewVAD(vm.config.VadThreshold, vm.config.SampleRate, vm.config.SilenceThreshold*500)

	session := &VoiceDialogueSession{
		vm:  vm,
		vad: vad,
	}

	if err := session.StartDialogue(ctx, onTranscription); err != nil {
		return err
	}

	vm.dialogueSession = session
	return nil
}

// StopAlwaysOn stops the always-on dialogue session.
func (vm *VoiceManager) StopAlwaysOn() error {
	vm.mu.Lock()
	session := vm.dialogueSession
	vm.mu.Unlock()

	if session == nil {
		return fmt.Errorf("no active always-on dialogue session")
	}

	return session.StopDialogue()
}

// IsDialogueActive returns whether the always-on dialogue mode is currently active.
func (vm *VoiceManager) IsDialogueActive() bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	if vm.dialogueSession == nil {
		return false
	}
	return vm.dialogueSession.IsActive()
}

func (vm *VoiceManager) GetConfig() VoiceConfig {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.config
}

func (vm *VoiceManager) UpdateConfig(config VoiceConfig) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.config = config
	vm.recorder.config = config
	vm.transcriber.config = config
}
