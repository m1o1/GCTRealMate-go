package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gctrm/fixes"

	"github.com/pelletier/go-toml/v2"
)

const maxConfigSize = 1 << 20

type configuration struct {
	Version    int             `toml:"version"`
	BugFixes   map[string]bool `toml:"bug_fixes"`
	Extensions map[string]bool `toml:"extensions"`
	Semantics  map[string]bool `toml:"semantics"`
	Encoding   map[string]bool `toml:"encoding"`
	Validation map[string]bool `toml:"validation"`
	CLI        map[string]bool `toml:"cli"`
}

// setChoice is shared by TOML and --set=section.key=true|false.
func setChoice(f *flags, key string, value bool) error {
	if strings.HasPrefix(key, "bug_fixes.") {
		if key == "bug_fixes.console_only" {
			f.allowNonConsoleInstructions = !value
			return nil
		}
		return f.fixes.Set(strings.TrimPrefix(key, "bug_fixes."), value)
	}
	switch key {
	case "extensions.dot_op":
		f.dotOp = value
	case "extensions.branch_expressions":
		f.branchExpressions = value
	case "extensions.expression_syntax":
		f.expressionSyntax = value
	case "extensions.implicit_sections":
		f.implicitSections = value
	case "extensions.additional_console_instructions":
		f.additionalConsoleInstructions = value
	case "semantics.decimal_leading_zeros":
		v := !value
		f.compatibility.OctalLiterals = &v
	case "semantics.c_operator_precedence":
		v := !value
		f.compatibility.LeftToRightExpressions = &v
	case "semantics.signed_64_bit_aliases":
		v := !value
		f.compatibility.Unsigned32BitAliases = &v
	case "encoding.gnu_branch_hints":
		v := !value
		f.compatibility.BranchHints = &v
	case "encoding.sign_extend_data_slots":
		v := !value
		f.compatibility.ZeroExtendedData = &v
	case "encoding.alternative_float_nan":
		v := !value
		f.compatibility.FloatNaN = &v
	case "encoding.alternative_double_nan":
		v := !value
		f.compatibility.DoubleNaN = &v
	case "validation.strict_register_prefixes":
		v := !value
		f.compatibility.RegisterPrefixes = &v
	case "validation.reject_duplicate_labels":
		f.validation.RejectDuplicateLabels = value
	case "validation.strict_macro_calls":
		f.validation.StrictMacroCalls = value
	case "validation.reject_undefined_macros":
		f.validation.RejectUndefinedMacros = value
	case "validation.reject_address_annotations":
		f.validation.RejectAddressAnnotations = value
	case "validation.reject_data_overflow":
		f.validation.RejectDataOverflow = value
	case "cli.exact_ini_matching":
		f.exactINI = value
	case "cli.flat_logs":
		f.flatLog = value
	case "cli.lf_line_endings":
		f.lf = value
	default:
		return fmt.Errorf("unknown configuration key %s", key)
	}
	return nil
}

// decodeConfig validates the current schema before assembly.
func decodeConfig(data []byte) (flags, error) {
	if len(data) > maxConfigSize {
		return flags{}, fmt.Errorf("configuration exceeds 1 MiB")
	}
	config := configuration{Version: 2}
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return flags{}, err
	}
	if config.Version != 2 {
		return flags{}, fmt.Errorf("unsupported configuration version %d (expected 2)", config.Version)
	}
	f := flags{fixes: fixes.All()}
	for _, table := range []struct {
		name   string
		values map[string]bool
	}{
		{"bug_fixes", config.BugFixes}, {"extensions", config.Extensions}, {"semantics", config.Semantics},
		{"encoding", config.Encoding}, {"validation", config.Validation}, {"cli", config.CLI},
	} {
		for key, value := range table.values {
			if err := setChoice(&f, table.name+"."+key, value); err != nil {
				return flags{}, err
			}
		}
	}
	return f, nil
}

// configArguments removes invocation-wide config selectors. They must precede
// all source files, and cannot be combined. Values of -o/-b and names after --
// are not reinterpreted as options. Help/version never depend on a config file.
func configArguments(args []string, executable string) ([]string, string, bool, error) {
	path := ""
	if executable != "" {
		path = strings.TrimSuffix(executable, filepath.Ext(executable)) + ".toml"
	}
	var out []string
	selected, explicit, seenInput := false, false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i:]...)
			break
		}
		if arg == "-o" || strings.EqualFold(arg, "-b") {
			out = append(out, arg)
			if i+1 < len(args) {
				i++
				out = append(out, args[i])
			}
			continue
		}
		if arg == "--help" || arg == "-h" {
			return nil, "", false, errHelp
		}
		if arg == "--version" {
			return nil, "", false, errVersion
		}
		if arg == "--no-config" || arg == "--config" || strings.HasPrefix(arg, "--config=") {
			if seenInput {
				return nil, "", false, fmt.Errorf("config selection must precede all input files")
			}
			if selected {
				return nil, "", false, fmt.Errorf("use only one --config or --no-config option")
			}
			selected = true
			if arg == "--no-config" {
				path = ""
				continue
			}
			explicit = true
			if arg == "--config" {
				if i+1 == len(args) {
					return nil, "", false, fmt.Errorf("--config requires a path")
				}
				i++
				path = args[i]
			} else {
				path = strings.TrimPrefix(arg, "--config=")
			}
			if path == "" {
				return nil, "", false, fmt.Errorf("--config requires a path")
			}
			continue
		}
		out = append(out, arg)
		if !strings.HasPrefix(arg, "-") {
			seenInput = true
		}
	}
	return out, path, explicit, nil
}

func loadConfig(path string, explicit bool) (flags, error) {
	if path == "" {
		return decodeConfig(nil)
	}
	file, err := os.Open(path)
	if !explicit && os.IsNotExist(err) {
		return decodeConfig(nil)
	}
	if err != nil {
		return flags{}, fmt.Errorf("configuration %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return flags{}, fmt.Errorf("configuration %s: %w", path, err)
	}
	f, err := decodeConfig(data)
	if err != nil {
		return flags{}, fmt.Errorf("configuration %s: %w", path, err)
	}
	return f, nil
}
