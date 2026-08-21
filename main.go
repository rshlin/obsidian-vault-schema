package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// stringSliceFlag implements flag.Value to collect a repeatable flag (e.g.
// --exclude-dir passed more than once) into a slice.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

const usage = `obsidian-vault-lint check [flags]

Type-driven mode (validates every note under a vault against the schema its
discriminator field selects):
  --vault DIR               vault root directory
  --schemas DIR             directory of *.schema.json files
  --discriminator FIELD     frontmatter field naming the schema (default "type")
  --relaxed DIR             top-level directory exempt from enum/pattern/type
                            violations located at a placeholder value (e.g. "templates")
  --placeholder-pattern RE  regexp identifying a placeholder value
                            (default "\{\{.*?\}\}")
  --exclude-dir NAME        directory name to skip at any depth (repeatable)
  --exclude-file PATH       file path relative to the vault root to skip
                            entirely (repeatable)

Explicit-schema mode (validates a file glob against one schema, for additive
checks a discriminator can't express):
  --files GLOB              glob of files to validate
  --schema FILE              schema file to validate them against
`

func run(args []string) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	vault := fs.String("vault", "", "vault root directory (type-driven mode)")
	schemasDir := fs.String("schemas", "", "directory of *.schema.json files (type-driven mode)")
	discriminator := fs.String("discriminator", "type", "frontmatter field naming the schema to use")
	relaxedDir := fs.String("relaxed", "", "top-level directory exempt from placeholder-located violations")
	placeholderPattern := fs.String("placeholder-pattern", `\{\{.*?\}\}`, "regexp identifying a template placeholder value")
	var excludeDirs stringSliceFlag
	fs.Var(&excludeDirs, "exclude-dir", "directory name to skip at any depth (repeatable)")
	var excludeFiles stringSliceFlag
	fs.Var(&excludeFiles, "exclude-file", "file path relative to the vault root to skip entirely (repeatable)")
	files := fs.String("files", "", "glob of files to validate (explicit-schema mode)")
	schemaFile := fs.String("schema", "", "one schema file to validate --files against (explicit-schema mode)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	explicitMode := *files != "" || *schemaFile != ""
	typeMode := *vault != "" || *schemasDir != ""

	switch {
	case explicitMode && typeMode:
		fmt.Fprintln(os.Stderr, "check: --files/--schema cannot be combined with --vault/--schemas")
		return 2

	case explicitMode:
		if *files == "" || *schemaFile == "" {
			fmt.Fprintln(os.Stderr, "check: --files and --schema are both required together")
			return 2
		}
		report, err := RunFilesCheck(*files, *schemaFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check:", err)
			return 2
		}
		return report.Finish()

	case typeMode:
		if *vault == "" || *schemasDir == "" {
			fmt.Fprintln(os.Stderr, "check: --vault and --schemas are both required together")
			return 2
		}
		report, err := RunVaultCheck(*vault, *schemasDir, *discriminator, *relaxedDir, *placeholderPattern, excludeDirs, excludeFiles)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check:", err)
			return 2
		}
		return report.Finish()

	default:
		fmt.Fprintln(os.Stderr, "check: one of --vault/--schemas or --files/--schema is required")
		return 2
	}
}
