package snapchat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// SpotlightVideo is one trending Spotlight video record.
type SpotlightVideo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	ViewCount int64  `json:"view_count"`
	DurationS int    `json:"duration_s"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
}

var nextDataRE = regexp.MustCompile(
	`<script\s+id="__NEXT_DATA__"\s+type="application/json">([^<]+)</script>`)

// Spotlight fetches trending Spotlight videos. Returns up to limit records.
func (c *Client) Spotlight(ctx context.Context, limit int) ([]*SpotlightVideo, error) {
	url := c.cfg.BaseURL + "/spotlight"
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	vids, err := parseSpotlight(body)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(vids) > limit {
		vids = vids[:limit]
	}
	return vids, nil
}

// parseSpotlight extracts SpotlightVideo records from the __NEXT_DATA__ blob.
func parseSpotlight(body []byte) ([]*SpotlightVideo, error) {
	m := nextDataRE.FindSubmatch(body)
	if m == nil {
		// Not a hard error: Spotlight may be gated or missing on this IP.
		return nil, nil
	}

	var root map[string]any
	if err := json.Unmarshal(m[1], &root); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}

	items := spotlightItems(root)
	var out []*SpotlightVideo
	for _, it := range items {
		v := parseSpotlightItem(it)
		if v != nil && v.ID != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

// spotlightItems navigates the __NEXT_DATA__ tree to find video items.
func spotlightItems(root map[string]any) []map[string]any {
	paths := [][]string{
		{"props", "pageProps", "spotlightVideos"},
		{"props", "pageProps", "content"},
		{"props", "pageProps", "initialData", "spotlightVideos"},
	}
	for _, path := range paths {
		node := walkAny(root, path...)
		if arr, ok := node.([]any); ok && len(arr) > 0 {
			var out []map[string]any
			for _, el := range arr {
				if m2, ok := el.(map[string]any); ok {
					out = append(out, m2)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func parseSpotlightItem(raw map[string]any) *SpotlightVideo {
	id := anyStr(raw, "snapId", "id", "videoId")
	title := anyStr(raw, "title", "caption", "description")
	author := anyStr(raw, "authorUsername", "username", "creator")
	thumb := anyStr(raw, "thumbnailUrl", "thumbnail", "coverUrl")
	viewCount := anyInt64(raw, "viewCount", "views")
	dur := anyInt(raw, "durationMs", "duration")
	if dur > 1000 {
		dur = dur / 1000 // convert ms to s
	}
	url := ""
	if id != "" {
		url = "https://www.snapchat.com/spotlight/" + id
	}
	return &SpotlightVideo{
		ID:        id,
		Title:     title,
		Author:    author,
		ViewCount: viewCount,
		DurationS: dur,
		Thumbnail: thumb,
		URL:       url,
	}
}

// walkAny traverses a nested map[string]any by successive string keys.
func walkAny(node any, keys ...string) any {
	for _, k := range keys {
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[k]
	}
	return node
}

// anyStr returns the first non-empty string value for keys in raw.
func anyStr(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// anyInt64 returns the first numeric value for keys in raw.
func anyInt64(raw map[string]any, keys ...string) int64 {
	for _, k := range keys {
		switch v := raw[k].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		}
	}
	return 0
}

// anyInt returns the first numeric value for keys in raw as int.
func anyInt(raw map[string]any, keys ...string) int {
	return int(anyInt64(raw, keys...))
}
