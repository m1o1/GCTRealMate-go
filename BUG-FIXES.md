# Individual fixes

Version **0.15.0-go** defaults every option in `[bug_fixes]` to **true**.
All options in the other five tables default to **false**. An omitted key keeps
its default; the table is not an all-or-nothing switch.

```toml
version = 2
[bug_fixes]
additional_console_instructions = true
lha = false # Restore only this C++ opcode defect; other fixes stay enabled.
```

`additional_console_instructions` fills gaps in the C++ reference's GameCube/Wii
instruction support and defaults true. Broader PowerPC support is the separate
`extensions.non_console_instructions` option, which defaults false.

Use `--set=bug_fixes.lha=false` for an individual CLI/INI override.
`--bug-fixes=true|false` is a bulk CLI/INI convenience: it sets **every** option
in this table, including `additional_console_instructions`, at that point in option order. A later
`--set` overrides just one choice. There is no master TOML boolean; old scalar
`bug_fixes`, `bug_fixes.console_only`, and `extensions.additional_console_instructions` keys are rejected. The bulk switch never changes non-console support.

```powershell
# All corrections enabled, with just the lha correction disabled.
.\bin\gctrm.exe --no-config --set=bug_fixes.lha=false -i source.asm
# All characterized quirks, while still restricting the target to GameCube/Wii.
.\bin\gctrm.exe --no-config --bug-fixes=false --set=extensions.non_console_instructions=false -i source.asm
```

The following is the complete current list. Each key is independently selectable.
Detailed C++ evidence remains in [CPP-BUGS.md](CPP-BUGS.md).

## additional_console_instructions

