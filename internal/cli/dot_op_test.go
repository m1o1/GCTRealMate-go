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

func TestDotOpConfigAndOverrides(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, decimal := range []string{"false", "true"} {
			for _, setting := range []string{"", "dot_op=false\n", "dot_op=true\n"} {
				for _, override := range []string{"", "--set=extensions.dot_op=false", "--set=extensions.dot_op=true"} {
					t.Run(fmt.Sprintf("fixed=%t/%s/%s/%s", fixed, decimal, setting, override), func(t *testing.T) {
						dir := t.TempDir()
						config, input := filepath.Join(dir, "settings.toml"), filepath.Join(dir, "probe.asm")
						writeTestFile(t, config, fmt.Sprintf("bug_fixes=%t\n[semantics]\ndecimal_leading_zeros=%s\n[extensions]\n%s", fixed, decimal, setting))
						writeTestFile(t, input, "Probe\n.op lha r3,0(r4) @ $80001000\n")
						args := []string{"--config", config, "-i"}
						if override != "" {
							args = append(args, override)
						}
						args = append(args, "--set=semantics.decimal_leading_zeros="+decimal, fmt.Sprintf("--bug-fixes=%t", fixed), input)
						var out, diagnostic bytes.Buffer
						if code := Run(context.Background(), args, "", &out, &diagnostic); code != 0 {
							t.Fatal(code, diagnostic.String())
						}
						data, err := os.ReadFile(filepath.Join(dir, "probe.GCT"))
						if err != nil {
							t.Fatal(err)
						}
						enabled := setting == "dot_op=true\n"
						if override != "" {
							enabled = strings.HasSuffix(override, "=true")
						}
						if !enabled {
							if len(data) != 16 {
								t.Fatalf("disabled .op emitted a patch: %x", data)
							}
							return
						}
						want := uint32(0xa0640000)
						if fixed {
							want = 0xa8640000
						}
						if len(data) != 24 || binary.BigEndian.Uint32(data[12:]) != want {
							t.Fatalf("unexpected GCT: %x", data)
						}
					})
				}
			}
		}
	}
}

func TestDotOpDefaultsAndPrecedence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	for _, path := range []string{"", filepath.Join(dir, "missing.toml"), "../../gctrm.toml"} {
		f, err := loadConfig(path, false)
		if err != nil || f.dotOp {
			t.Fatal(path, f, err)
		}
	}
	for _, config := range []string{"", "bug_fixes=false", "[semantics]\ndecimal_leading_zeros=true"} {
		f, err := decodeConfig([]byte(config))
		if err != nil || f.dotOp {
			t.Fatal(config, f, err)
		}
	}
	for _, config := range []string{"dot_op='true'", "dot_op=1", "dot_op=true\ndot_op=false", "[legacy]\ndot_op=true"} {
		if _, err := decodeConfig([]byte(config)); err == nil {
			t.Fatalf("accepted invalid config %s", config)
		}
	}
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[extensions]\ndot_op=true\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --set=extensions.dot_op=false\nsecond.asm : --set=extensions.dot_op=true\n")
	jobs, _, err := plan([]string{"first.asm", "--set=extensions.dot_op=false", "second.asm", "--bug-fixes=true", "--set=semantics.decimal_leading_zeros=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.flags.dotOp {
			t.Fatalf("lost explicit opt-out for %s", j.file)
		}
	}
	jobs, _, err = plan([]string{"--set=extensions.dot_op=true", "first.asm", "--bug-fixes=false", "--set=semantics.decimal_leading_zeros=true", "third.asm"}, exe)
	if err != nil || len(jobs) != 2 || !jobs[0].flags.dotOp || !jobs[1].flags.dotOp {
		t.Fatal(jobs, err)
	}
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[extensions]\ndot_op=false\n")
	jobs, _, err = plan([]string{"--no-config", "-i", "first.asm"}, exe)
	if err != nil || jobs[0].flags.dotOp {
		t.Fatal(jobs, err)
	}
	for _, arg := range []string{"--set=extensions.dot_op", "--set=extensions.dot_op=1", "--set=extensions.dot_op=TRUE"} {
		if _, _, err := plan([]string{arg, "first.asm"}, ""); err == nil {
			t.Fatalf("accepted %s", arg)
		}
	}
}
