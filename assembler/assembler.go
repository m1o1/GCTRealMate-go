// Package assembler compiles GCTRealMate source into Gecko GCT files.
// Each call owns its state and is safe to run concurrently with other calls.
// Parsing and encoding finish before callers receive any output.
package assembler

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gctrm/internal/dialect"
	"gctrm/internal/ppc"
)

// Dialect selects source semantics, independently of the BugFixes policy.
type Dialect = dialect.Mode

// Compatibility overrides individual preset choices; nil fields inherit.
type Compatibility = dialect.Overrides

const (
	Legacy Dialect = dialect.Legacy // Default: compatible v0.2.6 source semantics.
	Modern Dialect = dialect.Modern // Decimal literals, C precedence, GNU hints.
)

// Validation selects optional restrictions on reference-accepted source.
// Zero values preserve reference handling; machine encoding checks use BugFixes.
type Validation struct {
	RejectDuplicateLabels    bool
	StrictMacroCalls         bool
	RejectUndefinedMacros    bool
	RejectAddressAnnotations bool
	RejectDataOverflow       bool
}

// Options configure source loading and address-dependent branch resolution.
type Options struct {
	BugFixes                      bool  // Opt in to corrections; false preserves known C++ quirks.
	DotOp                         *bool // Accept .op as an alias for op; nil defaults to false.
	ExpressionSyntax              bool  // Additional expression forms beyond the reference grammar.
	ImplicitSections              bool  // Permit source without an initial section name.
	AdditionalConsoleInstructions bool  // Permit implemented console mnemonics absent from the reference.
	Validation                    Validation
	BranchExpressions             bool // Accept additional numeric branch target forms; default false.
	AllowNonConsoleInstructions   bool // Permit retained non-Gekko/Broadway forms; default false.
	Dialect                       Dialect
	Compatibility                 Compatibility
	BaseAddress                   *uint32 // Address of the GCT header in target memory.
	ConvertAbsolute               bool    // Convert ba/bla into relative b/bl instructions.
	RepairPathCase                bool
	ReadFile                      func(string) ([]byte, error) // Optional include loader; defaults to os.ReadFile.
	rules                         dialect.Rules
}

// Code is one named section and its encoded words, excluding the GCT envelope.
type Code struct {
	Name   string
	Offset uint32
	Words  []uint32
}

// Result contains the successfully assembled codeset.
type Result struct {
	Codes      []Code
	logEntries []logEntry
	bugFixes   bool
}

type logEntry struct {
	include string
	section int
	depth   int
}

// Bytes returns a new big-endian GCT with its standard header and terminator.
func (r *Result) Bytes() []byte {
	out := appendWords(nil, 0x00d0c0de, 0x00d0c0de)
	for _, c := range r.Codes {
		out = appendWords(out, c.Words...)
	}
	return appendWords(out, 0xf0000000, 0)
}

