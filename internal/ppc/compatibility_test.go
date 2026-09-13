package ppc

import (
	"testing"

	"gctrm/fixes"
	"gctrm/internal/dialect"
)

func TestExplicitHintDoesNotCarry(t *testing.T) {
	// GNU Binutils 2.37, -mgekko and -mbroadway, independently emitted
	// these words. See validation/gnu-hints.json for the capture and tool hash.
	for _, tc := range []struct {
		source string
		word   uint32
	}{
		{"bc+ 13,2,0x10", 0x41a20010},
		{"bc+ 13,2,-0x10", 0x41a2fff0},
	} {
		got, err := Encode(tc.source, Context{Fixes: fixes.FromBool(true), Dialect: dialect.Modern})
		if err != nil || got != tc.word {
			t.Fatalf("%s: %08x (%v), GNU %08x", tc.source, got, err, tc.word)
		}
	}
	for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
		for _, source := range []string{"bc+ 13,2,0x10", "bc+ 13,2,-0x10"} {
			word, err := Encode(source, Context{Fixes: fixes.FromBool(true), Dialect: mode})
			if err != nil || (word>>21)&30 != 12 {
				t.Fatalf("%s: hint changed other BO bits: %08x %v", source, word, err)
			}
		}
	}
}
