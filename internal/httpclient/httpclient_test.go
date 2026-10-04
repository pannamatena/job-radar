package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fastConfig returns a Config with tiny durations so tests don't take real
// seconds. Callers override individual fields as needed.
func fastConfig() Config {
	return Config{
		UserAgent:       "job-radar-test/1 (+https://example.test)",
		MinHostInterval: 20 * time.Millisecond,
		BackoffBase:     1 * time.Millisecond,
		Timeout:         2 * time.Second,
	}
}

func TestGet_SendsUserAgentAndReadsBody(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, "hello world")
	}))
	defer srv.Close()

	c := New(fastConfig())
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != "hello world" {
		t.Fatalf("body = %q", resp.Body)
	}
	if gotUA != "job-radar-test/1 (+https://example.test)" {
		t.Fatalf("User-Agent = %q, not the configured value", gotUA)
	}
}

func TestGet_PerHostRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := fastConfig()
	cfg.MinHostInterval = 60 * time.Millisecond
	c := New(cfg)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), srv.URL); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}
	// 3 requests spaced by at least 60ms => at least 2 intervals = 120ms.
	if elapsed := time.Since(start); elapsed < 110*time.Millisecond {
		t.Fatalf("3 requests took %v, expected the per-host rate limit to space them to >=120ms", elapsed)
	}
}

func TestGet_MaxInFlight(t *testing.T) {
	var inFlight, maxSeen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if n <= old || atomic.CompareAndSwapInt32(&maxSeen, old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := fastConfig()
	cfg.MaxInFlight = 2
	cfg.MinHostInterval = 1 * time.Millisecond // don't let spacing hide concurrency
	c := New(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Get(context.Background(), srv.URL)
		}()
	}
	wg.Wait()

	if maxSeen > 2 {
		t.Fatalf("observed %d concurrent requests, MaxInFlight=2 should cap it at 2", maxSeen)
	}
}

func TestGet_PerRunRequestCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := fastConfig()
	cfg.MaxRequestsPerRun = 3
	cfg.MinHostInterval = 1 * time.Millisecond
	c := New(cfg)

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), srv.URL); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}
	_, err := c.Get(context.Background(), srv.URL)
	if !errors.Is(err, ErrRequestCapReached) {
		t.Fatalf("4th Get err = %v, want ErrRequestCapReached", err)
	}
	if total, _ := c.Stats(); total != 3 {
		t.Fatalf("Stats total = %d, want 3 (capped call must not count)", total)
	}
}

func TestGet_ResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", 5000))
	}))
	defer srv.Close()

	cfg := fastConfig()
	cfg.MaxResponseBytes = 1000
	c := New(cfg)

	_, err := c.Get(context.Background(), srv.URL)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
}

func TestGet_RetriesOn503ThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	c := New(fastConfig())
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.StatusCode != 200 || string(resp.Body) != "ok" {
		t.Fatalf("got %d %q, want 200 ok", resp.StatusCode, resp.Body)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("server saw %d attempts, want 2 (one 503 then one success)", got)
	}
}

func TestGet_NoRetryOn404(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := New(fastConfig())
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Get returned error for 404, want the response: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("server saw %d attempts, want 1 (4xx must not be retried)", got)
	}
}

func TestGet_CircuitBreakerOpensAndRecovers(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	// Controllable clock so we can jump past the cooldown without sleeping.
	now := time.Unix(0, 0)
	var clockMu sync.Mutex
	cfg := fastConfig()
	cfg.MinHostInterval = 1 * time.Millisecond
	cfg.MaxRetries = 1
	cfg.CircuitThreshold = 2
	cfg.CircuitCooldown = 1 * time.Minute
	cfg.now = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return now
	}
	c := New(cfg)

	// Two failing calls trip the breaker (threshold=2).
	for i := 0; i < 2; i++ {
		if _, err := c.Get(context.Background(), srv.URL); err == nil {
			t.Fatalf("call %d: expected an error from the 500 response", i)
		}
	}
	// Third call is rejected immediately by the open breaker.
	before := atomic.LoadInt32(&attempts)
	_, err := c.Get(context.Background(), srv.URL)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
	if atomic.LoadInt32(&attempts) != before {
		t.Fatalf("breaker-open call still hit the server")
	}

	// Advance past the cooldown; the breaker should close and allow a request.
	clockMu.Lock()
	now = now.Add(2 * time.Minute)
	clockMu.Unlock()
	before = atomic.LoadInt32(&attempts)
	if _, err := c.Get(context.Background(), srv.URL); err == nil {
		t.Fatalf("expected the 500 to still error after recovery")
	}
	if atomic.LoadInt32(&attempts) <= before {
		t.Fatalf("breaker did not close after cooldown; server was not contacted")
	}
}

func TestGet_HonoursContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(fastConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.Get(ctx, srv.URL); err == nil {
		t.Fatalf("expected a context/timeout error")
	}
}
