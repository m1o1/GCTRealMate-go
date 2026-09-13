package ppc

import (
	"fmt"
	"strings"

	"gctrm/internal/expr"
)

// These names were accepted by the original generic PowerPC table, but are
// not instructions of Gekko/Broadway. Keep them for useful diagnostics.
var unsupportedInstructions = map[string]bool{
	"dcba": true,
	"ld":   true, "ldu": true, "lwa": true, "std": true, "stdu": true,
	"ldx": true, "ldux": true, "lwax": true, "lwaux": true, "stdx": true, "stdux": true,
	"td": true, "tdi": true, "mulld": true, "divd": true, "divdu": true,
	"mulhd": true, "mulhdu": true, "sld": true, "srd": true, "srad": true,
	"extsw": true, "cntlzd": true, "fctid": true, "fctidz": true, "fcfid": true,
	"fsqrt": true, "fsqrts": true, "frsqrtes": true, "fsels": true,
	"fress": true,
	"cmpd":  true, "cmpdi": true, "cmpld": true, "cmpldi": true,
}

// Operand roles distinguish register classes from numeric fields even where
// several instructions share a bit layout. Bare register numbers are allowed.
// g=GPR, f=FPR, c=CR field, b=CR bit, s=segment, i=numeric expression.
var layoutRoles = [...]string{
	none: "", rrr: "ggg", rr0: "gg", r0r: "gg", logical: "ggg",
	logical2: "gg", duplicate: "gg", immediate: "ggi", negImmediate: "ggi",
	loadImmediate: "gi", unsignedImmediate: "ggi", memory: "gig",
	rotate: "ggiii", floatingMultiply: "fff", floatingFour: "ffff",
	compareFloat: "cff", cache: "gg", onlyRT: "g", crThree: "bbb",
	crTwo: "bb", crOne: "b", swapped: "ggg", onlyRB: "g", crFields: "cc",
	crField: "c", segmentRead: "gs", segmentWrite: "sg", fpscrMask: "if",
	fpscrImmediate: "ci", fpscrBit: "b",
}

func operandRoles(name string, s spec) string {
	roles := layoutRoles[s.form]
	switch name {
	case "tw", "twi", "td", "tdi":
		roles = "ig" + roles[2:]
	case "lswi", "stswi", "srawi":
		roles = "ggi"
	case "rlwnm":
		roles = "gggii"
	case "mffs":
		roles = "f"
	default:
		if strings.HasPrefix(name, "lf") || strings.HasPrefix(name, "stf") {
			roles = "f" + roles[1:]
		} else if strings.HasPrefix(name, "f") || strings.HasPrefix(name, "ps_") {
			roles = strings.ReplaceAll(roles, "g", "f")
		}
	}
	return roles
}

func (e *encoder) register(i int, role byte) uint32 {
	bits, prefixes := 5, []string{"r"}
	switch role {
	case 'f':
		prefixes = []string{"fr", "f"}
	case 'c':
		bits, prefixes = 3, []string{"cr"}
	case 'b':
		prefixes = []string{"cr"}
	case 's':
		bits, prefixes = 4, []string{"sr"}
	}
	return e.number(i, bits, prefixes...)
}

func (e *encoder) number(i, bits int, prefixes ...string) uint32 {
	s := strings.TrimSpace(e.args[i])
	if e.ctx.Lookup != nil {
		if v, ok := e.ctx.Lookup(s); ok {
			if e.ctx.BugFixes && (v < 0 || v >= 1<<bits) {
				e.fail(fmt.Sprintf("operand %q outside 0..%d", s, (1<<bits)-1))
			}
			return uint32(v)
		}
	}
	if e.rules.RegisterPrefixes {
		prefixes = append([]string{"fr", "cr", "r", "f"}, prefixes...)
	}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(s), p) {
			s = s[len(p):]
			break
		}
	}
	var v int64
	var err error
	if e.ctx.BugFixes {
		v, err = expr.Eval(s, e.ctx.Lookup)
	} else {
		v, err = legacyNumber(s)
	}
	if err != nil {
		e.fail(fmt.Sprintf("operand %q: %v", e.args[i], err))
		return 0
	}
	if e.ctx.BugFixes && (v < 0 || v >= 1<<bits) {
		e.fail(fmt.Sprintf("operand %q outside 0..%d", e.args[i], (1<<bits)-1))
	}
	return uint32(v)
}

func (e *encoder) validateOperands(name string, s spec) {
	for i, role := range []byte(operandRoles(name, s)) {
		if role != 'i' {
			e.register(i, role)
		}
	}
}

// Validate relationships between encoded registers. Privilege, memory
// mappings, FPSCR and HID state remain runtime responsibilities.
func (e *encoder) validateMemory(name string, word uint32) {
	if !e.ctx.BugFixes {
		return
	}
	rt, ra := (word>>21)&31, (word>>16)&31
	if (strings.HasPrefix(name, "l") || strings.HasPrefix(name, "st") || strings.HasPrefix(name, "psq_")) &&
		(strings.HasSuffix(name, "u") || strings.HasSuffix(name, "ux")) {
		if ra == 0 {
			e.fail("update-form memory instruction requires a nonzero base register")
		}
		if strings.HasPrefix(name, "l") && !strings.HasPrefix(name, "lf") && ra == rt {
			e.fail("update-form integer load requires different destination and base registers")
		}
	}
	if name == "lmw" && ra >= rt {
		e.fail("lmw base register overlaps the registers being loaded")
	}
	if name == "lswi" {
		n := (word >> 11) & 31
		if n == 0 {
			n = 32
		}
		for j := uint32(0); j < (n+3)/4; j++ {
			if (rt+j)&31 == ra {
				e.fail("lswi base register overlaps the registers being loaded")
			}
		}
	}
	// For lswx the byte count is in XER, so the full overlap cannot be known
	// at assembly time. Reject the statically invalid destination/base case.
	if name == "lswx" && (rt == ra || rt == (word>>11)&31) {
		e.fail("lswx destination overlaps an address register")
	}
}
