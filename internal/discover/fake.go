package discover

import (
	"context"
	"time"
)

// Fake is an offline discovery provider. It returns canned BookTok/Reddit/press
// hits so the verb, rendering, and data shape can be exercised without Exa
// credits or a network.
type Fake struct{}

func NewFake() *Fake { return &Fake{} }

func (f *Fake) Name() string  { return SourceFake }
func (f *Fake) Enabled() bool { return true }
func (f *Fake) Trending(_ context.Context, _ string, _ string, count int) ([]Candidate, error) {
	now := time.Now()
	base := []Candidate{
		{
			Title:        "The Hedge Witch of Bree",
			Author:       "Mara Vance",
			WhyTrending:  "BookTok callout: \"$3 kindle find that punched above its weight\"",
			SourceURL:    "https://www.tiktok.com/@booktok/example",
			DiscoveredAt: now,
		},
		{
			Title:        "Salt and Iron",
			Author:       "Devon Reyes",
			WhyTrending:  "r/fantasy thread: \"underrated 2026 debut, nobody's talking about this\"",
			SourceURL:    "https://www.reddit.com/r/fantasy/example",
			DiscoveredAt: now,
		},
		{
			Title:        "The Cartographer's Daughter",
			Author:       "Imogen Hart",
			WhyTrending:  "Tor.com roundup: 2026 SFF debuts to watch",
			SourceURL:    "https://www.tor.com/2026/example",
			DiscoveredAt: now,
		},
	}
	if count > 0 && count < len(base) {
		return base[:count], nil
	}
	return base, nil
}
