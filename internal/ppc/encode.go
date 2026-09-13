// Package ppc encodes PowerPC instructions used by Gecko codesets.
package ppc

import (
	"fmt"
	"strings"

	"gctrm/fixes"
	"gctrm/internal/dialect"
	"gctrm/internal/expr"
)

// Context supplies addresses and symbols for an individual instruction.
// RelativeLabel returns the byte displacement from this instruction.
type Context struct {
	ExpressionSyntax              bool
	AdditionalConsoleInstructions bool
	Fixes                         fixes.Policy
	BranchExpressions             bool
	AllowNonConsoleInstructions   bool
	Dialect                       dialect.Mode
	Compatibility                 dialect.Overrides
	Address                       *uint32
	Lookup                        expr.Lookup
	RelativeLabel                 func(string) (int64, bool)
	ConvertAbsolute               bool
}

// Encode assembles one instruction into a host-independent 32-bit word.
func Encode(text string, ctx Context) (uint32, error) {
	rules, err := ctx.Dialect.Resolve(ctx.Compatibility)
	if err != nil {
		return 0, err
	}
	rules.Fixes = ctx.Fixes
	rules.ExpressionSyntax = ctx.ExpressionSyntax
	text = strings.TrimSpace(text)
	name, rest := text, ""
	if i := strings.IndexAny(text, " \t,"); i >= 0 {
		name, rest = text[:i], strings.TrimSpace(text[i:])
		rest = strings.TrimSpace(strings.TrimPrefix(rest, ","))
	}
	name = strings.ToLower(name)
	baseName := strings.TrimSuffix(name, ".")
	if !ctx.AdditionalConsoleInstructions && addedConsoleInstruction(baseName) {
		if !ctx.Fixes.UnknownInstructions {
			return legacyAddedInstruction(text, baseName, ctx)
		}
		return 0, fmt.Errorf("%s requires extensions.additional_console_instructions=true", name)
	}
	if !ctx.AllowNonConsoleInstructions && (unsupportedInstructions[baseName] || strings.HasSuffix(baseName, "o") && unsupportedInstructions[strings.TrimSuffix(baseName, "o")]) {
		return 0, nonConsoleError(name)
	}
	args := operands(rest)
	e := encoder{args: args, ctx: ctx, rules: rules}
	if err := e.checkComparisonTarget(name); err != nil {
		return 0, err
	}
	if word, handled, err := e.extendedMemory(name); handled {
		return word, err
	}
	if !ctx.Fixes.Comparisons {
		if word, handled, err := e.legacySpecial(name); handled {
			return word, err
		}
	}
	if name == "mftb" || name == "mftbu" || name == "mftbl" {
		return e.timeBase(name)
	}
	if name == "mttbl" || name == "mttbu" {
		if err := e.count(1); err != nil {
			return 0, err
		}
		spr := "284"
		if name == "mttbu" {
			spr = "285"
		}
		e.args = []string{spr, args[0]}
		return e.move("mtspr")
	}
	if strings.HasPrefix(name, "b") {
		return e.branch(name)
	}
	if strings.HasPrefix(name, "psq_") {
		return e.quantized(name)
	}
	if strings.HasPrefix(name, "cmp") {
		return e.compare(name)
	}
	if strings.HasPrefix(name, "mtspr") || strings.HasPrefix(name, "mfspr") || name == "mtcr" || name == "mtcrf" {
		return e.move(name)
	}
	for _, prefix := range []string{"mt", "mf"} {
		if strings.HasPrefix(name, prefix) {
			if _, ok := specialRegister(name[2:]); ok {
				if err := e.count(1); err != nil {
					return 0, err
				}
				if prefix == "mt" {
					e.args = []string{name[2:], args[0]}
					return e.move("mtspr")
				}
				e.args = []string{args[0], name[2:]}
				return e.move("mfspr")
			}
		}
	}
	// Pseudoinstructions reduce to canonical operand layouts.
	legacyShiftCarry := false
	dot := strings.HasSuffix(name, ".")
	bare := strings.TrimSuffix(name, ".")
	if bare == "slwi" || bare == "srwi" || bare == "clrlwi" || bare == "clrrwi" || bare == "rotlwi" || bare == "rotlw" {
		if err := e.count(3); err != nil {
			return 0, err
		}
		if bare == "rotlw" {
			args = append(args, "0", "31")
			name = "rlwnm"
		} else {
			n := e.number(2, 5)
			if e.err != nil {
				return 0, e.err
			}
			sh, mb, me := n, uint32(0), uint32(31)
			switch bare {
			case "slwi":
				me = 31 - n
			case "srwi":
				sh = 32 - n
				if ctx.Fixes.ShiftRightZero {
					sh &= 31
				} else if n == 0 {
					// Model the historical carry after encoding, so restoring it
					// does not require disabling checks on the user's operands.
					legacyShiftCarry, sh = true, 0
				}
				mb = n
			case "clrlwi":
				sh = 0
				mb = n
			case "clrrwi":
				sh = 0
				me = 31 - n
			}
			args = []string{args[0], args[1], fmt.Sprint(sh), fmt.Sprint(mb), fmt.Sprint(me)}
			name = "rlwinm"
		}
		if dot {
			name += "."
		}
		e.args = args
	}
	s, ok, nonConsole := lookupInstruction(name, ctx.Fixes.UnknownInstructions)
	record, overflow := false, false
	if !ok {
		name = strings.TrimSuffix(name, ".")
		record = dot
		s, ok, nonConsole = lookupInstruction(name, ctx.Fixes.UnknownInstructions)
		if !ok && strings.HasSuffix(name, "o") {
			name = strings.TrimSuffix(name, "o")
			s, ok, nonConsole = lookupInstruction(name, ctx.Fixes.UnknownInstructions)
			overflow = true
		}
	}
	if ok && nonConsole && !ctx.AllowNonConsoleInstructions {
		return 0, nonConsoleError(name)
	}
	if !ok {
		if !ctx.Fixes.UnknownInstructions {
			if strings.HasPrefix(name, "c") && !strings.HasPrefix(name, "cntl") && !strings.HasPrefix(name, "cmp") {
				return 19 << 26, nil
			}
			return 0xffffffff, nil
		}
		return 0, fmt.Errorf("unknown instruction %q", name)
	}
	if ctx.Fixes.SuffixValidation && record && !s.record {
		return 0, fmt.Errorf("%s does not support record suffix", name)
	}
	if ctx.Fixes.SuffixValidation && overflow && !s.overflow {
		return 0, fmt.Errorf("%s does not support overflow suffix", name)
	}
	if !ctx.Fixes.SuffixValidation && !s.record {
		record = false
	}
	switch name {
	case "lha":
		if !ctx.Fixes.LHA {
			s.base = 40 << 26
		}
	case "eqv":
		if !ctx.Fixes.EQV {
			s.form = duplicate
		}
	case "crandc":
		if !ctx.Fixes.CRAndC {
			s.base = 19<<26 | 257<<1
		}
	case "crorc":
		if !ctx.Fixes.CROrC {
			s.base = 19<<26 | 449<<1
		}
	}
	if !ctx.Fixes.PairedSingleRecord && strings.HasPrefix(name, "ps_") {
		record = false
	}
	n := operandCounts[s.form]
	if name == "eqv" && ctx.Fixes.OperandCounts {
		n = 3
	}
	if err := e.count(n); err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	e.validateOperands(name, s)
	if name == "eqv" && !ctx.Fixes.EQV && ctx.Fixes.OperandRanges && len(args) >= 3 {
		// The old encoding ignores this source, but its field-width check
		// remains independently selectable.
		e.register(2, 'g')
	}
	if e.err != nil {
		return 0, e.err
	}
	roles := operandRoles(name, s)
	r := func(i int) uint32 {
		if roles[i] == 'i' {
			return e.number(i, 5)
		}
		return e.register(i, roles[i])
	}
	var v uint32
	switch s.form {
	case none:
	case rrr, crThree:
		v = r(0)<<21 + r(1)<<16 + r(2)<<11
	case swapped:
		v = r(0)<<21 + r(2)<<16 + r(1)<<11
	case rr0:
		v = r(0)<<21 + r(1)<<16
	case r0r:
		v = r(0)<<21 + r(1)<<11
	case logical:
		v = r(1)<<21 + r(0)<<16 + r(2)<<11
	case logical2:
		v = r(1)<<21 + r(0)<<16
	case duplicate:
		v = r(1)<<21 + r(0)<<16 + r(1)<<11
	case immediate, negImmediate:
		imm := e.value(2)
		if s.form == negImmediate {
			imm = -imm
		}
		v = r(0)<<21 + r(1)<<16 + e.immediate(imm, 16)
	case loadImmediate:
		v = r(0)<<21 + e.immediate(e.value(1), 16)
	case unsignedImmediate:
		v = r(1)<<21 + r(0)<<16 + e.immediate(e.value(2), 16)
	case memory:
		v = r(0)<<21 + r(2)<<16 + e.immediate(e.value(1), 16)
	case rotate:
		v = r(1)<<21 + r(0)<<16 + r(2)<<11 + r(3)<<6 + r(4)<<1
	case floatingMultiply:
		v = r(0)<<21 + r(1)<<16 + r(2)<<6
	case floatingFour:
		v = r(0)<<21 + r(1)<<16 + r(3)<<11 + r(2)<<6
	case compareFloat:
		v = e.register(0, 'c')<<23 + r(1)<<16 + r(2)<<11
	case cache:
		v = r(0)<<16 + r(1)<<11
	case onlyRT:
		v = r(0) << 21
	case crTwo:
		v = r(0)<<21 + r(1)<<16 + r(1)<<11
	case crOne:
		v = r(0)<<21 + r(0)<<16 + r(0)<<11
	case onlyRB:
		v = r(0) << 11
	case crFields:
		v = e.register(0, 'c')<<23 + e.register(1, 'c')<<18
	case crField:
		v = e.register(0, 'c') << 23
	case segmentRead:
		v = r(0)<<21 + e.register(1, 's')<<16
	case segmentWrite:
		v = r(1)<<21 + e.register(0, 's')<<16
	case fpscrMask:
		v = e.number(0, 8)<<17 + r(1)<<11
	case fpscrImmediate:
		v = e.register(0, 'c')<<23 + e.number(1, 4)<<12
	case fpscrBit:
		v = e.register(0, 'b') << 21
	}
	if record {
		v |= 1
	}
	if overflow {
		v |= 1 << 10
	}
	word := s.base | v
	if !ctx.Fixes.OperandRanges {
		word = s.base + v
	}
	if overflow && !ctx.Fixes.OverflowSuffix {
		word = word - (1 << 10) + 400
	}
	if legacyShiftCarry {
		word += 1 << 16
	}
	e.validateMemory(name, word)
	return word, e.err
}

