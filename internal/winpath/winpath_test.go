package winpath

import (
	"strings"
	"testing"
)

// fake makes this a Windows where "X" is outside the code page and the 8.3 names are a table.
func fake(t *testing.T, shorts map[string]string) {
	t.Helper()
	oldE, oldS, oldH := Enabled, SafeFunc, ShortFunc
	Enabled = true
	SafeFunc = func(s string) bool { return !strings.Contains(s, "X") }
	ShortFunc = func(p string) (string, bool) {
		s, ok := shorts[p]
		return s, ok && !strings.Contains(s, "X")
	}
	t.Cleanup(func() { Enabled, SafeFunc, ShortFunc = oldE, oldS, oldH })
}

func TestCompiler(t *testing.T) {
	fake(t, map[string]string{
		`C:\Users\JXrgen`:            `C:\Users\JRGEN~1`,
		`C:\Users\JXrgen\proj`:       `C:\Users\JRGEN~1\proj`,
		`C:\Users\JXrgen\proj\a b.h`: `C:\Users\JRGEN~1\proj\AB~1.H`,
		`C:/Users/JXrgen/proj`:       `C:\Users\JRGEN~1\proj`,
	})
	for _, c := range []struct{ in, want string }{
		{`C:\Users\proj\a.h`, `C:\Users\proj\a.h`},                     // safe: unchanged
		{`C:\Users\JXrgen\proj\a b.h`, `C:\Users\JRGEN~1\proj\AB~1.H`}, // the whole path exists
		{`C:\Users\JXrgen\proj\new.o`, `C:\Users\JRGEN~1\proj\new.o`},  // the folder exists, the file does not
		{`C:/Users/JXrgen/proj/new.o`, `C:\Users\JRGEN~1\proj/new.o`},  // forward slashes
		{`C:\Users\JXrgen\none\x.h`, `C:\Users\JRGEN~1\none\x.h`},      // only the user folder exists
		{`C:\Users\JXrgen\proj\Xnew.o`, `C:\Users\JXrgen\proj\Xnew.o`}, // the new part is unsafe: no safe form
		{`D:\NoShortNames\X\f.h`, `D:\NoShortNames\X\f.h`},             // no short name at all
	} {
		if got := Compiler(c.in); got != c.want {
			t.Errorf("Compiler(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIncludeSpelling(t *testing.T) {
	fake(t, map[string]string{`C:\Users\JXrgen\inc\x.h`: `C:\Users\JRGEN~1\inc\Y.H`})
	for _, c := range []struct{ in, want string }{
		{`"C:/Users/inc/x.h"`, `"C:/Users/inc/x.h"`},
		{`<vector>`, `<vector>`},
		{`"x.h"`, `"x.h"`},
		{`"C:\Users\JXrgen\inc\x.h"`, `"C:/Users/JRGEN~1/inc/Y.H"`}, // with slashes, inside the quotes
		{`"Xrelative.h"`, `"Xrelative.h"`},                          // a relative name has no short form
	} {
		if got := IncludeSpelling(c.in); got != c.want {
			t.Errorf("IncludeSpelling(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	if got := IncludeFile(`C:\Users\JXrgen\inc\x.h`); got != "C:/Users/JRGEN~1/inc/Y.H" {
		t.Errorf("IncludeFile = %q", got)
	}
}

func TestDisabledIsTheIdentity(t *testing.T) {
	fake(t, map[string]string{`C:\X`: `C:\X~1`})
	Enabled = false // a system other than Windows
	if !ACPSafe("X") || Compiler(`C:\X`) != `C:\X` || IncludeSpelling(`"C:\X"`) != `"C:\X"` {
		t.Error("a system other than Windows changed a path")
	}
	if _, ok := Short(`C:\X`); ok {
		t.Error("Short answered on a system other than Windows")
	}
}
