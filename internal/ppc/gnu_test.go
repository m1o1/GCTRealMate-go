package ppc

import (
	"debug/elf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gctrm/fixes"
	"gctrm/internal/dialect"
)

// TestGNUCorpus uses words produced by GNU as, independently of GCTRealMate.
// The fixture contains only forms accepted in BOTH -mgekko and -mbroadway modes.
// Enable numeric branch expressions explicitly to accept GNU decimal targets.
func TestGNUCorpus(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gnu.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenInstruction
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	failures := 0
	for _, tc := range cases {
		got, err := Encode(tc.Assembly, Context{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, BranchExpressions: true, Dialect: dialect.Modern})
		if got != tc.Word || err != nil {
			if failures < 20 {
				t.Errorf("%s: got %08x (%v), GNU %08x", tc.Assembly, got, err, tc.Word)
			}
			failures++
		}
		legacy, legacyErr := Encode(tc.Assembly, Context{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true, BranchExpressions: true, Dialect: dialect.Legacy})
		// GNU's hint convention differs from the legacy dialect. On backward
		// conditional branches compare every bit except BO's prediction bit;
		// the legacy prediction bit itself has independent C++ capture tests.
		mask := uint32(0xffffffff)
		if tc.Word>>26 == 16 && tc.Word&0x8000 != 0 {
			mask &^= 1 << 21
		}
		if legacyErr != nil || legacy&mask != tc.Word&mask {
			t.Errorf("legacy %s: %08x (%v), GNU %08x mask %08x", tc.Assembly, legacy, legacyErr, tc.Word, mask)
		}
	}
	t.Logf("%d GNU instruction vectors, %d mismatches", len(cases), failures)
}

type gnuCase struct{ source, gas string }

// Unlike gnuCandidates (which varies our supported layouts), this inventory
// comes from an independent decoder and catches entirely missing mnemonics.
func TestConsoleInstructionInventory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "dolphin-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Instructions []string `json:"instructions"`
		Unsupported  []string `json:"unsupported"`
	}
	if err = json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join("testdata", "gnu.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus []goldenInstruction
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, tc := range corpus {
		present[strings.Fields(tc.Assembly)[0]] = true
	}
	for _, name := range inventory.Instructions {
		if slices.Contains(inventory.Unsupported, name) {
			if _, err := Encode(name+" r3,r4", Context{Fixes: fixes.FromBool(true)}); err == nil {
				t.Errorf("accepted unsupported inventory entry %s", name)
			}
			continue
		}
		if !present[name] {
			t.Errorf("console instruction %s has no independent encoding coverage", name)
		}
	}
	t.Logf("%d independently listed names: %d covered, %d explicitly unsupported", len(inventory.Instructions), len(inventory.Instructions)-len(inventory.Unsupported), len(inventory.Unsupported))
}

