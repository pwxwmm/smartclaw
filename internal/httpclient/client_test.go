package httpclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_DefaultTimeout(t *testing.T) {
	t.Parallel()

	client := NewClient(0)
	if client == nil {
		t.Fatal("NewClient(0) returned nil")
	}
	if client.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, DefaultTimeout)
	}
}

func TestNewClient_CustomTimeout(t *testing.T) {
	t.Parallel()

	customTimeout := 10 * time.Second
	client := NewClient(customTimeout)
	if client.Timeout != customTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, customTimeout)
	}
}

func TestNewClient_HasTransport(t *testing.T) {
	t.Parallel()

	client := NewClient(30 * time.Second)
	if client.Transport == nil {
		t.Error("NewClient() Transport is nil")
	}
}

func TestNewClient_TransportSettings(t *testing.T) {
	t.Parallel()

	client := NewClient(30 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Transport is not *http.Transport")
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 10", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != 10*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 10s", transport.ResponseHeaderTimeout)
	}
	if transport.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want 1s", transport.ExpectContinueTimeout)
	}
}

func TestDefaultClient(t *testing.T) {
	t.Parallel()

	client := DefaultClient()
	if client == nil {
		t.Fatal("DefaultClient() returned nil")
	}
	if client.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, DefaultTimeout)
	}
}

func TestSharedTransport(t *testing.T) {
	t.Parallel()

	transport := SharedTransport()
	if transport == nil {
		t.Fatal("SharedTransport() returned nil")
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 10", transport.MaxIdleConnsPerHost)
	}
}

func TestNewClient_MakesSuccessfulRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient(5 * time.Second)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestNewClient_TimeoutExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(50 * time.Millisecond)
	_, err := client.Get(server.URL)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestSharedTransport_Settings(t *testing.T) {
	t.Parallel()

	transport := SharedTransport()
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != 10*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 10s", transport.ResponseHeaderTimeout)
	}
	if transport.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want 1s", transport.ExpectContinueTimeout)
	}
}

func TestDefaultTimeout_Value(t *testing.T) {
	t.Parallel()

	if DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want 30s", DefaultTimeout)
	}
}
