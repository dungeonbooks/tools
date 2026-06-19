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
