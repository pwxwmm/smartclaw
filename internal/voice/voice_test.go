package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestVoiceMode_Constants(t *testing.T) {
	t.Parallel()

	if VoiceModeDisabled != 0 {
		t.Errorf("VoiceModeDisabled = %d, want 0", VoiceModeDisabled)
	}
	if VoiceModePushToTalk != 1 {
		t.Errorf("VoiceModePushToTalk = %d, want 1", VoiceModePushToTalk)
	}
	if VoiceModeAlwaysOn != 2 {
		t.Errorf("VoiceModeAlwaysOn = %d, want 2", VoiceModeAlwaysOn)
	}
}

func TestVoiceConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := VoiceConfig{}
	if config.Mode != VoiceModeDisabled {
		t.Errorf("Default Mode = %d, want %d", config.Mode, VoiceModeDisabled)
	}
	if config.Language != "" {
		t.Errorf("Default Language should be empty, got %q", config.Language)
	}
	if config.SampleRate != 0 {
		t.Errorf("Default SampleRate should be 0, got %d", config.SampleRate)
	}
}

func TestVoiceConfig_JSON(t *testing.T) {
	t.Parallel()

	config := VoiceConfig{
		Mode:         VoiceModePushToTalk,
		Language:     "en",
		Model:        "whisper-1",
		SampleRate:   16000,
		VadEnabled:   true,
		VadThreshold: 0.5,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}

	var decoded VoiceConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if decoded.Mode != VoiceModePushToTalk {
		t.Errorf("Mode = %d, want %d", decoded.Mode, VoiceModePushToTalk)
	}
	if decoded.Language != "en" {
		t.Errorf("Language = %q, want %q", decoded.Language, "en")
	}
	if decoded.SampleRate != 16000 {
		t.Errorf("SampleRate = %d, want 16000", decoded.SampleRate)
	}
}

func TestNewVAD_Defaults(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0, 0, 0)
	if vad.threshold != 0.5 {
		t.Errorf("default threshold = %f, want 0.5", vad.threshold)
	}
	if vad.sampleRate != 16000 {
		t.Errorf("default sampleRate = %d, want 16000", vad.sampleRate)
	}
	if vad.silenceMs != 2000 {
		t.Errorf("default silenceMs = %d, want 2000", vad.silenceMs)
	}
}

func TestNewVAD_CustomValues(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.8, 44100, 3000)
	if vad.threshold != 0.8 {
		t.Errorf("threshold = %f, want 0.8", vad.threshold)
	}
	if vad.sampleRate != 44100 {
		t.Errorf("sampleRate = %d, want 44100", vad.sampleRate)
	}
	if vad.silenceMs != 3000 {
		t.Errorf("silenceMs = %d, want 3000", vad.silenceMs)
	}
}

func TestVAD_Detect_ShortAudio(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)

	shortAudio := make([]byte, 512)
	result := vad.Detect(shortAudio)
	if !result {
		t.Error("Detect() with short audio (< 1024 bytes) should return true")
	}
}

func TestVAD_Detect_EmptyAudio(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)
	result := vad.Detect([]byte{})
	if !result {
		t.Error("Detect() with empty audio should return true")
	}
}

func makeLoudAudio(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size-1; i += 2 {
		data[i] = 0xFF
		data[i+1] = 0x7F
	}
	return data
}

func makeSilentAudio(size int) []byte {
	return make([]byte, size)
}

func TestVAD_Detect_SilentAudio(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)
	silentAudio := makeSilentAudio(2048)
	result := vad.Detect(silentAudio)
	if result {
		t.Error("Detect() with silent audio should return false")
	}
}

func TestVAD_Detect_LoudAudio(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)
	loudAudio := makeLoudAudio(2048)
	result := vad.Detect(loudAudio)
	if !result {
		t.Error("Detect() with loud audio should return true")
	}
}

