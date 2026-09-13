package ppc

import (
	"gctrm/internal/dialect"
	"testing"
)

func TestKnownEncodings(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want uint32
	}{
		{"nop", 0x60000000}, {"li r3, 1", 0x38600001}, {"lis r3, 0x8000", 0x3c608000}, {"addi r3, r4, -1", 0x3864ffff},
		{"lwz r3, 0x20(r4)", 0x80640020}, {"stw r3, -4(r1)", 0x9061fffc}, {"mr r3,r4", 0x7c832378},
		{"blr", 0x4e800020}, {"bctrl", 0x4e800421}, {"b 0x10", 0x48000010}, {"bl -0x4", 0x4bfffffd},
		{"beq 0x10", 0x41820010}, {"bne -0x4", 0x40a2fffc}, {"cmpwi r3,1", 0x2c030001},
		{"mflr r0", 0x7c0802a6}, {"mtlr r0", 0x7c0803a6}, {"rfi", 0x4c000064},
		{"lha r3,0(r4)", 0xa8640000}, {"addo r3,r4,r5", 0x7c642e14}, {"addo. r3,r4,r5", 0x7c642e15},
		{"psq_l f0, 0(r3), 0, 0", 0xe0030000}, {"psq_l f0, -8(r3), 0, 0", 0xe0030ff8},
		{"fadds f1,f2,f3", 0xec22182a}, {"fmuls f1,f2,f3", 0xec2200f2}, {"crandc 1,2,3", 0x4c221902},
		{"eqv r3,r4,r5", 0x7c832a38}, {"srwi r3,r4,0", 0x5483003e},
	} {
		t.Run(tc.s, func(t *testing.T) {
			got, err := Encode(tc.s, Context{BugFixes: true})
			if err != nil || got != tc.want {
				t.Fatalf("got %08X, %v; want %08X", got, err, tc.want)
			}
		})
	}
}
func TestBranchAddresses(t *testing.T) {
	pc := uint32(0x80001000)
	for _, tc := range []struct {
		s       string
		convert bool
		want    uint32
	}{{"bl $80001020", false, 0x48000021}, {"bla 0x1020", true, 0x48000021}, {"ba 0x1020", false, 0x48001022}} {
		got, err := Encode(tc.s, Context{BugFixes: true, Address: &pc, ConvertAbsolute: tc.convert})
		if err != nil || got != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	got, err := Encode("b end", Context{BugFixes: true, RelativeLabel: func(s string) (int64, bool) { return 8, s == "end" }})
	if err != nil || got != 0x48000008 {
		t.Fatal(got, err)
	}
}
func TestInstructionErrors(t *testing.T) {
	for _, s := range []string{"", "bogus r3", "addi r3", "li r32,1", "lwz r3,0(r99)", "li r3,0x10000", "b 3", "b 0x2000000", "beq 0x8000", "bl $80000000", "b missing", "nop.", "orco r3,r4,r5", "mfspr r3,nope", "psq_l f0,0(r3),2,0", "psq_foo f0,0(r3),0,0", "cmpwhat r3,r4", "blr 5", "mtcrf 256,r3"} {
		t.Run(s, func(t *testing.T) {
			if _, err := Encode(s, Context{BugFixes: true}); err == nil {
				t.Fatalf("accepted %q", s)
			}
		})
	}
}
func FuzzEncode(f *testing.F) {
	for _, s := range []string{"nop", "addi r3,r4,-1", "psq_l f0,0(r3),0,0", "b $80001234", "b 16+4", "b missing", "ld r3,8(r4)", "cmp cr0,1,r3,r4", "fsqrt f3,f4"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}
		for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
			for _, fixed := range []bool{false, true} {
				for _, allow := range []bool{false, true} {
					for _, extended := range []bool{false, true} {
						_, _ = Encode(s, Context{BugFixes: fixed, BranchExpressions: extended, AllowNonConsoleInstructions: allow, Dialect: mode})
					}
				}
			}
		}
	})
}
