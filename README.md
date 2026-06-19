# marty

Marty — the bookseller's wizard, as a Go CLI. Internal tooling for Dungeon Books.

## Commands

| Command | What it does |
|---|---|
| `marty book <title\|isbn>` | Look up a book with rich metadata (cover, description, rating, genres) |

An ISBN argument looks the book up directly; anything else is treated as a search phrase.
Add `--json` for machine-readable output.

## Sources

`book` layers three sources, stopping once the important fields are filled:

1. **Hardcover** (`HARDCOVER_API_TOKEN`) — ratings, genre tags, descriptions, new releases.
2. **Google Books** (`GOOGLE_BOOKS_API_KEY`, optional) — broad coverage fallback.
3. **Open Library** (keyless) — last-resort gap fill.

Without a Hardcover token it degrades to Google Books + Open Library.

## Develop

Requires Go 1.24+.

```sh
cp .env.example .env        # set HARDCOVER_API_TOKEN
go build ./...
go test ./...
go run ./cmd/marty book "the will of the many"
go run ./cmd/marty book 9780593128282 --json
```
