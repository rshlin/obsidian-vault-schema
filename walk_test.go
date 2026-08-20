package main

import "testing"

func TestWalkMarkdownFindsAllNotes(t *testing.T) {
	paths, err := walkMarkdown("testdata/vault")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 5 {
		t.Fatalf("expected 5 notes, got %d: %v", len(paths), paths)
	}
}

func TestWalkMarkdownSkipsDotDirs(t *testing.T) {
	paths, err := walkMarkdown("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if p == "testdata/.hidden/note.md" {
			t.Fatal("expected dot-directories to be skipped")
		}
	}
}
