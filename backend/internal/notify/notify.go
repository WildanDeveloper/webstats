package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
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

// notifyHTTPClient returns a client that refuses cross-origin redirects so
// credentials sent to the configured provider can never be forwarded to a
// redirect target on a different origin.
func notifyHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("redirect limit exceeded")
			}
			if len(via) > 0 && (req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host) {
				return errors.New("cross-origin redirect denied")
			}
			return nil
		},
	}
}

type Capability string

const (
	CapabilityEmail Capability = "email"
	CapabilityChat  Capability = "chat"
)

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

func CapabilityOf(kind string) Capability {
	switch kind {
	case KindTelegram, KindSlack, KindDiscord:
		return CapabilityChat
	case KindSMTP, KindResend, KindSendgrid, KindMailgun, KindPostmark, KindBrevo:
		return CapabilityEmail
	}
	return ""
}

func NewSenderFor(kind string, cfg map[string]any, fromEmail string, cap Capability) (Sender, error) {
	if cap == "" || CapabilityOf(kind) != cap {
		return nil, errors.New("provider does not support required delivery capability")
	}
	return NewSender(kind, cfg, fromEmail)
}

func NewSender(kind string, cfg map[string]any, fromEmail string) (Sender, error) {
	if CapabilityOf(kind) == CapabilityEmail {
		if _, err := parseEmailAddress(fromEmail); err != nil {
			return nil, errors.New("invalid from email")
		}
	}
	switch kind {
	case KindSMTP:
		return newSMTP(cfg, fromEmail)
	case KindResend:
		return newResend(cfg, fromEmail)
	case KindSendgrid:
		return newSendgrid(cfg, fromEmail)
	case KindMailgun:
		return newMailgun(cfg, fromEmail)
	case KindPostmark:
		return newPostmark(cfg, fromEmail)
	case KindBrevo:
		return newBrevo(cfg, fromEmail)
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
	return &telegramSender{
		botToken: token,
		chatID:   chatID,
		url:      "https://api.telegram.org/bot" + token + "/sendMessage",
	}, nil
}

type telegramSender struct {
	botToken string
	chatID   string
	url      string
}

func (t *telegramSender) Send(ctx context.Context, m Message) error {
	if m.To != "" {
		return errors.New("chat provider does not support email recipients")
	}
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
		return transportErr("telegram request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := notifyHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return transportErr("telegram send", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram status %d", resp.StatusCode)
	}
	return nil
}

var blockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"div": true, "dl": true, "dd": true, "dt": true, "fieldset": true,
	"figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "li": true, "main": true, "nav": true, "ol": true,
	"p": true, "pre": true, "section": true, "table": true, "tr": true,
	"ul": true,
}

// htmlToText flattens simple alert emails into readable plain text for
// chat providers that do not render HTML.
func htmlToText(s string) string {
	var b strings.Builder
	var link string
	skip := ""
	for i := 0; i < len(s); {
		c := s[i]
		if c == '<' {
			j := tagEnd(s[i:])
			if j < 0 {
				break
			}
			name, closing, attrs := parseTag(s[i+1 : i+j])
			i += j + 1
			if skip != "" {
				if name == skip && closing {
					skip = ""
				}
				continue
			}
			switch {
			case name == "script" || name == "style":
				skip = name
			case name == "br":
				b.WriteByte('\n')
			case name == "a" && !closing:
				link = attrValue(attrs, "href")
			case name == "a" && closing:
				if link != "" {
					b.WriteString(" (" + link + ")")
					link = ""
				}
			case blockTags[name]:
				b.WriteByte('\n')
			case name == "td" || name == "th":
				if closing {
					b.WriteString("  ")
				}
			}
			continue
		}
		if skip == "" {
			b.WriteByte(c)
		}
		i++
	}
	out := html.UnescapeString(b.String())
	lines := strings.Split(out, "\n")
	for k, line := range lines {
		lines[k] = strings.Join(strings.Fields(line), " ")
	}
	out = strings.Join(lines, "\n")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}

func tagEnd(s string) int {
	var quote byte
	for i := 1; i < len(s); i++ {
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
		} else if s[i] == '\'' || s[i] == '"' {
			quote = s[i]
		} else if s[i] == '>' {
			return i
		}
	}
	return -1
}

