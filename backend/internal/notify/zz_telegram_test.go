package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSenderTelegram(t *testing.T) {
	s, err := NewSender("telegram", map[string]any{"bot_token": "123:abc", "chat_id": "-1001"}, "")
	if err != nil {
		t.Fatalf("telegram sender: %v", err)
	}
	if _, ok := s.(*telegramSender); !ok {
		t.Fatalf("kind telegram returned %T", s)
	}
	if _, err := NewSender("telegram", map[string]any{"bot_token": "123:abc"}, ""); err == nil {
		t.Fatal("missing chat_id accepted")
	}
}

func TestTelegramPayload(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s, err := NewSender("telegram", map[string]any{"bot_token": "123:abc", "chat_id": "-1001"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := s.(*telegramSender)
	ts.url = srv.URL
	if err := ts.Send(context.Background(), Message{
		HTML: `<div><p>Site is down</p><table><tr><td>Site</td><td>Example</td></tr></table></div>`,
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got["chat_id"] != "-1001" {
		t.Fatalf("chat_id = %v", got["chat_id"])
	}
	text, _ := got["text"].(string)
	if want := "Site is down"; !contains(text, want) {
		t.Fatalf("text %q missing %q", text, want)
	}
	if !contains(text, "Example") {
		t.Fatalf("text %q missing table cell content", text)
	}
}

func TestHtmlToText(t *testing.T) {
	in := `<p>a &amp; b</p><tr><td>x</td><td>y</td></tr><script>evil()</script>`
	out := htmlToText(in)
	if contains(out, "<") || contains(out, "script") {
		t.Fatalf("tags survived: %q", out)
	}
	if !contains(out, "a & b") || !contains(out, "x") {
		t.Fatalf("content lost: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
