package platform

import (
	"fmt"
	"log/slog"

	"github.com/instructkr/smartclaw/internal/gateway"
)

type TerminalAdapter struct{}

func NewTerminalAdapter() *TerminalAdapter {
	return &TerminalAdapter{}
}

func (ta *TerminalAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	slog.Info("terminal: delivering response", "user", userID, "session", response.SessionID)
	fmt.Println(response.Content)
	return nil
}

func (ta *TerminalAdapter) Name() string {
	return "terminal"
}

type WebAdapter struct {
	broadcastFunc func(message []byte)
}

func NewWebAdapter(broadcastFunc func(message []byte)) *WebAdapter {
	return &WebAdapter{broadcastFunc: broadcastFunc}
}

func (wa *WebAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	if wa.broadcastFunc != nil {
		msg := fmt.Sprintf(`{"type":"response","user_id":"%s","session_id":"%s","content":%q}`,
			userID, response.SessionID, response.Content)
		wa.broadcastFunc([]byte(msg))
	}
	return nil
}

func (wa *WebAdapter) Name() string {
	return "web"
}
