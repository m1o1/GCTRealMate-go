package ppc

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"gctrm/internal/dialect"
)

func TestNonConsoleGNUCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/non-console-gnu.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Cases []goldenInstruction }
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("empty independent corpus")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Assembly, func(t *testing.T) {
			got, err := Encode(tc.Assembly, Context{BugFixes: true, AllowNonConsoleInstructions: true})
			if err != nil || got != tc.Word {
				t.Fatalf("Go %08x (%v), GNU %08x", got, err, tc.Word)
			}
		})
	}
}

func TestNonConsoleInstructionsRequireOptIn(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
			ctx := Context{BugFixes: fixed, Dialect: mode}
			for name := range unsupportedInstructions {
				for _, suffix := range []string{"", ".", "o", "o."} {
					if _, err := Encode(name+suffix+" 3,4,5", ctx); err == nil || !strings.Contains(err.Error(), "validation.console_only=false") {
						t.Fatalf("fixed=%t mode=%d %s%s: expected target rejection, got %v", fixed, mode, name, suffix, err)
					}
				}
			}
			for _, source := range []string{"cmp cr0,1,r3,r4", "cmpi 0,0x1,r3,4", "cmpl cr0,1,r3,r4", "cmpli 0,1,r3,4"} {
				if _, err := Encode(source, ctx); err == nil || !strings.Contains(err.Error(), "validation.console_only=false") {
					t.Fatalf("fixed=%t mode=%d %s: %v", fixed, mode, source, err)
				}
			}
		}
	}
	// Legacy prefix matching must not bypass the target restriction.
	for _, source := range []string{"fsqrtmisspelled f3,f4", "frsqrtesx f3,f4", "fselsx f3,f4,f5,f6"} {
		if _, err := Encode(source, Context{}); err == nil || !strings.Contains(err.Error(), "validation.console_only=false") {
			t.Fatalf("%s: %v", source, err)
		}
	}
}

func TestTargetAndBugFixPoliciesAreIndependent(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		ctx := Context{BugFixes: fixed, AllowNonConsoleInstructions: true}
		for _, tc := range []struct {
			source            string
			legacy, corrected uint32
		}{
			{"ld r3,8(r4)", 0xe8640020, 0xe8640008},
			{"lha r3,0(r4)", 0xa0640000, 0xa8640000},
			{"fsqrt f3,f4", 0xfc60202c, 0xfc60202c},
			{"cmpd r3,r4", 0x7c032000, 0x7c232000},
		} {
			t.Run(fmt.Sprintf("%s/fixed=%t", tc.source, fixed), func(t *testing.T) {
				want := tc.legacy
				if fixed {
					want = tc.corrected
				}
				if got, err := Encode(tc.source, ctx); err != nil || got != want {
					t.Fatalf("got %08x (%v), want %08x", got, err, want)
				}
			})
		}
	}
	for _, source := range []string{"ld r32,8(r4)", "ld r3,2(r4)", "ld r3,65536(r4)", "ldu r3,8(r3)", "stdu r3,8(r0)", "fsqrt f32,f4", "mulld r3,r4,r5,6"} {
		if _, err := Encode(source, Context{BugFixes: true, AllowNonConsoleInstructions: true}); err == nil {
			t.Fatalf("target opt-in disabled operand validation: %s", source)
		}
	}
}
