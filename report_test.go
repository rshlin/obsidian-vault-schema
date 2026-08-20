package main

import "testing"

func TestReportFinishReturnsExitCode(t *testing.T) {
	r := &Report{Title: "schema", Checked: 3}
	if code := r.Finish(); code != 0 {
		t.Fatalf("expected 0 with no errors, got %d", code)
	}

	r = &Report{Title: "schema", Checked: 3}
	r.Errorf("a.md", "bad %s", "field")
	if code := r.Finish(); code != 1 {
		t.Fatalf("expected 1 with an error present, got %d", code)
	}
}

func TestReportWarnDoesNotAffectExitCode(t *testing.T) {
	r := &Report{Title: "schema", Checked: 1}
	r.Warnf("a.md", "just a warning")
	if code := r.Finish(); code != 0 {
		t.Fatalf("expected 0 with only a warning, got %d", code)
	}
}
