# Confirmed C++ defects and compatibility differences

Current configuration is documented in [CONFIGURATION.md](CONFIGURATION.md).
All category flags default false; only bug fixes default true. Policies and
validation results below retain their original versions unless stated otherwise.

This is the complete list of findings confirmed so far for CodecSMW GCTRealMate
v0.2.6, commit `9115d23c65c9479e8822968786ac8eec55b7f515`. It is not a claim
that every possible defect has been found, or that other forks/releases behave
identically. The C++ checkout and executable were not modified.

Current corrections and individual source choices are documented in
[BUG-FIXES.md](BUG-FIXES.md) and the [compatibility contract](COMPATIBILITY.md).
The 57-probe table below is a preserved **0.2.0-go audit snapshot**, not a fresh
capture of the current binary. Its C++ evidence remains applicable. The source
and CLI capture suites separately check reference-compatible behavior.

On 2026-09-12, 57 small source probes were run against the then-current executables. Results,
source inputs, process status, complete GCT bytes, and executable hashes are in
[cpp-bug-probes.json](validation/cpp-bug-probes.json). For 18 instruction examples,
GNU as independently reproduced the Go words in both Gekko and Broadway modes.
The earlier [console validation](CONSOLE-VALIDATION.md) supplies broader encoding
and emulator evidence. A C++/Go difference alone was not treated as proof of a bug.

Two earlier statements need correction: plain `.ELSE` also fails in this build,
and the raw-byte loss reproduced here occurs at end of input; the tested section
transition flushes correctly. The fresh checks also confirm a `cmpli` defect and
dropped terms in chained alias arithmetic.

## Valid instruction encoding

The entries below group related failures; 25 groups overall is an organizational
count, not a count of every affected mnemonic or operand combination.

| ID | Defect | Reproduced behavior and consequence |
| --- | --- | --- |
| 1 | `lha` selects the wrong primary opcode | `lha r3,0(r4)` emits `A0640000` (`lhz`) instead of `A8640000`. A halfword `FFFF` becomes `0000FFFF` instead of `FFFFFFFF`. |
| 2 | `eqv` ignores its third operand | `eqv r3,r4,r5` encodes both sources as r4. The equivalence of a value with itself is all ones, regardless of r5. |
| 3 | `crandc` and `crorc` match shorter mnemonic prefixes | They emit `crand` and `cror`, losing the complement of the second input. This can change subsequent branch conditions. |
| 4 | Overflow suffix uses decimal constants | `o`/`o.` add 400/401 instead of `0x400`/`0x401`. `addo r3,r4,r5` emits `7C642BA4` instead of `7C642E14`; extended-opcode bits are corrupted. |
| 5 | Zero-distance `srwi` overflows its shift field | `srwi r3,r4,0` emits `5484003E` rather than `5483003E`, changing the destination register in this example. |
| 6 | Negative quantized displacement is not masked to 12 bits | `psq_l f0,-8(r3),0,0` emits `E002FFF8` instead of `E0030FF8`: the base becomes r2 and W/I become 1/7. The matching store example fails too. |
| 7 | Indexed quantized operands are mis-indexed | `psq_lx`, `psq_lux`, `psq_stx`, and `psq_stux` attempt to parse the mnemonic as a register (`vecReg(0)`). All four valid probes abort without a GCT. Source inspection additionally shows reversed update selectors for the two indexed store forms. |
| 8 | Paired-single record suffix is ignored | `ps_add. f1,f2,f3` emits `1022182A` instead of `1022182B`, omitting the requested CR1 update. |
| 9 | Hexadecimal register/rotate fields are partially parsed as decimal | `cmpw r5,0xD` selects r0 instead of r13; `rlwinm r0,r0,0x3,0x0,0x1c` emits `54000000` instead of `54001838`. Even if a dialect required decimal here, silently accepting only the leading zero is a diagnostic defect. |
| 10 | `cmpli` reaches the wrong encoder | `cmpli 0,0,r3,1` emits `7C001840` instead of `28030001`. A duplicated `cmpi` test leaves the intended logical-immediate case missing, and the `cmpl` prefix matches later. This is an L=0, valid console instruction. |

