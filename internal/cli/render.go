package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/discover"
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
		fmt.Fprintln(w, b.ISBN13)
	}
	if b.Description != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, paragraphs(b.Description, 80))
	}
	var links []string
	if b.HardcoverURL != "" {
		links = append(links, "Hardcover: "+b.HardcoverURL)
	}
	if b.GoogleURL != "" {
		links = append(links, "Google Books: "+b.GoogleURL)
	}
	if len(links) > 0 {
		fmt.Fprintln(w)
		for _, l := range links {
			fmt.Fprintln(w, l)
		}
	}
	return nil
}

func trim(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func renderTrending(w io.Writer, cs []discover.Candidate, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}
	if len(cs) == 0 {
		return nil
	}
	for i, c := range cs {
		head := c.Title
		if c.Author != "" {
			head += " — " + c.Author
		}
		fmt.Fprintf(w, "%d. %s\n", i+1, head)
		if c.WhyTrending != "" {
			fmt.Fprintln(w, indent(wrapLine(c.WhyTrending, 76), "   "))
		}
		if c.ISBN13 != "" {
			fmt.Fprintln(w, "   "+c.ISBN13)
		}
		if c.SourceURL != "" {
			fmt.Fprintln(w, "   "+c.SourceURL)
		}
		if i < len(cs)-1 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

var blankLine = regexp.MustCompile(`\n\s*\n`)

func paragraphs(s string, width int) string {
	var out []string
	for _, p := range blankLine.Split(s, -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, wrapLine(p, width))
		}
	}
	return strings.Join(out, "\n\n")
}

// indent prefixes every line of s, so wrapped continuation lines keep their
// alignment under the numbered entry instead of falling back to column 0.
func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
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
