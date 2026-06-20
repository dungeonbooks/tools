package discover

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCachePutGetRoundTrip(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	cs := []Candidate{{Title: "A", Author: "Z", WhyTrending: "buzz", SourceURL: "u"}}
	key := cacheKey(SourceFake, TypeAuto, "q", 5)
	if hit, _, err := c.Get(key); err != nil || hit != nil {
		t.Fatalf("expected miss on empty cache, got hit=%v err=%v", hit, err)
	}
	if err := c.Put(key, cs); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(key)
	if err != nil || !ok {
		t.Fatalf("expected hit, ok=%v err=%v", ok, err)
	}
	if len(got) != 1 || got[0].Title != "A" {
		t.Fatalf("bad round-trip: %+v", got)
	}
}

func TestCacheRecordSpendAccumulates(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()

	// Fake results are free; Exa records spend.
	for _, d := range []float64{0.007, 0.05, 0.003} {
		if err := c.RecordSpend(d); err != nil {
			t.Fatal(err)
		}
	}
	dollars, calls, err := c.Usage()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if dollars < 0.059 || dollars > 0.061 {
		t.Fatalf("expected ~0.06, got %.4f", dollars)
	}
}

func TestCacheRejectsExpiredEntries(t *testing.T) {
	c := newTestCache(t)
	defer c.Close()
	cs := []Candidate{{Title: "old"}}
	key := cacheKey(SourceExa, TypeAuto, "q", 5)
	if err := c.Put(key, cs); err != nil {
		t.Fatal(err)
	}
	// Backdate the entry past the TTL.
	if _, err := c.db.Exec(`UPDATE trending_cache SET created_at = ? WHERE key = ?`,
		time.Now().Add(-cacheTTL-time.Hour).Unix(), key); err != nil {
		t.Fatal(err)
	}
	if hit, ok, err := c.Get(key); err != nil || ok || hit != nil {
		t.Fatalf("expected expired miss, ok=%v err=%v", ok, err)
	}
	// The expired row should be purged on read so the file stays bounded.
	var n int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM trending_cache WHERE key = ?`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected expired row to be deleted, found %d", n)
	}
}

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
