package assembler

import (
	"context"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

func TestProjectPlusSourceForms(t *testing.T) {
	r := assemble(t, `Code Menu data
.alias 076_OFFSET = 8
byte[4] | continue the array
1,2, | and its address
3,4, |
@ $80001000
* op word 0x12345678 @ $80001004
.op nop @ $80001008
CODE @ $80002000
{
first: second-label:
li,r3,1
lwz r4,076_OFFSET(3)
beq crouchCheck
b second-label
crouchCheck: blr
}
`)
	want := []uint32{0x04001000, 0x01020304, 0x04001004, 0x12345678, 0x04001008, 0x60000000,
		0x06002000, 20, 0x38600001, 0x80830008, 0x41820008, 0x4bfffff4, 0x4e800020, 0}
	if len(r.Codes) != 1 || r.Codes[0].Name != "Code Menu data" || !reflect.DeepEqual(r.Codes[0].Words, want) {
		t.Fatalf("unexpected output: %+v", r.Codes)
	}
}

func TestIEEEDataLiterals(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   uint32
	}{
		{"float Infinity", 0x7f800000}, {"float -Inf", 0xff800000}, {"float Infinite", 0x7f800000},
		{"float NaN", 0x7fffffff}, {"float 1.5f", 0x3fc00000},
	} {
		r := assemble(t, "IEEE\n"+tc.source+" @ $80001000")
		if r.Codes[0].Words[1] != tc.want {
			t.Fatalf("%s: %08x", tc.source, r.Codes[0].Words)
		}
	}
	r := assemble(t, "IEEE\ndouble -Infinity @ $80001000")
	b := r.Bytes()
	if got := binary.BigEndian.Uint64(b[16:24]); got != math.Float64bits(math.Inf(-1)) {
		t.Fatalf("double: %x", got)
	}
}

func TestSourceTyposRemainErrors(t *testing.T) {
	for _, source := range []string{
		".macro X(<r>)\n{\nli <r>,1\n}\nCODE @ $80001000\n{\n%X(r3\n}",
		"CODE @ $80001000\n{\n%undefined()\n}",
		"HOOK @ $80001000 12345678\n{\nnop\n}",
		"op addi r3,r3,1 @ $80001002",
		"op word[2] 1,2 @ $80001000",
		"CODE @ $80001000\n{\nlabel: nop\nlabel: blr\n}",
		"CODE @ $80001000\n{\nbeq absent\n}",
	} {
		if _, err := Assemble(context.Background(), "bad.asm", []byte("Test\n"+source), Options{BugFixes: true, ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()}); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
}
