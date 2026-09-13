package assembler

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gctrm/internal/expr"
)

type nodeKind uint8

const (
	rawNode nodeKind = iota
	instructionNode
	dataNode
	writeNode
	blockNode
	labelNode
	directiveNode
)

type node struct {
	expressionSyntax bool
	kind             nodeKind
	pos              Position
	text             string
	address          uint32
	body             []node
	block            string
	values           map[string]string
}
type section struct {
	flushTail bool
	name      string
	pos       Position
	nodes     []node
}
type macro struct {
	params []string
	body   []token
}
type scope struct {
	values map[string]string
	macros map[string]macro
	parent *scope
}

func newScope(parent *scope) *scope { return &scope{map[string]string{}, map[string]macro{}, parent} }
func (s *scope) value(name string) (string, bool) {
	for ; s != nil; s = s.parent {
		if v, ok := s.values[strings.ToLower(name)]; ok {
			return v, true
		}
	}
	return "", false
}
func (s *scope) snapshot() map[string]string {
	m := map[string]string{}
	var visit func(*scope)
	visit = func(p *scope) {
		if p == nil {
			return
		}
		visit(p.parent)
		for k, v := range p.values {
			m[k] = v
		}
	}
	visit(s)
	return m
}
func (s *scope) lookup(name string) (int64, bool) {
	name = strings.TrimPrefix(name, "$")
	v, ok := s.value(name)
	if !ok {
		return 0, false
	}
	n, e := strconv.ParseInt(v, 10, 64)
	return n, e == nil
}
func (s *scope) findMacro(name string) (macro, bool) {
	for ; s != nil; s = s.parent {
		if m, ok := s.macros[name]; ok {
			return m, true
		}
	}
	return macro{}, false
}

type frontend struct {
	ctx         context.Context
	opts        Options
	root        string
	sections    []section
	scope       *scope
	activeFiles map[string]bool
	expanded    int
	disabled    bool
	logEntries  []logEntry
}

var identifier = regexp.MustCompile(`^(?:[A-Za-z_.$][A-Za-z0-9_.$]*|[0-9]+_[A-Za-z0-9_.$]+)$`)
var labelIdentifier = regexp.MustCompile(`^[A-Za-z_.$][A-Za-z0-9_.$-]*$`)
var aliasTokens = regexp.MustCompile(`[A-Za-z0-9_]+`)

