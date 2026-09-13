package assembler

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gctrm/fixes"
)

// Historical corrected-mode fixtures exercised these features and checks
// before they became independent choices. New policy tests use explicit defaults.
func strictValidation() Validation {
	return Validation{RejectDuplicateLabels: true, StrictMacroCalls: true,
		RejectUndefinedMacros: true, RejectAddressAnnotations: true, RejectDataOverflow: true}
}

func enabledDotOp() *bool { enabled := true; return &enabled }

func assemble(t *testing.T, s string) *Result {
	t.Helper()
	r, e := Assemble(context.Background(), "test.asm", []byte(s), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestGoldenFiles(t *testing.T) {
	for _, name := range []string{"core", "includes"} {
		t.Run(name, func(t *testing.T) {
			r, e := Compile(context.Background(), filepath.Join("testdata", name+".asm"), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
			if e != nil {
				t.Fatal(e)
			}
			data, e := os.ReadFile(filepath.Join("testdata", name+".hex"))
			if e != nil {
				t.Fatal(e)
			}
			want, e := hex.DecodeString(strings.Join(strings.Fields(string(data)), ""))
			if e != nil {
				t.Fatal(e)
			}
			if name == "core" {
				// Keep the C++ oracle intact and account for independently
				// verified corrections at exact word offsets only.
				for _, c := range []struct {
					offset        int
					before, after uint32
				}{
					{0x114, 0x90001000, 0x90000000}, // Gecko BA carries upper 7 bits
					{0x118, 0x04000000, 0x04001000}, // keep the write offset
					{0x16c, 0x00000000, 0x80000000}, // preserve explicit BA value
					{0x174, 0x10000000, 0x90000000}, // preserve explicit PO value
				} {
					if binary.BigEndian.Uint32(want[c.offset:]) != c.before {
						t.Fatalf("C++ fixture changed at %#x", c.offset)
					}
					binary.BigEndian.PutUint32(want[c.offset:], c.after)
				}
			}
			if !bytes.Equal(r.Bytes(), want) {
				for i := 0; i < len(want) && i < len(r.Bytes()); i++ {
					if want[i] != r.Bytes()[i] {
						t.Errorf("first mismatch at byte %#x", i)
						break
					}
				}
				t.Fatalf("got %X\nwant %X", r.Bytes(), want)
			}
		})
	}
}
func TestLabelsAndPadding(t *testing.T) {
	r := assemble(t, "Code\nHOOK @ $80001000\n{\nstart: li r3,1\nb start\nb %END%\n}\n")
	want := "00d0c0de00d0c0dec200100000000002386000014bfffffc4800000400000000f000000000000000"
	if hex.EncodeToString(r.Bytes()) != want {
		t.Fatalf("%x", r.Bytes())
	}
	if !strings.Contains(r.Log(), "Code @ Off 0x8") {
		t.Fatal(r.Log())
	}
	if !strings.HasPrefix(r.Text(true, true), "RSBE01\n\nCode\n* C2001000") {
		t.Fatal(r.Text(true, true))
	}
}
func TestRawDataAtEOF(t *testing.T) {
	r := assemble(t, "Data\nbyte[3] 1,2,3\n")
	if got := hex.EncodeToString(r.Bytes()); got != "00d0c0de00d0c0de0102030000000000f000000000000000" {
		t.Fatal(got)
	}
}
func TestDiagnostics(t *testing.T) {
	for _, s := range []string{
		"Code\nHOOK @ $80001000\n{\nb nowhere\n}", "Code\nCODE @ $80001000\n{\nli r32,1\n}",
		"Code\n.macro X(<a>)\n{\nli <a>,1\n}\nHOOK @ $80001000\n{\n%X()\n}",
		"Code\n.alias x=1/0", "Code\n.include missing.asm", "Code\nHOOK @ $80001000\n{\nnop", "Code\n.GOTO->missing",
		"Code\nbyte[2] 1 @ $80001000", "Code\nword 0x100000000 @ $80001000", "Code\n.UNKNOWN", "Code\nlabel:\nlabel:",
		"Code\n.macro X()\n{\n%X()\n}\n%X()", "Code\nstring \"unterminated", "Code\n/* unterminated",
	} {
		t.Run(s, func(t *testing.T) {
			r, e := Assemble(context.Background(), "bad.asm", []byte(s), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
			if e == nil || r != nil {
				t.Fatalf("got result %v and error %v", r, e)
			}
			if !strings.Contains(e.Error(), "bad.asm:") {
				t.Fatal(e)
			}
		})
	}
}
func TestDisabledCode(t *testing.T) {
	r := assemble(t, "!Disabled\nHOOK @ $80001000\n{\nnonsense\n}\nEnabled\nop nop @ $80001000\n")
	if len(r.Codes) != 1 || r.Codes[0].Name != "Enabled" {
		t.Fatal(r)
	}
}
func TestLocalScope(t *testing.T) {
	r := assemble(t, "Scoped\n.alias x=1\nHOOK @ $80001000\n{\n.alias x=2\nli r3,x\n}\nop li r3,x @ $80001004")
	if r.Codes[0].Words[2] != 0x38600002 || r.Codes[0].Words[5] != 0x38600001 {
		t.Fatal(r.Codes[0].Words)
	}
}
func TestIncludeCycle(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.asm")
	source := []byte("Code\n.include a.asm\n")
	_, e := Assemble(context.Background(), file, source, Options{Fixes: fixes.FromBool(true), ReadFile: func(string) ([]byte, error) { return source, nil }})
	if e == nil || !strings.Contains(e.Error(), "cycle") {
		t.Fatal(e)
	}
}
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := Assemble(ctx, "test.asm", nil, Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestConcurrentAssemblies(t *testing.T) {
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, e := Assemble(context.Background(), "test.asm", []byte("Code\nop nop @ $80001000"), Options{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, DotOp: enabledDotOp(), Validation: strictValidation()})
			if e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
}
func TestBranchBase(t *testing.T) {
	base := uint32(0x80500000)
	r, e := Assemble(context.Background(), "test.asm", []byte("Code\nHOOK @ $80001000\n{\nbl $80500030\n}\n"), Options{Fixes: fixes.FromBool(true), BaseAddress: &base})
	if e != nil {
		t.Fatal(e)
	}
	if r.Codes[0].Words[2] != 0x48000021 {
		t.Fatalf("%08X", r.Codes[0].Words[2])
	}
}
func FuzzAssemble(f *testing.F) {
	for _, s := range []string{"Code\nop nop @ $80001000", "Code\nHOOK @ $80001000\n{\nblr\n}", "", "{", ".macro X()"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 8192 {
			t.Skip()
		}
		for _, mode := range []Dialect{Legacy, Modern} {
			for _, fixed := range []bool{false, true} {
				for _, allow := range []bool{false, true} {
					_, _ = Assemble(context.Background(), "fuzz.asm", []byte(s), Options{Fixes: fixes.FromBool(fixed), AllowNonConsoleInstructions: allow, Dialect: mode, ReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist }})
				}
			}
		}
	})
}
