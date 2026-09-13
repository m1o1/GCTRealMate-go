package assembler

import (
	"fmt"
	"testing"
)

// This small decoder follows Gecko OS codehandleronly.s: _readcodes masks BA
// with 0xfe000000, pointer addressing uses all of PO, then _write and _hook1
// add the 25-bit code address. It is an independent address check, not a full
// interpreter or a substitute for executing the handler on console hardware.
func geckoTargets(t *testing.T, words []uint32) []uint32 {
	t.Helper()
	ba, po := uint32(0x80000000), uint32(0x80000000)
	var targets []uint32
	for i := 0; i < len(words); {
		a, b := words[i], words[i+1]
		i += 2
		switch a {
		case 0x42000000:
			ba = b
			continue
		case 0x4a000000:
			po = b
			continue
		case 0xe0000000:
			ba, po = b&0xffff0000, b<<16
			continue
		}
		base := ba & 0xfe000000
		if a&0x10000000 != 0 {
			base = po
		}
		op := (a >> 24) & 0xee
		switch op {
		case 0, 2, 4, 6, 0xc2:
			targets = append(targets, base+(a&0x01ffffff))
		default:
			t.Fatalf("address decoder does not implement %08x", a)
		}
		if op == 6 {
			i += int((b+7)/8) * 2
		}
		if op == 0xc2 {
			i += int(b) * 2
		}
		if i > len(words) {
			t.Fatal("truncated Gecko payload")
		}
	}
	return targets
}

func TestGeckoEffectiveAddresses(t *testing.T) {
	for _, address := range []uint32{0x80001000, 0x81001234, 0x817ffffc, 0x90001000, 0x91fffffc, 0x92001234, 0x93fffffc} {
		for _, statement := range []string{
			"byte 0xab @ $%08x", "half 0xabcd @ $%08x", "word 0xabcdef01 @ $%08x",
			"op nop @ $%08x", "byte[5] 1,2,3,4,5 @ $%08x",
			"CODE @ $%08x\n{\nli r3,1\nblr\n}", "HOOK @ $%08x\n{\nnop\n}",
		} {
			s := fmt.Sprintf(statement, address)
			t.Run(s, func(t *testing.T) {
				r := assemble(t, "Effective addresses\n"+s)
				got := geckoTargets(t, r.Codes[0].Words)
				if len(got) != 1 || got[0] != address {
					t.Fatalf("target %08x, want %08x", got, address)
				}
			})
		}
	}
}

func TestGeckoFullWidthValues(t *testing.T) {
	for _, tc := range []struct {
		source      string
		code, value uint32
	}{
		{".BA = $80000000", 0x42000000, 0x80000000},
		{".PO = $90000000", 0x4a000000, 0x90000000},
		{".BA <- $80001000", 0x40000000, 0x80001000},
		{".BA -> $80001000", 0x44000000, 0x80001000},
		{".GR3 = $ffffffff", 0x80000003, 0xffffffff},
		{".GR3 += $80000000", 0x80100003, 0x80000000},
		{".GR3 ^= ffffffff", 0x86400003, 0xffffffff},
		{".GR3 <-(32) $90001000", 0x82200003, 0x90001000},
		{".GR3 ->(32) $90001000", 0x84200003, 0x90001000},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got := assemble(t, "Values\n"+tc.source).Codes[0].Words
			if len(got) != 2 || got[0] != tc.code || got[1] != tc.value {
				t.Fatalf("got %08x, want %08x %08x", got, tc.code, tc.value)
			}
		})
	}
}

// BrawlCrate's Parameter.cs defines memory IC/LA/RA=0/1/2 and
// type Basic/Float/Bit=0/1/2, encoded at bits 28 and 24 respectively.
func TestPSATagsAgainstBrawlCrate(t *testing.T) {
	for mem, bank := range []string{"IC", "LA", "RA"} {
		for kind, name := range []string{"basic", "float", "bit"} {
			for _, index := range []uint32{0, 3, 0xffffff} {
				typed := fmt.Sprintf("%s_%s %d", bank, name, index)
				want := uint32(mem)<<28 | uint32(kind)<<24 | index
				for _, block := range []bool{false, true} {
					s := typed + " @ $80001000"
					wordIndex := 1
					if block {
						s = "CODE @ $80001000\n{\n" + typed + "\n}"
						wordIndex = 2
					}
					got := assemble(t, "PSA\n"+s).Codes[0].Words[wordIndex]
					if got != want {
						t.Errorf("%s block=%v: got %08x want %08x", typed, block, got, want)
					}
				}
			}
		}
	}
}
