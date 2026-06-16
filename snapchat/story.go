package snapchat

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit/errs"
)

// Story is one public Snapchat story record.
type Story struct {
	Username    string `json:"username"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	MediaCount  int    `json:"media_count"`
	URL         string `json:"url"`
}

var (
	ogTitleRE    = regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]*)"`)
	ogTitleRE2   = regexp.MustCompile(`<meta\s+content="([^"]*)"\s+property="og:title"`)
	ogDescRE     = regexp.MustCompile(`<meta\s+property="og:description"\s+content="([^"]*)"`)
	ogDescRE2    = regexp.MustCompile(`<meta\s+content="([^"]*)"\s+property="og:description"`)
	ogImageRE    = regexp.MustCompile(`<meta\s+property="og:image"\s+content="([^"]*)"`)
	ogImageRE2   = regexp.MustCompile(`<meta\s+content="([^"]*)"\s+property="og:image"`)
	snapCountRE  = regexp.MustCompile(`(\d+)\s+[Ss]nap`)
)

// GetStory fetches the public story metadata for username.
func (c *Client) GetStory(ctx context.Context, username string) (*Story, error) {
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	if username == "" {
		return nil, errs.Usage("username is required")
	}
	url := c.cfg.StoryBase + "/@" + username
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	s, err := parseStory(body, username)
	if err != nil {
		return nil, err
	}
	s.URL = fmt.Sprintf("%s/@%s", c.cfg.StoryBase, username)
	return s, nil
}

// ParseStory extracts Story fields from the og: meta tags in body (exported for tests).
func ParseStory(body []byte, username string) (*Story, error) {
	return parseStory(body, username)
}

// parseStory extracts Story fields from the og: meta tags in body.
func parseStory(body []byte, username string) (*Story, error) {
	title := ogMatch(body, ogTitleRE, ogTitleRE2)
	if strings.TrimSpace(title) == "" {
		return nil, errs.NotFound("story not found: @%s", username)
	}
	desc := ogMatch(body, ogDescRE, ogDescRE2)
	thumb := ogMatch(body, ogImageRE, ogImageRE2)

	count := 0
	if m := snapCountRE.FindSubmatch(body); m != nil {
		if n, err := strconv.Atoi(string(m[1])); err == nil {
			count = n
		}
	}

	return &Story{
		Username:    username,
		Title:       title,
		Description: desc,
		Thumbnail:   thumb,
		MediaCount:  count,
	}, nil
}

// ogMatch tries primary regex then fallback for attribute-order variants.
func ogMatch(body []byte, primary, fallback *regexp.Regexp) string {
	if m := primary.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	if m := fallback.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return ""
}
