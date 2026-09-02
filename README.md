# obsidian-vault-schema

`obsidian-vault-lint` validates a Markdown note's YAML frontmatter against a JSON Schema — either
resolved from a discriminator field (`type:` by default) or given explicitly. It knows nothing
about any particular vault's conventions; a vault supplies its own `*.schema.json` files.

## Build

    make build      # -> bin/obsidian-vault-lint
    make test

## Install

    make install    # -> ~/.local/bin/obsidian-vault-lint

`~/.local/bin` because that is what is on PATH; `go install` would put it in
`$(go env GOPATH)/bin`, which is not. Override with `make install PREFIX=/usr/local`
or `make install BINDIR=/some/dir`.

## Usage

Type-driven mode — validate every note under a vault against the schema its `type` (or
`--discriminator`) selects, merged with `_base.schema.json` if one exists:

    obsidian-vault-lint check --vault . --schemas schemas

Explicit-schema mode — validate a file glob against one schema, for additive checks a
discriminator can't express:

    obsidian-vault-lint check --files 'projects/*/specs/*/tasks/*.md' \
      --schema schemas/_code-task-overlay.schema.json

`--relaxed DIR` exempts files under a top-level directory (e.g. `templates`) from
enum/pattern/type violations located at a value matching `--placeholder-pattern`
(default `\{\{.*?\}\}`) — required-field presence is always checked in full.

`--exclude-dir NAME` (repeatable) skips a directory by exact name at any depth in the vault.
`--exclude-file PATH` (repeatable) skips one file by its exact path relative to the vault root.

See the `knowledge` vault's `decisions/0009-adopt-json-schema-validation-via-standalone-linter.md`
for why this exists as its own tool.
