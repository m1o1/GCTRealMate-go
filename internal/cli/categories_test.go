package cli

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCategoryDefaults(t *testing.T) {
	defaults, err := decodeConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !defaults.bugFixes || defaults.dotOp || defaults.branchExpressions || defaults.expressionSyntax || defaults.implicitSections || defaults.additionalConsoleInstructions || !defaults.allowNonConsoleInstructions || defaults.validation != (reflect.Zero(reflect.TypeOf(defaults.validation)).Interface()) || defaults.exactINI || defaults.flatLog || defaults.lf {
		t.Fatal(defaults)
	}
	for _, config := range []string{"[semantics]\nunknown=false", "[encoding]\nsign_extend_data_slots=1", "[validation]\nstrict_macro_calls='false'", "[cli]\nflat_logs=[]"} {
		if _, err := decodeConfig([]byte(config)); err == nil {
			t.Fatal(config)
		}
	}
}

func TestOptionalSourcePoliciesIndependentOfFixes(t *testing.T) {
	type example struct {
		key, source, baseline, enabled string
		baselineError, enabledError    bool
	}
	cases := []example{
		{"extensions.dot_op", "Probe\n.op nop @ $80001000", "", "0400100060000000", false, false},
		{"extensions.expression_syntax", "Probe\nop li r3,(2+3) @ $80001000", "", "0400100038600005", true, false},
		{"extensions.implicit_sections", "op nop @ $80001000", "", "0400100060000000", true, false},
		{"validation.strict_register_prefixes", "Probe\nop fadd r3,r4,r5 @ $80001000", "04001000fc64282a", "", false, true},
		{"validation.reject_duplicate_labels", "Probe\nlabel:\nlabel:\nop nop @ $80001000", "0400100060000000", "", false, true},
		{"validation.strict_macro_calls", "Probe\n.macro X()\n{\nop nop @ $80001000\n}\n%X(unused)", "0400100060000000", "", false, true},
		{"validation.reject_undefined_macros", "Probe\n%Absent()", "", "", false, true},
		{"validation.reject_address_annotations", "Probe\nop nop @ $80001000 note", "0400100060000000", "", false, true},
		{"validation.reject_data_overflow", "Probe\nbyte 256 @ $80001000", "0000100000000000", "", false, true},
		{"validation.reject_data_overflow", "Probe\nword 0x100000000 @ $80001000", "", "", true, true},
	}
	for _, tc := range cases {
		for _, fixed := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/fixed=%t/enabled=%t", tc.key, fixed, enabled), func(t *testing.T) {
					dir := t.TempDir()
					cfg, src := filepath.Join(dir, "settings.toml"), filepath.Join(dir, "probe.asm")
					group, key, _ := strings.Cut(tc.key, ".")
					writeTestFile(t, cfg, fmt.Sprintf("bug_fixes=%t\n[%s]\n%s=%t", fixed, group, key, enabled))
					writeTestFile(t, src, tc.source)
					output := filepath.Join(dir, "probe.GCT")
					writeTestFile(t, output, "keep previous output")
					var out, diagnostic bytes.Buffer
					status := Run(context.Background(), []string{"--config", cfg, "-i", src}, "", &out, &diagnostic)
					want, wantError := tc.baseline, tc.baselineError
					if enabled {
						want, wantError = tc.enabled, tc.enabledError
					}
					data, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					if wantError {
						if status != 1 || string(data) != "keep previous output" {
							t.Fatal(status, diagnostic.String(), string(data))
						}
					} else if status != 0 || len(data) < 16 || hex.EncodeToString(data[8:len(data)-8]) != want {
						t.Fatalf("%d %s %x want %s", status, diagnostic.String(), data, want)
					}
				})
			}
		}
	}
}

func TestSetOverridePrecedenceAndPersistence(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[validation]\nstrict_macro_calls=true\n[extensions]\nexpression_syntax=true")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "first.asm.extra : --set=validation.strict_macro_calls=false\nfirst.asm : --set=extensions.dot_op=true --set=validation.strict_macro_calls=false")
	jobs, _, err := plan([]string{"--set=cli.exact_ini_matching=true", "--set=validation.strict_macro_calls=true", "first.asm", "--set=semantics.decimal_leading_zeros=true", "second.asm"}, exe)
	if err != nil || len(jobs) != 2 {
		t.Fatal(jobs, err)
	}
	for _, j := range jobs {
		if !j.flags.dotOp || !j.flags.expressionSyntax || !j.flags.validation.StrictMacroCalls {
			t.Fatal(j.file, j.flags)
		}
	}
	for _, arg := range []string{"--set=extensions.dot_op", "--set=extensions.dot_op=1", "--set=extensions.dot_op=TRUE", "--set=validation.typo=false"} {
		if _, _, err := plan([]string{arg, "probe.asm"}, ""); err == nil {
			t.Fatal(arg)
		}
	}
}
