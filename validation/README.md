# Validation records

These reports retain the versions, hashes, inputs and outcomes of their recorded
runs. Older reports are historical evidence, not executions of the current build.
See [individual-fixes-0.14.0.json](individual-fixes-0.14.0.json) for the current config checks, [checks.json](checks.json) for earlier checks, and
[the compatibility contract](../COMPATIBILITY.md) for their scope.

Personal filesystem paths in reports and fixture diagnostics have been replaced
with placeholders such as `<workspace>`, `<project-plus>` and `<home>`.
The replacements affect location metadata and diagnostic paths, not source
instructions, expected machine words, GCT bytes, executable hashes or outcomes.
The placeholders are not literal paths to use when reproducing a run; supply
your own paths to the validation scripts in `tools/`.

## Individual correction controls (0.13.0)

[individual-fixes-0.13.0.json](individual-fixes-0.13.0.json) records 156 checks
against the rebuilt executable: 39 settings, both states, through TOML and through
a conflicting CLI override. Inputs and expected outputs are retained in
[individual-fixes.json](../internal/cli/testdata/individual-fixes.json), and the
same cases run in `go test ./internal/cli`. Existing reference/GNU fixtures also
pass under `go test ./...`; `go vet ./...` passes. These checks validate the config
split and its encoding behavior, not execution of arbitrary mixed policies on
hardware. Earlier reports retain the settings and binary hashes actually tested.

## Console support classification (0.14.0)

[individual-fixes-0.14.0.json](individual-fixes-0.14.0.json) records 156 passing
executable checks with missing console instructions enabled by default under
`bug_fixes.additional_console_instructions`. Broader PowerPC support is the
independent, default-false `extensions.non_console_instructions`. Unit tests
also verify that bulk fix settings do not change the broader target selection.
The 0.13 report used the earlier fixture at commit `c0bae90`, which included
`console_only` instead; its results are historical. Tests and `go vet` pass.
