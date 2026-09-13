# Bug fixes and compatibility mode

Version **0.11.0-go** enables corrections by default in CLI/configuration:

```toml
version = 2
bug_fixes = true
```

Every categorized flag defaults to false. Missing/empty automatic configuration
and `--no-config` use these same defaults. `--bug-fixes=false` preserves
characterized C++ v0.2.6 quirks.
The library's plain `Options.BugFixes` remains explicit; set true for corrections.

## What the flag controls

This is one switch for the correction set, separate from the categorized
source, representation, validation and CLI choices. It is not an instruction-by-instruction list of fix toggles.
The following table explains the observable differences. The detailed C++
evidence and original probe inputs are in [CPP-BUGS.md](CPP-BUGS.md).

| Area | `false`: compatibility mode | `true`: CLI/config default |
| --- | --- | --- |
| `lha` | Uses the C++ `lhz` opcode: `lha r3,0(r4)` emits `a0640000`. | Emits `a8640000`, the algebraic halfword load. |
| `eqv` | Repeats the first source register, ignoring the third operand. | Encodes both supplied source registers. |
| `crandc`, `crorc` | Uses `crand`/`cror` encodings from prefix matching. | Encodes the complement operation requested. |
| Overflow suffixes | Adds decimal 400/401 to the instruction word. | Sets the OE/Rc bits. `addo r3,r4,r5`: `7c642ba4` becomes `7c642e14`. |
| Zero-distance `srwi` | The shift value 32 carries into another field: `srwi r3,r4,0` emits `5484003e`. | Masks the shift field, emitting `5483003e`. |
| Negative quantized displacement | Adds the full negative value, borrowing into base/W/I fields. | Inserts only the signed 12-bit displacement. |
| Indexed `psq_*x` forms | Returns a bounded error and no GCT, corresponding to the reference's failed compilation. | Encodes the supported indexed load/store forms, including their update selectors. |
| Paired-single record suffix | Ignores the requested record bit. | Sets Rc for supported record forms. |
| Hex register/numeric fields | Preserves decimal partial parsing: `0xD` is parsed as zero. | Parses the complete hexadecimal field. |
| Generic comparisons and `cmpli` | Preserves the C++ argument selection, including `cmpli` reaching the register-comparison encoder and discarded/misinterpreted L operands. | Encodes the intended comparison. `validation.console_only` separately restricts comparisons to L=0. |
| Explicit BO plus prediction suffix | Addition can carry out of BO's prediction bit, as in `bc+ 13,2,...`. | Sets only the prediction bit. The independent `encoding.gnu_branch_hints` choice still controls the direction convention. |
| Raw data at end of input | Drops the pending raw-byte queue, matching the reference. A section transition still flushes it. | Flushes pending bytes at EOF. |
| Scanner `*` and `\|` | Drops `*` in outer-scanned source and treats `\|` as line continuation. Inline `op ... @` uses the reference's separate scanning behavior. | Preserves multiplication; aliases and GR directives can use OR. |
| Alias terms | Preserves the accumulator bug: `2 + 3 + 4` gives 5; later terms affect a different accumulator. C++ partial numeric parsing is retained. | Consumes the complete expression using the selected precedence and width. |
| GR load/store index | Omits the selected Gecko-register index. | Encodes the requested index. |
| BA/PO qualifiers | Forms broken in C++, such as `.BA = PO+$1000` and `.BA -> GR3+$1000`, return bounded errors. | Parses the supported qualifiers correctly. |
| Directive bit 31 | Masks affected BA/PO/GR operand values with `7fffffff`. | Preserves all 32 bits. |
| MEM2 direct writes | Places the full address in BA and emits zero offset, reproducing the wrong target. | Splits BA and the write offset correctly. |
| MEM2 hooks | Sets PO and emits zero offset, as the C++ hook bug does. | Uses BA and the correct hook offset. Pointer-based MEM2 CODE is unchanged. |
| Direct PSA data | Erases bank/type tags, keeping only the index. | Preserves bank/type tags. Block PSA data already preserves them. |
| `.GOTO_F` | Omits the second word, allowing the reference's malformed command framing. | Emits the complete pair. |
| Backward Gecko label fixups | Adds the signed offset into the whole command, allowing borrowing into control bits. | Inserts the displacement into the low field and checks its range/alignment. |
| `.ELSE`, `.ELSE_RESET` | Plain ELSE returns a bounded error; ELSE_RESET is omitted. | Emits both supported ELSE forms. |
| Missing branch labels | Emits a zero-displacement branch rather than reporting an unresolved symbol. Bare decimal branch targets also follow the old label interpretation. | Diagnoses missing symbols. Additional numeric forms require the separate `extensions.branch_expressions` opt-in. |
| Unknown/misspelled instructions | Preserves known reference acceptance: unknown instructions can emit `ffffffff`; conditional-register names can emit `4c000000`; floating arithmetic prefix matching can accept a misspelling such as `fmulls`. | Requires a recognized instruction name. |
| Invalid operands | Permits extra operands, unchecked field overflow, invalid update-load register relationships, and masked branch ranges where the reference does. | Checks operand counts, widths, suffix legality, register relationships, and branch ranges/alignment. |
| Broader PowerPC encoding (when `validation.console_only = false`) | Retains the reference's encodings, including its DS displacement convention and discarded comparison L bits. | Uses aligned byte displacements for DS forms and encodes the comparison L bit. The target flag controls availability in both modes. |
| Missing input/include status | Reports the missing file and produces no GCT, but returns status 0 like the reference. | Returns a nonzero failure status. |

