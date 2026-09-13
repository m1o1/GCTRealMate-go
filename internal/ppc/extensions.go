package ppc

import "strings"

// These console encoders were added beyond the reference's implemented
// repertoire. Correcting an existing mnemonic is controlled by BugFixes.
func addedConsoleInstruction(name string) bool {
	if strings.HasPrefix(name, "bso") || strings.HasPrefix(name, "bns") {
		return true
	}
	switch name {
	case "dcbf", "dcbi", "dcbst", "dcbt", "dcbtst", "dcbz", "eieio", "sync", "sc",
		"clrlwi", "clrrwi", "rotlwi", "lwarx", "stwcx":
		return true
	case "dcbz_l", "eciwx", "ecowx", "mcrf", "mcrfs", "mcrxr", "mfmsr",
		"mfsr", "mfsrin", "mftb", "mftbl", "mftbu", "mttbl", "mttbu",
		"mtfsb0", "mtfsb1", "mtfsf", "mtfsfi", "mtmsr", "mtsr", "mtsrin", "tlbie", "tlbsync":
		return true
	case "mfspr", "mtspr", "mflr", "mtlr", "mfctr", "mtctr", "mfxer", "mtxer", "mfcr", "mtcr", "mtcrf", "mffs":
		return false
	}
	if strings.HasPrefix(name, "mf") || strings.HasPrefix(name, "mt") {
		_, ok := specialRegister(name[2:])
		return ok
	}
	return false
}

func legacyAddedInstruction(text, name string, ctx Context) (uint32, error) {
	alias := ""
	switch name {
	case "lwarx":
		alias = "lwa"
	case "stwcx":
		alias = "stw"
	case "rotlwi":
		alias = "rotlw"
	}
	if strings.HasPrefix(name, "bso") || strings.HasPrefix(name, "bns") {
		alias = "b"
	}
	if alias != "" {
		rest := ""
		if i := strings.IndexAny(text, " \t,"); i >= 0 {
			rest = text[i:]
		}
		return Encode(alias+rest, ctx)
	}
	if strings.HasPrefix(name, "m") {
		return 31 << 26, nil
	}
	if strings.HasPrefix(name, "c") {
		return 19 << 26, nil
	}
	return 0xffffffff, nil
}
