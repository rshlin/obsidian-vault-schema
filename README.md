# obsidian-vault-schema

`obsidian-vault-lint` validates a Markdown note's YAML frontmatter against a JSON Schema — either
resolved from a discriminator field (`type:` by default) or given explicitly.

It knows nothing about any particular vault's conventions. The schemas, the discriminator field,
the relaxed directory, the placeholder pattern and both exclude sets are all supplied by the
caller, so the same binary serves any Obsidian vault that keeps `*.schema.json` files somewhere.

## Optional, not required

**This is an accelerator, not a dependency.** A vault's *primary* schema check is its own in-tree
Python validator (`scripts/check_schema.py`), which needs no toolchain, no install and no build
step. This binary is the faster path over the same rules, for vaults large enough to notice.

A vault must pass every check with this tool absent from `PATH`. So the wiring is always a
guarded, additive pass: the vault's `make check` runs it only when `command -v` finds it, takes
its file set from the vault's own config rather than a second copy of the exclude list, and treats
"not installed" as silence rather than failure. Because it is a different JSON Schema
implementation over the same notes and the same schemas, the most it can do is catch a keyword the
Python side does not implement.

If the two ever disagree, the Python one is right by definition — it is the specification, and a
disagreement is a bug in this tool.

## Install

    make install    # -> ~/.local/bin/obsidian-vault-lint

`~/.local/bin` because that is what is on `PATH`; `go install` would put it in
`$(go env GOPATH)/bin`, which is not. Override either half:

    make install PREFIX=/usr/local     # -> /usr/local/bin/obsidian-vault-lint
    make install BINDIR=/some/dir      # -> /some/dir/obsidian-vault-lint

Other targets: `make build` (into `bin/`), `make test`, `make tidy`, `make clean`.

A vault generated from the knowledge template can install it without coming here at all —
`make install-lint` in the vault builds this checkout into `~/.local/bin` and tells you whether
`PATH` actually picked it up.

## CLI

    obsidian-vault-lint check [flags]

`check` is the only subcommand and it is mandatory. There is no `--help` and no `--version`: any
first argument other than `check` prints the usage block to **stderr** and exits 2, so do not
write a wrapper that treats a nonzero exit from `--help` as a tool failure.

The two modes are mutually exclusive — combining them is a usage error.

### Type-driven mode

Validates every `.md` file under a vault against the schema its discriminator selects.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--vault DIR` | — | vault root to walk (required) |
| `--schemas DIR` | — | directory of `*.schema.json` files (required) |
| `--discriminator FIELD` | `type` | frontmatter field naming the schema |
| `--relaxed DIR` | — | top-level directory exempt from placeholder-located violations |
| `--placeholder-pattern RE` | `\{\{.*?\}\}` | regexp identifying a placeholder value |
| `--exclude-dir NAME` | — | directory name to skip at any depth (repeatable) |
| `--exclude-file PATH` | — | file path relative to the vault root to skip (repeatable) |

    obsidian-vault-lint check --vault . --schemas schemas \
      --relaxed templates --exclude-dir .obsidian --exclude-file README.md

### Explicit-schema mode

Validates a file glob against one schema, with no base merge and no discriminator lookup — how a
vault expresses an additive check over a subset of notes its discriminator cannot name.

| Flag | Meaning |
| --- | --- |
| `--files GLOB` | glob of files to validate (required) |
| `--schema FILE` | the one schema to validate them against (required) |

    obsidian-vault-lint check --files 'projects/*/tasks/*.md' \
      --schema schemas/_task-overlay.schema.json

## How a schema is resolved

A note declaring `type: reference` is validated against `<schemas>/reference.schema.json`, merged
via `allOf` with `<schemas>/_base.schema.json` when that file exists. The type schema is required:
its absence is exactly what "unknown type" means. Neither file needs an `$id`.

The discriminator value must match `^[a-z][a-z0-9-]*$`. It arrives from hand-written frontmatter
and becomes part of a filename, so anything with a separator, a dot, a traversal or a leading
slash is refused **before** it reaches the filesystem — a note cannot read a schema outside
`--schemas`. This is the same constraint the Python validator applies, so both accept exactly the
same set of type names.

## Relaxed mode

`--relaxed DIR` exempts files under one top-level directory (e.g. `templates`) from
enum/pattern/type violations **located at** a value matching `--placeholder-pattern`. It is
narrow on purpose: required-field presence is always checked in full, everywhere, and a real
violation at a non-placeholder value in a relaxed directory is still reported.

## Output and exit codes

Findings go to **stdout**, one per line, in the same `warn`/`FAIL`/`OK` vocabulary a vault's own
check scripts use, so a `make check` line reads the same whichever tool emitted it:

      FAIL  notes/bad-reference.md: /status: value must be one of "draft", "review", "canonical"
    schema: 128 checked, 1 error(s), 0 warning(s)

A clean run prints only the summary, prefixed:

    OK  schema: 128 checked, 0 error(s), 0 warning(s)

| Exit | Meaning |
| --- | --- |
| 0 | everything validated |
| 1 | validation errors found (details on stdout) |
| 2 | usage or configuration error (message on stderr) |
