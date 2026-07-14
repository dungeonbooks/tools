# marty

Marty — the bookseller's wizard, as a Go CLI. Internal tooling for Dungeon Books.

## Commands

| Command | What it does |
|---|---|
| `marty book <title\|isbn>` | Look up a book with rich metadata (cover, description, rating, genres) |
| `marty resolve <title>` | Resolve a title to a **verified** ISBN-13 |
| `marty trending [query]` | Discover trending books from web buzz (Exa-backed, cached) |
| `marty mcp` | Serve book lookup as MCP tools over stdio (for agents) |

An ISBN argument looks the book up directly; anything else is treated as a search phrase.
Add `--json` for machine-readable output, or `--source hardcover|google|openlibrary` to
force a single source (default merges all three).

## Resolving

`book` returns whatever the sources rank first, which is occasionally the wrong book.
`resolve` adds the verification you'd otherwise have to do by hand, and is what you want
whenever an ISBN is going to be written down somewhere.

```sh
marty resolve "The Beauty of Games" --author "Frank Lantz"
# 9780262376303  The Beauty of Games — Frank Lantz (2023)  [confidence 1.00]

marty resolve --isbn 9780262542951        # verify an ISBN you were handed
```

**Always pass `--author`.** A bare title mismatches badly: "Playing at the World" on its
own resolves to *King Lear*, "Unboxed" to a Dr. Seuss box set.

The exit code is the contract: `0` is a verified match, `1` means nothing cleared the
confidence floor. On an exit-1 the ISBN, if any, is a rejected best guess — never use it.
Some books are in none of the sources, and unverified is the correct answer for those.

A failed lookup is reported separately (`LOOKUP FAILED`, and `retryable: true` in JSON).
A rate-limited provider and a book that no catalogue carries both come back empty, but
"retry" and "this book has no ISBN" are opposite instructions — so they are never
conflated. If you see `retryable`, call again before concluding anything about the book.

How a match is verified: the returned title is scored against what was asked for (token
overlap on both the full title and the title with any subtitle stripped, so a subtitled
edition passes while a sequel does not), an author mismatch is penalised, and a weak score
retries against `--source google` then `--source openlibrary`, keeping the best hit.
Below `0.60` the result is marked unverified.

## Agents (MCP)

`marty mcp` serves that same resolution over the Model Context Protocol, so an agent can
call it as a tool instead of shelling out and parsing text. Two read-only tools:
`resolve_book` (title, author) and `resolve_isbn` (isbn). Both return the full result as
structured content — `verified`, `confidence`, `reason`, `isbn13`, and metadata — so a
caller cannot mistake a rejected guess for an answer.

`.mcp.json` in this repo registers the server for Claude Code; approve it once when
prompted. For any other client, the command is `go run ./cmd/marty mcp` (or the built
binary with the `mcp` subcommand) from the repo root.

## Sources

`book` layers three sources, stopping once the important fields are filled:

1. **Hardcover** (`HARDCOVER_API_TOKEN`) — ratings, genre tags, descriptions, new releases.
2. **Google Books** (`GOOGLE_BOOKS_API_KEY`, optional) — broad coverage fallback.
3. **Open Library** (keyless) — last-resort gap fill.

Without a Hardcover token it degrades to Google Books + Open Library.

Credentials come from the environment, then `./.env`, then `$MARTY_ENV`, then
`/etc/dungeonbooks/marty.env` — first one to define a variable wins. Keeping the token in
`/etc/dungeonbooks/marty.env` puts it outside every checkout, so it survives a `git pull`
and is shared by whatever launches the MCP server.

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
