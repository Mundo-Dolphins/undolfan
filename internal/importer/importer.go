package importer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/undolfan/internal/bluesky"
	"github.com/Mundo-Dolphins/undolfan/internal/content"
	"github.com/Mundo-Dolphins/undolfan/internal/state"
)

type BlueskyClient interface {
	GetAuthorFeed(ctx context.Context, actor, cursor string, limit int) (*bluesky.FeedResponse, error)
	GetPostThread(ctx context.Context, uri string, depth int) (*bluesky.ThreadResponse, error)
}

type Config struct {
	RepoRoot           string
	Actor              string
	MinimumThreadPosts int
	Full               bool
	DryRun             bool
	MaxPages           int
	OverlapPages       int
}

type Result struct {
	New, Updated, Unchanged int
	PostsFetched            int
	CandidateRoots          int
	ImagesDownloaded        int
}

type Importer struct {
	client     BlueskyClient
	httpClient *http.Client
	now        func() time.Time
}

func New(client BlueskyClient) *Importer {
	return &Importer{
		client: client,
		httpClient: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		}},
		now: time.Now,
	}
}

func (im *Importer) Sync(ctx context.Context, cfg Config) (Result, error) {
	if cfg.RepoRoot == "" {
		cfg.RepoRoot = "."
	}
	if cfg.Actor == "" {
		cfg.Actor = "undolfan.mundodolphins.es"
	}
	if cfg.MinimumThreadPosts <= 0 {
		cfg.MinimumThreadPosts = 2
	}
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 1000
	}
	if cfg.OverlapPages <= 0 {
		cfg.OverlapPages = 2
	}
	statePath := filepath.Join(cfg.RepoRoot, "data", "bluesky-state.json")
	st, err := state.Load(statePath, cfg.Actor)
	if err != nil {
		return Result{}, err
	}
	full := cfg.Full || st.LastSync.IsZero()
	feed, err := im.fetchFeed(ctx, cfg.Actor, full, cfg.MaxPages, cfg.OverlapPages)
	if err != nil {
		return Result{}, err
	}
	res := Result{PostsFetched: len(feed)}
	roots := candidateRoots(feed)
	res.CandidateRoots = len(roots)
	w := content.Writer{Root: cfg.RepoRoot}
	seenURIs := map[string]string{}
	dirtyState := false
	for _, root := range roots {
		thread, err := im.client.GetPostThread(ctx, root.URI, 100)
		if err != nil {
			return res, fmt.Errorf("get thread %s: %w", root.URI, err)
		}
		own := ownThreadPosts(thread.Thread, root, authorKey(root.Author))
		if len(own) == 0 {
			own = []bluesky.Post{root}
		}
		sort.Slice(own, func(i, j int) bool { return own[i].Record.CreatedAt.Before(own[j].Record.CreatedAt) })
		entry := buildEntry(root, own, cfg.MinimumThreadPosts)
		mapping, ok := st.Roots[root.URI]
		previous := mapping
		if ok && mapping.Slug != "" {
			// Preserve URL forever even when title/text changes or short becomes article.
		} else {
			mapping.Slug = uniqueSlug(st, content.SlugFromText(root.Record.Text, content.RKey(root.URI), 70), root.URI)
			dirtyState = true
		}
		entry.Images, err = im.collectImages(ctx, cfg, mapping.Slug, entry, own)
		if err != nil {
			return res, err
		}
		wr := content.Unchanged
		if !cfg.DryRun {
			var err error
			wr, err = w.WriteEntry(mapping.Slug, entry)
			if err != nil {
				return res, err
			}
			switch wr {
			case content.Created:
				res.New++
			case content.Updated:
				res.Updated++
			case content.Unchanged:
				res.Unchanged++
			}
		}
		mapping.ContentType = entry.ContentType
		mapping.PostCount = entry.PostCount
		if wr != content.Unchanged || !ok || previous.ContentType != mapping.ContentType || previous.PostCount != mapping.PostCount || previous.Slug != mapping.Slug {
			mapping.LastSeen = im.now().UTC()
			st.Roots[root.URI] = mapping
			dirtyState = true
		}
		for _, post := range own {
			seenURIs[post.URI] = root.URI
			if st.ImportedURIs[post.URI] != root.URI {
				st.ImportedURIs[post.URI] = root.URI
				dirtyState = true
			}
		}
	}
	for uri, root := range seenURIs {
		if st.ImportedURIs[uri] != root {
			st.ImportedURIs[uri] = root
			dirtyState = true
		}
	}
	if st.Actor != cfg.Actor {
		st.Actor = cfg.Actor
		dirtyState = true
	}
	if dirtyState {
		st.LastSync = im.now().UTC()
	}
	if !cfg.DryRun && dirtyState {
		if err := state.Save(statePath, st); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (im *Importer) fetchFeed(ctx context.Context, actor string, full bool, maxPages, overlapPages int) ([]bluesky.Post, error) {
	var posts []bluesky.Post
	cursor := ""
	pages := maxPages
	if !full {
		pages = overlapPages
	}
	for page := 0; page < pages; page++ {
		resp, err := im.client.GetAuthorFeed(ctx, actor, cursor, 100)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Feed {
			posts = append(posts, item.Post)
		}
		if resp.Cursor == "" {
			break
		}
		cursor = resp.Cursor
	}
	return posts, nil
}

func candidateRoots(feed []bluesky.Post) []bluesky.Post {
	byURI := map[string]bluesky.Post{}
	for _, post := range feed {
		byURI[post.URI] = post
	}
	var roots []bluesky.Post
	for _, post := range feed {
		if post.Record.Reply != nil {
			if parent, ok := byURI[post.Record.Reply.Parent.URI]; ok && sameAuthor(parent, post) {
				continue
			}
		}
		roots = append(roots, post)
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Record.CreatedAt.Before(roots[j].Record.CreatedAt) })
	return roots
}

