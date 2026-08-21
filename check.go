package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RunVaultCheck validates every note under vaultDir against the schema its
// discriminator field selects. relaxedDir names a top-level directory (e.g.
// "templates") whose files are exempt from enum/pattern/type violations
// specifically located at a placeholder value — required-field presence is
// always fully checked, everywhere. excludeDirs names directories (matched
// exactly by name, at any depth) never walked at all; excludeFiles names
// paths relative to vaultDir never read or reported.
func RunVaultCheck(vaultDir, schemasDir, discriminator, relaxedDir, placeholderPattern string, excludeDirs, excludeFiles []string) (*Report, error) {
	placeholderRe, err := regexp.Compile(placeholderPattern)
	if err != nil {
		return nil, fmt.Errorf("--placeholder-pattern: %w", err)
	}

	excludeDirSet := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		excludeDirSet[d] = true
	}
	excludeFileSet := make(map[string]bool, len(excludeFiles))
	for _, f := range excludeFiles {
		excludeFileSet[filepath.ToSlash(f)] = true
	}

	paths, err := walkMarkdownExcluding(vaultDir, excludeDirSet)
	if err != nil {
		return nil, err
	}

	schemas := NewSchemaSet(schemasDir)
	report := &Report{Title: "schema"}
	for _, path := range paths {
		rel, err := filepath.Rel(vaultDir, path)
		if err != nil {
			rel = path
		}
		if excludeFileSet[filepath.ToSlash(rel)] {
			continue
		}
		checkOneNote(report, path, rel, schemas, discriminator, relaxedDir, placeholderRe)
	}
	return report, nil
}

func checkOneNote(report *Report, path, rel string, schemas *SchemaSet, discriminator, relaxedDir string, placeholderRe *regexp.Regexp) {
	report.Checked++

	data, err := os.ReadFile(path)
	if err != nil {
		report.Errorf(rel, "%v", err)
		return
	}
	raw, _, ok := splitFrontmatter(data)
	if !ok {
		report.Errorf(rel, "no YAML frontmatter (file must start with ---)")
		return
	}
	fm, err := parseFrontmatter(raw)
	if err != nil {
		report.Errorf(rel, "%v", err)
		return
	}

	value, _ := fm[discriminator].(string)
	if value == "" {
		report.Errorf(rel, "missing or non-string %q field", discriminator)
		return
	}

	schema, err := schemas.Compile(value)
	if err != nil {
		report.Errorf(rel, "%v", err)
		return
	}

	instance, err := toInstance(fm)
	if err != nil {
		report.Errorf(rel, "%v", err)
		return
	}

	relaxed := relaxedDir != "" && firstPathComponent(rel) == relaxedDir
	if err := schema.Validate(instance); err != nil {
		for _, issue := range flattenValidationError(err) {
			if relaxed && isPlaceholderAt(instance, issue.Location, placeholderRe) {
				continue
			}
			report.Errorf(rel, "%s", issue.format())
		}
	}
}

func firstPathComponent(rel string) string {
	rel = filepath.ToSlash(rel)
	head, _, found := strings.Cut(rel, "/")
	if !found {
		return rel
	}
	return head
}

// RunFilesCheck validates every file matching glob against one explicit
// schema (no base merge, no discriminator lookup) — how additive checks
// like the knowledge vault's code-task overlay fields are expressed without
// this tool knowing what a "code task" is.
func RunFilesCheck(glob, schemaPath string) (*Report, error) {
	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("--files %q: %w", glob, err)
	}
	sort.Strings(paths)

	schema, err := CompileSchemaFile(schemaPath)
	if err != nil {
		return nil, err
	}

	report := &Report{Title: "overlay"}
	for _, path := range paths {
		report.Checked++
		data, err := os.ReadFile(path)
		if err != nil {
			report.Errorf(path, "%v", err)
			continue
		}
		raw, _, ok := splitFrontmatter(data)
		if !ok {
			report.Errorf(path, "no YAML frontmatter (file must start with ---)")
			continue
		}
		fm, err := parseFrontmatter(raw)
		if err != nil {
			report.Errorf(path, "%v", err)
			continue
		}
		instance, err := toInstance(fm)
		if err != nil {
			report.Errorf(path, "%v", err)
			continue
		}
		if err := schema.Validate(instance); err != nil {
			for _, issue := range flattenValidationError(err) {
				report.Errorf(path, "%s", issue.format())
			}
		}
	}
	return report, nil
}
