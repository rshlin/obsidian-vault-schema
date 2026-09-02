package lint

import (
	"os"
	"path/filepath"
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
	report, err := RunFilesCheck("testdata/overlay/*.md", "testdata/overlay/_task-overlay.schema.json")
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
	report, err := RunFilesCheck("testdata/overlay/no-such-*.md", "testdata/overlay/_task-overlay.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected an empty, passing report for zero matches, got %+v", report)
	}
}

// End-to-end proof of filesystem containment: a note may not reach a schema
// outside --schemas by writing traversal into its discriminator. The temp
// layout deliberately puts a *loadable* schema one level above the schemas
// directory, so a regression here would not merely error differently — it
// would silently pass.
func TestRunVaultCheckRefusesTypeThatEscapesSchemasDir(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	schemas := filepath.Join(root, "schemas")
	for _, d := range []string{vault, schemas} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	permissive := `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`
	write(filepath.Join(schemas, "reference.schema.json"), permissive)
	// Reachable as "../pwned" from the schemas directory, and nowhere else.
	write(filepath.Join(root, "pwned.schema.json"), permissive)

	note := func(typeValue string) string {
		return "---\ntype: " + typeValue + "\ntitle: t\nstatus: draft\nsummary: s\n---\n"
	}
	write(filepath.Join(vault, "escape.md"), note("../pwned"))
	write(filepath.Join(vault, "passwd.md"), note("../../../etc/passwd"))
	write(filepath.Join(vault, "absolute.md"), note("/etc/passwd"))
	write(filepath.Join(vault, "good.md"), note("reference"))

	report, err := RunVaultCheck(vault, schemas, "type", "", `\{\{.*?\}\}`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 4 {
		t.Fatalf("expected 4 checked, got %d", report.Checked)
	}
	if len(report.Errors) != 3 {
		t.Fatalf("expected exactly the 3 traversal notes to error, got %d: %+v", len(report.Errors), report.Errors)
	}
	for _, f := range report.Errors {
		if f.Path == "good.md" {
			t.Fatalf("a well-formed type must still resolve: %s", f.Message)
		}
		if !strings.Contains(f.Message, "not a usable schema name") {
			t.Fatalf("expected %s to be refused by name, got: %s", f.Path, f.Message)
		}
		// The message names the offending field, and never a path the value
		// managed to build.
		if !strings.HasPrefix(f.Message, `"type: `) {
			t.Fatalf("expected the message to quote the offending field, got: %s", f.Message)
		}
	}
}
