package assembler

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gctrm/fixes"
)

// These full GCTs were captured from the pinned, unmodified C++ executable.
// Exceptions are retained as evidence, not silently edited into the oracle.
func TestLegacyCompatibilityCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Name, Source, Exception string
			Hex                     string `json:"gct_hex"`
			Exit                    int    `json:"exit_code"`
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for _, tc := range report.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Exit != 0 || tc.Hex == "" {
				t.Fatal("invalid reference capture")
			}
			// This historical comparison explicitly includes the numeric-branch extension.
			r, err := Assemble(context.Background(), "test.asm", []byte(tc.Source), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), BranchExpressions: true})
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(tc.Hex)
			if err != nil {
				t.Fatal(err)
			}
			if tc.Exception != "" {
				// Keep the reference bytes immutable. Account for only these
				// isolated defects/extensions, never a blanket mismatch allowance.
				var word uint32
				switch tc.Name {
				case "op_b 010":
					word = 0x48000008
				case "branch_bc+ 13,2,0x10":
					word = 0x41a20010
				case "branch_bc+ 13,2,-0x10":
					word = 0x4182fff0
				default:
					t.Fatalf("unreviewed exception %s", tc.Name)
				}
				binary.BigEndian.PutUint32(want[16:20], word)
			}
			if got := hex.EncodeToString(r.Bytes()); got != hex.EncodeToString(want) {
				t.Fatalf("Go %s; expected %x (original C++ %s)", got, want, tc.Hex)
			}
		})
	}
}

func TestDialectChoices(t *testing.T) {
	for _, tc := range []struct {
		body           string
		legacy, modern uint32
	}{
		{"li r3,010", 0x38600008, 0x3860000a},
		{".alias x = 6 ^ 3 & 1\nword x", 1, 7},
		{".alias x = 0xffffffff + 1\n.alias y = x / 2\nword y", 0, 0x80000000},
		{".alias x = 0 - 1\n.alias y = x / 2\nword y", 0x7fffffff, 0},
		{".alias x = 2 + 3 + 4\nword x", 9, 9}, // retained dropped-term fix
		{".alias x = 2 + 8 / 2\nword x", 5, 6}, // explicit left-to-right policy
		{".alias x = (2 + 8) / 2\nword x", 5, 5},
		{"bdnz -0x10", 0x4220fff0, 0x4200fff0},
		{"bc 13,2,-0x10", 0x4182fff0, 0x41a2fff0},
		{"float NaN", 0x7fffffff, 0x7fc00000},
		{"float -NaN", 0xffffffff, 0xffc00000},
		{"byte -1", 0xff, 0xffffffff},
		{"half -1", 0xffff, 0xffffffff},
		{".alias x=0-1\nbyte x", 0xff, 0xffffffff},
	} {
		for _, mode := range []Dialect{Legacy, Modern} {
			source := "Test\nCODE @ $80001000\n{\n" + tc.body + "\n}\n"
			r, err := Assemble(context.Background(), "test.asm", []byte(source), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), Dialect: mode})
			want := tc.legacy
			if mode == Modern {
				want = tc.modern
			}
			if err != nil {
				t.Fatalf("%s mode %d: %v", tc.body, mode, err)
			}
			if got := r.Codes[0].Words[2]; got != want {
				t.Errorf("%s mode %d: %08x != %08x", tc.body, mode, got, want)
			}
		}
	}
}

func TestDialectIndependentErrors(t *testing.T) {
	for _, source := range []string{
		"li r32,1", "lwzu r3,4(r3)", "bcctr 16,0", "ld r3,0(r4)",
		"crclr 6,6", "b missing", "li r3,1/0", "li r3,08garbage",
		".alias x=1/0\nword x", "psq_l f0,-2049(r3),0,0",
	} {
		for _, mode := range []Dialect{Legacy, Modern} {
			_, err := Assemble(context.Background(), "bad.asm", []byte("Test\nCODE @ $80001000\n{\n"+source+"\n}"), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), Dialect: mode})
			if err == nil {
				t.Errorf("mode %d accepted %s", mode, source)
			}
		}
	}
	_, err := Assemble(context.Background(), "test.asm", nil, Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), Dialect: 99})
	if err == nil || !strings.Contains(err.Error(), "dialect") {
		t.Fatal(err)
	}
}

func TestLegacyNarrowDataLimits(t *testing.T) {
	for _, source := range []string{"byte 256", "byte -129", ".alias x=0-129\nbyte x", "half 65536", ".alias x=0-32769\nhalf x"} {
		_, err := Assemble(context.Background(), "test.asm", []byte("Test\nCODE @ $80001000\n{\n"+source+"\n}"), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
		if err == nil {
			t.Errorf("narrowing hid overflow in %s", source)
		}
	}
	r := assemble(t, "Test\n.alias x=0-1\nCODE @ $80001000\n{\nbyte[2] x,0\nhalf[2] x,0\n}")
	// Each typed array packs its elements, then occupies whole instruction
	// slots. Single byte/half pseudo-operations occupy one slot each.
	if got := hex.EncodeToString(r.Bytes()); got != "00d0c0de00d0c0de0600100000000008ff000000ffff0000f000000000000000" {
		t.Fatal(got)
	}
}

// Guard the previously audited fixes in both modes. This is a regression check
// against the saved Go audit, not a replacement for its independent evidence.
// The historical audit accepted expanded branch syntax; request it explicitly.
func TestRetainedAuditCases(t *testing.T) {
	data, err := os.ReadFile("../validation/cpp-bug-probes.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Name, Source string
			Go           struct {
				Hex  string `json:"gct_hex"`
				Exit int    `json:"exit_code"`
			}
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	legacyChanges := map[string]struct {
		offset int
		word   uint32
	}{
		"precedence":               {12, 0x38600005},
		"alias_mixed_add_div":      {12, 0x38600005},
		"alias_bitwise_precedence": {12, 0x38600001},
		"branch_hint":              {16, 0x4220fff0},
		"leading_zero_radix":       {16, 0x38600008},
		"nan":                      {12, 0x7fffffff},
	}
	for _, tc := range report.Cases {
		for _, mode := range []Dialect{Legacy, Modern} {
			r, err := Assemble(context.Background(), "audit.asm", []byte(tc.Source), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation(), BranchExpressions: true,
				Dialect: mode, ReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
			})
			if tc.Go.Exit != 0 {
				if err == nil {
					t.Errorf("%s mode %d: accepted invalid audit input", tc.Name, mode)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s mode %d: %v", tc.Name, mode, err)
			}
			want, err := hex.DecodeString(tc.Go.Hex)
			if err != nil {
				t.Fatal(err)
			}
			if change, ok := legacyChanges[tc.Name]; ok && mode == Legacy {
				binary.BigEndian.PutUint32(want[change.offset:], change.word)
			}
			if hex.EncodeToString(r.Bytes()) != hex.EncodeToString(want) {
				t.Errorf("%s mode %d: %x != %x", tc.Name, mode, r.Bytes(), want)
			}
		}
	}
}
