package candidates

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("candidate not found")

// Store is an in-memory candidate store.
type Store struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]Candidate
}

func NewStore() *Store {
	return &Store{nextID: 1, byID: map[int64]Candidate{}}
}

// Upsert inserts a candidate, or updates an existing one with the same ISBN-13.
func (s *Store) Upsert(c Candidate) Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ISBN13 != "" {
		for id, existing := range s.byID {
			if existing.ISBN13 == c.ISBN13 {
				c.ID, c.DiscoveredAt = id, existing.DiscoveredAt
				s.byID[id] = c
				return c
			}
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
