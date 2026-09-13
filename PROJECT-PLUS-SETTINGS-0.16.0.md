# Project+ source-context fixes and validation: 0.16.0

Both gaps found in the [0.15.0 audit](PROJECT-PLUS-SETTINGS-0.15.0.md) are resolved.
All six untouched Project+ GCTs now match the actual packaged executable with
`--no-config --bug-fixes=false`. No expression workaround or source edits are
needed for that compatibility comparison.

The [full report](validation/project-plus-settings-0.16.0.json) records the
executable/config hashes, original and adapted source hashes, diagnostics,
output hashes and individual option comparisons. It uses the same packaged
executable as before, SHA-256
`05ec46d81d7598e5b011ac2eaef7a2fadfb904b7bb95aec1db8ce75ba94d412a`.
All 168 original Project+ source files remained unchanged. No output was deployed.

## What changed

Scalar data operands inside PPC blocks now use the reference operand grammar,
which already supports addition. `word 2+3`, `word Alias+4`, and
`word Alias+0x60` work without `extensions.expression_syntax`. The same applies
to pseudo-instruction writes such as `op word 2+3 @ $80001000`. Standalone data
writes and array fields retain their separate grammar. Parentheses, binary
literals and the other expanded expression forms remain optional.

The new **`extensions.floating_point_data = false`** option controls float/double
data in CODE/HOOK/PULSE blocks and four-byte float data in op writes. With it off,
the existing `bug_fixes.unknown_instructions` check diagnoses these forms when
enabled. Disabling that check reproduces the packaged parser's one-word fallback:
`float` emits `FC000000`, and `double` emits `FFFFFFFF`. Enabling the extension
emits actual floating data, with the separately selected NaN patterns. Label
offsets account for the resulting size. `op double` still rejects eight-byte data
when the extension is enabled because an op write must fit one word.

Ordinary `float NaN @ $80001000` and `double 1.0 @ $80001000` work without the new
extension. Fixed-point `scalar` data and CPU floating-point instructions are
unaffected. The TOML template and [configuration guide](CONFIGURATION.md) document
these contexts and examples. All 39 fixes remain enabled by default; all 21
other options default false.

## Full Project+ results

| Entrypoint | Untouched with fixes off | Untouched with current defaults |
| --- | --- | --- |
| RSBE01 | Matches packaged, 101,576 bytes | Extra operand in `crclr 6, 6`, line 410 |
| BOOST | Matches packaged, 63,144 bytes | Unaligned instruction write, `MyMusic.asm:152` |
| NETPLAY | Matches packaged, 101,880 bytes | Extra operand in `crclr 6, 6`, line 418 |
| NETBOOST | Matches packaged, 63,208 bytes | Unaligned instruction write, `MyMusic.asm:152` |
| MDEF | Matches packaged, 15,920 bytes | Succeeds and matches |
| DEFINE | Matches packaged, 656 bytes | Succeeds and matches |

The remaining four default-mode failures are expected effects of the selected
input checks, not the repaired expression bug. These are first diagnostics;
disabling one check does not prove an entrypoint contains no other invalid lines.

Both programs also assembled the same separately adapted staging sources using
the [previously documented patch](validation/project-plus-staging.patch). All six
Go builds succeed with **literal current defaults**, without an expression
override. Output sizes match the packaged builds. The same 84 words differ:
74 directive bit-31 corrections, four `lha` corrections, four hexadecimal-register
corrections, and two hexadecimal rotate-field corrections. MDEF/DEFINE match
exactly; the four main GCTs contain those differences. Some staging edits infer
missing source intent, so these are assembly comparisons rather than validated
game fixes.

## Other settings and regression protection

Independent toggle runs on both successful profiles retain the previous results:

- GNU branch hints change 64 words across the six outputs, each by bit `00200000`.
- Alternative float NaNs change 18 words from `7FFFFFFF` to `7FC00000`.
- Alternative double NaNs and strict register prefixes do not change this
  Project+ snapshot. Focused probes separately verify their behavior.
- Enabling `expression_syntax` or `floating_point_data` adds no changes to these
  Project+ builds. Neither option is now required for compatibility.
- `.op` adds 16 bytes to each untouched RSBE01/NETPLAY output; the adapted patch
  already normalizes those lines. Optional source checks retain their previously
  documented diagnostics on untouched inputs. INI matching is not exercised
  because INI loading is disabled; log/newline choices affect sidecars only.

Counts include shared source each time it appears in an output. The matrix tests
individual options, not every combination, and does not measure gameplay or timing.

[Forty focused packaged-executable cases](validation/source-context-0.16.0.json)
cover literal/alias addition and float/double parsing in CODE, HOOK, PULSE and op
contexts. All compatibility byte comparisons, default-policy checks and extension
checks pass. The native captures are retained in
[source-context-reference.json](assembler/testdata/source-context-reference.json)
and replayed by `go test`. Further tests check branch-label layout, expression
scope, TOML/INI/CLI precedence, NaN-option independence and preservation of existing
output files after errors. `go test ./...` and `go vet ./...` pass.

Reproduce the full comparison with `tools/compare_project_plus_settings.py` and
the same source/reference/config arguments documented in the earlier report,
using a fresh work directory. The current harness uses fixes-off for its untouched
option baseline and literal defaults for its adapted baseline. Reproduce focused
captures with:

```powershell
python tools/validate_source_context.py --assembler bin/gctrm.exe --reference reference/Project+/GCTRealMate.exe --work-dir scratch/source-context
```

The full comparison harness returns 1 when literal defaults do not match all six
untouched entries; its JSON still records the complete successful compatibility
comparison. The focused harness returns zero when all its checks pass.
