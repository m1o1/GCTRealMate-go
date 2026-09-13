package ppc

import (
	"fmt"
	"regexp"
	"strings"

	"gctrm/internal/expr"
)

var branchSymbol = regexp.MustCompile(`^[A-Za-z_.$][A-Za-z0-9_.$]*$`)

func (e *encoder) branch(name string) (uint32, error) {
	if name == "b" || name == "bl" || name == "ba" || name == "bla" {
		if err := e.count(1); err != nil {
			return 0, err
		}
		aa := strings.HasSuffix(name, "a") && !e.ctx.ConvertAbsolute
		offset, err := e.target(e.args[0], aa, strings.HasSuffix(name, "a") && e.ctx.ConvertAbsolute)
		if err != nil {
			return 0, err
		}
		if e.ctx.BugFixes && (offset&3 != 0 || offset < -(1<<25) || offset >= (1<<25)) {
			return 0, fmt.Errorf("branch displacement %d is unaligned or outside signed 26-bit range", offset)
		}
		v := uint32(18<<26) | uint32(offset)&0x03fffffc
		if strings.Contains(name, "l") {
			v |= 1
		}
		if aa {
			v |= 2
		}
		return v, nil
	}
	hint := strings.HasSuffix(name, "+")
	hasHint := hint || strings.HasSuffix(name, "-")
	if hasHint {
		name = name[:len(name)-1]
	}
	bo, bi := uint32(0), uint32(0)
	suffix := ""
	explicit := false
	for _, c := range []struct {
		name   string
		bo, bi uint32
	}{{"bdnz", 16, 0}, {"bdz", 18, 0}, {"beq", 12, 2}, {"bne", 4, 2}, {"blt", 12, 0}, {"bge", 4, 0}, {"bgt", 12, 1}, {"ble", 4, 1}, {"bso", 12, 3}, {"bns", 4, 3}} {
		if strings.HasPrefix(name, c.name) {
			bo, bi, suffix = c.bo, c.bi, name[len(c.name):]
			break
		}
	}
	if strings.HasPrefix(name, "bctr") {
		bo, suffix = 20, name[1:]
	} else if strings.HasPrefix(name, "blr") {
		bo, suffix = 20, name[1:]
	} else if bo == 0 && strings.HasPrefix(name, "bc") {
		if len(e.args) < 2 {
			return 0, fmt.Errorf("bc requires BO and BI")
		}
		bo, bi = e.number(0, 5), e.register(1, 'b')
		e.args = e.args[2:]
		suffix = name[2:]
		explicit = true
	} else if bo == 0 {
		return 0, fmt.Errorf("unknown branch %q", name)
	}
	indirect := strings.HasPrefix(suffix, "lr") || strings.HasPrefix(suffix, "ctr")
	v := uint32(16 << 26)
	if indirect {
		v = 19 << 26
		if strings.HasPrefix(suffix, "ctr") {
			if e.ctx.BugFixes && bo&4 == 0 {
				return 0, fmt.Errorf("branch to CTR cannot also decrement/test CTR (BO must have bit 2 set)")
			}
			v |= 528 << 1
			suffix = suffix[3:]
		} else {
			v |= 16 << 1
			suffix = suffix[2:]
		}
	}
	link := false
	absolute := false
	if strings.HasPrefix(suffix, "l") {
		link = true
		suffix = suffix[1:]
	}
	if suffix == "a" && !indirect {
		absolute = true
		suffix = ""
	}
	if suffix != "" {
		return 0, fmt.Errorf("invalid branch suffix %q", suffix)
	}
	if !explicit && bo&16 == 0 && (indirect && len(e.args) == 1 || !indirect && len(e.args) == 2) {
		bi += 4 * e.register(0, 'c')
		e.args = e.args[1:]
	}
	if indirect {
		if err := e.count(0); err != nil {
			return 0, err
		}
	} else {
		if err := e.count(1); err != nil {
			return 0, err
		}
		d, err := e.target(e.args[0], absolute, false)
		if err != nil {
			return 0, err
		}
		if e.ctx.BugFixes && (d&3 != 0 || d < -(1<<15) || d >= (1<<15)) {
			return 0, fmt.Errorf("conditional branch displacement %d is unaligned or outside signed 16-bit range", d)
		}
		v |= uint32(d) & 0xfffc
		// Legacy toggles the BO prediction bit on every backward branch,
		// even an unsuffixed explicit bc. Modern follows GNU defaults.
		if e.rules.BranchHints {
			if hint {
				if e.ctx.BugFixes {
					bo |= 1
				} else {
					bo++
				}
			}
			hint = false
			if d < 0 {
				bo ^= 1
			}
		} else if hasHint && d < 0 {
			hint = !hint
		}
	}
	if hint {
		if !e.ctx.BugFixes && e.rules.BranchHints {
			bo++
		} else {
			bo |= 1
		}
	}
	v |= bo<<21 | bi<<16
	if link {
		v |= 1
	}
	if absolute {
		v |= 2
	}
	return v, e.err
}
func (e *encoder) target(s string, absolute, convert bool) (int64, error) {
	if e.ctx.RelativeLabel != nil {
		if d, ok := e.ctx.RelativeLabel(s); ok {
			if absolute {
				return 0, fmt.Errorf("absolute branch cannot use a relative label")
			}
			return d, nil
		}
	}
	if !e.ctx.BranchExpressions && !strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "-0x") {
		if !e.ctx.BugFixes {
			return 0, nil
		}
		if _, err := expr.EvalRules(s, e.ctx.Lookup, e.rules); err != nil {
			return 0, fmt.Errorf("unresolved branch target %q", s)
		}
		return 0, fmt.Errorf("numeric branch target %q requires extensions.branch_expressions=true (or --branch-expressions=true)", s)
	}
	addressed := strings.HasPrefix(s, "$")
	v, err := expr.EvalRules(s, e.ctx.Lookup, e.rules)
	if err != nil {
		// Opting into numeric expressions does not opt into the missing-label
		// correction. Malformed arithmetic still reports its evaluation error.
		if !e.ctx.BugFixes && !addressed && branchSymbol.MatchString(s) {
			return 0, nil
		}
		return 0, err
	}
	if (addressed || convert) && (v < 0 || v > 0xffffffff) {
		return 0, fmt.Errorf("branch address must fit 32 bits")
	}
	if (addressed && !absolute) || convert {
		if e.ctx.Address == nil {
			return 0, fmt.Errorf("branch to address requires a known instruction address; provide -b for HOOK")
		}
		if convert && !addressed && v >= 0 && v < 0x80000000 {
			v += 0x80000000
		}
		return int64(int32(uint32(v) - *e.ctx.Address)), nil
	}
	if v >= 0x80000000 && v <= 0xffffffff {
		v = int64(int32(v))
	}
	return v, nil
}
