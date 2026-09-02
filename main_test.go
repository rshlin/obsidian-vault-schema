package main

import "testing"

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
	code := run([]string{"check", "--vault", "testdata/vault", "--schemas", "testdata/schemas", "--files", "x", "--schema", "y"})
	if code != 2 {
		t.Fatalf("expected exit code 2 when both modes' flags are set, got %d", code)
	}
}

func TestRunCheckVaultMode(t *testing.T) {
	code := run([]string{"check", "--vault", "testdata/vault", "--schemas", "testdata/schemas"})
	if code != 1 {
		t.Fatalf("expected exit code 1 (testdata/vault has known-bad fixtures), got %d", code)
	}
}

func TestRunCheckFilesMode(t *testing.T) {
	code := run([]string{
		"check", "--files", "testdata/overlay/good-task.md",
		"--schema", "testdata/overlay/_task-overlay.schema.json",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0 for the good task fixture alone, got %d", code)
	}
}
