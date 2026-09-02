package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// baseSchemaFilename is the schema every type schema is implicitly merged
// with, if present.
const baseSchemaFilename = "_base.schema.json"

// typeNamePattern constrains a discriminator value before it is allowed to
// become part of a filename. The value arrives from hand-written frontmatter,
// so without this a note declaring `type: ../../../etc/passwd` would walk
// straight out of the schemas directory through filepath.Join. No separators,
// no dots, no traversal, no absolute paths — checked before anything touches
// the filesystem. Deliberately identical to the in-tree Python validator's
// TYPE_NAME, so both implementations accept exactly the same names.
const typeNamePattern = `^[a-z][a-z0-9-]*$`

var typeNameRe = regexp.MustCompile(typeNamePattern)

// ValidTypeName reports whether a discriminator value is usable as a schema
// filename. It is the containment guard, not a style check: every caller that
// turns a discriminator into a path must pass it first.
func ValidTypeName(value string) bool {
	return typeNameRe.MatchString(value)
}

// SchemaSet compiles and caches "<type>.schema.json" files from one
// directory, each merged with baseSchemaFilename via allOf. Not safe for
// concurrent use.
type SchemaSet struct {
	dir      string
	compiled map[string]*jsonschema.Schema
}

func NewSchemaSet(dir string) *SchemaSet {
	return &SchemaSet{dir: dir, compiled: map[string]*jsonschema.Schema{}}
}

// Compile returns the compiled schema for a discriminator value, merging
// "<dir>/_base.schema.json" (if present) with "<dir>/<value>.schema.json"
// (required — its absence is what "unknown type" means). No $id is needed
// in either file; AddResource's URL argument is enough for $ref to resolve.
func (s *SchemaSet) Compile(value string) (*jsonschema.Schema, error) {
	// Before the cache, before filepath.Join, before any os call: a value that
	// cannot be a schema name never becomes a path.
	if !ValidTypeName(value) {
		return nil, fmt.Errorf("%q is not a usable schema name — it must match %s", value, typeNamePattern)
	}

	if sch, ok := s.compiled[value]; ok {
		return sch, nil
	}

	typePath := filepath.Join(s.dir, value+".schema.json")
	if _, err := os.Stat(typePath); err != nil {
		return nil, fmt.Errorf("no schema file for type %q (looked for %s)", value, typePath)
	}

	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020

	hasBase := false
	basePath := filepath.Join(s.dir, baseSchemaFilename)
	if f, err := os.Open(basePath); err == nil {
		defer f.Close()
		if err := c.AddResource("mem://base.json", f); err != nil {
			return nil, fmt.Errorf("loading %s: %w", basePath, err)
		}
		hasBase = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading %s: %w", basePath, err)
	}

	typeFile, err := os.Open(typePath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", typePath, err)
	}
	defer typeFile.Close()
	if err := c.AddResource("mem://type.json", typeFile); err != nil {
		return nil, fmt.Errorf("loading %s: %w", typePath, err)
	}

	sch, err := compileMerged(c, hasBase)
	if err != nil {
		return nil, fmt.Errorf("compiling schema for type %q: %w", value, err)
	}
	s.compiled[value] = sch
	return sch, nil
}

// CompileSchemaFile compiles exactly one schema file with no base merge —
// how the explicit "--files/--schema" overlay mode validates a file set
// against one schema.
func CompileSchemaFile(path string) (*jsonschema.Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020
	if err := c.AddResource("mem://overlay.json", f); err != nil {
		return nil, fmt.Errorf("loading %s: %w", path, err)
	}
	return c.Compile("mem://overlay.json")
}

func compileMerged(c *jsonschema.Compiler, hasBase bool) (*jsonschema.Schema, error) {
	refs := `[{"$ref":"mem://type.json"}]`
	if hasBase {
		refs = `[{"$ref":"mem://base.json"},{"$ref":"mem://type.json"}]`
	}
	wrapper := fmt.Sprintf(`{"$schema":"https://json-schema.org/draft/2020-12/schema","allOf":%s}`, refs)
	if err := c.AddResource("mem://merged.json", strings.NewReader(wrapper)); err != nil {
		return nil, err
	}
	return c.Compile("mem://merged.json")
}
