package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dungeonbooks/tools/internal/clierr"
	"github.com/dungeonbooks/tools/internal/resolve"
)

// The exit code is the contract this command is built around, so it is pinned
// here. 3 and 5 must never collapse into each other: "no catalogue has this
// book" and "the provider fell over" are opposite facts, and an agent that
// cannot tell them apart will report an outage as if the book did not exist.
func TestResolveExitCodes(t *testing.T) {
	tests := []struct {
		name string
		r    resolve.Result
		want int
	}{
		{
			name: "verified",
			r:    resolve.Result{Verified: true, Confidence: 1, ISBN13: "9780262376303", Title: "The Beauty of Games"},
			want: 0,
		},
		{
			name: "weak match — the book was not identified",
			r:    resolve.Result{Reason: "weak title/author match (confidence 0.41 < 0.60)", ISBN13: "9780743482769", Title: "King Lear"},
			want: 3,
		},
		{
			name: "no catalogue carries it",
			r:    resolve.Result{Reason: "no source returned a result with an ISBN"},
			want: 3,
		},
		{
			name: "provider outage — says nothing about the book",
			r:    resolve.Result{Reason: "the lookup failed: 3 of 3 sources errored", Retryable: true},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clierr.Code(resolveErr(tt.r)); got != tt.want {
				t.Errorf("exit code = %d, want %d", got, tt.want)
			}
		})
	}
}

// A rejected ISBN must never reach stdout, where a caller reading the command's
// output would take it for the answer. It belongs in the error, which goes to
// stderr and carries an explicit warning.
func TestRenderResolvedKeepsARejectedISBNOffStdout(t *testing.T) {
	rejected := resolve.Result{
		Query:  "It Stephen King",
		Reason: "weak title/author match (confidence 0.25 < 0.60)",
		ISBN13: "9781501110368",
		Title:  "It Ends With Us",
		Author: "Colleen Hoover",
	}

	var out bytes.Buffer
	if err := renderResolved(&out, rejected, false); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty: an unverified ISBN must not be printed as a result", out.String())
	}

	err := resolveErr(rejected)
	if !strings.Contains(err.Error(), "9781501110368") {
		t.Errorf("error does not surface the rejected candidate: %q", err)
	}
	if !strings.Contains(err.Error(), "do not use it") {
		t.Errorf("error does not warn the caller off the guess: %q", err)
	}
}

// With --json the payload still goes to stdout on a failure — a caller asking
// for JSON wants the rejected candidate and its reason — but the exit code
// still reports the failure.
func TestRenderResolvedJSONEmitsPayloadEvenWhenUnverified(t *testing.T) {
	r := resolve.Result{Query: "x", Reason: "no source returned a result with an ISBN"}

	var out bytes.Buffer
	if err := renderResolved(&out, r, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"verified": false`) {
		t.Errorf("JSON payload missing the verdict: %s", out.String())
	}
	if code := clierr.Code(resolveErr(r)); code != 3 {
		t.Errorf("exit code = %d, want 3 even though JSON was written", code)
	}
}

func TestRenderResolvedVerifiedGoesToStdout(t *testing.T) {
	r := resolve.Result{
		Verified: true, Confidence: 1,
		ISBN13: "9780262376303", Title: "The Beauty of Games", Author: "Frank Lantz", Year: 2023,
	}

	var out bytes.Buffer
	if err := renderResolved(&out, r, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"9780262376303", "The Beauty of Games", "Frank Lantz", "2023", "1.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout = %q, missing %q", got, want)
		}
	}
}
