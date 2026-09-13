# Wii instruction encoding audit

Current configuration is documented in [CONFIGURATION.md](CONFIGURATION.md).
All category flags default false; only bug fixes default true. Policies and
validation results below retain their original versions unless stated otherwise.

Current configuration groups syntax/target additions in
[extensions](CONFIGURATION.md#extensions). Version 0.9 also makes extra numeric
branch forms a separate default-off choice. The integration results below
retain their original versions and executable hashes.

This is the initial audit snapshot. The subsequent
[console validation](CONSOLE-VALIDATION.md) adds independent GNU comparisons,
Dolphin execution, and functional fixes. Its results supersede the earlier
execution limitations and the unresolved PSA/MEM2 statements below. The current
assembler rejects recognized non-console forms by default, independently of
`bug_fixes`; enabling them requires `allow_non_console_instructions = true`.
CLI/config now defaults to `bug_fixes = true`; explicit `false` reproduces known
C++ behavior, including its defects. See
[the current policy](BUG-FIXES.md). Descriptions of earlier Go behavior below
are historical.

The later [C++ defect catalog](CPP-BUGS.md) repeats and expands the output probes.
It reproduces indexed quantized failures, `cmpli`, and both ELSE failures, and
corrects the earlier statement about plain `.ELSE` below.

Checked 2026-09-12 against the locally built GCTRealMate v0.2.6 C++ reference
at commit `9115d23c65c9479e8822968786ac8eec55b7f515` and the delivered Go binary.
The initial compatibility notes conflated encoding defects with other behavior
changes. This audit corrects that classification.

## Reproduced output differences

These are actual instruction words extracted from both generated GCT files,
not values reported by a disassembler. Each input was assembled in a
`CODE @ $80001000` block. Words are hexadecimal, in big-endian display order.

| Input | C++ output | Go output | Defect in the C++ output |
| --- | --- | --- | --- |
| `lha r3,0(r4)` | `A0640000` | `A8640000` | Selects `lhz`, losing sign extension. |
| `eqv r3,r4,r5` | `7C832238` | `7C832A38` | Encodes `r4` twice, ignoring `r5`. |
| `crandc 1,2,3` | `4C221A02` | `4C221902` | Selects `crand`, without complementing the second input. |
| `crorc 1,2,3` | `4C221B82` | `4C221B42` | Selects `cror`, without complementing the second input. |
| `addo r3,r4,r5` | `7C642BA4` | `7C642E14` | Adds decimal 400 instead of `0x400`, changing extended-opcode bits. |
| `srwi r3,r4,0` | `5484003E` | `5483003E` | Encodes destination `r4` instead of `r3`. |
| `psq_l f0,-8(r3),0,0` | `E002FFF8` | `E0030FF8` | Encodes base `r2`, W=1 and I=7 instead of base `r3`, W=0 and I=0. |
| `ps_add. f1,f2,f3` | `1022182A` | `1022182B` | Leaves Rc clear, omitting the requested CR1 update. |

The source explains these results: `lha` uses primary opcode 40; `eqv` selects
the macro that repeats the source register; mnemonic prefix tests catch `crandc`
as `crand` and `crorc` as `cror`; overflow suffix handling uses decimal constants;
zero shifts and negative quantized displacements are added without suitable
field masking; paired-single handling does not apply the record suffix.

The indexed quantized instructions have an additional source-level failure:
`opPairedSingle` calls `vecReg(0)`, attempting to convert the mnemonic itself to
a register number. This was inspected in source; it is not an additional
successful end-to-end execution in the table above.

The integer meanings and opcode selections agree with Dolphin's
[instruction table](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/PPCTables.cpp)
and [integer interpreter](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/Interpreter/Interpreter_Integer.cpp).
The OE, shift, register, and quantized displacement fields agree with its
[instruction layout](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/Gekko.h).
IBM's [Broadway manual](https://pokeacer.xyz/wii/pdf/BroadwayUserManual.pdf)
also documents the quantized format on page 107 and the paired-single record
form on page 509. These findings therefore include Wii-specific extensions.

## Retracted or narrowed claims

The explicit comparison L=1 and 64-bit DS-form changes are not Wii fixes.
Broadway defines L=1 comparisons as invalid and primary opcodes 58/62 as illegal;
see the same manual, pages 86-87 and 372-375. The Go assembler currently accepts
those generic forms, so successful assembly does not certify Wii compatibility.

Expression precedence is a language-design change. PSA tagging and MEM2 hook
behavior required independent validation of the target formats/handler, which
was subsequently performed. The initial claim that plain `.ELSE` worked was
incorrect: the later probe aborts; `.ELSE_RESET` is silently omitted.
See [CPP-BUGS.md](CPP-BUGS.md) for the current classification and concrete evidence.

## Limits

This is a source and output-word audit, not a hardware execution test. Neither
these generated snippets nor a complete Project+ build was run on Wii or Dolphin.
The existing 327-word matching corpus establishes agreement for its samples;
it cannot establish that every accepted instruction is valid on Broadway.
This audit changes documentation only; it does not change the Go executable.
