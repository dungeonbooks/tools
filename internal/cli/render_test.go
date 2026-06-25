package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/discover"
)

// update rewrites the golden fixtures instead of comparing against them:
//
//	go test ./internal/cli -run TestRender -update
var update = flag.Bool("update", false, "rewrite golden files")

func sampleBook() bookmeta.Book {
	return bookmeta.Book{
		ISBN13:       "9780765326355",
		Title:        "The Way of Kings",
		Author:       "Brandon Sanderson",
		Description:  "Roshar is a world of stone and storms.\n\nKaladin must lead men through war.",
		Subjects:     []string{"Epic fantasy", "Magic", "War", "Adventure"},
		Series:       "The Stormlight Archive",
		Rating:       4.6,
		RatingsCount: 312045,
		PageCount:    1007,
		Year:         2010,
		HardcoverURL: "https://hardcover.app/books/the-way-of-kings",
		GoogleURL:    "https://books.google.com/books?id=way-of-kings",
	}
}

func sampleCandidates() []discover.Candidate {
	return []discover.Candidate{
		{
			Title:       "The Will of the Many",
			Author:      "James Islington",
			ISBN13:      "9781982141172",
			WhyTrending: "Surging on BookTok for its hard-magic academy setting and twist-heavy plotting that readers keep reposting.",
			SourceURL:   "https://example.com/will-of-the-many",
		},
		{
			Title:  "A Sorceress Comes to Call",
			Author: "T. Kingfisher",
		},
	}
}

func TestRenderBook(t *testing.T) {
	t.Run("human", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderBook(&buf, sampleBook(), false); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "book_human.txt", buf.Bytes())
	})
	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderBook(&buf, sampleBook(), true); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "book.json", buf.Bytes())
	})
}

func TestRenderTrending(t *testing.T) {
	t.Run("human_exa", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderTrending(&buf, sampleCandidates(), discover.SourceExa, false); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "trending_exa_human.txt", buf.Bytes())
	})
	t.Run("human_fake_labels_fixture", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderTrending(&buf, sampleCandidates(), discover.SourceFake, false); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "trending_fake_human.txt", buf.Bytes())
	})
	t.Run("json_carries_source", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderTrending(&buf, sampleCandidates(), discover.SourceFake, true); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "trending_fake.json", buf.Bytes())
	})
	t.Run("json_empty_is_array_not_null", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderTrending(&buf, nil, discover.SourceExa, true); err != nil {
			t.Fatal(err)
		}
		assertGolden(t, "trending_empty.json", buf.Bytes())
	})
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./internal/cli -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
