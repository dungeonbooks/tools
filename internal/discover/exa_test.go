package discover

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExaDisabledWithoutKey(t *testing.T) {
	e := NewExa("", httptest.NewServer(http.NewServeMux()).Client())
	if e.Enabled() {
		t.Fatal("expected disabled with empty key")
	}
	if _, err := e.Trending(context.Background(), "x", TypeAuto, 3); err == nil {
		t.Fatal("expected error when disabled")
	}
}

func TestExaTrendingParsesAnswerAndReportsSpend(t *testing.T) {
	var spent float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/answer" || r.Header.Get("x-api-key") != "k" {
			t.Errorf("unexpected request: %s %s key=%q", r.Method, r.URL, r.Header.Get("x-api-key"))
		}
		buf, _ := io.ReadAll(r.Body)
		body := string(buf)
		for _, want := range []string{`"type":"auto"`, `"numResults":3`, `"outputSchema"`, "fantasy debut"} {
			if !contains(body, want) {
				t.Errorf("request body missing %q", want)
			}
		}
		w.Write([]byte(`{
  "answer": {
    "books": [
      {"title":"Onyx Storm","author":"Rebecca Yarros","why_trending":"sold 2.7M in week one","source_url":"https://example.com/onyx","isbn":"9781250898124"},
      {"title":"No Author Book","why_trending":"viral","source_url":"https://example.com/x"},
      {"title":"","why_trending":"should be skipped","source_url":"https://example.com/skip"}
    ]
  },
  "costDollars": {"total": 0.05}
}`))
	}))
	defer srv.Close()

	e := NewExa("k", srv.Client())
	e.base = srv.URL
	e.OnSpend(func(d float64) { spent = d })

	cs, err := e.Trending(context.Background(), "fantasy debut", TypeAuto, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected 2 candidates (empty title skipped), got %d", len(cs))
	}
	if cs[0].Title != "Onyx Storm" || cs[0].Author != "Rebecca Yarros" || cs[0].ISBN13 != "9781250898124" {
		t.Fatalf("bad first: %+v", cs[0])
	}
	if cs[1].Author != "" {
		t.Fatalf("expected empty author, got %q", cs[1].Author)
	}
	if spent != 0.05 {
		t.Fatalf("expected spend 0.05, got %f", spent)
	}
	if cs[0].ISBN13 != "9781250898124" {
		t.Fatalf("expected real ISBN, got %q", cs[0].ISBN13)
	}
}

func TestExaCapsToCountAndDropsPlaceholderISBN(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"answer":{"books":[
			{"title":"B1","why_trending":"a","source_url":"u","isbn":"N/A"},
			{"title":"B2","why_trending":"b","source_url":"u","isbn":"n/a"},
			{"title":"B3","why_trending":"c","source_url":"u"},
			{"title":"B4","why_trending":"d","source_url":"u"}
		]},"costDollars":{"total":0.01}}`))
	}))
	defer srv.Close()
	e := NewExa("k", srv.Client())
	e.base = srv.URL
	cs, err := e.Trending(context.Background(), "q", TypeAuto, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("expected cap to 2, got %d", len(cs))
	}
	for _, c := range cs {
		if c.ISBN13 != "" {
			t.Fatalf("expected N/A dropped to empty, got %q", c.ISBN13)
		}
	}
}

func TestExaErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	e := NewExa("k", srv.Client())
	e.base = srv.URL
	if _, err := e.Trending(context.Background(), "x", TypeAuto, 2); err == nil {
		t.Fatal("expected error on 429")
	}
}

func TestExaEmptyQueryUsesDefault(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		got = string(buf)
		w.Write([]byte(`{"answer":{"books":[]}}`))
	}))
	defer srv.Close()
	e := NewExa("k", srv.Client())
	e.base = srv.URL
	e.Trending(context.Background(), "", TypeAuto, 1)
	if !contains(got, "BookTok") {
		t.Fatalf("empty query should fall back to default, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
