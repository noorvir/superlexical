# Current Work: Compiler diagnostics and stages

## Goals

Fix broken-link error recovery and source locations for custom schemas. Keep the public Compile API and minimal Directory schema unchanged.

## Current Direction

Read and parse documents, validate records and collect identities, then resolve links. One internal document holds metadata, source locations, links, and headings. Preserve structured property paths, including array indexes and punctuation. Replace the schema validator because its flattened context does not expose lossless path segments; keep three direct dependencies. Decode from the existing YAML syntax tree to avoid parsing YAML twice.

## Success Criteria

- [x] A link through a regular file reports a diagnostic and validation continues.
- [x] Array items and quoted property names report exact source lines without path collisions.
- [x] Existing validation and link behavior remains covered.
- [x] Regressions compare the complete diagnostic output.
- [x] Main compiler stages are explicit; no new public API or extension framework.
- [x] Tests, race check, and vet pass; usage notes match the implementation.

## Validation Result

Public API tests verify complete diagnostics for broken-path recovery, array items, quoted and colliding field names, nested missing fields, unknown properties, and aliases. Existing tests pass. `go test -race ./...` and `go vet ./...` passed; `gofmt -l core` returned no files. Reviewed changes against the pre-edit source copy. The API signature and Directory schema are unchanged. Diagnostic wording changed with the validator; remote HTTP schema references are no longer loaded automatically and this limit is documented.
