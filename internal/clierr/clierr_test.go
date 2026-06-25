package clierr

import (
	"errors"
	"fmt"
	"testing"
)

func TestCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain", errors.New("boom"), 1},
		{"usage", Usage(errors.New("bad flag")), 2},
		{"not found", NotFound(errors.New("missing")), 3},
		{"auth", Auth(errors.New("no key")), 4},
		{"upstream", Upstream(errors.New("502")), 5},
		{"rate limited", RateLimited(errors.New("429")), 7},
		{"wrapped deeper", fmt.Errorf("context: %w", Auth(errors.New("no key"))), 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Code(c.err); got != c.want {
				t.Errorf("Code(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestWrapNilStaysNil(t *testing.T) {
	if got := Auth(nil); got != nil {
		t.Errorf("Auth(nil) = %v, want nil", got)
	}
}

func TestErrorPreservesMessage(t *testing.T) {
	err := NotFound(errors.New("no book found for \"xyz\""))
	if got, want := err.Error(), "no book found for \"xyz\""; got != want {
		t.Errorf("Error() = %q, want %q (no code suffix)", got, want)
	}
}
