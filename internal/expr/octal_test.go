package expr

import (
	"testing"

	"gctrm/internal/dialect"
)

func TestLeadingZeroIntegersAreAlwaysOctal(t *testing.T) {
	for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
		for source, want := range map[string]int64{
			"0": 0, "00": 0, "010": 8, "077": 63, "10": 10,
			"-010": -8, "+010": 8, "0x10": 16, "$10": 16, "0b10": 2,
		} {
			got, err := EvalMode(source, nil, mode)
			if err != nil || got != want {
				t.Fatalf("mode %d: %s = %d, %v; want %d", mode, source, got, err, want)
			}
		}
		for _, source := range []string{"08", "09", "018", "-08"} {
			if _, err := EvalMode(source, nil, mode); err == nil {
				t.Fatalf("mode %d accepted %s", mode, source)
			}
			if _, err := Alias(source, nil, mode); err == nil {
				t.Fatalf("mode %d accepted alias %s", mode, source)
			}
		}
		got, err := Alias("010", nil, mode)
		if err != nil || got != 8 {
			t.Fatal(mode, got, err)
		}
	}
}

func TestFieldRadixIsUnaffectedBySourceSyntax(t *testing.T) {
	// Register/numeric fields always used a separate decimal grammar, even when
	// ordinary integer literals were configurable. Removing that option does
	// not reinterpret existing register spellings.
	for source, want := range map[string]int64{"010": 10, "08": 8, "0x10": 16, "0b10": 2} {
		got, err := EvalField(source, nil)
		if err != nil || got != want {
			t.Fatal(source, got, err)
		}
	}
}
