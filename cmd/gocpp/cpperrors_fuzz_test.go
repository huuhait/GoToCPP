package main

import (
	"reflect"
	"testing"

	"github.com/huuhait/gocpp/tools/fuzz/fuzzkit"
)

// FuzzCppErrors feeds the mapper of the C++ compilers' and linkers' errors (parseCXXErrors,
// cxxClassifier.userCXXErrors, parseUndefined) with text that looks like what g++, clang++, ld and
// lld print, and with any other text (Phase 15 step 15.8): it must not panic, must end, and must
// give the same answer twice.
//
//	go test -fuzz '^FuzzCppErrors$' -fuzztime 5m ./cmd/gocpp
func FuzzCppErrors(f *testing.F) {
	for _, s := range []string{
		tGP + ": In function 'void gpkg_main::main()':\n" + tGP + ":7:34: error: cannot convert 'gp::string' to 'const char*'\n    7 |  x\n      |  ^\n" +
			"In file included from /usr/include/c++/15/cstdio:47,\n                 from " + tRT + "/gocpp/ptr.hpp:89,\n                 from " + tGen + ":2:\n" +
			"/usr/include/stdio.h:366:43: note: initializing argument 1 of 'int printf(const char*, ...)'\n",
		tGP + ":7:14: error: no viable conversion from 'gp::string' to 'const char *'\n    7 |     ::printf(x);\n" +
			tRT + "/gocpp/string.hpp:77:5: note: candidate function\n1 error generated.\n",
		"In file included from " + tGP + ":3:\n" + tSys + ": In instantiation of 'constexpr void std::__insertion_sort(...) [with _RandomAccessIterator = int]':\n" +
			tSys + ":1817:25:   required from 'constexpr void std::__final_insertion_sort(...)'\n" + tGP + ":6:52:   required from here\n" +
			tSys + ":1780:17: error: no type named 'value_type' in 'struct std::iterator_traits<int>'\n",
		tGen + ":10:5: error: expected ';' after expression\n" + tGen + ":12:1: warning: unused variable 'x'\nfatal error: too many errors emitted\n",
		"/usr/bin/ld: /work/main.gp.o: in function `gpkg_main::main()':\n/src/main.gp:6:(.text+0xa): undefined reference to `zlibVersion'\ncollect2: error: ld returned 1 exit status\n",
		"ld.lld: error: undefined symbol: zlibVersion\n>>> referenced by main.gp:9\n>>>               main.o:(gpkg_main::main())\nclang++: error: linker command failed with exit code 1\n",
		"\x1b[01m\x1b[K" + tGP + ":7:34:\x1b[m\x1b[K \x1b[01;31m\x1b[Kerror: \x1b[m\x1b[Kboom\n",
		"c++: fatal error: no input files\ncompilation terminated.\n",
		"x:1:1: error: \n:::\n:0:0: note: in instantiation of\n    required from here\n",
	} {
		f.Add([]byte(s))
	}
	user := make(cppLineSet)
	for _, l := range []int{3, 4, 5, 6, 7, 8, 9, 12, 20, 30} {
		user.add(tGP, l)
	}
	classifiers := []*cxxClassifier{
		{generated: tGen, include: tRT, user: user},
		{generated: "/work", include: tRT, user: user},
		{generated: "", include: "", user: nil},
	}
	f.Fuzz(func(t *testing.T, out []byte) {
		if len(out) > 1<<20 {
			return
		}
		fuzzkit.Guard("FuzzCppErrors", func() {
			d1, d2 := parseCXXErrors(out), parseCXXErrors(out)
			if !reflect.DeepEqual(d1, d2) {
				t.Fatalf("parseCXXErrors gives two answers for one text")
			}
			for _, c := range classifiers {
				errs, others, ok := c.userCXXErrors(out)
				if ok != (len(errs) > 0) || others < 0 {
					t.Fatalf("userCXXErrors: ok=%v with %d errors, %d others", ok, len(errs), others)
				}
			}
			if r1, r2 := parseUndefined(out), parseUndefined(out); !reflect.DeepEqual(r1, r2) {
				t.Fatalf("parseUndefined gives two answers for one text")
			}
		})
	})
}
