# Instruction target restriction

In **0.16.0-go**, broader PowerPC support is an optional extension:

```toml
[bug_fixes]
additional_console_instructions = true
[extensions]
non_console_instructions = false
```

`additional_console_instructions=true` fills implemented GameCube/Wii instruction
gaps in C++. `non_console_instructions=false` rejects recognized instructions
that cannot run on these consoles. Set the latter to true to permit the
implemented broader forms. `--bug-fixes=true|false` only controls fixes; it
never changes this extension. Both choices have independent CLI/INI `--set` overrides.

The library exposes `Options.AdditionalConsoleInstructions` and
`Options.AllowNonConsoleInstructions` respectively. Its explicit booleans both
have zero value false; set the first true to match CLI defaults.

This setting does not disable double-precision floating-point support on Wii,
double data, or wider intermediate constant calculations in the assembler.

## Interaction with bug fixes

| `extensions.non_console_instructions` | Relevant encoding fixes | Behavior |
| --- | --- | --- |
| false | either | Reject recognized non-console forms. |
| true | enabled | Permit implemented broader forms with corrected encodings. |
| true | disabled | Permit them with characterized reference encodings. |

For example, `ld r3,8(r4)` is rejected when `non_console_instructions` is false.
With `non_console_instructions=true` and `ds_displacement=false`, it emits `e8640020`, preserving the old
displacement-times-four convention. With `ds_displacement=true`, it treats 8 as a byte
displacement and emits `e8640008`; it checks four-byte alignment and field width.
Corrected mode also encodes the L bit in 64-bit comparisons; compatibility mode
retains the characterized C++ comparison quirks, including discarded L bits.

Target selection does not itself fix console encodings. `lha r3,0(r4)` continues
to emit `a0640000` with `lha=false` or `a8640000` with `lha=true`, regardless of the
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
non-console cases, select all relevant fixes disabled and
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