The relevant implementation is [PPCop.cpp](../GCTRealMate-source/src/PPCop.cpp)
and its field macros in [PPCop.h](../GCTRealMate-source/include/PPCop.h).
Independent semantics include Dolphin's
[integer implementation](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/Interpreter/Interpreter_Integer.cpp),
[paired-single implementation](https://github.com/dolphin-emu/dolphin/blob/master/Source/Core/Core/PowerPC/Interpreter/Interpreter_Paired.cpp),
and the GNU-generated words recorded with the probes.

## Source processing and Gecko data

| ID | Defect | Reproduced behavior and consequence |
| --- | --- | --- |
| 11 | Pending raw bytes are dropped at EOF | A named section ending with `byte 0x12` produces no payload. The pending raw-byte queue is not flushed at end of input. A section-transition control probe does flush correctly. |
| 12 | Scanner consumes expression operators | The scanner discards `*` even inside an alias/directive, and treats `\|` as line continuation before the implemented OR operator can see it. `.ALIAS x = 2 * 3` becomes 23; `.GR4 *= 2` becomes assignment. The OR probes either produce zero words or abort. |
| 13 | Chained alias arithmetic drops terms | `.ALIAS x = 2 + 3 + 4` yields 5 instead of 9. This cannot be explained by operator precedence: all operations are addition. The evaluator updates and appends separate temporary values, then returns only the first. |
| 14 | Gecko-register loads/stores omit the register index | `.GR5 <-(16) $1000` emits `82100000` instead of `82100005`. A store from GR6 similarly encodes GR0. |
| 15 | Qualified BA/PO operand parsing uses wrong offsets/lengths | `.BA = PO+$1000` and `.BA -> GR3+$1000` abort. The qualifier is checked at a fixed/wrong position, or a three-character substring is compared with `GR`, causing address conversion to receive nonnumeric text. |
| 16 | Directive operands lose bit 31 | `.PO = $90000000` emits `10000000`; `.GR3 = $ffffffff` emits `7FFFFFFF`. BA/PO and several GR paths mask full-width values with `0x7FFFFFFF`. |
| 17 | MEM2 direct writes and hooks form wrong effective addresses | A word write to `90001000` sets BA to that address but emits offset zero; the handler masks BA, so the effective target is `90000000`. A C2 hook additionally sets PO although C2 uses BA, because the source tests the enum constant `hookCode` instead of comparing `writeType`. With default BA it targets `80000000`. The pointer-based MEM2 CODE probe is correct and matches Go. |
| 18 | Direct PSA writes erase bank/type tags | `RA_float 3 @ $80001000` emits `00000003` rather than `21000003`: assignment of the index overwrites the union holding the tag. C++ block data uses OR and preserves the correct tag. This is separate from the initial Go port's reversed Float/Bit mapping. |
| 19 | `.GOTO_F` omits its second word | The statement that should append zero is inside a C++ line comment. The probe emits an odd word count and misaligns following Gecko commands. |
| 20 | Backward Gecko label fixups borrow into control bits | The backward probe emits `661FFFFE` instead of `6620FFFE`. The signed offset is added to the whole code word instead of being inserted into the low 16 bits, changing the GOTO condition. |
| 21 | Both ELSE variants fail | `.ELSE_RESET` is silently omitted because `ELSE_` is compared with `ELSE`. Plain `.ELSE` reaches a substring operation beyond the end of the string and aborts. The previous claim that plain `.ELSE` worked was incorrect for this build. |

The relevant implementation is [compileGCT.cpp](../GCTRealMate-source/src/compileGCT.cpp),
with label insertion in `PPCop::enforceOffset`. The
[Gecko handler](https://github.com/iGlitch/GeckoOS/blob/master/Gecko_src/code%20handler/codehandleronly.s)
defines BA/PO interpretation, 8-byte command framing, and the signed low-field
GOTO displacement. The saved probes distinguish correct controls from failing
forms instead of treating every MEM2 or PSA use as broken.

## Error handling

These concern invalid source and build automation rather than misencoding a
well-formed supported instruction.

| ID | Defect | Reproduced behavior and consequence |
| --- | --- | --- |
| 22 | Unresolved branch labels succeed | `b missing` produces a branch-to-self and exits successfully instead of reporting an unresolved symbol. |
| 23 | Unknown instructions and invalid fields can produce a successful GCT | `garbage r3,r4,r5` emits `FFFFFFFF`; `add r32,r4,r5` spills into opcode bits; an out-of-range branch changes meaning through masking; an invalid update load is accepted. Go rejects these samples. |
| 24 | An `op` line without `@` does not stop at EOF | `op nop` failed to finish within the 3-second probe deadline; source inspection shows an address-seeking loop without an EOF condition. The probe stopped only its own child process. |
| 25 | Reported compilation errors can return exit status zero | A missing include prints an error and does not produce a GCT, but the process returns success. A build script relying on exit status can proceed after failure. |

Duplicate labels and an unterminated macro-call parenthesis were also accepted
in probes that Go rejects. Those are recorded as stricter validation differences;
they are not counted here as independent valid-program encoding defects.

## Differences that are not automatically bugs

The examples in this section compare C++ with **0.2.0-go / modern semantics**.
The legacy language choices match C++ for ordinary branch hints, NaN bytes,
leading-zero literals, alias arithmetic order/width, and harmless numeric
register spellings. It also restores legacy INI matching, native CLI text line
endings and include-tree logs; separate flags opt into the alternatives.

### Default conditional-branch prediction

The `bdnz -0x10` probe emits `4220FFF0` in C++ and `4200FFF0` in Go/GNU.
Both decrement/test CTR and use the same destination. The difference is a
prediction hint, which may affect timing or performance, not the branch's
architectural condition. The Go default was chosen to match GNU. This is a
compatibility decision, not proof that C++'s default is invalid. Explicit hint
syntax allows the programmer to state the desired convention. The
[Broadway manual](https://pokeacer.xyz/wii/pdf/BroadwayUserManual.pdf) describes
prediction separately from resolution of the actual branch condition.

### NaN bit patterns

For `float NaN`, C++ emits `7FFFFFFF` and Go emits `7FC00000`. Both are quiet
NaNs. IEEE floating-point represents NaNs with multiple payload patterns; a
generic NaN literal need not select one universal bit pattern. The bytes still
matter if code inspects or serializes them or uses a payload as a sentinel.
Use an explicit `word` when a particular representation is required. This
choice does not fix a numerical defect in the C++ NaN value.

### Expression precedence

The measured alias `6 ^ 3 & 1` evaluates to 1 in C++ and 7 in Go. The C++
bitwise evaluator applies the operations left to right: `(6 ^ 3) & 1`. Go's
C-style expression grammar makes AND bind more tightly: `6 ^ (3 & 1)`.
Either rule can define an assembler language; the CPU ISA does not prescribe
source-expression precedence. Existing aliases can need migration even if both
results are legal constants. This is distinct from `2 + 3 + 4` losing its last
term, or `*` disappearing, which are defects listed above.

### Numeric notation and arithmetic width

`li r3,010` emits the value 8 in C++ and 10 in Go. C++'s base-zero conversion
recognizes legacy octal notation; the Go frontend's unprefixed numbers are
decimal. Write `8` or `0x08` for an unambiguous eight. Go also uses bounded signed
64-bit expression evaluation rather than C++'s mixed 32-bit conversions and
wrapping behavior; rejecting overflow or producing a different intermediate
result is a language policy, not a Wii opcode correction.

Bare branch constants are another syntax difference: C++ recognizes the tested
numeric branch form with a `0x`/`-0x` prefix (or `$` absolute-address syntax),
whereas Go can accept decimal expressions with `extensions.branch_expressions = true`. In C++, `b 20` is taken as a label
reference and then suffers the unresolved-label bug; it is not evidence of a
valid decimal instruction being encoded as hexadecimal. Use `b 0x14` for a
20-byte displacement supported by both syntaxes.

### Target policy, missing features, and permissive syntax

C++ contains generic PowerPC operations that GameCube/Wii cannot execute.
Supporting a wider target family is not inherently an encoder bug. Go defaults
to rejecting 64-bit operations, L=1 comparisons, and other recognized non-console
forms. The independent target flag can permit implemented broader forms.
The earlier DS-form/L-field changes were therefore
not fixes to valid Wii instructions. Missing supported mnemonics are coverage
gaps; silent success with an unknown opcode is a separate diagnostic defect.

For the L field specifically, C++'s `% 1` does discard a requested 1. That would
be an encoding defect if judging a claimed general 64-bit PowerPC mode, but
the appropriate console behavior is rejection, not emission of L=1. The earlier
DS-form change also involved the assembler's displacement-unit convention;
it cannot establish a Wii bug for instructions that Wii does not implement.

The Go frontend's extra expression syntax, `.op` alias, implicit initial section,
and stricter handling of malformed calls/duplicate labels are interface choices.
The C++ `.op` probe is ignored; Go intentionally recognizes it, but that alone
does not establish that C++ promised this alias. CLI changes such as no final
pause, exact INI basename matching, LF text, different log layout, and atomic
output replacement are likewise product/compatibility choices.

### Project+ source and initial Go mistakes

The malformed calls, missing labels, and misspelled instructions repaired in
the staged Project+ comparison are defects or ambiguities in that input snapshot,
not automatically defects in the C++ assembler. Their inferred fixes have not
been gameplay-validated. See [PROJECT-PLUS-VALIDATION.md](PROJECT-PLUS-VALIDATION.md).

The initial Go port also contained mistakes, including reversed PSA Float/Bit
tags and MEM2/high-bit handling. Validation fixed them. The C++ direct-PSA tag
overwrite, MEM2 failures, and directive truncation listed here were separately
reproduced; a Go correction was not assumed to prove a matching C++ defect.

## Explicit BO and hint overlap found during compatibility characterization

The new [source captures](assembler/testdata/compatibility.json) include
`bc+ 13,2,0x10` and `bc+ 13,2,-0x10`. C++ emits `41c20010` and `41e2fff0`:
`checkBranchCondition` adds the prediction bit to a BO field whose bit is already
set, carrying into another BO bit. GNU emits `41a20010` and `41a2fff0` in both
console modes; see [the independent capture](validation/gnu-hints.json).
With bug fixes enabled, the Go encoder sets only the prediction bit, with the legacy
direction adjustment applied afterward (`41a20010` / `4182fff0`). This behavior
is enabled by the current CLI/config default; explicit `bug_fixes = false`
reproduces the C++ words. These particular BO values also
contain CPU-ignored bits, so the extra changed bit is not by itself evidence
that these two probes have different branch conditions. The known observable
difference includes prediction; it must not be conflated with the valid
unsuffixed backward-hint convention restored by this pass.
