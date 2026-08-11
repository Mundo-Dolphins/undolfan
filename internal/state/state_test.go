package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCorruptState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{nope`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, "actor"); err == nil {
		t.Fatalf("expected corrupt state error")
	}
}

func TestSaveIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := New("actor")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("state changed without data change")
	}
}