// TestRefreshGNUCorpus is an opt-in integration test. GNU as, not our encoder,
// computes every expected word. The second target must produce identical bytes.
func TestRefreshGNUCorpus(t *testing.T) {
	exe := os.Getenv("GCTRM_GNU_AS")
	if exe == "" || os.Getenv("GCTRM_UPDATE_GNU") != "1" {
		t.Skip("set GCTRM_GNU_AS and GCTRM_UPDATE_GNU=1 to refresh independent fixtures")
	}
	cases := gnuCandidates()
	gekko, rejectedGC := assembleGNU(t, exe, "gekko", cases)
	broadway, rejectedWii := assembleGNU(t, exe, "broadway", cases)
	var fixture []goldenInstruction
	for _, c := range cases {
		gc, okGC := gekko[c.source]
		wii, okWii := broadway[c.source]
		if okGC != okWii || okGC && gc != wii {
			t.Fatalf("target disagreement for %s: Gekko %08x/%v, Broadway %08x/%v", c.source, gc, okGC, wii, okWii)
		}
		if okGC {
			fixture = append(fixture, goldenInstruction{c.source, gc})
		}
	}
	writeJSON := func(name string, value any) {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join("testdata", name), append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON("gnu.json", fixture)
	writeJSON("gnu-rejected.json", map[string]map[string]string{"gekko": rejectedGC, "broadway": rejectedWii})
	t.Logf("%d candidate instructions; %d accepted by both targets, %d rejected", len(cases), len(fixture), len(rejectedGC))
}

func assembleGNU(t *testing.T, exe, target string, cases []gnuCase) (map[string]uint32, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	source, object := filepath.Join(dir, "source.s"), filepath.Join(dir, "source.o")
	remaining := slices.Clone(cases)
	rejected := map[string]string{}
	errorLine := regexp.MustCompile(`source\.s:(\d+): Error: (.*)`)
	for pass := 0; pass < 5; pass++ {
		var text strings.Builder
		text.WriteString(".text\n")
		for _, c := range remaining {
			fmt.Fprintln(&text, c.gas)
		}
		if err := os.WriteFile(source, []byte(text.String()), 0600); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command(exe, "-m"+target, "-mregnames", "-mbig", "-a32", "-o", object, source).CombinedOutput()
		if err != nil {
			matches := errorLine.FindAllStringSubmatch(string(output), -1)
			if len(matches) == 0 {
				t.Fatalf("GNU as failed: %v\n%s", err, output)
			}
			bad := map[int]bool{}
			for _, m := range matches {
				line, _ := strconv.Atoi(m[1])
				index := line - 2
				if index < 0 || index >= len(remaining) {
					t.Fatalf("invalid GNU diagnostic line: %s", m[0])
				}
				bad[index] = true
				rejected[remaining[index].source] = m[2]
			}
			var next []gnuCase
			for i, c := range remaining {
				if !bad[i] {
					next = append(next, c)
				}
			}
			remaining = next
			continue
		}
		f, err := elf.Open(object)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		for _, section := range f.Sections {
			if (section.Type == elf.SHT_REL || section.Type == elf.SHT_RELA) && section.Size != 0 {
				t.Fatalf("unresolved GNU relocations in %s", section.Name)
			}
		}
		data, err := f.Section(".text").Data()
		if err != nil || len(data) != len(remaining)*4 {
			t.Fatalf("GNU .text: %d bytes for %d words, %v", len(data), len(remaining), err)
		}
		words := make(map[string]uint32, len(remaining))
		for i, c := range remaining {
			words[c.source] = binary.BigEndian.Uint32(data[i*4:])
		}
		return words, rejected
	}
	t.Fatal("GNU assembler still reports errors after filtering")
	return nil, nil
}

func gnuCandidates() []gnuCase {
	seen := map[string]bool{}
	var cases []gnuCase
	add := func(source, gas string) {
		if !seen[source] {
			cases = append(cases, gnuCase{source, gas})
			seen[source] = true
		}
	}
	// Exact forms found in Project+ that the C++ parser reads as zero.
	for _, s := range []string{"cmpw r5,0xD", "rlwinm r0,r0,0x3,0x0,0x1c"} {
		add(s, s)
	}
	for name, s := range instructions {
		for i := range 32 {
			a, b, c, d := i, (i+7)%32, (i+19)%32, (i+23)%32
			imm := []int{-32768, -1, 0, 1, 32767, 32768, 65535}[i%7]
			args := ""
			switch s.form {
			case rrr, crThree, logical, swapped:
				args = fmt.Sprintf("%d,%d,%d", a, b, c)
			case rr0, r0r, logical2, duplicate, crTwo:
				args = fmt.Sprintf("%d,%d", a, b)
			case crOne, onlyRT:
				args = fmt.Sprint(a)
			case immediate, negImmediate:
				args = fmt.Sprintf("%d,%d,%d", a, b, imm)
			case loadImmediate, unsignedImmediate:
				args = fmt.Sprintf("%d,%d", a, imm)
				if s.form == unsignedImmediate {
					args = fmt.Sprintf("%d,%d,%d", a, b, uint16(imm))
				}
			case memory:
				args = fmt.Sprintf("%d,%d(r%d)", a, imm, b)
			case rotate:
				args = fmt.Sprintf("%d,%d,%d,%d,%d", a, b, c, d, (i+13)%32)
			case floatingMultiply:
				args = fmt.Sprintf("%d,%d,%d", a, b, c)
			case floatingFour:
				args = fmt.Sprintf("%d,%d,%d,%d", a, b, c, d)
			case compareFloat:
				args = fmt.Sprintf("%d,%d,%d", i%8, b, c)
			case cache:
				args = fmt.Sprintf("%d,%d", a, b)
			case onlyRB:
				args = fmt.Sprint(a)
			case crFields:
				args = fmt.Sprintf("%d,%d", i%8, (i+3)%8)
			case crField:
				args = fmt.Sprint(i % 8)
			case segmentRead:
				args = fmt.Sprintf("%d,%d", a, i%16)
			case segmentWrite:
				args = fmt.Sprintf("%d,%d", i%16, a)
			case fpscrMask:
				args = fmt.Sprintf("%d,%d", i*8, a)
			case fpscrImmediate:
				args = fmt.Sprintf("%d,%d", i%8, i%16)
			case fpscrBit:
				args = fmt.Sprint(a)
			}
			names := []string{name}
			if s.record {
				names = append(names, name+".")
			}
			if s.overflow {
				names = append(names, name+"o", name+"o.")
			}
			for _, n := range names {
				text := strings.TrimSpace(n + " " + args)
				add(text, text)
			}
		}
	}
	for _, name := range []string{"slwi", "srwi", "clrlwi", "clrrwi", "rotlwi", "rotlw"} {
		for i := range 32 {
			for _, suffix := range []string{"", "."} {
				s := fmt.Sprintf("%s%s r%d,r%d,%d", name, suffix, i, (i+9)%32, i)
				add(s, s)
			}
		}
	}
	for _, name := range []string{"psq_l", "psq_lu", "psq_st", "psq_stu", "psq_lx", "psq_lux", "psq_stx", "psq_stux"} {
		for i := range 32 {
			base := (i + 3) % 32
			if base == 0 && (strings.HasSuffix(name, "u") || strings.HasSuffix(name, "ux")) {
				base = 1 // update forms forbid RA=0 even though GNU accepts it
			}
			for _, off := range []int{-2048, -1, 0, 1, 2047} {
				args := fmt.Sprintf("f%d,%d(r%d),%d,%d", i, off, base, i%2, (i/2)%8)
				if strings.HasSuffix(name, "x") {
					args = fmt.Sprintf("f%d,r%d,r%d,%d,%d", i, base, (i+11)%32, i%2, (i/2)%8)
				}
				add(name+" "+args, name+" "+args)
			}
		}
	}
	for _, n := range []string{"cmpw", "cmplw", "cmpwi", "cmplwi"} {
		for i := range 32 {
			s := fmt.Sprintf("%s cr%d,r%d,%d", n, i%8, i, (i+17)%32)
			add(s, s)
		}
	}
	for _, n := range []string{"cmp", "cmpl", "cmpi", "cmpli"} {
		for i := range 32 {
			s := fmt.Sprintf("%s %d,0,%d,%d", n, i%8, i, (i+17)%32)
			add(s, s)
		}
	}
	for _, n := range []string{"bc", "bcl", "bca", "bcla", "bclr", "bclrl", "bcctr", "bcctrl"} {
		for _, bo := range []int{4, 5, 12, 13, 16, 17, 18, 19, 20} {
			for _, bi := range []int{0, 2, 7, 16, 31} {
				s := fmt.Sprintf("%s %d,%d", n, bo, bi)
				gas := s
				if n == "bc" || n == "bcl" {
					s += ",-16"
					gas += ",.-16"
				}
				if n == "bca" || n == "bcla" {
					s += ",16"
					gas = s
				}
				add(s, gas)
			}
		}
	}
	for _, n := range []string{"b", "bl", "ba", "bla", "beq", "bne", "blt", "bge", "bgt", "ble", "bso", "bns", "bdnz", "bdz", "beql", "bnel", "beqa", "beqla"} {
		for _, off := range []int{-32768, -16, -4, 0, 4, 16, 32764} {
			for _, hint := range []string{"", "+", "-"} {
				if len(n) < 4 && strings.HasPrefix(n, "b") && (n == "b" || n == "bl" || n == "ba" || n == "bla") && hint != "" {
					continue
				}
				s, gas := fmt.Sprintf("%s%s %d", n, hint, off), fmt.Sprintf("%s%s .%+d", n, hint, off)
				if strings.HasSuffix(n, "a") {
					gas = s
				}
				add(s, gas)
			}
		}
	}
	for _, n := range []string{"blr", "blrl", "bctr", "bctrl", "beqlr", "bnelr", "beqctr", "bnectr", "bdnzlr", "bdzlr"} {
		for _, hint := range []string{"", "+", "-"} {
			add(n+hint, n+hint)
		}
	}
	for _, spr := range []int{0, 1, 8, 9, 18, 19, 22, 25, 26, 27, 272, 275, 528, 543, 912, 919, 1008, 1009, 1010, 1013, 1023} {
		for _, r := range []int{0, 7, 16, 31} {
			s := fmt.Sprintf("mfspr r%d,%d", r, spr)
			add(s, s)
			s = fmt.Sprintf("mtspr %d,r%d", spr, r)
			add(s, s)
		}
	}
	for i := range 32 {
		for _, n := range []string{"mftb", "mftbu", "mftbl", "mttbu", "mttbl"} {
			s := fmt.Sprintf("%s r%d", n, i)
			add(s, s)
		}
		for _, tbr := range []int{268, 269} {
			s := fmt.Sprintf("mftb r%d,%d", i, tbr)
			add(s, s)
		}
		s := fmt.Sprintf("mtcrf %d,r%d", i*8, i)
		add(s, s)
	}
	for bf := range 8 {
		for other := range 8 {
			for _, n := range []string{"mcrf", "mcrfs"} {
				s := fmt.Sprintf("%s cr%d,cr%d", n, bf, other)
				add(s, s)
			}
		}
		for value := range 16 {
			for _, suffix := range []string{"", "."} {
				s := fmt.Sprintf("mtfsfi%s cr%d,%d", suffix, bf, value)
				add(s, s)
			}
		}
	}
	for mask := range 256 {
		for _, reg := range []int{0, 15, 31} {
			for _, suffix := range []string{"", "."} {
				s := fmt.Sprintf("mtfsf%s %d,f%d", suffix, mask, reg)
				add(s, s)
			}
		}
	}
	slices.SortFunc(cases, func(a, b gnuCase) int { return strings.Compare(a.source, b.source) })
	return cases
}
