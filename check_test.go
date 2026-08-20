package main

import "testing"

func TestRunVaultCheckReportsGoodAndBad(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "", `\{\{.*?\}\}`)
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 5 {
		t.Fatalf("expected 5 checked, got %d", report.Checked)
	}
	// bad-reference.md (bad status) + bad-spec-missing-pr.md (missing pr) +
	// template-reference.md (bad status, not relaxed here) = at least 3 files with issues.
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
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "type", "templates", `\{\{.*?\}\}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Errors {
		if f.Path == "templates/template-reference.md" {
			t.Fatalf("relaxed mode should have suppressed the placeholder status violation, got: %s", f.Message)
		}
	}
}

func TestRunVaultCheckUnknownDiscriminatorField(t *testing.T) {
	report, err := RunVaultCheck("testdata/vault", "testdata/schemas", "nonexistent_field", "", `\{\{.*?\}\}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != report.Checked {
		t.Fatalf("expected every file to error on a missing discriminator field, got %d/%d", len(report.Errors), report.Checked)
	}
}
