// Package candidates tracks trending books worth sourcing.
package candidates

import "time"

// IngramStatus is the sourcing verdict. Beyond these it may hold a verbatim
// status line.
type IngramStatus string

const (
	IngramUnknown    IngramStatus = "unknown"
	IngramAvailable  IngramStatus = "available"
	IngramNotCarried IngramStatus = "not_carried"
)

type Status string

const (
	StatusTracked   Status = "tracked"
	StatusDismissed Status = "dismissed"
	StatusPromoted  Status = "promoted"
)

type Candidate struct {
	ID           int64        `json:"id"`
	ISBN13       string       `json:"isbn13,omitempty"`
	Title        string       `json:"title"`
	Author       string       `json:"author,omitempty"`
	Publisher    string       `json:"publisher,omitempty"`
	WhyTrending  string       `json:"why_trending,omitempty"`
	SourceURL    string       `json:"source_url,omitempty"`
	CoverURL     string       `json:"cover_url,omitempty"`
	IngramStatus IngramStatus `json:"ingram_status"`
	Status       Status       `json:"status"`
	DiscoveredAt time.Time    `json:"discovered_at"`
}

// ListFilter narrows a List query. Zero-value fields are ignored.
type ListFilter struct {
	Status       Status
	IngramStatus IngramStatus
}
