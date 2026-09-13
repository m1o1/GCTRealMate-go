package ppc

// Instructions sharing a field layout share one encoder. Entries contain only
// ISA data, so adding an ordinary instruction does not require parser changes.
type layout uint8

const (
	none      layout = iota
	rrr              // RT, RA, RB
	rr0              // RT, RA
	r0r              // RT, RB
	logical          // RA, RS, RB
	logical2         // RA, RS
	duplicate        // RA, RS (RB = RS)
	immediate        // RT, RA, SI
	negImmediate
	loadImmediate     // RT, SI
	unsignedImmediate // RA, RS, UI
	memory            // RT, D(RA)
	rotate            // RA, RS, SH/RB, MB, ME
	floatingMultiply  // FRT, FRA, FRC
	floatingFour      // FRT, FRA, FRC, FRB
	compareFloat      // BF, FRA, FRB
	cache             // RA, RB
	onlyRT
	crThree
	crTwo
	crOne
	swapped // RT, RB, RA
	onlyRB
	crFields       // BF, BFA
	crField        // BF
	segmentRead    // RT, SR
	segmentWrite   // SR, RS
	fpscrMask      // FM, FRB
	fpscrImmediate // BF, U
	fpscrBit       // BT
)

type spec struct {
	base             uint32
	form             layout
	record, overflow bool
}

var operandCounts = [...]int{none: 0, rrr: 3, rr0: 2, r0r: 2, logical: 3, logical2: 2, duplicate: 2, immediate: 3, negImmediate: 3, loadImmediate: 2, unsignedImmediate: 3, memory: 3, rotate: 5, floatingMultiply: 3, floatingFour: 4, compareFloat: 3, cache: 2, onlyRT: 1, crThree: 3, crTwo: 2, crOne: 1, swapped: 3, onlyRB: 1, crFields: 2, crField: 1, segmentRead: 2, segmentWrite: 2, fpscrMask: 2, fpscrImmediate: 2, fpscrBit: 1}

var instructions = makeInstructions()

