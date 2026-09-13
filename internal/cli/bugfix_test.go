package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	for _, precedence := range []string{"false", "true"} {
		for _, setting := range []string{"", fixesTOML(false), fixesTOML(true)} {
			for _, override := range []string{"", "--bug-fixes=false", "--bug-fixes=true"} {
				t.Run(precedence+"/"+setting+"/"+override, func(t *testing.T) {
					dir := t.TempDir()
					config, input := filepath.Join(dir, "config.toml"), filepath.Join(dir, "probe.asm")
					writeTestFile(t, config, setting+fmt.Sprintf("[semantics]\nc_operator_precedence=%s\n", precedence))
					writeTestFile(t, input, "Probe\nop lha r3,0(r4) @ $80001000\n")
					args := []string{"--config", config, "-i"}
					if override != "" {
						args = append(args, override)
					}
					// A semantics option must not reset the independent fixes choice.
					args = append(args, "--set=semantics.c_operator_precedence="+precedence, input)
					var out, diagnostic bytes.Buffer
					if code := Run(context.Background(), args, "", &out, &diagnostic); code != 0 {
						t.Fatal(code, diagnostic.String())
					}
					fixed := setting != fixesTOML(false)
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
	for _, source := range []string{"", "[semantics]\nc_operator_precedence=true"} {
		f, err := decodeConfig([]byte(source))
		if err != nil || !f.fixes.LHA {
			t.Fatal("fixes must default to enabled", f, err)
		}
	}
	f, err := loadConfig("../../gctrm.toml", true)
	if err != nil || !f.fixes.LHA {
		t.Fatal("template must enable fixes", f, err)
	}
	for _, source := range []string{"bug_fixes='true'", "bug_fixes=1", fixesTOML(true) + strings.TrimSuffix(fixesTOML(false), "\n"), "[legacy]\n" + strings.TrimSuffix(fixesTOML(true), "\n")} {
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
				writeTestFile(t, config, fixesTOML(false))
				args = append(args, "--no-config")
			case "empty explicit":
				writeTestFile(t, config, "")
				args = append(args, "--config", config)
			case "template":
				args = append(args, "--config", "../../gctrm.toml")
			case "explicit false":
				writeTestFile(t, config, fixesTOML(false))
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
	writeTestFile(t, config, fixesTOML(true))
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm : --bug-fixes=false\nsecond.asm : --bug-fixes=true\n")
	jobs, _, err := plan([]string{"--config", config, "first.asm", "--bug-fixes=false", "second.asm", "third.asm", "--set=semantics.c_operator_precedence=true", "fourth.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.flags.fixes.LHA {
			t.Fatalf("expected disabled fixes for %s", j.file)
		}
	}
	jobs, _, err = plan([]string{"--config", config, "--bug-fixes=true", "first.asm"}, exe)
	if err != nil || !jobs[0].flags.fixes.LHA {
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

func fixesTOML(enabled bool) string {
	var out strings.Builder
	for _, key := range []string{"lha", "eqv", "crandc", "crorc", "overflow_suffix", "shift_right_zero", "quantized_displacement", "indexed_quantized", "paired_single_record", "numeric_fields", "comparisons", "branch_prediction", "raw_data_eof", "scanner_multiply", "scanner_or", "alias_terms", "gr_index", "address_qualifiers", "directive_bit31", "mem2_writes", "mem2_hooks", "psa_tags", "goto_false", "gecko_label_offsets", "else_directives", "missing_labels", "unknown_instructions", "operand_counts", "operand_ranges", "suffix_validation", "register_relationships", "branch_ranges", "address_alignment", "psa_index_range", "gecko_line_framing", "ds_displacement", "missing_file_status", "text_line_termination"} {
		fmt.Fprintf(&out, "bug_fixes.%s=%t\n", key, enabled)
	}
	return out.String()
}
