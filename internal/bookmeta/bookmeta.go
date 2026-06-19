package bookmeta

import "strings"

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
