# Current Work: Validation CLI and feedback

## Goals

Return readable feedback for all independently detectable errors in a full-directory validation. Add a CLI and checked-in example workspaces for agents and people to test.

## Current Direction

Keep Compile(root, schema) as the shared entrypoint. A malformed document must not stop checks on other documents or readable Markdown bodies. Local file and link failures become file diagnostics; unusable root/schema inputs remain setup errors. Explain when broken YAML prevents further metadata checks rather than inventing recovery from an invalid syntax tree.

CLI contract:

```text
superlexical validate [--schema FILE] [--watch] ROOT
exit 0 = valid, 1 = document errors, 2 = setup or command error
output = file + line + field + readable error; final error count
```

Place valid and invalid sample workspaces in testdata. Use them from automated tests and the CLI. Provide optional event-based watch mode and document the one-shot command for post-edit hooks. Use fsnotify in the CLI for native change events; keep it out of the core package. Group event bursts and revalidate the full root.

## Success Criteria

- [x] Report multiple metadata, parsing, identity, and link errors in one run.
- [x] Error messages identify the location and required correction clearly.
- [x] CLI supports default and supplied schemas, help, and meaningful exit codes.
- [x] Checked-in samples cover successful and failed validation.
- [x] Automatic full-directory revalidation is supported through watch mode and a post-edit command.
- [x] Tests, CLI runs against both samples, race check, and vet pass.

## Validation Result

`go test ./...`, `go test -race ./...`, and `go vet ./...` passed. The unreadable-file regression ran without a skip. The built CLI passed the valid sample (exit 0), invalid sample (all nine diagnostics, exit 1), supplied schema (exit 0), missing root (exit 2), and help. A real watch process detected an edit, accepted an atomic repair, and stopped on Ctrl+C. Watch tests also cover new subdirectories, deleted link targets, and external schema file updates. Reviewed the changed core code, CLI, sample data, and usage notes.
