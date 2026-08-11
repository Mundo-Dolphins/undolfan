package content

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type WriteResult string

const (
	Unchanged WriteResult = "unchanged"
	Created   WriteResult = "created"
	Updated   WriteResult = "updated"
)

type Writer struct {
	Root string
}

func (w Writer) WriteEntry(slug string, entry Entry) (WriteResult, error) {
	cleanSlug := SlugFromText(slug, RKey(entry.RootURI), 120)
	if cleanSlug != slug {
		return "", fmt.Errorf("unsafe slug %q", slug)
	}
	dir := w.BundleDir(slug, entry)
	if err := w.migrateFlatBundle(slug, dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "index.md")
	body := render(slug, entry)
	old, err := os.ReadFile(path)
	existed := err == nil
	if existed && bytes.Equal(old, body) {
		return Unchanged, nil
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	if existed {
		return Updated, nil
	}
	return Created, nil
}

func (w Writer) BundleDir(slug string, entry Entry) string {
	return filepath.Join(w.Root, "content", "posts", entry.Date.Format("2006"), entry.Date.Format("01"), slug)
}

func (w Writer) migrateFlatBundle(slug, newDir string) error {
	oldDir := filepath.Join(w.Root, "content", "posts", slug)
	if oldDir == newDir {
		return nil
	}
	if _, err := os.Stat(oldDir); err != nil {
		return nil
	}
	if _, err := os.Stat(newDir); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return err
	}
	return os.Rename(oldDir, newDir)
}

func render(slug string, e Entry) []byte {
	var b bytes.Buffer
	b.WriteString("---\n")
	yamlString(&b, "title", e.Title)
	yamlString(&b, "slug", slug)
	b.WriteString("date: " + e.Date.Format("2006-01-02T15:04:05-07:00") + "\n")
	b.WriteString("lastmod: " + e.LastMod.Format("2006-01-02T15:04:05-07:00") + "\n")
	yamlString(&b, "description", e.Description)
	b.WriteString("draft: false\n")
	yamlString(&b, "content_type", e.ContentType)
	yamlString(&b, "bluesky_url", e.BlueskyURL)
	yamlString(&b, "bluesky_root_uri", e.RootURI)
	if e.RootCID != "" {
		yamlString(&b, "bluesky_root_cid", e.RootCID)
	}
	b.WriteString(fmt.Sprintf("bluesky_post_count: %d\n", e.PostCount))
	writeList(&b, "tags", e.Tags)
	var imageNames []string
	for _, img := range e.Images {
		imageNames = append(imageNames, img.FileName)
	}
	writeList(&b, "images", imageNames)
	b.WriteString("---\n\n")
	for i, post := range e.Posts {
		if i > 0 {
			b.WriteString("\n\n")
			b.WriteString("---\n\n")
		}
		b.WriteString(strings.TrimSpace(post.Markdown))
		b.WriteString("\n")
	}
	for _, img := range e.Images {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("![%s](%s)\n", escapeAlt(img.Alt), img.FileName))
	}
	for _, quote := range e.QuotedPosts {
		if quote.URL == "" && quote.Text == "" {
			continue
		}
		b.WriteString("\n> ")
		if quote.Text != "" {
			b.WriteString(strings.ReplaceAll(strings.TrimSpace(quote.Text), "\n", "\n> "))
		}
		if quote.URL != "" {
			b.WriteString("\n>\n> [Post citado en Bluesky](" + quote.URL + ")")
		}
		b.WriteString("\n")
	}
	return b.Bytes()
}

func yamlString(b *bytes.Buffer, key, value string) {
	b.WriteString(key + ": " + quoteYAML(value) + "\n")
}

func quoteYAML(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return "\"" + value + "\""
}

func writeList(b *bytes.Buffer, key string, values []string) {
	b.WriteString(key + ":")
	if len(values) == 0 {
		b.WriteString(" []\n")
		return
	}
	sort.Strings(values)
	b.WriteString("\n")
	for _, value := range values {
		b.WriteString("  - " + quoteYAML(value) + "\n")
	}
}

func escapeAlt(s string) string {
	return strings.ReplaceAll(s, "]", "\\]")
}
