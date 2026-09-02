package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests guard the repository layout itself, because the failure it
// prevents is invisible to every other test in the tree.
//
// `go install` names the binary after the directory holding package main. Move
// this package to the module root and `go install <module>@latest` starts
// producing a binary called "obsidian-vault-schema" — a name nothing invokes.
// The compile still succeeds, every behavioural test still passes, and
// `make install` still writes the right name, so nothing goes red; the only
// thing that breaks is the published install path, and a vault whose linter
// pass is guarded by `command -v obsidian-vault-lint` then skips in silence
// with a green `make check`. The tests below are what goes red instead.

// moduleRoot returns the repository root and asserts it really is one. A test
// binary runs with its own package directory as the working directory, so the
// root is two levels up from cmd/<name>/.
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected the module root two levels above %s, found no go.mod in %s: %v", wd, root, err)
	}
	return root
}

// TestBinaryNameComesFromThisDirectory pins the one fact that decides what the
// installed binary is called.
func TestBinaryNameComesFromThisDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	const want = "obsidian-vault-lint"
	if got := filepath.Base(wd); got != want {
		t.Fatalf("package main lives in %q, so `go install` would name the binary %q, not %q.\n"+
			"Every Makefile in this estate invokes the bare name %q, and a vault's linter pass is\n"+
			"guarded by `command -v %s` — under any other name that pass skips in silence and the\n"+
			"vault reports a green check with the Go validator never having run.",
			got, got, want, want, want)
	}
}

// TestModuleRootHasNoGoPackage keeps the module root empty of Go source.
//
// A *library* package at the root is not inert: `go build -o FILE .` against
// one exits 0 and writes a non-executable ar archive to FILE rather than
// refusing. Pointed at a bindir — what an installer does — that silently
// replaces the tool on PATH with an unrunnable file, `command -v` stops
// finding it, and the guarded linter pass goes quiet. With no package here the
// same command fails loudly instead.
func TestModuleRootHasNoGoPackage(t *testing.T) {
	root := moduleRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			found = append(found, e.Name())
		}
	}
	if len(found) > 0 {
		t.Fatalf("the module root %s must hold no Go source, found: %s\n"+
			"A package main here installs under the module's name, not the binary's.\n"+
			"A library package here makes `go build -o BINDIR/obsidian-vault-lint .` exit 0 while\n"+
			"writing a non-executable archive over the installed tool. Keep the CLI in\n"+
			"cmd/obsidian-vault-lint/ and the library in lint/.",
			root, strings.Join(found, ", "))
	}
}

// TestREADMEDocumentsTheRealInstallPath fails when the documented one-command
// install stops matching the layout — the drift that hands a user an install
// they think worked.
func TestREADMEDocumentsTheRealInstallPath(t *testing.T) {
	root := moduleRoot(t)

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	var modulePath string
	for _, line := range strings.Split(string(gomod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			modulePath = strings.TrimSpace(rest)
			break
		}
	}
	if modulePath == "" {
		t.Fatal("go.mod declares no module path")
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	want := modulePath + "/cmd/" + filepath.Base(wd) + "@latest"

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), want) {
		t.Fatalf("README.md does not document the install path this layout actually produces:\n"+
			"  go install %s\n"+
			"Whatever it documents instead either installs nothing or installs the wrong name.", want)
	}
}
