package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type Message struct {
	From    string
	To      string
	Subject string
	HTML    string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

const (
	KindSMTP     = "smtp"
	KindResend   = "resend"
	KindSendgrid = "sendgrid"
	KindMailgun  = "mailgun"
	KindPostmark = "postmark"
	KindBrevo    = "brevo"
	KindTelegram = "telegram"
	KindSlack    = "slack"
	KindDiscord  = "discord"
)

var ProviderKinds = []string{KindSMTP, KindResend, KindSendgrid, KindMailgun, KindPostmark, KindBrevo, KindTelegram, KindSlack, KindDiscord}

func NewSender(kind string, cfg map[string]any, fromEmail string) (Sender, error) {
	switch kind {
	case KindSMTP:
		return newSMTP(cfg, fromEmail)
	case KindResend:
		return newResend(cfg, fromEmail), nil
	case KindSendgrid:
		return newSendgrid(cfg, fromEmail), nil
	case KindMailgun:
		return newMailgun(cfg, fromEmail), nil
	case KindPostmark:
		return newPostmark(cfg, fromEmail), nil
	case KindBrevo:
		return newBrevo(cfg, fromEmail), nil
	case KindTelegram:
		return newTelegram(cfg)
	case KindSlack:
		return newSlack(cfg)
	case KindDiscord:
		return newDiscord(cfg)
	}
	return nil, fmt.Errorf("unknown provider kind: %s", kind)
}

func newTelegram(cfg map[string]any) (Sender, error) {
	token, _ := cfg["bot_token"].(string)
	chatID, _ := cfg["chat_id"].(string)
	if token == "" || chatID == "" {
		return nil, errors.New("telegram requires bot_token and chat_id")
	}
	return &telegramSender{botToken: token, chatID: chatID, url: "https://api.telegram.org/bot" + token + "/sendMessage"}, nil
}

type telegramSender struct {
	botToken string
	chatID   string
	url      string
}

func (t *telegramSender) Send(ctx context.Context, m Message) error {
	text := messageText(m)
	body, err := json.Marshal(map[string]any{
		"chat_id": t.chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// htmlToText flattens simple alert emails into readable plain text for
// chat providers that do not render HTML.
func htmlToText(s string) string {
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "</tr>", "\n")
	s = strings.ReplaceAll(s, "</td>", "  ")
	s = strings.ReplaceAll(s, "</h3>", "\n")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			for i < len(s) && s[i] != '>' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	out := b.String()
	out = strings.Join(strings.Fields(strings.ReplaceAll(out, "&amp;", "&")), " ")
	return strings.TrimSpace(out)
}

// messageText renders a Message as chat plain text, used by the Slack and
// Discord webhook senders.
func messageText(m Message) string {
	if m.Text != "" {
		return m.Text
	}
	subject := m.Subject
	if subject != "" {
		return subject + "\n" + htmlToText(m.HTML)
	}
	return htmlToText(m.HTML)
}

func webhookSender(cfg map[string]any, name, defaultURL string, build func(text string) ([]byte, error)) (Sender, error) {
	u, _ := cfg["webhook_url"].(string)
	if u == "" {
		u = defaultURL
	}
	if !strings.HasPrefix(u, "https://") {
		return nil, errors.New(name + " requires an https webhook_url")
	}
	return &webhookMessageSender{url: u, build: build, name: name}, nil
}

func newSlack(cfg map[string]any) (Sender, error) {
	return webhookSender(cfg, "slack", "", func(text string) ([]byte, error) {
		return json.Marshal(map[string]any{"text": text})
	})
}

func newDiscord(cfg map[string]any) (Sender, error) {
	return webhookSender(cfg, "discord", "", func(text string) ([]byte, error) {
		if len(text) > 1900 {
			text = text[:1900] + "…"
		}
		return json.Marshal(map[string]any{"content": text})
	})
}

// webhookMessageSender posts pre-built JSON to an incoming webhook URL
// (Slack, Discord, or any compatible chat hook).
type webhookMessageSender struct {
	url   string
	name  string
	build func(text string) ([]byte, error)
}

func (w *webhookMessageSender) Send(ctx context.Context, m Message) error {
	body, err := w.build(messageText(m))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "webstats/1.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("%s send: %w", w.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s status %d: %s", w.name, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func newSMTP(cfg map[string]any, fromEmail string) (Sender, error) {
	host, _ := cfg["host"].(string)
	port, _ := cfg["port"].(float64)
	user, _ := cfg["user"].(string)
	pass, _ := cfg["pass"].(string)
	enc, _ := cfg["encryption"].(string)
	if host == "" || port <= 0 {
		return nil, errors.New("smtp requires host and port")
	}
	return &smtpSender{host: host, port: int(port), user: user, pass: pass, ssl: enc == "ssl", from: fromEmail}, nil
}

type smtpSender struct {
	host string
	port int
	user string
	pass string
	ssl  bool
	from string
}

func (s *smtpSender) Send(ctx context.Context, m Message) error {
	from := m.From
	if from == "" {
		from = s.from
	}
	addr := net.JoinHostPort(s.host, fmt.Sprint(s.port))
	msg := []byte("From: " + from + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + mimeHeader(m.Subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" + m.HTML)

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	if s.ssl {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			var tlsConn *tls.Conn
			tlsConn = tls.Client(conn, &tls.Config{ServerName: s.host})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return fmt.Errorf("smtp tls handshake: %w", err)
			}
			conn = tlsConn
		}
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp hello: %w", err)
	}
	defer c.Close()
	if !s.ssl {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		} else if s.user != "" {
			return errors.New("smtp: server does not support STARTTLS but auth is configured")
		}
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("smtp cancelled: %w", err)
	}
	if s.user != "" {
		auth := smtp.PlainAuth("", s.user, s.pass, s.host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return c.Quit()
}

func newResend(cfg map[string]any, fromEmail string) Sender {
	return &apiSender{
		name: "resend",
		url:  "https://api.resend.com/emails",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+str(cfg["api_key"])) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"from": m.From, "to": []string{m.To},
				"subject": m.Subject, "html": m.HTML, "text": m.Text,
			})
		},
	}
}

func newSendgrid(cfg map[string]any, fromEmail string) Sender {
	region, _ := cfg["region"].(string)
	base := "https://api.sendgrid.com/v3/mail/send"
	if region == "eu" {
		base = "https://api.eu.sendgrid.com/v3/mail/send"
	}
	fromName, _ := cfg["from_name"].(string)
	return &apiSender{
		name: "sendgrid",
		url:  base,
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+str(cfg["api_key"])) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"personalizations": []map[string]any{{"to": []map[string]string{{"email": m.To}}}},
				"from":             map[string]string{"email": fromEmail, "name": fromName},
				"subject":          m.Subject,
				"content":          []map[string]string{{"type": "text/html", "value": m.HTML}},
			})
		},
	}
}

