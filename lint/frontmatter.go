package lint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// utf8BOM is the byte-order mark some editors leave in front of a note's
// opening "---". It is invisible, so without stripping it a note that plainly
// does start with frontmatter is reported as having none, which sends its
// author to look at the one thing that is not wrong. The primary Python
// validator reads notes as utf-8-sig, which drops it; this matches.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// splitFrontmatter separates a note's leading "---\n...\n---" YAML block from
// its body. ok is false when the file has no frontmatter at all.
func splitFrontmatter(data []byte) (raw []byte, body []byte, ok bool) {
	const fence = "---"
	data = bytes.TrimPrefix(data, utf8BOM)
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

// maxYAMLDepth bounds the node walk. YAML permits an alias that refers to a
// node still being built (`&a [*a]`), which is a cycle in the node graph and
// an unbounded walk here. A depth limit turns that into an error instead of a
// hang.
const maxYAMLDepth = 100

// parseFrontmatter parses the raw YAML block into a plain map.
//
// The document is walked as a yaml.Node tree rather than unmarshalled straight
// into a map, because the raw text of every plain scalar is needed: yaml.v3
// and PyYAML resolve bare words differently, and the Python validator is the
// specification. See scalar.go.
func parseFrontmatter(raw []byte) (map[string]interface{}, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, fmt.Errorf("frontmatter is empty")
	}
	value, err := nodeValue(doc.Content[0], 0)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("frontmatter is empty")
	}
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("frontmatter must be a mapping, got %s", jsonTypeName(value))
	}
	return m, nil
}

// nodeValue converts one YAML node into the plain Go value the Python
// validator would see for it.
func nodeValue(n *yaml.Node, depth int) (interface{}, error) {
	if depth > maxYAMLDepth {
		return nil, fmt.Errorf("frontmatter is nested more than %d levels deep, "+
			"or contains a recursive alias", maxYAMLDepth)
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return nodeValue(n.Content[0], depth+1)

	case yaml.AliasNode:
		if n.Alias == nil {
			return nil, fmt.Errorf("frontmatter is not valid YAML: unresolved alias %q", n.Value)
		}
		return nodeValue(n.Alias, depth+1)

	case yaml.ScalarNode:
		// An explicit tag wins over both the style and the plain resolver:
		// PyYAML's constructor is chosen by tag, so `!!int "42"` is an int
		// despite the quotes and `!!str 42` is text despite looking numeric.
		if explicitlyTagged(n) {
			return taggedValue(n.Tag, n.Value)
		}
		// Style 0 is a plain scalar. Anything quoted, literal or folded is a
		// string in both implementations and is never re-resolved.
		if n.Style != 0 {
			return n.Value, nil
		}
		return resolvePlain(n.Value)

	case yaml.SequenceNode:
		if explicitlyTagged(n) {
			switch n.Tag {
			case "!!seq":
			case "!!omap", "!!pairs":
				// PyYAML yields a list of (key, value) tuples, which its
				// validator then normalises into two-element lists.
				return pairsValue(n, depth)
			default:
				return nil, unknownTagError(n.Tag)
			}
		}
		out := make([]interface{}, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := nodeValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil

	case yaml.MappingNode:
		if explicitlyTagged(n) {
			switch n.Tag {
			case "!!map":
			case "!!set":
				// PyYAML builds a Python set, which is not a JSON value.
				return nonJSON{PyName: "set"}, nil
			default:
				return nil, unknownTagError(n.Tag)
			}
		}
		return mappingValue(n, depth)
	}
	return nil, fmt.Errorf("frontmatter is not valid YAML: unsupported node")
}

// explicitlyTagged reports whether the author wrote a tag on this node.
// yaml.v3 also fills in Tag for untagged nodes with the result of its own
// YAML-1.2 resolution — "!!int" for a bare 42, "!!timestamp" for a bare date —
// and honouring those would reintroduce exactly the resolver drift scalar.go
// exists to remove. Only yaml.TaggedStyle distinguishes the two.
func explicitlyTagged(n *yaml.Node) bool {
	return n.Style&yaml.TaggedStyle != 0 && n.Tag != ""
}

// pairsValue builds the shape PyYAML gives an "!!omap" or "!!pairs": a list of
// (key, value) pairs, which the Python validator's normalise() then flattens
// into two-element lists. Producing a list of one-key maps instead — which is
// what an untagged read of the same YAML gives — would make the two validators
// see different shapes for the same note.
func pairsValue(n *yaml.Node, depth int) (interface{}, error) {
	out := make([]interface{}, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return nil, fmt.Errorf("frontmatter is not valid YAML: expected a "+
				"single mapping of key to value in %s", n.Tag)
		}
		k, err := nodeValue(item.Content[0], depth+1)
		if err != nil {
			return nil, err
		}
		v, err := nodeValue(item.Content[1], depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, []interface{}{k, v})
	}
	return out, nil
}

