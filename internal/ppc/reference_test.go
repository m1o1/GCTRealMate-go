package ppc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gctrm/fixes"
)

type goldenInstruction struct {
	Assembly string `json:"assembly"`
	Word     uint32 `json:"word"`
}

// TestReferenceCorpus compares against checked-in words emitted by the C++
// v0.2.6 executable. Set both environment variables to regenerate deliberately.
func TestReferenceCorpus(t *testing.T) {
	path := filepath.Join("testdata", "reference.json")
	if os.Getenv("GCTRM_UPDATE_GOLDENS") == "1" {
		updateReference(t, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []goldenInstruction
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, tc := range corpus {
		t.Run(tc.Assembly, func(t *testing.T) {
			source := tc.Assembly
			name, _, _ := strings.Cut(source, " ")
			bare := strings.TrimSuffix(name, ".")
			if unsupportedInstructions[bare] || bare == "lmw" || bare == "lswi" {
				if _, err := Encode(source, Context{Fixes: fixes.FromBool(true)}); err == nil {
					t.Fatalf("accepted historical non-console/invalid input %s", source)
				}
				return
			}
			word, err := Encode(source, Context{Fixes: fixes.FromBool(true)})
			if err != nil || word != tc.Word {
				t.Fatalf("got %08X (%v), C++ emitted %08X", word, err, tc.Word)
			}
		})
	}
	t.Logf("retained %d C++ samples; valid words compared and historical non-console/invalid inputs rejected", len(corpus))
}
func updateReference(t *testing.T, path string) {
	t.Helper()
	exe := os.Getenv("GCTRM_REFERENCE")
	if exe == "" {
		t.Fatal("GCTRM_REFERENCE must identify the locally built C++ v0.2.6 executable")
	}
	// Keep this historical oracle's original input list fixed. New console
	// instructions are verified against GNU, not an older C++ implementation.
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var previous []goldenInstruction
	if err := json.Unmarshal(existing, &previous); err != nil {
		t.Fatal(err)
	}
	source := make([]string, len(previous))
	for i, tc := range previous {
		source[i] = tc.Assembly
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "reference.asm")
	text := "Instruction corpus\nCODE @ $80001000\n{\n" + strings.Join(source, "\n") + "\n}\n"
	if err := os.WriteFile(input, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-q", input)
	cmd.Stdin = strings.NewReader("\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reference: %v\n%s", err, out)
	}
	data, err := os.ReadFile(strings.TrimSuffix(input, ".asm") + ".GCT")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 16+4*len(source)+8 {
		t.Fatalf("short reference output (%d bytes)", len(data))
	}
	corpus := make([]goldenInstruction, len(source))
	for i, s := range source {
		corpus[i] = goldenInstruction{s, binary.BigEndian.Uint32(data[16+i*4:])}
	}
	data, err = json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Recorded %d C++ instruction encodings\n", len(corpus))
}
