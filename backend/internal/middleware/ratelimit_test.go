package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Two stable keys, so "a different key is unaffected" reads clearly.
var (
	keyA = uuid.New()
	keyB = uuid.New()
)

func newTestLimiter(perMinute int) *keyLimiter {
	return &keyLimiter{
		capacity: float64(perMinute),
		refill:   float64(perMinute) / 60,
		buckets:  make(map[uuid.UUID]*bucket),
	}
}

func TestBucketAllowsABurstThenRefuses(t *testing.T) {
	l := newTestLimiter(60)
	now := time.Now()

	for i := 1; i <= 60; i++ {
		ok, _, _ := l.allow(keyA, now)
		if !ok {
			t.Fatalf("request %d refused, want the first 60 allowed", i)
		}
	}

	ok, remaining, retryAfter := l.allow(keyA, now)
	if ok {
		t.Error("the 61st request was allowed")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
	// Not an exact number: pinning the rounding rule would break whenever the
	// limit changes.
	if retryAfter < time.Second {
		t.Errorf("retryAfter = %v, want at least a second", retryAfter)
	}
}

func TestBucketRefillsOverTime(t *testing.T) {
	l := newTestLimiter(60)
	now := time.Now()

	for i := 0; i < 60; i++ {
		l.allow(keyA, now)
	}

	ok, remaining, _ := l.allow(keyA, now.Add(30*time.Second))
	if !ok {
		t.Fatal("still refused after 30 seconds")
	}
	if remaining != 29 {
		t.Errorf("remaining = %d, want 29", remaining)
	}
}

func TestBucketsAreIndependentPerKey(t *testing.T) {
	l := newTestLimiter(2)
	now := time.Now()

	l.allow(keyA, now)
	l.allow(keyA, now)
	if ok, _, _ := l.allow(keyA, now); ok {
		t.Fatal("key 7 should be out of tokens")
	}

	if ok, _, _ := l.allow(keyB, now); !ok {
		t.Error("key 8 was throttled by key 7 - buckets are not independent")
	}
}

func TestBucketIsSafeUnderConcurrency(t *testing.T) {
	l := newTestLimiter(50)
	now := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _, _ := l.allow(keyA, now); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 50 {
		t.Errorf("allowed %d of 200, want exactly the capacity of 50", allowed)
	}
}

func TestRateLimitHeadersAndStatus(t *testing.T) {
	h := RateLimitByKey(2)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	key := uuid.New()
	call := func(withKey bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/listings", nil)
		if withKey {
			// The same key every time: a fresh one would get a fresh bucket.
			req = req.WithContext(context.WithValue(req.Context(), apiKeyIDKey{}, key))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := call(true); rec.Code != http.StatusNoContent || rec.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Errorf("first call: status %d, remaining %q", rec.Code, rec.Header().Get("X-RateLimit-Remaining"))
	}
	call(true)

	rec := call(true)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third call: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retry < 1 {
		t.Errorf("Retry-After = %q, want a positive integer", rec.Header().Get("Retry-After"))
	}
	if body := rec.Body.String(); body != `{"error":"Rate limit exceeded"}` {
		t.Errorf("body = %s, want the usual error envelope", body)
	}
}

func TestBrowserRequestsAreNotLimited(t *testing.T) {
	h := RateLimitByKey(1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/listings", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusNoContent)
		}
		if rec.Header().Get("X-RateLimit-Remaining") != "" {
			t.Error("a browser request carried rate-limit headers")
		}
	}
}

type countingToucher struct{ calls int }

func (c *countingToucher) TouchKey(context.Context, uuid.UUID) error {
	c.calls++
	return nil
}

func TestUsageIsRecordedAtMostOncePerInterval(t *testing.T) {
	store := &countingToucher{}
	h := TouchAPIKey(store, time.Minute)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	key := uuid.New()
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/listings", nil)
		req = req.WithContext(context.WithValue(req.Context(), apiKeyIDKey{}, key))
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	if store.calls != 1 {
		t.Errorf("wrote %d times for three requests, want 1", store.calls)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/listings", nil))
	if store.calls != 1 {
		t.Errorf("a browser request recorded key usage")
	}
}

func TestALimitBelowOneFallsBackToTheDefault(t *testing.T) {
	for _, configured := range []int{0, -5} {
		t.Run(strconv.Itoa(configured), func(t *testing.T) {
			h := RateLimitByKey(configured)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/listings", nil)
			req = req.WithContext(context.WithValue(req.Context(), apiKeyIDKey{}, uuid.New()))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want %d - the request was refused outright", rec.Code, http.StatusNoContent)
			}
			if got := rec.Header().Get("X-RateLimit-Limit"); got != strconv.Itoa(defaultRateLimitPerMinute) {
				t.Errorf("X-RateLimit-Limit = %q, want the default %d", got, defaultRateLimitPerMinute)
			}
		})
	}
}

func TestTheBucketMapStaysBounded(t *testing.T) {
	l := newTestLimiter(60)
	now := time.Now()

	for i := 0; i < maxTrackedKeys+500; i++ {
		id := uuid.New()
		l.allow(id, now)
		l.allow(id, now)
	}

	if len(l.buckets) > maxTrackedKeys {
		t.Errorf("tracking %d buckets, want at most %d", len(l.buckets), maxTrackedKeys)
	}
}
