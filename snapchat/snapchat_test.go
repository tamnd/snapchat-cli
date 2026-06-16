package snapchat_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/any-cli/kit/errs"
	. "github.com/tamnd/snapchat-cli/snapchat"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL == "" {
		t.Error("BaseURL is empty")
	}
	if cfg.StoryBase == "" {
		t.Error("StoryBase is empty")
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
	if cfg.Rate <= 0 {
		t.Error("Rate must be > 0")
	}
	if cfg.Retries <= 0 {
		t.Error("Retries must be > 0")
	}
	if cfg.Timeout <= 0 {
		t.Error("Timeout must be > 0")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient(DefaultConfig())
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestIsBlocked_403(t *testing.T) {
	if !IsBlocked(403, []byte("forbidden"), http.Header{}) {
		t.Error("expected blocked on 403")
	}
}

func TestIsBlocked_429(t *testing.T) {
	if !IsBlocked(429, []byte("rate limited"), http.Header{}) {
		t.Error("expected blocked on 429")
	}
}

func TestIsBlocked_CloudflareChallenge(t *testing.T) {
	body := []byte("<title>Just a moment</title>")
	if !IsBlocked(200, body, http.Header{}) {
		t.Error("expected blocked on Cloudflare challenge body")
	}
}

func TestIsBlocked_CFOptVar(t *testing.T) {
	body := []byte("window._cf_chl_opt = {}")
	if !IsBlocked(200, body, http.Header{}) {
		t.Error("expected blocked on _cf_chl_opt body")
	}
}

func TestIsBlocked_Normal(t *testing.T) {
	body := []byte(`<html><head><meta property="og:title" content="snap's Story"/></head></html>`)
	if IsBlocked(200, body, http.Header{}) {
		t.Error("expected NOT blocked on normal story body")
	}
}

func TestIsBlocked_CFMitigatedHeader(t *testing.T) {
	h := http.Header{}
	h.Set("cf-mitigated", "challenge")
	if !IsBlocked(200, []byte("ok"), h) {
		t.Error("expected blocked when cf-mitigated: challenge header is set")
	}
}

func TestGetSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent header")
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.StoryBase = srv.URL
	cfg.Rate = 0

	c := NewClient(cfg)
	body, err := c.Get(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want hello", body)
	}
}

// isRateLimited checks for errs.KindRateLimited (the "blocked" exit-5 kind).
func isRateLimited(err error) bool {
	return errs.KindOf(err) == errs.KindRateLimited
}

// isNotFound checks for errs.KindNotFound.
func isNotFound(err error) bool {
	return errs.KindOf(err) == errs.KindNotFound
}

// isUsage checks for errs.KindUsage.
func isUsage(err error) bool {
	return errs.KindOf(err) == errs.KindUsage
}
