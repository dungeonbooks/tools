package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

func renderBook(w io.Writer, b bookmeta.Book, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(b)
	}

	title := b.Title
	if b.Author != "" {
		title += " by " + b.Author
	}
	if b.Year > 0 {
		title += fmt.Sprintf(" (%d)", b.Year)
	}
	fmt.Fprintln(w, title)

	var facts []string
	if b.Rating > 0 {
		facts = append(facts, fmt.Sprintf("★ %.1f (%d)", b.Rating, b.RatingsCount))
	}
	if b.PageCount > 0 {
		facts = append(facts, fmt.Sprintf("%d pages", b.PageCount))
	}
	if len(b.Subjects) > 0 {
		facts = append(facts, strings.Join(trim(b.Subjects, 3), ", "))
	}
	if len(facts) > 0 {
		fmt.Fprintln(w, strings.Join(facts, " · "))
	}
	if b.Series != "" {
		fmt.Fprintln(w, "Series: "+b.Series)
	}
	if b.ISBN13 != "" {
		fmt.Fprintln(w, "ISBN "+b.ISBN13)
	}
	if b.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, paragraphs(b.Description, 80))
	}
	if b.HardcoverURL != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Hardcover: "+b.HardcoverURL)
	}
	return nil
}

func trim(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// paragraphs wraps each paragraph to width, preserving blank lines between them.
func paragraphs(s string, width int) string {
	var out []string
	for _, p := range strings.Split(s, "\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, wrapLine(p, width))
		}
	}
	return strings.Join(out, "\n\n")
}

func wrapLine(s string, width int) string {
	var out strings.Builder
	line := 0
	for i, word := range strings.Fields(s) {
		if i > 0 {
			if line+1+len(word) > width {
				out.WriteByte('\n')
				line = 0
			} else {
				out.WriteByte(' ')
				line++
			}
		}
		out.WriteString(word)
		line += len(word)
	}
	return out.String()
}
