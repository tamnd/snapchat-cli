package snapchat_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/tamnd/snapchat-cli/snapchat"
)

const storyHTML = `<!DOCTYPE html>
<html>
<head>
<meta property="og:title" content="nasa's Story" />
<meta property="og:description" content="3 Snaps" />
<meta property="og:image" content="https://cf.sc-cdn.net/img/thumb.jpg" />
</head>
<body><h1>nasa</h1></body>
</html>`

const emptyStoryHTML = `<!DOCTYPE html>
<html><head></head><body></body></html>`

const cloudflareHTML = `<html><head><title>Just a moment</title></head><body></body></html>`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.StoryBase = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return NewClient(cfg), srv
}

func TestGetStory_OK(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(storyHTML))
	})
	defer srv.Close()

	s, err := c.GetStory(context.Background(), "nasa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Username != "nasa" {
		t.Errorf("Username = %q, want nasa", s.Username)
	}
	if s.Title != "nasa's Story" {
		t.Errorf("Title = %q, want nasa's Story", s.Title)
	}
	if s.Thumbnail != "https://cf.sc-cdn.net/img/thumb.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
	if s.MediaCount != 3 {
		t.Errorf("MediaCount = %d, want 3", s.MediaCount)
	}
}

func TestGetStory_AtPrefix(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(storyHTML))
	})
	defer srv.Close()

	s, err := c.GetStory(context.Background(), "@nasa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Username != "nasa" {
		t.Errorf("@ not stripped: Username = %q", s.Username)
	}
}

func TestGetStory_NotFound(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(emptyStoryHTML))
	})
	defer srv.Close()

	_, err := c.GetStory(context.Background(), "doesnotexist")
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected NotFound, got: %v", err)
	}
}

func TestGetStory_Blocked_403(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer srv.Close()

	_, err := c.GetStory(context.Background(), "nasa")
	if err == nil {
		t.Fatal("expected blocked error, got nil")
	}
	if !isRateLimited(err) {
		t.Errorf("expected RateLimited (blocked), got: %v", err)
	}
}

func TestGetStory_Blocked_Cloudflare(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(cloudflareHTML))
	})
	defer srv.Close()

	_, err := c.GetStory(context.Background(), "nasa")
	if err == nil {
		t.Fatal("expected blocked error, got nil")
	}
	if !isRateLimited(err) {
		t.Errorf("expected RateLimited (blocked), got: %v", err)
	}
}

func TestGetStory_EmptyUsername(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClient(cfg)
	_, err := c.GetStory(context.Background(), "")
	if err == nil {
		t.Fatal("expected usage error for empty username")
	}
	if !isUsage(err) {
		t.Errorf("expected Usage error, got: %v", err)
	}
}

func TestParseStory_MediaCount(t *testing.T) {
	body := []byte(`<html><head>
<meta property="og:title" content="test's Story" />
<meta property="og:description" content="5 Snaps" />
<meta property="og:image" content="https://example.com/img.jpg" />
</head></html>`)

	s, err := ParseStory(body, "test")
	if err != nil {
		t.Fatalf("ParseStory: %v", err)
	}
	if s.MediaCount != 5 {
		t.Errorf("MediaCount = %d, want 5", s.MediaCount)
	}
}
