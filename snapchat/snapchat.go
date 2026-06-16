// Package snapchat is the library behind the snap command line:
// the HTTP client, block detection, and the typed data models for
// story.snapchat.com and www.snapchat.com.
//
// Snapchat is Tier C (anti-bot). Requests from datacenter IPs usually receive
// a Cloudflare challenge page (HTTP 403 or 200 with a JS interstitial) instead
// of real data. The client detects all known block patterns and returns
// errs.Blocked so callers can exit with code 5.
package snapchat

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/any-cli/kit/errs"
)

// DefaultUserAgent mimics a real desktop Chrome browser on macOS.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/124.0.0.0 Safari/537.36"

// Host and base URLs.
const (
	Host      = "snapchat.com"
	BaseURL   = "https://www.snapchat.com"
	StoryBase = "https://story.snapchat.com"
)

// Config holds tunable parameters for Client.
type Config struct {
	BaseURL   string
	StoryBase string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		StoryBase: StoryBase,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   2,
		Timeout:   20 * time.Second,
	}
}

// Client is a rate-limited HTTP client for Snapchat public endpoints.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Get fetches url with pacing and retries (exported for tests).
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	return c.get(ctx, url)
}

// get fetches url with pacing and retries.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	var r io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err2 := gzip.NewReader(resp.Body)
		if err2 != nil {
			return nil, false, fmt.Errorf("gzip: %w", err2)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	b, err := io.ReadAll(io.LimitReader(r, 4<<20))
	if err != nil {
		return nil, true, err
	}

	if IsBlocked(resp.StatusCode, b, resp.Header) {
		return nil, false, errs.RateLimited("snapchat: bot-blocked (status %d)", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, errs.NotFound("not found: %s", url)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	return b, false, nil
}

// IsBlocked reports whether the response signals a Cloudflare/CDN bot block.
func IsBlocked(status int, body []byte, h http.Header) bool {
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return true
	}
	if h.Get("cf-mitigated") == "challenge" {
		return true
	}
	s := string(body)
	if strings.Contains(s, "Just a moment") ||
		strings.Contains(s, "window._cf_chl_opt") ||
		strings.Contains(s, "__cf_bm") {
		return true
	}
	if status == http.StatusServiceUnavailable && strings.Contains(s, "Cloudflare") {
		return true
	}
	return false
}

// pace blocks until at least Rate has elapsed since the last request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

// backoff returns the wait duration for a retry attempt.
func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
