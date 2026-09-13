# Configuration

Version **0.13.0-go** defaults every `[bug_fixes]` option to **true**, including the GameCube/Wii target restriction. Every option in `[extensions]`, `[semantics]`, `[encoding]`, `[validation]`, and `[cli]` defaults to **false**. The template is [gctrm.toml](gctrm.toml); [BUG-FIXES.md](BUG-FIXES.md) documents every correction with examples.

## Loading and overrides

The CLI automatically reads `<executable-basename>.toml` beside itself, if present. The root template is not automatically found by `bin/gctrm.exe`; use `--config gctrm.toml` or copy it beside the executable. `--config PATH` replaces the automatic file. `--no-config` skips TOML, while `-i` independently skips INI for the following input. Config selection must precede all inputs, and selectors cannot be combined.

Put `version` before any table header. `[bug_fixes]` contains individual boolean keys; the former scalar `bug_fixes` is rejected. Booleans must be unquoted `true`/`false`. Unknown or duplicate keys, wrong types, invalid schema versions, missing explicit files and files over 1 MiB fail before assembly. Only schema version 2 is accepted; omitted version means 2. Output paths cannot overwrite selected config or source files.

Precedence: defaults < TOML < matching INI < CLI options preceding the input. Use `--set=section.key=true|false` for **any** canonical flag, for example:

```powershell
.\bin\gctrm.exe --config gctrm.toml --set=extensions.dot_op=true --set=validation.strict_macro_calls=true -i source.asm
```

All these choices persist across inputs until changed. `-a`, `-b`, and `-i` remain per-input. `--set=cli.exact_ini_matching=...` takes effect before looking up that input’s INI. Explicit CLI options win over that INI line.

`--bug-fixes=true|false` sets all fixes and `console_only` together; later `--set=bug_fixes.KEY=...` options override individual choices. All category overrides use `--set`; unknown options are errors in both CLI and INI.

## Complete flag reference

All defaults below are false except the individual `[bug_fixes]` options. Each flag is independently configurable.

## bug_fixes

All 39 keys default true. See the [complete fix reference and examples](BUG-FIXES.md) and commented [template](gctrm.toml). Setting one key never changes another key. Target restriction is `bug_fixes.console_only`; the old non-console extension key is rejected.

## extensions

### dot_op

**Default: false.** False ignores dotted writes; true assembles them using the same encoding and address rules as `op`. This also applies inside macros/includes. Enabling it can activate previously ignored patches.

### branch_expressions

**Default: false.** False uses resolved labels or the existing `0x`, `-0x`, and `$` target forms. True also accepts bare numeric targets: `b 20` and `b 16+4` emit `48000014`, like `b 0x14`. Radix/precedence remain separate. Binary/parenthesized forms additionally need `expression_syntax`. This does not provide local-label arithmetic or relocations. Use compact expressions without operand-splitting whitespace. Missing-label and range/alignment checks use `bug_fixes.missing_labels` and `bug_fixes.branch_ranges`.

### expression_syntax

**Default: false.** True enables binary `0b` literals, parentheses, shifts and unary complement in expressions, arithmetic in data/count fields, and broader operand/address expressions. False retains basic alias operators `+ - * / % & ^ |`, literals/named constants in data, and reference-style addition in operands. It does not change precedence, arithmetic width or literal radix. `bug_fixes.alias_terms` and the scanner fixes repair evaluation of supported expressions; it does not enable new grammar. With fixes off, the historical alias evaluator and scanner still retain their characterized quirks, so enabling syntax does not guarantee corrected evaluation. For example, `.alias x = (2 + 3) * 4` requires this extension and corrected evaluation; `word 2+3` also requires it.

### implicit_sections

**Default: false.** False requires a section title before emitted code/data; a missing title produces a bounded diagnostic. True creates an initial section named `Codes`. Declarations/includes can precede the first section, but emitted content must meet this rule. This affects section names/logs and acceptance; it does not add bytes to an otherwise identical named section.

### additional_console_instructions