func parseTag(tag string) (name string, closing bool, attrs string) {
	tag = strings.TrimSpace(tag)
	if strings.HasPrefix(tag, "/") {
		closing = true
		tag = strings.TrimSpace(tag[1:])
	}
	i := strings.IndexAny(tag, " \t\r\n/")
	if i < 0 {
		return strings.ToLower(tag), closing, ""
	}
	return strings.ToLower(tag[:i]), closing, tag[i:]
}

func attrValue(attrs, key string) string {
	for attrs != "" {
		attrs = strings.TrimLeft(attrs, " \t\r\n/")
		i := strings.IndexAny(attrs, "= \t\r\n")
		if i < 0 {
			return ""
		}
		name := attrs[:i]
		attrs = strings.TrimLeft(attrs[i:], " \t\r\n")
		if !strings.HasPrefix(attrs, "=") {
			continue
		}
		attrs = strings.TrimLeft(attrs[1:], " \t\r\n")
		if attrs == "" {
			return ""
		}
		var value string
		if attrs[0] == '\'' || attrs[0] == '"' {
			i = strings.IndexByte(attrs[1:], attrs[0])
			if i < 0 {
				return ""
			}
			value, attrs = attrs[1:i+1], attrs[i+2:]
		} else {
			i = strings.IndexAny(attrs, " \t\r\n")
			if i < 0 {
				i = len(attrs)
			}
			value, attrs = attrs[:i], attrs[i:]
		}
		if strings.EqualFold(name, key) {
			return value
		}
	}
	return ""
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

func transportErr(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", prefix, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", prefix, context.DeadlineExceeded)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%s: %w", prefix, context.DeadlineExceeded)
	}
	return errors.New(prefix + ": delivery failed")
}

func webhookSender(cfg map[string]any, name, defaultURL string, build func(text string) ([]byte, error)) (Sender, error) {
	u, _ := cfg["webhook_url"].(string)
	if u == "" {
		u = defaultURL
	}
	if !strings.HasPrefix(u, "https://") {
		return nil, errors.New(name + " requires an https webhook_url")
	}
	return &webhookMessageSender{url: u, name: name, build: build}, nil
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
	if m.To != "" {
		return errors.New("chat provider does not support email recipients")
	}
	body, err := w.build(messageText(m))
	if err != nil {
		return transportErr(w.name+" encode", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return transportErr(w.name+" request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "webstats/1.0")
	resp, err := notifyHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return transportErr(w.name+" send", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s status %d", w.name, resp.StatusCode)
	}
	return nil
}

var smtpSendTimeout = 30 * time.Second

func parsePort(v any) (int, error) {
	var port int
	switch p := v.(type) {
	case float64:
		if p != float64(int(p)) {
			return 0, errors.New("smtp port must be an integer")
		}
		port = int(p)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return 0, errors.New("smtp port must be an integer")
		}
		port = n
	default:
		return 0, errors.New("smtp requires host and port")
	}
	if port < 1 || port > 65535 {
		return 0, errors.New("smtp port must be between 1 and 65535")
	}
	return port, nil
}

func newSMTP(cfg map[string]any, fromEmail string) (Sender, error) {
	host, _ := cfg["host"].(string)
	user, _ := cfg["user"].(string)
	pass, _ := cfg["pass"].(string)
	enc, _ := cfg["encryption"].(string)
	port, err := parsePort(cfg["port"])
	if err != nil {
		return nil, err
	}
	if host == "" {
		return nil, errors.New("smtp requires host and port")
	}
	s := &smtpSender{host: host, port: port, user: user, pass: pass, from: fromEmail}
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "ssl":
		s.ssl = true
	case "starttls", "":
		s.startTLS = true
	case "none", "plain":
	default:
		return nil, errors.New("smtp encryption must be none, starttls or ssl")
	}
	return s, nil
}

type smtpSender struct {
	host     string
	port     int
	user     string
	pass     string
	ssl      bool
	startTLS bool
	from     string
}

func parseEmailAddress(raw string) (*mail.Address, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return nil, errors.New("invalid email address")
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || !strings.Contains(addr.Address, "@") {
		return nil, errors.New("invalid email address")
	}
	return addr, nil
}

func (s *smtpSender) Send(ctx context.Context, m Message) (sendErr error) {
	ctx, cancel := context.WithTimeout(ctx, smtpSendTimeout)
	defer cancel()
	defer func() {
		if sendErr != nil && ctx.Err() != nil {
			sendErr = transportErr("smtp send", ctx.Err())
		}
	}()
	fromRaw := m.From
	if fromRaw == "" {
		fromRaw = s.from
	}
	fromAddr, err := parseEmailAddress(fromRaw)
	if err != nil {
		return fmt.Errorf("smtp from address invalid: %w", err)
	}
	toAddr, err := parseEmailAddress(m.To)
	if err != nil {
		return fmt.Errorf("smtp to address invalid: %w", err)
	}
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	msg := []byte("From: " + fromAddr.String() + "\r\n" +
		"To: " + toAddr.String() + "\r\n" +
		"Subject: " + mimeHeader(m.Subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" + m.HTML)

	deadline, _ := ctx.Deadline()
	dialer := &net.Dialer{Timeout: 15 * time.Second, Deadline: deadline}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return transportErr("smtp dial", err)
	}
	defer rawConn.Close()
	stop := context.AfterFunc(ctx, func() { rawConn.Close() })
	defer stop()
	if err := rawConn.SetDeadline(deadline); err != nil {
		return transportErr("smtp deadline", err)
	}
	var conn net.Conn = rawConn
	if s.ssl {
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: s.host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return transportErr("smtp tls handshake", err)
		}
		conn = tlsConn
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return transportErr("smtp hello", err)
	}
	if !s.ssl && s.startTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: server does not support STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
			return transportErr("smtp starttls", err)
		}
	}
	if s.user != "" {
		auth := smtp.PlainAuth("", s.user, s.pass, s.host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(toAddr.Address); err != nil {
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

func newResend(cfg map[string]any, fromEmail string) (Sender, error) {
	apiKey, _ := cfg["api_key"].(string)
	if apiKey == "" {
		return nil, errors.New("resend requires api_key")
	}
	return &apiSender{
		name: "resend",
		url:  "https://api.resend.com/emails",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apiKey) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"from": m.From, "to": []string{m.To},
				"subject": m.Subject, "html": m.HTML, "text": m.Text,
			})
		},
	}, nil
}

