# Validation record

Configuration note: these reports retain historical version/settings names. Current 0.13 configuration uses individual `[bug_fixes]` keys, all enabled by default; see [BUG-FIXES.md](BUG-FIXES.md). The old scalar TOML key is no longer accepted.

Current configuration is documented in [CONFIGURATION.md](CONFIGURATION.md).
All category flags default false; only bug fixes default true. Policies and
validation results below retain their original versions unless stated otherwise.

Current configuration groups syntax/target additions in
[extensions](CONFIGURATION.md#extensions). Version 0.9 also makes extra numeric
branch forms a separate default-off choice. The integration results below
retain their original versions and executable hashes.

The current corrected-mode (`bug_fixes = true`) results are in
[CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md):
14,749 independent GNU encoding comparisons, 218 passing Dolphin execution
checks across GameCube/Wii interpreter/JIT, the resolved instruction inventory,
and a Go 1.27.1 executable. All six Project+ entrypoints also build using explicit
staging adaptations. Explicit compatibility mode (`bug_fixes = false`, and
`dot_op = false` starting with 0.8.0-go) matches
all six unmodified C++ builds byte for byte, as documented in
[PROJECT-PLUS-VALIDATION.md](PROJECT-PLUS-VALIDATION.md).
The record below is the original baseline; its coverage numbers, execution
limitations, permissive instruction behavior, and binary hash are historical.

## Original baseline

Run date: 2026-09-12. Host: Windows amd64. Go: `go1.26.5`.

Reference repository: `https://github.com/CodecSMW/GCTRealMate.git`

Reference commit: `9115d23c65c9479e8822968786ac8eec55b7f515` (v0.2.6).
Its C++ source was built locally as an x86 executable with MSVC 14.44, C++20,
and optimization enabled. The tracked reference checkout remains unchanged.

| Check | Result |
| --- | --- |
| `go test ./...` | Pass |
| `go vet ./...` | Pass |
| Statement coverage | 87.0% overall |
| `assembler` coverage | 86.4% |
| `internal/cli` coverage | 82.0% |
| `internal/expr` coverage | 97.8% |
| `internal/ppc` coverage | 89.5% |
| C++ instruction oracle | 327 encodings match exactly |
| C++ complete codeset oracle | `core.asm`: 472 bytes; `includes.asm`: 32 bytes; both match exactly |
| Intentional output differences | Explicit expected-word regression tests pass; this alone does not validate the chosen semantics |
| Concurrent assembly | 20 independent calls pass |
| CLI end-to-end | Example produces an 88-byte GCT plus text and log |
| Windows amd64 build | Pass; delivered in `bin/gctrm.exe` |
| Linux amd64 cross-build | Pass; compilation only |
| macOS arm64 cross-build | Pass; compilation only |
| Expression fuzzing | 877,236 executions, no crash found |
| Instruction fuzzing | 244,530 executions, no crash found |
| Assembler fuzzing | 27,654 executions, no crash found |
| Race detector | Not run successfully: Windows race build requires GCC, which is unavailable |

The three fuzz campaigns ran for approximately ten seconds each. These bounded
runs are useful robustness checks, not proof that all malformed input is safe.
They ran before the final file organization and small parsing refinements; the
full unit/regression suite and vet checks passed again after those changes.

The tiny process entry point is covered by the executed CLI example rather than
unit-test instrumentation. The race command failed while building `runtime/cgo`,
before running any tests. The ordinary Go implementation and builds do not need
CGo or a C compiler.

The generated GCT files have not been executed in a Wii, Dolphin, or a complete
Project+ deployment. Compatibility is limited to the tested cases and the
explicitly documented behavior in [COMPATIBILITY.md](COMPATIBILITY.md).

Delivered Windows executable SHA-256:

```text
DECFEEC92683E40E072B5CE6C88BDB3CC8CD13CC90D2A66F024B513FBAB9C273
```
