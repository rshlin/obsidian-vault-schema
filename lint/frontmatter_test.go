package lint

import "testing"

func TestSplitFrontmatter(t *testing.T) {
	data := []byte("---\ntype: reference\ntitle: Foo\n---\n# Body\n")
	raw, body, ok := splitFrontmatter(data)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if string(raw) != "type: reference\ntitle: Foo" {
		t.Fatalf("raw = %q", raw)
	}
	if string(body) != "# Body\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestSplitFrontmatterMissing(t *testing.T) {
	_, _, ok := splitFrontmatter([]byte("# no frontmatter\n"))
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestParseFrontmatterBadYAML(t *testing.T) {
	_, err := parseFrontmatter([]byte("type: [unterminated"))
	if err == nil {
		t.Fatal("expected an error for invalid YAML")
	}
}

func TestParseFrontmatterEmpty(t *testing.T) {
	_, err := parseFrontmatter([]byte("# just a comment\n"))
	if err == nil {
		t.Fatal("expected an error for empty frontmatter")
	}
}

func TestToInstanceNormalizesDate(t *testing.T) {
	raw := []byte("type: reference\ncreated: 2026-08-18\nupdated: \"2026-08-18\"\n")
	m, err := parseFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := toInstance(m)
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := inst.(map[string]interface{})
	if !ok {
		t.Fatalf("instance is %T, want map[string]interface{}", inst)
	}
	if obj["created"] != "2026-08-18" {
		t.Fatalf("created = %#v, want normalized string \"2026-08-18\"", obj["created"])
	}
	if obj["updated"] != "2026-08-18" {
		t.Fatalf("updated = %#v, want \"2026-08-18\"", obj["updated"])
	}
}

func TestToInstanceNormalizesNestedDate(t *testing.T) {
	raw := []byte("type: reference\nnested:\n  when: 2026-01-02\nlist:\n  - 2026-01-03\n  - plain\n")
	m, err := parseFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := toInstance(m)
	if err != nil {
		t.Fatal(err)
	}
	obj := inst.(map[string]interface{})
	nested := obj["nested"].(map[string]interface{})
	if nested["when"] != "2026-01-02" {
		t.Fatalf("nested.when = %#v", nested["when"])
	}
	list := obj["list"].([]interface{})
	if list[0] != "2026-01-03" {
		t.Fatalf("list[0] = %#v", list[0])
	}
}