// operands accepts comma-separated expressions and conventional d(rA) syntax.
func operands(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	if strings.Contains(s, ",") {
		depth, start := 0, 0
		for i, c := range s {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			case ',':
				if depth == 0 {
					out = append(out, strings.TrimSpace(s[start:i]))
					start = i + 1
				}
			}
		}
		out = append(out, strings.TrimSpace(s[start:]))
	} else {
		out = strings.Fields(s)
	}
	var result []string
	for _, arg := range out {
		// Legacy source mixes commas and whitespace between simple operands.
		// Preserve spaces inside expressions and displacement(base) forms.
		if fields := strings.Fields(arg); len(fields) > 1 && !strings.ContainsAny(arg, "()+-*/%<>&^|~") {
			result = append(result, fields...)
			continue
		}
		if i := strings.LastIndex(arg, "("); i > 0 && strings.HasSuffix(arg, ")") {
			reg := strings.TrimSpace(arg[i+1 : len(arg)-1])
			displacement := strings.TrimSpace(arg[:i])
			if reg != "" && displacement != "" && strings.IndexAny(reg, " ()+-*/%<>&^|~") < 0 && !strings.ContainsRune("+-*/%<>&^|~", rune(displacement[len(displacement)-1])) {
				result = append(result, strings.TrimSpace(arg[:i]), reg)
				continue
			}
		}
		result = append(result, arg)
	}
	return result
}

