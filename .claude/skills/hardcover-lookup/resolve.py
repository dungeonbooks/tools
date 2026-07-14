#!/usr/bin/env python3
"""Resolve a book title to a verified ISBN-13 + metadata using the marty CLI.

Read-only. Wraps `marty book` with the hard-won lessons from real lookups:

  * Bare titles mismatch badly (e.g. "Playing at the World" -> "King Lear").
    Always pass the author; this script folds it into the query.
  * marty's default source-merge sometimes returns a wrong book whose title
    barely matches. This script verifies the returned title against the
    request and, on a weak match, falls back to --source google then
    --source openlibrary, keeping the best-scoring hit.
  * Some books simply aren't in any source (e.g. "Playing Place" by Chad
    Randl). When nothing clears the confidence bar the result is marked
    unverified and the process exits non-zero so callers don't trust it.

Usage:
    resolve.py "<title>" [--author "<author>"] [--json]
    resolve.py --isbn 9780262542951 [--json]

Token: needs HARDCOVER_API_TOKEN for best results (ratings/genres + the
Hardcover record). Resolution order:
    1. HARDCOVER_API_TOKEN already in the environment
    2. file at $MARTY_ENV
    3. /etc/dungeonbooks/marty.env
Without a token it still works, degraded, on Google Books + Open Library.
"""
import argparse
import json
import os
import re
import subprocess
import sys
from difflib import SequenceMatcher

DEFAULT_ENV = "/etc/dungeonbooks/marty.env"
CONFIDENCE_FLOOR = 0.60
SOURCES = [None, "google", "openlibrary"]  # None = marty's default merge


def repo_root() -> str:
    # this file lives at <repo>/.claude/skills/hardcover-lookup/resolve.py
    here = os.path.dirname(os.path.abspath(__file__))
    return os.path.abspath(os.path.join(here, "..", "..", ".."))


def load_env_file(path: str) -> None:
    """Inject KEY=VALUE pairs from an env file into os.environ (no override)."""
    try:
        with open(path) as fh:
            for raw in fh:
                line = raw.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, val = line.split("=", 1)
                key = key.strip()
                val = val.strip().strip('"').strip("'")
                # tolerate trailing "# comment" on unquoted values
                if "#" in val and not raw.split("=", 1)[1].lstrip().startswith(('"', "'")):
                    val = val.split("#", 1)[0].strip()
                os.environ.setdefault(key, val)
    except FileNotFoundError:
        pass


def ensure_token() -> bool:
    if os.environ.get("HARDCOVER_API_TOKEN"):
        return True
    for path in (os.environ.get("MARTY_ENV"), DEFAULT_ENV):
        if path and os.path.exists(path):
            load_env_file(path)
            if os.environ.get("HARDCOVER_API_TOKEN"):
                return True
    return bool(os.environ.get("HARDCOVER_API_TOKEN"))


def marty_bin() -> str:
    if os.environ.get("MARTY_BIN"):
        return os.environ["MARTY_BIN"]
    root = repo_root()
    binary = os.path.join(root, "marty")
    if not os.path.exists(binary):
        subprocess.run(
            ["go", "build", "-o", "marty", "./cmd/marty"],
            cwd=root, check=True,
        )
    return binary


def run_marty(query: str, source: str | None) -> dict | None:
    cmd = [marty_bin(), "book", query, "--json"]
    if source:
        cmd += ["--source", source]
    try:
        out = subprocess.run(
            cmd, cwd=repo_root(), capture_output=True, text=True, timeout=60,
        )
    except subprocess.TimeoutExpired:
        return None
    if out.returncode != 0 or not out.stdout.strip():
        return None
    try:
        return json.loads(out.stdout)
    except json.JSONDecodeError:
        return None


def norm(s: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"[^a-z0-9 ]", " ", (s or "").lower())).strip()


def normalize_isbn(s: str) -> str:
    return re.sub(r"[^0-9Xx]", "", s or "").upper()


def valid_isbn13(s: str) -> bool:
    d = normalize_isbn(s)
    if len(d) != 13 or not d.isdigit():
        return False
    checksum = sum((1 if i % 2 == 0 else 3) * int(c) for i, c in enumerate(d))
    return checksum % 10 == 0


