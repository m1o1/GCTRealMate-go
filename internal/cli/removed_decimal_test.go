package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDecimalLeadingZerosOptionRemoved(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		if _, err := decodeConfig([]byte("[semantics]\ndecimal_leading_zeros=" + value)); err == nil || !strings.Contains(err.Error(), "unknown configuration key") {
			t.Fatal("removed TOML key accepted", err)
		}
		option := "--set=semantics.decimal_leading_zeros=" + value
		if _, _, err := plan([]string{"--no-config", option, "probe.asm"}, ""); err == nil {
			t.Fatal("removed CLI key accepted")
		}
		dir := t.TempDir()
		exe := filepath.Join(dir, "gctrm.exe")
		writeTestFile(t, filepath.Join(dir, "gctrm.ini"), "probe.asm : "+option)
		if _, _, err := plan([]string{"probe.asm"}, exe); err == nil {
			t.Fatal("removed INI key accepted")
		}
	}
}
