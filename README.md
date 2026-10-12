# Superlexical

Superlexical stores structured records in linked Markdown files. The core compiler validates a folder without changing its files. Prism supplies the UI. Other services own relational and vector indexes.

This repository uses one root Go module. The compiler library is in `core`; the CLI is in `cmd/superlexical`.

## Repository layout

| Path | Contents |
| --- | --- |
| `core/`, `cmd/`, `extensions/` | Superlexical Core: the Go compiler and CLI |
| `api/` | HTTP API for a workspace, built with [huma](https://huma.rocks) |
| `prism/web/` | Prism web app: TanStack Start, React, Tailwind CSS, shadcn/ui theme |
| `prism/mobile/` | Prism iOS app: SwiftUI |
| `ray/` | Ray, the agent service: TypeScript on Bun |

## Develop

Install [Go](https://go.dev), [Bun](https://bun.sh), and Xcode 26 or later. Then install the JavaScript dependencies:

```sh
bun install
```

Start Core, Ray, and the Prism web app. Each one reloads when its source changes:

```sh
bun run dev
```

| Service | Address | Reload |
| --- | --- | --- |
| Core | http://127.0.0.1:4100 | `wgo` builds and starts the server again |
| Ray | http://127.0.0.1:4200 | `bun --watch` |
| Prism web | http://127.0.0.1:4300 | Vite hot module replacement |

Core serves the sample workspace in `core/testdata/workspace`. Prism web sends `/api` requests to Core and `/ray` requests to Ray.

Build the Prism iOS app and start it on a simulator. Give a device name to use a simulator other than iPhone 17 Pro:

```sh
bun run dev:mobile
bun run dev:mobile "iPhone Air"
```

The iOS app uses Core and Ray at `127.0.0.1`. To change the addresses or set a signing team, copy `prism/mobile/Config/Local.example.xcconfig` to `Local.xcconfig`.

Run all checks before you commit:

```sh
bun run check
```

## Use the API

```sh
go run ./cmd/superlexical serve --addr 127.0.0.1:4100 ./core/testdata/workspace
```

| Endpoint | Result |
| --- | --- |
| `GET /health` | Reports that the server is running |
| `GET /workspace/diagnostics` | Validates the workspace and lists its errors |
| `GET /openapi.json` | OpenAPI 3.1 specification |
| `GET /docs` | API documentation |

After you change an endpoint, write the specification and the Prism web types again:

```sh
bun run generate:api
```

This command updates `openapi.json` and `prism/web/src/lib/schema.d.ts`. Commit both files.

## Use the CLI

Build the executable from the repository root:

```sh
go build -o bin/superlexical ./cmd/superlexical
```

Validate the included workspace directly with Go:

```sh
go run ./cmd/superlexical validate ./core/testdata/workspace
```

Run the executable from a workspace root:

```sh
superlexical validate .
```

Validate the single-schema samples:

```sh
./bin/superlexical validate --schema ./extensions/directory/schema.json ./core/testdata/valid
./bin/superlexical validate --schema ./extensions/directory/schema.json ./core/testdata/invalid
```

The valid sample passes. The invalid sample deliberately reports eight errors. The records use fictional people and organizations. The files live in `core/testdata`, the normal Go location for test data, and are checked by the automated tests. They do not need a registry.

Each error identifies its file, line, and field:

```text
people/alice.md:5: name: expected string; found number
people/bob.md:2: name: required field is missing; add it
```

Use another schema, or keep validation running after edits:

```sh
./bin/superlexical validate --schema ./extensions/directory/schema.json ./core/testdata/valid
./bin/superlexical validate --watch ./core/testdata/workspace
```

Flags go before the root path. Exit codes are `0` for valid data, `1` for configuration or document errors, and `2` for a command, schema, configuration-read, root-directory, or watcher setup failure. Validation errors go to stderr; successful checks go to stdout. `--help` shows usage.

For an agent workflow, run `superlexical validate ROOT` after each completed group of edits and give its output back to the agent. A nonzero exit code means the changes have not passed validation. This command always checks the full directory, so it can find broken incoming links after a rename or deletion.

Watch mode runs an initial validation, then uses native filesystem events to trigger another full check after 150 ms without events. It handles new subdirectories and atomic saves, and also watches extension configuration changes and any supplied schema file. It continues after validation errors. Ctrl+C stops it and returns the last validation status. A watcher failure stops with exit code `2`. Watch mode does not create a consistent filesystem snapshot; use the one-shot command after an agent finishes its edits when a final pass is required. Filesystems that do not supply native change events need the post-edit command instead.

## Workspace configuration

The compiler reads `.superlexical/extensions.json` from the selected workspace root:

```json
{
  "extensions": [
    {
      "name": "directory",
      "dependencies": []
    }
  ]
}
```

The `directory` extension uses its embedded schema for all Markdown files under `directory/`. No separate extension manifests, schema paths, or data-folder settings are used. Files outside that folder are not compiled. Assets can be linked from anywhere inside the workspace root. Links to Markdown outside the compiled folders report an error.

The configuration requires an `extensions` array and each entry requires `name` and `dependencies`. An empty extensions array enables no record validation. Unknown extension names, duplicate entries, and nonempty dependencies produce errors. Configuration fields outside this format are rejected. A missing or unreadable configuration cannot pass validation. Configuration diagnostics identify the file and array entry; their line is the start of the file. Missing Directory folders also produce errors.

## Use the compiler

```go
package main

import (
    "fmt"
    "log"
    "os"

    "superlexical/core"
)

func main() {
    diagnostics, err := core.CompileWorkspace(".")
    if err != nil {
        log.Fatal(err)
    }
    for _, diagnostic := range diagnostics {
        fmt.Println(diagnostic)
    }
    if len(diagnostics) != 0 {
        os.Exit(1)
    }
}
```

`CompileWorkspace(root)` reads the extension configuration and checks enabled records. `Compile(root, schema)` checks one folder against supplied JSON Schema bytes. The Directory schema is available as `directory.Schema` from `superlexical/extensions/directory`. The `--schema` CLI option selects this single-folder mode and bypasses workspace configuration.

A successful validation returns no diagnostics and no error. Invalid content returns diagnostics with a root-relative path, a 1-based line number, a field, and a message. Source lines come from the original YAML tree, including array items and quoted property names. Missing fields and alias contents point to the nearest enclosing source location. `Field` is a display label; dots in property names are retained, so it is not a machine-readable path. An unreadable document or link target is reported as a diagnostic, and other files are still checked. An unusable schema or root directory returns an error; callers must not treat that result as valid.

The compiler checks all `.md` files recursively within each selected record folder, including `.MD`. It skips `.git` and `.superlexical` directories. Every compiled Markdown file must have YAML front matter that passes its schema. An existing empty record folder is valid. In single-schema mode, every Markdown file in the selected root is a record.

The compiler reads and parses documents, validates records and IDs, then resolves links. It returns all independently detectable errors in that run. If a YAML block has a syntax error, its metadata cannot be checked further until that syntax is fixed; its readable Markdown body and other documents are still checked. A broken link, including a path through a regular file, produces a diagnostic without stopping the remaining checks.

The caller must keep the folder stable during compilation. This version does not create a filesystem snapshot or manage concurrent edits.

## Minimal Directory schema

Schema.org defines the vocabulary. JSON Schema Draft 7 defines the validation rules. This is a small application profile of [Person](https://schema.org/Person) and [Organization](https://schema.org/Organization), not the full Schema.org vocabulary or a JSON-LD processor.

Each record has four required fields. Extra fields are allowed:

| Field | Rule |
| --- | --- |
| `@context` | Exactly `https://schema.org` |
| `@id` | An absolute URI of at least 21 characters, including its prefix, with no whitespace; unique in the root |
| `@type` | `Person` or `Organization` |
| `name` | A string with at least one non-whitespace character |

These required fields are Superlexical rules. Schema.org itself does not impose them. The compiler does not fetch the context URL. Keep IDs unchanged when names or paths change; a single compilation can check uniqueness, but cannot check identity across revisions.

Agents and UIs create IDs; the compiler never generates or rewrites them. Random IDs are recommended. The minimum length catches short labels but does not prove randomness or Nano ID use. Duplicate checks compare complete IDs across all records in one compilation, not across separate runs. Custom schemas define their own ID format and length rules.

`directory/people/alice.md`:

```markdown
---
"@context": https://schema.org
"@id": person:zt507zg4MOyd_2TGcpFB0
"@type": Person
name: Alice
---

# Alice

Works with [Acme](../organizations/acme.md#team).
```

`directory/organizations/acme.md`:

```markdown
---
"@context": https://schema.org
"@id": organization:DZO8p_bVF1ENdFAYHmxc7
"@type": Organization
name: Acme
---

# Team

[Alice](../people/alice.md) works here.
```

Extra fields can contain values, lists, or nested objects. The built-in schema does not constrain their values. Caller-supplied schemas can validate extra fields, reject unknown fields, and define other record types without changes to the compiler. Use references within the supplied schema, such as `#/definitions/person`. The validator also supports explicit `file:` schema references, so use trusted schemas. Remote HTTP schema references are not loaded.

## Link rules

- CommonMark links, images, autolinks, and reference links are parsed with Goldmark. Text inside code is not a link. Undefined reference labels are plain text under CommonMark.
- Relative paths resolve from the source file's folder. Paths starting with `/` resolve from the selected root.
- Local targets must be existing regular files inside the root. Assets can be link targets. Paths cannot escape the root, including through symbolic links.
- Markdown fragments must match a heading ID generated by Goldmark's `WithAutoHeadingID`. For example, `# Hello World` has ID `hello-world`; a repeated heading gets `hello-world-1`. Consumers should use the same heading rules.
- Fragment-only links resolve within the source document. URL escapes, such as `%20` in filenames, are decoded. Query strings do not affect local file lookup.
- External URLs receive syntax checks only. The compiler makes no requests to link targets and cannot verify remote availability. `file:` URLs are rejected; use relative paths instead.
- Fragments on non-Markdown assets are not interpreted. Raw HTML links and anchors, Obsidian `[[wikilinks]]`, and block references are outside the CommonMark contract.
- Directory symbolic links are not scanned. Markdown files that are symbolic links are rejected. A symbolic link to an asset inside the root can be a link target.

## Dependencies and checks

The core uses three direct dependencies for parsing and validation:

- `goccy/go-yaml`: YAML parsing, source positions, and JSON conversion.
- `santhosh-tekuri/jsonschema/v6`: JSON Schema validation with structured error paths.
- `yuin/goldmark`: CommonMark parsing and heading IDs.

The CLI uses `fsnotify/fsnotify` for native file change events across operating systems. It adds no watcher dependency to the core package.

Run from the repository root:

```sh
go test -race ./...
go vet ./...
```

The tests use checked-in samples and temporary directories. They cover the compiler API, CLI feedback and exit codes, and real file changes in watch mode. This version contains no database, update subscription service, or executable extension runtime.