**Default: false.** This also covers additions made in the initial rewrite: `clrlwi`, `clrrwi`, `rotlwi`, `dcbf`, `dcbi`, `dcbst`, `dcbt`, `dcbtst`, `dcbz`, `eieio`, `sync`, `sc`, `lwarx`, `stwcx.`; `bso`/`bns` branch aliases; and synthesized named-SPR move aliases beyond the original LR/CTR/XER forms. True permits `dcbz_l`, `eciwx`, `ecowx`, `mcrf`, `mcrfs`, `mcrxr`, `mfmsr`, `mfsr`, `mfsrin`, `mftb`, `mtfsb0`, `mtfsb1`, `mtfsf`, `mtfsfi`, `mtmsr`, `mtsr`, `mtsrin`, `tlbie`, `tlbsync`, with implemented record forms and `mftbl/mftbu/mttbl/mttbu` aliases. False treats them as absent: with fixes on, it reports an error; with fixes off, it retains the characterized reference fallback (including prefix interpretation or an unknown-instruction word). Target restriction is controlled by `bug_fixes.console_only`. Repairs to already-implemented instructions, including indexed paired-single forms, remain bug fixes.

## semantics

### decimal_leading_zeros

**Default: false.** False: `010` is eight and `08` is invalid octal. True: `010` is ten and `08` is eight. Applies to ordinary integer expressions and aliases, including enabled branch expressions. Explicit hexadecimal, register numbering, address radix and floating literals are unaffected. This changes meanings, not merely accepted spelling.

### c_operator_precedence

**Default: false.** False: binary operators have equal precedence, so `.alias x = 2 + 3 * 4` gives 20. True: multiplication binds first, giving 14. The precedence order from low to high is `|`, `^`, `&`, shifts, `+ -`, `* / %`; equal levels associate left to right. Parentheses, when enabled, bind first either way. This choice does not enable missing grammar or fix discarded terms.

### signed_64_bit_aliases

**Default: false.** False: alias literals fit 32 bits, intermediate arithmetic wraps modulo 2^32, division/remainder are unsigned, right shift fills with zeros, and shift counts fit 0..31. True: signed 64-bit arithmetic, signed division toward zero, sign-propagating right shift, and counts 0..63. Overflow wraps at the selected width. `.alias x = 0xffffffff + 1` becomes 0 versus 4294967296. Width does not enable shift syntax or widen emitted fields. This also changes recognition of unsigned 32-bit negative patterns in narrow data: false accepts `byte 0xffffffff` as -1; with strict data checking, true rejects that positive value.

## encoding

### gnu_branch_hints

**Default: false.** False retains the direction-adjusted C++ convention: `bdnz -0x10` emits `4220fff0`. True emits `4200fff0`, matching the GNU comparison profile. The ordinary branch condition and destination remain the same; prediction/timing and bytes can differ. The separate C++ BO carry defect is corrected by `bug_fixes.branch_prediction`. Neither convention promises better performance.

### sign_extend_data_slots

**Default: false.** False: block `byte -1` emits `000000ff` and `half -1` emits `0000ffff`. True: both emit `ffffffff`. A scalar narrow value inside CODE/HOOK/PULSE or an `op byte/half` write occupies four bytes. Explicit arrays pack their declared width, and ordinary standalone byte/half writes keep their width; those are unaffected. This is bit extension, not a syntax extension.

### alternative_float_nan

**Default: false.** False emits `7fffffff` for `float NaN`; true emits `7fc00000`. Negative NaNs additionally set the sign bit. Both are quiet NaNs; only stored payload bits change. Finite numbers and infinities are unaffected. Use an explicit `word` for a required pattern. This does not control CPU NaN propagation.

### alternative_double_nan

**Default: false.** False emits `7fffffff ffffffff`; true emits `7ff80000 00000001` for `double NaN`. Sign handling and scope match the float choice, but the two flags are independent. Each double still occupies eight bytes.

## validation

### strict_register_prefixes

**Default: false.** False preserves generic historical prefixes: `fadd r3,r4,r5` still addresses floating registers. True requires `r` for GPRs, `f/fr` for FPRs, `cr` for condition operands and `sr` for segment registers; bare numbers remain accepted. Numeric fields must use unprefixed numbers. Canonical spellings encode identically. Actual field-width and illegal-register-relationship checks remain bug fixes.

### reject_duplicate_labels

**Default: false.** False retains the first definition, matching characterized C++ behavior. True reports a source-location error for duplicate PPC or Gecko labels. Missing labels are independently diagnosed by `bug_fixes.missing_labels`.

### strict_macro_calls

**Default: false.** False accepts characterized extra arguments (ignored) and a missing final closing parenthesis. True requires a complete call and exact argument count. Too few arguments, invalid declarations and recursion limits still fail safely. This does not control undefined macro handling.

### reject_undefined_macros

