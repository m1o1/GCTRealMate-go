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
			for _, setting := range []string{"", "non_console_instructions=false\n", "non_console_instructions=true\n"} {
				for _, override := range []string{"", "--set=extensions.non_console_instructions=false", "--set=extensions.non_console_instructions=true"} {
					t.Run(fmt.Sprintf("fixed=%t/%s/%s/%s", fixed, decimal, setting, override), func(t *testing.T) {
						dir := t.TempDir()
						config, input := filepath.Join(dir, "config.toml"), filepath.Join(dir, "probe.asm")
						writeTestFile(t, config, fmt.Sprintf("bug_fixes=%t\n[semantics]\ndecimal_leading_zeros=%s\n[extensions]\n%s", fixed, decimal, setting))
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
						allowed := strings.Contains(setting, "=true")
						if override != "" {
							allowed = strings.HasSuffix(override, "=true")
						}
						data, err := os.ReadFile(output)
						if err != nil {
							t.Fatal(err)
						}
						if !allowed {
							if status != 1 || !strings.Contains(diagnostic.String(), "extensions.non_console_instructions=true") || string(data) != "previous output" {
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
		if err != nil || f.allowNonConsoleInstructions {
			t.Fatal(f, err)
		}
	}
	f, err := loadConfig("../../gctrm.toml", true)
	if err != nil || f.allowNonConsoleInstructions {
		t.Fatal(f, err)
	}
	for _, source := range []string{"allow_non_console_instructions='true'", "allow_non_console_instructions=1", "non_console_instructions=true\nallow_non_console_instructions=false", "[legacy]\nallow_non_console_instructions=true"} {
		if _, err := decodeConfig([]byte(source)); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
}

func TestNonConsoleINIPrecedenceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[extensions]\nnon_console_instructions=true\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --set=extensions.non_console_instructions=false\nsecond.asm : --set=extensions.non_console_instructions=true\n")
	jobs, _, err := plan([]string{"first.asm", "--set=extensions.non_console_instructions=false", "second.asm", "--bug-fixes=true", "--set=semantics.decimal_leading_zeros=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if job.flags.allowNonConsoleInstructions {
			t.Fatalf("unexpected opt-in for %s", job.file)
		}
	}
	jobs, _, err = plan([]string{"--set=extensions.non_console_instructions=true", "first.asm", "--set=semantics.decimal_leading_zeros=true", "--bug-fixes=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if !job.flags.allowNonConsoleInstructions {
			t.Fatalf("lost opt-in for %s", job.file)
		}
	}
	for _, arg := range []string{"--set=extensions.non_console_instructions", "--set=extensions.non_console_instructions=1", "--set=extensions.non_console_instructions=TRUE"} {
		if _, _, err := plan([]string{arg, "first.asm"}, ""); err == nil {
			t.Fatalf("accepted %s", arg)
		}
	}
}

func TestNonConsoleDefaultConfigPaths(t *testing.T) {
	for _, mode := range []string{"no executable", "missing automatic", "empty automatic", "no config", "empty explicit", "template", "explicit opt-in"} {
		for _, fixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fixed=%t", mode, fixed), func(t *testing.T) {
				dir := t.TempDir()
				exe, cfg := filepath.Join(dir, "gctrm.exe"), filepath.Join(dir, "gctrm.toml")
				src, output := filepath.Join(dir, "probe.asm"), filepath.Join(dir, "probe.GCT")
				writeTestFile(t, src, "Probe\nop ld r3,8(r4) @ $80001000")
				writeTestFile(t, output, "previous output")
				args := []string{"-i", fmt.Sprintf("--bug-fixes=%t", fixed)}
				switch mode {
				case "no executable":
					exe = ""
				case "empty automatic":
					writeTestFile(t, cfg, "")
				case "no config":
					writeTestFile(t, cfg, "[extensions]\nnon_console_instructions=true")
					args = append(args, "--no-config")
				case "empty explicit":
					writeTestFile(t, cfg, "")
					args = append(args, "--config", cfg)
				case "template":
					args = append(args, "--config", "../../gctrm.toml")
				case "explicit opt-in":
					writeTestFile(t, cfg, "[extensions]\nnon_console_instructions=true")
				}
				var out, diagnostic bytes.Buffer
				status := Run(context.Background(), append(args, src), exe, &out, &diagnostic)
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if mode != "explicit opt-in" {
					if status != 1 || string(data) != "previous output" || !strings.Contains(diagnostic.String(), "extensions.non_console_instructions=true") {
						t.Fatal(status, diagnostic.String(), string(data))
					}
					return
				}
				want := uint32(0xe8640020)
				if fixed {
					want = 0xe8640008
				}
				if status != 0 || len(data) != 24 || binary.BigEndian.Uint32(data[12:]) != want {
					t.Fatalf("%d %s %x", status, diagnostic.String(), data)
				}
			})
		}
	}
}
