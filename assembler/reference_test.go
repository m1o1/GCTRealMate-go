package assembler

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRefreshReferenceFixtures is opt-in. The ordinary suite needs neither C++
// nor the reference repository because its oracle outputs are checked in.
func TestRefreshReferenceFixtures(t *testing.T) {
	if os.Getenv("GCTRM_UPDATE_GOLDENS") != "1" {
		t.Skip("set GCTRM_UPDATE_GOLDENS=1 to regenerate the oracle fixtures")
	}
	exe := os.Getenv("GCTRM_REFERENCE")
	if exe == "" {
		t.Fatal("GCTRM_REFERENCE must identify the C++ v0.2.6 executable")
	}
	dir := t.TempDir()
	err := filepath.WalkDir("testdata", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("testdata", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		if filepath.Ext(path) != ".asm" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"core", "includes"} {
		input := filepath.Join(dir, name+".asm")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, exe, "-q", input)
		cmd.Stdin = strings.NewReader("\n")
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("reference: %v\n%s", err, output)
		}
		data, err := os.ReadFile(filepath.Join(dir, name+".GCT"))
		if err != nil {
			t.Fatal(err)
		}
		if len(data)%8 != 0 {
			t.Fatal("unaligned reference output")
		}
		var text strings.Builder
		for i := 0; i < len(data); i += 8 {
			fmt.Fprintf(&text, "%X\n", data[i:i+8])
		}
		if err = os.WriteFile(filepath.Join("testdata", name+".hex"), []byte(text.String()), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
