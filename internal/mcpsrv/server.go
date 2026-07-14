// Package mcpsrv exposes marty's book resolution over the Model Context
// Protocol, so an agent can look a book up as a tool call instead of shelling
// out and parsing text.
//
// The tools are read-only: they look books up and never write to any account.
package mcpsrv

import (
	"context"
	"fmt"

	"github.com/dungeonbooks/tools/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is reported to clients during initialize.
const Version = "0.1.0"

const resolveBookDescription = `Resolve a book title to a VERIFIED ISBN-13 plus metadata (author, year, rating, Hardcover URL). Read-only. Use this whenever you need the ISBN or canonical metadata for a book you know by title — for example turning a reading list or an article's book mentions into ISBNs.

Always pass "author" when you know it. A bare title mismatches badly: "Playing at the World" on its own resolves to King Lear, and "Unboxed" to a Dr. Seuss box set.

Check "verified" before you use anything. When verified is false, the isbn13 field is a rejected best guess, NOT this book's ISBN — do not quote it, cite it, or write it anywhere. Report that the book could not be resolved and say why ("reason" explains). Some books are in none of the metadata sources, so unverified is a legitimate final answer for them; substituting a plausible ISBN for a real one is the failure this tool exists to prevent.

If "retryable" is true the lookup itself failed — a provider errored or timed out. That is a fact about the provider, not about the book. Call again before saying anything about whether the book exists.`

const resolveISBNDescription = `Verify that an ISBN-13 names a real book, and return that book's metadata. Read-only. Use this to check an ISBN you were given or found, rather than assuming it is correct.

Rejects malformed ISBNs on the check digit before spending a network call, and catches the case where a provider cannot find the ISBN and quietly falls back to returning a different book.

When "verified" is false the ISBN does not name the book described, and the metadata (if any) belongs to some other book.`

type bookArgs struct {
	Title  string `json:"title" jsonschema:"the book's title, without the author's name"`
	Author string `json:"author,omitempty" jsonschema:"the author's name; supply this whenever it is known, as it is what prevents a confident wrong match"`
}

type isbnArgs struct {
	ISBN string `json:"isbn" jsonschema:"the ISBN-13 to verify; hyphens and spaces are fine"`
}

// New builds the marty MCP server over the given lookup.
func New(lk resolve.Lookup) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "marty",
		Title:   "Marty — book lookup",
		Version: Version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "resolve_book",
		Title:       "Resolve a book title to a verified ISBN",
		Description: resolveBookDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args bookArgs) (*mcp.CallToolResult, resolve.Result, error) {
		r := resolve.Title(ctx, lk, args.Title, args.Author)
		return summarize(r), r, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "resolve_isbn",
		Title:       "Verify an ISBN-13",
		Description: resolveISBNDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args isbnArgs) (*mcp.CallToolResult, resolve.Result, error) {
		r := resolve.ISBN(ctx, lk, args.ISBN)
		return summarize(r), r, nil
	})

	return s
}

// Run serves the given lookup over stdio until the client disconnects.
func Run(ctx context.Context, lk resolve.Lookup) error {
	return New(lk).Run(ctx, &mcp.StdioTransport{})
}

// summarize leads the tool result with a sentence stating the verdict, because
// the structured payload alone lets an unverified ISBN read as a real one at a
// glance. The SDK still attaches the full Result as structured content.
func summarize(r resolve.Result) *mcp.CallToolResult {
	var text string
	switch {
	case r.Verified:
		text = fmt.Sprintf("VERIFIED (confidence %.2f): %s — %s", r.Confidence, r.ISBN13, r.Describe())
	case r.Retryable:
		// Reason already spells out that this says nothing about the book.
		text = "LOOKUP FAILED: " + r.Reason
	case r.ISBN13 != "":
		text = fmt.Sprintf("UNVERIFIED: %s. The best guess was %s (%s), which failed verification: do not use it as this book's ISBN.",
			r.Reason, r.ISBN13, r.Describe())
	default:
		text = fmt.Sprintf("UNVERIFIED: %s. No ISBN can be given for this book; report that rather than supplying one.", r.Reason)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func ptr[T any](v T) *T { return &v }
