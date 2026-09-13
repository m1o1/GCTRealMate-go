package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNonConsoleConfigDefaultsAndOverrides(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, decimal := range []string{"false", "true"} {
			for _, setting := range []string{"", "console_only=true\n", "console_only=false\n"} {
				for _, override := range []string{"", "--set=validation.console_only=true", "--set=validation.console_only=false"} {
					t.Run(fmt.Sprintf("fixed=%t/%s/%s/%s", fixed, decimal, setting, override), func(t *testing.T) {
						dir := t.TempDir()
						config, input := filepath.Join(dir, "config.toml"), filepath.Join(dir, "probe.asm")
						writeTestFile(t, config, fmt.Sprintf("bug_fixes=%t\n[semantics]\ndecimal_leading_zeros=%s\n[validation]\n%s", fixed, decimal, setting))
						writeTestFile(t, input, "Probe\nCODE @ $80001000\n{\nld r3,8(r4)\nlha r3,0(r4)\n}\n")
						output := filepath.Join(dir, "probe.GCT")
						writeTestFile(t, output, "previous output")
						args := []string{"--config", config, "-i"}
						if override != "" {
							args = append(args, override)
						}
						args = append(args, "--set=semantics.decimal_leading_zeros="+decimal, input)
						var out, diagnostic bytes.Buffer
						status := Run(context.Background(), args, "", &out, &diagnostic)
						allowed := !strings.Contains(setting, "=true")
						if override != "" {
							allowed = strings.HasSuffix(override, "=false")
						}
						data, err := os.ReadFile(output)
						if err != nil {
							t.Fatal(err)
						}
						if !allowed {
							if status != 1 || !strings.Contains(diagnostic.String(), "validation.console_only=false") || string(data) != "previous output" {
								t.Fatal(status, diagnostic.String(), string(data))
							}
							return
						}
						if status != 0 {
							t.Fatal(status, diagnostic.String())
						}
						ld, lha := uint32(0xe8640020), uint32(0xa0640000)
						if fixed {
							ld, lha = 0xe8640008, 0xa8640000
						}
						if binary.BigEndian.Uint32(data[16:]) != ld || binary.BigEndian.Uint32(data[20:]) != lha {
							t.Fatalf("policy mismatch: %x", data)
						}
					})
				}
			}
		}
	}
	for _, source := range []string{"", "bug_fixes=true", "[semantics]\ndecimal_leading_zeros=true"} {
		f, err := decodeConfig([]byte(source))
		if err != nil || !f.allowNonConsoleInstructions {
			t.Fatal(f, err)
		}
	}
	f, err := loadConfig("../../gctrm.toml", true)
	if err != nil || !f.allowNonConsoleInstructions {
		t.Fatal(f, err)
	}
	for _, source := range []string{"allow_non_console_instructions='true'", "allow_non_console_instructions=1", "console_only=false\nallow_non_console_instructions=false", "[legacy]\nallow_non_console_instructions=true"} {
		if _, err := decodeConfig([]byte(source)); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
}

func TestNonConsoleINIPrecedenceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[validation]\nconsole_only=false\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --set=validation.console_only=true\nsecond.asm : --set=validation.console_only=false\n")
	jobs, _, err := plan([]string{"first.asm", "--set=validation.console_only=true", "second.asm", "--bug-fixes=true", "--set=semantics.decimal_leading_zeros=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if job.flags.allowNonConsoleInstructions {
			t.Fatalf("unexpected opt-in for %s", job.file)
		}
	}
	jobs, _, err = plan([]string{"--set=validation.console_only=false", "first.asm", "--set=semantics.decimal_leading_zeros=true", "--bug-fixes=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if !job.flags.allowNonConsoleInstructions {
			t.Fatalf("lost opt-in for %s", job.file)
		}
	}
	for _, arg := range []string{"--set=validation.console_only", "--set=validation.console_only=1", "--set=validation.console_only=TRUE"} {
		if _, _, err := plan([]string{arg, "first.asm"}, ""); err == nil {
			t.Fatalf("accepted %s", arg)
		}
	}
}
