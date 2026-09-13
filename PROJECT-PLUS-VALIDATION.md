# Project+ full-source assembly comparison

Current configuration is documented in [CONFIGURATION.md](CONFIGURATION.md).
All category flags default false; only bug fixes default true. Policies and
validation results below retain their original versions unless stated otherwise.

Current configuration groups syntax/target additions in
[extensions](CONFIGURATION.md#extensions). Version 0.9 also makes extra numeric
branch forms a separate default-off choice. The integration results below
retain their original versions and executable hashes. The replay harness
explicitly selects dot-op and branch-expression extensions for its corrected
profile, and disables both for its compatibility profile; this is a harness
choice, not coupling between the assembler settings.

This records **0.6.0-go** runs. Starting with **0.7.0-go**, CLI/config defaults
to `bug_fixes = true`; the byte-identical comparisons below require explicit
`bug_fixes = false`. In 0.8.0-go, also set `dot_op = false` for C++ input
acceptance. The encoder is unchanged. The report files retain their
original executable hashes, rather than claiming new Project+ runs.

Run on 2026-09-12 using the **0.6.0-go** executable (Go 1.27.1,
legacy dialect), the retained C++ GCTRealMate v0.2.6 executable, and the actual
executable bundled with the local Project+ installation. All runs explicitly
set **`allow_non_console_instructions = false`**. With the
explicit **`bug_fixes = false`**, all six untouched entrypoints build and match
C++ byte for byte. With **`bug_fixes = true`**, two untouched entrypoints build
and four produce diagnostics. A separate adapted-source comparison builds all
six with fixes enabled and accounts for all 84 changed words.

The input was the local `<project-plus>` installation. The harness copied
its source into separate Go and C++ staging directories and verified that all
168 original files retained their SHA-256 hashes. It did not modify the
installation, deploy any generated codeset, or use the user's emulator profile.

## Explicit compatibility: unmodified sources

The compatibility comparison uses `--no-config --bug-fixes=false
--allow-non-console-instructions=false -i` with legacy
source choices. Go and C++ produce identical GCT hashes for all six entrypoints:

| Entrypoint | Assembly base | Identical GCT bytes |
| --- | --- | ---: |
| `RSBE01.txt` | `80566528` | 101,576 |
| `BOOST.txt` | `80550010` | 63,144 |
| `NETPLAY.txt` | `80566528` | 101,880 |
| `NETBOOST.txt` | `80550010` | 63,208 |
| `Source/Injects/MDEF.txt` | None | 15,920 |
| `Source/Injects/DEFINE.txt` | None | 656 |

The four main entrypoints also use `-a`. No source adaptations apply to this
pass. A subsequent adapted pass also matches C++ exactly with fixes disabled.
Both sets of hashes are recorded in the
[default-mode report](validation/project-plus-compatibility.json).
Reproducing C++ includes its known defects and does not establish game behavior.

### Executable bundled with Project+

A separate comparison used the actual `<project-plus>/GCTRealMate.exe`,
whose SHA-256 is `05ec46d81d7598e5b011ac2eaef7a2fadfb904b7bb95aec1db8ce75ba94d412a`.
This is a different executable from the locally built v0.2.6 reference used in
the original comparison. All six unmodified GCTs match Go byte for byte at the
sizes above; all six adapted GCTs match too. The explicit base/conversion flags
match the bundled `GCTRealMate.ini`, with INI loading disabled for repeatability.
All 168 original source hashes remain unchanged. The
[bundled-executable report](validation/project-plus-bundled.json) records both
executable hashes and every output hash. This confirms the local installation;
other Project+ releases or assembler binaries have not been compared here.

## Corrections: unmodified-source pass

Before any adaptations, both tools compile byte-for-byte copies of the original
source, with INI settings disabled and the same explicit base/conversion flags.
The current corrected-mode report uses the executable bundled with Project+.
With `--bug-fixes=true`, Go builds `MDEF.txt` (15,920 bytes) and `DEFINE.txt` (656 bytes). It rejects
`RSBE01.txt` and `NETPLAY.txt` at an unterminated macro invocation in
`FilePatchCode.asm:677`, reached during expansion at line 716. `BOOST.txt` and
`NETBOOST.txt` fail at the stray trailing word on a hook address in `PSA.asm:214`.
These are the first diagnostics, not a claim that they are the only bad lines.

C++ exits zero and emits GCTs for all six unmodified entrypoints. Its unmodified
RSBE01 and NETPLAY outputs are each 24 bytes shorter than their adapted versions;
the other four sizes are unchanged. Success and framing alone do not establish
correct game behavior. The JSON `unmodified_builds` records process results,
first Go diagnostics, output presence, sizes and hashes separately from the
adapted comparison below. No source repair is credited as untouched compatibility.

## Corrections: adapted outputs compared

| Entrypoint | Assembly base | Go and C++ GCT bytes |
| --- | --- | ---: |
| `RSBE01.txt` | `80566528` | 101,600 |
| `BOOST.txt` | `80550010` | 63,144 |
| `NETPLAY.txt` | `80566528` | 101,904 |
| `NETBOOST.txt` | `80550010` | 63,208 |
| `Source/Injects/MDEF.txt` | None | 15,920 |
| `Source/Injects/DEFINE.txt` | None | 656 |

Every process exited successfully, and every output had valid GCT framing.
The four main entrypoints used absolute-branch conversion (`-a`) and the listed
codeset bases. The comparison found 84 differing 32-bit words:

| Difference | Words | Explanation |
| --- | ---: | --- |
| Full-width directive values | 74 | Go retains address bit 31; C++ truncates it |
| `lha` encoding | 4 | Go emits opcode 42 rather than C++'s opcode 40 |
| Hexadecimal register number | 4 | `cmpw r5,0xD` selects r13; C++ selects r0 |
| Hexadecimal rotate fields | 2 | Go reads the hex fields numerically; C++ clears them |

The hexadecimal register/rotate examples have exact independent GNU vectors in
the regression corpus, and Dolphin checks verify the corrected `lha` behavior.
The 74 full-width differences are second words of `44000000` BA-store codes;
their practical effect under Project+'s handler and memory mapping has not been
verified. Classifying their encoding difference is not a gameplay validation.
The former 64 prediction-bit and 18 NaN-payload differences are gone under the
legacy dialect. Matching sizes and classifying all 84 differences do not prove
that the original or adapted codesets are correct in the game.

## Staging adaptations

The source snapshot contains malformed calls, undefined labels, ambiguous
annotations, and invalid instruction forms. With fixes enabled, the assembler
diagnoses these. The harness makes 16 recorded adaptations in 14 source files, identically
for the Go and C++ staging trees. Some adaptations are interpretations of intent,
not verified game fixes. The complete [staging patch](validation/project-plus-staging.patch)
is the authoritative list of changed lines.

| Source | Staging adaptation |
| --- | --- |
| `RSBE01.txt`, `NETPLAY.txt`, `Community/ItemEx.asm`, `Project+/TexFlags.asm` | Remove the redundant second operand from `crclr 6,6` |
| `Project+/FilePatchCode.asm` | Close an unterminated `%LoadAddress` call |
| `Community/PSA/PSA.asm` | Make a stray word after a hook address a comment |
| `Project+/Debug/Stage Collisions.asm` | Use the defined `storeAlpha` macro for an undefined `setAlpha` call |
| `Project+/MyMusic.asm` | Preserve an unaligned instruction write as raw data `3b7b0010` |
| `ProjectM/Ledge.asm` | Add the missing `bne-` mnemonic in two crawl-direction checks, inferred from surrounding logic |
| `ProjectM/Modifier/VariableSet.asm` | Put the missing `notFighter` label at the existing register-restoring exit |
| `Community/PSA/NewCommands.asm` | Put the missing `forceCorrection` label at the documented command-28 block |
| `Project+/CSE.asm` | Remove a duplicate `didNotFind` label, retaining the default-info block's label |
| `Project+/Debug/Capsule Renderer.asm` | Correct eight `fmulls` spellings to `fmuls` |
| `Project+/Debug/modifiedDebug.asm` | Preserve two invalid `lmw` words as raw data; interpret two `addi.` forms as `addic.`; normalize two `.op` aliases for C++ |

Paths in the table omit the common `Source/` prefix except the root TXT files.
The unaligned MyMusic write and overlapping `lmw` operands deliberately remain
raw data for comparison. They have not been made valid console instructions.
The inferred branch, label, and `addic.` changes also need game-level review
before deployment. No patch was applied to the user's source installation.

The full [corrected-mode JSON report](validation/project-plus.json) includes the
source manifest, tool hashes, output hashes, build logs, adaptations, and
word-level differences. The separate [default-mode report](validation/project-plus-compatibility.json)
records the compatibility comparison. Neither run is a gameplay test or a
deployment of the staging adaptations.

## Reproduction

Use Python 3.9+, the Go executable, and either a locally built C++ v0.2.6
reference or the executable bundled with the installation being checked.
Run from the Go project directory and choose a scratch directory outside the
source installation:

```powershell
python tools/validate_project_plus.py --source-dir C:\path\to\Project+ --assembler bin/gctrm.exe --reference ..\GCTRealMate-source\build\local\GCTRealMate.exe --work-dir C:\scratch\gctrm-project-plus
```

The harness defaults to the compatibility comparison (`--bug-fixes false`),
independently of the CLI's newer default. Repeat with `--bug-fixes true`
and a separate work directory to check the corrected mode. The harness supplies
`--no-config --allow-non-console-instructions=false` and disables INI so local
configuration does not affect the run.
It explicitly disables `.op` in compatibility mode and enables it in corrected
mode to reproduce the historical alias settings, independently of new defaults.

The [harness](tools/validate_project_plus.py) writes both staged trees, logs,
the patch, and `results.json` into that scratch directory. It runs and records
the unmodified pass first; the expected corrected-parser failures in that pass do
not make the adapted comparison fail. It fails on a failed
build, invalid framing, changed original source, unequal output sizes, or an
unclassified difference. With fixes disabled, it additionally requires identical
unmodified and adapted GCT hashes; any difference fails the comparison.
Its source adaptations are specific to this recorded
snapshot and should be reviewed when validating another distribution.
