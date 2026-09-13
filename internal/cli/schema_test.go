package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Only the current configuration interface is accepted. A rejected setting
// must fail before an existing output can be replaced, including in INI files.
func TestObsoleteConfigurationRejectedBeforeAssembly(t *testing.T) {
	configs := []string{"version=1", "dialect='legacy'", "dialect='modern'"}
	for _, key := range []string{
		"validation.console_only", "dot_op", "allow_non_console_instructions", "extensions.allow_non_console_instructions",
		"legacy.octal_literals", "legacy.left_to_right_expressions", "legacy.unsigned_32_bit_aliases",
		"legacy.branch_hints", "legacy.register_prefixes", "legacy.zero_extended_data",
		"legacy.float_nan", "legacy.double_nan", "legacy.ini_prefix_matching",
		"legacy.include_tree_logs", "legacy.native_line_endings",
	} {
		for _, value := range []bool{false, true} {
			configs = append(configs, fmt.Sprintf("%s=%t", key, value))
		}
	}
	options := []string{
		"--set=validation.console_only=false", "--set=validation.console_only=true",
		"--dialect=legacy", "--dialect=modern",
		"--dot-op=false", "--dot-op=true",
		"--branch-expressions=false", "--branch-expressions=true",
		"--allow-non-console-instructions=false", "--allow-non-console-instructions=true",
		"--ini-match=legacy", "--ini-match=exact",
		"--log-format=legacy", "--log-format=flat",
		"--line-endings=native", "--line-endings=lf",
	}
	check := func(t *testing.T, config, ini, option string) {
		t.Helper()
		dir := t.TempDir()
		exe, src := filepath.Join(dir, "gctrm.exe"), filepath.Join(dir, "probe.asm")
		output := filepath.Join(dir, "probe.GCT")
		writeTestFile(t, src, "Probe\nop nop @ $80001000")
		writeTestFile(t, output, "previous output")
		writeTestFile(t, filepath.Join(dir, "gctrm.toml"), config)
		writeTestFile(t, filepath.Join(dir, "gctrm.ini"), ini)
		args := []string{}
		if option != "" {
			args = append(args, option)
		}
		args = append(args, src)
		var out, diagnostic bytes.Buffer
		status := Run(context.Background(), args, exe, &out, &diagnostic)
		data, err := os.ReadFile(output)
		if status != 2 || diagnostic.Len() == 0 || err != nil || string(data) != "previous output" {
			t.Fatalf("status %d, error %v, diagnostic %q, output %q", status, err, diagnostic.String(), data)
		}
	}
	for _, config := range configs {
		t.Run("TOML/"+config, func(t *testing.T) { check(t, config, "", "") })
	}
	for _, option := range options {
		t.Run("CLI/"+option, func(t *testing.T) { check(t, "", "", option) })
		t.Run("INI/"+option, func(t *testing.T) { check(t, "", "probe.asm : "+option, "") })
	}
}
