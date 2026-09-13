# Validation records

These reports retain the versions, hashes, inputs and outcomes of their recorded
runs. Older reports are historical evidence, not executions of the current build.
See [checks.json](checks.json) for the latest recorded checks and
[the compatibility contract](../COMPATIBILITY.md) for their scope.

Personal filesystem paths in reports and fixture diagnostics have been replaced
with placeholders such as `<workspace>`, `<project-plus>` and `<home>`.
The replacements affect location metadata and diagnostic paths, not source
instructions, expected machine words, GCT bytes, executable hashes or outcomes.
The placeholders are not literal paths to use when reproducing a run; supply
your own paths to the validation scripts in `tools/`.
