package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteEntryUsesYearMonthBundleDirectory(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Title:       "Training camp",
		Description: "Resumen",
		ContentType: "short",
		Date:        time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC),
		LastMod:     time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC),
		RootURI:     "at://did:plc:author/app.bsky.feed.post/abc",
		PostCount:   1,
		Posts:       []Post{{Markdown: "Texto"}},
	}
	w := Writer{Root: dir}
	if _, err := w.WriteEntry("training-camp", entry); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "content", "posts", "2026", "08", "training-camp", "index.md")); err != nil {
		t.Fatal(err)
	}
}

func TestWriteEntryMigratesFlatBundle(t *testing.T) {
	dir := t.TempDir()
	oldDir := filepath.Join(dir, "content", "posts", "training-camp")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "image-01.jpg"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := Entry{
		Title:       "Training camp",
		Description: "Resumen",
		ContentType: "short",
		Date:        time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC),
		LastMod:     time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC),
		RootURI:     "at://did:plc:author/app.bsky.feed.post/abc",
		PostCount:   1,
		Posts:       []Post{{Markdown: "Texto"}},
	}
	if _, err := (Writer{Root: dir}).WriteEntry("training-camp", entry); err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(dir, "content", "posts", "2026", "08", "training-camp")
	if _, err := os.Stat(filepath.Join(newDir, "image-01.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old flat bundle still exists or unexpected error: %v", err)
	}
}
