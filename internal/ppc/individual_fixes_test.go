package ppc

import (
	"testing"

	"gctrm/fixes"
)

func TestEncodingCorrectionsDoNotDisableOperandChecks(t *testing.T) {
	policy := fixes.All()
	policy.EQV = false
	policy.ShiftRightZero = false
	policy.LHA = false
	for _, source := range []string{
		"eqv r3,r4,r32", "eqv r3,r4,r5,r6",
		"srwi r3,r4,32", "lha r32,0(r4)",
	} {
		if word, err := Encode(source, Context{Fixes: policy}); err == nil {
			t.Fatalf("%s unexpectedly emitted %08x", source, word)
		}
	}
}
