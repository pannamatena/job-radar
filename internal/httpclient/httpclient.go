// Package httpclient is the single, shared HTTP client that every outbound
// request in job-radar must go through — sources, tools and notifiers alike.
// Routing all traffic through one place lets us be a polite, predictable
// client of every site we touch and makes it impossible for a bug or a
// misconfiguration to flood anyone. The rules it enforces (regardless of user
// config) are described in BUILD_PLAN.md §9a point 9 and ADR-0022:
//
//   - Identify ourselves with a descriptive User-Agent.
//   - Rate-limit per host (at most one request per MinHostInterval).
//   - Limit how many requests are in flight at once across the whole run.
//   - Cap the total number of requests in a single run.
//   - Time out every request and cap the response size.
//   - Back off on 429/503 (honouring Retry-After) and transient 5xx errors,
//     with a small bounded number of retries; never retry other 4xx.
//   - Trip a per-host circuit breaker after repeated failures so we stop
//     hammering a struggling host for the rest of the run.
//
// The DB-backed limits (a hard 60-minute minimum between fetches of the same
// source, cross-run cooling-off, and conditional-request caching) live with
// the store and are layered on top of this client in a later phase.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Sentinel errors callers can test for with errors.Is.
var (
	// ErrRequestCapReached means this run has already made the maximum number
	// of outbound requests allowed. No further requests will be attempted.
	ErrRequestCapReached = errors.New("httpclient: per-run request cap reached")

	// ErrCircuitOpen means too many requests to this host have failed, so the
	// host is being skipped for a cooling-off period.
	ErrCircuitOpen = errors.New("httpclient: circuit breaker open for host")

	// ErrResponseTooLarge means the response body exceeded the configured size
	// limit and was not fully read.
	ErrResponseTooLarge = errors.New("httpclient: response body too large")
)

// Default limits. These are deliberately conservative; see ADR-0022.
const (
	defaultTimeout          = 15 * time.Second
	defaultMaxResponseBytes = 10 << 20 // 10 MiB
	defaultMinHostInterval  = 1 * time.Second
	defaultMaxInFlight      = 2
	defaultMaxRequests      = 200
	defaultMaxRetries       = 2
	defaultBackoffBase      = 500 * time.Millisecond
	defaultCircuitThreshold = 5
	defaultCircuitCooldown  = 5 * time.Minute
)

// Config tunes the shared client. The zero value is not valid; use New, which
// fills in safe defaults for any field left at its zero value. Durations are
// injectable mainly so tests can use tiny values instead of real seconds.
type Config struct {
	UserAgent         string            // sent on every request; required
	Timeout           time.Duration     // per-request timeout
	MaxResponseBytes  int64             // response body size cap
	MinHostInterval   time.Duration     // minimum spacing between requests to one host
	MaxInFlight       int               // max concurrent requests across the whole run
	MaxRequestsPerRun int               // hard cap on total requests this run
	MaxRetries        int               // retries after the first attempt (transient failures only)
	BackoffBase       time.Duration     // base delay for exponential backoff
	CircuitThreshold  int               // consecutive host failures before the breaker trips
	CircuitCooldown   time.Duration     // how long the breaker stays open
	Transport         http.RoundTripper // underlying transport; nil uses a sane default
	now               func() time.Time  // injectable clock for tests; nil uses time.Now
}

// Client is the shared HTTP client. Create one per run with New and share it.
// It is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
	sem  chan struct{} // bounds concurrent requests

	mu       sync.Mutex
	reqCount int
	hosts    map[string]*hostState
}

type hostState struct {
	nextAllowed  time.Time // earliest time the next request to this host may start
	failures     int       // consecutive failures
	openUntil    time.Time // breaker is open until this time (zero = closed)
	requestCount int       // for visibility / logging
}

// Response is the result of a successful HTTP exchange. A non-2xx status is
// still returned here (not as an error) so callers can decide what a given
// status means for them; transport errors, exhausted retries and the limits
// above are returned as errors.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	URL        string
}