## Interaction with the other choices

Corrections do not enable any category flag. Expanded expression syntax, `.op`,
additional numeric branch forms, implicit sections and added console mnemonics
are independent extensions. Duplicate-label checks, macro-call strictness,
undefined-macro rejection, address-annotation rejection and data-overflow checks
are independent validation policies. They all default off. Strict register
prefixes are likewise optional, while illegal machine operands remain checked.

The shared expression evaluator corrects lost terms/operators within the selected
grammar. Adding parentheses, binary literals, shifts, or expression-capable fields
is selected by `extensions.expression_syntax`; syntax and arithmetic interpretation
are separate. Preserving native scanner/alias bugs can obscure enabled expression
features, so historical full-expression comparisons explicitly enable fixes.

The console restriction is now `validation.console_only`, default false to
preserve C++ availability.
See [CONFIGURATION.md](CONFIGURATION.md) for all individual flags.

```powershell
# Corrections with reference source/configuration choices.
.\bin\gctrm.exe --no-config -i source.asm
# Characterized C++ encoding/parser quirks too.
.\bin\gctrm.exe --no-config --bug-fixes=false -i source.asm
```

## Compatibility boundary

Compatibility is measured against the pinned Windows C++ executable, not every
fork, compiler, or possible undefined behavior. With fixes and the `.op` alias
explicitly disabled, the compatibility path is tested
without altering the source or patching the expected C++ words:

- All 327 historical instruction words match with non-console support explicitly
  enabled, including reference-only forms. The optional console-only policy rejects those broader forms.
- All 66 source captures and both complete GCT fixtures match exactly.
- The 57 defect probes, with non-console support explicitly enabled, match the reference's GCT/no-GCT outcomes; native crashes
  and a hang become bounded errors rather than identical process termination.
- The three captured CLI cases match their GCT/text/log files.
- Seven additional fresh C++ captures check prefix acceptance, ignored record
  suffixes/macros, address annotations, alias truncation, and odd-word text output.
- All six unmodified Project+ entrypoints match the retained C++ GCTs byte for byte.

Crashes, infinite loops, out-of-bounds accesses, and process-specific exception
codes are **not** reproduced. Source/expansion limits, include-cycle detection,
context cancellation, config validation, output-collision protection, and the
noninteractive CLI remain in force. Diagnostics and console status text are not
byte-for-byte copies of C++. A malformed input outside the characterized cases
can still be rejected differently. Passing these comparisons is not a proof of
equivalence for every possible input.

The GNU/Dolphin correctness profiles explicitly enable **`bug_fixes = true`**
and the syntax/instruction extensions required by their sources.
Explicit compatibility mode intentionally retains invalid or unintended encodings;
the prior hardware-correctness claim must not be applied to that mode. See
[CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md) for the corrected-mode checks.
