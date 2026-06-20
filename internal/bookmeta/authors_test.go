package bookmeta

import "testing"

func TestAuthorsMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Isabel J. Kim", "Isabel Kim", true},                 // dropped middle initial
		{"Isabel Kim", "Isabel J. Kim", true},                 // order of args doesn't matter
		{"J.R.R. Tolkien", "John Ronald Reuel Tolkien", true}, // initials vs full given names
		{"Adrian Tchaikovsky", "adrian tchaikovsky", true},    // case-insensitive
		{"Émile Zola", "É. Zola", true},                       // non-ASCII initial (rune-compared)
		{"Émile Zola", "Albert Zola", false},                  // non-ASCII given name mismatch
		{"Emily Henry", "O. Henry", false},                    // same surname, different person
		{"Mara Vance", "Devon Reyes", false},                  // unrelated
		{"Isabel Kim", "Isabel Cho", false},                   // shared given name, different surname
		{"Isabel J. Kim", "", false},                          // empty side never matches
		{"", "Isabel J. Kim", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := AuthorsMatch(c.a, c.b); got != c.want {
			t.Errorf("AuthorsMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
