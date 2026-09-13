package assembler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"gctrm/fixes"
)

func TestPackagedSourceContextCaptures(t *testing.T) {
	data, err := os.ReadFile("testdata/source-context-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source, GCT string }
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 40 {
		t.Fatalf("expected 40 native captures, got %d", len(fixture.Cases))
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(result.Bytes()); got != tc.GCT {
				t.Fatalf("got %s; packaged %s", got, tc.GCT)
			}
		})
	}
}

func TestReferenceBlockAdditionWithoutExtensions(t *testing.T) {
	// word 2+3 and word x+4 are also captured from the packaged executable
	// in validation/project-plus-settings-0.15.0.json.
	for _, fixed := range []bool{false, true} {
		for _, tc := range []struct {
			body string
			want uint32
		}{
			{"word 2+3", 5},
			{".alias x = 0x1000\nword x+4", 0x1004},
			{".alias x = 0x1000\nword x+0x60", 0x1060},
			{"word 0x10+010+-2", 22},
			{"byte 0xfe+1", 0xff},
			{"half 0xfffe+1", 0xffff},
		} {
			t.Run(fmt.Sprintf("fixed=%t/%s", fixed, tc.body), func(t *testing.T) {
				source := "Probe\nCODE @ $80001000\n{\n" + tc.body + "\n}"
				result, err := Assemble(context.Background(), "probe.asm", []byte(source), Options{Fixes: fixes.FromBool(fixed)})
				if err != nil {
					t.Fatal(err)
				}
				if got := result.Codes[0].Words; !reflect.DeepEqual(got, []uint32{0x06001000, 4, tc.want, 0}) {
					t.Fatalf("got %08x", got)
				}
			})
		}
	}
}

func TestDataExpressionContextsStaySeparate(t *testing.T) {
	for _, source := range []string{
		"word 2+3 @ $80001000", // Raw data conversion, not a PPC pseudo-instruction.
		"word[1+1] 2,3 @ $80001000",
		"CODE @ $80001000\n{\nword (2+3)\n}",
		"CODE @ $80001000\n{\nword 2*3\n}",
	} {
		for _, extended := range []bool{false, true} {
			_, err := Assemble(context.Background(), "probe.asm", []byte("Probe\n"+source), Options{Fixes: fixes.All(), ExpressionSyntax: extended})
			if (err == nil) != extended {
				t.Fatalf("%s, extended=%t: %v", source, extended, err)
			}
		}
	}
}

func TestFloatingPointDataContextPolicy(t *testing.T) {
	for _, tc := range []struct {
		body     string
		fallback uint32
		data     []uint32
	}{
		{"float NaN", 0xfc000000, []uint32{0x7fffffff}},
		{"float 1.0", 0xfc000000, []uint32{0x3f800000}},
		{"double NaN", 0xffffffff, []uint32{0x7fffffff, 0xffffffff}},
		{"double 1.0", 0xffffffff, []uint32{0x3ff00000, 0}},
	} {
		for _, form := range []string{"CODE @ $80001000", "HOOK @ $80001000", "PULSE", "op"} {
			for _, checkUnknown := range []bool{false, true} {
				for _, enabled := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/check=%t/extension=%t", tc.body, form, checkUnknown, enabled), func(t *testing.T) {
						policy := fixes.All()
						policy.UnknownInstructions = checkUnknown
						source := "Probe\n" + form + "\n{\n" + tc.body + "\n}"
						if form == "op" {
							source = "Probe\nop " + tc.body + " @ $80001000"
						}
						result, err := Assemble(context.Background(), "probe.asm", []byte(source), Options{Fixes: policy, FloatingPointData: enabled})
						if !enabled && checkUnknown {
							if err == nil || !strings.Contains(err.Error(), "extensions.floating_point_data=true") {
								t.Fatalf("expected extension diagnostic, got %v", err)
							}
							return
						}
						if form == "op" && enabled && len(tc.data) == 2 {
							if err == nil || !strings.Contains(err.Error(), "exactly one instruction word") {
								t.Fatalf("op must still reject eight-byte data: %v", err)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						want := tc.data
						if !enabled {
							want = []uint32{tc.fallback}
						}
						start := 2
						if form == "op" && len(want) == 1 {
							start = 1
						}
						if got := result.Codes[0].Words[start : start+len(want)]; !reflect.DeepEqual(got, want) {
							t.Fatalf("got %08x, want %08x", got, want)
						}
					})
				}
			}
		}
	}
}

func TestFloatingPointDataSizesDriveBranchLabels(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		policy := fixes.All()
		policy.UnknownInstructions = false
		source := "Probe\nCODE @ $80001000\n{\nb end\ndouble 1.0\nend:\nblr\n}"
		result, err := Assemble(context.Background(), "probe.asm", []byte(source), Options{Fixes: policy, FloatingPointData: enabled})
		if err != nil {
			t.Fatal(err)
		}
		want := []uint32{0x06001000, 12, 0x48000008, 0xffffffff, 0x4e800020, 0}
		if enabled {
			want = []uint32{0x06001000, 16, 0x4800000c, 0x3ff00000, 0, 0x4e800020}
		}
		if !reflect.DeepEqual(result.Codes[0].Words, want) {
			t.Fatalf("enabled=%t: %08x", enabled, result.Codes[0].Words)
		}
	}
}
