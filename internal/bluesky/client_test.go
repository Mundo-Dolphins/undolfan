package bluesky

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAuthorFeedPaginationResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/app.bsky.feed.getAuthorFeed" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("actor") != "undolfan.mundodolphins.es" {
			t.Fatalf("actor query missing")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("missing user agent")
		}
		w.Write([]byte(`{"cursor":"next","feed":[{"post":{"uri":"at://did/post/a","author":{"handle":"undolfan.mundodolphins.es"},"record":{"text":"A","createdAt":"2026-08-01T10:00:00Z"}}}]}`))
	}))
	defer server.Close()
	client := New(WithBaseURL(server.URL))
	resp, err := client.GetAuthorFeed(context.Background(), "undolfan.mundodolphins.es", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Cursor != "next" || len(resp.Feed) != 1 {
		t.Fatalf("unexpected response %#v", resp)
	}
}

func TestRetriesTransientHTTP(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"feed":[]}`))
	}))
	defer server.Close()
	client := New(WithBaseURL(server.URL))
	if _, err := client.GetAuthorFeed(context.Background(), "actor", "", 100); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected retry, got %d calls", calls)
	}
}

func TestHTTPErrorIsTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()
	client := New(WithBaseURL(server.URL))
	_, err := client.GetPostThread(context.Background(), "at://did/post/a", 10)
	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("expected ErrHTTP, got %v", err)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected typed 404, got %v", err)
	}
}
