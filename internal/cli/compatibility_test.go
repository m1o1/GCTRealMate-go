package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyCLICaptures(t *testing.T) {
	data, err := os.ReadFile("testdata/compatibility.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Name, Ini      string
			Files, Outputs map[string]string
			Args           []string
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for _, tc := range report.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()
			for name, src := range tc.Files {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(src), 0600); err != nil {
					t.Fatal(err)
				}
			}
			exe := filepath.Join(dir, "reference.exe")
			if tc.Ini != "" {
				if err := os.WriteFile(filepath.Join(dir, "reference.ini"), []byte(tc.Ini), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string(nil), tc.Args...)
			for i, a := range args {
				if !strings.HasPrefix(a, "-") {
					args[i] = filepath.Join(dir, a)
				}
			}
			var out, diagnostic bytes.Buffer
			if status := Run(context.Background(), args, exe, &out, &diagnostic); status != 0 {
				t.Fatal(status, diagnostic.String())
			}
			for name, text := range tc.Outputs {
				want, err := hex.DecodeString(text)
				if err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS != "windows" && strings.HasSuffix(name, ".txt") {
					want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
				}
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || !bytes.Equal(got, want) {
					t.Errorf("%s: %q (%v); C++ %q", name, got, err, want)
				}
			}
		})
	}
}

func TestCompatibilityOptions(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "gctrm.exe")
	if err := os.WriteFile(strings.TrimSuffix(exe, ".exe")+".ini", []byte("x.asm.extra : -g\nx.asm : -l\n"), 0600); err != nil {
		t.Fatal(err)
	}
	jobs, _, err := plan([]string{"--set=cli.exact_ini_matching=true", "--set=semantics.c_operator_precedence=true", "--set=cli.flat_logs=true", "--set=cli.lf_line_endings=true", "x.asm", "--set=semantics.c_operator_precedence=false", "y.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	a, b := jobs[0].flags, jobs[1].flags
	if a.convert || !a.log || !a.exactINI || !a.flatLog || !a.lf || a.compatibility.LeftToRightExpressions == nil || *a.compatibility.LeftToRightExpressions || b.compatibility.LeftToRightExpressions == nil || !*b.compatibility.LeftToRightExpressions || !b.flatLog || !b.lf {
		t.Fatal(jobs)
	}
	for _, arg := range []string{"--dialect", "--dialect=other", "--ini-match=bad", "--log-format=bad", "--line-endings=bad", "--unknown=yes"} {
		if _, _, err := plan([]string{arg, "x.asm"}, ""); err == nil {
			t.Errorf("accepted %s", arg)
		}
	}
}

func TestAlternativeCLIOutputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.asm")
	if err := os.WriteFile(path, []byte("Main\n.include part.asm\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "part.asm"), []byte("Part\nop li r3,010 @ $80001000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := runWithFixes(context.Background(), []string{"--set=semantics.c_operator_precedence=true", "--set=cli.flat_logs=true", "--set=cli.lf_line_endings=true", "-t", "-l", path}, "", &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "main.GCT"))
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint32(data[12:]); got != 0x38600008 {
		t.Fatalf("%08x", got)
	}
	data, err = os.ReadFile(filepath.Join(dir, "main_log.txt"))
	if err != nil || string(data) != "Main @ Off 0x8\nPart @ Off 0x8\n" {
		t.Fatal(string(data), err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "main_codeset.txt"))
	if err != nil || bytes.ContainsRune(data, '\r') {
		t.Fatal(string(data), err)
	}
}
