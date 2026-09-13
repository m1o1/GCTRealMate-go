package expr

import (
	"testing"

	"gctrm/internal/dialect"
)

func TestEval(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want int64
	}{{"2 + 3 * 4", 14}, {"(2 + 3) * 4", 20}, {"$80001234 & 0xffff", 0x1234}, {"0b1010 << 3 | 1", 81}, {"~0 & 255", 255}, {"-12 / 5", -2}, {"17 % 5", 2}, {"0xffff ^ 0xff", 0xff00}, {"1 << (3 + 1)", 16}, {"0x80000000 >> 16", 32768}, {"+7 - -2", 9}} {
		t.Run(tc.s, func(t *testing.T) {
			got, err := Eval(tc.s, nil)
			if err != nil || got != tc.want {
				t.Fatalf("got %d, %v; want %d", got, err, tc.want)
			}
		})
	}
	v, err := Eval("base + 4", func(s string) (int64, bool) { return 100, s == "base" })
	if err != nil || v != 104 {
		t.Fatal(v, err)
	}
	for _, s := range []string{"", "missing", "1 / 0", "2 % 0", "1 << 64", "2 >> -1", "(2+3", "2 garbage", "2 +", "@", "3 ** 2"} {
		if _, err := Eval(s, nil); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func FuzzEval(f *testing.F) {
	for _, s := range []string{"1+2", "$80000000", "(a)", "~0", "1/0"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}
		for _, mode := range []dialect.Mode{dialect.Legacy, dialect.Modern} {
			_, _ = EvalMode(s, nil, mode)
			_, _ = Alias(s, nil, mode)
		}
	})
}

func TestLegacyAliasBoundaries(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   int64
	}{
		{"0xffffffff + 1", 0}, {"(0xffffffff + 1) / 2", 0},
		{"0 - 1", 0xffffffff}, {"(0 - 1) / 2", 0x7fffffff},
		{"~0", 0xffffffff}, {"~0 >> 31", 1}, {"0xffffffff * 0xffffffff", 1},
		{"2 + 3 * 4", 20}, {"6 ^ 3 & 1", 1}, {"010 + 1", 9},
		{"2 + 3 + 4", 9}, {"1 << 31", 0x80000000},
	} {
		got, err := Alias(tc.source, nil, dialect.Legacy)
		if err != nil || got != tc.want {
			t.Errorf("%s: %d %v; want %d", tc.source, got, err, tc.want)
		}
	}
	for _, source := range []string{"08", "0x100000000", "1<<32", "1<<-1", "1/0", "1%0"} {
		if _, err := Alias(source, nil, dialect.Legacy); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
}