// New builds a shared client, filling in defaults for any unset Config field.
func New(cfg Config) *Client {
	if cfg.UserAgent == "" {
		cfg.UserAgent = "job-radar"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = defaultMaxResponseBytes
	}
	if cfg.MinHostInterval <= 0 {
		cfg.MinHostInterval = defaultMinHostInterval
	}
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = defaultMaxInFlight
	}
	if cfg.MaxRequestsPerRun <= 0 {
		cfg.MaxRequestsPerRun = defaultMaxRequests
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = defaultBackoffBase
	}
	if cfg.CircuitThreshold <= 0 {
		cfg.CircuitThreshold = defaultCircuitThreshold
	}
	if cfg.CircuitCooldown <= 0 {
		cfg.CircuitCooldown = defaultCircuitCooldown
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}
	transport := cfg.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{
		cfg:   cfg,
		http:  &http.Client{Transport: transport},
		sem:   make(chan struct{}, cfg.MaxInFlight),
		hosts: make(map[string]*hostState),
	}
}

// RequestOption customises a single request (e.g. conditional-request headers).
type RequestOption func(*http.Request)

// WithHeader sets a request header, used for things like If-None-Match.
func WithHeader(key, value string) RequestOption {
	return func(r *http.Request) { r.Header.Set(key, value) }
}

// Get performs a GET with all the shared-client protections applied.
func (c *Client) Get(ctx context.Context, rawURL string, opts ...RequestOption) (*Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("httpclient: invalid URL %q: %w", rawURL, err)
	}
	host := u.Host

	// Circuit breaker first: a host being skipped makes no outbound request, so
	// it must not consume a request-cap slot or a host rate-limit slot.
	if err := c.checkCircuit(host); err != nil {
		return nil, err
	}

	// Per-run total request cap (checked and reserved up front).
	if err := c.reserveRequest(); err != nil {
		return nil, err
	}

	// Per-host rate limit: wait until this host's next slot, then proceed.
	if err := c.awaitHostSlot(ctx, host); err != nil {
		return nil, err
	}

	// Global concurrency limit.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	resp, err := c.doWithRetries(ctx, u.String(), host, opts)
	c.recordOutcome(host, err, resp)
	return resp, err
}

// reserveRequest atomically checks and increments the per-run request counter.
func (c *Client) reserveRequest() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reqCount >= c.cfg.MaxRequestsPerRun {
		return ErrRequestCapReached
	}
	c.reqCount++
	return nil
}

// checkCircuit returns ErrCircuitOpen if the host's breaker is currently open.
func (c *Client) checkCircuit(host string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	hs := c.hostLocked(host)
	if !hs.openUntil.IsZero() {
		if c.cfg.now().Before(hs.openUntil) {
			return fmt.Errorf("%w: %s", ErrCircuitOpen, host)
		}
		// Cooldown elapsed: close the breaker and give the host another chance.
		hs.openUntil = time.Time{}
		hs.failures = 0
	}
	return nil
}

// awaitHostSlot reserves this host's next request slot and sleeps until it is
// due, so requests to the same host are spaced by at least MinHostInterval.
func (c *Client) awaitHostSlot(ctx context.Context, host string) error {
	c.mu.Lock()
	hs := c.hostLocked(host)
	now := c.cfg.now()
	start := hs.nextAllowed
	if start.Before(now) {
		start = now
	}
	hs.nextAllowed = start.Add(c.cfg.MinHostInterval)
	wait := start.Sub(now)
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// doWithRetries makes the request, retrying transient failures with backoff.
func (c *Client) doWithRetries(ctx context.Context, urlStr, host string, opts []RequestOption) (*Response, error) {
	var lastErr error
	var pendingRetryAfter time.Duration // server-requested delay carried to the next attempt
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.backoff(attempt)
			if pendingRetryAfter > 0 {
				delay = pendingRetryAfter
			}
			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
		}

		resp, retryable, ra, err := c.doOnce(ctx, urlStr, opts)
		if err == nil && !retryable {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("httpclient: %s returned HTTP %d", host, resp.StatusCode)
		}
		pendingRetryAfter = ra
		if !retryable {
			return resp, err
		}
	}
	return nil, fmt.Errorf("httpclient: giving up on %s after %d retries: %w", host, c.cfg.MaxRetries, lastErr)
}

