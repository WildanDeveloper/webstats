package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResendPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer re_test" {
			t.Errorf("auth = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["from"] != "WebStats <noreply@example.com>" {
			t.Errorf("from = %v", body["from"])
		}
		if tos := body["to"].([]any); len(tos) != 1 || tos[0] != "ops@example.com" {
			t.Errorf("to = %v", body["to"])
		}
		if body["subject"] != "Alert" {
			t.Errorf("subject = %v", body["subject"])
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s, err := newResend(map[string]any{"api_key": "re_test"}, "WebStats <noreply@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/emails"
	err = s.Send(context.Background(), Message{From: "WebStats <noreply@example.com>", To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendgridPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/mail/send" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer SG_test" {
			t.Errorf("auth = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		p := body["personalizations"].([]any)[0].(map[string]any)
		if p["to"].([]any)[0].(map[string]any)["email"] != "ops@example.com" {
			t.Errorf("to = %v", p["to"])
		}
		from := body["from"].(map[string]any)
		if from["email"] != "noreply@example.com" {
			t.Errorf("from = %v", from)
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()

	s, err := newSendgrid(map[string]any{"api_key": "SG_test", "from_name": "WebStats"}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/v3/mail/send"
	if err := s.Send(context.Background(), Message{To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"}); err != nil {
		t.Fatal(err)
	}
}

func TestMailgunPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/mg.example.com/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "api" || p != "key-test" {
			t.Errorf("basic auth = %q %q %v", u, p, ok)
		}
		r.ParseForm()
		if r.PostForm.Get("to") != "ops@example.com" || r.PostForm.Get("subject") != "Alert" || r.PostForm.Get("html") != "<p>hi</p>" {
			t.Errorf("form = %v", r.PostForm)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s, err := newMailgun(map[string]any{"domain": "mg.example.com", "api_key": "key-test"}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/v3/mg.example.com/messages"
	if err := s.Send(context.Background(), Message{To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"}); err != nil {
		t.Fatal(err)
	}
}

func TestPostmarkPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Postmark-Server-Token"); got != "pm_test" {
			t.Errorf("token = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["To"] != "ops@example.com" || body["MessageStream"] != "outbound" || body["From"] != "noreply@example.com" {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s, err := newPostmark(map[string]any{"server_token": "pm_test"}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/email"
	if err := s.Send(context.Background(), Message{To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"}); err != nil {
		t.Fatal(err)
	}
}

func TestBrevoPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "xkeysib-test" {
			t.Errorf("api-key = %q", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["htmlContent"] != "<p>hi</p>" {
			t.Errorf("htmlContent = %v", body["htmlContent"])
		}
		sender := body["sender"].(map[string]any)
		if sender["email"] != "noreply@example.com" {
			t.Errorf("sender = %v", sender)
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()

	s, err := newBrevo(map[string]any{"api_key": "xkeysib-test", "from_name": "WebStats"}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/v3/smtp/email"
	if err := s.Send(context.Background(), Message{To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"}); err != nil {
		t.Fatal(err)
	}
}

func TestAPISenderErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"message":"unauthorized"}`)
	}))
	defer srv.Close()

	s, err := newResend(map[string]any{"api_key": "bad"}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	s.(*apiSender).url = srv.URL + "/emails"
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "s", HTML: "x"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}

func TestWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Webstats-Secret") != "sec" {
			t.Errorf("secret missing")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["event"] != "site_down" {
			t.Errorf("event = %v", body["event"])
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	w := &Webhook{URL: srv.URL, Secret: "sec"}
	if err := w.Send(context.Background(), map[string]any{"event": "site_down"}); err != nil {
		t.Fatal(err)
	}
}

func TestSMTPStartTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		line := func(c net.Conn) string {
			buf := make([]byte, 0, 512)
			one := make([]byte, 1)
			for {
				if _, err := c.Read(one); err != nil {
					return ""
				}
				buf = append(buf, one[0])
				if len(buf) > 1 && buf[len(buf)-1] == '\n' {
					return string(buf)
				}
			}
		}
		write := func(s string) { conn.Write([]byte(s)) }
		write("220 mock ESMTP\r\n")
		for {
			l := line(conn)
			switch {
			case strings.HasPrefix(l, "EHLO"):
				write("250-mock\r\n250-STARTTLS\r\n250 8BITMIME\r\n")
			case strings.HasPrefix(l, "STARTTLS"):
				write("220 go ahead\r\n")
				conn.Close()
				return
			case l == "":
				return
			default:
				write("250 ok\r\n")
			}
		}
	}()

	s := &smtpSender{host: "127.0.0.1", port: ln.Addr().(*net.TCPAddr).Port, user: "", pass: "", ssl: false, startTLS: true, from: "noreply@example.com"}
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "Alert", HTML: "<p>x</p>"})
	if err == nil || !strings.Contains(err.Error(), "starttls") {
		t.Fatalf("expected starttls failure after TLS handoff, got %v", err)
	}
}

func TestNewSenderUnknownKind(t *testing.T) {
	if _, err := NewSender("carrier-pigeon", nil, ""); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func startMockSMTP(t *testing.T, advertiseStartTLS bool) (string, func() (string, string, string)) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	mailFrom, rcptTo, body := "", "", ""
	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		write := func(s string) { conn.Write([]byte(s)) }
		write("220 mock ESMTP\r\n")
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if strings.TrimSpace(line) == "." {
					mu.Lock()
					body += line
					mu.Unlock()
					write("250 ok\r\n")
					inData = false
				} else {
					mu.Lock()
					body += line
					mu.Unlock()
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO") || strings.HasPrefix(line, "HELO"):
				if advertiseStartTLS {
					write("250-mock\r\n250-STARTTLS\r\n250 8BITMIME\r\n")
				} else {
					write("250-mock\r\n250 8BITMIME\r\n")
				}
			case strings.HasPrefix(line, "MAIL FROM:"):
				mu.Lock()
				mailFrom = strings.TrimSpace(line)
				mu.Unlock()
				write("250 ok\r\n")
			case strings.HasPrefix(line, "RCPT TO:"):
				mu.Lock()
				rcptTo = strings.TrimSpace(line)
				mu.Unlock()
				write("250 ok\r\n")
			case strings.HasPrefix(line, "DATA"):
				write("354 go ahead\r\n")
				inData = true
			case strings.HasPrefix(line, "QUIT"):
				write("221 bye\r\n")
				return
			default:
				write("250 ok\r\n")
			}
		}
	}()
	return ln.Addr().String(), func() (string, string, string) {
		mu.Lock()
		defer mu.Unlock()
		return mailFrom, rcptTo, body
	}
}

func TestSMTPEnvelopeMailboxOnlyHeaderFormatted(t *testing.T) {
	addr, captured := startMockSMTP(t, false)
	host, port, _ := net.SplitHostPort(addr)
	portNum, _ := strconv.Atoi(port)
	s, err := NewSender("smtp", map[string]any{"host": host, "port": float64(portNum), "encryption": "none"}, "WebStats <noreply@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), Message{From: "WebStats <noreply@example.com>", To: "Ops Team <ops@example.com>", Subject: "Alert", HTML: "<p>x</p>"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	mailFrom, rcptTo, body := captured()
	if !strings.HasPrefix(mailFrom, "MAIL FROM:<noreply@example.com>") {
		t.Errorf("envelope mail from = %q", mailFrom)
	}
	if rcptTo != "RCPT TO:<ops@example.com>" {
		t.Errorf("envelope rcpt to = %q", rcptTo)
	}
	if !strings.Contains(body, `From: "WebStats" <noreply@example.com>`) {
		t.Errorf("header from missing display name: %q", body)
	}
	if !strings.Contains(body, `To: "Ops Team" <ops@example.com>`) {
		t.Errorf("header to missing display name: %q", body)
	}
}

func TestSMTPRejectsInvalidAddresses(t *testing.T) {
	addr, _ := startMockSMTP(t, false)
	host, port, _ := net.SplitHostPort(addr)
	portNum, _ := strconv.Atoi(port)
	s := &smtpSender{host: host, port: portNum, ssl: false, startTLS: true, from: "WebStats <noreply@example.com>"}
	if err := s.Send(context.Background(), Message{To: "a@b.com", Subject: "s"}); err == nil {
		t.Fatal("expected invalid from to fail")
	}
	if err := s.Send(context.Background(), Message{From: "noreply@example.com", To: "not-an-email", Subject: "s"}); err == nil {
		t.Fatal("expected invalid to to fail")
	}
}

func TestSMTPStallAfterDialIsBounded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()
	old := smtpSendTimeout
	smtpSendTimeout = 500 * time.Millisecond
	defer func() { smtpSendTimeout = old }()
	s := &smtpSender{host: "127.0.0.1", port: ln.Addr().(*net.TCPAddr).Port, ssl: false, startTLS: true, from: "noreply@example.com"}
	start := time.Now()
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "s", HTML: "<p>x</p>"})
	if err == nil {
		t.Fatal("expected stall to fail")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("send not bounded: %v", elapsed)
	}
}

func TestSMTPCallerCancelBreaksStuckIO(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	s := &smtpSender{host: "127.0.0.1", port: ln.Addr().(*net.TCPAddr).Port, ssl: false, startTLS: true, from: "noreply@example.com"}
	start := time.Now()
	err = s.Send(ctx, Message{To: "a@b.com", Subject: "s", HTML: "<p>x</p>"})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("cancel did not break stuck IO: %v", elapsed)
	}
}

func TestSMTPStartTLSRequiredWhenConfigured(t *testing.T) {
	addr, _ := startMockSMTP(t, false)
	host, port, _ := net.SplitHostPort(addr)
	portNum, _ := strconv.Atoi(port)
	s, err := NewSender("smtp", map[string]any{"host": host, "port": float64(portNum)}, "noreply@example.com")
	if err != nil {
		t.Fatal(err)
	}
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "s", HTML: "<p>x</p>"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected STARTTLS enforcement error, got %v", err)
	}
}

func TestSMTPInvalidEncryptionRejected(t *testing.T) {
	if _, err := NewSender("smtp", map[string]any{"host": "smtp.example.com", "port": 587.0, "encryption": "tls"}, "noreply@example.com"); err == nil {
		t.Fatal("invalid encryption mode accepted")
	}
}

func TestTransportErrorsAreSanitized(t *testing.T) {
	w := &Webhook{URL: "http://user:secret@127.0.0.1:1/hook?token=abc"}
	err := w.Send(context.Background(), map[string]any{"event": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || strings.Contains(msg, "token=abc") || strings.Contains(msg, "user:") {
		t.Fatalf("credentials leaked: %q", msg)
	}
	s := &apiSender{name: "resend", url: "http://user:secret@127.0.0.1:1/emails?token=abc", auth: func(r *http.Request) {}, body: func(m Message) ([]byte, error) { return []byte("{}"), nil }}
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "s"})
	if err == nil {
		t.Fatal("expected error")
	}
	if msg := err.Error(); strings.Contains(msg, "secret") || strings.Contains(msg, "token=abc") || strings.Contains(msg, "user:") {
		t.Fatalf("credentials leaked: %q", msg)
	}
}

func TestNewSenderForCapability(t *testing.T) {
	if _, err := NewSenderFor("telegram", map[string]any{"bot_token": "t", "chat_id": "c"}, "", CapabilityEmail); err == nil {
		t.Fatal("telegram accepted for email capability")
	}
	if _, err := NewSenderFor("resend", map[string]any{"api_key": "k"}, "noreply@example.com", CapabilityEmail); err != nil {
		t.Fatalf("resend rejected for email capability: %v", err)
	}
	if _, err := NewSenderFor("resend", map[string]any{"api_key": "k"}, "noreply@example.com", CapabilityChat); err == nil {
		t.Fatal("resend accepted for chat capability")
	}
	if _, err := NewSenderFor("resend", map[string]any{"api_key": "k"}, "noreply@example.com", ""); err == nil {
		t.Fatal("empty capability accepted")
	}
}

func TestChatSendersRejectEmailRecipient(t *testing.T) {
	ts := &telegramSender{botToken: "t", chatID: "c", url: "https://api.telegram.org/bot/sendMessage"}
	if err := ts.Send(context.Background(), Message{To: "a@b.com", Subject: "s"}); err == nil {
		t.Fatal("telegram accepted email recipient")
	}
	ws := &webhookMessageSender{name: "slack", build: func(text string) ([]byte, error) { return json.Marshal(map[string]any{"text": text}) }}
	if err := ws.Send(context.Background(), Message{To: "a@b.com", Subject: "s"}); err == nil {
		t.Fatal("webhook sender accepted email recipient")
	}
}

func TestNewSenderSlackMissingWebhook(t *testing.T) {
	if _, err := NewSender("slack", map[string]any{}, ""); err == nil {
		t.Fatal("expected error for slack without webhook_url")
	}
}

func TestAPISendersRequireCredentials(t *testing.T) {
	cases := []struct {
		kind string
		cfg  map[string]any
	}{
		{"resend", map[string]any{}},
		{"sendgrid", map[string]any{"region": "eu"}},
		{"mailgun", map[string]any{"domain": "mg.example.com"}},
		{"mailgun", map[string]any{"api_key": "key"}},
		{"postmark", map[string]any{}},
		{"brevo", map[string]any{"from_name": "WebStats"}},
	}
	for _, tc := range cases {
		if _, err := NewSender(tc.kind, tc.cfg, "noreply@example.com"); err == nil {
			t.Errorf("%s accepted incomplete config %v", tc.kind, tc.cfg)
		}
	}
}

func TestSMTPRequiresHostAndValidPort(t *testing.T) {
	cases := []map[string]any{
		{"host": "smtp.example.com", "port": 0.0},
		{"host": "smtp.example.com", "port": -25.0},
		{"host": "smtp.example.com", "port": 443.5},
		{"host": "smtp.example.com", "port": 70000.0},
		{"host": "smtp.example.com", "port": "not-a-port"},
		{"port": 587.0},
	}
	for _, cfg := range cases {
		if _, err := NewSender("smtp", cfg, "noreply@example.com"); err == nil {
			t.Errorf("smtp accepted config %v", cfg)
		}
	}
	if _, err := NewSender("smtp", map[string]any{"host": "smtp.example.com", "port": 587.0}, "noreply@example.com"); err != nil {
		t.Errorf("integer port rejected: %v", err)
	}
	if _, err := NewSender("smtp", map[string]any{"host": "smtp.example.com", "port": "465"}, "noreply@example.com"); err != nil {
		t.Errorf("numeric string port rejected: %v", err)
	}
}

func TestNewSenderRejectsBadFromEmail(t *testing.T) {
	if _, err := NewSender("resend", map[string]any{"api_key": "k"}, "not-an-address"); err == nil {
		t.Fatal("invalid from email accepted")
	}
}
