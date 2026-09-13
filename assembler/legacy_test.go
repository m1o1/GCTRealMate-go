package assembler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUncorrectedSourceCorpus(t *testing.T) {
	dotOp := false
	for _, filename := range []string{"../validation/cpp-bug-probes.json", "testdata/compatibility.json"} {
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Cases []struct {
				Name, Source string
				Hex          string `json:"gct_hex"`
				CPP          *struct {
					Hex string `json:"gct_hex"`
				} `json:"cpp"`
			}
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		for _, tc := range report.Cases {
			t.Run(filepath.Base(filename)+"/"+tc.Name, func(t *testing.T) {
				want := tc.Hex
				if tc.CPP != nil {
					want = tc.CPP.Hex
				}
				r, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{
					DotOp:                       &dotOp,
					AllowNonConsoleInstructions: true,
					ReadFile:                    func(string) ([]byte, error) { return nil, os.ErrNotExist },
				})
				if want == "" {
					if err == nil {
						t.Fatal("reference produces no GCT, expected bounded failure")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := hex.EncodeToString(r.Bytes()); got != want {
					t.Fatalf("Go %s; C++ %s", got, want)
				}
			})
		}
	}
}

func TestUncorrectedGoldenFiles(t *testing.T) {
	for _, name := range []string{"core", "includes"} {
		r, err := Compile(context.Background(), "testdata/"+name+".asm", Options{})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("testdata/" + name + ".hex")
		if err != nil {
			t.Fatal(err)
		}
		want := strings.ToLower(strings.Join(strings.Fields(string(data)), ""))
		if got := hex.EncodeToString(r.Bytes()); got != want {
			t.Fatalf("%s: Go %s; C++ %s", name, got, want)
		}
	}
}

func TestUncorrectedTextAndPrefixCaptures(t *testing.T) {
	data, err := os.ReadFile("testdata/bug-fix-compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Name, Source, Text, Log string
			Hex                     string `json:"gct_hex"`
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for _, tc := range report.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			r, err := Assemble(context.Background(), "probe.asm", []byte(tc.Source), Options{})
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(r.Bytes()) != tc.Hex || r.Text(false, false) != tc.Text || r.Log() != tc.Log {
				t.Fatalf("mismatch: GCT %x; text %q; log %q", r.Bytes(), r.Text(false, false), r.Log())
			}
		})
	}
}