func TestVAD_DetectWithCallback_Voice(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)

	voiceCalled := false
	silenceCalled := false

	loudAudio := makeLoudAudio(2048)
	vad.DetectWithCallback(loudAudio, func() { voiceCalled = true }, func() { silenceCalled = true })

	if !voiceCalled {
		t.Error("onVoiceStart should be called for loud audio")
	}
	if silenceCalled {
		t.Error("onSilence should not be called for loud audio")
	}
}

func TestVAD_DetectWithCallback_Silence(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)

	voiceCalled := false
	silenceCalled := false

	silentAudio := makeSilentAudio(2048)
	vad.DetectWithCallback(silentAudio, func() { voiceCalled = true }, func() { silenceCalled = true })

	if voiceCalled {
		t.Error("onVoiceStart should not be called for silent audio")
	}
	if !silenceCalled {
		t.Error("onSilence should be called for silent audio")
	}
}

func TestVAD_DetectWithCallback_NilCallbacks(t *testing.T) {
	t.Parallel()

	vad := NewVAD(0.5, 16000, 2000)

	loudAudio := makeLoudAudio(2048)
	result := vad.DetectWithCallback(loudAudio, nil, nil)
	if !result {
		t.Error("DetectWithCallback should still return detection result with nil callbacks")
	}
}

func TestTranscriptionResult_JSON(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := TranscriptionResult{
		Text:       "hello world",
		Language:   "en",
		Duration:   5.2,
		Confidence: 0.95,
		Timestamp:  now,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}

	var decoded TranscriptionResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if decoded.Text != "hello world" {
		t.Errorf("Text = %q, want %q", decoded.Text, "hello world")
	}
	if decoded.Language != "en" {
		t.Errorf("Language = %q, want %q", decoded.Language, "en")
	}
}

func TestNewVoiceManager(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	if vm == nil {
		t.Fatal("NewVoiceManager() returned nil")
	}
}

func TestNewVoiceManager_Defaults(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	config := vm.GetConfig()

	if config.SampleRate != 16000 {
		t.Errorf("SampleRate = %d, want 16000", config.SampleRate)
	}
	if config.Language != "en" {
		t.Errorf("Language = %q, want %q", config.Language, "en")
	}
	if config.RecordingTimeout != 30 {
		t.Errorf("RecordingTimeout = %d, want 30", config.RecordingTimeout)
	}
	if config.SilenceThreshold != 3 {
		t.Errorf("SilenceThreshold = %d, want 3", config.SilenceThreshold)
	}
}

func TestNewVoiceManager_CustomConfig(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{
		SampleRate:       44100,
		Language:         "ja",
		RecordingTimeout: 60,
	})
	config := vm.GetConfig()

	if config.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want 44100", config.SampleRate)
	}
	if config.Language != "ja" {
		t.Errorf("Language = %q, want %q", config.Language, "ja")
	}
	if config.RecordingTimeout != 60 {
		t.Errorf("RecordingTimeout = %d, want 60", config.RecordingTimeout)
	}
}

func TestNewVoiceManager_DefaultEndpoint(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	if vm.transcriber.endpoint != "https://api.openai.com/v1/audio/transcriptions" {
		t.Errorf("default endpoint = %q, want OpenAI transcription endpoint", vm.transcriber.endpoint)
	}
}

func TestNewVoiceManager_CustomEndpoint(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{ApiEndpoint: "https://custom.api.com/v1/audio"})
	if vm.transcriber.endpoint != "https://custom.api.com/v1/audio" {
		t.Errorf("endpoint = %q, want custom endpoint", vm.transcriber.endpoint)
	}
}

func TestVoiceManager_SetMode(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetMode(VoiceModePushToTalk)

	if vm.GetMode() != VoiceModePushToTalk {
		t.Errorf("GetMode() = %d, want %d", vm.GetMode(), VoiceModePushToTalk)
	}
}

func TestVoiceManager_AddKeyterm(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.AddKeyterm("smartclaw")
	vm.AddKeyterm("assistant")

	terms := vm.GetKeyterms()
	if len(terms) != 2 {
		t.Fatalf("GetKeyterms() returned %d terms, want 2", len(terms))
	}
}

