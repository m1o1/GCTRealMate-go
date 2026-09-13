package assembler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var registerDirective = regexp.MustCompile(`(?i)^\.?GR([0-9a-f])(.+)$`)

func directiveAddress(n node, text string) (uint32, error) {
	if !n.expressionSyntax && strings.ContainsAny(text, "()+*/%<>&^|~") {
		return 0, fmt.Errorf("address expression requires extensions.expression_syntax=true")
	}
	return parseAddress(text, lookupValues(n.values))
}

func encodeDirective(n node) ([]uint32, *fixup, error) {
	s := strings.TrimPrefix(compact(n.text), ".")
	upper := strings.ToUpper(s)
	pair := func(a, b uint32) ([]uint32, *fixup, error) { return []uint32{a, b}, nil, nil }
	switch upper {
	case "RESET":
		return pair(0xe0000000, 0x80008000)
	case "ENDIF":
		return pair(0xe2000001, 0)
	case "ENDIF_RESET":
		return pair(0xe2000001, 0x80008000)
	case "ELSE":
		return pair(0xe2100001, 0)
	case "ELSE_RESET":
		return pair(0xe2100001, 0x80008000)
	case "END":
		return pair(0xf0000000, 0)
	}
	for prefix, word := range map[string]uint32{"GOTO->": 0x66200000, "GOTO_T->": 0x66000000, "GOTO_F->": 0x66100000} {
		if strings.HasPrefix(upper, prefix) {
			label := s[len(prefix):]
			if !identifier.MatchString(label) {
				return nil, nil, fmt.Errorf("invalid GOTO label %q", label)
			}
			return []uint32{word, 0}, &fixup{name: label, lines: true}, nil
		}
	}
	if strings.HasPrefix(upper, "BA") || strings.HasPrefix(upper, "PO") {
		word := uint32(0x40000000)
		if upper[:2] == "PO" {
			word |= 0x08000000
		}
		rhs := s[2:]
		labelAllowed := false
		switch {
		case strings.HasPrefix(rhs, "<-"):
			rhs = rhs[2:]
			labelAllowed = true
		case strings.HasPrefix(rhs, "<+"):
			rhs = rhs[2:]
			word |= 0x00100000
		case strings.HasPrefix(rhs, "->"):
			word |= 0x04000000
			rhs = rhs[2:]
		case strings.HasPrefix(rhs, "+="):
			word |= 0x02100000
			rhs = rhs[2:]
		case strings.HasPrefix(rhs, "="):
			word |= 0x02000000
			rhs = rhs[1:]
		default:
			return nil, nil, fmt.Errorf("invalid BA/PO operation")
		}
		if labelAllowed && !strings.HasPrefix(rhs, "$") && !strings.HasPrefix(strings.ToLower(rhs), "0x") {
			if _, ok := lookupValues(n.values)(rhs); !ok && identifier.MatchString(rhs) {
				return []uint32{word&0x08000000 | 0x46000000, 0}, &fixup{name: rhs}, nil
			}
		}
		modifier, tail, err := addressModifiers(rhs, true)
		if err != nil {
			return nil, nil, err
		}
		word |= modifier
		value, err := directiveAddress(n, tail)
		if err != nil {
			return nil, nil, err
		}
		return pair(word, value)
	}
	if m := registerDirective.FindStringSubmatch(s); m != nil {
		reg, _ := strconv.ParseUint(m[1], 16, 4)
		rhs := m[2]
		word := uint32(reg)
		if strings.HasPrefix(rhs, "<-") || strings.HasPrefix(rhs, "->") {
			if rhs[:2] == "<-" {
				word |= 0x82000000
			} else {
				word |= 0x84000000
			}
			rhs = rhs[2:]
			width := 32
			for _, bits := range []int{8, 16, 32} {
				prefix := fmt.Sprintf("(%d)", bits)
				if strings.HasPrefix(rhs, prefix) {
					width = bits
					rhs = rhs[len(prefix):]
					break
				}
			}
			if width == 16 {
				word |= 0x00100000
			}
			if width == 32 {
				word |= 0x00200000
			}
			mods, tail, err := addressModifiers(rhs, false)
			if err != nil {
				return nil, nil, err
			}
			word |= mods
			value, err := directiveAddress(n, tail)
			if err != nil {
				return nil, nil, err
			}
			return pair(word, value)
		}
		if strings.HasPrefix(rhs, "=") || strings.HasPrefix(rhs, "+=") {
			word |= 0x80000000
			if rhs[0] == '+' {
				word |= 0x00100000
				rhs = rhs[2:]
			} else {
				rhs = rhs[1:]
			}
			mods, tail, err := addressModifiers(rhs, false)
			if err != nil {
				return nil, nil, err
			}
			word |= mods
			value, err := directiveAddress(n, tail)
			if err != nil {
				return nil, nil, err
			}
			return pair(word, value)
		}
		if len(rhs) < 3 {
			return nil, nil, fmt.Errorf("invalid Gecko register operation")
		}
		operations := map[byte]uint32{'+': 0, '*': 1, '|': 2, '&': 3, '^': 4, '[': 5, ']': 6, '(': 7, ')': 8, 'a': 9, 'x': 10}
		op, ok := operations[rhs[0]]
		if !ok || (rhs[1] != '=' && rhs[1] != '<') {
			return nil, nil, fmt.Errorf("unknown Gecko register operator")
		}
		word |= op << 20
		if rhs[1] == '<' {
			word |= 0x20000
		}
		rhs = rhs[2:]
		if strings.HasPrefix(rhs, "$") {
			word |= 0x10000
			rhs = rhs[1:]
		}
		if strings.HasPrefix(strings.ToUpper(rhs), "GR") {
			if len(rhs) != 3 {
				return nil, nil, fmt.Errorf("invalid Gecko register %q", rhs)
			}
			other, err := strconv.ParseUint(rhs[2:], 16, 4)
			if err != nil {
				return nil, nil, err
			}
			return pair(word|0x88000000, uint32(other))
		}
		value, err := directiveAddress(n, rhs)
		if err != nil {
			return nil, nil, err
		}
		return pair(word|0x86000000, value)
	}
	return nil, nil, fmt.Errorf("unknown Gecko directive %q", n.text)
}
func addressModifiers(rhs string, allowRegister bool) (uint32, string, error) {
	word := uint32(0)
	upper := strings.ToUpper(rhs)
	if strings.HasPrefix(upper, "BA+") {
		word |= 0x10000
		rhs = rhs[3:]
	} else if strings.HasPrefix(upper, "PO+") {
		word |= 0x10010000
		rhs = rhs[3:]
	}
	if allowRegister && strings.HasPrefix(strings.ToUpper(rhs), "GR") {
		if len(rhs) < 4 || rhs[3] != '+' {
			return 0, "", fmt.Errorf("expected GRn+address")
		}
		reg, err := strconv.ParseUint(rhs[2:3], 16, 4)
		if err != nil {
			return 0, "", err
		}
		word |= 0x1000 | uint32(reg)
		rhs = rhs[4:]
	}
	return word, rhs, nil
}
