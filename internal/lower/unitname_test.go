package lower

import (
	"strings"
	"testing"
)

// unitName keeps a safe file name as it is, and gives names that differ only in the characters it
// replaces different units: the hash of the whole name is what keeps "a b.gp" and "a$b.gp" apart.
func TestUnitNameUnique(t *testing.T) {
	for _, name := range []string{"a.gp", "geom_test.gp", "x-y+z.gp", "A1.gp"} {
		if got, want := unitName("/p/"+name), name+".cpp"; got != want {
			t.Errorf("unitName(%q) = %q, want %q", name, got, want)
		}
	}
	names := []string{"a b.gp", "a$b.gp", "a b.gp ", "a\tb.gp", "a'b.gp", "a`b.gp", "a;b.gp", "é.gp", "ü.gp", "a b.gp.x", "a_b.gp"}
	seen := make(map[string]string)
	for _, n := range names {
		got := unitName("/p/" + n)
		if strings.ContainsAny(got, " $'`;\t") || strings.ContainsFunc(got, func(r rune) bool { return r >= 0x80 }) {
			t.Errorf("unitName(%q) = %q has a character that is no good in a file name of a unit", n, got)
		}
		if !strings.HasSuffix(got, ".cpp") {
			t.Errorf("unitName(%q) = %q is not a C++ file", n, got)
		}
		if other, ok := seen[got]; ok {
			t.Errorf("unitName(%q) and unitName(%q) are both %q", n, other, got)
		}
		seen[got] = n
	}
	// The same name is always the same unit.
	if unitName("/p/a b.gp") != unitName("/q/a b.gp") {
		t.Error("the name of a unit depends on the folder")
	}
}