func TestVoiceManager_SetKeyterms(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.AddKeyterm("old")
	vm.SetKeyterms([]string{"new1", "new2", "new3"})

	terms := vm.GetKeyterms()
	if len(terms) != 3 {
		t.Fatalf("GetKeyterms() returned %d terms, want 3", len(terms))
	}
}

func TestVoiceManager_GetKeyterms_ReturnsCopy(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetKeyterms([]string{"term1"})

	terms := vm.GetKeyterms()
	terms[0] = "modified"

	original := vm.GetKeyterms()
	if original[0] == "modified" {
		t.Error("GetKeyterms() should return a copy, not reference to internal slice")
	}
}

func TestVoiceManager_CheckKeyterms(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetKeyterms([]string{"smartclaw", "assistant", "hello"})

	matched := vm.CheckKeyterms("Please activate smartclaw now")
	if len(matched) != 1 || matched[0] != "smartclaw" {
		t.Errorf("CheckKeyterms() = %v, want [smartclaw]", matched)
	}
}

func TestVoiceManager_CheckKeyterms_CaseInsensitive(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetKeyterms([]string{"SmartClaw"})

	matched := vm.CheckKeyterms("please activate smartclaw now")
	if len(matched) != 1 {
		t.Errorf("CheckKeyterms() should match case-insensitively, got %v", matched)
	}
}

func TestVoiceManager_CheckKeyterms_NoMatch(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetKeyterms([]string{"smartclaw"})

	matched := vm.CheckKeyterms("hello world")
	if len(matched) != 0 {
		t.Errorf("CheckKeyterms() with no match = %v, want empty", matched)
	}
}

func TestVoiceManager_CheckKeyterms_MultipleMatches(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	vm.SetKeyterms([]string{"smartclaw", "assistant"})

	matched := vm.CheckKeyterms("smartclaw assistant mode")
	if len(matched) != 2 {
		t.Errorf("CheckKeyterms() with multiple matches = %v, want 2 items", matched)
	}
}

func TestVoiceManager_UpdateConfig(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{})
	newConfig := VoiceConfig{
		Language: "fr",
		Mode:     VoiceModeAlwaysOn,
	}
	vm.UpdateConfig(newConfig)

	config := vm.GetConfig()
	if config.Language != "fr" {
		t.Errorf("Language = %q, want %q", config.Language, "fr")
	}
	if config.Mode != VoiceModeAlwaysOn {
		t.Errorf("Mode = %d, want %d", config.Mode, VoiceModeAlwaysOn)
	}
}

func TestVoiceRecorder_IsRecording(t *testing.T) {
	t.Parallel()

	vr := &VoiceRecorder{
		config:      VoiceConfig{SampleRate: 16000},
		audioBuffer: bytes.NewBuffer(nil),
	}
	if vr.IsRecording() {
		t.Error("New recorder should not be recording")
	}
}

func TestVoiceRecorder_StopWithoutStart(t *testing.T) {
	t.Parallel()

	vr := &VoiceRecorder{
		config:      VoiceConfig{SampleRate: 16000},
		audioBuffer: bytes.NewBuffer(nil),
	}
	_, err := vr.StopRecording()
	if err == nil {
		t.Error("StopRecording() without StartRecording should return error")
	}
}

func TestVoiceTranscriber_NoAPIKey(t *testing.T) {
	t.Parallel()

	vt := &VoiceTranscriber{
		config:   VoiceConfig{ApiKey: ""},
		client:   &http.Client{Timeout: 5 * time.Second},
		endpoint: "https://api.example.com",
	}

	_, err := vt.Transcribe(context.Background(), []byte("fake audio"))
	if err == nil {
		t.Error("Transcribe() without API key should return error")
	}
}