**Default: true.** Enable implemented GameCube/Wii mnemonics absent from the C++ reference. This includes the additional cache, synchronization, system/segment-register, atomic-memory and floating-status instructions, plus the implemented aliases. The [configuration reference](CONFIGURATION.md#additional_console_instructions) lists the full set.

Example: `mfsr r3,sr0` emits `7C6004A6` with true. With false it is unavailable: `unknown_instructions=true` reports an error, while false preserves the characterized C++ fallback. It does not enable non-console forms.

## lha

**Default: true.** Encode algebraic halfword loads instead of zero-extending loads.

Example: lha r3,0(r4): false emits A0640000; true emits A8640000.

## eqv

**Default: true.** Use both supplied source registers instead of repeating the first.

Example: eqv r3,r4,r5: false uses r4 twice; true uses r4 and r5.

## crandc

**Default: true.** Encode CR AND-complement rather than CR AND.

Example: crandc 1,2,3: false encodes crand; true complements the second source.

## crorc

**Default: true.** Encode CR OR-complement rather than CR OR.

Example: crorc 1,2,3: false encodes cror; true complements the second source.

## overflow_suffix

**Default: true.** Set OE/Rc bits instead of adding decimal 400/401.

Example: addo r3,r4,r5: false emits 7C642BA4; true emits 7C642E14.

## shift_right_zero

**Default: true.** Prevent a zero-distance srwi from carrying into the destination field.

Example: srwi r3,r4,0: false emits 5484003E; true emits 5483003E.

## quantized_displacement

**Default: true.** Insert only the signed 12-bit quantized displacement.

Example: psq_l f0,-8(r3),0,0: false borrows into base/W/I fields; true preserves those fields.

## indexed_quantized

**Default: true.** Encode the existing indexed psq load/store forms and update selectors.

Example: psq_lx f1,r3,r4,0,0: false reports a bounded failure; true assembles it.

## paired_single_record

**Default: true.** Honor supported paired-single record suffixes.

Example: ps_add. f1,f2,f3: false ignores the dot; true sets Rc.

## numeric_fields

**Default: true.** Parse complete hexadecimal register and numeric fields instead of decimal prefixes.

Example: cmpw r5,0xD: false selects register 0; true selects register 13.

## comparisons

**Default: true.** Correct generic comparison operand selection, cmpli routing, and comparison L encoding.

Example: cmpli 0,0,r3,1: true compares r3 to immediate 1; false follows the incorrect register-comparison path. L=1 still requires extensions.non_console_instructions=true.

## branch_prediction

**Default: true.** Set the explicit BO prediction bit without carrying into another BO bit.

Example: bc+ 13,2,0x10: false increments BO to 14; true keeps BO 13. The GNU hint convention is a separate option.

## raw_data_eof

**Default: true.** Flush pending raw bytes at the end of input.

Example: A final byte 0x12 is dropped with false and padded/emitted with true. Section transitions flush in either mode.

## scanner_multiply

**Default: true.** Preserve multiplication operators in outer-scanned source.

Example: A standalone .GR4 *= 00000002 loses its star with false; true emits the multiply operation. Inline op scanning is unchanged.

## scanner_or

**Default: true.** Preserve OR operators in aliases and GR directives.

Example: A standalone .GR4 |= 00000002 is treated as continuation with false; true emits the OR operation. With unknown_instructions=true, the truncated directive is rejected.

## alias_terms

**Default: true.** Evaluate every alias term instead of losing later accumulator updates and partially parsing numbers.

Example: .alias x = 2 + 3 + 4: false gives 5; true gives 9. Scanner fixes and expression extensions remain separate.

## gr_index

**Default: true.** Preserve the selected GR index in load/store directives.

Example: .GR5 <-(16) $00001000: false omits index 5; true encodes GR5.

## address_qualifiers

**Default: true.** Parse the supported BA/PO and GR qualifiers correctly.

Example: .BA = PO+$1000: false reports a bounded failure; true assembles it.

## directive_bit31

**Default: true.** Preserve the high bit of BA/PO/GR directive values.

Example: .PO = $90000000: false stores 10000000; true stores 90000000.

## mem2_writes

**Default: true.** Split direct-write addresses into BA and the correct offset.

Example: word 0x11223344 @ $90001000: false targets the wrong offset; true writes to 90001000.

## mem2_hooks

**Default: true.** Use BA and the correct offset for MEM2 hooks.

Example: HOOK @ $90001000 with a blr body: false sets PO with zero hook offset; true targets 90001000.

## psa_tags

**Default: true.** Preserve bank/type tags in direct PSA writes.

Example: RA_float 3 @ $80001000: false keeps only index 3; true preserves the RA/float tags. Block data is unchanged.

## goto_false

**Default: true.** Emit the second word of .GOTO_F commands.

Example: .GOTO_F -> next emits a complete pair with true. False emits only its first word; other checks may reject the malformed result.

## gecko_label_offsets

**Default: true.** Insert Gecko label offsets into the low field and check their range/alignment.

Example: A backward .GOTO -> back preserves command bits with true; false can borrow into them.

## else_directives

**Default: true.** Emit the supported .ELSE and .ELSE_RESET commands.

Example: .ELSE fails safely with false; .ELSE_RESET is omitted. True emits the appropriate pair for either form.

## missing_labels

**Default: true.** Diagnose unresolved PPC and Gecko labels instead of retaining zero displacements.

Example: b missing with no such label: false emits a zero-distance branch; true reports an error.

## unknown_instructions

**Default: true.** Require recognized instruction names rather than reference prefix/fallback acceptance.

Example: garbage r3,r4,r5: false emits FFFFFFFF; true reports an error. This also covers fmulls and unknown CR-like names.

## operand_counts

**Default: true.** Reject extra machine-instruction operands instead of ignoring them.

Example: add r3,r4,r5,r6: false ignores r6; true reports an error. Missing required operands always fail safely.

## operand_ranges

**Default: true.** Check encoded register/numeric field widths and immediate ranges.

Example: add r32,r4,r5: false allows the field to overflow; true reports an error. Hex parsing is controlled by numeric_fields.

## suffix_validation

**Default: true.** Reject unsupported record/overflow suffixes.

Example: addi. r3,r4,1: false ignores the dot; true reports an error. Legal OE and paired-single Rc encoding have separate fixes.

## register_relationships

**Default: true.** Reject statically invalid memory-register combinations and CTR branch conditions.

Example: lwzu r3,4(r3): false accepts overlapping registers; true rejects them. bcctr with a BO that tests/decrements CTR is also rejected.

## branch_ranges

**Default: true.** Reject PPC branch displacements that are unaligned or out of range.

Example: b 0x2000000: false masks the displacement; true rejects it.

## address_alignment

**Default: true.** Require word-aligned PPC write/block addresses.

Example: op nop @ $80001001: false accepts it; true reports an alignment error.

## psa_index_range

**Default: true.** Check that PSA variable indices fit 24 bits.

Example: RA_float 0x1000000 @ $80001000: false truncates the index; true rejects it.

## gecko_line_framing

**Default: true.** Reject sections with an incomplete eight-byte Gecko line.

Example: With goto_false=false, a lone .GOTO_F can leave an odd word count: true rejects it; false permits that malformed framing.

## ds_displacement

**Default: true.** Encode DS-form offsets as aligned byte displacements rather than multiplying by four.

Example: With extensions.non_console_instructions=true, ld r3,8(r4) emits E8640020 with false or E8640008 with true. Misaligned offsets are rejected with true.

## missing_file_status

**Default: true.** Return failure status for missing input/include files.

Example: .include absent.asm produces no GCT either way: false returns status 0; true returns nonzero.

## text_line_termination

**Default: true.** Terminate a final odd word in text output before the section separator.

Example: For a three-word section, false ends the last word with one newline; true adds its line terminator plus the blank separator. Such sections also require gecko_line_framing=false.

## Interactions and compatibility

Independent switches do not guarantee that every combination successfully
assembles every input. Validation stays active until its own switch is disabled:
for example, restoring truncated `.GOTO_F` output can trigger `gecko_label_offsets`
or `gecko_line_framing`. Reproducing that malformed output requires disabling those
checks too; `text_line_termination` affects only its text rendering. Restoring
scanner operator loss can prevent the alias evaluator from receiving an expression.

The missing-console-instruction fix enables the implemented console mnemonics.
Other corrections do not enable a syntax extension, alternative
numeric semantics, NaN representation, or optional source validation policy.
Enabling `extensions.non_console_instructions` affects recognized instructions; raw words are never
decoded to enforce a CPU target.

The library uses `Options.Fixes` (`fixes.Policy`). `fixes.All()` enables the
corrections; the zero policy preserves characterized quirks. Target availability
remains `Options.AllowNonConsoleInstructions`: it maps to `extensions.non_console_instructions` and defaults false. `Options.AdditionalConsoleInstructions=true` enables the missing-console-instruction fix. The CLI additionally applies `missing_file_status`.

All-off reference behavior and all-on corrected behavior remain covered by the
existing instruction/source fixtures. Earlier GNU/Dolphin and Project+ reports
record the executable and settings actually tested; they do not establish CPU
correctness for arbitrary mixed configurations. Crashes, hangs, unsafe accesses,
and process-specific exception codes are not reproduced. Resource limits,
include-cycle detection, config validation, output protection, and cancellation
remain active. See [CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md).