func newSendgrid(cfg map[string]any, fromEmail string) (Sender, error) {
	apiKey, _ := cfg["api_key"].(string)
	if apiKey == "" {
		return nil, errors.New("sendgrid requires api_key")
	}
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
		auth: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apiKey) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"personalizations": []map[string]any{{"to": []map[string]string{{"email": m.To}}}},
				"from":             map[string]string{"email": fromEmail, "name": fromName},
				"subject":          m.Subject,
				"content":          []map[string]string{{"type": "text/html", "value": m.HTML}},
			})
		},
	}, nil
}

func newMailgun(cfg map[string]any, fromEmail string) (Sender, error) {
	domain, _ := cfg["domain"].(string)
	apiKey, _ := cfg["api_key"].(string)
	if domain == "" || apiKey == "" {
		return nil, errors.New("mailgun requires domain and api_key")
	}
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
	}, nil
}

func newPostmark(cfg map[string]any, fromEmail string) (Sender, error) {
	token, _ := cfg["server_token"].(string)
	if token == "" {
		return nil, errors.New("postmark requires server_token")
	}
	return &apiSender{
		name: "postmark",
		url:  "https://api.postmarkapp.com/email",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("X-Postmark-Server-Token", token) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"From": m.From, "To": m.To, "Subject": m.Subject,
				"HtmlBody": m.HTML, "TextBody": m.Text, "MessageStream": "outbound",
			})
		},
	}, nil
}

func newBrevo(cfg map[string]any, fromEmail string) (Sender, error) {
	apiKey, _ := cfg["api_key"].(string)
	if apiKey == "" {
		return nil, errors.New("brevo requires api_key")
	}
	fromName, _ := cfg["from_name"].(string)
	return &apiSender{
		name: "brevo",
		url:  "https://api.brevo.com/v3/smtp/email",
		from: fromEmail,
		auth: func(r *http.Request) { r.Header.Set("api-key", apiKey) },
		body: func(m Message) ([]byte, error) {
			return json.Marshal(map[string]any{
				"sender":      map[string]string{"email": fromEmail, "name": fromName},
				"to":          []map[string]string{{"email": m.To}},
				"subject":     m.Subject,
				"htmlContent": m.HTML,
			})
		},
	}, nil
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
		return transportErr(s.name+" request", err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webstats/1.0")
	s.auth(req)
	resp, err := notifyHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return transportErr(s.name+" send", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s status %d", s.name, resp.StatusCode)
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
	resp, err := notifyHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return transportErr("webhook send", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
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
