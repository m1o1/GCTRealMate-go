package ppc

import (
	"fmt"
	"testing"

	"gctrm/fixes"
	"gctrm/internal/dialect"
)

func TestBranchExtensionsIndependentPolicy(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
				t.Run(fmt.Sprintf("fixed=%t/extended=%t/mode=%d", fixed, extended, mode), func(t *testing.T) {
					ctx := Context{Fixes: fixes.FromBool(fixed), BranchExpressions: extended, ExpressionSyntax: true, Dialect: mode}
					for _, target := range []string{"20", "16+4", "0b10100", "024"} {
						want := uint32(20)
						if target == "024" && mode == dialect.Modern {
							want = 24
						}
						for _, branch := range []struct {
							name string
							base uint32
						}{{"b", 0x48000000}, {"bl", 0x48000001}, {"ba", 0x48000002}, {"bla", 0x48000003}, {"beq", 0x41820000}, {"bc 12,2,", 0x41820000}} {
							word, err := Encode(branch.name+" "+target, ctx)
							if !extended && fixed {
								if err == nil {
									t.Fatalf("accepted %s %s", branch.name, target)
								}
								continue
							}
							displacement := want
							if !extended {
								displacement = 0
							}
							if err != nil || word != branch.base|displacement {
								t.Fatalf("%s %s: %08x, %v", branch.name, target, word, err)
							}
						}
					}
					for _, source := range []string{"b 0x14", "b -0x4", "b missing", "b 0x3", "beq 0x8000"} {
						_, err := Encode(source, ctx)
						wantError := fixed && source != "b 0x14" && source != "b -0x4"
						if (err != nil) != wantError {
							t.Fatalf("%s: %v", source, err)
						}
					}
					// Existing labels, addressed targets, and conversion are unaffected.
					pc := uint32(0x80001000)
					ctx.Address = &pc
					ctx.RelativeLabel = func(s string) (int64, bool) { return 20, s == "end" }
					for _, source := range []string{"b end", "b $80001014"} {
						word, err := Encode(source, ctx)
						if err != nil || word != 0x48000014 {
							t.Fatal(source, word, err)
						}
					}
					ctx.ConvertAbsolute = true
					word, err := Encode("ba 0x1014", ctx)
					if err != nil || word != 0x48000014 {
						t.Fatal(word, err)
					}
				})
			}
		}
	}
}
