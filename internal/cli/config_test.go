package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gctrm/assembler"
	"gctrm/fixes"
)

const configProbe = `Probe
.alias precedence = 6 ^ 3 & 1
.alias wrapped = 0xffffffff + 1
.alias reduced = wrapped / 2
CODE @ $80001000
{
li r3,010
word precedence
word reduced
bdnz -0x10
byte -1
float NaN
double NaN
fadd f3,f4,f5
lha r3,0(r4)
}
`

var sourceSwitches = []string{
	"semantics.decimal_leading_zeros", "semantics.c_operator_precedence", "semantics.signed_64_bit_aliases",
	"encoding.gnu_branch_hints", "validation.strict_register_prefixes", "encoding.sign_extend_data_slots", "encoding.alternative_float_nan", "encoding.alternative_double_nan",
}

func probeWords(legacy [8]bool) []uint32 {
	choose := func(index int, old, modern uint32) uint32 {
		if legacy[index] {
			return old
		}
		return modern
	}
	return []uint32{
		choose(0, 0x38600008, 0x3860000a), choose(1, 1, 7), choose(2, 0, 0x80000000),
		choose(3, 0x4220fff0, 0x4200fff0), choose(5, 0xff, 0xffffffff),
		choose(6, 0x7fffffff, 0x7fc00000), choose(7, 0x7fffffff, 0x7ff80000),
		choose(7, 0xffffffff, 1), 0xfc64282a, 0xa8640000,
	}
}

// Every combination is checked against observable assembly results, including
// register-spelling acceptance. Width, radix and precedence must be independent.
func TestAllSourceConfigurations(t *testing.T) {
	for mask := 0; mask < 1<<len(sourceSwitches); mask++ {
		var legacy [8]bool
		var config strings.Builder
		config.WriteString("version=2\n")
		for i, key := range sourceSwitches {
			legacy[i] = mask&(1<<i) != 0
			fmt.Fprintf(&config, "%s=%t\n", key, !legacy[i])
		}
		f, err := decodeConfig([]byte(config.String()))
		if err != nil {
			t.Fatal(err)
		}
		opts := assembler.Options{Fixes: fixes.FromBool(true), Compatibility: f.compatibility}
		r, err := assembler.Assemble(context.Background(), "probe.asm", []byte(configProbe), opts)
		if err != nil {
			t.Fatalf("configuration %08b: %v", mask, err)
		}
		if got, want := r.Codes[0].Words[2:12], probeWords(legacy); !reflect.DeepEqual(got, want) {
			t.Fatalf("configuration %08b: %08x != %08x", mask, got, want)
		}
		_, err = assembler.Assemble(context.Background(), "register.asm", []byte("Registers\nop fadd r3,r4,r5 @ $80001000"), opts)
		if (err == nil) != legacy[4] {
			t.Fatalf("register prefix switch %t: %v", legacy[4], err)
		}
		_, err = assembler.Assemble(context.Background(), "invalid.asm", []byte("Invalid\nop lwzu r3,4(r3) @ $80001000"), opts)
		if err == nil {
			t.Fatalf("configuration %08b disabled instruction validation", mask)
		}
	}
}

