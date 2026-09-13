package ppc

import (
	"fmt"
	"strconv"
	"strings"
)

// These non-console entries existed in the C++ table. Target selection controls
// their availability independently of corrected versus legacy encoding.
var legacyInstructions = map[string]spec{
	"ldx":      {31<<26 | 21<<1, rrr, false, false},
	"ldux":     {31<<26 | 53<<1, rrr, false, false},
	"lwax":     {31<<26 | 341<<1, rrr, false, false},
	"lwaux":    {31<<26 | 373<<1, rrr, false, false},
	"stdx":     {31<<26 | 149<<1, rrr, false, false},
	"stdux":    {31<<26 | 181<<1, rrr, false, false},
	"mulld":    {31<<26 | 233<<1, rrr, true, true},
	"mulhdu":   {31<<26 | 9<<1, rrr, true, false},
	"mulhd":    {31<<26 | 73<<1, rrr, true, false},
	"divdu":    {31<<26 | 457<<1, rrr, true, true},
	"divd":     {31<<26 | 489<<1, rrr, true, true},
	"td":       {31<<26 | 68<<1, rrr, false, false},
	"tdi":      {2 << 26, immediate, false, false},
	"sld":      {31<<26 | 27<<1, logical, true, false},
	"srd":      {31<<26 | 539<<1, logical, true, false},
	"srad":     {31<<26 | 794<<1, logical, true, false},
	"extsw":    {31<<26 | 986<<1, logical2, true, false},
	"cntlzd":   {31<<26 | 58<<1, logical2, true, false},
	"fctidz":   {63<<26 | 815<<1, r0r, true, false},
	"fctid":    {63<<26 | 814<<1, r0r, true, false},
	"fcfid":    {63<<26 | 846<<1, r0r, true, false},
	"fsqrt":    {63<<26 | 22<<1, r0r, true, false},
	"fsqrts":   {59<<26 | 22<<1, r0r, true, false},
	"fress":    {59<<26 | 24<<1, r0r, true, false},
	"frsqrtes": {59<<26 | 26<<1, r0r, true, false},
	"fsels":    {59<<26 | 23<<1, floatingFour, true, false},
}

func lookupInstruction(name string, bugFixes bool) (s spec, ok, nonConsole bool) {
	// These historical spellings have no corrected instruction definition.
	if bugFixes && (name == "fress" || name == "fsels") {
		return spec{}, false, true
	}
	s, ok = instructions[name]
	if !ok {
		s, ok = legacyInstructions[name]
		nonConsole = ok
		if !ok && !bugFixes && strings.HasPrefix(name, "f") && !strings.HasSuffix(name, ".") {
			// The reference tests these arithmetic names as prefixes, then
			// selects precision from the suffix, even for misspellings.
			for _, prefix := range []string{"fadd", "fsub", "fmul", "fdiv", "fsqrt", "fres", "frsqrte", "fsel", "fmadd", "fmsub", "fnmadd", "fnmsub"} {
				if strings.HasPrefix(name, prefix) {
					s, ok = instructions[prefix]
					if !ok {
						s, ok = legacyInstructions[prefix]
						nonConsole = ok
					}
					if strings.HasSuffix(name, "s") || len(name) > 1 && name[len(name)-2] == 's' {
						if s.base>>26 != 59 {
							_, consoleSingle := instructions[prefix+"s"]
							nonConsole = nonConsole || !consoleSingle
						}
						s.base = s.base&0x03ffffff | 59<<26
					}
					break
				}
			}
		}
	}
	return s, ok, nonConsole
}

// legacyNumber models the reference's decimal stoi field conversion, including
// partial conversion of hexadecimal text. It never reads beyond the operand.
func legacyNumber(s string) (int64, error) {
	s = strings.TrimSpace(s)
	i := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, fmt.Errorf("invalid legacy numeric field %q", s)
	}
	v, err := strconv.ParseInt(s[:i], 10, 32)
	return v, err
}

// legacySpecial contains reference-only operand selection for comparisons.
func (e *encoder) legacySpecial(name string) (uint32, bool, error) {
	if name == "cmp" || name == "cmpl" || name == "cmpli" {
		if err := e.count(3); err != nil && !(e.ctx.Fixes.OperandCounts && len(e.args) == 4) {
			return 0, true, err
		}
		v := uint32(31<<26) + e.register(0, 'c')<<23 + e.register(1, 'g')<<16 + e.register(2, 'g')<<11
		if name != "cmp" {
			v += 64
		}
		return v, true, e.err
	}
	return 0, false, nil
}
