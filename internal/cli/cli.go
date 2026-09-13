// Package cli handles command-line policy and file output, separately from assembly.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gctrm/assembler"
	"gctrm/fixes"
)

const Version = "0.14.0-go (GameCube/Wii; GCTRealMate v0.2.6 syntax)"
const help = `Usage: gctrm [options] source.asm [options] another.asm

Assemble Gecko and PowerPC source into a .GCT beside each input.
  -t          Write codeset text
  -*          Write text with asterisks
  -g          Write text with asterisks and RSBE01 header
  -l          Write an include-tree log (unless cli.flat_logs is true)
  -b:ADDRESS  Address of the GCT header (hex); -b ADDRESS also works
  -a          Convert absolute ba/bla to relative branches
  -r          Repair case mismatches in include paths
  -i          Ignore INI settings for the following input
  -q          Accepted for compatibility; the CLI is noninteractive
  -o PATH     Set GCT output path (one input only)
  --config PATH     Use this TOML instead of <executable-basename>.toml
  --no-config       Ignore TOML; INI remains separate
  --bug-fixes=true|false       Set all fixes, including missing console instructions
  --set=section.key=true|false Set any categorized option
  --help            Show help
  --version         Show version

TOML groups: [bug_fixes], [extensions], [semantics], [encoding], [validation], [cli].
Every [bug_fixes] option defaults true; every other option defaults false.
Missing console mnemonics are enabled; syntax and validation extensions are opt-ins.
Set extensions.non_console_instructions=true to permit implemented broader
PowerPC forms. Other fixes and additional console mnemonics remain independent.
Full flag descriptions and examples: CONFIGURATION.md.

Example: --set=extensions.dot_op=true --set=validation.strict_macro_calls=true

Options precede their inputs. All long options persist across inputs; -a, -b,
and -i are per-input. Config selection is global and precedes all inputs.
Precedence: defaults < TOML < matching INI < CLI. INI example:
  patch.asm : --set=validation.strict_register_prefixes=true -t
`

type flags struct {
	fixes                                             fixes.Policy
	dotOp                                             bool
	branchExpressions                                 bool
	expressionSyntax                                  bool
	implicitSections                                  bool
	additionalConsoleInstructions                     bool
	validation                                        assembler.Validation
	allowNonConsoleInstructions                       bool
	text, stars, convert, log, repair, inline, ignore bool
	base                                              *uint32
	compatibility                                     assembler.Compatibility
	exactINI, flatLog, lf                             bool
}
type job struct {
	file  string
	flags flags
}

// Run executes the CLI and returns a conventional exit code. executable anchors
// the optional .ini file, making behavior independent of the current directory.
func Run(ctx context.Context, args []string, executable string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, help)
		return 0
	}
	jobs, output, err := plan(args, executable)
	if errors.Is(err, errHelp) {
		fmt.Fprint(stdout, help)
		return 0
	}
	if errors.Is(err, errVersion) {
		fmt.Fprintln(stdout, Version)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "gctrm:", err)
		return 2
	}
	status := 0
	for _, j := range jobs {
		result, err := assembler.Compile(ctx, j.file, assembler.Options{Fixes: j.flags.fixes, DotOp: &j.flags.dotOp, ExpressionSyntax: j.flags.expressionSyntax, ImplicitSections: j.flags.implicitSections, AdditionalConsoleInstructions: j.flags.additionalConsoleInstructions, Validation: j.flags.validation, BranchExpressions: j.flags.branchExpressions, AllowNonConsoleInstructions: j.flags.allowNonConsoleInstructions, Compatibility: j.flags.compatibility, BaseAddress: j.flags.base, ConvertAbsolute: j.flags.inline, RepairPathCase: j.flags.repair})
		if err != nil {
			fmt.Fprintln(stderr, err)
			// The reference reports missing inputs/includes but returns success.
			if j.flags.fixes.MissingFileStatus || !errors.Is(err, os.ErrNotExist) {
				status = 1
			}
			continue
		}
		stem := strings.TrimSuffix(j.file, filepath.Ext(j.file))
		destination := stem + ".GCT"
		if output != "" {
			destination = output
		}
		if samePath(destination, j.file) {
			fmt.Fprintln(stderr, "gctrm: output would overwrite source")
			status = 1
			continue
		}
		if err = writeAtomic(destination, result.Bytes()); err != nil {
			fmt.Fprintln(stderr, err)
			status = 1
			continue
		}
		if j.flags.text {
			if err = writeAtomic(stem+"_codeset.txt", textBytes(result.Text(j.flags.stars, j.flags.convert), j.flags.lf)); err != nil {
				fmt.Fprintln(stderr, err)
				status = 1
			}
		}
		if j.flags.log {
			log := result.Log()
			if j.flags.flatLog {
				log = result.FlatLog()
			}
			if err = writeAtomic(stem+"_log.txt", textBytes(log, j.flags.lf)); err != nil {
				fmt.Fprintln(stderr, err)
				status = 1
			}
		}
		fmt.Fprintf(stdout, "%s: %d bytes, %d code sections\n", destination, len(result.Bytes()), len(result.Codes))
	}
	return status
}

