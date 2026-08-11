package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/undolfan/internal/bluesky"
)

type fakeClient struct {
	pages   [][]bluesky.Post
	threads map[string]bluesky.ThreadResponse
}

func (f *fakeClient) GetAuthorFeed(ctx context.Context, actor, cursor string, limit int) (*bluesky.FeedResponse, error) {
	page := 0
	if cursor != "" {
		page = int(cursor[0] - '0')
	}
	if page >= len(f.pages) {
		return &bluesky.FeedResponse{}, nil
	}
	next := ""
	if page+1 < len(f.pages) {
		next = string(rune('0' + page + 1))
	}
	var items []bluesky.FeedItem
	for _, post := range f.pages[page] {
		items = append(items, bluesky.FeedItem{Post: post})
	}
	return &bluesky.FeedResponse{Cursor: next, Feed: items}, nil
}

func (f *fakeClient) GetPostThread(ctx context.Context, uri string, depth int) (*bluesky.ThreadResponse, error) {
	resp, ok := f.threads[uri]
	if !ok {
		return &bluesky.ThreadResponse{Thread: bluesky.ThreadNode{Post: post(uri, "Fallback", "")}}, nil
	}
	return &resp, nil
}

func TestIndependentPostBecomesShort(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "Tua no está participando hoy.", "")
	dir, res := runSync(t, []bluesky.Post{a}, thread(a), false)
	if res.New != 1 {
		t.Fatalf("expected one new entry, got %#v", res)
	}
	md := onlyIndex(t, dir)
	assertContains(t, md, `content_type: "short"`)
	assertContains(t, md, "bluesky_post_count: 1")
}

func TestSimpleThreadBecomesOneArticle(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "1/ Entrenamiento abierto.", "")
	b := reply("at://did:author/app.bsky.feed.post/b", "2/ Tua participa en individuales.", a, a)
	dir, _ := runSync(t, []bluesky.Post{a, b}, thread(a, b), false)
	files := indexes(t, dir)
	if len(files) != 1 {
		t.Fatalf("expected one entry, got %d", len(files))
	}
	md := read(t, files[0])
	assertContains(t, md, `content_type: "article"`)
	assertContains(t, md, "bluesky_post_count: 2")
	if strings.Contains(md, "2/") {
		t.Fatalf("thread marker not cleaned:\n%s", md)
	}
}

func TestLongThreadOrdering(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A", "")
	b := reply("at://did:author/app.bsky.feed.post/b", "B", a, a)
	c := reply("at://did:author/app.bsky.feed.post/c", "C", a, b)
	d := reply("at://did:author/app.bsky.feed.post/d", "D", a, c)
	dir, _ := runSync(t, []bluesky.Post{a, b, c, d}, thread(a, b, c, d), false)
	md := onlyIndex(t, dir)
	assertContains(t, md, "bluesky_post_count: 4")
	if !(strings.Index(md, "A") < strings.Index(md, "B") && strings.Index(md, "B") < strings.Index(md, "C") && strings.Index(md, "C") < strings.Index(md, "D")) {
		t.Fatalf("posts not ordered:\n%s", md)
	}
}

func TestMixedContentProducesThreeEntries(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A root", "")
	b := reply("at://did:author/app.bsky.feed.post/b", "B reply", a, a)
	c := post("at://did:author/app.bsky.feed.post/c", "C alone", "")
	d := post("at://did:author/app.bsky.feed.post/d", "D root", "")
	e := reply("at://did:author/app.bsky.feed.post/e", "E reply", d, d)
	dir := t.TempDir()
	client := &fakeClient{
		pages: [][]bluesky.Post{{a, b, c, d, e}},
		threads: map[string]bluesky.ThreadResponse{
			a.URI: thread(a, b),
			c.URI: thread(c),
			d.URI: thread(d, e),
		},
	}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.New != 3 {
		t.Fatalf("expected three entries, got %#v", res)
	}
	if len(indexes(t, dir)) != 3 {
		t.Fatalf("expected three index files")
	}
}

func TestShortEvolvesToArticleSameBundle(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "Tua entrena con el equipo.", "")
	dir, res := runSync(t, []bluesky.Post{a}, thread(a), false)
	if res.New != 1 {
		t.Fatalf("first sync failed: %#v", res)
	}
	firstPath := indexes(t, dir)[0]
	firstMD := read(t, firstPath)
	b := reply("at://did:author/app.bsky.feed.post/b", "Luego participa en 11 contra 11.", a, a)
	client := &fakeClient{pages: [][]bluesky.Post{{a, b}}, threads: map[string]bluesky.ThreadResponse{a.URI: thread(a, b)}}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || len(indexes(t, dir)) != 1 {
		t.Fatalf("expected same bundle update, got %#v files=%d", res, len(indexes(t, dir)))
	}
	if firstPath != indexes(t, dir)[0] {
		t.Fatalf("bundle path changed")
	}
	secondMD := read(t, firstPath)
	assertContains(t, secondMD, `content_type: "article"`)
	assertContains(t, secondMD, "bluesky_post_count: 2")
	if !strings.Contains(firstMD, `content_type: "short"`) {
		t.Fatalf("first sync was not short")
	}
	assertContains(t, secondMD, `date: 2026-08-01T10:00:00+00:00`)
	assertContains(t, secondMD, `lastmod: 2026-08-01T10:01:00+00:00`)
}

