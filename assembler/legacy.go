package assembler

import (
	"fmt"
	"strconv"
	"strings"

	"gctrm/fixes"
	"gctrm/internal/expr"
)

// Source-address policy is independent of encoding corrections. Optional full
// expressions are tried first; permissive reference annotations remain valid
// unless explicitly rejected.
func parseSourceAddress(s string, lookup expr.Lookup, expressions, strict bool) (uint32, error) {
	if expressions {
		v, err := parseAddress(s, lookup)
		if err == nil || strict {
			return v, err
		}
	}
	if strict {
		if strings.ContainsAny(s, "+-*/%()<>~&|^") {
			return 0, fmt.Errorf("address expression requires extensions.expression_syntax=true")
		}
		return parseAddress(s, lookup)
	}
	return parseAddressPolicy(s, lookup, false)
}

func parseAddressPolicy(s string, lookup expr.Lookup, bugFixes bool) (uint32, error) {
	if bugFixes {
		return parseAddress(s, lookup)
	}
	s = compact(s)
	if lookup != nil {
		if v, ok := lookup(strings.TrimPrefix(s, "$")); ok {
			return uint32(v), nil
		}
	}
	s = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(s), "$"), "0x")
	if len(s) > 8 {
		s = s[:8]
	}
	v, err := strconv.ParseUint(s, 16, 32)
	return uint32(v), err
}

// encodeDirectivePolicy preserves deterministic v0.2.6 quirks without copying
// its parser's unsafe indexing or exception handling into the corrected encoder.
func encodeDirectivePolicy(n node, policy fixes.Policy) ([]uint32, *fixup, error) {
	s := strings.ToUpper(strings.TrimPrefix(compact(n.text), "."))
	if !policy.UnknownInstructions && len(s) == 3 && strings.HasPrefix(s, "GR") && strings.ContainsRune("0123456789ABCDEF", rune(s[2])) {
		return []uint32{0, 0}, nil, nil
	}
	if !policy.ElseDirectives && s == "ELSE" {
		return nil, nil, fmt.Errorf("legacy GCTRealMate cannot compile .ELSE; enable bug_fixes.else_directives")
	}
	if !policy.ElseDirectives && s == "ELSE_RESET" {
		return nil, nil, nil
	}
	if !policy.AddressQualifiers && (strings.HasPrefix(s, "BA") || strings.HasPrefix(s, "PO")) {
		rhs := s[2:]
		// = checks qualifiers one byte too far along; -> and assignment
		// compare three characters against GR, so qualified forms fail.
		if strings.HasPrefix(rhs, "=") && (strings.Contains(rhs, "PO+") || strings.Contains(rhs, "BA+")) ||
			(strings.HasPrefix(rhs, "=") || strings.HasPrefix(rhs, "+=") || strings.HasPrefix(rhs, "->")) && strings.Contains(rhs, "GR") {
			return nil, nil, fmt.Errorf("legacy GCTRealMate cannot parse this BA/PO qualifier; enable bug_fixes.address_qualifiers")
		}
	}
	words, fix, err := encodeDirective(n)
	if err != nil {
		return nil, nil, err
	}
	if !policy.GotoFalse && strings.HasPrefix(s, "GOTO_F->") {
		return words[:1], fix, nil
	}
	if len(words) == 2 && fix == nil && (strings.HasPrefix(s, "BA") || strings.HasPrefix(s, "PO") || strings.HasPrefix(s, "GR")) {
		if !policy.DirectiveBit31 {
			words[1] &= 0x7fffffff
		}
		kind := words[0] & 0xee000000
		if !policy.GRIndex && (kind == 0x82000000 || kind == 0x84000000) {
			words[0] &^= 15
		}
	}
	return words, fix, nil
}
