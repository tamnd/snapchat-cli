package snapchat_test

import (
	"context"
	"net/http"
	"testing"
)

const spotlightHTML = `<!DOCTYPE html>
<html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"spotlightVideos":[
  {"snapId":"vid1","title":"Funny Cat","authorUsername":"user1","thumbnailUrl":"https://cdn.snap.com/t1.jpg","viewCount":1000,"durationMs":15000},
  {"snapId":"vid2","title":"Dance Move","authorUsername":"user2","thumbnailUrl":"https://cdn.snap.com/t2.jpg","viewCount":5000,"durationMs":10000}
]}}}
</script>
</body></html>`

func TestSpotlight_OK(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(spotlightHTML))
	})
	defer srv.Close()

	vids, err := c.Spotlight(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vids) != 2 {
		t.Fatalf("got %d videos, want 2", len(vids))
	}
	if vids[0].ID != "vid1" {
		t.Errorf("vids[0].ID = %q, want vid1", vids[0].ID)
	}
	if vids[1].ID != "vid2" {
		t.Errorf("vids[1].ID = %q, want vid2", vids[1].ID)
	}
	if vids[0].Author != "user1" {
		t.Errorf("vids[0].Author = %q, want user1", vids[0].Author)
	}
	if vids[0].DurationS != 15 {
		t.Errorf("vids[0].DurationS = %d, want 15", vids[0].DurationS)
	}
}

func TestSpotlight_Limit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(spotlightHTML))
	})
	defer srv.Close()

	vids, err := c.Spotlight(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vids) != 1 {
		t.Errorf("got %d videos with limit 1, want 1", len(vids))
	}
}

func TestSpotlight_Blocked(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer srv.Close()

	_, err := c.Spotlight(context.Background(), 10)
	if err == nil {
		t.Fatal("expected blocked error, got nil")
	}
	if !isRateLimited(err) {
		t.Errorf("expected RateLimited (blocked), got: %v", err)
	}
}

func TestSpotlight_NoNextData(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body><p>no data</p></body></html>"))
	})
	defer srv.Close()

	vids, err := c.Spotlight(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vids) != 0 {
		t.Errorf("expected empty result, got %d", len(vids))
	}
}