func TestArticleExpansionSameBundle(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A", "")
	b := reply("at://did:author/app.bsky.feed.post/b", "B", a, a)
	dir, _ := runSync(t, []bluesky.Post{a, b}, thread(a, b), false)
	firstPath := indexes(t, dir)[0]
	c := reply("at://did:author/app.bsky.feed.post/c", "C", a, b)
	client := &fakeClient{pages: [][]bluesky.Post{{a, b, c}}, threads: map[string]bluesky.ThreadResponse{a.URI: thread(a, b, c)}}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || firstPath != indexes(t, dir)[0] {
		t.Fatalf("expected same bundle update, got %#v", res)
	}
	assertContains(t, read(t, firstPath), "bluesky_post_count: 3")
}

func TestThirdPartyReplyExcludedButAuthorReplyIncluded(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A", "")
	third := post("at://did:third/app.bsky.feed.post/b", "THIRD", "third.example")
	third.Record.Reply = &bluesky.ReplyRef{Root: bluesky.StrongRef{URI: a.URI}, Parent: bluesky.StrongRef{URI: a.URI}}
	c := reply("at://did:author/app.bsky.feed.post/c", "C", a, a)
	dir, _ := runSync(t, []bluesky.Post{a, c}, bluesky.ThreadResponse{Thread: bluesky.ThreadNode{Post: a, Replies: []bluesky.ThreadNode{{Post: third}, {Post: c}}}}, false)
	md := onlyIndex(t, dir)
	assertContains(t, md, "A")
	assertContains(t, md, "C")
	if strings.Contains(md, "THIRD") {
		t.Fatalf("third-party post imported:\n%s", md)
	}
}

func TestAuthorReplyToThirdCanBecomeOwnArticle(t *testing.T) {
	third := post("at://did:third/app.bsky.feed.post/a", "Third root", "third.example")
	b := reply("at://did:author/app.bsky.feed.post/b", "Own start", third, third)
	c := reply("at://did:author/app.bsky.feed.post/c", "Own continuation", b, b)
	dir, _ := runSync(t, []bluesky.Post{b, c}, thread(b, c), false)
	md := onlyIndex(t, dir)
	assertContains(t, md, `content_type: "article"`)
	assertContains(t, md, "Own start")
	assertContains(t, md, "Own continuation")
	if strings.Contains(md, "Third root") {
		t.Fatalf("third-party context imported as body")
	}
}

func TestIdempotentSecondSync(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A", "")
	dir, res := runSync(t, []bluesky.Post{a}, thread(a), false)
	if res.New != 1 {
		t.Fatal(res)
	}
	statePath := filepath.Join(dir, "data", "bluesky-state.json")
	before := read(t, statePath)
	client := &fakeClient{pages: [][]bluesky.Post{{a}}, threads: map[string]bluesky.ThreadResponse{a.URI: thread(a)}}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unchanged != 1 || res.New != 0 || res.Updated != 0 {
		t.Fatalf("expected unchanged second sync, got %#v", res)
	}
	after := read(t, statePath)
	if before != after {
		t.Fatalf("state changed on idempotent sync\nbefore:%s\nafter:%s", before, after)
	}
}

func TestEditedPostUpdatesExistingBundleAndState(t *testing.T) {
	a := post("at://did:author/app.bsky.feed.post/a", "A", "")
	dir, _ := runSync(t, []bluesky.Post{a}, thread(a), false)
	statePath := filepath.Join(dir, "data", "bluesky-state.json")
	before := read(t, statePath)
	edited := a
	edited.Record.Text = "A edited"
	client := &fakeClient{pages: [][]bluesky.Post{{edited}}, threads: map[string]bluesky.ThreadResponse{edited.URI: thread(edited)}}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 {
		t.Fatalf("expected updated content, got %#v", res)
	}
	assertContains(t, onlyIndex(t, dir), "A edited")
	if before == read(t, statePath) {
		t.Fatalf("state should update when content changes")
	}
}

func TestImageWrittenOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("jpeg"))
	}))
	defer server.Close()
	a := post("at://did:author/app.bsky.feed.post/a", "A with image", "")
	a.Embed = &bluesky.Embed{Images: []bluesky.EmbedImage{{Alt: "Campo", Fullsize: server.URL + "/image.jpg", AspectRatio: &bluesky.AspectRatio{Width: 1200, Height: 800}}}}
	dir, _ := runSync(t, []bluesky.Post{a}, thread(a), false)
	md := onlyIndex(t, dir)
	assertContains(t, md, `images:`)
	assertContains(t, md, `image-01.jpg`)
	if _, err := os.Stat(filepath.Join(filepath.Dir(indexes(t, dir)[0]), "image-01.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "content", "posts", "2026", "08", "a-with-image", "image-01.jpg")); err != nil {
		t.Fatalf("image should be written directly into year/month bundle: %v", err)
	}
}

func TestImageDownloadRetriesTransientStatus(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("jpeg"))
	}))
	defer server.Close()
	a := post("at://did:author/app.bsky.feed.post/a", "A with flaky image", "")
	a.Embed = &bluesky.Embed{Images: []bluesky.EmbedImage{{Alt: "Campo", Fullsize: server.URL + "/image.jpg"}}}
	dir, res := runSync(t, []bluesky.Post{a}, thread(a), false)
	if res.New != 1 {
		t.Fatalf("sync failed: %#v", res)
	}
	if calls != 2 {
		t.Fatalf("expected retry, got %d calls", calls)
	}
	assertContains(t, onlyIndex(t, dir), `image-01.jpg`)
}

func TestImageDownloadPersistentTransientStatusIsSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
	}))
	defer server.Close()
	a := post("at://did:author/app.bsky.feed.post/a", "A with unavailable image", "")
	a.Embed = &bluesky.Embed{Images: []bluesky.EmbedImage{{Alt: "Campo", Fullsize: server.URL + "/image.jpg"}}}
	dir, res := runSync(t, []bluesky.Post{a}, thread(a), false)
	if res.New != 1 {
		t.Fatalf("sync should keep content without unavailable image: %#v", res)
	}
	md := onlyIndex(t, dir)
	if strings.Contains(md, `image-01.jpg`) {
		t.Fatalf("missing image should not be referenced:\n%s", md)
	}
}

func runSync(t *testing.T, feed []bluesky.Post, tr bluesky.ThreadResponse, dry bool) (string, Result) {
	t.Helper()
	dir := t.TempDir()
	client := &fakeClient{pages: [][]bluesky.Post{feed}, threads: map[string]bluesky.ThreadResponse{}}
	for _, post := range feed {
		if post.Record.Reply == nil {
			client.threads[post.URI] = tr
		}
	}
	if len(feed) > 0 {
		client.threads[feed[0].URI] = tr
	}
	res, err := New(client).Sync(context.Background(), Config{RepoRoot: dir, Actor: "undolfan.mundodolphins.es", Full: true, DryRun: dry})
	if err != nil {
		t.Fatal(err)
	}
	return dir, res
}

func post(uri, text, handle string) bluesky.Post {
	if handle == "" {
		handle = "undolfan.mundodolphins.es"
	}
	minute := strings.TrimPrefix(uri[strings.LastIndex(uri, "/")+1:], "")
	offset := int(minute[0] - 'a')
	return bluesky.Post{
		URI:    uri,
		CID:    "cid-" + uri[strings.LastIndex(uri, "/")+1:],
		Author: bluesky.Author{DID: didFor(handle), Handle: handle},
		Record: bluesky.Record{Text: text, CreatedAt: time.Date(2026, 8, 1, 10, offset, 0, 0, time.UTC)},
	}
}

func reply(uri, text string, root, parent bluesky.Post) bluesky.Post {
	p := post(uri, text, "")
	p.Record.Reply = &bluesky.ReplyRef{Root: bluesky.StrongRef{URI: root.URI, CID: root.CID}, Parent: bluesky.StrongRef{URI: parent.URI, CID: parent.CID}}
	return p
}

func thread(posts ...bluesky.Post) bluesky.ThreadResponse {
	if len(posts) == 0 {
		return bluesky.ThreadResponse{}
	}
	root := bluesky.ThreadNode{Post: posts[0]}
	for _, p := range posts[1:] {
		root.Replies = append(root.Replies, bluesky.ThreadNode{Post: p})
	}
	return bluesky.ThreadResponse{Thread: root}
}

func indexes(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	root := filepath.Join(dir, "content", "posts")
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(path) == "index.md" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func onlyIndex(t *testing.T, dir string) string {
	t.Helper()
	files := indexes(t, dir)
	if len(files) != 1 {
		t.Fatalf("expected one index, got %d", len(files))
	}
	return read(t, files[0])
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertContains(t *testing.T, s, want string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Fatalf("missing %q in:\n%s", want, s)
	}
}

func didFor(handle string) string {
	if handle == "third.example" {
		return "did:plc:third"
	}
	return "did:plc:author"
}
