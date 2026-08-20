package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// baseSchemaFilename is the schema every type schema is implicitly merged
// with, if present.
const baseSchemaFilename = "_base.schema.json"

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
