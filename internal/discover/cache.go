package discover

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const (
	cacheTTL = 24 * time.Hour
)

// Cache is a local SQLite store for paid discovery results, keyed by the
// resolved query/source/type/count tuple. It also tracks cumulative Exa spend
// so the ~$20 credit burn is visible. Third-party API data is cached only for
// internal staff use (terms forbid public serving from these caches).
type Cache struct {
	db *sql.DB
}

func OpenCache(path string) (*Cache, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS trending_cache (
	key         TEXT PRIMARY KEY,
	candidates  TEXT NOT NULL,
	created_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS exa_usage (
	id          INTEGER PRIMARY KEY CHECK (id = 1),
	total_cents INTEGER NOT NULL DEFAULT 0,
	calls       INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO exa_usage (id, total_cents, calls) VALUES (1, 0, 0);
`); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

func (c *Cache) Close() error { return c.db.Close() }

// cacheKey is the stable tuple a result is cached under.
func cacheKey(source, typ, query string, count int) string {
	return fmt.Sprintf("%s|%s|%s|%d", source, typ, query, count)
}

// Get returns unexpired cached candidates for the key, or (nil,false) on miss.
func (c *Cache) Get(key string) ([]Candidate, bool, error) {
	var raw string
	var createdAt int64
	err := c.db.QueryRow(`SELECT candidates, created_at FROM trending_cache WHERE key = ?`, key).Scan(&raw, &createdAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Since(time.Unix(createdAt, 0)) > cacheTTL {
		return nil, false, nil
	}
	var cs []Candidate
	if err := json.Unmarshal([]byte(raw), &cs); err != nil {
		return nil, false, err
	}
	return cs, true, nil
}

func (c *Cache) Put(key string, cs []Candidate) error {
	raw, err := json.Marshal(cs)
	if err != nil {
		return err
	}
	_, err = c.db.Exec(
		`INSERT OR REPLACE INTO trending_cache (key, candidates, created_at) VALUES (?, ?, ?)`,
		key, string(raw), time.Now().Unix(),
	)
	return err
}

// RecordSpend adds a paid call's cost (dollars) to the running total.
func (c *Cache) RecordSpend(dollars float64) error {
	cents := int(dollars*100 + 0.5)
	_, err := c.db.Exec(`UPDATE exa_usage SET total_cents = total_cents + ?, calls = calls + 1 WHERE id = 1`, cents)
	return err
}

// Usage reports cumulative Exa spend in dollars and call count.
func (c *Cache) Usage() (dollars float64, calls int, err error) {
	var cents int
	err = c.db.QueryRow(`SELECT total_cents, calls FROM exa_usage WHERE id = 1`).Scan(&cents, &calls)
	if err != nil {
		return 0, 0, err
	}
	return float64(cents) / 100.0, calls, nil
}
