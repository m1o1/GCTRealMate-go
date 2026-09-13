package assembler

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gctrm/internal/dialect"
	"gctrm/internal/expr"
)

var dataWidths = map[string]int{"uint8_t": 1, "int8_t": 1, "byte": 1, "uint16_t": 2, "int16_t": 2, "half": 2, "uint32_t": 4, "int32_t": 4, "int": 4, "word": 4, "address": 4, "float": 4, "scalar": 4, "double": 8, "string": 1, "ic_basic": 4, "ic_bit": 4, "ic_float": 4, "la_basic": 4, "la_bit": 4, "la_float": 4, "ra_basic": 4, "ra_bit": 4, "ra_float": 4}

func isDataType(name string) bool { _, ok := dataWidths[name]; return ok }
func lookupValues(values map[string]string) expr.Lookup {
	return func(name string) (int64, bool) {
		s, ok := values[strings.ToLower(strings.TrimPrefix(name, "$"))]
		if !ok {
			return 0, false
		}
		v, e := strconv.ParseInt(s, 10, 64)
		return v, e == nil
	}
}

func encodeData(n node, inBlock bool, rules dialect.Rules) ([]byte, bool, error) {
	name, rest := head(n.text)
	width := dataWidths[name]
	if width == 0 {
		return nil, false, fmt.Errorf("unknown data type %q", name)
	}
	count, explicit := 1, false
	if strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return nil, false, fmt.Errorf("unclosed array size")
		}
		v, e := expr.EvalDataRules(rest[1:end], lookupValues(n.values), rules)
		if e != nil {
			return nil, false, e
		}
		if v < 1 || v > 1<<20 {
			return nil, false, fmt.Errorf("array size outside 1..1048576")
		}
		count = int(v)
		explicit = true
		rest = strings.TrimSpace(rest[end+1:])
	}
	args, err := splitArgs(rest)
	if err != nil {
		return nil, false, err
	}
	if explicit && len(args) > 0 && args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}
	if len(args) != count {
		return nil, false, fmt.Errorf("expected %d %s values, got %d", count, name, len(args))
	}
	out := make([]byte, 0, count*width)
	for _, arg := range args {
		if v, ok := n.values[strings.ToLower(arg)]; ok {
			arg = v
		}
		if name == "string" {
			s, e := strconv.Unquote(arg)
			if e != nil {
				return nil, false, fmt.Errorf("invalid string: %w", e)
			}
			out = append(out, []byte(s)...)
			out = append(out, 0)
			continue
		}
		var value uint64
		if name == "float" || name == "double" || name == "scalar" {
			switch strings.ToLower(arg) {
			case "infinite", "+infinite":
				arg = "+Inf"
			case "-infinite":
				arg = "-Inf"
			}
			bits := 32
			if name == "double" {
				bits = 64
			}
			// strconv does not accept signed NaN spellings. Parse them explicitly
			// and select a stable payload instead of inheriting the host runtime.
			nanToken := strings.TrimSuffix(strings.ToLower(arg), "f")
			if strings.HasPrefix(nanToken, "+") || strings.HasPrefix(nanToken, "-") {
				nanToken = nanToken[1:]
			}
			nan := nanToken == "nan"
			parseArg := arg
			if nan {
				parseArg = "NaN"
			}
			f, e := strconv.ParseFloat(parseArg, bits)
			if e != nil && strings.HasSuffix(strings.ToLower(arg), "f") {
				f, e = strconv.ParseFloat(arg[:len(arg)-1], bits)
			}
			if e != nil {
				return nil, false, e
			}
			if name == "scalar" && (math.IsNaN(f) || math.IsInf(f, 0)) {
				return nil, false, fmt.Errorf("non-finite numeric value")
			}
			switch name {
			case "float":
				value = uint64(math.Float32bits(float32(f)))
			case "double":
				value = math.Float64bits(f)
			case "scalar":
				scaled := float32(f) * 60000
				if scaled < math.MinInt32 || float64(scaled) > math.MaxInt32 {
					return nil, false, fmt.Errorf("scalar outside signed 32-bit range")
				}
				value = uint64(uint32(int32(scaled)))
			}
			if nan {
				if bits == 32 {
					value = 0x7fc00000
					if rules.FloatNaN {
						value = 0x7fffffff
					}
					if strings.HasPrefix(arg, "-") {
						value |= 1 << 31
					}
				} else {
					value = 0x7ff8000000000001
					if rules.DoubleNaN {
						value = 0x7fffffffffffffff
					}
					if strings.HasPrefix(arg, "-") {
						value |= 1 << 63
					}
				}
			}
		} else {
			var v int64
			var e error
			if name == "address" {
				var a uint32
				a, e = parseSourceAddress(arg, lookupValues(n.values), rules.ExpressionSyntax, true)
				v = int64(a)
			} else {
				v, e = expr.EvalDataRules(arg, lookupValues(n.values), rules)
			}
			if e != nil {
				return nil, false, e
			}
			if strings.Contains(name, "_") && (strings.HasPrefix(name, "ic_") || strings.HasPrefix(name, "la_") || strings.HasPrefix(name, "ra_")) {
				if rules.BugFixes && (v < 0 || v > 0xffffff) {
					return nil, false, fmt.Errorf("PSA variable index outside 24-bit range")
				}
				base := uint32(0)
				if name[:2] == "la" {
					base = 0x10000000
				}
				if name[:2] == "ra" {
					base = 0x20000000
				}
				if strings.HasSuffix(name, "_bit") {
					base |= 0x02000000
				}
				if strings.HasSuffix(name, "_float") {
					base |= 0x01000000
				}
				value = uint64(base | uint32(v)&0xffffff)
				if !rules.BugFixes && !inBlock {
					value = uint64(uint32(v) & 0xffffff)
				}
			} else {
				// The native reference converts through a 32-bit unsigned value
				// before narrowing. Permissive byte/half casts do not admit values
				// beyond that conversion's range.
				if !rules.RejectDataOverflow && (v < -0xffffffff || v > 0xffffffff) {
					return nil, false, fmt.Errorf("value %d exceeds the reference 32-bit data conversion range", v)
				}
				// Legacy aliases store negative values as unsigned 32-bit
				// two's-complement patterns. Recover the signed value before
				// checking a narrower data field, just as for PPC immediates.
				if rules.Unsigned32BitAliases && width < 4 && v >= 0x80000000 && v <= 0xffffffff {
					v = int64(int32(v))
				}
				if rules.RejectDataOverflow && (v < -(1<<(width*8-1)) || v > int64((uint64(1)<<(width*8))-1)) {
					return nil, false, fmt.Errorf("value %d does not fit %s", v, name)
				}
				value = uint64(v)
			}
		}
		if inBlock && !explicit && width < 4 {
			if rules.ZeroExtendedData {
				value &= (uint64(1) << (width * 8)) - 1
			}
			width = 4
		}
		for i := width - 1; i >= 0; i-- {
			out = append(out, byte(value>>uint(i*8)))
		}
	}
	return out, name == "string" || name == "double", nil
}
func bytesToWords(data []byte) []uint32 {
	out := make([]uint32, (len(data)+3)/4)
	for i, b := range data {
		out[i/4] |= uint32(b) << uint(24-8*(i%4))
	}
	return out
}
func appendWords(data []byte, words ...uint32) []byte {
	for _, w := range words {
		data = binary.BigEndian.AppendUint32(data, w)
	}
	return data
}
