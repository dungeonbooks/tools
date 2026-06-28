---
name: hardcover-lookup
description: Resolve book titles to verified ISBN-13s and metadata (author, year, rating, Hardcover URL) using the marty CLI. Read-only — looks books up, never writes to any account. Use when an agent needs the ISBN or canonical metadata for a book given its title (and ideally author), e.g. turning a reading list or article's book mentions into ISBNs.
---

# Hardcover book lookup

Read-only book resolution. Given a title (and ideally an author), returns a
**verified** ISBN-13 plus metadata. Wraps the `marty` CLI from this repo and
layers in match-verification so callers can trust the result.

## Quick start

```sh
# from the repo root (this skill lives at .claude/skills/hardcover-lookup/)
python3 .claude/skills/hardcover-lookup/resolve.py "<title>" --author "<author>"
python3 .claude/skills/hardcover-lookup/resolve.py "<title>" --author "<author>" --json
python3 .claude/skills/hardcover-lookup/resolve.py --isbn 9780262542951
```

Output (human): `9780262376303  The Beauty of Games — Frank Lantz (2023)  [confidence 1.0]`
Output (`--json`): `{query, verified, confidence, isbn13, title, author, year, rating,
ratings_count, hardcover_url}`. On an unverified result a `reason` field explains
why (weak match, invalid ISBN, different ISBN returned, or not found); `rating`
and `ratings_count` are present only when `marty` reports them (needs a token).

**Exit code is the contract:** `0` = verified match, `1` = unverified (weak
match or nothing found, details on stderr). Never trust an exit-1 ISBN.

## Always pass `--author`

Bare titles mismatch badly — `"Playing at the World"` alone resolves to *King
Lear*, `"Unboxed"` to a Dr. Seuss box set. The author disambiguates and is
folded into the search query. If you only have a title, the result is far less
reliable; treat a low `confidence` as a maybe.

## How it verifies (so you don't have to)

`marty`'s default source-merge occasionally returns a confidently-wrong book
(a romance novel for "Playing Place"). The resolver guards against this:

1. Queries `marty book "<title> <author>" --json`.
2. Scores the returned title against the request (token containment + ratio);
   penalizes an author mismatch.
3. On a weak score, retries `--source google` then `--source openlibrary`,
   keeping the best hit.
4. Below the confidence floor (0.60) → marked `verified: false`, exit 1.

Some books are in **no** source (e.g. "Playing Place" by Chad Randl was absent
from Hardcover). Those come back unverified — that's the correct answer, not a
bug. Report it rather than substituting a wrong ISBN.

## Setup (per machine)

1. **Go 1.25+** (matches `go.mod`) must be installed to build `marty` on first
   run, and **Python 3.10+** to run `resolve.py` (it uses `X | None` type syntax).
2. **API token** — for ratings/genres and the Hardcover record, the resolver
   needs `HARDCOVER_API_TOKEN`. It is found in this order:
   - `HARDCOVER_API_TOKEN` already in the environment, else
   - the file at `$MARTY_ENV`, else
   - `/etc/dungeonbooks/marty.env`.

   Without a token it still works, degraded, on Google Books + Open Library
   (keyless). `GOOGLE_BOOKS_API_KEY` is optional and only widens coverage.
3. **marty binary** — built automatically on first run into `<repo>/marty`.
   Override with `MARTY_BIN=/path/to/marty` if you keep it elsewhere.

The token never lives in the repo (`.env` is gitignored). Keep it in
`/etc/dungeonbooks/marty.env` so it survives `git pull` and is shared across
agents on the box.