func makeInstructions() map[string]spec {
	m := make(map[string]spec)
	add := func(name string, op, xo uint32, form layout, record, overflow bool) {
		m[name] = spec{op<<26 | xo<<1, form, record, overflow}
	}
	group := func(entries map[string]uint32, op uint32, form layout, record, overflow bool) {
		for n, x := range entries {
			add(n, op, x, form, record, overflow)
		}
	}
	for n, op := range map[string]uint32{"lbz": 34, "lbzu": 35, "lhz": 40, "lhzu": 41, "lha": 42, "lhau": 43, "lwz": 32, "lwzu": 33, "lfs": 48, "lfsu": 49, "lfd": 50, "lfdu": 51, "lmw": 46, "stb": 38, "stbu": 39, "sth": 44, "sthu": 45, "stw": 36, "stwu": 37, "stfs": 52, "stfsu": 53, "stfd": 54, "stfdu": 55, "stmw": 47} {
		add(n, op, 0, memory, false, false)
	}
	group(map[string]uint32{"lbzx": 87, "lbzux": 119, "lhzx": 279, "lhzux": 311, "lhax": 343, "lhaux": 375, "lwzx": 23, "lwzux": 55, "lfsx": 535, "lfsux": 567, "lfdx": 599, "lfdux": 631, "lhbrx": 790, "lwbrx": 534, "lswi": 597, "lswx": 533, "stbx": 215, "stbux": 247, "sthx": 407, "sthux": 439, "stwx": 151, "stwux": 183, "stfsx": 663, "stfsux": 695, "stfdx": 727, "stfdux": 759, "sthbrx": 918, "stwbrx": 662, "stswi": 725, "stswx": 661, "stfiwx": 983, "lwarx": 20}, 31, rrr, false, false)
	add("stwcx.", 31, 150, rrr, false, false)
	t := m["stwcx."]
	t.base |= 1
	m["stwcx."] = t
	for n, op := range map[string]uint32{"addi": 14, "addis": 15, "addic": 12, "addic.": 13, "subfic": 8, "mulli": 7, "twi": 3} {
		add(n, op, 0, immediate, false, false)
	}
	for n, op := range map[string]uint32{"subi": 14, "subis": 15, "subic": 12, "subic.": 13} {
		add(n, op, 0, negImmediate, false, false)
	}
	add("li", 14, 0, loadImmediate, false, false)
	add("lis", 15, 0, loadImmediate, false, false)
	for n, op := range map[string]uint32{"ori": 24, "oris": 25, "xori": 26, "xoris": 27, "andi.": 28, "andis.": 29} {
		add(n, op, 0, unsignedImmediate, false, false)
	}
	group(map[string]uint32{"add": 266, "addc": 10, "adde": 138, "subf": 40, "subfc": 8, "subfe": 136, "mullw": 235, "divw": 491, "divwu": 459}, 31, rrr, true, true)
	group(map[string]uint32{"sub": 40, "subc": 8, "sube": 136}, 31, swapped, true, true)
	group(map[string]uint32{"addme": 234, "addze": 202, "subfme": 232, "subfze": 200, "neg": 104}, 31, rr0, true, true)
	group(map[string]uint32{"mulhw": 75, "mulhwu": 11}, 31, rrr, true, false)
	group(map[string]uint32{"and": 28, "andc": 60, "nand": 476, "xor": 316, "or": 444, "orc": 412, "nor": 124, "eqv": 284, "slw": 24, "srw": 536, "sraw": 792, "srawi": 824}, 31, logical, true, false)
	group(map[string]uint32{"mr": 444, "not": 124}, 31, duplicate, true, false)
	group(map[string]uint32{"extsb": 954, "extsh": 922, "cntlzw": 26}, 31, logical2, true, false)
	for n, op := range map[string]uint32{"rlwinm": 21, "rlwimi": 20, "rlwnm": 23} {
		add(n, op, 0, rotate, true, false)
	}
	group(map[string]uint32{"crand": 257, "cror": 449, "crxor": 193, "crnand": 225, "crnor": 33, "creqv": 289, "crandc": 129, "crorc": 417}, 19, crThree, false, false)
	group(map[string]uint32{"crmove": 449, "crnot": 33}, 19, crTwo, false, false)
	group(map[string]uint32{"crclr": 193, "crset": 289}, 19, crOne, false, false)
	group(map[string]uint32{"icbi": 982, "dcbf": 86, "dcbi": 470, "dcbst": 54, "dcbt": 278, "dcbtst": 246, "dcbz": 1014}, 31, cache, false, false)
	for n, b := range map[string]uint32{"nop": 0x60000000, "isync": 0x4c00012c, "sync": 0x7c0004ac, "eieio": 0x7c0006ac, "rfi": 0x4c000064, "sc": 0x44000002} {
		m[n] = spec{base: b}
	}
	add("mfcr", 31, 19, onlyRT, false, false)
	add("mffs", 63, 583, onlyRT, true, false)
	group(map[string]uint32{"tw": 4}, 31, rrr, false, false)
	group(map[string]uint32{"fmr": 72, "fneg": 40, "fabs": 264, "fnabs": 136, "frsp": 12, "fctiwz": 15, "fctiw": 14}, 63, r0r, true, false)
	group(map[string]uint32{"fcmpo": 32, "fcmpu": 0}, 63, compareFloat, false, false)
	for _, op := range []uint32{63, 59} {
		suffix := ""
		if op == 59 {
			suffix = "s"
		}
		for n, x := range map[string]uint32{"fadd": 21, "fsub": 20, "fdiv": 18} {
			add(n+suffix, op, x, rrr, true, false)
		}
		add("fmul"+suffix, op, 25, floatingMultiply, true, false)

		for n, x := range map[string]uint32{"fmadd": 29, "fmsub": 28, "fnmadd": 31, "fnmsub": 30} {
			add(n+suffix, op, x, floatingFour, true, false)
		}
	}
	group(map[string]uint32{"ps_abs": 264, "ps_nabs": 136, "ps_mr": 72, "ps_neg": 40, "ps_res": 24, "ps_rsqrte": 26}, 4, r0r, true, false)
	group(map[string]uint32{"ps_add": 21, "ps_sub": 20, "ps_div": 18, "ps_merge00": 528, "ps_merge01": 560, "ps_merge10": 592, "ps_merge11": 624}, 4, rrr, true, false)
	group(map[string]uint32{"ps_mul": 25, "ps_muls0": 12, "ps_muls1": 13}, 4, floatingMultiply, true, false)
	group(map[string]uint32{"ps_madds0": 14, "ps_madds1": 15, "ps_madd": 29, "ps_msub": 28, "ps_nmadd": 31, "ps_nmsub": 30, "ps_sum0": 10, "ps_sum1": 11, "ps_sel": 23}, 4, floatingFour, true, false)
	group(map[string]uint32{"ps_cmpo0": 32, "ps_cmpo1": 96, "ps_cmpu0": 0, "ps_cmpu1": 64}, 4, compareFloat, false, false)
	// fres has only a single-precision opcode despite having no trailing suffix.
	add("fres", 59, 24, r0r, true, false)
	add("frsqrte", 63, 26, r0r, true, false)
	add("fsel", 63, 23, floatingFour, true, false)
	add("dcbz_l", 4, 1014, cache, false, false)
	group(map[string]uint32{"eciwx": 310, "ecowx": 438}, 31, rrr, false, false)
	add("mcrf", 19, 0, crFields, false, false)
	add("mcrfs", 63, 64, crFields, false, false)
	add("mcrxr", 31, 512, crField, false, false)
	group(map[string]uint32{"mfmsr": 83, "mtmsr": 146}, 31, onlyRT, false, false)
	add("mfsr", 31, 595, segmentRead, false, false)
	add("mtsr", 31, 210, segmentWrite, false, false)
	group(map[string]uint32{"mfsrin": 659, "mtsrin": 242}, 31, r0r, false, false)
	group(map[string]uint32{"mtfsb0": 70, "mtfsb1": 38}, 63, fpscrBit, true, false)
	add("mtfsf", 63, 711, fpscrMask, true, false)
	add("mtfsfi", 63, 134, fpscrImmediate, true, false)
	add("tlbie", 31, 306, onlyRB, false, false)
	add("tlbsync", 31, 566, none, false, false)
	return m
}
