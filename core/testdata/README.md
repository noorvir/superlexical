# Compiler samples

These workspaces use fictional records. Tests and the CLI use the same files; no fixture registry or data generator is needed.

- `valid/`: three records with links between people, an organization, headings, and an asset.
- `invalid/`: several independent errors, including invalid fields, a duplicate ID, malformed YAML, and broken links.

Run from the repository root:

```sh
go run ./cmd/superlexical validate ./core/testdata/valid
go run ./cmd/superlexical validate ./core/testdata/invalid
```

The second command is meant to fail. Validate each workspace separately; this README is not a Directory record.
