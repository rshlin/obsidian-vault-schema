package main

import "testing"

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
