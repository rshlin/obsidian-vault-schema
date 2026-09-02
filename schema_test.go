package main

import (
	"strings"
	"testing"
)

func TestSchemaSetCompilesTypeMergedWithBase(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	sch, err := s.Compile("reference")
	if err != nil {
		t.Fatal(err)
	}
	// Missing "summary" (required by base) must fail.
	err = sch.Validate(map[string]interface{}{"type": "reference", "title": "x", "status": "draft"})
	if err == nil {
		t.Fatal("expected an error: base's required 'summary' should still apply")
	}
}

func TestSchemaSetCachesCompiledSchema(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	a, err := s.Compile("reference")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Compile("reference")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("expected Compile to return the cached schema on the second call")
	}
}

func TestSchemaSetUnknownType(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	_, err := s.Compile("no-such-type")
	if err == nil {
		t.Fatal("expected an error for a type with no schema file")
	}
}

func TestSchemaSetIfThen(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	sch, err := s.Compile("spec")
	if err != nil {
		t.Fatal(err)
	}
	instance := map[string]interface{}{
		"type": "spec", "title": "x", "status": "draft", "summary": "y",
		"spec_status": "in_review", "pr": nil,
	}
	if err := sch.Validate(instance); err == nil {
		t.Fatal("expected an error: pr required once spec_status is in_review")
	}
	instance["spec_status"] = "in_progress"
	if err := sch.Validate(instance); err != nil {
		t.Fatalf("in_progress with no pr should be valid, got %v", err)
	}
}

func TestCompileSchemaFile(t *testing.T) {
	sch, err := CompileSchemaFile("testdata/schemas/reference.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	// No base merge in this mode: "summary" is NOT required here.
	if err := sch.Validate(map[string]interface{}{"type": "reference"}); err != nil {
		t.Fatalf("expected no error without base merge, got %v", err)
	}
}

// A discriminator value arrives from hand-written frontmatter and is turned
// into a filename. Every value that is not a bare schema name must be refused
// by name, before Compile builds a path from it.
func TestSchemaSetRejectsUnusableTypeNames(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	for _, bad := range []string{
		"../../../etc/passwd", // the traversal this guard exists for
		"..",
		"../reference",
		"sub/reference",
		`..\reference`,
		"/etc/passwd",
		"/reference",
		"reference.schema",
		".hidden",
		"Reference",   // uppercase
		"refer ence",  // whitespace
		"reference_x", // underscore
		"-leading",
		"1leading",
		"",
	} {
		sch, err := s.Compile(bad)
		if err == nil {
			t.Fatalf("expected %q to be refused as a schema name, got a compiled schema", bad)
		}
		if sch != nil {
			t.Fatalf("expected no schema alongside the error for %q", bad)
		}
		if !strings.Contains(err.Error(), "not a usable schema name") {
			t.Fatalf("expected %q to be refused by name before any filesystem access, got: %v", bad, err)
		}
	}
}

func TestSchemaSetAcceptsWellFormedTypeNames(t *testing.T) {
	for _, good := range []string{"reference", "spec", "a", "how-to", "adr-0001", "x9"} {
		if !ValidTypeName(good) {
			t.Fatalf("expected %q to be a usable schema name", good)
		}
	}
}
