package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/webstats/backend/internal/config"
)

// newTestBufferOpts builds a Buffer without a database. Callers inject
// dependencies and then call Run themselves so no dependency is captured
// mid-flight.
func newTestBufferOpts(bufferSize, batchSize int) *Buffer {
	cfg := &config.Config{BufferSize: bufferSize, BatchSize: batchSize, FlushEvery: time.Hour, IPHashSalt: "s"}
	b := NewBuffer(cfg, nil, nil, nil)
	b.retryEvery = 20 * time.Millisecond
	b.shutdownGrace = 5 * time.Millisecond
	return b
}

// Verifies B03: enqueue failures propagate to the caller instead of being
// dropped silently with an HTTP success.
func TestPushReturnsErrorWhenFull(t *testing.T) {
	b := newTestBufferOpts(1, 10)
	if err := b.Push(Record{Kind: "pageview", SiteID: "s", Path: "/"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Push(Record{Kind: "pageview", SiteID: "s", Path: "/"}); !errors.Is(err, ErrBufferFull) {
		t.Fatalf("expected ErrBufferFull, got %v", err)
	}
}

// Verifies B03 (retention) + B06 (batching): a persist failure must retain
// the batch for retry, and flushes must fill to BatchSize before persisting.
func TestFlushRetainsBatchOnPersistError(t *testing.T) {
	b := newTestBufferOpts(64, 3)

	var mu sync.Mutex
	var calls int
	var sizes []int
	b.persist = func(ctx context.Context, recs []Record) error {
		mu.Lock()
		sizes = append(sizes, len(recs))
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			return errors.New("simulated persistence failure")
		}
		return nil
	}
	b.Run(context.Background())
	t.Cleanup(func() { b.Stop() })

	for i := 0; i < 3; i++ {
		if err := b.Push(Record{Kind: "pageview", SiteID: "s", Path: "/"}); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(sizes) >= 2
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sizes) < 2 {
		t.Fatalf("batch not re-persisted after failure, sizes=%v", sizes)
	}
	if sizes[0] != 3 {
		t.Fatalf("first flush persisted %d records, want full batch of 3", sizes[0])
	}
	if sizes[1] != 3 {
		t.Fatalf("retry saw %d records, batch was not retained (want 3)", sizes[1])
	}
	if err := b.Stop(); err != nil {
		t.Fatalf("shutdown reported: %v", err)
	}
}

// Verifies B05: UTM attribution survives the Redis wire format round-trip.
func TestRecordUTMRoundTrip(t *testing.T) {
	r := Record{
		Kind: "pageview", SiteID: "s", SessionID: "sess", Path: "/", ID: "abc",
		UTM: map[string]string{"utm_source": "nl", "utm_medium": "email", "utm_campaign": "spring", "utm_content": "cta", "utm_term": "shoes"},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out Record
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"} {
		if out.UTM[k] != r.UTM[k] {
			t.Fatalf("UTM %s lost on wire: %q != %q", k, out.UTM[k], r.UTM[k])
		}
	}
}

// Verifies B17: client timestamps in the future are clamped to the server
// clock so they cannot pollute realtime presence.
func TestNormalizeClampsFutureTimestamp(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	b := testBuffer(t)
	now := time.Now().UTC()
	future := b.Normalize(map[string]any{
		"site_id": "s",
		"ts":      float64(now.Add(time.Hour).UnixMilli()),
	}, "8.8.8.8")
	if d := future.Ts.Sub(now); d > 5*time.Second {
		t.Fatalf("future ts accepted: +%v", d)
	}
	past := b.Normalize(map[string]any{
		"site_id": "s",
		"ts":      float64(now.Add(-time.Minute).UnixMilli()),
	}, "8.8.8.8")
	if d := now.Sub(past.Ts); d < 30*time.Second || d > 2*time.Minute {
		t.Fatalf("past ts mangled: %v", d)
	}
}