var errHelp = errors.New("help requested")
var errVersion = errors.New("version requested")

func plan(args []string, executable string) ([]job, string, error) {
	args, configPath, explicit, err := configArguments(args, executable)
	if err != nil {
		return nil, "", err
	}
	sticky, err := loadConfig(configPath, explicit)
	if err != nil {
		return nil, "", err
	}
	var jobs []job
	var pending []string
	output := ""
	literal := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !literal && arg == "--" {
			literal = true
			continue
		}
		if !literal && (arg == "--help" || arg == "-h") {
			return nil, "", errHelp
		}
		if !literal && arg == "--version" {
			return nil, "", errVersion
		}
		if !literal && arg == "-o" {
			i++
			if i == len(args) {
				return nil, "", fmt.Errorf("-o requires a path")
			}
			output = args[i]
			continue
		}
		if !literal && strings.HasPrefix(arg, "-") {
			if strings.EqualFold(arg, "-b") {
				i++
				if i == len(args) {
					return nil, "", fmt.Errorf("-b requires an address")
				}
				arg += ":" + args[i]
			}
			// Validate immediately, even if no input follows.
			check := flags{}
			if err := apply(&check, arg); err != nil {
				return nil, "", err
			}
			pending = append(pending, arg)
			continue
		}
		current := sticky
		current.base = nil
		current.inline = false
		current.ignore = false
		for _, p := range pending {
			if strings.HasPrefix(strings.ToLower(p), "-i") || strings.HasPrefix(p, "--set=cli.exact_ini_matching=") {
				if err := apply(&current, p); err != nil {
					return nil, "", err
				}
			}
		}
		if !current.ignore {
			settings, err := iniOptions(executable, arg, current.exactINI)
			if err != nil {
				return nil, "", err
			}
			for _, p := range settings {
				if err = apply(&current, p); err != nil {
					return nil, "", fmt.Errorf("settings: %w", err)
				}
			}
		}
		for _, p := range pending {
			if err := apply(&current, p); err != nil {
				return nil, "", err
			}
		}
		jobs = append(jobs, job{arg, current})
		sticky = current
		pending = nil
	}
	if len(pending) > 0 {
		return nil, "", fmt.Errorf("options must precede an input file")
	}
	if len(jobs) == 0 {
		return nil, "", fmt.Errorf("no source files supplied")
	}
	if output != "" && len(jobs) != 1 {
		return nil, "", fmt.Errorf("-o requires exactly one input")
	}
	// Reject collisions before processing any input.
	destinations := map[string]bool{}
	for _, j := range jobs {
		stem := strings.TrimSuffix(j.file, filepath.Ext(j.file))
		dest := stem + ".GCT"
		if output != "" {
			dest = output
		}
		files := []string{dest}
		if j.flags.text {
			files = append(files, stem+"_codeset.txt")
		}
		if j.flags.log {
			files = append(files, stem+"_log.txt")
		}
		for _, out := range files {
			if configPath != "" && samePath(out, configPath) {
				return nil, "", fmt.Errorf("output %s would overwrite the configuration", out)
			}
			for _, input := range jobs {
				if samePath(out, input.file) {
					return nil, "", fmt.Errorf("output %s would overwrite an input", out)
				}
			}
			full, err := filepath.Abs(out)
			if err != nil {
				return nil, "", err
			}
			key := strings.ToLower(full)
			if destinations[key] {
				return nil, "", fmt.Errorf("duplicate output %s", out)
			}
			destinations[key] = true
		}
	}
	return jobs, output, nil
}
func apply(f *flags, arg string) error {
	if strings.HasPrefix(arg, "--") {
		name, value, _ := strings.Cut(arg, "=")
		switch name {
		case "--set":
			key, setting, ok := strings.Cut(value, "=")
			if !ok || (setting != "true" && setting != "false") {
				return fmt.Errorf("--set expects section.key=true|false")
			}
			return setChoice(f, key, setting == "true")
		case "--bug-fixes":
			if value != "true" && value != "false" {
				return fmt.Errorf("--bug-fixes expects true or false")
			}
			f.fixes = fixes.FromBool(value == "true")
			f.additionalConsoleInstructions = value == "true"
		default:
			return fmt.Errorf("unknown option %q", arg)
		}
		return nil
	}
	option, val, has := strings.Cut(strings.TrimPrefix(arg, "-"), ":")
	option = strings.ToLower(option)
	if option == "b" {
		if !has || val == "" {
			return fmt.Errorf("-b requires a hexadecimal address")
		}
		val = strings.TrimPrefix(val, "$")
		val = strings.TrimPrefix(strings.ToLower(val), "0x")
		n, e := strconv.ParseUint(val, 16, 32)
		if e != nil {
			return fmt.Errorf("invalid base address %q", val)
		}
		v := uint32(n)
		if v&3 != 0 {
			return fmt.Errorf("base address must be word-aligned")
		}
		f.base = &v
		return nil
	}
	enabled := true
	if has {
		switch val {
		case "0":
			enabled = false
		case "1":
		default:
			return fmt.Errorf("%s expects :0 or :1", arg)
		}
	}
	switch option {
	case "g":
		f.convert = enabled
		f.stars = enabled
		f.text = enabled
	case "*":
		f.stars = enabled
		f.text = enabled
	case "t":
		f.text = enabled
	case "l":
		f.log = enabled
	case "r":
		f.repair = enabled
	case "a":
		f.inline = enabled
	case "i":
		f.ignore = enabled
	case "q", "p", "c": // Historical console/no-op switches.
	default:
		return fmt.Errorf("unknown option %q", arg)
	}
	return nil
}
func iniOptions(executable, input string, exact bool) ([]string, error) {
	if executable == "" {
		return nil, nil
	}
	path := strings.TrimSuffix(executable, filepath.Ext(executable)) + ".ini"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("settings file exceeds 1 MiB")
	}
	for lineNum, line := range strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		legacyMatch := strings.HasPrefix(strings.ToLower(line), strings.ToLower(filepath.Base(input)))
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		name, values, ok := strings.Cut(line, ":")
		if !exact && !legacyMatch {
			continue
		}
		if !ok {
			if !exact {
				return nil, nil
			}
			return nil, fmt.Errorf("%s:%d: missing colon", path, lineNum+1)
		}
		if exact && !strings.EqualFold(strings.Trim(strings.TrimSpace(name), "\""), filepath.Base(input)) {
			continue
		}
		tokens := strings.Fields(values)
		var out []string
		for i := 0; i < len(tokens); i++ {
			p := tokens[i]
			if p == "#" || strings.HasPrefix(p, ";") {
				break
			}
			if strings.EqualFold(p, "-b") {
				i++
				if i == len(tokens) {
					return nil, fmt.Errorf("settings: missing base address")
				}
				p += ":" + tokens[i]
			}
			out = append(out, p)
		}
		return out, nil
	}
	return nil, nil
}
func textBytes(s string, lf bool) []byte {
	if runtime.GOOS == "windows" && !lf {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	return []byte(s)
}
func samePath(a, b string) bool {
	aa, ea := filepath.Abs(a)
	bb, eb := filepath.Abs(b)
	if ea != nil || eb != nil {
		return false
	}
	if strings.EqualFold(aa, bb) {
		return true
	}
	as, e1 := os.Stat(aa)
	bs, e2 := os.Stat(bb)
	return e1 == nil && e2 == nil && os.SameFile(as, bs)
}
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".gctrm-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
