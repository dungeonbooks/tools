package bookmeta

import (
	"strings"
	"unicode"
)

type Book struct {
	ISBN13       string   `json:"isbn13,omitempty"`
	Title        string   `json:"title"`
	Author       string   `json:"author,omitempty"`
	Publisher    string   `json:"publisher,omitempty"`
	Description  string   `json:"description,omitempty"`
	CoverURL     string   `json:"cover_url,omitempty"`
	Subjects     []string `json:"subjects,omitempty"`
	Series       string   `json:"series,omitempty"`
	Rating       float64  `json:"rating,omitempty"`
	RatingsCount int      `json:"ratings_count,omitempty"`
	PageCount    int      `json:"page_count,omitempty"`
	Year         int      `json:"year,omitempty"`
	HardcoverURL string   `json:"hardcover_url,omitempty"`
	GoogleURL    string   `json:"google_url,omitempty"`
}

func NormalizeISBN(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

func PlausibleISBN13(s string) bool {
	if len(s) != 13 || (!strings.HasPrefix(s, "978") && !strings.HasPrefix(s, "979")) {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// AuthorsMatch reports whether two author strings plausibly name the same
// person. It requires the surname to agree and the leading given name to match
// or be an initial of the other, tolerating middle names and initials
// ("Isabel J. Kim" ~ "Isabel Kim", "J.R.R. Tolkien" ~ "John Ronald Reuel
// Tolkien"). Either side being empty is treated as no match: a confidence gate
// can only confirm agreement, never assume it. The check assumes "Given Surname"
// ordering, which is how every metadata provider here returns names.
func AuthorsMatch(a, b string) bool {
	at, bt := nameTokens(a), nameTokens(b)
	if len(at) == 0 || len(bt) == 0 {
		return false
	}
	if at[len(at)-1] != bt[len(bt)-1] {
		return false
	}
	return initialMatch(at[0], bt[0])
}

// nameTokens lowercases and splits a name into alphanumeric runs, so "J.R.R."
// becomes ["j","r","r"] and punctuation/spacing differences fall away.
func nameTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// initialMatch treats a single letter as the initial of a longer name sharing
// that first letter ("j" ~ "john"), and otherwise requires equality.
func initialMatch(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) == 1 || len(b) == 1 {
		return a[0] == b[0]
	}
	return false
}

func (dst *Book) Fill(src Book) {
	if dst.ISBN13 == "" {
		dst.ISBN13 = src.ISBN13
	}
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Author == "" {
		dst.Author = src.Author
	}
	if dst.Publisher == "" {
		dst.Publisher = src.Publisher
	}
	if dst.Description == "" {
		dst.Description = src.Description
	}
	if dst.CoverURL == "" {
		dst.CoverURL = src.CoverURL
	}
	if len(dst.Subjects) == 0 {
		dst.Subjects = src.Subjects
	}
	if dst.Series == "" {
		dst.Series = src.Series
	}
	if dst.Rating == 0 {
		dst.Rating = src.Rating
		dst.RatingsCount = src.RatingsCount
	}
	if dst.PageCount == 0 {
		dst.PageCount = src.PageCount
	}
	if dst.Year == 0 {
		dst.Year = src.Year
	}
	if dst.HardcoverURL == "" {
		dst.HardcoverURL = src.HardcoverURL
	}
	if dst.GoogleURL == "" {
		dst.GoogleURL = src.GoogleURL
	}
}