// Text formats named hexadecimal codes. convert adds the GCTconvert game ID.
func (r *Result) Text(asterisks, convert bool) string {
	var b strings.Builder
	if convert {
		b.WriteString("RSBE01\n\n")
	}
	for _, c := range r.Codes {
		b.WriteString(c.Name + "\n")
		for i, w := range c.Words {
			if i%2 == 0 {
				if asterisks {
					b.WriteString("* ")
				}
			} else {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%08X", w)
			if i%2 == 1 {
				b.WriteByte('\n')
			}
		}
		if r.bugFixes && len(c.Words)%2 != 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Log preserves include order and nesting, with code offsets from the GCT start.
// Library text formatters use LF; the CLI chooses file line endings.
func (r *Result) Log() string {
	if len(r.logEntries) == 0 {
		return r.FlatLog()
	}
	var b strings.Builder
	for _, entry := range r.logEntries {
		b.WriteString(strings.Repeat("\t", entry.depth))
		if entry.include != "" {
			fmt.Fprintln(&b, entry.include)
		} else {
			c := r.Codes[entry.section]
			fmt.Fprintf(&b, "%s @ Off 0x%x\n", c.Name, c.Offset)
		}
	}
	return b.String()
}

// FlatLog lists only code names and offsets, without include entries.
func (r *Result) FlatLog() string {
	var b strings.Builder
	for _, c := range r.Codes {
		fmt.Fprintf(&b, "%s @ Off 0x%x\n", c.Name, c.Offset)
	}
	return b.String()
}

// Compile loads a file and assembles it. It never writes output files.
func Compile(ctx context.Context, filename string, opts Options) (*Result, error) {
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	data, err := opts.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return Assemble(ctx, filename, data, opts)
}

// Assemble compiles supplied source. filename anchors diagnostics and includes.
func Assemble(ctx context.Context, filename string, source []byte, opts Options) (*Result, error) {
	rules, err := opts.Dialect.Resolve(opts.Compatibility)
	if err != nil {
		return nil, err
	}
	rules.BugFixes = opts.BugFixes
	rules.ExpressionSyntax = opts.ExpressionSyntax
	rules.RejectDataOverflow = opts.Validation.RejectDataOverflow
	opts.rules = rules
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(source) > 16<<20 {
		return nil, fmt.Errorf("source exceeds 16 MiB")
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	name, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	tokens, err := scanPolicy(name, source, opts.BugFixes)
	if err != nil {
		return nil, err
	}
	f := frontend{ctx: ctx, opts: opts, root: filepath.Dir(name), scope: newScope(nil), activeFiles: map[string]bool{name: true}}
	if err = f.parse(tokens, nil, nil, false, 0, 0); err != nil {
		return nil, err
	}
	result := &Result{logEntries: f.logEntries, bugFixes: opts.BugFixes}
	offset := uint32(8)
	for i, s := range f.sections {
		s.flushTail = i+1 < len(f.sections) || opts.BugFixes
		words, err := encodeSection(ctx, s, offset, opts)
		if err != nil {
			return nil, err
		}
		result.Codes = append(result.Codes, Code{s.name, offset, words})
		offset += uint32(len(words) * 4)
	}
	return result, nil
}

type fixup struct {
	index int
	name  string
	lines bool
	pos   Position
}

func encodeSection(ctx context.Context, s section, offset uint32, opts Options) ([]uint32, error) {
	var words []uint32
	var data []byte
	labels := map[string]int{}
	var fixes []fixup
	flush := func() {
		if len(data) == 0 {
			return
		}
		for len(data)%8 != 0 {
			data = append(data, 0)
		}
		words = append(words, bytesToWords(data)...)
		data = nil
	}
	for _, n := range s.nodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if n.kind == dataNode {
			b, _, err := encodeData(n, false, opts.rules)
			if err != nil {
				return nil, at(n.pos, err)
			}
			data = append(data, b...)
			continue
		}
		flush()
		switch n.kind {
		case labelNode:
			key := strings.ToLower(n.text)
			if _, ok := labels[key]; ok {
				if !opts.Validation.RejectDuplicateLabels {
					continue
				}
				return nil, at(n.pos, fmt.Errorf("duplicate label %q", n.text))
			}
			labels[key] = len(words)
		case rawNode:
			b, _ := hex.DecodeString(n.text)
			words = append(words, bytesToWords(b)...)
		case writeNode:
			w, err := encodeWrite(n, opts)
			if err != nil {
				return nil, at(n.pos, err)
			}
			words = append(words, w...)
		case blockNode:
			w, err := encodeBlock(ctx, n, offset+uint32(len(words)*4), opts)
			if err != nil {
				return nil, err
			}
			words = append(words, w...)
		case directiveNode:
			w, fix, err := encodeDirectivePolicy(n, opts.BugFixes)
			if err != nil {
				return nil, at(n.pos, err)
			}
			if fix != nil {
				fix.index = len(words)
				fix.pos = n.pos
				fixes = append(fixes, *fix)
			}
			words = append(words, w...)
		}
	}
	if s.flushTail {
		flush()
	}
	for _, f := range fixes {
		dest, ok := labels[strings.ToLower(f.name)]
		if !ok {
			if !opts.BugFixes {
				continue
			}
			return nil, at(f.pos, fmt.Errorf("undefined label %q", f.name))
		}
		delta := (dest-f.index)*4 - 8
		if f.lines {
			if opts.BugFixes && (dest-f.index)%2 != 0 {
				return nil, at(f.pos, fmt.Errorf("GOTO target is not 8-byte aligned"))
			}
			delta = int(float64(dest-f.index)/2 - 1)
		}
		if opts.BugFixes && (delta < -32768 || delta > 32767) {
			return nil, at(f.pos, fmt.Errorf("label displacement outside signed 16-bit range"))
		}
		if opts.BugFixes {
			words[f.index] = words[f.index]&0xffff0000 | uint32(uint16(delta))
		} else {
			words[f.index] = words[f.index]%0xffff0000 + uint32(int32(int16(delta)))
		}
	}
	if opts.BugFixes && len(words)%2 != 0 {
		return nil, at(s.pos, fmt.Errorf("Gecko section has an incomplete 8-byte line"))
	}
	return words, nil
}
func encodeWrite(n node, opts Options) ([]uint32, error) {
	name, rest := head(n.text)
	var data []byte
	var array bool
	var err error
	if name == "op" {
		if opts.BugFixes && n.address&3 != 0 {
			return nil, fmt.Errorf("instruction write is not word-aligned")
		}
		if kind, _ := head(rest); isDataType(kind) {
			literal := n
			literal.text = rest
			data, _, err = encodeData(literal, true, opts.rules)
			if err == nil && len(data) != 4 {
				err = fmt.Errorf("op data must occupy exactly one instruction word")
			}
		} else {
			w, e := ppc.Encode(rest, ppc.Context{BugFixes: opts.BugFixes, ExpressionSyntax: opts.ExpressionSyntax, AdditionalConsoleInstructions: opts.AdditionalConsoleInstructions, BranchExpressions: opts.BranchExpressions, AllowNonConsoleInstructions: opts.AllowNonConsoleInstructions, Dialect: opts.Dialect, Compatibility: opts.Compatibility, Address: &n.address, Lookup: lookupValues(n.values), ConvertAbsolute: opts.ConvertAbsolute})
			err = e
			data = appendWords(nil, w)
		}
	} else {
		data, array, err = encodeData(n, false, opts.rules)
	}
	if err != nil {
		return nil, err
	}
	kind := uint32(6)
	if !array {
		switch len(data) {
		case 1:
			kind = 0
		case 2:
			kind = 2
		case 4:
			kind = 4
		}
	}
	var words []uint32
	address := n.address
	if address >= 0x82000000 {
		base := uint32(0x42000000)
		if kind == 6 {
			base = 0x4a000000
			kind = 0x16
			words = append(words, base, address)
			address = 0
		} else {
			// The Gecko handler uses only BA's upper seven bits. The
			// remaining address bits belong to the write's address field.
			if opts.BugFixes {
				words = append(words, base, address&0xfe000000)
			} else {
				words = append(words, base, address)
				address = 0
			}
		}
	}
	words = append(words, kind<<24|address&0x01ffffff)
	if kind == 6 || kind == 0x16 {
		words = append(words, uint32(len(data)))
		for len(data)%8 != 0 {
			data = append(data, 0)
		}
	} else {
		for len(data) < 4 {
			data = append([]byte{0}, data...)
		}
	}
	words = append(words, bytesToWords(data)...)
	if n.address >= 0x82000000 {
		words = append(words, 0xe0000000, 0x80008000)
	}
	return words, nil
}
func encodeBlock(ctx context.Context, n node, gctOffset uint32, opts Options) ([]uint32, error) {
	labels := map[string]int{}
	sizes := make([]int, len(n.body))
	size := 0
	encodedData := make(map[int][]uint32)
	for i, item := range n.body {
		switch item.kind {
		case labelNode:
			key := strings.ToLower(item.text)
			if _, ok := labels[key]; ok {
				if !opts.Validation.RejectDuplicateLabels {
					continue
				}
				return nil, at(item.pos, fmt.Errorf("duplicate label %q", item.text))
			}
			labels[key] = size
		case dataNode:
			b, _, err := encodeData(item, true, opts.rules)
			if err != nil {
				return nil, at(item.pos, err)
			}
			encodedData[i] = bytesToWords(b)
			sizes[i] = len(encodedData[i])
		case rawNode, instructionNode:
			sizes[i] = 1
		default:
			return nil, at(item.pos, fmt.Errorf("invalid statement in PPC block"))
		}
		size += sizes[i]
	}
	var prefix []uint32
	first := n.address & 0x01ffffff
	switch n.block {
	case "hook":
		first |= 0xc2000000
	case "code":
		first |= 0x06000000
	case "pulse":
		first = 0xc0000000
	}
	if n.block != "pulse" && n.address >= 0x82000000 {
		if n.block == "code" {
			prefix = []uint32{0x4a000000, n.address}
			first = 0x16000000
		} else {
			prefix = []uint32{0x42000000, n.address & 0xfe000000}
			first = 0xc2000000 | n.address&0x01ffffff
			if !opts.BugFixes {
				prefix = []uint32{0x4a000000, n.address}
				first = 0xc2000000
			}
		}
	}
	var body []uint32
	for i, item := range n.body {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if item.kind == labelNode {
			continue
		}
		if item.kind == dataNode {
			body = append(body, encodedData[i]...)
			continue
		}
		if item.kind == rawNode {
			b, _ := hex.DecodeString(item.text)
			body = append(body, bytesToWords(b)...)
			continue
		}
		pc := uint32(0)
		var address *uint32
		if n.block == "code" {
			pc = n.address + uint32(len(body)*4)
			address = &pc
		} else if opts.BaseAddress != nil {
			pc = *opts.BaseAddress + gctOffset + uint32(len(prefix)*4) + 8 + uint32(len(body)*4)
			address = &pc
		}
		current := len(body)
		resolve := func(name string) (int64, bool) {
			switch strings.ToUpper(name) {
			case "%END%":
				return int64((size - current) * 4), true
			case "%START%":
				return int64(-current * 4), true
			}
			v, ok := labels[strings.ToLower(name)]
			return int64((v - current) * 4), ok
		}
		w, err := ppc.Encode(item.text, ppc.Context{BugFixes: opts.BugFixes, ExpressionSyntax: opts.ExpressionSyntax, AdditionalConsoleInstructions: opts.AdditionalConsoleInstructions, BranchExpressions: opts.BranchExpressions, AllowNonConsoleInstructions: opts.AllowNonConsoleInstructions, Dialect: opts.Dialect, Compatibility: opts.Compatibility, Address: address, Lookup: lookupValues(item.values), RelativeLabel: resolve, ConvertAbsolute: opts.ConvertAbsolute})
		if err != nil {
			return nil, at(item.pos, err)
		}
		body = append(body, w)
	}
	originalLen := len(body)
	if n.block == "hook" {
		if len(body)%2 == 0 {
			body = append(body, 0x60000000)
		}
		body = append(body, 0)
	} else if len(body)%2 != 0 {
		body = append(body, 0)
	}
	count := uint32(len(body) / 2)
	if n.block == "code" {
		count = uint32(originalLen * 4)
	}
	words := append(prefix, first, count)
	words = append(words, body...)
	if len(prefix) > 0 {
		words = append(words, 0xe0000000, 0x80008000)
	}
	return words, nil
}
func repairPathCase(path string) string {
	if _, e := os.Stat(path); e == nil {
		return path
	}
	parent, base := filepath.Dir(path), filepath.Base(path)
	if parent == path {
		return path
	}
	parent = repairPathCase(parent)
	entries, e := os.ReadDir(parent)
	if e != nil {
		return path
	}
	match := ""
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), base) {
			if match != "" {
				return path
			}
			match = entry.Name()
		}
	}
	if match != "" {
		return filepath.Join(parent, match)
	}
	return path
}
