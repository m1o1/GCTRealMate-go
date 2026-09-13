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

func TestExtensionSchema(t *testing.T) {
	for _, config := range []string{"", "[extensions]", "[semantics]\ndecimal_leading_zeros=true", "bug_fixes=false"} {
		f, err := decodeConfig([]byte(config))
		if err != nil || f.dotOp || f.branchExpressions || !f.allowNonConsoleInstructions {
			t.Fatal(f, err)
		}
	}
	for _, config := range []string{"branch_expressions=true", "extensions=false", "[extensions]\nunknown=true", "[extensions]\nbug_fixes=true", "[extensions]\ndialect='legacy'", "[extensions]\nbranch_expressions=1", "[extensions]\ndot_op='true'", "[extensions]\nallow_non_console_instructions='false'", "[extensions]\nbranch_expressions=true\nbranch_expressions=false"} {
		if _, err := decodeConfig([]byte(config)); err == nil {
			t.Fatalf("accepted %s", config)
		}
	}
	f, err := loadConfig("../../gctrm.toml", true)
	if err != nil || f.dotOp || f.branchExpressions || !f.allowNonConsoleInstructions || !f.bugFixes {
		t.Fatal(f, err)
	}
}

func TestBranchConfigAndCLI(t *testing.T) {
	for _, fixed := range []bool{false, true} {
		for _, setting := range []string{"", "branch_expressions=false", "branch_expressions=true"} {
			for _, override := range []string{"", "--set=extensions.branch_expressions=false", "--set=extensions.branch_expressions=true"} {
				for _, decimal := range []string{"false", "true"} {
					t.Run(fmt.Sprintf("fixed=%t/%s/%s/%s", fixed, setting, override, decimal), func(t *testing.T) {
						dir := t.TempDir()
						cfg, src := filepath.Join(dir, "settings.toml"), filepath.Join(dir, "probe.asm")
						writeTestFile(t, cfg, fmt.Sprintf("bug_fixes=%t\n[extensions]\n%s\n", fixed, setting))
						writeTestFile(t, src, "Probe\nop b 20 @ $80001000\nCODE @ $80001020\n{\nb 20\n}\n")
						outPath := filepath.Join(dir, "probe.GCT")
						writeTestFile(t, outPath, "previous output")
						args := []string{"--config", cfg, "-i"}
						if override != "" {
							args = append(args, override)
						}
						args = append(args, "--set=semantics.decimal_leading_zeros="+decimal, fmt.Sprintf("--bug-fixes=%t", fixed), src)
						var out, diagnostic bytes.Buffer
						status := Run(context.Background(), args, "", &out, &diagnostic)
						data, err := os.ReadFile(outPath)
						if err != nil {
							t.Fatal(err)
						}
						enabled := setting == "branch_expressions=true"
						if override != "" {
							enabled = strings.HasSuffix(override, "=true")
						}
						if !enabled && fixed {
							if status != 1 || string(data) != "previous output" || !strings.Contains(diagnostic.String(), "extensions.branch_expressions") {
								t.Fatal(status, diagnostic.String(), string(data))
							}
							return
						}
						want := uint32(0x48000000)
						if enabled {
							want |= 20
						}
						if status != 0 || len(data) != 40 || binary.BigEndian.Uint32(data[12:]) != want || binary.BigEndian.Uint32(data[24:]) != want {
							t.Fatalf("%d %s %x", status, diagnostic.String(), data)
						}
					})
				}
			}
		}
	}
}

func TestBranchINIPrecedenceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[extensions]\nbranch_expressions=true\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --set=extensions.branch_expressions=false\nsecond.asm : --set=extensions.branch_expressions=true\n")
	jobs, _, err := plan([]string{"first.asm", "--set=extensions.branch_expressions=false", "second.asm", "--set=semantics.decimal_leading_zeros=true", "--bug-fixes=true", "third.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.flags.branchExpressions {
			t.Fatal(j.file)
		}
	}
	jobs, _, err = plan([]string{"--set=extensions.branch_expressions=true", "first.asm", "--set=semantics.decimal_leading_zeros=false", "--bug-fixes=false", "third.asm"}, exe)
	if err != nil || len(jobs) != 2 || !jobs[0].flags.branchExpressions || !jobs[1].flags.branchExpressions {
		t.Fatal(jobs, err)
	}
	jobs, _, err = plan([]string{"--no-config", "-i", "first.asm"}, exe)
	if err != nil || jobs[0].flags.branchExpressions {
		t.Fatal(jobs, err)
	}
	for _, arg := range []string{"--set=extensions.branch_expressions", "--set=extensions.branch_expressions=1", "--set=extensions.branch_expressions=TRUE"} {
		if _, _, err := plan([]string{arg, "first.asm"}, ""); err == nil {
			t.Fatal(arg)
		}
	}
}
