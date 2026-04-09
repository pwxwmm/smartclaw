package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	config      VoiceConfig
	recorder    *VoiceRecorder
	transcriber *VoiceTranscriber
	keyterms    []string
	mu          sync.RWMutex
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

	tmpFile, err := ioutil.TempFile("", "voice-*.wav")
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
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

	tmpFile, err := ioutil.TempFile("", "voice-*.wav")
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

	data, err := ioutil.ReadFile(jsonFile)
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
	data, err := ioutil.ReadFile(filePath)
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
