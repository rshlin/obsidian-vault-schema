package lint

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

func TestFlattenValidationErrorDropsContainerEntries(t *testing.T) {
	s := NewSchemaSet("testdata/schemas")
	sch, err := s.Compile("reference")
	if err != nil {
		t.Fatal(err)
	}
	err = sch.Validate(map[string]interface{}{
		"type": "reference", "title": "x", "status": "not-a-real-status", "summary": "y",
	})
	issues := flattenValidationError(err)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 leaf issue (the status enum failure), got %d: %#v", len(issues), issues)
	}
	if issues[0].Location != "/status" {
		t.Fatalf("expected the single issue to be located at /status, got %q", issues[0].Location)
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

func TestValidationIssueFormat(t *testing.T) {
	root := validationIssue{Location: "", Message: "missing properties: 'title', 'summary'"}
	if got := root.format(); got != "missing properties: 'title', 'summary'" {
		t.Fatalf("expected a root-located issue to format without a location prefix, got %q", got)
	}

	field := validationIssue{Location: "/status", Message: "value must be one of 'draft', 'review', 'canonical'"}
	if got := field.format(); got != "/status: value must be one of 'draft', 'review', 'canonical'" {
		t.Fatalf("expected a field-located issue to include its location, got %q", got)
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