func newMailgun(cfg map[string]any, fromEmail string) Sender {
	domain, _ := cfg["domain"].(string)
	apiKey, _ := cfg["api_key"].(string)
	return &apiSender{
		name: "mailgun",
		url:  "https://api.mailgun.net/v3/" + domain + "/messages",
		from: fromEmail,
		auth: func(r *http.Request) {
			r.SetBasicAuth("api", apiKey)
		},
		body: func(m Message) ([]byte, error) {
			form := url.Values{}
			form.Set("from", m.From)
			form.Set("to", m.To)
			form.Set("subject", m.Subject)
			form.Set("html", m.HTML)
			form.Set("text", m.Text)
			return []byte(form.Encode()), nil
		},
		contentType: "application/x-www-form-urlencoded",
	}
}

func newPostmark(cfg map[string]any, fromEmail string) Sender {
	return &apiSender{
		name: "postmark",
		url:  "https://api.postmarkapp.com/email",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("X-Postmark-Server-Token", str(cfg["server_token"])) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"From": m.From, "To": m.To, "Subject": m.Subject,
				"HtmlBody": m.HTML, "TextBody": m.Text, "MessageStream": "outbound",
			})
		},
	}
}

func newBrevo(cfg map[string]any, fromEmail string) Sender {
	fromName, _ := cfg["from_name"].(string)
	return &apiSender{
		name: "brevo",
		url:  "https://api.brevo.com/v3/smtp/email",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("api-key", str(cfg["api_key"])) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"sender":      map[string]string{"email": fromEmail, "name": fromName},
				"to":          []map[string]string{{"email": m.To}},
				"subject":     m.Subject,
				"htmlContent": m.HTML,
			})
		},
	}
}

type apiSender struct {
	name        string
	url         string
	from        string
	auth        func(r *http.Request)
	body        func(m Message) ([]byte, error)
	contentType string
}

func (s *apiSender) Send(ctx context.Context, m Message) error {
	if s.from != "" && m.From == "" {
		m.From = s.from
	}
	body, err := s.body(m)
	if err != nil {
		return fmt.Errorf("%s encode: %w", s.name, err)
	}
	ct := s.contentType
	if ct == "" {
		ct = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s request: %w", s.name, err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webstats/1.0")
	s.auth(req)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s send: %w", s.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s status %d: %s", s.name, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

type Webhook struct {
	URL    string
	Secret string
}

func (w *Webhook) Send(ctx context.Context, payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "webstats/1.0")
	if w.Secret != "" {
		req.Header.Set("X-Webstats-Secret", w.Secret)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func mimeHeader(s string) string {
	clean := strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	if len(clean) == 0 {
		return ""
	}
	// Non-ASCII subject/site names must be RFC 2047 encoded, otherwise
	// mail servers mangle or reject the message.
	for _, r := range clean {
		if r > 127 {
			return mime.QEncoding.Encode("utf-8", clean)
		}
	}
	return clean
}
