package mcpsrv

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stub mirrors enrich: a source that finds nothing returns an empty book and no
// error, so an error means the request failed.
type stub map[string]bookmeta.Book

func (s stub) Book(_ context.Context, _, source string) (bookmeta.Book, error) {
	return s[source], nil
}

// brokenStub stands in for a provider outage.
type brokenStub struct{}

func (brokenStub) Book(_ context.Context, _, _ string) (bookmeta.Book, error) {
	return bookmeta.Book{}, errors.New("googlebooks: status 429")
}

// connect runs the real server over a real client session, so these exercise the
// wire contract — initialize, tools/list, tools/call — not just the handlers.
func connect(t *testing.T, lk resolve.Lookup) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()

	server := New(lk)
	serverSession, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func call(t *testing.T, s *mcp.ClientSession, tool string, args map[string]any) (*mcp.CallToolResult, resolve.Result) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned a tool error: %+v", tool, res.Content)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out resolve.Result
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return res, out
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestToolsAreListedWithSchemasAndGuidance(t *testing.T) {
	session := connect(t, stub{})

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	byName := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}
	for _, want := range []string{"resolve_book", "resolve_isbn"} {
		tool, ok := byName[want]
		if !ok {
			t.Fatalf("tool %q not advertised; got %v", want, byName)
		}
		if tool.InputSchema == nil {
			t.Errorf("%s has no input schema", want)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s has no output schema: callers would get untyped results", want)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", want)
		}
	}

	// The guard rails live in the description; nothing else can carry them.
	desc := byName["resolve_book"].Description
	for _, phrase := range []string{"verified", "author"} {
		if !strings.Contains(strings.ToLower(desc), phrase) {
			t.Errorf("resolve_book description never mentions %q:\n%s", phrase, desc)
		}
	}

	schema, err := json.Marshal(byName["resolve_book"].InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(schema, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Required) != 1 || got.Required[0] != "title" {
		t.Errorf("required = %v, want [title] (author must stay optional but urged)", got.Required)
	}
}

func TestResolveBookReturnsStructuredVerifiedResult(t *testing.T) {
	session := connect(t, stub{
		"": {ISBN13: "9780262376303", Title: "The Beauty of Games", Author: "Frank Lantz", Year: 2023},
	})

	res, out := call(t, session, "resolve_book", map[string]any{
		"title": "The Beauty of Games", "author": "Frank Lantz",
	})

	if !out.Verified {
		t.Fatalf("Verified = false (%s)", out.Reason)
	}
	if out.ISBN13 != "9780262376303" {
		t.Errorf("ISBN13 = %q", out.ISBN13)
	}
	if !strings.Contains(text(res), "VERIFIED") {
		t.Errorf("text content does not state the verdict: %q", text(res))
	}
}

// An unverified result must be unmistakable in the text a model reads, not only
// in a boolean it might skip.
func TestResolveBookAnnouncesAnUnverifiedResult(t *testing.T) {
	session := connect(t, stub{
		"": {ISBN13: "9780743482769", Title: "King Lear", Author: "William Shakespeare"},
	})

	res, out := call(t, session, "resolve_book", map[string]any{
		"title": "Playing at the World", "author": "Jon Peterson",
	})

	if out.Verified {
		t.Fatal("King Lear was verified")
	}
	body := text(res)
	if !strings.Contains(body, "UNVERIFIED") {
		t.Errorf("text content does not lead with UNVERIFIED: %q", body)
	}
	if !strings.Contains(body, "do not use it") {
		t.Errorf("text content does not tell the caller to discard the guess: %q", body)
	}
}

// A provider outage must not read as "this book does not exist". The text a
// model actually reads has to say so, not just a flag it might not consult.
func TestResolveBookDistinguishesAnOutageFromAnAbsentBook(t *testing.T) {
	session := connect(t, brokenStub{})

	res, out := call(t, session, "resolve_book", map[string]any{
		"title": "Playing Place", "author": "Chad Randl",
	})

	if out.Verified {
		t.Fatal("a failed lookup cannot verify")
	}
	if !out.Retryable {
		t.Error("Retryable = false; the caller cannot tell an outage from an absent book")
	}
	body := text(res)
	if !strings.Contains(body, "LOOKUP FAILED") {
		t.Errorf("text content does not announce the failure: %q", body)
	}
	if !strings.Contains(body, "says nothing about whether the book exists") {
		t.Errorf("text content lets an outage be read as a fact about the book: %q", body)
	}
}

func TestResolveISBNRejectsABadCheckDigit(t *testing.T) {
	session := connect(t, stub{})

	_, out := call(t, session, "resolve_isbn", map[string]any{"isbn": "9780262542950"})

	if out.Verified {
		t.Fatal("an ISBN with a bad check digit was verified")
	}
	if !strings.Contains(out.Reason, "check digit") {
		t.Errorf("Reason = %q, want it to name the check digit", out.Reason)
	}
}

// Input is validated against the schema before the handler runs, and a breach
// comes back as a tool error the model can read, not a transport failure.
func TestMissingTitleIsRejected(t *testing.T) {
	session := connect(t, stub{})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "resolve_book",
		Arguments: map[string]any{"author": "Frank Lantz"},
	})

	if err != nil {
		t.Fatalf("want a tool error, got a protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("a call with no title was accepted")
	}
	if !strings.Contains(text(res), "title") {
		t.Errorf("error does not name the missing field: %q", text(res))
	}
}
