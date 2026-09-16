package ingest

import (
	"sync"
	"time"
)

// siteLimiter is a token bucket per resolved site id. It protects the
// database from ingest floods (runaway trackers, loop bugs, abusive
// scripts) with a hard cap independent of the buffer's backpressure.
type siteLimiter struct {
	mu      sync.Mutex
	buckets map[string]*limiterBucket
	rate    float64 // tokens per second
	burst   float64
}

type limiterBucket struct {
	tokens float64
	at     time.Time
}

func newSiteLimiter(perMinute int) *siteLimiter {
	if perMinute <= 0 {
		return nil
	}
	return &siteLimiter{
		buckets: map[string]*limiterBucket{},
		rate:    float64(perMinute) / 60.0,
		burst:   float64(perMinute),
	}
}

func (l *siteLimiter) allow(key string) bool {
	if l == nil {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 65536 {
		for k, b := range l.buckets {
			if now.Sub(b.at) > time.Hour {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &limiterBucket{tokens: l.burst - 1, at: now}
		return true
	}
	b.tokens += now.Sub(b.at).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