**Default: false.** False ignores an undefined `%Name(...)` call like the characterized C++ behavior. True reports an error. Existing macro calls are unchanged. Missing include files remain errors, independently of this choice.

### reject_address_annotations

**Default: false.** False retains C++ source-address handling: it can use the leading eight hexadecimal digits and ignore following text. True requires a complete valid address. If expanded expressions are enabled, valid full expressions are evaluated first; otherwise they are not introduced by this validation flag. For example, `@ $80001000 note` is accepted with false and rejected with true. Enabling expressions can change how an arithmetic suffix is interpreted.

### reject_data_overflow

**Default: false.** False preserves declared-width truncation, including `byte 256` becoming zero. The reference conversion range still applies: values outside the native 32-bit unsigned conversion range are rejected rather than accepted as wider literals. True checks the signed-negative/unsigned-positive range before emitting a value: byte permits -128..255, half -32768..65535, word -2147483648..4294967295. Explicit arrays are checked element by element. This does not relax CPU operand/branch checks, PSA index checks, array allocation limits or non-finite scalar rejection.

## cli

### exact_ini_matching

**Default: false.** False selects the first line beginning with the input basename, case-insensitively. True requires equality with the name before the colon, allowing surrounding whitespace/quotes. For `patch.asm`, a preceding `patch.asm.backup` entry can win with false. Selected settings can change GCT bytes, not just presentation. Only the first matching line applies; lines are not merged.

### flat_logs

**Default: false.** False preserves include order/nesting in logs. True writes code sections and offsets without include-tree entries. Request a log with `-l`; this setting does not create one by itself. GCT and codeset-text bytes are unchanged.

### lf_line_endings

**Default: false.** False emits CRLF on Windows and LF on Unix-like hosts. True emits LF everywhere. Applies to codeset text/log files only; request them with `-t`/`-*`/`-g` and `-l`. Binary GCT bytes are unchanged. Library formatting methods always return LF strings; file serialization is the caller’s responsibility.

## Library API

The library does not read config/INI. Set `Options.Fixes = fixes.All()` (from `gctrm/fixes`) to match the CLI correction default; the zero `fixes.Policy` leaves corrections off. Each exported policy field selects one correction. `DotOp` is a pointer with nil meaning false. `BranchExpressions`, `ExpressionSyntax`, `ImplicitSections`, and `AdditionalConsoleInstructions` select extensions. `AllowNonConsoleInstructions` is the inverse of `bug_fixes.console_only` and defaults false. Set true only to permit implemented broader PowerPC forms. `Options.Validation` has `RejectDuplicateLabels`, `StrictMacroCalls`, `RejectUndefinedMacros`, `RejectAddressAnnotations`, and `RejectDataOverflow` booleans. The library's `Compatibility` fields select C++ behavior when true, so they invert the corresponding category flags. Nil inherits `Options.Dialect` (default `assembler.Legacy`); `assembler.Modern` is a library profile for the alternative source choices. TOML and CLI expose individual settings only.

| Category flag | Inverse `Compatibility` field |
| --- | --- |
| `semantics.decimal_leading_zeros` | `OctalLiterals` |
| `semantics.c_operator_precedence` | `LeftToRightExpressions` |
| `semantics.signed_64_bit_aliases` | `Unsigned32BitAliases` |
| `encoding.gnu_branch_hints` | `BranchHints` |
| `encoding.sign_extend_data_slots` | `ZeroExtendedData` |
| `encoding.alternative_float_nan` | `FloatNaN` |
| `encoding.alternative_double_nan` | `DoubleNaN` |
| `validation.strict_register_prefixes` | `RegisterPrefixes` |

No mutable global configuration is used. Options are resolved per call; do not mutate supplied options during assembly. `Result.Bytes()` returns GCT bytes; library text methods use LF.

## Compatibility limits

The defaults describe source/configuration policies, not complete emulation of the native process. The CLI remains noninteractive, diagnostics are its own, and temporary-file output replacement, collision protection, bounded expansion, include-cycle detection and cancellation remain in force. Crashes, hangs, invalid memory accesses and process-specific exception codes are never reproduced. Unknown malformed inputs can fail differently from C++. Per-file writes are atomic; a multi-input invocation is not one transaction.

Corrected instruction words and actual malformed machine-operand checks remain under [bug_fixes](BUG-FIXES.md). Validation results from older executables retain their recorded hashes; see [COMPATIBILITY.md](COMPATIBILITY.md) and [CONSOLE-VALIDATION.md](CONSOLE-VALIDATION.md).