func ownThreadPosts(node bluesky.ThreadNode, root bluesky.Post, author string) []bluesky.Post {
	var out []bluesky.Post
	var walk func(bluesky.ThreadNode)
	walk = func(n bluesky.ThreadNode) {
		if n.Post.URI != "" && authorKey(n.Post.Author) == author {
			if n.Post.URI == root.URI || n.Post.Record.Reply == nil || n.Post.Record.Reply.Root.URI == root.URI || n.Post.Record.Reply.Parent.URI == root.URI {
				out = append(out, n.Post)
			}
		}
		for _, child := range n.Replies {
			walk(child)
		}
	}
	walk(node)
	seen := map[string]bool{}
	filtered := out[:0]
	for _, post := range out {
		if !seen[post.URI] {
			seen[post.URI] = true
			filtered = append(filtered, post)
		}
	}
	return filtered
}

func buildEntry(root bluesky.Post, posts []bluesky.Post, minPosts int) content.Entry {
	title := content.TitleFromText(root.Record.Text, root.Record.CreatedAt, 110)
	desc := content.DescriptionFromText(root.Record.Text, 170)
	typ := "short"
	if len(posts) >= minPosts {
		typ = "article"
	}
	var entryPosts []content.Post
	tagSet := map[string]bool{}
	var quotes []content.Quote
	for _, post := range posts {
		md, tags := content.MarkdownFromPost(post)
		entryPosts = append(entryPosts, content.Post{URI: post.URI, Text: post.Record.Text, Markdown: md, CreatedAt: post.Record.CreatedAt})
		for _, tag := range tags {
			tagSet[tag] = true
		}
		if post.Embed != nil && post.Embed.Record != nil && post.Embed.Record.Record != nil {
			q := post.Embed.Record.Record
			quotes = append(quotes, content.Quote{URL: content.BlueskyURL(q.Author.Handle, q.URI), Author: q.Author.Handle, Text: q.Record.Text})
		}
	}
	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	last := posts[len(posts)-1].Record.CreatedAt
	return content.Entry{
		Title:       title,
		Description: desc,
		ContentType: typ,
		Date:        root.Record.CreatedAt,
		LastMod:     last,
		BlueskyURL:  content.BlueskyURL(root.Author.Handle, root.URI),
		RootURI:     root.URI,
		RootCID:     root.CID,
		PostCount:   len(posts),
		Tags:        tags,
		Posts:       entryPosts,
		QuotedPosts: quotes,
	}
}

func (im *Importer) collectImages(ctx context.Context, cfg Config, slug string, entry content.Entry, posts []bluesky.Post) ([]content.Image, error) {
	var imgs []content.Image
	bundleDir := content.Writer{Root: cfg.RepoRoot}.BundleDir(slug, entry)
	for _, post := range posts {
		if post.Embed == nil {
			continue
		}
		for _, img := range post.Embed.Images {
			if len(imgs) >= 24 {
				return imgs, nil
			}
			src := img.Fullsize
			if src == "" {
				src = img.Thumb
			}
			if src == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(src))
			if ext == "" || len(ext) > 5 {
				ext = ".jpg"
			}
			name := fmt.Sprintf("image-%02d%s", len(imgs)+1, ext)
			out := filepath.Join(bundleDir, name)
			if !cfg.DryRun {
				downloaded, available, err := im.downloadImage(ctx, src, out)
				if err != nil {
					return imgs, err
				}
				if !available {
					continue
				}
				if downloaded {
					// The caller logs aggregate content changes; image count is kept in tests via files.
				}
			}
			width, height := 0, 0
			if img.AspectRatio != nil {
				width, height = img.AspectRatio.Width, img.AspectRatio.Height
			}
			imgs = append(imgs, content.Image{FileName: name, Alt: img.Alt, Source: src, Width: width, Height: height})
		}
	}
	return imgs, nil
}

func (im *Importer) downloadImage(ctx context.Context, src, path string) (downloaded bool, available bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, true, nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return false, false, err
		}
		req.Header.Set("User-Agent", "UnDolFan-BlueskyImporter/0.1")
		resp, err := im.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return false, false, ctx.Err()
			}
			if attempt == 2 {
				return false, false, nil
			}
			sleepRetry(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			defer resp.Body.Close()
			return im.writeImage(resp.Body, path)
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		if !transientImageStatus(resp.StatusCode) {
			return false, false, nil
		}
		if attempt < 2 {
			sleepRetry(ctx, attempt)
		}
	}
	return false, false, nil
}

func (im *Importer) writeImage(body io.Reader, path string) (bool, bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, false, err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false, false, err
	}
	_, copyErr := io.Copy(f, io.LimitReader(body, 16<<20))
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return false, false, copyErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return false, false, closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, false, err
	}
	return true, true, nil
}

func transientImageStatus(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout || code >= 500
}

func sleepRetry(ctx context.Context, attempt int) {
	timer := time.NewTimer(time.Duration(attempt+1) * 200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func uniqueSlug(st *state.State, base, rootURI string) string {
	if base == "" {
		base = "post-" + content.RKey(rootURI)
	}
	used := map[string]bool{}
	for uri, mapping := range st.Roots {
		if uri != rootURI {
			used[mapping.Slug] = true
		}
	}
	slug := base
	for i := 2; used[slug]; i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return slug
}

func sameAuthor(a, b bluesky.Post) bool {
	return authorKey(a.Author) == authorKey(b.Author)
}

func authorKey(a bluesky.Author) string {
	if a.DID != "" {
		return a.DID
	}
	return a.Handle
}
