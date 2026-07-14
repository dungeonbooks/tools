package discover

import (
	"database/sql"
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

func TestCacheMigratesLegacyCentsSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.db")

	// Stand up a cache with the original cents-based schema, with spend already
	// recorded, then reopen it through OpenCache to trigger the migration.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE exa_usage (
	id          INTEGER PRIMARY KEY CHECK (id = 1),
	total_cents INTEGER NOT NULL DEFAULT 0,
	calls       INTEGER NOT NULL DEFAULT 0
);
INSERT INTO exa_usage (id, total_cents, calls) VALUES (1, 6, 3);
`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	c, err := OpenCache(path)
	if err != nil {
		t.Fatalf("open over legacy schema: %v", err)
	}
	defer c.Close()

	dollars, calls, err := c.Usage()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls preserved, got %d", calls)
	}
	if dollars < 0.0599 || dollars > 0.0601 {
		t.Fatalf("expected migrated total ~0.06, got %.4f", dollars)
	}

	// A sub-cent charge now accumulates exactly instead of rounding to zero.
	if err := c.RecordSpend(0.003); err != nil {
		t.Fatal(err)
	}
	dollars, _, _ = c.Usage()
	if dollars < 0.0629 || dollars > 0.0631 {
		t.Fatalf("expected ~0.063 after sub-cent spend, got %.4f", dollars)
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