// doOnce performs a single HTTP attempt. It returns the response (if the
// exchange completed), whether the outcome is worth retrying, a server-
// requested Retry-After delay (if any), and a transport error (if any).
func (c *Client) doOnce(ctx context.Context, urlStr string, opts []RequestOption) (resp *Response, retryable bool, retryAfter time.Duration, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, false, 0, fmt.Errorf("httpclient: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	for _, opt := range opts {
		opt(req)
	}

	httpResp, err := c.http.Do(req)
	if err != nil {
		// Transport-level error (timeout, connection refused, DNS…): retryable.
		return nil, true, 0, fmt.Errorf("httpclient: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, readErr := c.readBody(httpResp)
	if readErr != nil {
		return nil, false, 0, readErr
	}

	out := &Response{
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header,
		Body:       body,
		URL:        urlStr,
	}

	switch {
	case httpResp.StatusCode == http.StatusTooManyRequests || httpResp.StatusCode == http.StatusServiceUnavailable:
		// 429/503: back off, honouring Retry-After if present.
		return out, true, parseRetryAfter(httpResp.Header.Get("Retry-After"), c.cfg.now()), nil
	case httpResp.StatusCode >= 500:
		// Other transient server errors: retry with plain backoff.
		return out, true, 0, nil
	default:
		// 2xx, 3xx and other 4xx: done. 4xx is a client error we never retry.
		return out, false, 0, nil
	}
}

// readBody reads the response body up to the configured size limit.
func (c *Client) readBody(resp *http.Response) ([]byte, error) {
	limited := io.LimitReader(resp.Body, c.cfg.MaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("httpclient: read body: %w", err)
	}
	if int64(len(body)) > c.cfg.MaxResponseBytes {
		return nil, fmt.Errorf("%w: exceeded %d bytes", ErrResponseTooLarge, c.cfg.MaxResponseBytes)
	}
	return body, nil
}

// recordOutcome updates the host's failure count and breaker state.
func (c *Client) recordOutcome(host string, err error, resp *Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hs := c.hostLocked(host)
	hs.requestCount++

	failed := err != nil || (resp != nil && resp.StatusCode >= 500) ||
		(resp != nil && resp.StatusCode == http.StatusTooManyRequests)
	if failed {
		hs.failures++
		if hs.failures >= c.cfg.CircuitThreshold {
			hs.openUntil = c.cfg.now().Add(c.cfg.CircuitCooldown)
		}
		return
	}
	hs.failures = 0
}

func (c *Client) hostLocked(host string) *hostState {
	hs := c.hosts[host]
	if hs == nil {
		hs = &hostState{}
		c.hosts[host] = hs
	}
	return hs
}

func (c *Client) backoff(attempt int) time.Duration {
	// Exponential: base * 2^(attempt-1), with up to 50% jitter.
	d := c.cfg.BackoffBase << (attempt - 1)
	jitter := time.Duration(rand.Int63n(int64(d/2) + 1))
	return d + jitter
}

// Stats reports how many requests this run has made in total and per host, for
// logging and `job-radar report`.
func (c *Client) Stats() (total int, perHost map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	perHost = make(map[string]int, len(c.hosts))
	for h, hs := range c.hosts {
		perHost[h] = hs.requestCount
	}
	return c.reqCount, perHost
}

// sleep waits for d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// parseRetryAfter interprets a Retry-After header, which may be either a number
// of seconds or an HTTP date. Returns 0 if absent or unparseable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
