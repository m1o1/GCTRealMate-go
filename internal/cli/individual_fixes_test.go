package cli

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gctrm/fixes"
	"gctrm/internal/dialect"
)

// Expectations come from the retained reference captures and explicit instruction
// words, not the encoder under test. Every correction runs alone against otherwise
// corrected defaults, through both TOML and a conflicting CLI override.
func TestIndividualFixBehavior(t *testing.T) {
	data, err := os.ReadFile("testdata/individual-fixes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Key, Source string
		Off, On     *string
		OffText     string `json:"off_text"`
		OnText      string `json:"on_text"`
		OffStatus   *int   `json:"off_status"`
		Settings    map[string]bool
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		if seen[tc.Key] {
			t.Fatalf("duplicate probe %s", tc.Key)
		}
		seen[tc.Key] = true
		for _, enabled := range []bool{false, true} {
			for _, override := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%t/override=%t", tc.Key, enabled, override), func(t *testing.T) {
					dir := t.TempDir()
					config := filepath.Join(dir, "config.toml")
					input := filepath.Join(dir, "probe.asm")
					var cfg strings.Builder
					cfg.WriteString("[bug_fixes]\n")
					for key, value := range tc.Settings {
						fmt.Fprintf(&cfg, "%s=%t\n", key, value)
					}
					configured := enabled
					if override {
						configured = !enabled
					}
					fmt.Fprintf(&cfg, "%s=%t\n", tc.Key, configured)
					writeTestFile(t, config, cfg.String())
					writeTestFile(t, input, tc.Source)
					args := []string{"--config", config, "-i", "-t"}
					if override {
						args = append(args, fmt.Sprintf("--set=bug_fixes.%s=%t", tc.Key, enabled))
					}
					args = append(args, input)
					var out, diagnostic bytes.Buffer
					status := Run(context.Background(), args, "", &out, &diagnostic)
					want, wantText := tc.Off, tc.OffText
					if enabled {
						want, wantText = tc.On, tc.OnText
					}
					wantStatus := 0
					if want == nil {
						wantStatus = 1
					}
					if !enabled && tc.OffStatus != nil {
						wantStatus = *tc.OffStatus
					}
					if status != wantStatus {
						t.Fatalf("status %d, want %d: %s", status, wantStatus, diagnostic.String())
					}
					got, err := os.ReadFile(filepath.Join(dir, "probe.GCT"))
					if want == nil {
						if !os.IsNotExist(err) {
							t.Fatalf("failed assembly wrote output: %x, %v", got, err)
						}
						return
					}
					if err != nil || hex.EncodeToString(got) != *want {
						t.Fatalf("got %x, want %s: %v", got, *want, err)
					}
					if wantText != "" {
						text, err := os.ReadFile(filepath.Join(dir, "probe_codeset.txt"))
						if err != nil || strings.ReplaceAll(string(text), "\r\n", "\n") != wantText {
							t.Fatalf("text %q, want %q: %v", text, wantText, err)
						}
					}
				})
			}
		}
	}
	// The shipped template is also the inventory: adding an option requires a
	// behavioral probe, and defaults must not quietly drift from the template.
	template, err := os.ReadFile("../../gctrm.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(template), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[extensions]") {
			break
		}
		if key, value, ok := strings.Cut(line, " = "); ok && key != "version" {
			if value != "true" || !seen[key] {
				t.Fatalf("uncovered/non-default fix %s=%s", key, value)
			}
		}
	}
	if len(seen) != reflect.TypeOf(fixes.Policy{}).NumField()+1 {
		t.Fatal("missing behavioral probes")
	}
	configured, err := decodeConfig(template)
	defaults, defaultErr := decodeConfig(nil)
	configuredRules, _ := dialect.Legacy.Resolve(configured.compatibility)
	defaultRules, _ := dialect.Legacy.Resolve(defaults.compatibility)
	// Explicit reference choices and inherited reference choices resolve alike.
	defaults.compatibility = configured.compatibility
	if err != nil || defaultErr != nil || configuredRules != defaultRules || !reflect.DeepEqual(configured, defaults) {
		t.Fatal("template and omitted-config defaults differ", err, defaultErr)
	}
}

func TestIndividualFixIsolationAndINI(t *testing.T) {
	defaults, err := decodeConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.fixes != fixes.All() || defaults.allowNonConsoleInstructions {
		t.Fatal("incorrect defaults")
	}
	for _, key := range strings.Fields("lha eqv console_only") {
		f, err := decodeConfig([]byte("[bug_fixes]\n" + key + "=false"))
		if err != nil {
			t.Fatal(err)
		}
		want := defaults
		if key == "console_only" {
			want.allowNonConsoleInstructions = true
		} else if err := want.fixes.Set(key, false); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f, want) {
			t.Fatalf("%s changed unrelated settings", key)
		}
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[bug_fixes]\nlha=false\neqv=false")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "one.asm : --set=bug_fixes.eqv=true --set=bug_fixes.lha=true")
	jobs, _, err := plan([]string{"--set=bug_fixes.lha=false", "one.asm", "two.asm"}, exe)
	if err != nil || len(jobs) != 2 {
		t.Fatal(err, jobs)
	}
	for _, job := range jobs {
		if job.flags.fixes.LHA || !job.flags.fixes.EQV || !job.flags.fixes.OperandRanges || job.flags.allowNonConsoleInstructions {
			t.Fatal(job.flags)
		}
	}
	for _, bad := range []string{"bug_fixes=true", "[extensions]\nnon_console_instructions=false", "[bug_fixes]\nunknown=true", "[bug_fixes]\nlha='false'", "[bug_fixes]\nlha=true\nlha=false"} {
		if _, err := decodeConfig([]byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	jobs, _, err = plan([]string{"--bug-fixes=false", "--set=bug_fixes.lha=true", "--set=bug_fixes.console_only=true", "one.asm"}, "")
	if err != nil || !jobs[0].flags.fixes.LHA || jobs[0].flags.fixes.EQV || jobs[0].flags.allowNonConsoleInstructions {
		t.Fatal(err, jobs)
	}
}
