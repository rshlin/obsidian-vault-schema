package lint

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type validationIssue struct {
	Location string
	Message  string
}

// format renders a validationIssue for a report line: "location: message" when
// the issue names a field, or just "message" when it's located at the root
// (e.g. a "required" failure, which has no InstanceLocation of its own).
func (i validationIssue) format() string {
	if i.Location == "" {
		return i.Message
	}
	return i.Location + ": " + i.Message
}

// flattenValidationError turns a *jsonschema.ValidationError into one issue
// per leaf failure. BasicOutput's first entry is always a root summary line
// with an empty KeywordLocation ("doesn't validate with ...") — dropped here
// since it names no field. Intermediate entries from combinators (allOf, if-then,
// etc.) are also dropped, keeping only true leaves.
func flattenValidationError(err error) []validationIssue {
	if err == nil {
		return nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []validationIssue{{Message: err.Error()}}
	}
	basic := ve.BasicOutput()

	var candidates []jsonschema.BasicError
	for _, be := range basic.Errors {
		if be.KeywordLocation == "" {
			continue
		}
		candidates = append(candidates, be)
	}

	var out []validationIssue
	for i, be := range candidates {
		isContainer := false
		for j, other := range candidates {
			if i == j {
				continue
			}
			if strings.HasPrefix(other.KeywordLocation, be.KeywordLocation+"/") {
				isContainer = true
				break
			}
		}
		if isContainer {
			continue
		}
		out = append(out, validationIssue{Location: be.InstanceLocation, Message: be.Error})
	}
	return out
}

// resolvePointer looks up a JSON Pointer (RFC 6901, e.g. "/sources/0") inside
// a value shaped like toInstance's output. ok is false if any segment is
// missing or the wrong kind — never a panic.
func resolvePointer(v interface{}, pointer string) (interface{}, bool) {
	if pointer == "" {
		return v, true
	}
	cur := v
	for _, seg := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		switch node := cur.(type) {
		case map[string]interface{}:
			val, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = val
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// isPlaceholderAt reports whether the value at pointer inside instance is a
// string matching pattern — used to suppress enum/pattern/type violations on
// template placeholder values in --relaxed mode. required-field presence is
// never suppressed this way: the schema's own "required" already ran
// against the untouched instance before this is ever consulted.
func isPlaceholderAt(instance interface{}, pointer string, pattern *regexp.Regexp) bool {
	v, ok := resolvePointer(instance, pointer)
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && pattern.MatchString(s)
}
