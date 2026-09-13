package cli

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFloatingPointDataTOMLAndCLI(t *testing.T) {
	for _, configured := range []bool{false, true} {
		for _, override := range []string{"", "false", "true"} {
			for _, unknown := range []bool{false, true} {
				for _, alternative := range []bool{false, true} {
					t.Run(fmt.Sprintf("config=%t/override=%s/unknown=%t/nan=%t", configured, override, unknown, alternative), func(t *testing.T) {
						dir := t.TempDir()
						config, source := filepath.Join(dir, "settings.toml"), filepath.Join(dir, "probe.asm")
						writeTestFile(t, config, fmt.Sprintf("[extensions]\nfloating_point_data=%t\n[bug_fixes]\nunknown_instructions=%t\n[encoding]\nalternative_float_nan=%t\n", configured, unknown, alternative))
						writeTestFile(t, source, "Probe\nCODE @ $80001000\n{\nfloat NaN\n}")
						output := filepath.Join(dir, "probe.GCT")
						writeTestFile(t, output, "previous output")
						args := []string{"--config", config, "-i"}
						enabled := configured
						if override != "" {
							args = append(args, "--set=extensions.floating_point_data="+override)
							enabled = override == "true"
						}
						var out, diagnostic bytes.Buffer
						status := Run(context.Background(), append(args, source), "", &out, &diagnostic)
						data, err := os.ReadFile(output)
						if err != nil {
							t.Fatal(err)
						}
						if !enabled && unknown {
							if status != 1 || string(data) != "previous output" || !strings.Contains(diagnostic.String(), "extensions.floating_point_data=true") {
								t.Fatal(status, diagnostic.String(), string(data))
							}
							return
						}
						word := "fc000000"
						if enabled {
							word = "7fffffff"
							if alternative {
								word = "7fc00000"
							}
						}
						want := "00d0c0de00d0c0de0600100000000004" + word + "00000000f000000000000000"
						if status != 0 || hex.EncodeToString(data) != want {
							t.Fatalf("%d %s %x", status, diagnostic.String(), data)
						}
					})
				}
			}
		}
	}
}

func TestFloatingPointDataINIAndBulkFixes(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gctrm.exe")
	writeTestFile(t, filepath.Join(dir, "gctrm.toml"), "[extensions]\nfloating_point_data=false\n")
	writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "probe.asm : --set=extensions.floating_point_data=true\n")
	jobs, _, err := plan([]string{"--bug-fixes=false", "probe.asm"}, exe)
	if err != nil || len(jobs) != 1 || !jobs[0].flags.floatingPointData {
		t.Fatal(jobs, err)
	}
	jobs, _, err = plan([]string{"--set=extensions.floating_point_data=false", "probe.asm"}, exe)
	if err != nil || len(jobs) != 1 || jobs[0].flags.floatingPointData {
		t.Fatal(jobs, err)
	}
	for _, config := range []string{"[extensions]\nfloating_point_data=1", "[extensions]\nfloating_point_data='true'"} {
		if _, err := decodeConfig([]byte(config)); err == nil {
			t.Fatal("accepted", config)
		}
	}
	f, err := decodeConfig(nil)
	if err != nil || f.floatingPointData {
		t.Fatal("extension enabled by default", err)
	}
}
