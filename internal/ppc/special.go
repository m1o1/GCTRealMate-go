package ppc

import (
	"fmt"
	"strconv"
	"strings"
)

func specialRegister(s string) (uint32, bool) {
	s = strings.ToLower(s)
	known := map[string]uint32{"xer": 1, "lr": 8, "ctr": 9, "dsisr": 18, "dar": 19, "dec": 22, "sdr1": 25, "iabr": 1010, "dabr": 1013, "hid0": 1008, "hid1": 1009, "hid2": 920, "hid4": 1011, "wpar": 921, "dmau": 922, "dmal": 923, "pvr": 287, "tbl": 268, "tbu": 269, "ear": 282, "l2cr": 1017, "ictc": 1019}
	if n, ok := known[s]; ok {
		return n, true
	}
	for _, x := range []struct {
		prefix    string
		base, max uint32
	}{{"srr", 26, 1}, {"sprg", 272, 3}, {"gqr", 912, 7}, {"spr", 0, 1023}} {
		if strings.HasPrefix(s, x.prefix) {
			n, err := strconv.ParseUint(s[len(x.prefix):], 10, 32)
			return x.base + uint32(n), err == nil && uint32(n) <= x.max
		}
	}
	if (strings.HasPrefix(s, "ibat") || strings.HasPrefix(s, "dbat")) && len(s) == 6 && s[4] >= '0' && s[4] <= '3' && (s[5] == 'u' || s[5] == 'l') {
		n := uint32(528) + uint32(s[4]-'0')*2
		if s[0] == 'd' {
			n += 8
		}
		if s[5] == 'l' {
			n++
		}
		return n, true
	}
	n, err := strconv.ParseUint(s, 0, 10)
	return uint32(n), err == nil
}
func (e *encoder) move(name string) (uint32, error) {
	if name == "mtcr" {
		if err := e.count(1); err != nil {
			return 0, err
		}
		return 31<<26 | 144<<1 | 255<<12 | e.register(0, 'g')<<21, e.err
	}
	if name == "mtcrf" {
		if err := e.count(2); err != nil {
			return 0, err
		}
		return 31<<26 | 144<<1 | e.number(0, 8)<<12 | e.register(1, 'g')<<21, e.err
	}
	if name != "mtspr" && name != "mfspr" {
		return 0, fmt.Errorf("unknown instruction %q", name)
	}
	if err := e.count(2); err != nil {
		return 0, err
	}
	ri, si, xo := 0, 1, uint32(339)
	if name == "mtspr" {
		ri, si, xo = 1, 0, 467
	}
	spr, ok := specialRegister(e.args[si])
	if !ok {
		return 0, fmt.Errorf("invalid special register %q", e.args[si])
	}
	return 31<<26 | xo<<1 | e.register(ri, 'g')<<21 | (spr&31)<<16 | (spr>>5)<<11, e.err
}

func (e *encoder) timeBase(name string) (uint32, error) {
	tbr := uint32(268)
	if name == "mftbu" {
		tbr = 269
	}
	if name == "mftb" && len(e.args) == 2 {
		var ok bool
		tbr, ok = specialRegister(e.args[1])
		if !ok || (tbr != 268 && tbr != 269) {
			return 0, fmt.Errorf("mftb requires TBL (268) or TBU (269)")
		}
	} else if err := e.count(1); err != nil {
		return 0, err
	}
	return 31<<26 | 371<<1 | e.register(0, 'g')<<21 | (tbr&31)<<16 | (tbr>>5)<<11, e.err
}
func (e *encoder) quantized(name string) (uint32, error) {
	dot := strings.HasSuffix(name, ".")
	name = strings.TrimSuffix(name, ".")
	if dot {
		return 0, fmt.Errorf("quantized load/store has no record suffix")
	}
	indexed := strings.HasSuffix(name, "x")
	op, ok := map[string]uint32{"psq_l": 56, "psq_lu": 57, "psq_st": 60, "psq_stu": 61, "psq_lx": 6, "psq_lux": 38, "psq_stx": 7, "psq_stux": 39}[name]
	if !ok {
		return 0, fmt.Errorf("unknown instruction %q", name)
	}
	if err := e.count(5); err != nil {
		return 0, err
	}
	var v uint32
	if indexed && !e.ctx.Fixes.IndexedQuantized {
		return 0, fmt.Errorf("legacy GCTRealMate cannot encode indexed quantized instructions; enable bug_fixes.indexed_quantized")
	}
	if indexed {
		v = 4<<26 | op<<1 | e.register(0, 'f')<<21 | e.register(1, 'g')<<16 | e.register(2, 'g')<<11 | e.number(3, 1)<<10 | e.number(4, 3)<<7
	} else {
		v = op<<26 | e.register(0, 'f')<<21 | e.register(2, 'g')<<16 | e.number(3, 1)<<15 | e.number(4, 3)<<12 | e.immediate(e.value(1), 12)
	}
	if !e.ctx.Fixes.QuantizedDisplacement && !indexed {
		v = op<<26 + e.register(0, 'f')<<21 + e.register(2, 'g')<<16 + (e.number(3, 1)%2*8+e.number(4, 3)%8)<<12 + uint32(e.value(1))
	}
	e.validateMemory(name, v)
	return v, e.err
}
