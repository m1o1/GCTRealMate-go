// Package fixes selects corrections to characterized C++ behavior.
package fixes

import "fmt"

// Policy is copied by value; each correction is independent. The zero value
// preserves characterized quirks. Target availability is configured separately.
type Policy struct {

	// Encode algebraic halfword loads instead of zero-extending loads.
	LHA bool

	// Use both supplied source registers instead of repeating the first.
	EQV bool

	// Encode CR AND-complement rather than CR AND.
	CRAndC bool

	// Encode CR OR-complement rather than CR OR.
	CROrC bool

	// Set OE/Rc bits instead of adding decimal 400/401.
	OverflowSuffix bool

	// Prevent a zero-distance srwi from carrying into the destination field.
	ShiftRightZero bool

	// Insert only the signed 12-bit quantized displacement.
	QuantizedDisplacement bool

	// Encode the existing indexed psq load/store forms and update selectors.
	IndexedQuantized bool

	// Honor supported paired-single record suffixes.
	PairedSingleRecord bool

	// Parse complete hexadecimal register and numeric fields instead of decimal prefixes.
	NumericFields bool

	// Correct generic comparison operand selection, cmpli routing, and comparison L
	// encoding.
	Comparisons bool

	// Set the explicit BO prediction bit without carrying into another BO bit.
	BranchPrediction bool

	// Flush pending raw bytes at the end of input.
	RawDataEOF bool

	// Preserve multiplication operators in outer-scanned source.
	ScannerMultiply bool

	// Preserve OR operators in aliases and GR directives.
	ScannerOR bool

	// Evaluate every alias term instead of losing later accumulator updates and partially
	// parsing numbers.
	AliasTerms bool

	// Preserve the selected GR index in load/store directives.
	GRIndex bool

	// Parse the supported BA/PO and GR qualifiers correctly.
	AddressQualifiers bool

	// Preserve the high bit of BA/PO/GR directive values.
	DirectiveBit31 bool

	// Split direct-write addresses into BA and the correct offset.
	MEM2Writes bool

	// Use BA and the correct offset for MEM2 hooks.
	MEM2Hooks bool

	// Preserve bank/type tags in direct PSA writes.
	PSATags bool

	// Emit the second word of .GOTO_F commands.
	GotoFalse bool

	// Insert Gecko label offsets into the low field and check their range/alignment.
	GeckoLabelOffsets bool

	// Emit the supported .ELSE and .ELSE_RESET commands.
	ElseDirectives bool

	// Diagnose unresolved PPC and Gecko labels instead of retaining zero displacements.
	MissingLabels bool

	// Require recognized instruction names rather than reference prefix/fallback acceptance.
	UnknownInstructions bool

	// Reject extra machine-instruction operands instead of ignoring them.
	OperandCounts bool

	// Check encoded register/numeric field widths and immediate ranges.
	OperandRanges bool

	// Reject unsupported record/overflow suffixes.
	SuffixValidation bool

	// Reject statically invalid memory-register combinations and CTR branch conditions.
	RegisterRelationships bool

	// Reject PPC branch displacements that are unaligned or out of range.
	BranchRanges bool

	// Require word-aligned PPC write/block addresses.
	AddressAlignment bool

	// Check that PSA variable indices fit 24 bits.
	PSAIndexRange bool

	// Reject sections with an incomplete eight-byte Gecko line.
	GeckoLineFraming bool

	// Encode DS-form offsets as aligned byte displacements rather than multiplying by four.
	DSDisplacement bool

	// Return failure status for missing input/include files.
	MissingFileStatus bool

	// Terminate a final odd word in text output before the section separator.
	TextLineTermination bool
}

// All enables every correction.
func All() Policy { return FromBool(true) }

// FromBool builds a uniform policy, useful for reference comparisons.
func FromBool(enabled bool) Policy {
	return Policy{
		LHA:                   enabled,
		EQV:                   enabled,
		CRAndC:                enabled,
		CROrC:                 enabled,
		OverflowSuffix:        enabled,
		ShiftRightZero:        enabled,
		QuantizedDisplacement: enabled,
		IndexedQuantized:      enabled,
		PairedSingleRecord:    enabled,
		NumericFields:         enabled,
		Comparisons:           enabled,
		BranchPrediction:      enabled,
		RawDataEOF:            enabled,
		ScannerMultiply:       enabled,
		ScannerOR:             enabled,
		AliasTerms:            enabled,
		GRIndex:               enabled,
		AddressQualifiers:     enabled,
		DirectiveBit31:        enabled,
		MEM2Writes:            enabled,
		MEM2Hooks:             enabled,
		PSATags:               enabled,
		GotoFalse:             enabled,
		GeckoLabelOffsets:     enabled,
		ElseDirectives:        enabled,
		MissingLabels:         enabled,
		UnknownInstructions:   enabled,
		OperandCounts:         enabled,
		OperandRanges:         enabled,
		SuffixValidation:      enabled,
		RegisterRelationships: enabled,
		BranchRanges:          enabled,
		AddressAlignment:      enabled,
		PSAIndexRange:         enabled,
		GeckoLineFraming:      enabled,
		DSDisplacement:        enabled,
		MissingFileStatus:     enabled,
		TextLineTermination:   enabled,
	}
}

// Set changes one canonical correction key.
func (p *Policy) Set(key string, value bool) error {
	switch key {
	case "lha":
		p.LHA = value
	case "eqv":
		p.EQV = value
	case "crandc":
		p.CRAndC = value
	case "crorc":
		p.CROrC = value
	case "overflow_suffix":
		p.OverflowSuffix = value
	case "shift_right_zero":
		p.ShiftRightZero = value
	case "quantized_displacement":
		p.QuantizedDisplacement = value
	case "indexed_quantized":
		p.IndexedQuantized = value
	case "paired_single_record":
		p.PairedSingleRecord = value
	case "numeric_fields":
		p.NumericFields = value
	case "comparisons":
		p.Comparisons = value
	case "branch_prediction":
		p.BranchPrediction = value
	case "raw_data_eof":
		p.RawDataEOF = value
	case "scanner_multiply":
		p.ScannerMultiply = value
	case "scanner_or":
		p.ScannerOR = value
	case "alias_terms":
		p.AliasTerms = value
	case "gr_index":
		p.GRIndex = value
	case "address_qualifiers":
		p.AddressQualifiers = value
	case "directive_bit31":
		p.DirectiveBit31 = value
	case "mem2_writes":
		p.MEM2Writes = value
	case "mem2_hooks":
		p.MEM2Hooks = value
	case "psa_tags":
		p.PSATags = value
	case "goto_false":
		p.GotoFalse = value
	case "gecko_label_offsets":
		p.GeckoLabelOffsets = value
	case "else_directives":
		p.ElseDirectives = value
	case "missing_labels":
		p.MissingLabels = value
	case "unknown_instructions":
		p.UnknownInstructions = value
	case "operand_counts":
		p.OperandCounts = value
	case "operand_ranges":
		p.OperandRanges = value
	case "suffix_validation":
		p.SuffixValidation = value
	case "register_relationships":
		p.RegisterRelationships = value
	case "branch_ranges":
		p.BranchRanges = value
	case "address_alignment":
		p.AddressAlignment = value
	case "psa_index_range":
		p.PSAIndexRange = value
	case "gecko_line_framing":
		p.GeckoLineFraming = value
	case "ds_displacement":
		p.DSDisplacement = value
	case "missing_file_status":
		p.MissingFileStatus = value
	case "text_line_termination":
		p.TextLineTermination = value
	default:
		return fmt.Errorf("unknown configuration key bug_fixes.%s", key)
	}
	return nil
}
