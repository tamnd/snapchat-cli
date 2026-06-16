package snapchat

import (
	"context"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Snapchat driver.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "snapchat",
		Hosts:  []string{Host, "story.snapchat.com"},
		Identity: kit.Identity{
			Binary: "snap",
			Short:  "Read public Snapchat stories and Spotlight data",
			Long: `snap reads public Snapchat data over plain HTTPS.

Fetch story metadata for any public creator, or list trending Spotlight videos.
No API key required.

Note: Snapchat is Tier C (anti-bot). Requests from datacenter IPs are usually
blocked by Cloudflare. The CLI exits with code 5 when a block is detected.

Quick start:
  snap story nasa           NASA's public story metadata
  snap spotlight            trending Spotlight videos
  snap spotlight -n 10      top 10 Spotlight videos
  snap story snapchat -o json`,
			Site: Host,
			Repo: "https://github.com/tamnd/snapchat-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "story",
		Group:   "stories",
		Single:  true,
		Summary: "Fetch a public story by creator username",
		Args:    []kit.Arg{{Name: "username", Help: "creator @handle (e.g. nasa)"}},
	}, getStory)

	kit.Handle(app, kit.OpMeta{
		Name:    "spotlight",
		Group:   "discover",
		List:    true,
		Summary: "List trending Spotlight videos",
	}, listSpotlight)
}

// newClient builds the Snapchat client from the kit-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	dcfg := DefaultConfig()
	if cfg.UserAgent != "" {
		dcfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		dcfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		dcfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		dcfg.Timeout = cfg.Timeout
	}
	return NewClient(dcfg), nil
}

// --- input structs ---

type storyInput struct {
	Username string  `kit:"arg"          help:"creator @handle"`
	Client   *Client `kit:"inject"`
}

type spotlightInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getStory(ctx context.Context, in storyInput, emit func(*Story) error) error {
	s, err := in.Client.GetStory(ctx, in.Username)
	if err != nil {
		return mapErr(err)
	}
	return emit(s)
}

func listSpotlight(ctx context.Context, in spotlightInput, emit func(*SpotlightVideo) error) error {
	vids, err := in.Client.Spotlight(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, v := range vids {
		if err := emit(v); err != nil {
			return err
		}
	}
	return nil
}

// Classify implements the URI resolver interface.
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("snapchat reference is empty")
	}
	return "story", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "story" {
		return "", errs.Usage("snapchat has no resource type %q", uriType)
	}
	return StoryBase + "/@" + id, nil
}

// mapErr passes errors through; kit's typed error system handles exit codes.
func mapErr(err error) error {
	return err
}