func (f *frontend) parse(tokens []token, local *scope, target *[]node, inBlock bool, includeDepth, macroDepth int) error {
	for i := 0; i < len(tokens); i++ {
		if err := f.ctx.Err(); err != nil {
			return err
		}
		f.expanded++
		if f.expanded > 1_000_000 {
			return fmt.Errorf("expanded source exceeds one million statements")
		}
		tok := tokens[i]
		s := strings.TrimSpace(tok.text)
		if !inBlock {
			s = strings.TrimSpace(strings.TrimPrefix(s, "*"))
		}
		name, rest := head(s)
		if name == ".op" {
			if f.opts.DotOp == nil || !*f.opts.DotOp {
				continue
			}
			s = strings.TrimPrefix(s, ".")
			name = "op"
		}
		if !inBlock && (name == "code" || name == "hook") && !strings.Contains(rest, "@") && (i+1 == len(tokens) || tokens[i+1].text != "{") {
			f.newSection(s, tok.pos, includeDepth)
			continue
		}
		env := local
		if !inBlock {
			env = f.scope
		}
		if !inBlock && strings.HasPrefix(s, "!") {
			f.disabled = true
			continue
		}
		// Declarations consume their body even in disabled sections.
		if name == ".macro" {
			body, end, err := bodyAfter(tokens, i)
			if err != nil {
				return at(tok.pos, err)
			}
			i = end
			if f.disabled && !inBlock {
				continue
			}
			mn, params, err := macroCall(rest)
			if err != nil {
				return at(tok.pos, err)
			}
			seen := map[string]bool{}
			for _, p := range params {
				if !strings.HasPrefix(p, "<") || !strings.HasSuffix(p, ">") || !identifier.MatchString(p[1:len(p)-1]) || seen[p] {
					return at(tok.pos, fmt.Errorf("invalid or duplicate macro parameter %q", p))
				}
				seen[p] = true
			}
			env.macros[mn] = macro{params, body}
			continue
		}
		if s == "{" {
			_, end, err := bodyAfter(tokens, i-1)
			if err != nil {
				return at(tok.pos, err)
			}
			if !f.disabled {
				return at(tok.pos, fmt.Errorf("block without HOOK, CODE, or PULSE"))
			}
			i = end
			continue
		}
		if s == "}" {
			return at(tok.pos, fmt.Errorf("unexpected closing brace"))
		}
		if name == ".include" {
			path := strings.TrimSpace(rest)
			if strings.HasPrefix(path, "\"") {
				v, e := strconv.Unquote(path)
				if e != nil {
					return at(tok.pos, e)
				}
				path = v
			}
			f.logEntries = append(f.logEntries, logEntry{include: path, depth: includeDepth})
			path = strings.ReplaceAll(path, "\\", "/")
			base := f.root
			if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
				base = filepath.Dir(tok.pos.File)
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(base, path)
			}
			path = filepath.Clean(path)
			if f.opts.RepairPathCase {
				path = repairPathCase(path)
			}
			if includeDepth >= 16 {
				return at(tok.pos, fmt.Errorf("include depth exceeds 16"))
			}
			if f.activeFiles[path] {
				return at(tok.pos, fmt.Errorf("include cycle involving %s", path))
			}
			data, err := f.opts.ReadFile(path)
			if err != nil {
				return at(tok.pos, err)
			}
			if len(data) > 16<<20 {
				return at(tok.pos, fmt.Errorf("included file exceeds 16 MiB"))
			}
			included, err := scanPolicy(path, data, f.opts.Fixes)
			if err != nil {
				return err
			}
			f.activeFiles[path] = true
			err = f.parse(included, env, target, inBlock, includeDepth+1, macroDepth)
			delete(f.activeFiles, path)
			if err != nil {
				return err
			}
			continue
		}
		if !inBlock && f.disabled {
			if !looksLikeStatement(s) {
				f.newSection(s, tok.pos, includeDepth)
				env = f.scope
			} else {
				continue
			}
			continue
		}
		if name == ".alias" {
			key, val, ok := strings.Cut(rest, "=")
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			if !f.opts.Fixes.AliasTerms && !strings.HasPrefix(val, "\"") {
				val = compact(val)
			}
			if !ok || !identifier.MatchString(key) {
				return at(tok.pos, fmt.Errorf("expected .alias Name = expression"))
			}
			if strings.HasPrefix(val, "\"") {
				v, e := strconv.Unquote(val)
				if e != nil {
					return at(tok.pos, e)
				}
				env.values[strings.ToLower(key)] = v
			} else {
				n, e := expr.AliasRules(val, env.lookup, f.opts.rules)
				if e != nil {
					return at(tok.pos, e)
				}
				env.values[strings.ToLower(key)] = strconv.FormatInt(n, 10)
			}
			continue
		}
		if strings.HasPrefix(s, "%") {
			if macroDepth >= 32 {
				return at(tok.pos, fmt.Errorf("macro expansion depth exceeds 32"))
			}
			call := s[1:]
			if !f.opts.Validation.StrictMacroCalls && strings.Contains(call, "(") && !strings.HasSuffix(call, ")") {
				call += ")"
			}
			mn, args, err := macroCall(call)
			if err != nil {
				return at(tok.pos, err)
			}
			m, ok := env.findMacro(mn)
			if !ok {
				if !f.opts.Validation.RejectUndefinedMacros {
					continue
				}
				return at(tok.pos, fmt.Errorf("undefined macro %q", mn))
			}
			if len(args) < len(m.params) || f.opts.Validation.StrictMacroCalls && len(args) != len(m.params) {
				return at(tok.pos, fmt.Errorf("macro %s expects %d arguments, got %d", mn, len(m.params), len(args)))
			}
			args = args[:len(m.params)]
			pairs := make([]string, 0, len(args)*2)
			for j, arg := range args {
				if v, ok := env.value(arg); ok {
					arg = v
				}
				pairs = append(pairs, m.params[j], arg)
			}
			repl := strings.NewReplacer(pairs...)
			expanded := make([]token, len(m.body))
			for j, t := range m.body {
				expanded[j] = token{repl.Replace(t.text), t.pos}
			}
			if err = f.parse(expanded, env, target, inBlock, includeDepth, macroDepth+1); err != nil {
				return fmt.Errorf("%s: expanding %s: %w", tok.pos, mn, err)
			}
			continue
		}
		if !inBlock && len(f.sections) == 0 && !f.opts.ImplicitSections && (looksLikeStatement(s) || isHex(compact(s), 16) || strings.HasSuffix(s, ":")) {
			return at(tok.pos, fmt.Errorf("source requires an initial section name; enable extensions.implicit_sections to omit it"))
		}
		appendNode := func(n node) {
			n.expressionSyntax = f.opts.ExpressionSyntax
			if inBlock {
				*target = append(*target, n)
			} else {
				f.ensureSection(tok.pos, includeDepth)
				j := len(f.sections) - 1
				f.sections[j].nodes = append(f.sections[j].nodes, n)
			}
		}
		for {
			colon := strings.IndexByte(s, ':')
			if colon < 0 || !labelIdentifier.MatchString(strings.TrimSpace(s[:colon])) {
				break
			}
			if tail := strings.TrimSpace(s[colon+1:]); !inBlock && tail != "" && !looksLikeStatement(tail) {
				break
			}
			appendNode(node{kind: labelNode, pos: tok.pos, text: strings.TrimSpace(s[:colon])})
			s = strings.TrimSpace(s[colon+1:])
			name, rest = head(s)
		}
		if s == "" {
			continue
		}
		if name == "hook" || name == "code" || name == "pulse" {
			if inBlock {
				return at(tok.pos, fmt.Errorf("nested assembly block"))
			}
			var address uint32
			var err error
			if name != "pulse" {
				_, a, ok := strings.Cut(rest, "@")
				if !ok {
					return at(tok.pos, fmt.Errorf("%s requires @ address", name))
				}
				address, err = parseSourceAddress(a, env.lookup, f.opts.ExpressionSyntax, f.opts.Validation.RejectAddressAnnotations)
				if err != nil {
					return at(tok.pos, err)
				}
			} else if rest != "" {
				return at(tok.pos, fmt.Errorf("PULSE takes no address"))
			}
			if f.opts.Fixes.AddressAlignment && address&3 != 0 {
				return at(tok.pos, fmt.Errorf("block address is not word-aligned"))
			}
			body, end, err := bodyAfter(tokens, i)
			if err != nil {
				return at(tok.pos, err)
			}
			i = end
			n := node{kind: blockNode, pos: tok.pos, block: name, address: address}
			if err = f.parse(body, newScope(env), &n.body, true, includeDepth, macroDepth); err != nil {
				return err
			}
			appendNode(n)
			continue
		}
		if strings.HasPrefix(s, ".") || strings.HasPrefix(strings.ToUpper(s), "BA") && strings.ContainsAny(s, "=<>-") || strings.HasPrefix(strings.ToUpper(s), "PO") && strings.ContainsAny(s, "=<>-") {
			if inBlock {
				return at(tok.pos, fmt.Errorf("Gecko directive inside PPC block"))
			}
			appendNode(node{kind: directiveNode, pos: tok.pos, text: s, values: env.snapshot()})
			continue
		}
		hex := compact(strings.TrimPrefix(s, "*"))
		if isHex(hex, 16) && !inBlock {
			appendNode(node{kind: rawNode, pos: tok.pos, text: hex})
			continue
		}
		if inBlock && isHex(hex, 8) {
			appendNode(node{kind: rawNode, pos: tok.pos, text: hex})
			continue
		}
		if isDataType(name) || name == "op" {
			content, addressText, hasAddress := splitAddress(s)
			kind := dataNode
			var address uint32
			var err error
			if hasAddress {
				if inBlock {
					return at(tok.pos, fmt.Errorf("addressed write inside PPC block"))
				}
				kind = writeNode
				address, err = parseSourceAddress(addressText, env.lookup, f.opts.ExpressionSyntax, f.opts.Validation.RejectAddressAnnotations)
				if err != nil {
					return at(tok.pos, err)
				}
			}
			if name == "op" && !hasAddress {
				return at(tok.pos, fmt.Errorf("op requires @ address"))
			}
			appendNode(node{kind: kind, pos: tok.pos, text: strings.TrimSpace(content), address: address, values: env.snapshot()})
			continue
		}
		if inBlock {
			appendNode(node{kind: instructionNode, pos: tok.pos, text: expandAliases(s, env), values: env.snapshot()})
			continue
		}
		if looksLikeStatement(s) {
			return at(tok.pos, fmt.Errorf("invalid statement %q", s))
		}
		f.newSection(s, tok.pos, includeDepth)
	}
	return nil
}
func (f *frontend) ensureSection(p Position, depth int) {
	if len(f.sections) == 0 {
		f.logEntries = append(f.logEntries, logEntry{section: len(f.sections), depth: depth})
		f.sections = append(f.sections, section{name: "Codes", pos: p})
	}
}
func (f *frontend) newSection(name string, p Position, depth int) {
	f.logEntries = append(f.logEntries, logEntry{section: len(f.sections), depth: depth})
	f.sections = append(f.sections, section{name: name, pos: p})
	f.scope = newScope(nil)
	f.disabled = false
}
func bodyAfter(tokens []token, i int) ([]token, int, error) {
	if i+1 >= len(tokens) || tokens[i+1].text != "{" {
		return nil, 0, fmt.Errorf("expected opening brace")
	}
	depth := 1
	for j := i + 2; j < len(tokens); j++ {
		switch tokens[j].text {
		case "{":
			depth++
		case "}":
			depth--
		}
		if depth == 0 {
			return tokens[i+2 : j], j, nil
		}
	}
	return nil, 0, fmt.Errorf("missing closing brace")
}
func macroCall(s string) (string, []string, error) {
	i := strings.IndexByte(s, '(')
	if i < 1 || !strings.HasSuffix(strings.TrimSpace(s), ")") {
		return "", nil, fmt.Errorf("expected macro name(arguments)")
	}
	name := strings.TrimSpace(s[:i])
	if !identifier.MatchString(name) {
		return "", nil, fmt.Errorf("invalid macro name %q", name)
	}
	args, err := splitArgs(strings.TrimSpace(s)[i+1 : len(strings.TrimSpace(s))-1])
	return name, args, err
}
func expandAliases(s string, env *scope) string {
	var b strings.Builder
	start := 0
	for _, loc := range aliasTokens.FindAllStringIndex(s, -1) {
		b.WriteString(s[start:loc[0]])
		text := s[loc[0]:loc[1]]
		// Keep address aliases symbolic: replacing $Name with a decimal
		// expansion would reinterpret that decimal string as hexadecimal.
		if loc[0] == 0 || s[loc[0]-1] != '$' {
			if v, ok := env.value(text); ok {
				text = v
			}
		}
		b.WriteString(text)
		start = loc[1]
	}
	b.WriteString(s[start:])
	return b.String()
}
func parseAddress(s string, lookup expr.Lookup) (uint32, error) {
	s = compact(s)
	if s == "" {
		return 0, fmt.Errorf("missing address")
	}
	if lookup != nil {
		if v, ok := lookup(strings.TrimPrefix(s, "$")); ok {
			if v < 0 || v > 0xffffffff {
				return 0, fmt.Errorf("address out of range")
			}
			return uint32(v), nil
		}
	}
	if !strings.HasPrefix(s, "$") && !strings.HasPrefix(strings.ToLower(s), "0x") {
		s = "$" + s
	}
	v, err := expr.Eval(s, lookup)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 0xffffffff {
		return 0, fmt.Errorf("address out of range")
	}
	return uint32(v), nil
}
func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
func looksLikeStatement(s string) bool {
	n, _ := head(s)
	return strings.HasPrefix(s, ".") || strings.HasPrefix(s, "%") || strings.HasPrefix(s, "*") || strings.Contains(s, "@") || n == "hook" || n == "code" || n == "pulse" || isDataType(n) || isHex(compact(s), 16)
}
