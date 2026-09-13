package expr

import (
	"fmt"
	"strings"
	"unicode"

	"gctrm/internal/dialect"
)

// syntaxScope distinguishes the reference alias grammar from operands and
// literal data fields. Correcting an evaluator must not expand its language.
type syntaxScope uint8

const (
	operandSyntax syntaxScope = iota
	aliasSyntax
	literalSyntax
)

func checkSyntax(text string, extended bool, scope syntaxScope) error {
	if extended {
		return nil
	}
	s := strings.TrimSpace(text)
	if strings.ContainsAny(s, "()<>~") {
		return fmt.Errorf("expression %q requires extensions.expression_syntax=true", text)
	}
	start := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if unicode.IsSpace(rune(c)) {
			continue
		}
		if start && c == '0' && i+1 < len(s) && (s[i+1] == 'b' || s[i+1] == 'B') {
			return fmt.Errorf("binary literal requires extensions.expression_syntax=true")
		}
		if strings.ContainsRune("+-*/%&^|", rune(c)) {
			// Signed literal operands existed in the reference; unary complement
			// and arbitrary parenthesized expressions did not.
			if start && (c == '+' || c == '-') && scope != aliasSyntax {
				continue
			}
			if scope == literalSyntax || scope == operandSyntax && c != '+' {
				return fmt.Errorf("expression %q requires extensions.expression_syntax=true", text)
			}
			start = true
		} else {
			start = false
		}
	}
	return nil
}

// EvalDataRules limits reference data fields to literals and named constants.
func EvalDataRules(text string, lookup Lookup, rules dialect.Rules) (int64, error) {
	if err := checkSyntax(text, rules.ExpressionSyntax, literalSyntax); err != nil {
		return 0, err
	}
	return evaluate(text, lookup, rules, false)
}
