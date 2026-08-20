package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// splitFrontmatter separates a note's leading "---\n...\n---" YAML block from
// its body. ok is false when the file has no frontmatter at all.
func splitFrontmatter(data []byte) (raw []byte, body []byte, ok bool) {
	const fence = "---"
	if !bytes.HasPrefix(data, []byte(fence)) {
		return nil, data, false
	}
	lines := bytes.Split(data, []byte("\n"))
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSpace(lines[i]), []byte(fence)) {
			raw = bytes.Join(lines[1:i], []byte("\n"))
			body = bytes.Join(lines[i+1:], []byte("\n"))
			return raw, body, true
		}
	}
	return nil, data, false
}

// parseFrontmatter parses the raw YAML block into a plain map.
func parseFrontmatter(raw []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("frontmatter is empty")
	}
	return m, nil
}

// normalizeYAML walks a value produced by yaml.Unmarshal and rewrites
// anything encoding/json cannot represent — currently just time.Time, which
// yaml.v3 produces for bare-date and bare-timestamp scalars — into a
// JSON-safe equivalent. Dates round-trip as "2006-01-02" so a JSON Schema
// "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}$" still matches them.
func normalizeYAML(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, vv := range x {
			out[k] = normalizeYAML(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, vv := range x {
			out[i] = normalizeYAML(vv)
		}
		return out
	case time.Time:
		return x.Format("2006-01-02")
	default:
		return x
	}
}

// toInstance converts a parsed frontmatter map into exactly the
// representation santhosh-tekuri/jsonschema expects: types produced by
// encoding/json (float64 for numbers, map[string]interface{},
// []interface{}, string, bool, nil). The json round-trip after
// normalizeYAML is what guarantees that, rather than relying on yaml.v3's
// native decoding to happen to already match.
func toInstance(m map[string]interface{}) (interface{}, error) {
	b, err := json.Marshal(normalizeYAML(m))
	if err != nil {
		return nil, fmt.Errorf("frontmatter is not JSON-representable: %w", err)
	}
	var instance interface{}
	if err := json.Unmarshal(b, &instance); err != nil {
		return nil, err
	}
	return instance, nil
}
