package candidates

import "testing"

func TestUpsertIsIdempotentByISBN(t *testing.T) {
	s := NewStore()
	first := s.Upsert(Candidate{ISBN13: "9781250799609", Title: "Sublimation"})
	if first.Status != StatusTracked || first.IngramStatus != IngramUnknown {
		t.Fatalf("defaults not applied: %+v", first)
	}
	second := s.Upsert(Candidate{ISBN13: "9781250799609", Title: "Sublimation (rev)"})
	if second.ID != first.ID {
		t.Fatalf("expected same id, got %d then %d", first.ID, second.ID)
	}
	if got := s.List(ListFilter{}); len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
}

func TestUpsertDedupesISBNlessByTitleAuthor(t *testing.T) {
	s := NewStore()
	a := s.Upsert(Candidate{Title: "Green City Wars", Author: "Adrian Tchaikovsky"})
	_ = s.SetStatus(a.ID, StatusDismissed)
	b := s.Upsert(Candidate{Title: "green city  wars", Author: "Adrian Tchaikovsky"})
	if b.ID != a.ID {
		t.Fatalf("expected same id for ISBN-less re-track, got %d then %d", a.ID, b.ID)
	}
	if b.Status != StatusDismissed {
		t.Fatalf("expected lifecycle preserved (dismissed), got %q", b.Status)
	}
	if got := s.List(ListFilter{}); len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}
}
