package middleware

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxTrackedKeys = 10000

const defaultRateLimitPerMinute = 60

type bucket struct {
	tokens float64
	last   time.Time
}

type keyLimiter struct {
	capacity float64
	refill   float64 // tokens per second

	mu      sync.Mutex
	buckets map[uuid.UUID]*bucket
}

func (l *keyLimiter) allow(id uuid.UUID, now time.Time) (ok bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, seen := l.buckets[id]
	if !seen {
		l.evictFull(now)
		b = &bucket{tokens: l.capacity, last: now}
		l.buckets[id] = b
	} else {
		b.tokens = math.Min(l.capacity, b.tokens+now.Sub(b.last).Seconds()*l.refill)
		b.last = now
	}

	if b.tokens < 1 {
		wait := time.Duration((1-b.tokens)/l.refill*float64(time.Second)) + time.Second
		return false, 0, wait.Round(time.Second)
	}

	b.tokens--
	return true, int(b.tokens), 0
}

func (l *keyLimiter) evictFull(now time.Time) {
	if len(l.buckets) < maxTrackedKeys {
		return
	}

	for id, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.refill >= l.capacity {
			delete(l.buckets, id)
		}
	}

	for len(l.buckets) >= maxTrackedKeys {
		var oldest uuid.UUID
		var oldestAt time.Time
		for id, b := range l.buckets {
			if oldestAt.IsZero() || b.last.Before(oldestAt) {
				oldest, oldestAt = id, b.last
			}
		}
		delete(l.buckets, oldest)
	}
}

func RateLimitByKey(perMinute int) func(http.Handler) http.Handler {
	if perMinute < 1 {
		slog.Error("RATE_LIMIT_PER_MINUTE must be at least 1; using the default instead",
			"configured", perMinute, "using", defaultRateLimitPerMinute)
		perMinute = defaultRateLimitPerMinute
	}

	l := &keyLimiter{
		capacity: float64(perMinute),
		refill:   float64(perMinute) / 60,
		buckets:  make(map[uuid.UUID]*bucket),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := apiKeyID(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			allowed, remaining, retryAfter := l.allow(id, time.Now())

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(perMinute))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				writeAuthzError(w, http.StatusTooManyRequests, "Rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type keyToucher interface {
	TouchKey(ctx context.Context, id uuid.UUID) error
}

type keyUsageTracker struct {
	store    keyToucher
	interval time.Duration

	mu   sync.Mutex
	seen map[uuid.UUID]time.Time
}

func (k *keyUsageTracker) shouldTouch(id uuid.UUID, now time.Time) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	if last, ok := k.seen[id]; ok && now.Sub(last) < k.interval {
		return false
	}

	if len(k.seen) >= maxTrackedKeys {
		for key, at := range k.seen {
			if now.Sub(at) >= k.interval {
				delete(k.seen, key)
			}
		}
	}

	k.seen[id] = now
	return true
}

func TouchAPIKey(store keyToucher, interval time.Duration) func(http.Handler) http.Handler {
	tracker := &keyUsageTracker{
		store:    store,
		interval: interval,
		seen:     make(map[uuid.UUID]time.Time),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := apiKeyID(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if tracker.shouldTouch(id, time.Now()) {
				if err := tracker.store.TouchKey(r.Context(), id); err != nil {
					slog.Error("could not record API key usage", "key_id", id, "error", err)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
