# Instruction target restriction

In **0.12.0-go**, GameCube/Wii is the default target. Broader PowerPC forms
require an explicit extension:

```toml
bug_fixes = true
[extensions]
non_console_instructions = true
```

The default false rejects recognized non-console forms. Set true to permit the
implemented broader forms retained from C++; this does not make them executable
on Wii. Use `--set=extensions.non_console_instructions=true|false` in CLI/INI.
The library's `Options.AllowNonConsoleInstructions` boolean maps directly to this
setting and also defaults false. The separate `additional_console_instructions`
extension controls new console mnemonics. See [CONFIGURATION.md](CONFIGURATION.md).

This setting does not disable double-precision floating-point support on Wii,
double data, or wider intermediate constant calculations in the assembler.

## Interaction with bug fixes

| `extensions.non_console_instructions` | `bug_fixes` | Behavior |
| --- | --- | --- |
| `false` | `false` | Reject recognized non-console forms; preserve other characterized C++ quirks. |
| `false` | `true` | CLI/config default: reject recognized non-console forms; apply encoding and machine-operand fixes. |
| `true` | `false` | Permit retained broader forms using characterized C++ encodings and parsing. |
| `true` | `true` | Permit implemented broader forms with encoding and machine-operand fixes. |

For example, `ld r3,8(r4)` is rejected when this extension is false.
With it enabled, C++ compatibility mode emits `e8640020`, preserving the old
displacement-times-four convention. Corrected mode treats 8 as a byte
displacement and emits `e8640008`; it checks four-byte alignment and field width.
Corrected mode also encodes the L bit in 64-bit comparisons; compatibility mode
retains the characterized C++ comparison quirks, including discarded L bits.

Target selection does not itself fix console encodings. `lha r3,0(r4)` continues
to emit `a0640000` with fixes off or `a8640000` with fixes on, regardless of the
non-console setting. Enabling broader forms does not disable corrected-mode
register bounds, operand counts, or update-load restrictions.

## Scope

The retained broader forms include:

```text
ld ldu lwa std stdu ldx ldux lwax lwaux stdx stdux
mulld divd divdu mulhd mulhdu sld srd srad extsw cntlzd
td tdi fctid fctidz fcfid fsqrt fsqrts frsqrtes
cmpd cmpdi cmpld cmpldi
```

Their applicable record/overflow suffixes and explicit L=1 comparisons also
require `extensions.non_console_instructions = true`. Legacy floating-prefix acceptance cannot bypass the target
restriction for recognized non-console encodings.

This is a permission to use implemented broader forms, not a complete general
PowerPC assembler or a promise that every CPU supports them. No individual
non-console CPU model is selected. `dcba` still has no implemented encoder.
The historical `fress`/`fsels` spellings remain available only with the target
opt-in and fixes off; corrected mode rejects them. Output remains a Gecko GCT,
not an ELF object or a linked executable.

Raw `word` values and raw Gecko lines are data and are not disassembled by this
flag. With bug fixes off, unrelated legacy behavior such as unchecked fields or
unknown-instruction sentinel words also remains. The target flag alone is not
a guarantee that every emitted word is a valid console instruction.

## Compatibility and validation

To reproduce the complete historical C++ instruction corpus, including its
non-console cases, select `bug_fixes = false` and
`extensions.non_console_instructions = true`. All 327 captured words remain tested.

Tests cover defaults, both fix settings, independent semantics, TOML/CLI/INI precedence,
source forms/includes/macros, explicit L=1 requests, prefix matching, and output
preservation on failure. The opt-in corrected encodings have **83 independent
GNU as vectors**, saved in
[non-console-gnu.json](internal/ppc/testdata/non-console-gnu.json). These are
encoding comparisons, not execution tests on non-console hardware.

The prior six-entrypoint Project+ comparison matches the bundled C++ assembler
byte for byte with fixes and `.op` disabled. The corrected console
profile has 218 recorded Dolphin checks; these historical execution reports
retain their original executable versions and hashes. See [the Project+ report](PROJECT-PLUS-VALIDATION.md)
and [console validation](CONSOLE-VALIDATION.md).