def title_score(want: str, got: str) -> float:
    """How well `got` (may carry a subtitle) covers the requested title.

    Token overlap is scored as the *minimum* of recall and precision, so a
    short title that merely appears inside a longer, unrelated one (want="It"
    vs got="It Ends With Us") can't earn a perfect 1.0 from containment alone.
    The SequenceMatcher ratio backstops legitimate subtitle matches, where the
    requested title is a real prefix of the result.
    """
    w, g = norm(want), norm(got)
    if not w or not g:
        return 0.0
    wt, gt = set(w.split()), set(g.split())
    overlap = len(wt & gt)
    recall = overlap / len(wt)
    precision = overlap / len(gt)
    return max(min(recall, precision), SequenceMatcher(None, w, g).ratio())


def author_ok(want: str | None, got: str) -> bool:
    if not want:
        return True
    gt = set(norm(got).split())
    # any surname-ish token from the requested author present in the result
    return any(tok in gt for tok in norm(want).split() if len(tok) > 2)


def book_fields(data: dict) -> dict:
    """Pull the metadata fields we surface from a marty book record."""
    return {
        "isbn13": data.get("isbn13"),
        "title": data.get("title"),
        "author": data.get("author"),
        "year": data.get("year"),
        "rating": data.get("rating"),
        "ratings_count": data.get("ratings_count"),
        "hardcover_url": data.get("hardcover_url"),
    }


def resolve(title: str, author: str | None) -> dict:
    query = f"{title} {author}".strip() if author else title
    best, best_score = None, -1.0
    for src in SOURCES:
        data = run_marty(query, src)
        if not data or not data.get("isbn13"):
            continue
        score = title_score(title, data.get("title", ""))
        if author and not author_ok(author, data.get("author", "")):
            score -= 0.25
        if score > best_score:
            best, best_score = data, score
        if score >= 0.85:  # strong hit, stop early
            break
    if best is None:
        return {"query": query, "verified": False, "reason": "no result with an ISBN from any source"}
    confidence = round(max(best_score, 0.0), 2)
    verified = best_score >= CONFIDENCE_FLOOR
    result = book_fields(best)
    result.update({"query": query, "verified": verified, "confidence": confidence})
    if not verified:
        result["reason"] = f"weak title/author match (confidence {confidence} < {CONFIDENCE_FLOOR})"
    return result


def resolve_isbn(isbn: str) -> dict:
    want = normalize_isbn(isbn)
    if not valid_isbn13(want):
        return {"query": isbn, "verified": False,
                "reason": "not a valid ISBN-13 (need 13 digits with a correct check digit)"}
    data = run_marty(want, None)
    got = normalize_isbn(data.get("isbn13", "")) if data else ""
    if not got:
        return {"query": isbn, "verified": False, "reason": "no book for that ISBN"}
    result = book_fields(data)
    result["query"] = isbn
    if got != want:
        # marty fell back to a search and returned a different book.
        result["verified"] = False
        result["confidence"] = 0.0
        result["reason"] = f"marty returned a different ISBN ({data.get('isbn13')})"
        return result
    result.update({"verified": True, "confidence": 1.0})
    return result


def main() -> int:
    ap = argparse.ArgumentParser(description="Resolve a book to a verified ISBN-13 via marty.")
    ap.add_argument("title", nargs="?", help="book title")
    ap.add_argument("--author", help="author (strongly recommended; prevents mismatches)")
    ap.add_argument("--isbn", help="resolve a known ISBN instead of a title")
    ap.add_argument("--json", action="store_true", help="emit JSON")
    args = ap.parse_args()

    if not args.isbn and not args.title:
        ap.error("provide a title or --isbn")

    ensure_token()  # best-effort; lookup still works degraded without a token
    result = resolve_isbn(args.isbn) if args.isbn else resolve(args.title, args.author)

    if args.json:
        print(json.dumps(result, indent=2))
    elif result.get("verified"):
        print(f"{result['isbn13']}  {result['title']} — {result.get('author')} "
              f"({result.get('year')})  [confidence {result['confidence']}]")
    else:
        reason = result.get("reason") or f"weak match (confidence {result.get('confidence')})"
        print(f"UNVERIFIED: {reason}  query={result['query']!r}", file=sys.stderr)
        if result.get("isbn13"):
            print(f"  best guess: {result['isbn13']}  {result.get('title')} — {result.get('author')}",
                  file=sys.stderr)

    return 0 if result.get("verified") else 1


if __name__ == "__main__":
    sys.exit(main())
