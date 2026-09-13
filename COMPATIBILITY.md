# Compatibility contract — 0.14.0-go

The pinned reference is [CodecSMW GCTRealMate v0.2.6](https://github.com/CodecSMW/GCTRealMate/tree/9115d23c65c9479e8822968786ac8eec55b7f515),
built with MSVC on Windows. Its retained source/executable are unchanged.

CLI/config defaults target GameCube/Wii and enable bug fixes; all 39 `[bug_fixes]` keys default true, and all 21 other flags default false.
`[extensions]` adds language/instruction features; `[semantics]` changes numeric
interpretation; `[encoding]` selects byte conventions; `[validation]` adds
restrictions; `[cli]` selects INI/output alternatives. The template has no legacy
table. Only the current schema and categorized keys are accepted.

This preserves C++ octal, arithmetic, representation and permissive source
policies by default. `.op`, expanded expressions and implicit sections require opt-in. Missing console
instructions are enabled by `bug_fixes.additional_console_instructions`. Broader instructions already implemented
by C++ require `extensions.non_console_instructions = true`; the default rejects them.
See [CONFIGURATION.md](CONFIGURATION.md) for defaults, individual switches,
CLI/INI overrides and examples.

The regression suite replays the saved C++ fixtures, and replays historical Go
correctness profiles with their old syntax/validation choices explicitly enabled.
Old Project+/Dolphin reports are historical evidence, not executions of this binary.

## Explicit reference compatibility

C++ output compatibility with `--bug-fixes=false` and `extensions.dot_op = false` is checked against unmodified reference fixtures,
without adjusting expected words for Go fixes. Full historical instruction and
defect corpora explicitly enable non-console support where those cases require it:

- All 327 historical instruction words match, including the reference's
  non-console forms and invalid operand examples, with that opt-in enabled.
- All 66 source captures match exactly, including bare numeric branch labels
  and overlapping explicit BO/`+` encodings formerly excepted from comparison.
- Both complete GCT fixtures match without the four previous word adjustments.
- All 57 defect probes match their GCT/no-GCT outcomes. Reference crashes and
  a hang are represented by bounded errors, not copied process termination.
  This historical corpus also enables non-console forms explicitly.
- The three captured CLI cases match GCT, codeset text, and include-tree logs.
- Seven additional fresh C++ source captures match GCT/text/log output, including
  permissive prefix handling and the malformed odd-word `.GOTO_F` command.
- All six unmodified Project+ entrypoints match the retained C++ GCTs byte for
  byte. A separate run also matches the actual executable bundled with the
  local Project+ installation; see the [report](validation/project-plus-bundled.json).
  No source adaptations are needed for either unmodified comparison.

This is output compatibility with a pinned build, not proof that every possible
input, host compiler, or upstream fork behaves identically. Malformed cases
outside the captured set can differ. Explicit compatibility mode can intentionally
produce malformed Gecko framing or instructions unsuitable for GameCube/Wii.
It must not be confused with the corrected-mode hardware checks below.

## Corrected mode

With `--bug-fixes=true`, the implementation retains the previously validated
corrections: instruction opcodes and fields, paired-single forms, complete
hexadecimal parsing, scanner operators and alias terms, EOF flushing, Gecko
register indices, full-width directive values, BA/PO qualifiers, MEM2 addressing,
PSA tags, label fixups, `.GOTO_F`, and ELSE handling.

Corrections check actual machine-operand counts/widths, branch alignment/range,
illegal suffixes and register relationships, and missing labels. Optional source
checks and grammar additions are selected independently under the new categories.
`extensions.non_console_instructions = true` separately permits recognized non-console forms;
the default `non_console_instructions=false` restricts the target to GameCube/Wii. Raw words are not decoded.

The language choices remain independent. Defaults retain octal values,
unsigned aliases, permissive register prefixes, direction-adjusted branch hints,
zero-extended data slots, and the C++ NaN representations while correcting bugs.
Individual flags select decimal leading zeros, C-style precedence, signed 64-bit
aliases, strict prefixes, GNU hints, sign-extended negative slots, and alternative
quiet-NaN representations. These choices are not themselves CPU bug fixes.

With syntax/instruction extensions explicitly enabled, the tests retain all
14,749 independent GNU vectors: modern
matches exactly; legacy differs only in the characterized prediction bit for
backward conditional branches, which is separately compared with C++. Tests
also retain the previous 57-case Go audit and explicit invalid-input checks.
The Dolphin execution harness explicitly enables fixes and bypasses TOML/INI.
[CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md) records the emulator evidence and
physical-hardware limits. [PROJECT-PLUS-VALIDATION.md](PROJECT-PLUS-VALIDATION.md)
distinguishes explicit compatibility builds from corrected, separately adapted builds.

## Process and file behavior

Native crashes, infinite loops, invalid memory accesses, and exception codes are
not emulated. Limits on source size, nesting, includes, and expansion remain;
include cycles are rejected and context cancellation is supported. Error text,
progress output, and help are Go CLI messages rather than C++ transcript copies.
The CLI is noninteractive: `-q` is accepted, and `-p`/`-c` remain no-ops.

The CLI default returns failure for missing inputs/includes. Explicit
`--bug-fixes=false` reproduces the characterized status-zero behavior while
reporting the missing file and producing no GCT. Other bounded errors can return a nonzero status even where
C++ crashes or hangs; those process differences are explicit compatibility limits.

TOML validation and output-collision checks always apply. Failed assembly leaves
that input's existing GCT intact. Successful writes use temporary-file
replacement per file, not an all-files transaction across multiple inputs.
Library `Result.Text`, `Result.Log`, and `Result.FlatLog` return LF strings; CLI
file serialization chooses native or LF newlines. INI matching can indirectly
change GCT bytes by selecting different assembly options; log layout and newline
formatting alone cannot.
