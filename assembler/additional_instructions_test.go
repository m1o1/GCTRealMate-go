package assembler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// These outputs were freshly captured from the pinned C++ executable. Retain
// native fallback words when extensions/fixes are off, including m*/c* dispatch.
func TestAdditionalInstructionReferenceProfile(t *testing.T) {
	data, err := os.ReadFile("testdata/additional-instructions-compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Name, Source string
			CPP          struct {
				Hex string `json:"gct_hex"`
			}
		}
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 37 {
		t.Fatal(len(report.Cases))
	}
	for _, tc := range report.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			r, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{AllowNonConsoleInstructions: true})
			if tc.CPP.Hex == "" {
				if err == nil {
					t.Fatal("expected bounded failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(r.Bytes()); got != tc.CPP.Hex {
				t.Fatalf("%s != %s", got, tc.CPP.Hex)
			}
			if _, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{BugFixes: true, AllowNonConsoleInstructions: true}); err == nil {
				t.Fatal("accepted added instruction by default")
			}
			if _, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{BugFixes: true, AllowNonConsoleInstructions: true, AdditionalConsoleInstructions: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
