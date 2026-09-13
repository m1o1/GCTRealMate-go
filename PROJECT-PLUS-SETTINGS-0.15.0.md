# Project+ defaults and option comparison: 0.15.0

**Current defaults do not reproduce the packaged assembler byte for byte.**
On 2026-09-13, the packaged executable assembled all six untouched entrypoints;
Go 0.15.0 assembled only `DEFINE.txt`, whose output matched. This run also found
a Go expression-grammar compatibility regression. No assembler implementation,
configuration default, or Project+ installation file was changed during this audit.

The [machine-readable report](validation/project-plus-settings-0.15.0.json)
records binary/config hashes, 168 original source hashes, diagnostics, output
hashes, word differences, independent option runs and small reproductions.
It supersedes earlier claims about what the **current** configuration reproduces.
The older reports remain records of their older binaries and profiles.

## Tested inputs and executables

- Go: `0.15.0-go`, source commit `90d9eac`, executable SHA-256
  `b9481f16427329a5bc1c6c52285ba88eb63d5594afed7bbfbe83e33c812b8908`.
- Actual executable bundled with the tested Project+ installation, SHA-256
  `05ec46d81d7598e5b011ac2eaef7a2fadfb904b7bb95aec1db8ce75ba94d412a`.
- The original 168 source hashes match the earlier bundled-executable snapshot.
  Every original hash remained unchanged after this run.
- Both programs used `-q -i -t -l`. The four main entrypoints also used `-a`
  and their packaged INI assembly bases: `80566528` for RSBE01/NETPLAY,
  `80550010` for BOOST/NETBOOST. INI loading was disabled for repeatability.
- Explicit `--config gctrm.toml` and compiled `--no-config` defaults produced
  identical results: all 39 fixes true and all 20 other options false.

## Untouched sources

| Entrypoint | Packaged GCT bytes | Current Go defaults |
| --- | ---: | --- |
| RSBE01 | 101,576 | Fails: `crclr 6, 6` has an extra operand, line 410 |
| BOOST | 63,144 | Fails: unaligned instruction write, `MyMusic.asm:152` |
| NETPLAY | 101,880 | Fails: `crclr 6, 6` has an extra operand, line 418 |
| NETBOOST | 63,208 | Fails: unaligned instruction write, `MyMusic.asm:152` |
| MDEF | 15,920 | Fails: `C_Stick_Off+0x60` data expression rejected, `C-Stick.asm:21` |
| DEFINE | 656 | Succeeds and matches exactly |

These are the first diagnostics, not an exhaustive list of rejected lines.
An unsuccessful build is not counted as an output comparison.

Turning all fixes off **alone** still fails five entrypoints because
`extensions.expression_syntax=false` rejects data addition. For this snapshot,
the following explicit workaround makes all six untouched GCTs byte-identical
to the packaged executable:

```text
--no-config --bug-fixes=false --set=extensions.expression_syntax=true
```

This profile reproduces reference defects. It is neither the defaults nor a
recommendation to deploy unchecked outputs.

## Independent option effects

The first matrix uses the untouched sources and the compatibility workaround
above as its baseline. Each setting is changed independently. Counts sum across
the six generated GCTs; shared code included in several entrypoints is counted
once per output, not once per unique source instruction.

| Setting changed to true | Observed effect |
| --- | --- |
| `encoding.gnu_branch_hints` | 64 words change: RSBE01 18, BOOST 14, NETPLAY 18, NETBOOST 14. Every change is exactly bit `00200000`; sizes remain unchanged. |
| `encoding.alternative_float_nan` | 18 words change from `7FFFFFFF` to `7FC00000`: RSBE01 8, NETPLAY 8, DEFINE 2. Sizes remain unchanged. |
| `encoding.alternative_double_nan` | No Project+ output changes. A direct-write probe verifies `7FFFFFFF FFFFFFFF` becomes `7FF80000 00000001`. |
| `validation.strict_register_prefixes` | All six succeed with identical bytes. A separate probe rejects `fadd r3,r4,r5`; `fadd f3,f4,f5` still emits `FC64282A`. |
| `extensions.dot_op` | RSBE01 and NETPLAY each gain 16 bytes by activating two `.op nop` writes. Later offsets shift; positional word differences are not counts of changed instructions. |
| `extensions.branch_expressions` | No change in this source snapshot. |
| `extensions.implicit_sections` | No change in this source snapshot. |
| `extensions.non_console_instructions` | No change in this source snapshot. |
| `semantics.c_operator_precedence` | No change in this source snapshot. |
| `semantics.signed_64_bit_aliases` | No change in this source snapshot. |
| `encoding.sign_extend_data_slots` | No change in this source snapshot. |
| `validation.reject_duplicate_labels` | RSBE01/NETPLAY fail at duplicate `didNotFind`, `CSE.asm:375`; the other four outputs match. |
| `validation.strict_macro_calls` | RSBE01/NETPLAY fail at the unterminated `%LoadAddress` call, `FilePatchCode.asm:677`, expanded from line 716; the other four match. |
| `validation.reject_undefined_macros` | RSBE01/NETPLAY fail at `%setAlpha`, `Stage Collisions.asm:397`; the other four match. |
| `validation.reject_address_annotations` | BOOST/NETBOOST fail at `PSA.asm:214`; the other four match. |
| `validation.reject_data_overflow` | No change in this source snapshot. |
| `cli.exact_ini_matching` | Not exercised because `-i` disables INI loading. |
| `cli.flat_logs` | Changes all six log files; GCTs and codeset text remain identical. |
| `cli.lf_line_endings` | Changes all six codeset text files and logs on Windows; GCTs remain identical. |

