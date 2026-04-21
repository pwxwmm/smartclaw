package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
)

const emailMaxSubjectLen = 200
const emailMaxBodyLen = 100000

type EmailAdapter struct {
	smtpHost string
	smtpPort string
	username string
	password string
	imapHost string
	imapPort string
	gateway  *gateway.Gateway
	fromAddr string

	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	pollInterval time.Duration
}

func NewEmailAdapter(smtpHost, smtpPort, username, password, imapHost, imapPort string, gw *gateway.Gateway) *EmailAdapter {
	return &EmailAdapter{
		smtpHost:    smtpHost,
		smtpPort:    smtpPort,
		username:    username,
		password:    password,
		imapHost:    imapHost,
		imapPort:    imapPort,
		gateway:     gw,
		fromAddr:    username,
		pollInterval: 30 * time.Second,
	}
}

func (ea *EmailAdapter) Name() string { return "email" }

func (ea *EmailAdapter) Send(userID string, response *gateway.GatewayResponse) error {
	if _, err := mail.ParseAddress(userID); err != nil {
		return fmt.Errorf("email: invalid address %q: %w", userID, err)
	}

	subject := "SmartClaw Response"
	if len(response.Content) > emailMaxSubjectLen {
		subject = response.Content[:emailMaxSubjectLen-3] + "..."
	}

	body := response.Content
	if len(body) > emailMaxBodyLen {
		body = body[:emailMaxBodyLen-len("[...truncated]")] + "[...truncated]"
	}

	return ea.sendMail(userID, subject, body)
}

func (ea *EmailAdapter) Start(ctx context.Context) error {
	slog.Info("email: adapter starting", "smtp", ea.smtpHost+":"+ea.smtpPort, "imap", ea.imapHost+":"+ea.imapPort)

	ea.mu.Lock()
	if ea.running {
		ea.mu.Unlock()
		return fmt.Errorf("email: adapter already running")
	}
	ea.running = true
	ea.stopCh = make(chan struct{})
	ea.mu.Unlock()

	ticker := time.NewTicker(ea.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ea.Stop()
			return nil
		case <-ea.stopCh:
			return nil
		case <-ticker.C:
			ea.pollInbox(ctx)
		}
	}
}

func (ea *EmailAdapter) Stop() error {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	if ea.running {
		close(ea.stopCh)
		ea.running = false
	}
	slog.Info("email: adapter stopped")
	return nil
}

func (ea *EmailAdapter) sendMail(to, subject, body string) error {
	addr := ea.smtpHost + ":" + ea.smtpPort

	from := ea.fromAddr
	if from == "" {
		from = ea.username
	}

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	auth := smtp.PlainAuth("", ea.username, ea.password, ea.smtpHost)

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String())); err != nil {
		return fmt.Errorf("email: sendMail error: %w", err)
	}

	return nil
}

func (ea *EmailAdapter) pollInbox(ctx context.Context) {
	if ea.imapHost == "" {
		return
	}

	slog.Debug("email: polling inbox", "imap", ea.imapHost+":"+ea.imapPort)

	conn, err := connectIMAP(ea.imapHost, ea.imapPort, ea.username, ea.password)
	if err != nil {
		slog.Warn("email: IMAP connect error", "error", err)
		return
	}
	defer conn.close()

	messages, err := conn.fetchUnseen()
	if err != nil {
		slog.Warn("email: IMAP fetch error", "error", err)
		return
	}

	for _, msg := range messages {
		from := extractSender(msg.From)
		if from == "" {
			continue
		}

		text := strings.TrimSpace(msg.Body)
		if text == "" {
			continue
		}

		slog.Info("email: received message", "from", from, "subject", msg.Subject, "length", len(text))

		if ea.gateway != nil {
			msgCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			resp, err := ea.gateway.HandleMessage(msgCtx, from, "email", text)
			cancel()
			if err != nil {
				slog.Warn("email: gateway error", "error", err)
				continue
			}
			if resp != nil {
				ea.Send(from, resp)
			}
		}
	}
}

func extractSender(from string) string {
	if from == "" {
		return ""
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		parts := strings.Split(from, " ")
		for _, p := range parts {
			p = strings.Trim(p, "<>")
			if strings.Contains(p, "@") {
				return p
			}
		}
		return ""
	}
	return addr.Address
}

type imapMessage struct {
	From    string
	Subject string
	Body    string
}

type imapConn struct {
	host     string
	port     string
	username string
	password string
}

func connectIMAP(host, port, username, password string) (*imapConn, error) {
	return &imapConn{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}, nil
}

func (ic *imapConn) close() {}

func (ic *imapConn) fetchUnseen() ([]imapMessage, error) {
	return nil, nil
}
