package main

import "testing"

// A `go test` binary runs with its own package directory as the working
// directory, so the library's fixtures are reached from here by relative path.
// They are not copied into this directory: the CLI and the library must be
// exercised against the same notes, or a divergence between the two hides
// behind two copies of the truth.
const testdata = "../../lint/testdata"

func TestRunNoArgs(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Fatalf("expected exit code 2 for no args, got %d", code)
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	if code := run([]string{"bogus"}); code != 2 {
		t.Fatalf("expected exit code 2 for an unknown subcommand, got %d", code)
	}
}

func TestRunCheckRequiresAMode(t *testing.T) {
	if code := run([]string{"check"}); code != 2 {
		t.Fatalf("expected exit code 2 with neither mode's flags set, got %d", code)
	}
}

func TestRunCheckRejectsMixedModes(t *testing.T) {
	code := run([]string{"check", "--vault", testdata + "/vault", "--schemas", testdata + "/schemas", "--files", "x", "--schema", "y"})
	if code != 2 {
		t.Fatalf("expected exit code 2 when both modes' flags are set, got %d", code)
	}
}

func TestRunCheckVaultMode(t *testing.T) {
	code := run([]string{"check", "--vault", testdata + "/vault", "--schemas", testdata + "/schemas"})
	if code != 1 {
		t.Fatalf("expected exit code 1 (the vault fixture has known-bad notes), got %d", code)
	}
}

func TestRunCheckFilesMode(t *testing.T) {
	code := run([]string{
		"check", "--files", testdata + "/overlay/good-task.md",
		"--schema", testdata + "/overlay/_task-overlay.schema.json",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0 for the good task fixture alone, got %d", code)
	}
}