func mappingValue(n *yaml.Node, depth int) (interface{}, error) {
	out := make(map[string]interface{}, len(n.Content)/2)
	// Merge keys are applied after the mapping's own keys, because an explicit
	// key wins over a merged one — YAML 1.1 merge semantics, which PyYAML
	// implements and this vault's notes may rely on.
	var merges []*yaml.Node

	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode, valNode := n.Content[i], n.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Style == 0 && keyNode.Value == "<<" {
			merges = append(merges, valNode)
			continue
		}
		rawKey, err := nodeValue(keyNode, depth+1)
		if err != nil {
			return nil, err
		}
		key := pythonStr(rawKey)
		if _, exists := out[key]; exists {
			// yaml.v3 rejects a duplicate key and PyYAML silently keeps the
			// last one, so the vault's Python loader was taught to reject it
			// too: a key written twice is data being dropped, not a style
			// preference.
			return nil, fmt.Errorf("line %d: duplicate key %q in frontmatter — "+
				"one of the two values is silently discarded", keyNode.Line, key)
		}
		v, err := nodeValue(valNode, depth+1)
		if err != nil {
			return nil, err
		}
		out[key] = v
	}

	for _, m := range merges {
		if err := applyMerge(out, m, depth); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// applyMerge folds a "<<" value into an existing mapping. The value is either
// one mapping or a sequence of them; keys already present are left alone.
func applyMerge(out map[string]interface{}, m *yaml.Node, depth int) error {
	v, err := nodeValue(m, depth+1)
	if err != nil {
		return err
	}
	switch src := v.(type) {
	case map[string]interface{}:
		for k, val := range src {
			if _, exists := out[k]; !exists {
				out[k] = val
			}
		}
	case []interface{}:
		for _, entry := range src {
			sub, ok := entry.(map[string]interface{})
			if !ok {
				return fmt.Errorf("frontmatter is not valid YAML: a \"<<\" merge "+
					"list may only contain mappings, got %s", jsonTypeName(entry))
			}
			for k, val := range sub {
				if _, exists := out[k]; !exists {
					out[k] = val
				}
			}
		}
	default:
		return fmt.Errorf("frontmatter is not valid YAML: \"<<\" must merge a "+
			"mapping, got %s", jsonTypeName(v))
	}
	return nil
}

// jsonTypeName names a value the way a schema author would, so a message about
// the wrong shape reads in the vocabulary of the schema rather than of Go.
func jsonTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int64:
		return "integer"
	case float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "list"
	case map[string]interface{}:
		return "dict"
	}
	return fmt.Sprintf("%T", v)
}

// notJSON walks a parsed frontmatter value and returns a message for the first
// value JSON cannot hold, located by JSON Pointer, or "" when every value is
// JSON data.
//
// The schemas are JSON Schema, so a value outside the JSON data model has no
// defined behaviour under any keyword — and two independent implementations
// will invent different behaviour for it. NaN is the clearest case: Python's
// json.dumps writes a bare `NaN` (which is not JSON, and which no other reader
// accepts) while Go's refuses outright, so the same note passed one validator
// and failed the other. Both now refuse it, in the same words.
//
// The message is deliberately phrased in the primary validator's vocabulary —
// Python type names — so the two reports describe one value the same way.
func notJSON(v interface{}, pointer string) string {
	at := func(what string) string {
		if pointer == "" {
			return what
		}
		return pointer + ": " + what
	}
	switch x := v.(type) {
	case nil, bool, string, int64:
		return ""
	case float64:
		if math.IsNaN(x) {
			return at("nan is not a JSON number — a schema cannot judge a value JSON cannot hold")
		}
		if math.IsInf(x, 0) {
			name := "inf"
			if math.IsInf(x, -1) {
				name = "-inf"
			}
			return at(name + " is not a JSON number — a schema cannot judge a value JSON cannot hold")
		}
		return ""
	case nonJSON:
		return at("a " + x.PyName + " is not JSON data — a schema cannot judge a value JSON cannot hold")
	case []interface{}:
		for i, item := range x {
			if msg := notJSON(item, pointer+"/"+strconv.Itoa(i)); msg != "" {
				return msg
			}
		}
		return ""
	case map[string]interface{}:
		// Sorted, so the same note reports the same field every run.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if msg := notJSON(x[k], pointer+"/"+escapePointer(k)); msg != "" {
				return msg
			}
		}
		return ""
	}
	return at(fmt.Sprintf("a %T is not JSON data — a schema cannot judge a value JSON cannot hold", v))
}

// escapePointer encodes a mapping key for a JSON Pointer segment (RFC 6901).
func escapePointer(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

// toInstance converts a parsed frontmatter map into exactly the
// representation santhosh-tekuri/jsonschema expects: types produced by
// encoding/json (float64 for numbers, map[string]interface{},
// []interface{}, string, bool, nil). The json round-trip is what guarantees
// that, rather than relying on the node walk to happen to already match.
//
// Anything JSON cannot hold is refused first, by notJSON, so the report names
// the field rather than surfacing a marshaller's opinion of the whole document.
func toInstance(m map[string]interface{}) (interface{}, error) {
	if msg := notJSON(m, ""); msg != "" {
		return nil, errors.New(msg)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("frontmatter is not JSON-representable: %w", err)
	}
	var instance interface{}
	if err := json.Unmarshal(b, &instance); err != nil {
		return nil, err
	}
	return instance, nil
}
