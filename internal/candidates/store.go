package candidates

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("candidate not found")

// dedupKey identifies a candidate by ISBN-13, or by normalized title+author
// when no ISBN has been resolved, so re-discovery never duplicates a book.
func dedupKey(c Candidate) string {
	if c.ISBN13 != "" {
		return "isbn:" + c.ISBN13
	}
	return "ta:" + strings.ToLower(strings.Join(strings.Fields(c.Title+" "+c.Author), " "))
}

// Store is an in-memory candidate store.
type Store struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]Candidate
}

func NewStore() *Store {
	return &Store{nextID: 1, byID: map[int64]Candidate{}}
}

// Upsert inserts a candidate, or refreshes an existing one with the same dedup
// key, preserving its id, discovery time, and lifecycle (status, Ingram verdict).
func (s *Store) Upsert(c Candidate) Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := dedupKey(c)
	for id, existing := range s.byID {
		if dedupKey(existing) == k {
			c.ID = id
			c.DiscoveredAt = existing.DiscoveredAt
			c.Status = existing.Status
			c.IngramStatus = existing.IngramStatus
			s.byID[id] = c
			return c
		}
	}
	c.ID = s.nextID
	s.nextID++
	c.DiscoveredAt = time.Now().UTC()
	if c.Status == "" {
		c.Status = StatusTracked
	}
	if c.IngramStatus == "" {
		c.IngramStatus = IngramUnknown
	}
	s.byID[c.ID] = c
	return c
}

func (s *Store) List(f ListFilter) []Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Candidate, 0, len(s.byID))
	for _, c := range s.byID {
		if f.Status != "" && c.Status != f.Status {
			continue
		}
		if f.IngramStatus != "" && c.IngramStatus != f.IngramStatus {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DiscoveredAt.After(out[j].DiscoveredAt) })
	return out
}

func (s *Store) SetStatus(id int64, st Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byID[id]
	if !ok {
		return ErrNotFound
	}
	c.Status = st
	s.byID[id] = c
	return nil
}
