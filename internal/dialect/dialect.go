// Package dialect defines source choices independently of the bug-fix policy.
package dialect

import (
	"fmt"

	"gctrm/fixes"
)

type Mode uint8

const (
	Legacy Mode = iota
	Modern
)

// Overrides selects individual source behaviors. Nil inherits the preset;
// true selects the legacy behavior and false selects the modern behavior.
// The independent Fixes policy belongs to the assembly/encoding context.
type Overrides struct {
	LeftToRightExpressions *bool
	Unsigned32BitAliases   *bool
	BranchHints            *bool
	RegisterPrefixes       *bool
	ZeroExtendedData       *bool
	FloatNaN               *bool
	DoubleNaN              *bool
}

// Rules is a resolved, immutable value passed to the expression/data encoders.
type Rules struct {
	ExpressionSyntax       bool
	RejectDataOverflow     bool
	Fixes                  fixes.Policy
	LeftToRightExpressions bool
	Unsigned32BitAliases   bool
	BranchHints            bool
	RegisterPrefixes       bool
	ZeroExtendedData       bool
	FloatNaN               bool
	DoubleNaN              bool
}

// Resolve applies only explicitly supplied overrides to the selected preset.
func (m Mode) Resolve(o Overrides) (Rules, error) {
	if m != Legacy && m != Modern {
		return Rules{}, fmt.Errorf("unknown dialect %d", m)
	}
	choice := func(v *bool) bool {
		if v != nil {
			return *v
		}
		return m == Legacy
	}
	return Rules{
		ExpressionSyntax:       true, // Internal expression helpers expose the full grammar; assembler options narrow it.
		RejectDataOverflow:     true,
		Fixes:                  fixes.All(),
		LeftToRightExpressions: choice(o.LeftToRightExpressions),
		Unsigned32BitAliases:   choice(o.Unsigned32BitAliases),
		BranchHints:            choice(o.BranchHints),
		RegisterPrefixes:       choice(o.RegisterPrefixes),
		ZeroExtendedData:       choice(o.ZeroExtendedData),
		FloatNaN:               choice(o.FloatNaN),
		DoubleNaN:              choice(o.DoubleNaN),
	}, nil
}
