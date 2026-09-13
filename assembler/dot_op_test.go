package assembler

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"gctrm/fixes"
)

func TestDotOpIndependentPolicy(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, mode := range []Dialect{Legacy, Modern} {
			for _, setting := range []string{"default", "true", "false"} {
				for _, source := range []string{
					"Probe\n.op lha r3,0(r4) @ $80001000\n",
					"Probe\n.macro Patch()\n{\n.op lha r3,0(r4) @ $80001000\n}\n%Patch()\n",
					"Probe\n.include child.asm\n",
				} {
					t.Run(fmt.Sprintf("fixed=%t/mode=%d/dot=%s/%s", fixed, mode, setting, source), func(t *testing.T) {
						opts := Options{Fixes: fixes.FromBool(fixed), Dialect: mode, ReadFile: func(string) ([]byte, error) {
							return []byte(".op lha r3,0(r4) @ $80001000\n"), nil
						}}
						enabled := setting == "true"
						if setting != "default" {
							opts.DotOp = &enabled
						}
						// Plain op must work regardless of alias availability.
						r, err := Assemble(context.Background(), "probe.asm", []byte(source+"op lha r3,0(r4) @ $80001004\n"), opts)
						if err != nil {
							t.Fatal(err)
						}
						data := r.Bytes()
						want := uint32(0xa0640000)
						if fixed {
							want = 0xa8640000
						}
						patches := 1
						if enabled {
							patches = 2
						}
						if len(data) != 16+8*patches {
							t.Fatalf("unexpected GCT: %x", data)
						}
						for offset := 12; offset < len(data)-8; offset += 8 {
							if got := binary.BigEndian.Uint32(data[offset:]); got != want {
								t.Fatalf("%08x != %08x", got, want)
							}
						}
					})
				}
			}
		}
	}
	if _, err := Assemble(context.Background(), "invalid.asm", []byte("Probe\n.op lha r32,0(r4) @ $80001000\n"), Options{Fixes: fixes.FromBool(true), DotOp: enabledDotOp()}); err == nil {
		t.Fatal("alias bypassed operand validation")
	}
}
