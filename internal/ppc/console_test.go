package ppc

import (
	"fmt"
	"strings"
	"testing"

	"gctrm/fixes"
	"gctrm/internal/dialect"
)

func TestConsoleRejectsNonConsoleInstructions(t *testing.T) {
	// Broadway manual, defined/illegal instruction classes and instruction
	// tables; dcba is only an unimplemented placeholder in Dolphin's table.
	for _, name := range strings.Fields("ld ldu lwa std stdu ldx ldux lwax lwaux stdx stdux td tdi mulld divd divdu mulhd mulhdu sld srd srad extsw cntlzd fctid fctidz fcfid fsqrt fsqrts frsqrtes fsels cmpd cmpdi cmpld cmpldi dcba") {
		for _, suffix := range []string{"", ".", "o", "o."} {
			if _, err := Encode(name+suffix+" 3,4,5", Context{Fixes: fixes.FromBool(true)}); err == nil {
				t.Errorf("accepted %s%s", name, suffix)
			}
		}
	}
}

func TestConsoleOperandRestrictions(t *testing.T) {
	for _, source := range []string{
		"cmp cr0,1,r3,r4", "cmpi 7,1,3,4", "cmpl cr0,1,r3,r4", "cmpli 0,1,3,4",
		"mcrf cr8,cr0", "mcrfs cr0,cr8", "mcrxr cr8", "mtfsfi cr0,16", "mtfsfi cr8,0",
		"mtfsf 256,f1", "mtfsb0 32", "mtfsb1 -1", "mfsr r3,16", "mtsr -1,r3",
		"mftb r3,267", "mftb r3,270", "mftbu r3,269", "mtmsr r3,1", "tlbie r3,1",
		"addi. r3,r4,1", "mtmsr. r3", "mcrf. cr1,cr2", "dcbz_l. r0,r3",
		"li f3,1", "fadd r1,f2,f3", "lwz f1,0(r3)", "lfs r1,0(r3)", "lfs f1,0(f3)",
		"mfspr f3,lr", "mtspr lr,f3", "mtcrf cr1,r3", "mtfsf 1,r3", "mtfsfi cr0,r3",
		"srawi r3,r4,r5", "slwi r3,r4,r5", "rlwinm r3,r4,r5,0,31", "lswi r3,r4,r5",
		"bc r12,2,4", "bcctr 16,0", "bcctrl 18,0", "bdnzctr", "beq++ 4", "beq+- 4",
		"psq_l f0,0(r3),r0,0", "psq_l f0,0(r3),0,cr0", "psq_l f0,-2049(r3),0,0",
		"psq_l f0,4096(r3),0,0", "psq_l f0,0(r3),0,8", "psq_l. f0,0(r3),0,0",
		"lmw r0,0(r0)", "lmw r3,0(r3)", "lmw r3,0(r31)",
		"lswi r31,r0,8", "lswi r3,r4,0", "lswx r3,r3,r4", "lswx r3,r4,r3",
		"crclr 6,6", "mtfsf 1,f3,0,1",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Encode(source, Context{Fixes: fixes.FromBool(true), Dialect: dialect.Modern}); err == nil {
				t.Fatalf("accepted invalid console form %s", source)
			}
		})
	}
}

func TestConsoleMemoryUpdateRegisters(t *testing.T) {
	for _, name := range strings.Fields("lbzu lhzu lhau lwzu lfsu lfdu stbu sthu stwu stfsu stfdu lbzux lhzux lhaux lwzux lfsux lfdux stbux sthux stwux stfsux stfdux psq_lu psq_lux psq_stu psq_stux") {
		for _, base := range []int{0, 3, 4, 31} {
			reg := "r3"
			if strings.HasPrefix(name, "lf") || strings.HasPrefix(name, "stf") || strings.HasPrefix(name, "psq_") {
				reg = "f3"
			}
			source := fmt.Sprintf("%s %s,0(r%d)", name, reg, base)
			if strings.HasSuffix(name, "x") {
				source = fmt.Sprintf("%s %s,r%d,r5", name, reg, base)
			}
			if strings.HasPrefix(name, "psq_") {
				source += ",0,0"
			}
			_, err := Encode(source, Context{Fixes: fixes.FromBool(true)})
			invalid := base == 0 || base == 3 && strings.HasPrefix(name, "l") && reg == "r3"
			if (err != nil) != invalid {
				t.Errorf("%s: error %v, invalid=%v", source, err, invalid)
			}
		}
	}
}

func TestConsoleValidEdgeCases(t *testing.T) {
	for _, source := range []string{
		"lmw r3,0(r0)", "lmw r31,0(r30)", "lswi r31,r2,8", "lswi r3,r2,0",
		"stwu r1,-16(r1)", "lfsu f3,0(r3)", "psq_lu f3,0(r3),1,7",
		"mfsr r3,sr15", "mtsr sr15,r3", "mftb r3,tbl", "mftb r3,tbu", "mtfsf 255,f31",
		"mcrfs cr7,cr7", "mtfsfi. cr7,15", "mtfsb0. 31", "mtfsb1. 0",
		"li r3,1 + (2)", "lwz r3,(4+4)(r4)", "lwz r3,8(4)", "li,r3,1", "bc+ 0,2 0x4",
		// z bits in BO are explicitly ignored by these CPUs (manual table 12-6).
		"bc 31,31,0x4", "bcctr 31,31", "bclr 31,31", "cmp cr7,0,r31,r0",
	} {
		if _, err := Encode(source, Context{Fixes: fixes.FromBool(true), ExpressionSyntax: true, AdditionalConsoleInstructions: true}); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
	got, err := Encode("beq crouchCheck", Context{Fixes: fixes.FromBool(true), RelativeLabel: func(s string) (int64, bool) { return 4, s == "crouchCheck" }})
	if err != nil || got != 0x41820004 {
		t.Fatalf("label starting with cr: %08x %v", got, err)
	}
}