func TestIndividualSourceSwitchesThroughCLI(t *testing.T) {
	for _, baseline := range []bool{false, true} {
		for index, key := range sourceSwitches {
			t.Run(fmt.Sprintf("baseline=%t/%s", baseline, key), func(t *testing.T) {
				dir := t.TempDir()
				path, config := filepath.Join(dir, "probe.asm"), filepath.Join(dir, "config.toml")
				writeTestFile(t, path, configProbe)
				var legacy [8]bool
				for i := range legacy {
					legacy[i] = !baseline
				}
				legacy[index] = !legacy[index]
				var settings strings.Builder
				for i, name := range sourceSwitches {
					fmt.Fprintf(&settings, "%s=%t\n", name, !legacy[i])
				}
				writeTestFile(t, config, settings.String())
				var stdout, stderr bytes.Buffer
				if status := runWithFixes(context.Background(), []string{"--config", config, path}, "", &stdout, &stderr); status != 0 {
					t.Fatal(status, stderr.String())
				}
				data, err := os.ReadFile(filepath.Join(dir, "probe.GCT"))
				if err != nil {
					t.Fatal(err)
				}
				for i, want := range probeWords(legacy) {
					if got := binary.BigEndian.Uint32(data[16+i*4:]); got != want {
						t.Fatalf("word %d: %08x != %08x", i, got, want)
					}
				}
			})
		}
	}
}

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTOMLValidation(t *testing.T) {
	for _, config := range []string{
		"version=1", "version=3", "version='2'", "unknown=true", "[unknown]",
		"[semantcs]\ndecimal_leading_zeros=true", "[encoding]\nunknown=true",
		"[semantics]\ndecimal_leading_zeros='false'", "[semantics]\ndecimal_leading_zeros=1",
		"[semantics]\ndecimal_leading_zeros=false\ndecimal_leading_zeros=true", "semantics=false",
		"[semantics\ndecimal_leading_zeros=false", "[encoding]\nalternative_float_nan=[]", strings.Repeat(" ", maxConfigSize+1),
	} {
		if _, err := decodeConfig([]byte(config)); err == nil {
			t.Errorf("accepted %q", config[:min(len(config), 100)])
		}
	}
	// Quoted keys, comments, dotted keys and inline tables use ordinary TOML.
	f, err := decodeConfig([]byte("# mixed\nsemantics = { 'decimal_leading_zeros' = true }\nencoding.alternative_float_nan = false\n"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := assembler.Legacy.Resolve(f.compatibility)
	if err != nil || r.OctalLiterals || !r.FloatNaN || !r.DoubleNaN {
		t.Fatal(r, err)
	}
	for _, config := range []string{"", "version=2", "[semantics]"} {
		f, err := decodeConfig([]byte(config))
		if err != nil || f.exactINI || f.flatLog || f.lf {
			t.Fatal(f, err)
		}
	}
}

func TestExampleConfigAndIndependentLibraryCalls(t *testing.T) {
	data, err := os.ReadFile("../../gctrm.toml")
	if err != nil {
		t.Fatal(err)
	}
	f, err := decodeConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := assembler.Legacy.Resolve(f.compatibility)
	legacy, _ := assembler.Legacy.Resolve(assembler.Compatibility{})
	if err != nil || rules != legacy || f.exactINI || f.flatLog || f.lf {
		t.Fatal(rules, err)
	}
	// Configuration is per-call, not shared state. A mixed call must not change
	// the choices used by subsequent calls in the same process.
	config, err := decodeConfig([]byte("[semantics]\nsigned_64_bit_aliases=false\n[encoding]\nsign_extend_data_slots=true"))
	if err != nil {
		t.Fatal(err)
	}
	source := []byte("Test\n.alias negative=0-1\nCODE @ $80001000\n{\nbyte negative\n}")
	r, err := assembler.Assemble(context.Background(), "mixed.asm", source, assembler.Options{Fixes: fixes.FromBool(true), Compatibility: config.compatibility})
	if err != nil || r.Codes[0].Words[2] != 0xffffffff {
		t.Fatal(r, err)
	}
	r, err = assembler.Assemble(context.Background(), "legacy.asm", source, assembler.Options{Fixes: fixes.FromBool(true)})
	if err != nil || r.Codes[0].Words[2] != 0xff {
		t.Fatal(r, err)
	}
}

func FuzzConfig(f *testing.F) {
	for _, source := range []string{"", "[semantics]\ndecimal_leading_zeros=true", "[encoding]\nalternative_float_nan=true", "[bug_fixes]\nconsole_only=true", "version=99"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 8192 {
			t.Skip()
		}
		_, _ = decodeConfig([]byte(source))
	})
}

func TestConfigSelectionAndPrecedence(t *testing.T) {
	dir := t.TempDir()
	exe, config := filepath.Join(dir, "gctrm.exe"), filepath.Join(dir, "gctrm.toml")
	writeTestFile(t, config, "[semantics]\ndecimal_leading_zeros=false\n[cli]\nexact_ini_matching=true\nflat_logs=true\nlf_line_endings=true\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm.extra : -g\nfirst.asm : --set=semantics.decimal_leading_zeros=true -t\n")
	jobs, _, err := plan([]string{"first.asm"}, exe)
	if err != nil || jobs[0].flags.convert || !jobs[0].flags.text || jobs[0].flags.compatibility.OctalLiterals == nil || *jobs[0].flags.compatibility.OctalLiterals {
		t.Fatal(jobs, err)
	}
	jobs, _, err = plan([]string{"--set=semantics.decimal_leading_zeros=false", "first.asm", "second.asm"}, exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.flags.compatibility.OctalLiterals == nil || !*j.flags.compatibility.OctalLiterals || !j.flags.flatLog || !j.flags.lf {
			t.Fatal(j)
		}
	}
	jobs, _, err = plan([]string{"-i", "first.asm"}, exe)
	if err != nil || jobs[0].flags.compatibility.OctalLiterals == nil || !*jobs[0].flags.compatibility.OctalLiterals {
		t.Fatal(jobs, err)
	}
	jobs, _, err = plan([]string{"--no-config", "first.asm"}, exe)
	if err != nil || !jobs[0].flags.convert || jobs[0].flags.flatLog || jobs[0].flags.lf {
		t.Fatal(jobs, err)
	}
	// Explicit config replaces the sibling default instead of merging it.
	other := filepath.Join(dir, "other.toml")
	writeTestFile(t, other, strings.TrimSuffix(fixesTOML(true), "\n"))
	jobs, _, err = plan([]string{"--config=" + other, "-i", "first.asm"}, exe)
	if err != nil || jobs[0].flags.compatibility.OctalLiterals != nil || jobs[0].flags.exactINI {
		t.Fatal(jobs, err)
	}
	for _, args := range [][]string{
		{"--config"}, {"--config="}, {"--config", filepath.Join(dir, "absent.toml"), "first.asm"},
		{"--config", config, "--no-config", "first.asm"}, {"first.asm", "--no-config"},
		{"--config", config, "-o", config, "first.asm"},
	} {
		if _, _, err := plan(args, exe); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	// Selector-like output names and literal source names remain filenames.
	if _, _, err := plan([]string{"--no-config", "-o", "--config", "--", "--no-config"}, ""); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidConfigPreservesOutputs(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[encoding]\nalternative_float_nan='no'")
	input, output := filepath.Join(dir, "test.asm"), filepath.Join(dir, "test.GCT")
	writeTestFile(t, input, "Test\nop nop @ $80001000")
	writeTestFile(t, output, "keep me")
	var out, diagnostic bytes.Buffer
	if status := runWithFixes(context.Background(), []string{input}, exe, &out, &diagnostic); status != 2 {
		t.Fatal(status)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "keep me" || !strings.Contains(diagnostic.String(), "gctrm.toml") {
		t.Fatal(string(data), err, diagnostic.String())
	}
	for _, arg := range []string{"--help", "--version"} {
		if status := runWithFixes(context.Background(), []string{arg}, exe, &out, &diagnostic); status != 0 {
			t.Fatal(status)
		}
	}
	if status := runWithFixes(context.Background(), []string{"--no-config", input}, exe, &out, &diagnostic); status != 0 {
		t.Fatal(status, diagnostic.String())
	}
}

func TestIndependentOutputSwitches(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		dir := t.TempDir()
		config, input := filepath.Join(dir, "config.toml"), filepath.Join(dir, "root.asm")
		exe := filepath.Join(dir, "gctrm.exe")
		prefix, tree, native := mask&1 != 0, mask&2 != 0, mask&4 != 0
		writeTestFile(t, config, fmt.Sprintf("[cli]\nexact_ini_matching=%t\nflat_logs=%t\nlf_line_endings=%t\n", !prefix, !tree, !native))
		writeTestFile(t, input, "Root\n.include part.asm\n")
		writeTestFile(t, filepath.Join(dir, "part.asm"), "Part\nop nop @ $80001000\n")
		writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "root.asm.extra : -g\nroot.asm : -t\n")
		var out, diagnostic bytes.Buffer
		if status := runWithFixes(context.Background(), []string{"--config", config, "-l", input}, exe, &out, &diagnostic); status != 0 {
			t.Fatal(status, diagnostic.String())
		}
		log, err := os.ReadFile(filepath.Join(dir, "root_log.txt"))
		if err != nil {
			t.Fatal(err)
		}
		text, err := os.ReadFile(filepath.Join(dir, "root_codeset.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(log), "part.asm") != tree || bytes.ContainsRune(log, '\t') != tree || strings.HasPrefix(string(text), "RSBE01") != prefix {
			t.Fatalf("mask %d: %q / %q", mask, log, text)
		}
		if bytes.ContainsRune(log, '\r') != (native && runtime.GOOS == "windows") || bytes.ContainsRune(text, '\r') != (native && runtime.GOOS == "windows") {
			t.Fatalf("mask %d: wrong line endings", mask)
		}
	}
}
