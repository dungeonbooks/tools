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
	// Create tables, then migrate (older DBs need the total_micros column added),
	// then seed the singleton usage row. The seed lists only id so it never
	// references a column the migration is responsible for adding.
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS trending_cache (
	key         TEXT PRIMARY KEY,
	candidates  TEXT NOT NULL,
	created_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS exa_usage (
	id           INTEGER PRIMARY KEY CHECK (id = 1),
	total_micros INTEGER NOT NULL DEFAULT 0,
	calls        INTEGER NOT NULL DEFAULT 0
);
`); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateUsageToMicros(db); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO exa_usage (id) VALUES (1)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

// migrateUsageToMicros upgrades caches created with the original cents-based
// column. Cents rounded every sub-half-cent call to zero, so small Exa charges
// vanished from the lifetime total; micro-dollars hold the real per-call price.
func migrateUsageToMicros(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(exa_usage)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var hasCents, hasMicros bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		switch name {
		case "total_cents":
			hasCents = true
		case "total_micros":
			hasMicros = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasCents {
		return nil
	}
	if !hasMicros {
		if _, err := db.Exec(`ALTER TABLE exa_usage ADD COLUMN total_micros INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE exa_usage SET total_micros = total_cents * 10000 WHERE total_micros = 0`); err != nil {
		return err
	}
	// Dropping the legacy column is cosmetic; ignore failure on SQLite builds
	// without DROP COLUMN support, since the backfilled total is what matters.
	_, _ = db.Exec(`ALTER TABLE exa_usage DROP COLUMN total_cents`)
	return nil
}

func (c *Cache) Close() error { return c.db.Close() }

// cacheKey is the stable tuple a result is cached under. Fields are quoted with
// %q so a delimiter character inside query (or another field) can't shift the
// boundaries and collide with a different tuple.
func cacheKey(source, typ, query string, count int) string {
	return fmt.Sprintf("%q|%q|%q|%d", source, typ, query, count)
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
		// Drop the stale row so the file stays bounded and we don't re-check
		// the same expired entry on every read.
		_, _ = c.db.Exec(`DELETE FROM trending_cache WHERE key = ?`, key)
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

// RecordSpend adds a paid call's cost (dollars) to the running total. Cost is
// stored as micro-dollars so sub-cent charges accumulate exactly.
func (c *Cache) RecordSpend(dollars float64) error {
	micros := int64(dollars*1e6 + 0.5)
	_, err := c.db.Exec(`UPDATE exa_usage SET total_micros = total_micros + ?, calls = calls + 1 WHERE id = 1`, micros)
	return err
}

// Usage reports cumulative Exa spend in dollars and call count.
func (c *Cache) Usage() (dollars float64, calls int, err error) {
	var micros int64
	err = c.db.QueryRow(`SELECT total_micros, calls FROM exa_usage WHERE id = 1`).Scan(&micros, &calls)
	if err != nil {
		return 0, 0, err
	}
	return float64(micros) / 1e6, calls, nil
}
