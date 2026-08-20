package main

import (
	"regexp"
	"testing"
)

func TestFlattenValidationErrorDropsRootSummary(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	sch, err := s.Compile("reference")
	if err != nil {
		t.Fatal(err)
	}
	err = sch.Validate(map[string]interface{}{"type": "reference", "status": "draft"})
	issues := flattenValidationError(err)
	if len(issues) == 0 {
		t.Fatal("expected at least one issue")
	}
	for _, iss := range issues {
		if iss.Message == "" {
			t.Fatalf("issue with empty message: %#v", iss)
		}
	}
}

func TestFlattenValidationErrorNil(t *testing.T) {
	if issues := flattenValidationError(nil); issues != nil {
		t.Fatalf("expected nil, got %#v", issues)
	}
}

func TestResolvePointer(t *testing.T) {
	instance := map[string]interface{}{
		"status":  "draft",
		"sources": []interface{}{"agent", "human"},
	}
	v, ok := resolvePointer(instance, "/status")
	if !ok || v != "draft" {
		t.Fatalf("resolvePointer(/status) = %#v, %v", v, ok)
	}
	v, ok = resolvePointer(instance, "/sources/1")
	if !ok || v != "human" {
		t.Fatalf("resolvePointer(/sources/1) = %#v, %v", v, ok)
	}
	_, ok = resolvePointer(instance, "/nope")
	if ok {
		t.Fatal("expected ok=false for a missing key")
	}
	_, ok = resolvePointer(instance, "/sources/9")
	if ok {
		t.Fatal("expected ok=false for an out-of-range index")
	}
	v, ok = resolvePointer(instance, "")
	if !ok || v.(map[string]interface{})["status"] != "draft" {
		t.Fatalf("resolvePointer(\"\") should return the root")
	}
}

func TestIsPlaceholderAt(t *testing.T) {
	instance := map[string]interface{}{"status": "{{status}}", "title": "Real Title"}
	re := regexp.MustCompile(`\{\{.*?\}\}`)
	if !isPlaceholderAt(instance, "/status", re) {
		t.Fatal("expected /status to be recognized as a placeholder")
	}
	if isPlaceholderAt(instance, "/title", re) {
		t.Fatal("expected /title to NOT be recognized as a placeholder")
	}
	if isPlaceholderAt(instance, "/missing", re) {
		t.Fatal("expected a missing pointer to be false, not a panic")
	}
}
