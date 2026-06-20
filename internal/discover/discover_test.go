package discover

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name  string
	on    bool
	cands []Candidate
	err   error
}

func (f fakeProvider) Name() string  { return f.name }
func (f fakeProvider) Enabled() bool { return f.on }
func (f fakeProvider) Trending(_ context.Context, _, _ string, _ int) ([]Candidate, error) {
	return f.cands, f.err
}

func TestTrendingDefaultUsesFirstEnabled(t *testing.T) {
	disabled := fakeProvider{name: SourceExa, on: false}
	on := fakeProvider{name: SourceFake, on: true, cands: []Candidate{{Title: "X"}}}
	svc := NewService(disabled, on)
	cs, err := svc.Trending(context.Background(), "fantasy", "", TypeAuto, 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Title != "X" {
		t.Fatalf("got %+v", cs)
	}
}

func TestTrendingSourceForcesProvider(t *testing.T) {
	hidden := fakeProvider{name: SourceFake, on: false}
	svc := NewService(hidden)
	if _, err := svc.Trending(context.Background(), "", SourceFake, TypeAuto, 5, false); err == nil {
		t.Fatal("expected error when forced source is disabled")
	}

	seen := fakeProvider{name: SourceExa, on: true, cands: []Candidate{{Title: "Y"}}}
	svc = NewService(seen)
	cs, err := svc.Trending(context.Background(), "", SourceExa, TypeNeural, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Title != "Y" {
		t.Fatalf("got %+v", cs)
	}
}

func TestTrendingUnknownSourceErrors(t *testing.T) {
	svc := NewService(NewFake())
	if _, err := svc.Trending(context.Background(), "", "bogus", TypeAuto, 5, false); err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestTrendingNoProviderAvailable(t *testing.T) {
	svc := NewService(fakeProvider{name: SourceExa, on: false, err: errors.New("nope")})
	if _, err := svc.Trending(context.Background(), "", "", TypeAuto, 5, false); err == nil {
		t.Fatal("expected error when no provider enabled")
	}
}

func TestFakeReturnsCannedHitsAndRespectsCount(t *testing.T) {
	f := NewFake()
	cs, err := f.Trending(context.Background(), "anything", TypeAuto, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected 2, got %d", len(cs))
	}
	all, _ := f.Trending(context.Background(), "", TypeAuto, 0)
	if len(all) != 3 {
		t.Fatalf("expected 3 canned hits, got %d", len(all))
	}
	for _, c := range all {
		if c.Title == "" || c.WhyTrending == "" || c.SourceURL == "" {
			t.Fatalf("canned candidate missing fields: %+v", c)
		}
	}
}

func TestServiceCachesResultsAndRefreshBypasses(t *testing.T) {
	calls := 0
	p := countingProvider{name: SourceFake, cands: []Candidate{{Title: "X"}}, n: &calls}
	cache := newTestCache(t)
	defer cache.Close()
	svc := NewService(p).WithCache(cache)

	ctx := context.Background()
	if _, err := svc.Trending(ctx, "q", "", TypeAuto, 5, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Trending(ctx, "q", "", TypeAuto, 5, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 provider call (second served from cache), got %d", calls)
	}
	if _, err := svc.Trending(ctx, "q", "", TypeAuto, 5, true); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 provider calls after --refresh, got %d", calls)
	}
}

type countingProvider struct {
	name  string
	cands []Candidate
	n     *int
}

func (c countingProvider) Name() string  { return c.name }
func (c countingProvider) Enabled() bool { return true }
func (c countingProvider) Trending(_ context.Context, _, _ string, _ int) ([]Candidate, error) {
	*c.n++
	return c.cands, nil
}
