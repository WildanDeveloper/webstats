package notify

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	s := newResend(map[string]any{"api_key": "re_test"}, "WebStats <noreply@example.com>")
	s.(*apiSender).url = srv.URL + "/emails"
	err := s.Send(context.Background(), Message{From: "WebStats <noreply@example.com>", To: "ops@example.com", Subject: "Alert", HTML: "<p>hi</p>"})
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

	s := newSendgrid(map[string]any{"api_key": "SG_test", "from_name": "WebStats"}, "noreply@example.com")
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

	s := newMailgun(map[string]any{"domain": "mg.example.com", "api_key": "key-test"}, "noreply@example.com")
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

	s := newPostmark(map[string]any{"server_token": "pm_test"}, "noreply@example.com")
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

	s := newBrevo(map[string]any{"api_key": "xkeysib-test", "from_name": "WebStats"}, "noreply@example.com")
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

	s := newResend(map[string]any{"api_key": "bad"}, "noreply@example.com")
	s.(*apiSender).url = srv.URL + "/emails"
	err := s.Send(context.Background(), Message{To: "a@b.com", Subject: "s", HTML: "x"})
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

	s := &smtpSender{host: "127.0.0.1", port: ln.Addr().(*net.TCPAddr).Port, user: "", pass: "", ssl: false, from: "noreply@example.com"}
	err = s.Send(context.Background(), Message{To: "a@b.com", Subject: "Alert", HTML: "<p>x</p>"})
	if err == nil || !strings.Contains(err.Error(), "starttls") {
		t.Fatalf("expected starttls failure after TLS handoff, got %v", err)
	}
}

func TestNewSenderUnknownKind(t *testing.T) {
	if _, err := NewSender("slack", nil, ""); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
