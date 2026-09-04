# Current Work: Minimal core compiler

## Goals

Validate a Markdown root against a caller-supplied schema and check local Markdown links. Supply minimal Person and Organization schemas using Schema.org names.

## Current Direction

One root Go module. A small core library, with no server, database, watcher, or extension runtime yet. Indexing belongs to consumers, superseding the older wiki statement that the compiler builds query indexes.

Public contract:

```text
Compile(root directory, JSON Schema bytes) -> diagnostics, operational error
Diagnostic = relative file path + 1-based line + field + message
Record = @context (https://schema.org), @id (stable nonempty URI),
         @type (Person or Organization), name (nonblank string)
All four fields required; unknown fields rejected.
```

The supplied schema applies to every Markdown file under the root. Schema.org provides vocabulary; JSON Schema provides validation rules. IDs must be unique. Parse Markdown with a standard parser, check local files and heading fragments, and do not fetch external URLs. Do not change source files. Use third-party code only for YAML, JSON Schema, and Markdown parsing.

## Success Criteria

- [x] Root Go module and core package are usable from Go.
- [x] Minimal schemas validate both record types and reject invalid metadata.
- [x] Compiler accepts caller-supplied schemas and detects duplicate IDs.
- [x] Markdown links resolve across files and headings; code examples do not produce false links.
- [x] Diagnostics identify source locations; operational failures return errors.
- [x] Focused public API tests, race check, and vet pass. Usage and limits are documented.

## Progress

- [x] Read wiki requirements and inspect empty code directory.
- [x] Implement and verify the compiler.

## Validation Result

`go test ./...`, `go test -race ./...`, and `go vet ./...` passed. `gofmt -l core` returned no files. Reviewed the public API, schema, Markdown resolution rules, diagnostics, and dependency list. The README states the supported syntax and the stable-folder requirement.
