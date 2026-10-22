//go:build linux

package rtfiles_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The runtime must not reference libstdc++'s locale and stream machinery (job 15.fmtsize). A
// std::ostringstream in fmt.cpp made every Windows program (libstdc++ is linked statically there)
// carry about 750 KB of locale facets (time_get, money_get, num_put, wlocale-inst, ...): the
// hello world was 1.4 MB, and 0.44 MB without it. Nothing in the runtime needs a stream: the
// one function that does (stream_text, which prints a C++ value that has only an operator<<)
// is in src/fmtstream.cpp, an archive member that the linker takes only for a program that
// prints such a value.

// localeFragments are pieces of the demangled names of the machinery. An object of the runtime
// whose undefined symbols (the names it needs from the library) contain one of them drags it in.
var localeFragments = []string{
	"std::locale",
	"std::ios_base",
	"std::basic_ios",
	"std::basic_ostream",
	"std::basic_istream",
	"std::basic_iostream",
	"std::basic_streambuf",
	"std::basic_stringbuf",
	"basic_ostringstream",
	"basic_istringstream",
	"basic_stringstream",
	"std::time_get",
	"std::time_put",
	"std::money_get",
	"std::money_put",
	"std::num_get",
	"std::num_put",
	"std::ctype",
	"std::codecvt",
	"std::numpunct",
	"std::collate",
	"std::messages",
	"std::__use_facet",
	"std::__throw_ios_failure",
	"std::cout",
	"std::cerr",
	"std::cin",
	"std::ios_base::Init",
}

// streamFiles are the sources that may use streams: fmtstream.cpp, whose reason is above.
var streamFiles = map[string]bool{"src/fmtstream.cpp": true}

// floatFragments are the names of libstdc++'s floating-point std::to_chars: 190 KB in a program
// that links libstdc++ statically, and the reason fmt.cpp has its own digits on Windows
// (src/ftoa.hpp). Only src/ftoa_std.cpp may call them, and the Windows program does not link it.
var floatFragments = []string{
	"std::to_chars(char*, char*, double",
	"std::to_chars(char*, char*, float",
	"std::to_chars(char*, char*, long double",
}

// undefinedNames returns the demangled undefined symbols of the object obj.
func undefinedNames(t *testing.T, nm, obj string) []string {
	t.Helper()
	out, _, code, err := command(".", nil, true, nm, "-u", "-C", obj)
	if err != nil || code != 0 {
		t.Fatalf("%s -u -C %s: exit %d %v\n%s", nm, obj, code, err, out)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f := strings.SplitN(strings.TrimSpace(line), " ", 2); len(f) == 2 {
			names = append(names, f[1])
		}
	}
	return names
}

// checkNoLocale fails for each undefined name of each object of the runtime that is a fragment
// of the locale machinery, and, when noFloatChars, of libstdc++'s floating-point to_chars.
func checkNoLocale(t *testing.T, c cxx, nm string, more []string, noFloatChars bool) {
	t.Helper()
	rt := extractRuntime(t)
	objs, err := c.runtimeObjects(rt, more)
	if err != nil {
		t.Fatal(err)
	}
	for i, src := range rtfiles.Sources(c.targetOS()) {
		if streamFiles[src] {
			continue
		}
		fragments := localeFragments
		if noFloatChars && src != "src/ftoa_std.cpp" {
			fragments = append(append([]string{}, fragments...), floatFragments...)
		}
		for _, name := range undefinedNames(t, nm, objs[i]) {
			for _, f := range fragments {
				if strings.Contains(name, f) {
					t.Errorf("%s needs %q: it drags libstdc++'s locale machinery into every program (see locale_test.go)", src, name)
					break
				}
			}
		}
	}
}

func TestRuntimeUsesNoLocaleMachinery(t *testing.T) {
	t.Parallel()
	nm, err := exec.LookPath("nm")
	if err != nil {
		t.Skipf("no nm: %v", err)
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			checkNoLocale(t, c, nm, []string{"-O0"}, false)
		})
	}
	t.Run("windows", func(t *testing.T) {
		t.Parallel()
		c := mingw(t)
		wnm, err := exec.LookPath("x86_64-w64-mingw32-nm")
		if err != nil {
			wineSkip(t, "no x86_64-w64-mingw32-nm: "+err.Error())
		}
		checkNoLocale(t, c, wnm, winExtraFlags, true)
	})
}

// The test finds what it is for: an object that uses a stream is reported.
func TestLocaleCheckFindsAStream(t *testing.T) {
	t.Parallel()
	nm, err := exec.LookPath("nm")
	if err != nil {
		t.Skipf("no nm: %v", err)
	}
	var c cxx
	for _, cc := range compilers(t) {
		if cc.missing == "" {
			c = cc
			break
		}
	}
	if c.cc.Path == "" {
		t.Skip("no C++ compiler")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "stream.cpp")
	text := "#include <sstream>\nstd::string f(int x) { std::ostringstream os; os << x; return os.str(); }\n"
	if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(dir, "stream.o")
	if err := c.compileCmd(prioRT, dir, nil, c.flags([]string{"-O0", "-c", src, "-o", obj})); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range undefinedNames(t, nm, obj) {
		for _, f := range localeFragments {
			if strings.Contains(name, f) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("a std::ostringstream is not found in the undefined symbols %v", undefinedNames(t, nm, obj))
	}
}
