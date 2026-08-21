package main

import (
	"strings"
	"testing"
)

func TestRunVaultCheckReportsGoodAndBad(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 8 {
		t.Fatalf("expected 8 checked, got %d", report.Checked)
	}
	// bad-reference.md (bad status), bad-spec-missing-pr.md (missing pr),
	// template-reference.md (bad status, not relaxed here), template-missing-summary.md
	// (missing summary), plugin/some-plugin.md (non-note frontmatter) and NOTES.md (no
	// frontmatter at all) all have issues.
	if len(report.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	for _, f := range report.Errors {
		if f.Path == "good-reference.md" || f.Path == "good-spec-in-progress.md" {
			t.Fatalf("did not expect an error on %s: %s", f.Path, f.Message)
		}
	}
}

func TestRunVaultCheckRelaxedSuppressesPlaceholderOnly(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "templates", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Errors {
		if f.Path == "templates/template-reference.md" {
			t.Fatalf("relaxed mode should have suppressed the placeholder status violation, got: %s", f.Message)
		}
	}
}

func TestRunVaultCheckRelaxedDoesNotSuppressMissingRequired(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "templates", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var foundMissingSummary, foundPlaceholder bool
	for _, f := range report.Errors {
		if f.Path == "templates/template-missing-summary.md" {
			foundMissingSummary = true
			if strings.HasPrefix(f.Message, ": ") {
				t.Fatalf("expected no stray leading colon on a root-located issue, got %q", f.Message)
			}
		}
		if f.Path == "templates/template-reference.md" {
			foundPlaceholder = true
		}
	}
	if !foundMissingSummary {
		t.Fatal("expected relaxed mode to still report the missing summary field — required-field presence is never suppressed, even inside a relaxed directory")
	}
	if foundPlaceholder {
		t.Fatal("expected relaxed mode to still suppress the placeholder status violation on template-reference.md")
	}
}

func TestRunVaultCheckUnknownDiscriminatorField(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "nonexistent_field", "", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != report.Checked {
		t.Fatalf("expected every file to error on a missing discriminator field, got %d/%d", len(report.Errors), report.Checked)
	}
}

func TestRunVaultCheckExcludeDirSkipsMatchedDirEntirely(t *testing.T) {
	before, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var foundBefore bool
	for _, f := range before.Errors {
		if f.Path == "plugin/some-plugin.md" {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatal("expected plugin/some-plugin.md to error without --exclude-dir")
	}

	after, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`, []string{"plugin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range after.Errors {
		if f.Path == "plugin/some-plugin.md" {
			t.Fatal("expected plugin/some-plugin.md to be excluded by --exclude-dir")
		}
	}
	if after.Checked != before.Checked-1 {
		t.Fatalf("expected Checked to drop by exactly 1 when excluding plugin/, got %d vs %d", after.Checked, before.Checked)
	}
}

func TestRunVaultCheckExcludeFileSkipsMatchedPathEntirely(t *testing.T) {
	before, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var foundBefore bool
	for _, f := range before.Errors {
		if f.Path == "NOTES.md" {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatal("expected NOTES.md to error without --exclude-file")
	}

	after, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`, nil, []string{"NOTES.md"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range after.Errors {
		if f.Path == "NOTES.md" {
			t.Fatal("expected NOTES.md to be excluded by --exclude-file")
		}
	}
	if after.Checked != before.Checked-1 {
		t.Fatalf("expected Checked to drop by exactly 1 when excluding NOTES.md, got %d vs %d", after.Checked, before.Checked)
	}
}

func TestRunFilesCheckOverlay(t *testing.T) {
	report, err := RunFilesCheck("testdata/overlay/*.md", "testdata/overlay/_code-task-overlay.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 2 {
		t.Fatalf("expected 2 checked, got %d", report.Checked)
	}
	var badErrored bool
	for _, f := range report.Errors {
		if f.Path == "testdata/overlay/good-task.md" {
			t.Fatalf("did not expect an error on good-task.md: %s", f.Message)
		}
		if f.Path == "testdata/overlay/bad-task.md" {
			badErrored = true
		}
	}
	if !badErrored {
		t.Fatal("expected bad-task.md (bad task_id pattern, bad task_status) to error")
	}
}

func TestRunFilesCheckNoMatches(t *testing.T) {
	report, err := RunFilesCheck("testdata/overlay/no-such-*.md", "testdata/overlay/_code-task-overlay.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected an empty, passing report for zero matches, got %+v", report)
	}
}
