package assembler

import (
	"context"
	"fmt"
	"testing"
)

func TestBranchExtensionsInMacrosAndIncludes(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, wrapper := range []string{"direct", "macro", "include"} {
				t.Run(fmt.Sprintf("fixed=%t/enabled=%t/%s", fixed, enabled, wrapper), func(t *testing.T) {
					body := "b 20\n"
					prefix := ""
					if wrapper == "macro" {
						prefix = ".macro jump()\n{\nb 20\n}\n"
						body = "%jump()\n"
					} else if wrapper == "include" {
						body = ".include \"branch.asm\"\n"
					}
					source := "Probe\n" + prefix + "CODE @ $80001000\n{\n" + body + "}\n"
					r, err := Assemble(context.Background(), "probe.asm", []byte(source), Options{
						BugFixes: fixed, BranchExpressions: enabled,
						ReadFile: func(string) ([]byte, error) { return []byte("b 20\n"), nil },
					})
					if !enabled && fixed {
						if err == nil {
							t.Fatal("accepted expanded target without extension")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					want := uint32(0x48000000)
					if enabled {
						want |= 20
					}
					if r.Codes[0].Words[2] != want {
						t.Fatalf("%08x", r.Codes[0].Words[2])
					}
				})
			}
		}
	}
}