func TestVoiceTranscriber_TranscribeLocal_NoWhisper(t *testing.T) {
	t.Parallel()

	vt := &VoiceTranscriber{
		config: VoiceConfig{
			Language: "en",
			Model:    "base",
		},
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	_, err := vt.TranscribeLocal(context.Background(), []byte("fake audio"))
	if err == nil {
		t.Error("TranscribeLocal() without whisper binary should return error")
	}
}

func TestVoiceManager_StartPushToTalk_WrongMode(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{Mode: VoiceModeDisabled})
	err := vm.StartPushToTalk(context.Background())
	if err == nil {
		t.Error("StartPushToTalk() with wrong mode should return error")
	}
}

func TestVoiceManager_StopPushToTalk_NotRecording(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{Mode: VoiceModePushToTalk})
	_, err := vm.StopPushToTalk(context.Background())
	if err == nil {
		t.Error("StopPushToTalk() without recording should return error")
	}
}

func TestVoiceManager_TranscribeFile_Nonexistent(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{ApiKey: "test-key"})
	_, err := vm.TranscribeFile(context.Background(), "/nonexistent/audio.wav")
	if err == nil {
		t.Error("TranscribeFile() with nonexistent file should return error")
	}
}

func TestCreateMultipartWriter(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "voice-test-*.wav")
	if err != nil {
		t.Fatalf("CreateTemp() returned error: %v", err)
	}
	tmpFile.WriteString("fake audio data")
	tmpFile.Seek(0, 0)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	var body bytes.Buffer
	writer := createMultipartWriter(&body, tmpFile, VoiceConfig{
		Model:    "whisper-1",
		Language: "en",
	})

	contentType := writer.FormDataContentType()
	if contentType == "" {
		t.Error("FormDataContentType() should not be empty")
	}
	if body.Len() == 0 {
		t.Error("Body should contain multipart data")
	}
}

func TestDetectPlatform(t *testing.T) {
	platform := detectPlatform()
	if platform == "" {
		t.Error("detectPlatform() should not return empty string")
	}
}

func TestFindWhisperBinary(t *testing.T) {
	t.Parallel()

	path := findWhisperBinary()
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("findWhisperBinary() returned %q but file does not exist", path)
		}
	}
}

func TestVoiceManager_Transcribe_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		result := map[string]any{
			"text":     "hello world",
			"language": "en",
			"duration": 3.5,
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	vm := NewVoiceManager(VoiceConfig{
		ApiKey:      "test-key",
		ApiEndpoint: server.URL,
	})

	audioData := makeLoudAudio(2048)

	result, err := vm.transcriber.Transcribe(context.Background(), audioData)
	if err != nil {
		t.Fatalf("Transcribe() returned error: %v", err)
	}
	if result.Text != "hello world" {
		t.Errorf("Text = %q, want %q", result.Text, "hello world")
	}
	if result.Language != "en" {
		t.Errorf("Language = %q, want %q", result.Language, "en")
	}
}

func TestVoiceManager_Transcribe_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "internal server error")
	}))
	defer server.Close()

	vm := NewVoiceManager(VoiceConfig{
		ApiKey:      "test-key",
		ApiEndpoint: server.URL,
	})

	audioData := make([]byte, 2048)
	_, err := vm.transcriber.Transcribe(context.Background(), audioData)
	if err == nil {
		t.Error("Transcribe() with 500 response should return error")
	}
}

func TestVoiceManager_Transcribe_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "invalid api key")
	}))
	defer server.Close()

	vm := NewVoiceManager(VoiceConfig{
		ApiKey:      "bad-key",
		ApiEndpoint: server.URL,
	})

	audioData := make([]byte, 2048)
	_, err := vm.transcriber.Transcribe(context.Background(), audioData)
	if err == nil {
		t.Error("Transcribe() with 401 response should return error")
	}
}

func TestVoiceManager_GetConfig(t *testing.T) {
	t.Parallel()

	vm := NewVoiceManager(VoiceConfig{Mode: VoiceModeAlwaysOn})
	config := vm.GetConfig()
	if config.Mode != VoiceModeAlwaysOn {
		t.Errorf("Mode = %d, want %d", config.Mode, VoiceModeAlwaysOn)
	}
}
