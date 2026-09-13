package assembler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"gctrm/fixes"
)

func TestNonConsolePolicyThroughSourceForms(t *testing.T) {
	for _, source := range []string{
		"Probe\nop fsqrt f3,f4 @ $80001000\n",
		"Probe\nCODE @ $80001000\n{\nfsqrt f3,f4\n}\n",
		"Probe\nHOOK @ $80001000\n{\nfsqrt f3,f4\n}\n",
		"Probe\nPULSE\n{\nfsqrt f3,f4\n}\n",
		"Probe\n.macro X()\n{\nfsqrt f3,f4\n}\nCODE @ $80001000\n{\n%X()\n}\n",
		"Probe\n.include child.asm\n",
	} {
		for _, fixed := range []bool{false, true} {
			for _, allow := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/fixed=%t/allow=%t", source, fixed, allow), func(t *testing.T) {
					opts := Options{Fixes: fixes.FromBool(fixed), AllowNonConsoleInstructions: allow, ReadFile: func(string) ([]byte, error) {
						return []byte("Child\nop fsqrt f3,f4 @ $80001000\n"), nil
					}}
					_, err := Assemble(context.Background(), "probe.asm", []byte(source), opts)
					if allow && err != nil || !allow && (err == nil || !strings.Contains(err.Error(), "extensions.non_console_instructions=true")) {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestNonConsolePolicyDoesNotDecodeRawData(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		_, err := Assemble(context.Background(), "raw.asm", []byte("Raw\nCODE @ $80001000\n{\nword 0xe8640008\n}\n"), Options{Fixes: fixes.FromBool(fixed)})
		if err != nil {
			t.Fatal(err)
		}
	}
}
