package main

import "testing"

func TestWalkMarkdownFindsAllNotes(t *testing.T) {
	paths, err := walkMarkdown("testdata/vault")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 7 {
		t.Fatalf("expected 7 notes, got %d: %v", len(paths), paths)
	}
}

func TestWalkMarkdownExcludingSkipsNamedDirAtAnyDepth(t *testing.T) {
	paths, err := walkMarkdownExcluding("testdata/vault", map[string]bool{"plugin": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if p == "testdata/vault/plugin/some-plugin.md" {
			t.Fatal("expected the plugin/ directory to be excluded")
		}
	}
}

func TestWalkMarkdownExcludingNilMapMatchesWalkMarkdown(t *testing.T) {
	excluding, err := walkMarkdownExcluding("testdata/vault", nil)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := walkMarkdown("testdata/vault")
	if err != nil {
		t.Fatal(err)
	}
	if len(excluding) != len(plain) {
		t.Fatalf("expected walkMarkdownExcluding(nil) to match walkMarkdown, got %d vs %d", len(excluding), len(plain))
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
