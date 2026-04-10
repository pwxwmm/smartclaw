package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type BridgeConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type BridgeConnection struct {
	Config    *BridgeConfig
	Connected bool
	LastSeen  time.Time
	mu        sync.RWMutex
}

func NewBridgeConnection(config *BridgeConfig) *BridgeConnection {
	return &BridgeConnection{
		Config:    config,
		Connected: false,
		LastSeen:  time.Now(),
	}
}

func (bc *BridgeConnection) Connect() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	addr := net.JoinHostPort(bc.Config.Host, fmt.Sprintf("%d", bc.Config.Port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to bridge: %w", err)
	}
	defer conn.Close()

	bc.Connected = true
	bc.LastSeen = time.Now()
	return nil
}

func (bc *BridgeConnection) Disconnect() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.Connected = false
	return nil
}

func (bc *BridgeConnection) IsConnected() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.Connected
}

func (bc *BridgeConnection) Send(data []byte) error {
	if !bc.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return nil
}

func (bc *BridgeConnection) Receive() ([]byte, error) {
	if !bc.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return nil, nil
}

type BridgeServer struct {
	Config *BridgeConfig
	Server *http.Server
	mu     sync.RWMutex
}

func NewBridgeServer(config *BridgeConfig) *BridgeServer {
	return &BridgeServer{
		Config: config,
	}
}

func (bs *BridgeServer) Start() error {
	addr := fmt.Sprintf("%s:%d", bs.Config.Host, bs.Config.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/connect", bs.handleConnect)
	mux.HandleFunc("/disconnect", bs.handleDisconnect)
	mux.HandleFunc("/send", bs.handleSend)
	mux.HandleFunc("/receive", bs.handleReceive)

	bs.Server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := bs.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Bridge server error: %v\n", err)
		}
	}()

	return nil
}

func (bs *BridgeServer) Stop() error {
	if bs.Server != nil {
		return bs.Server.Shutdown(context.Background())
	}
	return nil
}

func (bs *BridgeServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}

func (bs *BridgeServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
}

func (bs *BridgeServer) handleSend(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (bs *BridgeServer) handleReceive(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
}

type BridgeClient struct {
	URL  string
	mu   sync.RWMutex
	conn *BridgeConnection
}

func NewBridgeClient(url string) *BridgeClient {
	return &BridgeClient{
		URL: url,
	}
}

func (bc *BridgeClient) Connect() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	config := &BridgeConfig{Host: "localhost", Port: 8080}
	bc.conn = NewBridgeConnection(config)
	return bc.conn.Connect()
}

func (bc *BridgeClient) Disconnect() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.conn != nil {
		return bc.conn.Disconnect()
	}
	return nil
}

func (bc *BridgeClient) SendMessage(msg string) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.conn == nil || !bc.conn.IsConnected() {
		return fmt.Errorf("not connected")
	}

	data := []byte(msg)
	return bc.conn.Send(data)
}

func (bc *BridgeClient) ReceiveMessage() (string, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.conn == nil || !bc.conn.IsConnected() {
		return "", fmt.Errorf("not connected")
	}

	data, err := bc.conn.Receive()
	return string(data), err
}
