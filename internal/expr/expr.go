// Package expr evaluates assembler integer expressions without executing code.
package expr

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"gctrm/internal/dialect"
)

// Lookup resolves a named constant. Names are passed through unchanged.
type Lookup func(string) (int64, bool)

// Eval supports decimal, 0x/$ hexadecimal, 0b binary, parentheses, unary
// + - ~, and C-style precedence for * / % + - << >> & ^ |.
// Arithmetic uses signed 64-bit integers; encoding checks field widths later.
func Eval(text string, lookup Lookup) (int64, error) {
	return EvalMode(text, lookup, dialect.Modern)
}

// EvalMode evaluates operands. Legacy uses octal leading-zero literals and
// left-to-right binary operators; Modern uses decimal and C-style precedence.
func EvalMode(text string, lookup Lookup, mode dialect.Mode) (int64, error) {
	rules, err := mode.Resolve(dialect.Overrides{})
	if err != nil {
		return 0, err
	}
	return EvalRules(text, lookup, rules)
}

// EvalRules evaluates an operand with independently selected source rules.
func EvalRules(text string, lookup Lookup, rules dialect.Rules) (int64, error) {
	if err := checkSyntax(text, rules.ExpressionSyntax, operandSyntax); err != nil {
		return 0, err
	}
	return evaluate(text, lookup, rules, false)
}

// Alias applies the legacy Windows assembler's unsigned 32-bit arithmetic.
// Every intermediate result wraps, including inside parentheses. Unlike the
// old parser, every term is consumed and division by zero is diagnosed.
func Alias(text string, lookup Lookup, mode dialect.Mode) (int64, error) {
	rules, err := mode.Resolve(dialect.Overrides{})
	if err != nil {
		return 0, err
	}
	return AliasRules(text, lookup, rules)
}

// AliasRules applies alias arithmetic width separately from radix and order.
func AliasRules(text string, lookup Lookup, rules dialect.Rules) (int64, error) {
	if !rules.BugFixes {
		return legacyAlias(strings.Join(strings.Fields(text), ""), lookup, rules)
	}
	if err := checkSyntax(text, rules.ExpressionSyntax, aliasSyntax); err != nil {
		return 0, err
	}
	return evaluate(text, lookup, rules, rules.Unsigned32BitAliases)
}

func evaluate(text string, lookup Lookup, rules dialect.Rules, word bool) (int64, error) {
	p := parser{s: strings.TrimSpace(text), lookup: lookup, rules: rules, word: word}
	v, err := p.parse(1, 0)
	p.space()
	if err == nil && p.i != len(p.s) {
		err = fmt.Errorf("unexpected token %q", p.s[p.i:])
	}
	return v, err
}

type parser struct {
	s      string
	i      int
	lookup Lookup
	rules  dialect.Rules
	word   bool
}

func (p *parser) narrow(v int64) int64 {
	if p.word {
		return int64(uint32(v))
	}
	return v
}

func (p *parser) space() {
	for p.i < len(p.s) && unicode.IsSpace(rune(p.s[p.i])) {
		p.i++
	}
}

var precedence = map[string]int{"|": 1, "^": 2, "&": 3, "<<": 4, ">>": 4, "+": 5, "-": 5, "*": 6, "/": 6, "%": 6}

func (p *parser) parse(min, depth int) (int64, error) {
	if depth > 128 {
		return 0, fmt.Errorf("expression nesting exceeds 128")
	}
	lhs, err := p.atom(depth + 1)
	if err != nil {
		return 0, err
	}
	for {
		p.space()
		if p.i == len(p.s) {
			break
		}
		op := p.s[p.i : p.i+1]
		if p.i+1 < len(p.s) && (p.s[p.i:p.i+2] == "<<" || p.s[p.i:p.i+2] == ">>") {
			op = p.s[p.i : p.i+2]
		}
		prec := precedence[op]
		if p.rules.LeftToRightExpressions && prec != 0 {
			prec = 1
		}
		if prec < min {
			break
		}
		p.i += len(op)
		rhs, err := p.parse(prec+1, depth+1)
		if err != nil {
			return 0, err
		}
		switch op {
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
		case "<<", ">>":
			limit := int64(63)
			if p.word {
				limit = 31
			}
			if rhs < 0 || rhs > limit {
				return 0, fmt.Errorf("shift count %d outside 0..%d", rhs, limit)
			}
			if op == "<<" {
				lhs <<= uint(rhs)
			} else {
				lhs >>= uint(rhs)
			}
		}
		lhs = p.narrow(lhs)
	}
	return lhs, nil
}
func (p *parser) atom(depth int) (int64, error) {
	if depth > 128 {
		return 0, fmt.Errorf("expression nesting exceeds 128")
	}
	p.space()
	if p.i == len(p.s) {
		return 0, fmt.Errorf("expected an expression")
	}
	c := p.s[p.i]
	if c == '+' || c == '-' || c == '~' {
		p.i++
		v, e := p.atom(depth + 1)
		if c == '-' {
			v = -v
		}
		if c == '~' {
			v = ^v
		}
		return p.narrow(v), e
	}
	if c == '(' {
		p.i++
		v, e := p.parse(1, depth+1)
		if e != nil {
			return 0, e
		}
		p.space()
		if p.i == len(p.s) || p.s[p.i] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.i++
		return v, nil
	}
	start := p.i
	for p.i < len(p.s) {
		c = p.s[p.i]
		if c != '_' && c != '$' && c != '.' && !unicode.IsLetter(rune(c)) && !unicode.IsDigit(rune(c)) {
			break
		}
		p.i++
	}
	if p.i == start {
		return 0, fmt.Errorf("unexpected character %q", p.s[p.i])
	}
	token := p.s[start:p.i]
	if p.lookup != nil {
		if v, ok := p.lookup(token); ok {
			return p.narrow(v), nil
		}
	}
	base, num := 10, token
	if strings.HasPrefix(num, "$") {
		base, num = 16, num[1:]
	} else if strings.HasPrefix(strings.ToLower(num), "0x") {
		base, num = 16, num[2:]
	} else if strings.HasPrefix(strings.ToLower(num), "0b") {
		base, num = 2, num[2:]
	} else if p.rules.OctalLiterals && len(num) > 1 && num[0] == '0' {
		base = 8
	}
	v, e := strconv.ParseUint(num, base, 64)
	if e != nil {
		return 0, fmt.Errorf("unknown symbol or invalid number %q", token)
	}
	if p.word && v > 0xffffffff {
		return 0, fmt.Errorf("alias literal %q exceeds 32 bits", token)
	}
	return p.narrow(int64(v)), nil
}
