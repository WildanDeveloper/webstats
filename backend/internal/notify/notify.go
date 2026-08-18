package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
)

var ProviderKinds = []string{KindSMTP, KindResend, KindSendgrid, KindMailgun, KindPostmark, KindBrevo}

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
	}
	return nil, fmt.Errorf("unknown provider kind: %s", kind)
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
	if s.ssl {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr,
			&tls.Config{ServerName: s.host})
	} else {
		conn, err = net.DialTimeout("tcp", addr, 15*time.Second)
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
	return clean
}
