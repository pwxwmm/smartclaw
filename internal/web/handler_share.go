package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func generateShareID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func (s *WebServer) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id required"})
		return
	}
	shareID := generateShareID()
	if s.handler.dataStore != nil {
		db := s.handler.dataStore.DB()
		if db != nil {
			_, err := db.Exec(`CREATE TABLE IF NOT EXISTS shared_sessions (share_id TEXT PRIMARY KEY, session_id TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, view_count INTEGER DEFAULT 0)`)
			if err == nil {
				db.Exec(`INSERT INTO shared_sessions (share_id, session_id) VALUES (?, ?)`, shareID, req.SessionID)
			}
		}
	}
	url := fmt.Sprintf("/share/%s", shareID)
	writeJSON(w, http.StatusOK, map[string]string{"share_id": shareID, "url": url})
}

func (s *WebServer) handleShareView(w http.ResponseWriter, r *http.Request) {
	shareID := r.URL.Path[len("/share/"):]
	if shareID == "" {
		http.NotFound(w, r)
		return
	}
	var sessionID string
	if s.handler.dataStore != nil {
		db := s.handler.dataStore.DB()
		if db != nil {
			db.Exec(`UPDATE shared_sessions SET view_count = view_count + 1 WHERE share_id = ?`, shareID)
			db.QueryRow(`SELECT session_id FROM shared_sessions WHERE share_id = ?`, shareID).Scan(&sessionID)
		}
	}
	if sessionID == "" {
		http.NotFound(w, r)
		return
	}
	var messages []map[string]any
	if s.handler.dataStore != nil {
		db := s.handler.dataStore.DB()
		if db != nil {
			rows, err := db.Query(`SELECT role, content, created_at FROM session_messages WHERE session_id = ? ORDER BY created_at`, sessionID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var role, content, ts string
					if rows.Scan(&role, &content, &ts) == nil {
						messages = append(messages, map[string]any{"role": role, "content": content, "timestamp": ts})
					}
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>SmartClaw - Shared Conversation</title><style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;background:#1a1a2e;color:#e0e0e0;max-width:800px;margin:0 auto;padding:20px}.header{text-align:center;padding:20px 0;border-bottom:1px solid #333;margin-bottom:20px}.header h1{font-size:18px;color:#ed8936}.msg{margin:12px 0;padding:12px 16px;border-radius:8px}.msg.user{background:#2a2a4a;margin-left:20%%}.msg.assistant{background:#252545;margin-right:20%%}.msg .role{font-size:11px;font-weight:600;color:#ed8936;margin-bottom:4px;text-transform:uppercase}.msg .content{line-height:1.6;white-space:pre-wrap;word-break:break-word}.msg .content code{background:#1a1a2e;padding:2px 6px;border-radius:3px;font-size:13px}.msg .content pre{background:#1a1a2e;padding:12px;border-radius:6px;overflow-x:auto;margin:8px 0}.footer{text-align:center;padding:20px 0;border-top:1px solid #333;margin-top:20px;font-size:12px;color:#888}</style></head><body><div class="header"><h1>&#129417; SmartClaw</h1><p>Shared Conversation</p></div>`)
	for _, m := range messages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		roleLabel := "You"
		if role == "assistant" {
			roleLabel = "SmartClaw"
		}
		fmt.Fprintf(w, `<div class="msg %s"><div class="role">%s</div><div class="content">%s</div></div>`, role, roleLabel, html.EscapeString(content))
	}
	fmt.Fprintf(w, `<div class="footer">Shared via SmartClaw</div></body></html>`)
}

func (s *WebServer) handleChatExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	var messages []map[string]any
	if s.handler.dataStore != nil {
		db := s.handler.dataStore.DB()
		if db != nil {
			rows, err := db.Query(`SELECT role, content, created_at FROM session_messages WHERE session_id = ? ORDER BY created_at`, sessionID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var role, content, ts string
					if rows.Scan(&role, &content, &ts) == nil {
						messages = append(messages, map[string]any{"role": role, "content": content, "timestamp": ts})
					}
				}
			}
		}
	}

	if format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=conversation.md")
		for _, m := range messages {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			fmt.Fprintf(w, "## %s\n\n%s\n\n---\n\n", role, content)
		}
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported format"})
}

func (s *WebServer) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Content string `json:"content"`
		Title   string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}

	if req.Title == "" {
		req.Title = "smartclaw-export"
	}

	sanitizedTitle := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, req.Title)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	bctx, bCancel := chromedp.NewContext(allocCtx)
	defer bCancel()

	ctx, cancel := context.WithTimeout(bctx, 30*time.Second)
	defer cancel()

	var pdfBuf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return fmt.Errorf("get frame tree: %w", err)
			}
			if frameTree == nil || frameTree.Frame == nil {
				return fmt.Errorf("no frame available")
			}
			return page.SetDocumentContent(frameTree.Frame.ID, req.Content).Do(ctx)
		}),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				Do(ctx)
			pdfBuf = data
			return err
		}),
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "PDF generation failed"})
		return
	}

	if len(pdfBuf) == 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "PDF generation produced empty result"})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, sanitizedTitle))
	w.Write(pdfBuf)
}
