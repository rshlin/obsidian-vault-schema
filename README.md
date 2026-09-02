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

## Agreeing with the Python validator

Two independent implementations of the same schema drift, and a vault that passes or fails
depending on which checker happened to be installed is worse than having no second checker: the
disagreement is invisible until it bites. So the parity is deliberate and tested, not hoped for.

The drift is almost never in the JSON Schema keywords — it is in **the YAML layer underneath
them**, because PyYAML implements YAML 1.1 and `gopkg.in/yaml.v3` implements a YAML-1.2-flavoured
core. Left alone the two disagree about what a bare word means. So plain scalars are resolved here
the way PyYAML's `SafeLoader` resolves them ([`scalar.go`](scalar.go)), and `testdata/pyyaml_scalars.json`
records what the vault's own loader produced for a sweep of tokens. `TestPlainScalarsMatchPyYAML`
replays every one of them: if it fails, the two validators now disagree about a value.

| | Both now say |
|---|---|
| `title: yes`, `no`, `on`, `off` | boolean (YAML 1.1), so a `type: string` field rejects it |
| `title: 1e3`, `0o17` | string — PyYAML resolves neither as a number |
| `title: 1:30` | integer `90` — YAML 1.1 sexagesimal |
| `created: 2026-01-31` | the string `2026-01-31`, quoted or not |
| `created: 2026-1-5` | the string `2026-1-5` — **not** silently zero-padded, so the ISO pattern rejects it |
| `created: 2026-01-31T10:30:00Z` | that whole string, so a date-only pattern rejects it |
| a key written twice | an error — one of the two values is being discarded |
| a UTF-8 BOM before `---` | stripped; the note has frontmatter |
| a recursive alias (`a: &x {b: *x}`) | an error, at a bounded depth, rather than a hang |
| `title: !!str 42` | the string `42` — an explicit tag beats the resolver |
| `title: !!int "42"` | the integer `42` — an explicit tag beats the quotes, too |
| `x: !!nosuchtag v`, `!Custom v` | an error; `SafeConstructor` defines no constructor for it |
| `score: .nan`, `.inf`, `-.inf` | an error naming the field — not a JSON number |
| `created: !!timestamp 2026-01-01` | an error naming the field — a date is not JSON data |
| `x: !!binary aGk=`, `!!set {a, b}` | an error naming the field — bytes and sets are not JSON data |

**Explicit tags are the case a style-only reader gets exactly backwards.** PyYAML picks a
constructor by *tag*, not by quoting: `!!int "42"` is an integer despite the quotes and `!!str 42`
is text despite looking numeric. `yaml.v3` also fills in an implicit tag for every scalar from its
own YAML-1.2 resolution, which must be ignored — only a tag the author actually wrote
(`yaml.TaggedStyle`) is honoured. `testdata/pyyaml_tags.json` records what the vault's loader built
for each one and `TestTaggedScalarsMatchPyYAML` replays them.

**Frontmatter has to be JSON data.** The schemas are JSON Schema, so a value outside the JSON data
model has no defined behaviour under any keyword — and two implementations will invent *different*
behaviour for it rather than agree. `NaN` is the clearest case: Python's `json.dumps` writes a bare
`NaN`, which is not JSON and which no other reader accepts, while Go's marshaller refuses outright,
so the same note passed one validator and failed the other. Both now refuse it, in the same words
and naming the field. The same rule covers the tags whose constructors build a non-JSON Python
object: `!!timestamp` (a date), `!!binary` (bytes), `!!set` (a set).

**Dates are the case worth spelling out.** `yaml.v3` parses both `2026-01-31` and `2026-1-5` into
`time.Time`, so re-rendering them as `YYYY-MM-DD` silently repairs the second one — it would pass
here and fail against the Python side, which leaves it a string. Nothing here converts a timestamp:
a date is judged as the characters its author typed. The vault's loader has PyYAML's timestamp
resolver removed for the same reason.

The remaining known difference is cosmetic: an integer larger than 2^63 becomes a float here and
stays exact in Python. Both are JSON *numbers*, so every keyword reaches the same verdict.

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
