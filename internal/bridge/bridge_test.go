package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewBridgeConnection(t *testing.T) {
	t.Parallel()

	config := &BridgeConfig{Host: "localhost", Port: 8080}
	bc := NewBridgeConnection(config)

	if bc.Config != config {
		t.Error("Config should be set")
	}
	if bc.Connected {
		t.Error("New connection should not be connected")
	}
}

func TestBridgeConnection_Disconnect(t *testing.T) {
	t.Parallel()

	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})
	err := bc.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() returned error: %v", err)
	}
	if bc.IsConnected() {
		t.Error("After Disconnect(), IsConnected() should be false")
	}
}

func TestBridgeConnection_IsConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})
	if bc.IsConnected() {
		t.Error("New connection should not report connected")
	}
}

func TestBridgeConnection_Send_NotConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})
	err := bc.Send([]byte("data"))
	if err == nil {
		t.Error("Send() when not connected should return error")
	}
}

func TestBridgeConnection_Receive_NotConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})
	_, err := bc.Receive()
	if err == nil {
		t.Error("Receive() when not connected should return error")
	}
}

func TestBridgeConnection_Connect_Fails(t *testing.T) {
	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 59999})
	err := bc.Connect()
	if err == nil {
		t.Error("Connect() to unavailable port should return error")
	}
}

func TestBridgeConnection_Connect_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("Cannot create test listener")
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	bc := NewBridgeConnection(&BridgeConfig{Host: "127.0.0.1", Port: addr.Port})
	if err := bc.Connect(); err != nil {
		t.Errorf("Connect() returned error: %v", err)
	}
	if !bc.IsConnected() {
		t.Error("IsConnected() should be true after successful Connect()")
	}
}

func TestBridgeConfig_JSON(t *testing.T) {
	t.Parallel()

	config := &BridgeConfig{Host: "example.com", Port: 9090}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}

	var decoded BridgeConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() returned error: %v", err)
	}
	if decoded.Host != "example.com" {
		t.Errorf("Host = %q, want %q", decoded.Host, "example.com")
	}
	if decoded.Port != 9090 {
		t.Errorf("Port = %d, want %d", decoded.Port, 9090)
	}
}

func TestNewBridgeServer(t *testing.T) {
	t.Parallel()

	config := &BridgeConfig{Host: "localhost", Port: 0}
	bs := NewBridgeServer(config)
	if bs.Config != config {
		t.Error("Config should be set")
	}
}

func TestNewBridgeClient(t *testing.T) {
	t.Parallel()

	bc := NewBridgeClient("http://localhost:8080")
	if bc.URL != "http://localhost:8080" {
		t.Errorf("URL = %q, want %q", bc.URL, "http://localhost:8080")
	}
}

func TestBridgeClient_SendMessage_NotConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeClient("http://localhost:8080")
	err := bc.SendMessage("hello")
	if err == nil {
		t.Error("SendMessage() when not connected should return error")
	}
}

func TestBridgeClient_ReceiveMessage_NotConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeClient("http://localhost:8080")
	_, err := bc.ReceiveMessage()
	if err == nil {
		t.Error("ReceiveMessage() when not connected should return error")
	}
}

func TestBridgeClient_Disconnect_NotConnected(t *testing.T) {
	t.Parallel()

	bc := NewBridgeClient("http://localhost:8080")
	err := bc.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() when not connected returned error: %v", err)
	}
}

func TestBridgeServer_HandleConnect(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 8080})
	req := httptest.NewRequest(http.MethodGet, "/connect", nil)
	w := httptest.NewRecorder()

	bs.handleConnect(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "connected" {
		t.Errorf("status = %q, want %q", result["status"], "connected")
	}
}

func TestBridgeServer_HandleDisconnect(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 8080})
	req := httptest.NewRequest(http.MethodGet, "/disconnect", nil)
	w := httptest.NewRecorder()

	bs.handleDisconnect(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "disconnected" {
		t.Errorf("status = %q, want %q", result["status"], "disconnected")
	}
}

func TestBridgeServer_HandleSend(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 8080})
	body := strings.NewReader(`{"message": "hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/send", body)
	w := httptest.NewRecorder()

	bs.handleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "sent" {
		t.Errorf("status = %q, want %q", result["status"], "sent")
	}
}

func TestBridgeServer_HandleSend_InvalidJSON(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 8080})
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/send", body)
	w := httptest.NewRecorder()

	bs.handleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestBridgeServer_HandleReceive(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 8080})
	req := httptest.NewRequest(http.MethodGet, "/receive", nil)
	w := httptest.NewRecorder()

	bs.handleReceive(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestBridgeServer_Stop_WithoutStart(t *testing.T) {
	t.Parallel()

	bs := NewBridgeServer(&BridgeConfig{Host: "localhost", Port: 0})
	err := bs.Stop()
	if err != nil {
		t.Errorf("Stop() without Start() returned error: %v", err)
	}
}

func TestBridgeServer_StartAndStop(t *testing.T) {
	bs := NewBridgeServer(&BridgeConfig{Host: "127.0.0.1", Port: 0})

	err := bs.Start()
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := bs.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestBridgeServer_FullHTTPFlow(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() returned error: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close()

	bs := NewBridgeServer(&BridgeConfig{Host: "127.0.0.1", Port: addr.Port})

	err = bs.Start()
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	defer bs.Stop()

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/connect", addr.Port)

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET /connect returned error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]string
	json.Unmarshal(body, &result)
	if result["status"] != "connected" {
		t.Errorf("status = %q, want %q", result["status"], "connected")
	}
}

func TestBridgeConnection_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bc.IsConnected()
		}()
	}
	wg.Wait()
}

func TestBridgeConnection_LastSeen(t *testing.T) {
	t.Parallel()

	before := time.Now()
	bc := NewBridgeConnection(&BridgeConfig{Host: "localhost", Port: 8080})
	after := time.Now()

	if bc.LastSeen.Before(before) || bc.LastSeen.After(after) {
		t.Errorf("LastSeen = %v, should be between %v and %v", bc.LastSeen, before, after)
	}
}
