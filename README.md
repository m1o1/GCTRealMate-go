# GCTRealMate in Go

A native Go assembler for Gecko codesets and GameCube/Wii PowerPC source,
based on [CodecSMW GCTRealMate v0.2.6](https://github.com/CodecSMW/GCTRealMate/tree/9115d23c65c9479e8822968786ac8eec55b7f515).
The upstream source and validation reference are pinned to that commit.

**0.15.0-go defaults: GameCube/Wii target, all individual fixes on, all other flags off.**
[gctrm.toml](gctrm.toml) groups choices into `[bug_fixes]`, `[extensions]`, `[semantics]`,
`[encoding]`, `[validation]`, and `[cli]`. It contains no `[legacy]` table.
All 39 fix settings default true; the 20 other flags default false. For example, octal notation,
32-bit unsigned aliases, historical register spellings and NaN bytes remain.
Missing console mnemonics are enabled by `bug_fixes.additional_console_instructions`.
`.op`, broader expression grammar and implicit sections require explicit opt-in. Optional source restrictions are separate
from encoding corrections. Recognized non-console instructions are rejected by default;
`extensions.non_console_instructions = true` explicitly permits broader PowerPC forms.

See [CONFIGURATION.md](CONFIGURATION.md) for every flag and its effects,
[BUG-FIXES.md](BUG-FIXES.md) for corrections, and
[COMPATIBILITY.md](COMPATIBILITY.md) for the validation boundary.
Configuration accepts only the documented keys and current schema.
The library has explicit options and no global state.

## Build and run

Requires Go 1.27 or newer. Built and tested with Go 1.27.1.

```powershell
go build -trimpath -o bin/gctrm.exe ./cmd/gctrm
.\bin\gctrm.exe --bug-fixes=true -t -l examples/demo.asm
```

The build command writes the Windows executable to `bin/gctrm.exe`.
On Linux/macOS, use `go build -o bin/gctrm ./cmd/gctrm`.

The example generates `examples/demo.GCT`, `examples/demo_codeset.txt`, and
`examples/demo_log.txt`. Use `--help` for all options. There is no interactive
pause, including when an error occurs.

```powershell
.\bin\gctrm.exe -q RSBE01.txt
.\bin\gctrm.exe -a -b:0x80566528 RSBE01.txt
.\bin\gctrm.exe -g -l RSBE01.txt -b:0x80550010 BOOST.txt
.\bin\gctrm.exe --bug-fixes=true -o custom.GCT examples/demo.asm
```

Options precede the input they affect. Output flags persist between inputs;
`-a`, `-b`, and `-i` reset for each input. Short boolean flags accept `:0` and `:1`.
Use `--set=section.key=true|false` for individual choices; these long options
persist across inputs. For example, `--set=semantics.c_operator_precedence=true`
selects C-style expression precedence without changing bug fixes or other settings.

Include-tree logs and native line endings are the default. Alternatives are
`--set=cli.flat_logs=true`, `--set=cli.lf_line_endings=true`, and
`--set=cli.exact_ini_matching=true`.
The executable's optional `.ini` file can supply per-input defaults:

```ini
RSBE01.txt : -a -b:0x80566528
BOOST.txt : -b:0x80550010
```

## Individual compatibility settings

Edit [gctrm.toml](gctrm.toml) to select categorized choices independently:

```powershell
.\bin\gctrm.exe --config gctrm.toml source.asm
```

```toml
version = 2
[bug_fixes]
additional_console_instructions = true
lha = true # Each fix is individually selectable; all default true.

[extensions]
dot_op = false

[semantics]
c_operator_precedence = false

```

The CLI reads `<executable-basename>.toml` beside itself. Use `--config` for the
root template. `--no-config` skips TOML; `-i` separately skips INI. Set any option
through CLI/INI with `--set=section.key=true|false`. Config, INI and CLI precedence
is described in the [guide](CONFIGURATION.md#loading-and-overrides).

## Source support

- Raw Gecko lines, named sections, disabled sections beginning with `!`.
- `HOOK`, `CODE`, `PULSE`, individual `op`/`.op` writes, and MEM1/MEM2 addresses.
- Integer, floating-point, scalar, address, string, array, and PSA variable data.
- Includes, scoped aliases, parameterized macros, local branch labels,
  `%START%`, and `%END%`.
- Integer arithmetic and logical instructions, loads/stores, branches,
  comparisons, rotates, shifts, condition/special registers, floating-point,
  paired-single, and quantized paired-single instructions.
- Cache, segment, TLB, MSR, time-base, and FPSCR operations, with console
  instruction and operand checks when fixes are enabled. Privilege and hardware state remain runtime concerns.
- `.BA`, `.PO`, `.GRn`, `.GOTO`, `.GOTO_T`, `.GOTO_F`, `.RESET`, `.ENDIF`,
  `.ELSE`, their reset variants, and `.END`.

Includes without `./` or `../` resolve from the root codeset's directory.
Explicit `./` and `../` paths resolve from the including file's directory.
Aliases and labels are case-insensitive; macro names are case-sensitive.
Each named section owns its aliases/macros; assembly blocks inherit those
definitions and can add temporary local definitions.

With `extensions.expression_syntax = true`, integer expressions support `$`/`0x` hexadecimal, `0b` binary, decimal,
parentheses, unary `+ - ~`, and `* / % + - << >> & ^ |`. Leading-zero integer
literals are always octal (`010` is 8; `08` is invalid). Legacy uses left-to-right
binary evaluation and unsigned 32-bit alias arithmetic; modern uses C-style
precedence and signed 64-bit alias arithmetic. With fixes disabled, the C++ scanner and alias quirks also
apply. See the compatibility contract for operand-field details.
Strings support quoted Go-style escapes, such as `\n` and `\"`.

See [COMPATIBILITY.md](COMPATIBILITY.md) for deliberate corrections and behavior
differences. This is not a claim of compatibility with every existing codeset.
See [CPP-BUGS.md](CPP-BUGS.md) for the confirmed C++ defects and detailed
explanations of intentional language and representation differences.
See [CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md) for independent GNU comparisons,
Dolphin execution, the resolved instruction inventory, and remaining hardware limits.
With `extensions.non_console_instructions = false` (the default), the assembler rejects recognized non-console forms independently
of the bug-fix policy. It retains GCTRM
source syntax and does not provide GNU object files, relocation, or linking.

## Code organization

| Location | Responsibility |
| --- | --- |
| `assembler/source.go` | Statement scanning, quoting, comments, source positions |
| `assembler/frontend.go` | Scopes, includes, macros, aliases, typed intermediate nodes |
| `assembler/assembler.go` | Public API, label resolution, block layout, GCT framing |
| `assembler/data.go` | Typed data and byte order |
| `assembler/directives.go` | Gecko register and control-flow directives |
| `internal/dialect` | Presets, independent source overrides, resolved rules |
| `internal/expr` | Bounded integer expression parser |
| `internal/ppc/table.go` | Declarative instruction definitions and field layouts |
| `internal/ppc/encode.go` | Common encodings and pseudoinstructions |
| `internal/ppc/validate.go` | Console profile, operand classes and register constraints |
| `internal/ppc/branch.go` | Branches, labels, address calculations, range checks |
| `internal/ppc/special.go` | Special-register and paired-single memory encodings |
| `internal/cli` | CLI/INI/TOML policy, output paths, atomic file replacement |
| `cmd/gctrm` | Process entry point and interrupt cancellation |
| `tools/validate_dolphin.py` | Standalone CPU and Gecko-handler execution checks |
| `tools/validate_project_plus.py` | Unmodified-source builds, then explicit adapted comparison |
| `tools/refresh_compatibility.py` | Reproduce pinned C++ source and CLI captures |

To add an ordinary opcode, add its name/opcode/layout to the instruction table
and a test against an independently known machine word. Specialized formats
belong in their own encoder. To add syntax, create a frontend node and handle it
in the appropriate data/directive/block stage.

## Library API

```go
result, err := assembler.Compile(ctx, "RSBE01.txt", assembler.Options{BugFixes: true})
if err != nil {
    return err
}
gct := result.Bytes() // independent byte slice, big-endian GCT
text := result.Text(true, false)
_ = gct
_ = text
```

Import `gctrm/assembler` from this module. `Assemble` accepts source bytes;
the library's plain `BugFixes` boolean remains explicit. Set it to `true` to
match the CLI's corrected default; zero-value options retain C++ quirks.
`Options.ReadFile` can supply an include loader for an editor or virtual
filesystem. Calls have no shared mutable assembler state. The caller should
not mutate options or source data during a call, or a result while reading it.
The local module path can be changed when publishing under your own repository.

## Verification

```powershell
go test ./...
go vet ./...
go test ./... -cover
go test ./internal/expr -run '^$' -fuzz FuzzEval -fuzztime 10s
go test ./internal/ppc -run '^$' -fuzz FuzzEncode -fuzztime 10s
go test ./assembler -run '^$' -fuzz FuzzAssemble -fuzztime 10s
```

Checked-in fixtures include **14,749 instruction encodings from GNU as** in
Gekko/Broadway modes and an independent inventory covering 236 instruction names
plus the explicitly unsupported `dcba` placeholder. With fixes disabled and the
non-console opt-in enabled, all 327
historical C++ instruction words, two complete GCT fixtures, and 66 source
captures match without changing expected words. The 57 defect probes retain
the reference's GCT/no-GCT outcomes, using bounded errors for native failures.
Three CLI captures and seven additional source captures verify GCT/text/log
output. Separate corrected-mode tests check the intended encodings and reject
invalid console forms; the historical probes guard corrections in both dialects.
With fixes enabled, modern matches the GNU words exactly. Legacy excludes only the prediction bit
on backward conditional branches from that comparison and tests it against C++.
Once the pinned TOML dependency is cached, normal tests need neither external
assemblers nor network access.
Separate tests cover format fixes, invalid operands, scoping, output preservation,
settings, and concurrency.
Another 83 GNU vectors check the implemented broader forms with both the
non-console opt-in and bug fixes enabled; those are not console execution tests.

The corrected mode passed **218 execution checks in Dolphin** across GameCube/Wii
interpreter/JIT configurations, including actual MEM1 and MEM2 C2 hook execution.
The prior comparison with fixes and `.op` disabled matches all six unmodified
Project+ GCTs byte for byte. Select all `[bug_fixes]` options set to false (or `--bug-fixes=false`) and `extensions.dot_op = false`
to retain those compatibility settings in 0.15.0-go.
Corrected mode builds two unmodified entrypoints and diagnoses malformed input
in four; separately adapted builds retain 84 explained differences from C++.
These assembly comparisons are not gameplay tests.
[CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md) and
[PROJECT-PLUS-VALIDATION.md](PROJECT-PLUS-VALIDATION.md) record reproduction,
source references, and limits. Physical-console execution remains untested.

To deliberately refresh the oracle outputs, build the pinned upstream C++
reference first and place its executable at `reference/GCTRealMate.exe`:

```powershell
$env:GCTRM_REFERENCE = (Resolve-Path 'reference/GCTRealMate.exe').Path
$env:GCTRM_UPDATE_GOLDENS = '1'
go test ./assembler -run TestRefreshReferenceFixtures -count=1
go test ./internal/ppc -run TestReferenceCorpus -count=1
Remove-Item Env:GCTRM_UPDATE_GOLDENS
go test ./...
```

Compatibility-mode tests use the original C++ expected bytes. Corrected-mode tests
separately account for the documented encoding fixes and rejection of invalid
console forms. See
[CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md) for current results;
[WII-AUDIT.md](WII-AUDIT.md) and [VALIDATION.md](VALIDATION.md) retain earlier
audit snapshots with their original limitations clearly marked.

## Attribution

Based on the GCTRealMate project and its contributors. The retained reference
commit is `9115d23c65c9479e8822968786ac8eec55b7f515` (v0.2.6). The Go implementation
reorganizes the design rather than translating the C++ classes line by line.
The upstream [GNU GPL version 3 license](COPYING) is preserved.
