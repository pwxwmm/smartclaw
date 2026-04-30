package server

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxParticipantsPerRoom = 10

const wheelInactiveTimeout = 30 * time.Second

var (
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomFull          = errors.New("room is full")
	ErrNotOwner          = errors.New("only the room owner can perform this action")
	ErrInvalidRole       = errors.New("invalid role; must be owner, editor, or viewer")
	ErrAlreadyInRoom     = errors.New("user is already in the room")
	ErrNotInRoom         = errors.New("user is not in the room")
	ErrWheelHeld         = errors.New("another user currently has the wheel")
	ErrKicked            = errors.New("user has been kicked from the room")
)

// Room represents a shared collaboration session where multiple users can
// watch and interact with the same AI agent session.
type Room struct {
	ID           string
	Name         string
	SessionID    string
	OwnerID      string
	CreatedAt    time.Time
	Participants map[string]*Participant
	ActiveEditor string // UserID of the participant who "has the wheel"
	WheelTakenAt time.Time

	mu sync.RWMutex
}

// Participant represents a user joined to a collaboration room.
type Participant struct {
	UserID   string
	Client   *WSClient
	Role     string // "owner", "editor", "viewer"
	JoinedAt time.Time
	IsTyping bool
}

// RoomManager manages all active collaboration rooms.
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewRoomManager creates a new RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

// CreateRoom creates a new collaboration room with a generated ID.
func (rm *RoomManager) CreateRoom(name, ownerID, sessionID string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room := &Room{
		ID:           uuid.New().String()[:8],
		Name:         name,
		SessionID:    sessionID,
		OwnerID:      ownerID,
		CreatedAt:    time.Now(),
		Participants: make(map[string]*Participant),
	}

	rm.rooms[room.ID] = room
	return room
}

// JoinRoom adds a participant to a room.
func (rm *RoomManager) JoinRoom(roomID, userID string, client *WSClient) (*Room, error) {
	rm.mu.RLock()
	room, ok := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !ok {
		return nil, ErrRoomNotFound
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if p, exists := room.Participants[userID]; exists {
		if p.Role == "kicked" {
			return nil, ErrKicked
		}
		return nil, ErrAlreadyInRoom
	}

	if len(room.Participants) >= maxParticipantsPerRoom {
		return nil, ErrRoomFull
	}

	// Check if user was previously kicked
	// First join gets "editor" if owner, "viewer" otherwise
	role := "viewer"
	if userID == room.OwnerID {
		role = "owner"
	}

	participant := &Participant{
		UserID:   userID,
		Client:   client,
		Role:     role,
		JoinedAt: time.Now(),
	}

	room.Participants[userID] = participant
	client.RoomID = roomID
	client.Role = role

	return room, nil
}

// LeaveRoom removes a participant from a room. If the room becomes empty it is deleted.
// If the owner leaves, ownership transfers to the first editor, or the room is deleted.
func (rm *RoomManager) LeaveRoom(roomID, userID string) {
	rm.mu.RLock()
	room, ok := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !ok {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	delete(room.Participants, userID)

	// Clear active editor if they left
	if room.ActiveEditor == userID {
		room.ActiveEditor = ""
	}

	// If owner left, transfer ownership
	if userID == room.OwnerID {
		for uid, p := range room.Participants {
			if p.Role == "editor" {
				room.OwnerID = uid
				p.Role = "owner"
				if p.Client != nil {
					p.Client.Role = "owner"
				}
				break
			}
		}
		// If no editor, promote first viewer
		if room.OwnerID == userID {
			for uid, p := range room.Participants {
				room.OwnerID = uid
				p.Role = "owner"
				if p.Client != nil {
					p.Client.Role = "owner"
				}
				break
			}
		}
	}

	// Delete room if empty
	if len(room.Participants) == 0 {
		rm.mu.Lock()
		delete(rm.rooms, roomID)
		rm.mu.Unlock()
	}
}

// GetRoom retrieves a room by ID.
func (rm *RoomManager) GetRoom(roomID string) (*Room, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	room, ok := rm.rooms[roomID]
	return room, ok
}

// ListRooms returns all active rooms.
func (rm *RoomManager) ListRooms() []*Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		result = append(result, room)
	}
	return result
}

