package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Existing corrected-behavior tests opt in explicitly. Separate policy tests
// below exercise real zero-configuration defaults and both flag states.
func runWithFixes(ctx context.Context, args []string, executable string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return Run(ctx, args, executable, stdout, stderr)
	}
	return Run(ctx, append([]string{"--bug-fixes=true"}, args...), executable, stdout, stderr)
}

func TestBugFixDefaultsAndOverrides(t *testing.T) {
	for _, decimal := range []string{"false", "true"} {
		for _, setting := range []string{"", "bug_fixes=false\n", "bug_fixes=true\n"} {
			for _, override := range []string{"", "--bug-fixes=false", "--bug-fixes=true"} {
				t.Run(decimal+"/"+setting+"/"+override, func(t *testing.T) {
					dir := t.TempDir()
					config, input := filepath.Join(dir, "config.toml"), filepath.Join(dir, "probe.asm")
					writeTestFile(t, config, setting+fmt.Sprintf("[semantics]\ndecimal_leading_zeros=%s\n", decimal))
					writeTestFile(t, input, "Probe\nop lha r3,0(r4) @ $80001000\n")
					args := []string{"--config", config, "-i"}
					if override != "" {
						args = append(args, override)
					}
					// A semantics option must not reset the independent fixes choice.
					args = append(args, "--set=semantics.decimal_leading_zeros="+decimal, input)
					var out, diagnostic bytes.Buffer
					if code := Run(context.Background(), args, "", &out, &diagnostic); code != 0 {
						t.Fatal(code, diagnostic.String())
					}
					fixed := setting != "bug_fixes=false\n"
					if override != "" {
						fixed = override == "--bug-fixes=true"
					}
					want := uint32(0xa0640000)
					if fixed {
						want = 0xa8640000
					}
					data, err := os.ReadFile(filepath.Join(dir, "probe.GCT"))
					if err != nil {
						t.Fatal(err)
					}
					if got := binary.BigEndian.Uint32(data[12:]); got != want {
						t.Fatalf("%08x != %08x", got, want)
					}
				})
			}
		}
	}
	for _, source := range []string{"", "[semantics]\ndecimal_leading_zeros=true"} {
		f, err := decodeConfig([]byte(source))
		if err != nil || !f.bugFixes {
			t.Fatal("fixes must default to enabled", f, err)
		}
	}
	f, err := loadConfig("../../gctrm.toml", true)
	if err != nil || !f.bugFixes {
		t.Fatal("template must enable fixes", f, err)
	}
	for _, source := range []string{"bug_fixes='true'", "bug_fixes=1", "bug_fixes=true\nbug_fixes=false", "[legacy]\nbug_fixes=true"} {
		if _, err := decodeConfig([]byte(source)); err == nil {
			t.Fatalf("accepted invalid config %s", source)
		}
	}
}

func TestBugFixDefaultConfigPaths(t *testing.T) {
	for _, mode := range []string{"no executable", "missing automatic", "empty automatic", "no config", "empty explicit", "template", "explicit false"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "gctrm.exe")
			input := filepath.Join(dir, "probe.asm")
			config := filepath.Join(dir, "gctrm.toml")
			writeTestFile(t, input, "Probe\nop lha r3,0(r4) @ $80001000\n")
			args := []string{"-i"}
			switch mode {
			case "no executable":
				exe = ""
			case "empty automatic":
				writeTestFile(t, config, "")
			case "no config":
				writeTestFile(t, config, "bug_fixes=false\n")
				args = append(args, "--no-config")
			case "empty explicit":
				writeTestFile(t, config, "")
				args = append(args, "--config", config)
			case "template":
				args = append(args, "--config", "../../gctrm.toml")
			case "explicit false":
				writeTestFile(t, config, "bug_fixes=false\n")
			}
			args = append(args, input)
			var out, diagnostic bytes.Buffer
			if code := Run(context.Background(), args, exe, &out, &diagnostic); code != 0 {
				t.Fatal(code, diagnostic.String())
			}
			data, err := os.ReadFile(filepath.Join(dir, "probe.GCT"))
			if err != nil {
				t.Fatal(err)
			}
			want := uint32(0xa8640000)
			if mode == "explicit false" {
				want = 0xa0640000
			}
			if got := binary.BigEndian.Uint32(data[12:]); got != want {
				t.Fatalf("%08x != %08x", got, want)
			}
		})
	}
}

func TestBugFixINIPrecedenceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	config := filepath.Join(dir, "config.toml")
	writeTestFile(t, config, "bug_fixes=true\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --bug-fixes=false\nsecond.asm : --bug-fixes=true\n")
	jobs, _, err := plan([]string{"--config", config, "first.asm", "--bug-fixes=false", "second.asm", "third.asm", "--set=semantics.decimal_leading_zeros=true", "fourth.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.flags.bugFixes {
			t.Fatalf("expected disabled fixes for %s", j.file)
		}
	}
	jobs, _, err = plan([]string{"--config", config, "--bug-fixes=true", "first.asm"}, exe)
	if err != nil || !jobs[0].flags.bugFixes {
		t.Fatal(jobs, err)
	}
}

func TestLegacyReportedIncludeFailureStatus(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "probe.asm")
	writeTestFile(t, input, "Probe\n.include absent.asm\n")
	for _, fixed := range []bool{false, true} {
		var out, diagnostic bytes.Buffer
		code := Run(context.Background(), []string{"--no-config", fmt.Sprintf("--bug-fixes=%t", fixed), "-i", input}, "", &out, &diagnostic)
		want := 0
		if fixed {
			want = 1
		}
		if code != want || diagnostic.Len() == 0 {
			t.Fatal(code, diagnostic.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "probe.GCT")); !os.IsNotExist(err) {
			t.Fatal("failure wrote output", err)
		}
	}
}
