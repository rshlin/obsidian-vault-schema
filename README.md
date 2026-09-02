# obsidian-vault-schema

`obsidian-vault-lint` validates the YAML frontmatter of Obsidian-style Markdown notes against JSON
Schema, choosing the schema for each note from a frontmatter discriminator field (`type:` by
default).

It knows nothing about any particular vault's conventions. The schemas, the discriminator field,
the relaxed directory, the placeholder pattern and both exclude sets are supplied by the caller, so
the same binary serves any vault that keeps `*.schema.json` files somewhere.

The interesting part is not the JSON Schema. It is the YAML underneath it — see
[Why it agrees with PyYAML](#why-it-agrees-with-pyyaml).

## Install

```sh
GOBIN="$HOME/.local/bin" go install github.com/rshlin/obsidian-vault-schema/cmd/obsidian-vault-lint@latest
```

Two details in that line are load-bearing.

**The `/cmd/obsidian-vault-lint` suffix.** `go install` names the binary after the directory holding
`package main`, so that path element *is* the binary's name. Installing the module root instead
fails outright and installs nothing:

```console
$ go install github.com/rshlin/obsidian-vault-schema@v0.1.0
go: github.com/rshlin/obsidian-vault-schema@v0.1.0: module github.com/rshlin/obsidian-vault-schema@v0.1.0 found, but does not contain package github.com/rshlin/obsidian-vault-schema
```

That failure is deliberate. See [Repository layout](#repository-layout) for what it is protecting
against.

**`GOBIN`.** With `GOBIN` unset, `go install` writes to `$(go env GOPATH)/bin` — `~/go/bin` on a
default setup — which is *not* on `PATH` on a stock macOS account. The install reports success, and
the tool stays invisible. Point `GOBIN` at a directory already on your `PATH`.

Confirm you got the thing you meant to:

```console
$ go version -m "$(command -v obsidian-vault-lint)" | sed -n '2,3p'
	path	github.com/rshlin/obsidian-vault-schema/cmd/obsidian-vault-lint
	mod	github.com/rshlin/obsidian-vault-schema	v0.1.0	h1:T0rV0LKWm/rJRVUzX0WqlF0uYLNQPs8F1NHwWTDSZ20=
```

From a checkout, `make install` builds straight into `~/.local/bin` for the same reason. Override
either half:

```sh
make install                       # -> ~/.local/bin/obsidian-vault-lint
make install PREFIX=/usr/local     # -> /usr/local/bin/obsidian-vault-lint
make install BINDIR=/some/dir      # -> /some/dir/obsidian-vault-lint
```

Other targets: `make build` (into `bin/`), `make test`, `make tidy`, `make clean`.

## Thirty seconds

A vault is a directory of Markdown notes. A schemas directory holds one `<type>.schema.json` per
note type. Every note names its type in frontmatter, and that name selects its schema.

```sh
mkdir -p demo-vault/schemas && cd demo-vault

cat > schemas/note.schema.json <<'EOF'
{
  "type": "object",
  "required": ["type", "title", "status"],
  "properties": {
    "type":   { "const": "note" },
    "title":  { "type": "string" },
    "status": { "enum": ["draft", "review", "done"] }
  }
}
EOF

cat > kafka-retention.md <<'EOF'
---
type: note
title: Kafka retention
status: draft
---

Retention is seven days.
EOF
```

```console
$ obsidian-vault-lint check --vault . --schemas schemas; echo "exit $?"
OK  schema: 1 checked, 0 error(s), 0 warning(s)
exit 0
```

Now add a note with a status the enum does not allow:

```sh
cat > rebalance.md <<'EOF'
---
type: note
title: Rebalance runbook
status: in-progress
---
EOF
```

```console
$ obsidian-vault-lint check --vault . --schemas schemas; echo "exit $?"
  FAIL  rebalance.md: /status: value must be one of "draft", "review", "done"
schema: 2 checked, 1 error(s), 0 warning(s)
exit 1
```

Findings go to stdout, one per line, located by JSON Pointer into the frontmatter. A clean run
prints only the summary, prefixed `OK`.

And now the one that is worth the install. This note looks fine:

```sh
cat > shipping.md <<'EOF'
---
type: note
title: no
status: draft
---
EOF
```

```console
$ obsidian-vault-lint check --vault . --schemas schemas
  FAIL  rebalance.md: /status: value must be one of "draft", "review", "done"
  FAIL  shipping.md: /title: expected string, but got boolean
schema: 3 checked, 2 error(s), 0 warning(s)
```

`no` is a boolean. That is YAML 1.1, which is what PyYAML implements and what Obsidian vaults are
full of, and it is the sort of thing two YAML readers disagree about silently.

## The two modes

```
obsidian-vault-lint check [flags]
```

`check` is the only subcommand and it is mandatory. The two modes below are mutually exclusive;
combining them is a usage error.

### Type-driven mode

Walks a vault and validates every `.md` file against the schema its discriminator selects.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--vault DIR` | — | vault root to walk (required) |
| `--schemas DIR` | — | directory of `*.schema.json` files (required) |
| `--discriminator FIELD` | `type` | frontmatter field naming the schema |
| `--relaxed DIR` | — | top-level directory exempt from placeholder-located violations |
| `--placeholder-pattern RE` | `\{\{.*?\}\}` | regexp identifying a placeholder value |
| `--exclude-dir NAME` | — | directory name to skip at any depth (repeatable) |
| `--exclude-file PATH` | — | file path relative to the vault root to skip (repeatable) |

```sh
obsidian-vault-lint check --vault . --schemas schemas \
  --relaxed templates --exclude-dir .obsidian --exclude-file README.md
```

Directories whose name begins with `.` are always skipped, so `.git` and `.obsidian` need no flag
unless you want to be explicit.

**Every `.md` file is a note.** There is no opt-in marker, so a vault's own `README.md` — which has
no frontmatter — is a failure, not a skip. That is what `--exclude-file` is for:

```sh
mkdir -p tiny
printf -- '---\ntype: note\ntitle: t\nstatus: draft\n---\n' > tiny/one.md
printf '# Tiny vault\n'                                     > tiny/README.md
```

```console
$ obsidian-vault-lint check --vault tiny --schemas schemas
  FAIL  README.md: no YAML frontmatter (file must start with ---)
schema: 2 checked, 1 error(s), 0 warning(s)

$ obsidian-vault-lint check --vault tiny --schemas schemas --exclude-file README.md
OK  schema: 1 checked, 0 error(s), 0 warning(s)
```

**How a schema is resolved.** A note declaring `type: reference` is validated against
`<schemas>/reference.schema.json`, merged via `allOf` with `<schemas>/_base.schema.json` when that
file exists. The type schema is required — its absence is exactly what "unknown type" means.
Neither file needs an `$id`.

The discriminator value must match `^[a-z][a-z0-9-]*$`. It arrives from hand-written frontmatter
and is about to become part of a filename, so it is checked before anything touches the
filesystem — a note cannot read a schema outside `--schemas`. The pattern is character-for-character
the Python validator's, so both accept exactly the same set of type names:

```sh
mkdir -p badtypes
printf -- '---\ntype: ../../../etc/passwd\n---\n' > badtypes/evil.md
printf -- '---\ntype: unheard-of\n---\n'          > badtypes/unknown.md
```

```console
$ obsidian-vault-lint check --vault badtypes --schemas schemas
  FAIL  evil.md: "type: ../../../etc/passwd" is not a usable schema name — it must match ^[a-z][a-z0-9-]*$
  FAIL  unknown.md: no schema file for type "unheard-of" (looked for schemas/unheard-of.schema.json)
schema: 2 checked, 2 error(s), 0 warning(s)
```

**Relaxed mode.** `--relaxed DIR` exempts files under one top-level directory from enum, pattern
and type violations *located at* a value matching `--placeholder-pattern` — because a note template
carries `{{title}}` where a real value will go. It is narrow on purpose: required-field presence is
still checked in full, and a real violation at a non-placeholder value in a relaxed directory is
still reported.

Continuing the vault above — clear out the bad notes, and add a template that uses placeholders
plus one that is genuinely missing a required field:

```sh
rm -rf rebalance.md shipping.md badtypes && mkdir -p templates

cat > templates/note.md <<'EOF'
---
type: note
title: "{{title}}"
status: "{{status}}"
---
EOF

cat > templates/broken.md <<'EOF'
---
type: note
title: "{{title}}"
---
EOF
```

```console
$ obsidian-vault-lint check --vault . --schemas schemas
  FAIL  templates/broken.md: missing properties: 'status'
  FAIL  templates/note.md: /status: value must be one of "draft", "review", "done"
schema: 3 checked, 2 error(s), 0 warning(s)

$ obsidian-vault-lint check --vault . --schemas schemas --relaxed templates
  FAIL  templates/broken.md: missing properties: 'status'
schema: 3 checked, 1 error(s), 0 warning(s)
```

The placeholder violation is gone; the missing required field is not.

### Explicit-schema mode

Validates a file glob against one schema, with no base merge and no discriminator lookup — for an
additive check over a subset of notes the discriminator cannot name.

| Flag | Meaning |
| --- | --- |
| `--files GLOB` | glob of files to validate (required) |
| `--schema FILE` | the one schema to validate them against (required) |

Quote the glob: it is expanded by the tool, not the shell. It is Go's `filepath.Glob`, and two of
its properties will bite you. Add one note three levels down to see them:

```sh
mkdir -p projects/alpha/tasks
printf -- '---\ntype: note\ntitle: t\nstatus: draft\n---\n' > projects/alpha/tasks/one.md
```

**There is no recursive globstar.** A doubled `*` is parsed as an ordinary `*` and matches a single
path segment, so `**/*.md` means `*/*.md` — one level down, not "everywhere". In the vault above,
that quietly checks the two templates and none of the deeper notes:

```console
$ obsidian-vault-lint check --files '**/*.md' --schema schemas/note.schema.json
  FAIL  templates/broken.md: missing properties: 'status'
  FAIL  templates/note.md: /status: value must be one of "draft", "review", "done"
overlay: 2 checked, 2 error(s), 0 warning(s)
```

**A glob that matches nothing is a pass.** Zero files validated is zero errors:

```console
$ obsidian-vault-lint check --files 'archive/*.md' --schema schemas/note.schema.json; echo "exit $?"
OK  overlay: 0 checked, 0 error(s), 0 warning(s)
exit 0
```

So spell the depth out, and read the count rather than only the exit code:

```console
$ obsidian-vault-lint check --files 'projects/*/tasks/*.md' --schema schemas/note.schema.json
OK  overlay: 1 checked, 0 error(s), 0 warning(s)
```

This mode titles its summary `overlay` rather than `schema`, so the two passes stay distinguishable
in one log.

## Exit codes

| Exit | Meaning |
| --- | --- |
| 0 | everything validated |
| 1 | validation errors found (details on stdout) |
| 2 | usage or configuration error, including a schema that fails to compile (message on stderr) |

There is no `--help` and no `--version`. Any first argument other than `check` prints the usage
block to **stderr** and exits 2 — including `--help` itself, which still prints the usage but is
not a success:

```console
$ obsidian-vault-lint --help >/dev/null; echo "exit $?"
obsidian-vault-lint check [flags]
[... usage block, on stderr ...]
exit 2
```

So do not write a wrapper that probes with `--help` and treats a nonzero exit as the tool being
broken or absent. Probe with `command -v` instead.

## Why it agrees with PyYAML

This was built as a *second opinion*. The vault it was written for already had a validator: an
independent implementation in Python, built on PyYAML, which remains the primary. The two were run
against a corpus of deliberately broken notes and reconciled until they agreed.

Agreeing on the easy cases is worthless. Both implementations were always going to reject a missing
required field and accept a well-formed note; if that were the whole story there would be no reason
to write the second one, and no reason to trust it. Everything that mattered was in the YAML layer
*underneath* the schema, where two independent readers of the same bytes normally drift apart
without either of them noticing — and where a note then passes or fails depending on which checker
happened to be installed. That is the one failure a two-validator design cannot absorb.

PyYAML implements YAML 1.1. `gopkg.in/yaml.v3` implements a YAML-1.2-flavoured core. Left alone they
disagree about what a bare word means. So plain scalars are resolved here the way PyYAML's
`SafeLoader` resolves them ([`lint/scalar.go`](lint/scalar.go)), and the agreement is pinned by
fixtures rather than hoped for: `lint/testdata/pyyaml_scalars.json` (103 tokens) and
`lint/testdata/pyyaml_tags.json` (43 tokens) record what the Python loader actually produced, and
`TestPlainScalarsMatchPyYAML` and `TestTaggedScalarsMatchPyYAML` replay every one. If either fails,
the two validators now disagree about a value.

What the cross-validation actually found:

**Plain scalars resolve by YAML 1.1 rules.** The right-hand column is what `yaml.Unmarshal` into an
`interface{}` gives you if you do nothing about it.

| Frontmatter | Value here, and in PyYAML | Stock `yaml.v3` |
| --- | --- | --- |
| `v: yes` `no` `on` `off` | boolean — so a `type: string` field rejects it | all four stay strings |
| `v: 1:30` | integer `90` — YAML 1.1 sexagesimal | the string `1:30` |
| `v: 1e3`, `v: 1.0e3` | the string — PyYAML's float pattern needs a decimal point *and* a **signed** exponent | number `1000` |
| `v: 1.0e+3` | number `1000` — the same value, one `+` away | number `1000` |
| `v: 0o17` | the string `0o17` — YAML 1.2 octal, which PyYAML does not know | integer `15` |
| `v: 017` | integer `15` — YAML 1.1 leading-zero octal | integer `15` |
| `v: 1_000` | integer `1000` — YAML 1.1 digit separators | integer `1000` |

**Dates keep the text their author wrote.** This is a departure from stock PyYAML, matched on the
Python side. PyYAML reads a bare `2026-01-31` as a `datetime.date` but leaves `2026-1-5` a string;
`yaml.v3` reads both as `time.Time`. Re-rendering a parsed date as `YYYY-MM-DD` therefore silently
*repairs* input the schema's ISO pattern is there to reject — and repairs it on only one of the two
sides. Both implementations have the timestamp resolver removed instead, so `created: 2026-1-5`
stays `"2026-1-5"` and fails, everywhere.

**An explicit tag beats quoting — and beats the resolver.** PyYAML picks a constructor by *tag*,
not by how the value is written. A reader that decides by quoting alone gets this exactly backwards,
and deciding by quoting is the natural thing to do once you are already overriding the resolver:

| Frontmatter | Deciding by quoting | Here, and in PyYAML |
| --- | --- | --- |
| `v: !!int "42"` | string `"42"` (it's quoted) | integer `42` |
| `v: !!str 42` | integer `42` (it looks numeric) | string `"42"` |
| `v: !!bool "yes"` | string `"yes"` | boolean `true` |
| `v: !!null anything` | string `"anything"` | `null` — the tag wins outright |

Stock `yaml.v3` gives a third answer again: it agrees on the first two rows and refuses the last two
outright (``yaml: cannot decode !!str `yes` as a !!bool``). It also fills in an *implicit* tag for
every scalar from its own YAML-1.2 resolution. Honouring that would reintroduce exactly the drift
this file exists to remove, so only a tag the author actually wrote (`yaml.TaggedStyle`) is
honoured.

**An unknown tag is an error, not a string.** PyYAML's `SafeConstructor` defines no constructor for
`!!nosuchtag` or `!Custom` and raises; `yaml.v3` accepts both as plain strings. Silently accepting
one is how a typo'd tag reaches production.

**Frontmatter has to be JSON data.** The schemas are JSON Schema, so a value outside the JSON data
model has no defined behaviour under any keyword — and two implementations will invent *different*
behaviour for it rather than agree. `NaN` is the clearest case: Python's `json.dumps` emits a bare
`NaN`, which is not JSON and which no other reader accepts, while Go's marshaller refuses outright.
The same note passed one validator and failed the other. Both now refuse it, in the same words and
naming the field.

The rule covers `.nan`, `.inf`, `-.inf`, and the tags whose constructors build a non-JSON Python
object: `!!timestamp` (a date), `!!binary` (bytes), `!!set` (a set).

**A key written twice is an error.** `yaml.v3` rejects it; PyYAML silently keeps the last one. The
Python side was taught to reject it too, because a duplicate key is data being dropped, not a style
preference.

Five notes, five of the behaviours above, one run. Each is a valid `note` except for one line:

```sh
mkdir -p edge
printf -- '---\ntype: note\ntitle: Rebalance\nstatus: draft\ncreated: !!timestamp 2026-01-01\n---\n' > edge/created.md
printf -- '---\ntype: note\ntitle: Rebalance\ntitle: Rebalance runbook\nstatus: draft\n---\n'       > edge/dup.md
printf -- '---\ntype: note\ntitle: !!int "notanint"\nstatus: draft\n---\n'                          > edge/notanint.md
printf -- '---\ntype: note\ntitle: Rebalance\nstatus: draft\nscore: .nan\n---\n'                    > edge/score.md
printf -- '---\ntype: note\ntitle: !Custom Rebalance\nstatus: draft\n---\n'                         > edge/tagged.md
```

```console
$ obsidian-vault-lint check --vault edge --schemas schemas
  FAIL  created.md: /created: a date is not JSON data — a schema cannot judge a value JSON cannot hold
  FAIL  dup.md: line 3: duplicate key "title" in frontmatter — one of the two values is silently discarded
  FAIL  notanint.md: frontmatter is not valid YAML: could not determine a int from "notanint"
  FAIL  score.md: /score: nan is not a JSON number — a schema cannot judge a value JSON cannot hold
  FAIL  tagged.md: frontmatter is not valid YAML: could not determine a constructor for the tag '!Custom'
schema: 5 checked, 5 error(s), 0 warning(s)
```

Two details in that output. Line numbers are relative to the frontmatter block, not the file, so
`dup.md`'s "line 3" is the fourth line of the file. And a note is refused at the *first* value JSON
cannot hold, in sorted key order — one note with both `.nan` and a timestamp reports one of them.

**And two smaller ones.** A UTF-8 BOM before `---` is stripped, so a note that plainly does have
frontmatter is not reported as having none. A recursive alias (`a: &x {b: *x}`) is an error at a
bounded depth rather than a hang.

### This is a compatibility choice, not a claim about correctness

None of the above says PyYAML is right. `yes` is a boolean because YAML 1.1 says so; YAML 1.2
removed that, and `yaml.v3` is closer to 1.2. Both readings are defensible, and if you are starting
a new format from scratch, 1.2 is the better one.

What is not defensible is a corpus that passes or fails depending on which checker ran. So this
implementation reproduces PyYAML's `SafeConstructor` deliberately, and the deliberateness is the
point: if you point this tool at frontmatter that has never been near PyYAML, `yes` will still be a
boolean here, and that is working as intended rather than a bug.

One known residual difference. Frontmatter is handed to the schema through `encoding/json`, so every
number arrives as a float64 and integers above 2^53 stop being exact — where Python keeps an exact
`int`. `type`, `minimum` and `maximum` reach the same verdict either way; an exact `const` or `enum`
on a value that large does not:

```sh
printf -- '---\ntype: note\ntitle: t\nstatus: draft\nv: 9007199254740993\n---\n' > big.md
echo '{"type":"object","properties":{"v":{"const":9007199254740993}}}'           > exact.schema.json
```

```console
$ obsidian-vault-lint check --files big.md --schema exact.schema.json
  FAIL  big.md: /v: value must be "9007199254740993"
overlay: 1 checked, 1 error(s), 0 warning(s)
```

Nothing in a note's frontmatter should be an integer that large, but if yours is, this is where the
two validators part company.

## Your schema may not be portable

Two JSON Schema constructs compile elsewhere and are refused here. Both are exit 2 — a schema that
does not compile is a configuration error, not a note that failed.

**Patterns are RE2.** Go's `regexp` has no backtracking and therefore no lookahead or lookbehind,
by design. This compiles in Python and in most JavaScript validators:

```json
{ "properties": { "slug": { "type": "string", "pattern": "^(?!draft-).*$" } } }
```

Here it does not:

```console
$ obsidian-vault-lint check --files note.md --schema lookahead.schema.json; echo "exit $?"
check: jsonschema mem://overlay.json compilation failed: '/properties/slug/pattern' does not validate with https://json-schema.org/draft/2020-12/schema#/allOf/1/$ref/properties/properties/additionalProperties/$dynamicRef/allOf/3/$ref/properties/pattern/format: '^(?!draft-).*$' is not valid 'regex'
exit 2
```

**`items` is 2020-12, not draft-7.** Draft-7 spelled a tuple as an array of schemas. 2020-12
renamed that to `prefixItems`, and `items` now takes a single schema, so the draft-7 spelling is
not merely deprecated — it is a type error:

```json
{ "properties": { "pair": { "type": "array", "items": [ { "type": "string" }, { "type": "integer" } ] } } }
```

```console
$ obsidian-vault-lint check --files note.md --schema tuple.schema.json; echo "exit $?"
check: jsonschema mem://overlay.json compilation failed: '/properties/pair/items' does not validate with https://json-schema.org/draft/2020-12/schema#/allOf/1/$ref/properties/properties/additionalProperties/$dynamicRef/allOf/1/$ref/properties/items/$dynamicRef/type: expected object or boolean, but got array
exit 2
```

Neither is a linter limitation to work around. A schema that relies on lookahead already means
something different under every RE2-based validator, and a draft-7 tuple already means something
different under 2020-12. Both are portability defects in the schema, and this tool is where you
find out.

## Repository layout

```
cmd/obsidian-vault-lint/    package main — the CLI, and the source of the binary's name
lint/                       the library: parsing, schema resolution, validation, reporting
lint/testdata/              fixtures, shared by both packages' tests
<root>                      no Go package, on purpose
```

The repository is named `obsidian-vault-schema` and the binary is named `obsidian-vault-lint`. That
mismatch is the whole reason for this layout.

**Why `package main` is not at the root.** `go install` names the binary after the directory holding
`package main`, and for a `package main` at a module root that directory is the module. So a root
`main.go` here would install a binary called **`obsidian-vault-schema`** — a name nothing invokes.
Callers run the bare name `obsidian-vault-lint`, and the usual wiring for an optional tool is a
`command -v obsidian-vault-lint` guard that treats "not installed" as silence rather than failure.
Put those together and a user who followed the documented one-command install gets a green
`make check` with the schema pass never running once, and nothing anywhere saying so.

Moving `main.go` up to "simplify the layout" reintroduces that, and reintroduces it silently: the
build still works, every behavioural test still passes, and `make install` still writes the right
name, because it names the output explicitly. The only thing that breaks is the `go install` path —
the one path no behavioural test covers.

**Why the root holds no Go package at all.** A *library* package at the root is not inert either.
`go build -o SOME/PATH .` against one exits 0 and writes a non-executable `ar` archive to
`SOME/PATH` instead of refusing. Pointed at a bindir — which is precisely what an installer does —
that replaces the real tool on `PATH` with an unrunnable file at mode `0644`. `command -v` then
stops finding it, and every guarded "run the linter if it is installed" branch goes quiet. With no
package at the root, the same command fails loudly (`no Go files in ...`, exit 1) and leaves the
installed binary alone.

Both failures look exactly like a successful install, so they are encoded as tests rather than as
comments. In [`cmd/obsidian-vault-lint/layout_test.go`](cmd/obsidian-vault-lint/layout_test.go):

- `TestBinaryNameComesFromThisDirectory` — asserts `package main` lives in a directory named
  `obsidian-vault-lint`.
- `TestModuleRootHasNoGoPackage` — asserts the module root contains no `.go` files.
- `TestREADMEDocumentsTheRealInstallPath` — reads the module path out of `go.mod`, derives the
  install path the layout actually produces, and greps this README for it. If the layout moves and
  the install line is not updated to match, `go test ./...` goes red.

## Scope

This is an optional accelerator, not a required dependency.

It was built for a vault whose primary schema check is its own in-tree Python validator — no
toolchain, no install, no build step. That vault passes every check with this binary absent from
`PATH`, and the wiring is deliberately additive: the schema pass runs this only when `command -v`
finds it, takes its file set from the vault's own config rather than a second copy of the exclude
list, and treats "not installed" as silence.

Because it is a different JSON Schema implementation reading the same notes against the same
schemas, the most it can do is catch a keyword the other side does not implement, and be faster
about it. If the two ever disagree on anything else, that is a bug here.

The Python validator lives in the vault repository this was extracted from, which is not published;
nothing in this repository depends on it.
