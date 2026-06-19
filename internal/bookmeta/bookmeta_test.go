package bookmeta

import "testing"

func TestPlausibleISBN13(t *testing.T) {
	cases := map[string]bool{
		"9780316595506": true,
		"9791234567890": true,
		"N/A":           false,
		"":              false,
		"0316595500":    false,
		"978031659550X": false,
		"123456789012":  false,
	}
	for in, want := range cases {
		if got := PlausibleISBN13(NormalizeISBN(in)); got != want {
			t.Errorf("PlausibleISBN13(%q) = %v, want %v", in, got, want)
		}
	}
}
