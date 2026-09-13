package expr

import (
	"fmt"
	"strconv"
	"strings"

	"gctrm/internal/dialect"
)

// legacyAlias models the reference's accumulator grammar. A + or - updates
// the current accumulator but leaves its RHS to start another one; only the
// first accumulator is returned. Invalid native accesses become bounded errors.
func legacyAlias(text string, lookup Lookup, rules dialect.Rules) (int64, error) {
	var tokens []string
	start := 0
	for i, c := range text {
		if c == ',' || c == '=' {
			tokens = append(tokens, text[start:i])
			start = i + 1
			continue
		}
		if strings.ContainsRune("+-*/&|^~%", c) {
			tokens = append(tokens, text[start:i], string(c))
			start = i + 1
		}
	}
	tokens = append(tokens, text[start:])
	var values []int64
	var firstExpression strings.Builder
	narrow := func(v int64) int64 {
		if rules.Unsigned32BitAliases {
			return int64(uint32(v))
		}
		return v
	}
	number := func(s string) (int64, error) {
		if lookup != nil {
			if v, ok := lookup(s); ok {
				return narrow(v), nil
			}
		}
		base, offset := 10, 0
		if strings.HasPrefix(strings.ToLower(s), "0x") {
			base, offset = 16, 2
		} else if strings.HasPrefix(s, "0") && rules.OctalLiterals {
			base = 8
		}
		i := offset
		for i < len(s) && strings.ContainsRune("0123456789abcdef"[:base], rune(strings.ToLower(s[i : i+1])[0])) {
			i++
		}
		if i == offset {
			return 0, fmt.Errorf("invalid legacy alias value %q", s)
		}
		bits := 64
		if rules.Unsigned32BitAliases {
			bits = 32
		}
		v, err := strconv.ParseUint(s[offset:i], base, bits)
		return int64(v), err
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if len(token) == 1 && strings.Contains("+-*/&|^%", token) {
			if len(values) == 0 || i+1 >= len(tokens) {
				return 0, fmt.Errorf("incomplete legacy alias expression")
			}
			rhs, err := number(tokens[i+1])
			if err != nil {
				return 0, err
			}
			lhs := values[len(values)-1]
			if len(values) == 1 {
				fmt.Fprintf(&firstExpression, "%s%d", token, rhs)
			}
			switch token {
			case "+":
				lhs += rhs
			case "-":
				lhs -= rhs
			case "*":
				lhs *= rhs
			case "/":
				if rhs == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				lhs /= rhs
			case "%":
				if rhs == 0 {
					return 0, fmt.Errorf("remainder by zero")
				}
				lhs %= rhs
			case "&":
				lhs &= rhs
			case "|":
				lhs |= rhs
			case "^":
				lhs ^= rhs
			}
			values[len(values)-1] = narrow(lhs)
			if token != "+" && token != "-" {
				i++
			}
		} else {
			v, err := number(token)
			if err != nil {
				return 0, err
			}
			values = append(values, v)
			if len(values) == 1 {
				fmt.Fprint(&firstExpression, v)
			}
		}
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("empty legacy alias")
	}
	if !rules.LeftToRightExpressions {
		return evaluate(firstExpression.String(), lookup, rules, rules.Unsigned32BitAliases)
	}
	return values[0], nil
}
