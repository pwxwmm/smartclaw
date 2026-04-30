package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/watchdog"
)

// --- Cost API stubs (5 routes) ---

func (s *WebServer) handleCostDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"today_total":       0.0,
		"week_total":        0.0,
		"month_total":       0.0,
		"projected_monthly": 0.0,
		"budget_fraction":   0.0,
		"budget_remaining":  0.0,
		"history":           []any{},
		"model_breakdown":   []any{},
		"optimization_tips": []any{},
	})
}

func (s *WebServer) handleCostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

func (s *WebServer) handleCostForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"daily_forecasts":    []any{},
		"trend_direction":    "stable",
		"risk_level":         "low",
		"budget_sustain_days": 30,
		"today_remaining":    0.0,
		"week_projected":     0.0,
		"month_projected":    0.0,
		"recommendations":    []any{},
	})
}

func (s *WebServer) handleCostProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

func (s *WebServer) handleCostReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	now := time.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"period":           now.Format("2006-01-02"),
		"total_cost":       0.0,
		"vs_previous_week": 0.0,
		"top_model":        nil,
		"top_project":      nil,
		"savings_realized": 0.0,
		"tips":             []any{},
		"forecast": map[string]any{
			"trend_direction":  "stable",
			"risk_level":       "low",
			"week_projected":   0.0,
			"month_projected":  0.0,
			"daily_forecasts":  []any{},
		},
	})
}

// --- Onboarding API stubs (3 routes) ---

func (s *WebServer) handleOnboardingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state": map[string]any{
			"step": 1,
		},
	})
}

func (s *WebServer) handleOnboardingStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state": map[string]any{
			"step": 2,
		},
	})
}

func (s *WebServer) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"step": 0,
	})
}

// --- Profile API stubs (5 routes) ---

func (s *WebServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"preferences":         map[string]string{},
		"communication_style": "",
		"top_patterns":        []any{},
		"knowledge_background": []any{},
		"conflicts":           []any{},
	})
}

func (s *WebServer) handleProfileObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

func (s *WebServer) handleProfileStyle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *WebServer) handleProfileObservationDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *WebServer) handleProfileObservationsDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

// handleProfileObservationRoutes dispatches /api/profile/observations/ sub-routes.
// Path patterns:
//   - /api/profile/observations/delete-all  -> handleProfileObservationsDeleteAll
//   - /api/profile/observations/<id>        -> handleProfileObservationDelete
func (s *WebServer) handleProfileObservationRoutes(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/profile/observations/")
	switch {
	case suffix == "delete-all":
		s.handleProfileObservationsDeleteAll(w, r)
	default:
		s.handleProfileObservationDelete(w, r)
	}
}

// --- Marketplace API stubs (5 routes) ---

func (s *WebServer) handleMarketplaceCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, []string{
		"Code Review",
		"Deployment",
		"Debugging",
		"Security",
		"SRE",
		"Testing",
		"Automation",
	})
}

func (s *WebServer) handleMarketplaceFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

func (s *WebServer) handleMarketplaceSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"skills":    []any{},
		"total":     0,
		"page":      1,
		"page_size": 20,
	})
}

func (s *WebServer) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *WebServer) handleMarketplacePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *Handler) handleWatchdogStatusWS(client *Client) {
	wd := watchdog.DefaultWatchdog()
	var status watchdog.WatchdogStatus
	if wd != nil {
		status = wd.GetStatus()
	} else {
		status = watchdog.WatchdogStatus{
			Enabled:       false,
			ActiveWatches: []watchdog.ProcessWatch{},
			RecentErrors:  []watchdog.DetectedError{},
		}
	}
	h.sendToClient(client, WSResponse{
		Type: "watchdog_status",
		Data: status,
	})
}
