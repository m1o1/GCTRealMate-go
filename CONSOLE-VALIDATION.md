# GameCube/Wii validation

Current configuration is documented in [CONFIGURATION.md](CONFIGURATION.md).
All category flags default false; only bug fixes default true. Policies and
validation results below retain their original versions unless stated otherwise.

Current configuration groups syntax/target additions in
[extensions](CONFIGURATION.md#extensions). Version 0.9 also makes extra numeric
branch forms a separate default-off choice. The integration results below
retain their original versions and executable hashes. The replay harness
explicitly enables branch expressions to preserve the syntax profile used
by those original runs.

This records the **0.6.0-go** execution/build runs and their executable hash.
Version **0.7.0-go** changes the CLI/config `bug_fixes` default to `true`, with
encoding code unchanged. Version **0.8.0-go** separately enables the `.op` alias
by default; the 0.8 alias-selection checks are in [dot-op.json](validation/dot-op.json).
The 0.7.0 default-selection checks are in
[default-bug-fixes.json](validation/default-bug-fixes.json); the emulator and
Project+ reports below retain their original tested versions and hashes.

Completed 2026-09-12 on Windows amd64 with Go 1.27.1, the current stable release
listed by the [Go project](https://go.dev/dl/?mode=json). This report supersedes
the earlier coverage gaps, permissive PowerPC profile, and execution limitations.
The tested assembler was **0.6.0-go**, targeting GCTRealMate v0.2.6 syntax.
All hardware-correctness and execution checks below explicitly use
**`bug_fixes = true`** and **`allow_non_console_instructions = false`**;
execution uses the legacy source dialect. The former default
`bug_fixes = false` preserves known C++ defects and must not inherit these
correctness claims. See [BUG-FIXES.md](BUG-FIXES.md) for the complete policy.

## Results

| Check | Result |
| --- | --- |
| Independent GNU console comparison | 14,749 words match exactly in modern; legacy differs only in separately tested prediction bits |
| Separate non-console opt-in comparison | 83 words match GNU with fixes enabled; encoding checks only |
| GNU target modes | Gekko and Broadway produce identical bytes for those vectors |
| Independent instruction inventory | 236 names covered; `dcba` explicitly rejected |
| Dolphin GameCube interpreter | 47/47 execution checks pass |
| Dolphin GameCube JIT | 47/47 execution checks pass |
| Dolphin Wii interpreter | 62/62 execution checks pass |
| Dolphin Wii JIT | 62/62 execution checks pass |
| Explicit `bug_fixes = false` Project+ comparison | All six unmodified entrypoints match C++ byte for byte |
| Corrected Project+ assembly in staging | Six entrypoints build with documented source adaptations |
| Corrected Project+ comparison with C++ | Equal output sizes; all 84 differing words accounted for |
| Go unit/regression suite and `go vet ./...` | Pass |
| Delivered Windows executable | Built with Go 1.27.1 and used for all final execution/build checks |
| CLI example | Produces an 88-byte GCT |

Machine-readable results are in [validation/](validation/). Physical console
execution and full Project+ gameplay have not been tested.

## Instruction support and operand validation

The previous list of 20 missing names has been resolved. These 19 instructions
were added, with their applicable record forms:

```text
dcbz_l eciwx ecowx mcrf mcrfs mcrxr mfmsr mfsr mfsrin
mftb mtfsb0 mtfsb1 mtfsf mtfsfi mtmsr mtsr mtsrin tlbie tlbsync
```

`dcba` was a false positive in the earlier name inventory. Dolphin has a table
entry but its [interpreter implementation](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/Interpreter/Interpreter_LoadStore.cpp)
explicitly identifies it as not a Gekko instruction. It is now rejected. The
Broadway instruction tables also do not list it as an implemented instruction.
The independent [inventory fixture](internal/ppc/testdata/dolphin-inventory.json)
contains 237 names from Dolphin, including overflow forms: 236 have GNU encoding
coverage and the unsupported placeholder has a rejection check.

The default target restriction rejects recognized non-console operations,
including 64-bit operations, L=1 comparisons, and unsupported square-root/conversion
forms, with either fix setting. The independent opt-in is documented in
[NON-CONSOLE.md](NON-CONSOLE.md). With fixes enabled, it additionally rejects
out-of-range fields, invalid suffixes, invalid update
loads/stores, statically detectable load-multiple/string register overlaps,
and CTR-decrement forms of `bcctr`. Branches and individual instruction writes
must be aligned. BO bits defined as ignored by the CPU remain accepted.
The rules follow IBM's [Broadway manual](https://pokeacer.xyz/wii/pdf/BroadwayUserManual.pdf),
including its implemented instruction tables and invalid-form definitions.

Time-base reads use TBR 268/269; time-base write aliases use SPR 284/285.
Register prefixes follow the selected source dialect. Legacy preserves the
historical numeric r/f/fr/cr spellings, while modern enforces register classes.
Both dialects enforce the encoded field widths and hardware constraints when
fixes are enabled. With fixes off, characterized unchecked operands remain;
reference-only target forms require the independent non-console opt-in.

Assembly-time checks cannot determine privilege, runtime addresses, XER-dependent
load-string overlap, processor state, or whether a numeric SPR is implemented and
accessible in the executing environment. Raw `word` data intentionally bypasses
instruction validation. Accepted source is not a proof of safe runtime behavior.

## Independent encoding comparison

GNU Binutils 2.37 `powerpc-eabi-as` assembled 15,249 candidate inputs using both
`-mgekko -mregnames -mbig -a32` and `-mbroadway -mregnames -mbig -a32`.
The test reads ELF `.text` bytes and rejects unresolved relocations. Both modes
accepted the same 14,749 vectors with identical words; 500 rejected inputs are
recorded separately. Expected words come from GNU, not this encoder or C++.
GNU documents the target modes in its [PowerPC options](https://sourceware.org/binutils/docs/as/PowerPC_002dOpts.html).

Vectors cover register variations, immediate boundaries, record/overflow forms,
branches, CR fields, SPR/TBR fields, paired-single arithmetic, all quantized W/I
combinations, segment operations, and FPSCR field/mask combinations. Modern follows GNU
prediction defaults; legacy preserves the C++ direction adjustment. Modern is
compared bit-for-bit for all 14,749 vectors. Legacy compares every bit except
the prediction bit on backward conditional branches; independent C++ source
captures cover that bit. Two additional GNU vectors cover explicit BO/hint
overlap. None of these convention changes alters the requested destination.

GNU target selection alone is not a hardware-validity oracle: this GNU version
accepts `fsqrt`/`fsqrts`, which Broadway does not implement. Separate tests check
invalid console forms against the manual. The name inventory detects missing
families independently of the vector generator; neither is exhaustive testing
of every operand combination or CPU behavior.

Fixtures: [accepted words](internal/ppc/testdata/gnu.json),
[GNU rejections](internal/ppc/testdata/gnu-rejected.json).
Generator/comparison: [gnu_test.go](internal/ppc/gnu_test.go).
Console constraints: [console_test.go](internal/ppc/console_test.go).

The GNU executable came from a
[pinned distribution](https://github.com/JLaferri/gecko/blob/e3183ba88332ba7f15a1c36664d9e5fc472d4794/powerpc-eabi-as.exe)
after the devkitPro package endpoint blocked downloads. Its SHA-256 is
`1803871EA124926C2063E22270B282A96AF42BD9E1305783291264317E9FCF82`.

## Execution in Dolphin

Dolphin 2509-149 was obtained from its
[official server](https://dl.dolphin-emu.org/builds/af/75/dolphin-master-2509-149-x64.7z).
The archive SHA-256 is
`767F95CA68ED0EBE3AC18308D4BFD53F9ABD26C28EE9A9A9ACD8225F5D674CED`.
Each run uses an isolated workspace profile, Null video, and the GDB protocol.
The harness does not require a game ROM or alter an existing emulator profile.

The script assembles [runtime.asm](validation/runtime.asm), wraps its CODE
payload in a standalone DOL, executes it, and reads actual memory results.
Each configuration passes 38 CPU checks: arithmetic/logical/branch corrections,
overflow and CR behavior, scalar and paired floating point, quantized loads,
CR/XER transfers, MSR and segment-register access, time-base reads, FPSCR
operations, TLB operations, `dcbz_l`, and external-control loads/stores.

The Wii DOL contains an unexecuted HID4 marker following
[Dolphin's DOL detection](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/Boot/DolReader.cpp).
Observed PVR values are `00083214` for GameCube and `00087102` for Wii.
Both interpreter and JIT pass. These tests establish emulator behavior for
selected inputs, not exhaustive floating-point, exception, or hardware behavior.
In particular, Dolphin models external-control operations as memory access;
the checks do not validate a physical device bus. The `dcbz_l` test enables its
HID prerequisites and observes a cleared mapped cache line; it does not verify
physical locked-cache allocation.

Each run also executes Dolphin's bundled `Sys/codehandler.bin` with Go-generated
GCT data, following the [handler installation](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/GeckoCode.cpp).
Nine checks cover MEM1 writes, PSA data, full-width directive values, CODE writes,
and an installed C2 hook whose payload executes and returns. Wii adds three MEM2
write checks. It then repeats all 12 handler checks with the codeset in MEM2,
including an actual MEM2 C2 payload execution and return. This gives 47 checks
per GameCube configuration, 62 per Wii configuration, and **218 total passes**.

For the MEM2 run the handler stays at `80001800`. The harness changes only its
two codeset-pointer instructions to point at `90001000`, and uses a hook at
`90021000`. The hook and payload are within relative-branch range. This does not
provide an automatic trampoline from a MEM1 codeset to an arbitrary MEM2 hook;
real deployments must choose a compatible placement or supply a trampoline.

Saved results include assembler/emulator hashes and per-check expectations:

- [GameCube interpreter](validation/gamecube-interpreter.json)
- [GameCube JIT](validation/gamecube-jit.json)
- [Wii interpreter](validation/wii-interpreter.json)
- [Wii JIT](validation/wii-jit.json)

## Full Project+ source build

With fixes disabled, all six unmodified entrypoints and all six adapted builds
match C++ byte for byte. With fixes enabled, the unmodified-source pass builds
two entrypoints with Go and diagnoses malformed
input in four; C++ produces GCTs for all six. A separate adapted pass compiles
all six with Go and C++ in independent staging directories. The source snapshot contains malformed
calls, missing labels, and invalid instruction forms, so the comparison applies
explicit, identical adaptations to both staged copies. It does not establish
that the unmodified distribution builds with strict validation or that inferred
source repairs preserve gameplay. All 168 original source files were unchanged.

The six output sizes agree, and all 84 differing words have explicit categories:
full-width directive values, `lha`, hexadecimal register numbers, and hexadecimal
rotate fields. The former branch-hint and NaN differences are gone in legacy mode.
See [PROJECT-PLUS-VALIDATION.md](PROJECT-PLUS-VALIDATION.md) for sizes, source
adaptations, and the complete reviewable patch. No generated codesets were deployed.

## Previously discovered functional corrections

The final tests retain the earlier corrections for MEM2 low address bits,
32-bit BA/PO/Gecko-register values, and PSA Float=1/Bit=2 tags. Format references
include the [Gecko OS handler](https://github.com/iGlitch/GeckoOS/blob/master/Gecko_src/code%20handler/codehandleronly.s)
and [BrawlCrate parameter implementation](https://github.com/BrawlCrate/BrawlCrate/blob/master/Ikarus/Moveset%20Entries/Scripts/Parameter.cs).
The historical 327 C++ samples remain checked in. Valid console samples still
compare with their original operand spellings, including the previously excluded
backward hint case. Invalid/non-console inputs retain explicit rejection checks.
The complete core fixture explicitly accounts for four independently checked
word corrections. [COMPATIBILITY.md](COMPATIBILITY.md) describes all differences.

## Compatibility regression checks

With fixes disabled and non-console forms explicitly enabled, all 327 historical
instruction words and the 57 historical defect probes match their original C++
words and GCT/no-GCT outcomes. All 66 source captures and two complete GCT
fixtures also remain compared. Bounded errors
replace native crashes/hangs. Three CLI captures check INI prefix matching and
nested include logs. Seven additional fresh captures check source quirks and
GCT/text/log output, including odd-word `.GOTO_F` text.

With fixes enabled, 63 of the 66 source captures match exactly and three have
narrowly named, tested corrections. The historical 57 audit cases also run as
corrected-mode regression checks in both dialects. The prior C++/Go audit JSON
remains a labeled historical snapshot.

Instruction and assembly fuzzers exercise both dialects, both fix settings,
and both non-console settings.
Current counts and logs are in [checks.json](validation/checks.json).
Cross-builds for Linux amd64 and macOS arm64 pass; these binaries were not executed.

## Independent TOML choices

The CLI provides 11 independent language/CLI switches plus the separate
`bug_fixes` and `allow_non_console_instructions` flags, described in
[CONFIGURATION.md](CONFIGURATION.md). Tests exercise all 256 source combinations
and all eight CLI combinations, including actual emitted words, register-spelling
acceptance, retained instruction validation, INI matching, log layout and newlines.
Sixteen individual-switch CLI runs verify overriding either preset. Additional
checks cover schema errors, explicit false values, omitted keys, preset resets,
automatic/explicit file selection, existing-output preservation and per-call
isolation. The delivered executable also assembled a mixed configuration with
decimal literals and modern float NaNs while retaining legacy double NaNs.

Additional tests cover omitted/false/true `bug_fixes` with both dialect presets,
CLI and INI overrides, persistence across inputs, and missing-input error status.
The flag remains independent when a fresh dialect preset is selected. Unit
tests additionally cover 36 target/fix/preset/config/CLI combinations and 24
target/fix/source-form combinations, including macros and includes. These
tests verify the target restriction separately from encoding corrections. Unit
tests, vet, module checksum verification, bounded fuzz campaigns, cross-builds,
and 218 corrected-mode Dolphin checks pass. Current and explicitly labeled
historical fuzz results are recorded in [checks.json](validation/checks.json).
The documented examples were also checked against the 0.6.0 executable:
105 runs, including 76 source-table cases and all four TOML recipes. Those results
are saved in [configuration-examples-0.6.0.json](validation/configuration-examples-0.6.0.json).
Current-version example checks are in [configuration-examples.json](validation/configuration-examples.json).

## Reproduction

Ordinary `go test ./...` runs the checked-in independent vectors without network
access or external tools. Run `go vet ./...` for static checks. To deliberately
regenerate GNU vectors:

```powershell
$env:GCTRM_GNU_AS = 'C:\path\to\powerpc-eabi-as.exe'
$env:GCTRM_UPDATE_GNU = '1'
go test ./internal/ppc -run '^TestRefreshGNUCorpus$' -v
Remove-Item Env:GCTRM_UPDATE_GNU
go test ./internal/ppc -run '^TestGNUCorpus$|^TestConsoleInstructionInventory$' -v
```

The execution harness uses Python 3.9+ and portable Dolphin with its `Sys`
directory. It writes inside the supplied work directory and stops its own child:

```powershell
python tools/validate_dolphin.py --dolphin C:\path\to\Dolphin.exe --assembler bin/gctrm.exe --work-dir C:\scratch\gctrm-wii --console wii --cpu interpreter
```

The harness explicitly passes `--no-config --bug-fixes=true
--allow-non-console-instructions=false -i` to the assembler.
Repeat with `--console gamecube` and/or `--cpu jit`. The script is
[validate_dolphin.py](tools/validate_dolphin.py). Project+ reproduction is described
in its separate report. Fuzz checks and their final counts are recorded in
[checks.json](validation/checks.json).

Tested 0.6.0-go assembler SHA-256:

```text
2560459502AA805195166B4515FF2BFE33FCCC66E2B3E67C1A43072A57CE42FB
```

The workspace-local Go archive was verified against the Go project's SHA-256:
`a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d`.

## Remaining limits

No connected physical GameCube/Wii was available for a hardware run. The work
completes the identified software gaps and available independent/emulator checks;
it does not certify every CPU state, exception, peripheral, cache side effect,
Gecko directive, or gameplay path. These limits remain even when assembly and
all listed tests pass.
