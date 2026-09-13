package ppc

import (
	"fmt"
	"strings"
)

func nonConsoleError(name string) error {
	return fmt.Errorf("%s is not supported by GameCube/Wii (Gekko/Broadway); non-console forms require extensions.non_console_instructions=true", name)
}

// Check explicit 64-bit comparison requests before legacy operand selection
// can discard L. Target selection must not depend on the bug-fix policy.
func (e *encoder) checkComparisonTarget(name string) error {
	if e.ctx.AllowNonConsoleInstructions || len(e.args) != 4 {
		return nil
	}
	switch name {
	case "cmp", "cmpi", "cmpl", "cmpli", "cmpw", "cmpwi", "cmplw", "cmplwi":
		width := *e
		width.ctx.BugFixes = true
		l := width.number(1, 1)
		if width.err != nil {
			return width.err
		}
		if l != 0 {
			return nonConsoleError(name + " with L=1")
		}
	}
	return nil
}

// DS-form loads/stores use aligned byte displacements in corrected mode.
// Compatibility mode preserves the reference's displacement-times-four rule.
func (e *encoder) extendedMemory(name string) (uint32, bool, error) {
	switch name {
	case "ld", "ldu", "lwa", "std", "stdu":
	default:
		return 0, false, nil
	}
	if err := e.count(3); err != nil {
		return 0, true, err
	}
	op, low := uint32(58), uint32(0)
	if strings.HasPrefix(name, "st") {
		op = 62
	}
	if strings.HasSuffix(name, "u") {
		low = 1
	}
	if name == "lwa" {
		low = 2
	}
	value := e.value(1)
	displacement := uint32(uint16(value * 4))
	if e.ctx.BugFixes {
		displacement = e.immediate(value, 16)
		if displacement&3 != 0 {
			e.fail("DS-form displacement must be a multiple of four bytes")
		}
	}
	word := op<<26 + e.register(0, 'g')<<21 + e.register(2, 'g')<<16 + displacement + low
	e.validateMemory(name, word)
	return word, true, e.err
}
