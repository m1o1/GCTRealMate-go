package assembler

import (
	"fmt"
	"strings"
)

// Position identifies the original source location, including included files.
type Position struct {
	File string
	Line int
}

func (p Position) String() string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// Error describes an assembly failure with a source location.
type Error struct {
	Position Position
	Message  string
	cause    error
}

func (e *Error) Error() string { return e.Position.String() + ": " + e.Message }
func (e *Error) Unwrap() error { return e.cause }
func at(p Position, err error) error {
	if err == nil {
		return nil
	}
	return &Error{p, err.Error(), err}
}

type token struct {
	text string
	pos  Position
}

// scan splits statements while retaining strings, line numbers, and braces.
// It deliberately has no knowledge of opcodes or macro semantics.
func scan(name string, source []byte) ([]token, error) {
	return scanPolicy(name, source, true)
}

func scanPolicy(name string, source []byte, bugFixes bool) ([]token, error) {
	text := strings.TrimPrefix(string(source), "\ufeff")
	var out []token
	var b strings.Builder
	line, start := 1, 1
	quote, escape, comment := false, false, false
	emit := func() {
		s := strings.TrimSpace(b.String())
		if s != "" {
			out = append(out, token{s, Position{name, start}})
		}
		b.Reset()
		start = line
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if comment {
			if c == '*' && i+1 < len(text) && text[i+1] == '/' {
				comment = false
				i++
			}
			if c == '\n' {
				line++
			}
			continue
		}
		if quote {
			b.WriteByte(c)
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quote = false
			}
			if c == '\n' {
				line++
			}
			continue
		}
		if b.Len() == 0 && c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			start = line
		}
		if c == '"' {
			quote = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(text) && text[i+1] == '*' {
			comment = true
			i++
			continue
		}
		pipeContinuation := false
		if c == '|' {
			prefix := strings.ToLower(strings.TrimSpace(b.String()))
			pipeContinuation = !bugFixes || !strings.HasPrefix(prefix, ".alias") && !strings.HasPrefix(prefix, ".gr")
		}
		if pipeContinuation {
			// GCTRealMate's | discards the rest of this physical line and
			// continues the same statement on the next one.
			for i < len(text) && text[i] != '\n' {
				i++
			}
			line++
			b.WriteByte(' ')
			continue
		}
		if c == '#' || (c == '/' && i+1 < len(text) && text[i+1] == '/') || (c == '\\' && i+1 < len(text) && text[i+1] == '\\') {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			emit()
			line++
			start = line
			continue
		}
		if !bugFixes && c == '*' {
			// Inline op reads through @ without the outer scanner's filtering.
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(b.String())), "op ") {
				continue
			}
		}
		switch c {
		case '\n':
			emit()
			line++
			start = line
		case '\r':
		case ';':
			emit()
		case '{', '}':
			emit()
			out = append(out, token{string(c), Position{name, line}})
		default:
			b.WriteByte(c)
		}
	}
	if quote {
		return nil, at(Position{name, start}, fmt.Errorf("unterminated string"))
	}
	if comment {
		return nil, at(Position{name, line}, fmt.Errorf("unterminated block comment"))
	}
	emit()
	return out, nil
}
func splitArgs(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []string
	depth, start := 0, 0
	quote, escape := false, false
	for i, c := range s {
		if quote {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quote = false
			}
			continue
		}
		switch c {
		case '"':
			quote = true
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
		if depth < 0 {
			return nil, fmt.Errorf("unmatched parenthesis")
		}
	}
	if depth != 0 || quote {
		return nil, fmt.Errorf("unclosed parenthesis or string")
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out, nil
}
func head(s string) (string, string) {
	i := strings.IndexAny(s, " \t[")
	if i < 0 {
		return strings.ToLower(s), ""
	}
	return strings.ToLower(s[:i]), strings.TrimSpace(s[i:])
}
func compact(s string) string { return strings.Join(strings.Fields(s), "") }

// splitAddress ignores @ inside a quoted string or parenthesized expression.
func splitAddress(s string) (string, string, bool) {
	quote, escape, depth := false, false, 0
	for i, c := range s {
		if quote {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quote = false
			}
			continue
		}
		switch c {
		case '"':
			quote = true
		case '(':
			depth++
		case ')':
			depth--
		case '@':
			if depth == 0 {
				return s[:i], s[i+1:], true
			}
		}
	}
	return s, "", false
}
