// Package bookmeta holds the shared book value type.
package bookmeta

import "strings"

type Book struct {
	ISBN13      string `json:"isbn13,omitempty"`
	Title       string `json:"title"`
	Author      string `json:"author,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	WhyTrending string `json:"why_trending,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	CoverURL    string `json:"cover_url,omitempty"`
}

func NormalizeISBN(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
}

// PlausibleISBN13 reports whether s is 13 digits with a 978/979 prefix.
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
