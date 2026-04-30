package server

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Hub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

type WSClient struct {
	ID        string
	UserID    string
	SessionID string
	RoomID    string
	Role      string
	hub       *Hub
	send      chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register(client *WSClient) {
	h.register <- client
}

func (h *Hub) Unregister(client *WSClient) {
	h.unregister <- client
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func newWSClient(hub *Hub, userID string) *WSClient {
	if userID == "" {
		userID = "default"
	}
	return &WSClient{
		ID:     uuid.New().String()[:8],
		UserID: userID,
		hub:    hub,
		send:   make(chan []byte, 256),
	}
}

type wsMessage struct {
	Type    string          `json:"type"`
	Content string          `json:"content,omitempty"`
	Name    string          `json:"name,omitempty"`
	Path    string          `json:"path,omitempty"`
	ID      string          `json:"id,omitempty"`
	Model   string          `json:"model,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type wsResponse struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
	ID      string `json:"id,omitempty"`
	Status  string `json:"status,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func (s *APIServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.RawQuery) > 2048 {
		http.Error(w, "query string too long", http.StatusBadRequest)
		return
	}

	if !s.noAuth && s.auth.IsAuthRequired() {
		token := r.URL.Query().Get("token")
		if token == "" {
			token = extractToken(r)
		}
		if !isValidToken(token) {
			http.Error(w, "invalid token format", http.StatusBadRequest)
			return
		}
		if !validateAccessToken(token, s.auth) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	userID := r.URL.Query().Get("user")
	if !isValidUserID(userID) {
		conn.Close(websocket.StatusPolicyViolation, "invalid user ID format")
		return
	}
	client := newWSClient(s.hub, userID)
	s.hub.Register(client)

	go s.wsWritePump(client, conn)
	go s.wsReadPump(client, conn)
}

func (s *APIServer) wsReadPump(client *WSClient, conn *websocket.Conn) {
	defer func() {
		if client.RoomID != "" && s.roomMgr != nil {
			s.roomMgr.LeaveRoom(client.RoomID, client.UserID)
			room, ok := s.roomMgr.GetRoom(client.RoomID)
			if ok {
				room.BroadcastPresence()
			}
			client.RoomID = ""
			client.Role = ""
		}
		s.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	conn.SetReadLimit(65536)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, message, err := conn.Read(ctx)
		cancel()
		if err != nil {
			closeCode := websocket.CloseStatus(err)
			if closeCode != websocket.StatusGoingAway && closeCode != -1 {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		s.dispatchWSMessage(client, message)
	}
}

func (s *APIServer) wsWritePump(client *WSClient, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			buf := make([]byte, 0, len(message))
			buf = append(buf, message...)

			n := len(client.send)
			for i := 0; i < n; i++ {
				buf = append(buf, '\n')
				buf = append(buf, <-client.send...)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := conn.Write(ctx, websocket.MessageText, buf)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *APIServer) dispatchWSMessage(client *WSClient, raw []byte) {
	var msg wsMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		s.wsSendError(client, "Invalid message format")
		return
	}

	switch msg.Type {
	case "chat":
		s.wsHandleChat(client, msg)
	case "cmd":
		s.wsHandleCommand(client, msg)
	case "session_list":
		s.wsHandleSessionList(client)
	case "room_create":
		s.wsHandleRoomCreate(client, msg)
	case "room_join":
		s.wsHandleRoomJoin(client, msg)
	case "room_leave":
		s.wsHandleRoomLeave(client, msg)
	case "room_list":
		s.wsHandleRoomList(client)
	case "room_take_wheel":
		s.wsHandleTakeWheel(client, msg)
	case "room_release_wheel":
		s.wsHandleReleaseWheel(client)
	case "room_set_role":
		s.wsHandleRoomSetRole(client, msg)
	case "room_chat":
		s.wsHandleRoomChat(client, msg)
	case "presence_update":
		s.wsHandlePresence(client, msg)
	default:
		s.wsSendError(client, "Unknown message type: "+msg.Type)
	}
}

func (s *APIServer) wsHandleChat(client *WSClient, msg wsMessage) {
	if s.gw == nil {
		s.wsSendError(client, "Gateway not available")
		return
	}

	content := msg.Content
	if content == "" {
		var data struct {
			Content   string `json:"content"`
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil {
			content = data.Content
		}
	}

	if content == "" {
		s.wsSendError(client, "content is required")
		return
	}

	resp, err := s.gw.HandleMessage(context.Background(), client.UserID, "ws", content)
	if err != nil {
		s.wsSendError(client, "Chat error: "+err.Error())
		return
	}

	s.wsSend(client, wsResponse{
		Type:    "chat",
		Content: resp.Content,
		ID:      resp.SessionID,
	})
}

func (s *APIServer) wsHandleCommand(client *WSClient, msg wsMessage) {
	if s.gw == nil {
		s.wsSendError(client, "Gateway not available")
		return
	}

	content := msg.Content
	if content == "" && len(msg.Data) > 0 {
		var data struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil {
			content = data.Content
		}
	}

	if strings.HasPrefix(content, "/") {
		content = strings.TrimSpace(content[1:])
	}

	resp, err := s.gw.HandleMessage(context.Background(), client.UserID, "ws", content)
	if err != nil {
		s.wsSendError(client, "Command error: "+err.Error())
		return
	}

	s.wsSend(client, wsResponse{
		Type:    "cmd",
		Content: resp.Content,
		ID:      resp.SessionID,
	})
}

func (s *APIServer) wsHandleSessionList(client *WSClient) {
	if s.store != nil {
		sessions, err := s.store.ListAllSessions(50)
		if err != nil {
			s.wsSendError(client, "Failed to list sessions")
			return
		}
		s.wsSend(client, wsResponse{Type: "session_list", Data: sessions})
		return
	}

	if s.sessMgr != nil {
		sessions, err := s.sessMgr.List()
		if err != nil {
			s.wsSendError(client, "Failed to list sessions")
			return
		}
		s.wsSend(client, wsResponse{Type: "session_list", Data: sessions})
		return
	}

	s.wsSend(client, wsResponse{Type: "session_list", Data: []any{}})
}

func (s *APIServer) wsSend(client *WSClient, resp wsResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	case <-time.After(5 * time.Second):
		slog.Warn("ws send timeout", "client_id", client.ID)
	}
}

func (s *APIServer) wsSendError(client *WSClient, message string) {
	s.wsSend(client, wsResponse{Type: "error", Message: message})
}

func isValidToken(s string) bool {
	if s == "" {
		return false
	}
	return isValidAlphanumeric(s, 512, "._-")
}

func isValidUserID(s string) bool {
	if s == "" {
		return true
	}
	return isValidAlphanumeric(s, 128, "._-@")
}

func (s *APIServer) wsHandleRoomCreate(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	var req struct {
		Name      string `json:"name"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		req.Name = msg.Name
		req.SessionID = msg.ID
	}
	if req.Name == "" {
		req.Name = "Collaboration Session"
	}

	room := s.roomMgr.CreateRoom(req.Name, client.UserID, req.SessionID)

	s.wsSend(client, wsResponse{
		Type: "room_created",
		Data: map[string]any{
			"room_id": room.ID,
			"name":    room.Name,
			"ws_url":  "/ws?room=" + room.ID,
		},
	})
}

func (s *APIServer) wsHandleRoomJoin(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		req.RoomID = msg.ID
	}
	if req.RoomID == "" {
		s.wsSendError(client, "room_id is required")
		return
	}

	room, err := s.roomMgr.JoinRoom(req.RoomID, client.UserID, client)
	if err != nil {
		s.wsSend(client, wsResponse{Type: "room_error", Message: err.Error()})
		return
	}

	s.wsSend(client, wsResponse{
		Type: "room_joined",
		Data: map[string]any{
			"room_id":       room.ID,
			"name":          room.Name,
			"session_id":    room.SessionID,
			"owner_id":      room.OwnerID,
			"participants":  room.participantList(),
			"active_editor": room.ActiveEditor,
		},
	})

	room.BroadcastMessage(wsResponse{
		Type: "room_participant_joined",
		Data: map[string]any{
			"room_id": room.ID,
			"user_id": client.UserID,
			"role":     client.Role,
		},
	})
	room.BroadcastPresence()
}

func (s *APIServer) wsHandleRoomLeave(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	roomID := client.RoomID
	if roomID == "" {
		var req struct {
			RoomID string `json:"room_id"`
		}
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			req.RoomID = msg.ID
		}
		roomID = req.RoomID
	}

	if roomID == "" {
		s.wsSendError(client, "not in a room")
		return
	}

	room, ok := s.roomMgr.GetRoom(roomID)
	userID := client.UserID

	s.roomMgr.LeaveRoom(roomID, userID)
	client.RoomID = ""
	client.Role = ""

	s.wsSend(client, wsResponse{Type: "room_left", Data: map[string]any{"room_id": roomID}})

	if ok {
		room.BroadcastMessage(wsResponse{
			Type: "room_participant_left",
			Data: map[string]any{
				"room_id": roomID,
				"user_id": userID,
			},
		})
		room.BroadcastPresence()
	}
}

func (s *APIServer) wsHandleRoomList(client *WSClient) {
	if s.roomMgr == nil {
		s.wsSend(client, wsResponse{Type: "room_list", Data: []any{}})
		return
	}

	rooms := s.roomMgr.ListRooms()
	summaries := make([]map[string]any, 0, len(rooms))
	for _, room := range rooms {
		summaries = append(summaries, roomSummary(room))
	}

	s.wsSend(client, wsResponse{Type: "room_list", Data: summaries})
}

func (s *APIServer) wsHandleTakeWheel(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	if client.RoomID == "" {
		s.wsSendError(client, "not in a room")
		return
	}

	room, ok := s.roomMgr.GetRoom(client.RoomID)
	if !ok {
		s.wsSendError(client, "room not found")
		return
	}

	if room.TakeWheel(client.UserID) {
		room.BroadcastMessage(wsResponse{
			Type: "room_wheel_taken",
			Data: map[string]any{
				"room_id": room.ID,
				"user_id": client.UserID,
			},
		})
		room.BroadcastPresence()
	} else {
		s.wsSend(client, wsResponse{
			Type:    "room_error",
			Message: ErrWheelHeld.Error(),
		})
	}
}

func (s *APIServer) wsHandleReleaseWheel(client *WSClient) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	if client.RoomID == "" {
		s.wsSendError(client, "not in a room")
		return
	}

	room, ok := s.roomMgr.GetRoom(client.RoomID)
	if !ok {
		s.wsSendError(client, "room not found")
		return
	}

	room.ReleaseWheel(client.UserID)
	room.BroadcastMessage(wsResponse{
		Type: "room_wheel_released",
		Data: map[string]any{
			"room_id": room.ID,
			"user_id": client.UserID,
		},
	})
	room.BroadcastPresence()
}

func (s *APIServer) wsHandleRoomSetRole(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	if client.RoomID == "" {
		s.wsSendError(client, "not in a room")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		s.wsSendError(client, "invalid request data")
		return
	}

	if req.UserID == "" || req.Role == "" {
		s.wsSendError(client, "user_id and role are required")
		return
	}

	err := s.roomMgr.SetRole(client.RoomID, client.UserID, req.UserID, req.Role)
	if err != nil {
		s.wsSend(client, wsResponse{Type: "room_error", Message: err.Error()})
		return
	}

	room, _ := s.roomMgr.GetRoom(client.RoomID)
	if room != nil {
		room.BroadcastPresence()
	}
}

func (s *APIServer) wsHandleRoomChat(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil {
		s.wsSendError(client, "room system not available")
		return
	}

	if client.RoomID == "" {
		s.wsSendError(client, "not in a room")
		return
	}

	room, ok := s.roomMgr.GetRoom(client.RoomID)
	if !ok {
		s.wsSendError(client, "room not found")
		return
	}

	room.mu.RLock()
	activeEditor := room.ActiveEditor
	room.mu.RUnlock()

	if activeEditor != "" && client.UserID != activeEditor {
		s.wsSend(client, wsResponse{Type: "room_error", Message: "only the active editor can send messages"})
		return
	}

	content := msg.Content
	if content == "" {
		var data struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil {
			content = data.Content
		}
	}
	if content == "" {
		s.wsSendError(client, "content is required")
		return
	}

	room.BroadcastMessage(wsResponse{
		Type:    "room_message",
		Content: content,
		Data: map[string]any{
			"room_id": room.ID,
			"user_id": client.UserID,
		},
	})

	if s.gw != nil {
		resp, err := s.gw.HandleMessage(context.Background(), client.UserID, "ws", content)
		if err != nil {
			room.BroadcastMessage(wsResponse{
				Type:    "room_error",
				Message: "agent error: " + err.Error(),
			})
			return
		}

		room.BroadcastMessage(wsResponse{
			Type:    "room_message",
			Content: resp.Content,
			Data: map[string]any{
				"room_id":    room.ID,
				"user_id":    "agent",
				"session_id": resp.SessionID,
			},
		})
	}
}

func (s *APIServer) wsHandlePresence(client *WSClient, msg wsMessage) {
	if s.roomMgr == nil || client.RoomID == "" {
		return
	}

	room, ok := s.roomMgr.GetRoom(client.RoomID)
	if !ok {
		return
	}

	var req struct {
		IsTyping bool `json:"is_typing"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return
	}

	room.mu.Lock()
	if p, exists := room.Participants[client.UserID]; exists {
		p.IsTyping = req.IsTyping
	}
	room.mu.Unlock()

	room.BroadcastPresence()
}
