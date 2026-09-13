package assembler

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIntentionalCorrections(t *testing.T) {
	cases := []struct {
		name, source string
		want         []uint32
	}{
		{"multiply alias", ".ALIAS x = (2 + 3) * 4 | 1\nop li r3,x @ $80001000", []uint32{0x04001000, 0x38600015}},
		{"register multiply", ".GR4 *= 00000002", []uint32{0x86100004, 2}},
		{"register load", ".GR5 <-(16) $80001000", []uint32{0x82100005, 0x80001000}},
		{"register store", ".GR6 ->(8) PO+$00001000", []uint32{0x94010006, 0x1000}},
		{"else reset", ".ELSE_RESET", []uint32{0xe2100001, 0x80008000}},
		{"PSA direct write", "RA_float 3 @ $80001000", []uint32{0x04001000, 0x21000003}},
		{"PSA block", "CODE @ $80001000\n{\nRA_float 3\n}", []uint32{0x06001000, 4, 0x21000003, 0}},
		{"MEM2 hook", "HOOK @ $90001000\n{\nblr\n}", []uint32{0x42000000, 0x90000000, 0xc2001000, 1, 0x4e800020, 0, 0xe0000000, 0x80008000}},
		{"negative GOTO", "back:\n* 04001000 60000000\n.GOTO->back", []uint32{0x04001000, 0x60000000, 0x6620fffe, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := assemble(t, "Example\n"+tc.source+"\n")
			if !reflect.DeepEqual(r.Codes[0].Words, tc.want) {
				t.Fatalf("got %08X want %08X", r.Codes[0].Words, tc.want)
			}
		})
	}
}
func TestAllDataTypes(t *testing.T) {
	for name, width := range dataWidths {
		t.Run(name, func(t *testing.T) {
			arg := "1"
			switch name {
			case "string":
				arg = "\"a@b\""
			case "float", "double", "scalar":
				arg = "1.5"
			case "address":
				arg = "$80001000"
			}
			r := assemble(t, fmt.Sprintf("Example\n%s %s @ $80001000", name, arg))
			if len(r.Codes[0].Words) < 2 {
				t.Fatal(r)
			}
			_ = width
		})
	}
	r := assemble(t, "Example\nCODE @ $80001000\n{\nbyte 1\nhalf 2\nword 3\nfloat 1\nstring \"a@b\"\n}")
	if got := hex.EncodeToString(r.Bytes()); got != "00d0c0de00d0c0de06001000000000140000000100000002000000033f8000006140620000000000f000000000000000" {
		t.Fatal(got)
	}
}
func TestDataValidation(t *testing.T) {
	for _, source := range []string{"byte[0] 1", "byte[2000000] 1", "byte[2 1,2", "byte 256", "half -32769", "word[2] 1", "scalar NaN", "scalar +Inf", "scalar 1000000", "RA_float 0x1000000", "string nope", ".macro Bad(<x>,<x>)\n{\nnop\n}"} {
		if _, e := Assemble(context.Background(), "bad.asm", []byte("Example\n"+source), Options{BugFixes: true, ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()}); e == nil {
			t.Errorf("accepted %s", source)
		}
	}
}
func TestCaseRepair(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "Parts"), 0700)
	os.WriteFile(filepath.Join(dir, "Parts", "Code.asm"), []byte("op nop @ $80001000"), 0600)
	r, e := Assemble(context.Background(), filepath.Join(dir, "root.asm"), []byte("Example\n.include parts/code.asm"), Options{BugFixes: true, ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), RepairPathCase: true})
	if e != nil || len(r.Codes[0].Words) != 2 {
		t.Fatal(r, e)
	}
}
func TestDirectiveVariants(t *testing.T) {
	for _, source := range []string{".BA <- $80001000", ".BA -> PO+$80001000", ".PO += $1000", ".BA <+ BA+GR3+$1000", ".GR1 ^= GR2", ".GR1 |< $FF", ".GR1 = BA+$1000", ".GOTO_T->next\nnext:", ".GOTO_F->next\nnext:", ".END", ".ELSE"} {
		assemble(t, "Example\n"+source)
	}
	for _, source := range []string{".BA ???", ".GR1 ?= 1", ".GR1 ^= GR99", ".BA = GR9", ".GOTO->!", ".GR1 -> $xyz"} {
		if _, e := Assemble(context.Background(), "bad.asm", []byte("Example\n"+source), Options{BugFixes: true, ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()}); e == nil {
			t.Errorf("accepted %s", source)
		}
	}
}
