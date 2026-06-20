# marty

Marty — the bookseller's wizard, as a Go CLI. Internal tooling for Dungeon Books.

## Commands

| Command | What it does |
|---|---|
| `marty book <title\|isbn>` | Look up a book with rich metadata (cover, description, rating, genres) |
| `marty trending [query]` | Discover trending books from web buzz (Exa-backed, cached) |

An ISBN argument looks the book up directly; anything else is treated as a search phrase.
Add `--json` for machine-readable output, or `--source hardcover|google|openlibrary` to
force a single source (default merges all three).

## Sources

`book` layers three sources, stopping once the important fields are filled:

1. **Hardcover** (`HARDCOVER_API_TOKEN`) — ratings, genre tags, descriptions, new releases.
2. **Google Books** (`GOOGLE_BOOKS_API_KEY`, optional) — broad coverage fallback.
3. **Open Library** (keyless) — last-resort gap fill.

Without a Hardcover token it degrades to Google Books + Open Library.

## Discovery

`trending` surfaces books gaining buzz (BookTok, Reddit, press) via the paid Exa
provider, then resolves missing ISBNs through the metadata pipeline. Set `EXA_API_KEY`
to enable it; without a key `trending` errors rather than guessing.

Results are cached in SQLite (24h TTL) so repeat queries don't re-spend; `--refresh`
bypasses the cache for one call and `--no-cache` disables it. When a paid call happens,
its exact cost (from Exa) prints to stderr alongside the lifetime total. `--no-resolve`
skips ISBN lookups.

An offline `Fake` provider (canned hits, no network, no credits) backs dev and tests. It
is **never** selected automatically — fabricated titles must not pass as real discovery —
so reach it only with `--source fake`, which labels its output as fixture data. `--type`
sets the search mode (`auto` default, `neural`, `keyword`); `--count` caps results.

## Develop

Requires Go 1.24+.

```sh
cp .env.example .env        # set HARDCOVER_API_TOKEN
make hooks                  # enable the pre-commit hook (once per clone)
make check                  # build + vet + gofmt + test (what CI runs)
make run ARGS='book "the will of the many"'
make install                # put the marty binary on your PATH
```

Targets: `build`, `test`, `vet`, `fmt`, `fmt-check`, `tidy`, `run`, `install`, `clean`,
`check`, `hooks`. CI (`.github/workflows/ci.yml`) runs `check` on every push and PR.