The twentieth option, `extensions.expression_syntax`, is already true in this
baseline. Turning it **false** causes the five expression failures described
above. Absence of output changes does not prove an option is ineffective on
other sources. The run tests individual toggles, not all their combinations.

NaN probes use direct data writes, such as `float NaN @ $80001000`, to exercise
the same parsing context in both programs. The default patterns match packaged
output in that context. These payload measurements and branch bit comparisons
do not establish gameplay effects or timing effects.

Register spelling validation is distinct from `bug_fixes.numeric_fields`:
`cmpw r5,0xD` emits `7C050000` in the packaged program and `7C056800` with Go
fixes enabled. Strict prefix checking does not cause this register-number
difference; the numeric parsing fix does. Likewise, `bug_fixes.branch_prediction`
is separate from GNU hint conventions: `bc+ 13,2,0x10` emits packaged `41C20010`
versus corrected `41A20010`, whereas default `bdnz -0x10` matches packaged
`4220FFF0` and the GNU option changes it to `4200FFF0`.

## Adapted sources with fixes enabled

Both programs also used copies with exactly the previously documented
[staging patch](validation/project-plus-staging.patch), affecting 14 source
files. Some edits infer missing source intent; they are not validated game fixes.

Even those adapted copies fail five builds under literal current defaults due
to the expression regression. With **only** `extensions.expression_syntax=true`
added to defaults, all six adapted builds succeed. Compared with the packaged
assembler on those same adapted sources, four outputs differ and two match:

| Difference | Changed 32-bit words across outputs |
| --- | ---: |
| `bug_fixes.directive_bit31` | 74 |
| `bug_fixes.lha` | 4 |
| `bug_fixes.numeric_fields`: hexadecimal register | 4 |
| `bug_fixes.numeric_fields`: hexadecimal rotate fields | 2 |
| Total | 84 |

Sizes match between programs for every adapted input. MDEF and DEFINE match
exactly; the four main outputs contain the differences above. The independent
option matrix on this adapted, fixes-enabled baseline reproduces the 64 branch
hint and 18 float NaN changes, with no strict-register or double-NaN changes.
The staging patch already normalized `.op` and repaired the optional validation
failures, so enabling those options adds no differences in this matrix.

## Compatibility gaps exposed by the probes

1. **Existing data addition is incorrectly gated as an extension.** Both
   `word 2+3` and `word x+4` with `.alias x = 0x1000`, inside a CODE block,
   assemble in the packaged program as 5 and `1004`. Go rejects both with
   `expression_syntax=false`, regardless of the bulk fix setting. Enabling the
   expression option reproduces packaged bytes. The current TOML example
   describing `word 2+3` as added syntax is therefore inaccurate. Previous
   focused config tests missed this; the fresh full-source run and native
   minimal reproductions expose it.
2. **Scalar float/double inside CODE are parsed differently.** For
   `CODE @ $80001000 { float NaN }` on separate lines, packaged emits `FC000000`,
   while Go emits float data `7FFFFFFF`. With `double NaN`, packaged emits one
   word `FFFFFFFF`, while Go emits eight bytes `7FFFFFFFFFFFFFFF`. Finite `1.0`
   reproductions show the same distinction, so this is not a NaN payload
   preference. Go's broader block-data acceptance is not gated by a current
   extension flag. This gap does not occur in the measured Project+ outputs.

These findings remain unresolved in the tested 0.15.0 implementation. They are
documented here rather than changing the executable under test or relabeling
workaround runs as default compatibility. No emulator or hardware execution was
performed in this audit.

## Reproduction

Run the [comparison harness](tools/compare_project_plus_settings.py) with Python
3.11+, a fresh scratch directory, and your local installation:

```powershell
python tools/compare_project_plus_settings.py --source-dir reference/Project+ --assembler bin/gctrm.exe --reference reference/Project+/GCTRealMate.exe --config gctrm.toml --work-dir scratch/settings
```

It writes `results.json`, copies, GCTs, text and logs under the scratch directory.
Exit status 1 records that defaults did not match all six inputs; the report is
still complete. It refuses to reuse an existing scratch directory.

To repeat the adapted matrix, first generate the documented copies with
`tools/validate_project_plus.py --bug-fixes true`, then add
`--adapted-source-dir <that-run>/go` to a fresh comparison run. The new harness
records changed source hashes and selects its own profiles; it does not inherit
the older harness's extension or validation settings. The published JSON also
labels the tested version/commit and classifies the 84 words.