type encoder struct {
	rules dialect.Rules
	args  []string
	ctx   Context
	err   error
}

func (e *encoder) fail(s string) {
	if e.err == nil {
		e.err = fmt.Errorf("%s", s)
	}
}
func (e *encoder) count(n int) error {
	if len(e.args) < n || e.ctx.Fixes.OperandCounts && len(e.args) != n {
		return fmt.Errorf("expected %d operands, got %d", n, len(e.args))
	}
	return nil
}
func (e *encoder) value(i int) int64 {
	v, err := expr.EvalRules(e.args[i], e.ctx.Lookup, e.rules)
	if err != nil && e.err == nil {
		e.err = err
	}
	return v
}
func (e *encoder) immediate(v int64, bits uint) uint32 {
	// Accept full-width two's complement aliases as signed immediates.
	if v >= 0x80000000 && v <= 0xffffffff {
		v = int64(int32(v))
	}
	if e.ctx.Fixes.OperandRanges && (v < -(1<<(bits-1)) || v > (1<<bits)-1) {
		e.fail(fmt.Sprintf("immediate %d does not fit %d bits", v, bits))
	}
	return uint32(v) & ((1 << bits) - 1)
}
func (e *encoder) compare(name string) (uint32, error) {
	names := map[string]bool{"cmp": true, "cmpw": true, "cmpd": true, "cmpi": true, "cmpwi": true, "cmpdi": true, "cmpl": true, "cmplw": true, "cmpld": true, "cmpli": true, "cmplwi": true, "cmpldi": true}
	if !names[name] {
		return 0, fmt.Errorf("unknown comparison %q", name)
	}
	imm := strings.HasSuffix(name, "i")
	logical := strings.HasPrefix(name, "cmpl")
	bf, l := uint32(0), uint32(0)
	if strings.Contains(name, "d") {
		l = 1
	}
	switch len(e.args) {
	case 2:
	case 3:
		bf = e.register(0, 'c')
		e.args = e.args[1:]
	case 4:
		bf = e.register(0, 'c')
		l = e.number(1, 1)
		e.args = e.args[2:]
	default:
		return 0, fmt.Errorf("comparison requires 2, 3, or 4 operands")
	}
	if !e.ctx.AllowNonConsoleInstructions && l != 0 {
		return 0, nonConsoleError(name + " with L=1")
	}
	v := bf<<23 | e.register(0, 'g')<<16
	if e.ctx.Fixes.Comparisons {
		v |= l << 21
	}
	if imm {
		op := uint32(11)
		if logical {
			op = 10
		}
		v |= op<<26 | e.immediate(e.value(1), 16)
	} else {
		v |= 31<<26 | e.register(1, 'g')<<11
		if logical {
			v |= 32 << 1
		}
	}
	return v, e.err
}