// SetRole changes a participant's role. Only the room owner can do this.
func (rm *RoomManager) SetRole(roomID, callerID, targetUserID, role string) error {
	if role != "editor" && role != "viewer" && role != "kicked" {
		return ErrInvalidRole
	}

	rm.mu.RLock()
	room, ok := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !ok {
		return ErrRoomNotFound
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if room.OwnerID != callerID {
		return ErrNotOwner
	}

	if targetUserID == room.OwnerID {
		return errors.New("cannot change the owner's role")
	}

	p, exists := room.Participants[targetUserID]
	if !exists {
		return ErrNotInRoom
	}

	if role == "kicked" {
		delete(room.Participants, targetUserID)
		if room.ActiveEditor == targetUserID {
			room.ActiveEditor = ""
		}
		if p.Client != nil {
			p.Client.RoomID = ""
			p.Client.Role = ""
		}
		return nil
	}

	p.Role = role
	if p.Client != nil {
		p.Client.Role = role
	}
	return nil
}

// GetActiveEditor returns the user ID of the participant who currently has the wheel.
func (rm *RoomManager) GetActiveEditor(roomID string) string {
	rm.mu.RLock()
	room, ok := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !ok {
		return ""
	}

	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.ActiveEditor
}

// TakeWheel attempts to take editor control. Succeeds if no one has the wheel,
// the requesting user already has it, or the current holder has been inactive > 30s.
func (r *Room) TakeWheel(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Already the editor
	if r.ActiveEditor == userID {
		return true
	}

	// No one has the wheel
	if r.ActiveEditor == "" {
		r.ActiveEditor = userID
		r.WheelTakenAt = time.Now()
		return true
	}

	// Current holder has been inactive
	if time.Since(r.WheelTakenAt) > wheelInactiveTimeout {
		r.ActiveEditor = userID
		r.WheelTakenAt = time.Now()
		return true
	}

	return false
}

// ReleaseWheel releases editor control back to viewer.
func (r *Room) ReleaseWheel(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ActiveEditor == userID {
		r.ActiveEditor = ""
		r.WheelTakenAt = time.Time{}
	}
}

// BroadcastMessage sends a wsResponse to all participants in the room.
func (r *Room) BroadcastMessage(msg wsResponse) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.Participants {
		if p.Client == nil {
			continue
		}
		select {
		case p.Client.send <- data:
		default:
			// client buffer full, skip
		}
	}
}

// BroadcastPresence sends a presence update to all participants.
func (r *Room) BroadcastPresence() {
	r.mu.RLock()
	participants := make([]map[string]any, 0, len(r.Participants))
	for _, p := range r.Participants {
		participants = append(participants, map[string]any{
			"user_id":   p.UserID,
			"role":      p.Role,
			"is_typing": p.IsTyping,
			"joined_at": p.JoinedAt.Format(time.RFC3339),
		})
	}
	activeEditor := r.ActiveEditor
	r.mu.RUnlock()

	r.BroadcastMessage(wsResponse{
		Type: "presence_update",
		Data: map[string]any{
			"room_id":        r.ID,
			"participants":   participants,
			"active_editor":  activeEditor,
		},
	})
}

// participantList returns a serializable list of participants.
func (r *Room) participantList() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]map[string]any, 0, len(r.Participants))
	for _, p := range r.Participants {
		list = append(list, map[string]any{
			"user_id":   p.UserID,
			"role":      p.Role,
			"is_typing": p.IsTyping,
			"joined_at": p.JoinedAt.Format(time.RFC3339),
		})
	}
	return list
}

// roomSummary returns a serializable summary of the room.
func roomSummary(r *Room) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]any{
		"id":              r.ID,
		"name":            r.Name,
		"session_id":      r.SessionID,
		"owner_id":        r.OwnerID,
		"created_at":      r.CreatedAt.Format(time.RFC3339),
		"participant_count": len(r.Participants),
		"active_editor":   r.ActiveEditor,
	}
}
