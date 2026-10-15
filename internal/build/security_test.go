// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gocpp: this file is cmd/go/internal/work/security_test.go of Go 1.25.14,
// with the cases of splitPkgConfigOutput from build_test.go. The changes:
// package build; the overrides are GOCPP_TEST_ALLOW and GOCPP_TEST_DISALLOW
// (t.Setenv); no TestCheckCompilerFlagsForInternalLink (gocpp has no internal
// linking); -mabi=lp64d and -mdouble-float are not good compiler flags, as
// gocpp refuses the flags that change the ABI (refused.go); gocpp's own
// cases at the end (TestCheckCxxFlags, TestRefusedFlags, TestAbsPaths).

package build

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"testing"
)

var goodCompilerFlags = [][]string{
	{"-DFOO"},
	{"-Dfoo=bar"},
	{"-Ufoo"},
	{"-Ufoo1"},
	{"-F/Qt"},
	{"-F", "/Qt"},
	{"-I/"},
	{"-I/etc/passwd"},
	{"-I."},
	{"-O"},
	{"-O2"},
	{"-Osmall"},
	{"-W"},
	{"-Wall"},
	{"-Wp,-Dfoo=bar"},
	{"-Wp,-Ufoo"},
	{"-Wp,-Dfoo1"},
	{"-Wp,-Ufoo1"},
	{"-flto"},
	{"-fobjc-arc"},
	{"-fno-objc-arc"},
	{"-fomit-frame-pointer"},
	{"-fno-omit-frame-pointer"},
	{"-fpic"},
	{"-fno-pic"},
	{"-fPIC"},
	{"-fno-PIC"},
	{"-fpie"},
	{"-fno-pie"},
	{"-fPIE"},
	{"-fno-PIE"},
	{"-fsplit-stack"},
	{"-fno-split-stack"},
	{"-fstack-xxx"},
	{"-fno-stack-xxx"},
	{"-fsanitize=hands"},
	{"-ftls-model=local-dynamic"},
	{"-g"},
	{"-ggdb"},
	{"-march=souza"},
	{"-mcmodel=medium"},
	{"-mcpu=123"},
	{"-mfpu=123"},
	{"-mtls-dialect=gnu"},
	{"-mtls-dialect=gnu2"},
	{"-mtls-dialect=trad"},
	{"-mtls-dialect=desc"},
	{"-mtls-dialect=xyz"},
	{"-msimd=lasx"},
	{"-msimd=xyz"},
	{"-mrelax"},
	{"-mstrict-align"},
	{"-mlsx"},
	{"-mlasx"},
	{"-mfrecipe"},
	{"-mlam-bh"},
	{"-mlamcas"},
	{"-mld-seq-sa"},
	{"-mno-relax"},
	{"-mno-strict-align"},
	{"-mno-lsx"},
	{"-mno-lasx"},
	{"-mno-frecipe"},
	{"-mno-lam-bh"},
	{"-mno-lamcas"},
	{"-mno-ld-seq-sa"},
	{"-mlarge-data-threshold=16"},
	{"-mtune=happybirthday"},
	{"-mstack-overflow"},
	{"-mno-stack-overflow"},
	{"-mmacosx-version"},
	{"-mnop-fun-dllimport"},
	{"-pthread"},
	{"-std=c99"},
	{"-xc"},
	{"-D", "FOO"},
	{"-D", "foo=bar"},
	{"-I", "."},
	{"-I", "/etc/passwd"},
	{"-I", "世界"},
	{"-I", "=/usr/include/libxml2"},
	{"-I", "dir"},
	{"-I", "$SYSROOT/dir"},
	{"-isystem", "/usr/include/mozjs-68"},
	{"-include", "/usr/include/mozjs-68/RequiredDefines.h"},
	{"-framework", "Chocolate"},
	{"-x", "c"},
	{"-v"},
}

var badCompilerFlags = [][]string{
	{"-D@X"},
	{"-D-X"},
	{"-Ufoo=bar"},
	{"-F@dir"},
	{"-F-dir"},
	{"-I@dir"},
	{"-I-dir"},
	{"-O@1"},
	{"-Wa,-foo"},
	{"-W@foo"},
	{"-Wp,-DX,-D@X"},
	{"-Wp,-UX,-U@X"},
	{"-g@gdb"},
	{"-g-gdb"},
	{"-march=@dawn"},
	{"-march=-dawn"},
	{"-mcmodel=@model"},
	{"-mfpu=@0"},
	{"-mfpu=-0"},
	{"-mlarge-data-threshold=@12"},
	{"-mtls-dialect=@gnu"},
	{"-mtls-dialect=-gnu"},
	{"-msimd=@none"},
	{"-msimd=-none"},
	{"-std=@c99"},
	{"-std=-c99"},
	{"-x@c"},
	{"-x-c"},
	{"-D", "@foo"},
	{"-D", "-foo"},
	{"-I", "@foo"},
	{"-I", "-foo"},
	{"-I", "=@obj"},
	{"-include", "@foo"},
	{"-framework", "-Caffeine"},
	{"-framework", "@Home"},
	{"-x", "--c"},
	{"-x", "@obj"},
}

func TestCheckCompilerFlags(t *testing.T) {
	for _, f := range goodCompilerFlags {
		if err := checkCompilerFlags("test", "test", f); err != nil {
			t.Errorf("unexpected error for %q: %v", f, err)
		}
	}
	for _, f := range badCompilerFlags {
		if err := checkCompilerFlags("test", "test", f); err == nil {
			t.Errorf("missing error for %q", f)
		}
	}
}

var goodLinkerFlags = [][]string{
	{"-Fbar"},
	{"-lbar"},
	{"-Lbar"},
	{"-fpic"},
	{"-fno-pic"},
	{"-fPIC"},
	{"-fno-PIC"},
	{"-fpie"},
	{"-fno-pie"},
	{"-fPIE"},
	{"-fno-PIE"},
	{"-fsanitize=hands"},
	{"-g"},
	{"-ggdb"},
	{"-march=souza"},
	{"-mcpu=123"},
	{"-mfpu=123"},
	{"-mtune=happybirthday"},
	{"-pic"},
	{"-pthread"},
	{"-Wl,--hash-style=both"},
	{"-Wl,-rpath,foo"},
	{"-Wl,-rpath,$ORIGIN/foo"},
	{"-Wl,-R", "/foo"},
	{"-Wl,-R", "foo"},
	{"-Wl,-R,foo"},
	{"-Wl,--just-symbols=foo"},
	{"-Wl,--just-symbols,foo"},
	{"-Wl,--warn-error"},
	{"-Wl,--no-warn-error"},
	{"foo.so"},
	{"_世界.dll"},
	{"./x.o"},
	{"libcgosotest.dylib"},
	{"-F", "framework"},
	{"-l", "."},
	{"-l", "/etc/passwd"},
	{"-l", "世界"},
	{"-L", "framework"},
	{"-framework", "Chocolate"},
	{"-v"},
	{"-Wl,-sectcreate,__TEXT,__info_plist,${SRCDIR}/Info.plist"},
	{"-Wl,-framework", "-Wl,Chocolate"},
	{"-Wl,-framework,Chocolate"},
	{"-Wl,-unresolved-symbols=ignore-all"},
	{"-Wl,-z,relro"},
	{"-Wl,-z,relro,-z,now"},
	{"-Wl,-z,now"},
	{"-Wl,-z,noexecstack"},
	{"libcgotbdtest.tbd"},
	{"./libcgotbdtest.tbd"},
	{"-Wl,--push-state"},
	{"-Wl,--pop-state"},
	{"-Wl,--push-state,--as-needed"},
	{"-Wl,--push-state,--no-as-needed,-Bstatic"},
	{"-Wl,--just-symbols,."},
	{"-Wl,-framework,."},
	{"-Wl,-rpath,."},
	{"-Wl,-rpath-link,."},
	{"-Wl,-sectcreate,.,.,."},
	{"-Wl,-syslibroot,."},
	{"-Wl,-undefined,."},
}

var badLinkerFlags = [][]string{
	{"-DFOO"},
	{"-Dfoo=bar"},
	{"-W"},
	{"-Wall"},
	{"-fobjc-arc"},
	{"-fno-objc-arc"},
	{"-fomit-frame-pointer"},
	{"-fno-omit-frame-pointer"},
	{"-fsplit-stack"},
	{"-fno-split-stack"},
	{"-fstack-xxx"},
	{"-fno-stack-xxx"},
	{"-mstack-overflow"},
	{"-mno-stack-overflow"},
	{"-mnop-fun-dllimport"},
	{"-std=c99"},
	{"-xc"},
	{"-D", "FOO"},
	{"-D", "foo=bar"},
	{"-I", "FOO"},
	{"-L", "@foo"},
	{"-L", "-foo"},
	{"-x", "c"},
	{"-D@X"},
	{"-D-X"},
	{"-I@dir"},
	{"-I-dir"},
	{"-O@1"},
	{"-Wa,-foo"},
	{"-W@foo"},
	{"-g@gdb"},
	{"-g-gdb"},
	{"-march=@dawn"},
	{"-march=-dawn"},
	{"-std=@c99"},
	{"-std=-c99"},
	{"-x@c"},
	{"-x-c"},
	{"-D", "@foo"},
	{"-D", "-foo"},
	{"-I", "@foo"},
	{"-I", "-foo"},
	{"-l", "@foo"},
	{"-l", "-foo"},
	{"-framework", "-Caffeine"},
	{"-framework", "@Home"},
	{"-Wl,-framework,-Caffeine"},
	{"-Wl,-framework", "-Wl,@Home"},
	{"-Wl,-framework", "@Home"},
	{"-Wl,-framework,Chocolate,@Home"},
	{"-Wl,--hash-style=foo"},
	{"-x", "--c"},
	{"-x", "@obj"},
	{"-Wl,-rpath,@foo"},
	{"-Wl,-R,foo,bar"},
	{"-Wl,-R,@foo"},
	{"-Wl,--just-symbols,@foo"},
	{"../x.o"},
	{"-Wl,-R,"},
	{"-Wl,-O"},
	{"-Wl,-e="},
	{"-Wl,-e,"},
	{"-Wl,-R,-flag"},
	{"-Wl,--push-state,"},
	{"-Wl,--push-state,@foo"},
	{"-fplugin=./-Wl,--push-state,-R.so"},
	{"./-Wl,--push-state,-R.c"},
}

func TestCheckLinkerFlags(t *testing.T) {
	for _, f := range goodLinkerFlags {
		if err := checkLinkerFlags("test", "test", f); err != nil {
			t.Errorf("unexpected error for %q: %v", f, err)
		}
	}
	for _, f := range badLinkerFlags {
		if err := checkLinkerFlags("test", "test", f); err == nil {
			t.Errorf("missing error for %q", f)
		}
	}
}

func TestCheckFlagAllowDisallow(t *testing.T) {
	if err := checkCompilerFlags("TEST", "test", []string{"-disallow"}); err == nil {
		t.Fatalf("missing error for -disallow")
	}
	t.Setenv("GOCPP_TEST_ALLOW", "-disallo")
	if err := checkCompilerFlags("TEST", "test", []string{"-disallow"}); err == nil {
		t.Fatalf("missing error for -disallow with GOCPP_TEST_ALLOW=-disallo")
	}
	t.Setenv("GOCPP_TEST_ALLOW", "-disallow")
	if err := checkCompilerFlags("TEST", "test", []string{"-disallow"}); err != nil {
		t.Fatalf("unexpected error for -disallow with GOCPP_TEST_ALLOW=-disallow: %v", err)
	}
	t.Setenv("GOCPP_TEST_ALLOW", "")

	if err := checkCompilerFlags("TEST", "test", []string{"-Wall"}); err != nil {
		t.Fatalf("unexpected error for -Wall: %v", err)
	}
	t.Setenv("GOCPP_TEST_DISALLOW", "-Wall")
	if err := checkCompilerFlags("TEST", "test", []string{"-Wall"}); err == nil {
		t.Fatalf("missing error for -Wall with GOCPP_TEST_DISALLOW=-Wall")
	}
	t.Setenv("GOCPP_TEST_ALLOW", "-Wall") // disallow wins
	if err := checkCompilerFlags("TEST", "test", []string{"-Wall"}); err == nil {
		t.Fatalf("missing error for -Wall with GOCPP_TEST_DISALLOW=-Wall and GOCPP_TEST_ALLOW=-Wall")
	}

	t.Setenv("GOCPP_TEST_ALLOW", "-fplugin.*")
	t.Setenv("GOCPP_TEST_DISALLOW", "-fplugin=lint.so")
	if err := checkCompilerFlags("TEST", "test", []string{"-fplugin=faster.so"}); err != nil {
		t.Fatalf("unexpected error for -fplugin=faster.so: %v", err)
	}
	if err := checkCompilerFlags("TEST", "test", []string{"-fplugin=lint.so"}); err == nil {
		t.Fatalf("missing error for -fplugin=lint.so: %v", err)
	}
}

func TestSplitPkgConfigOutput(t *testing.T) {
	for _, test := range []struct {
		in   []byte
		want []string
	}{
		{[]byte(`-r:foo -L/usr/white\ space/lib -lfoo\ bar -lbar\ baz`), []string{"-r:foo", "-L/usr/white space/lib", "-lfoo bar", "-lbar baz"}},
		{[]byte(`-lextra\ fun\ arg\\`), []string{`-lextra fun arg\`}},
		{[]byte("\textra     whitespace\r\n"), []string{"extra", "whitespace\r"}},
		{[]byte("     \r\n      "), []string{"\r"}},
		{[]byte(`"-r:foo" "-L/usr/white space/lib" "-lfoo bar" "-lbar baz"`), []string{"-r:foo", "-L/usr/white space/lib", "-lfoo bar", "-lbar baz"}},
		{[]byte(`"-lextra fun arg\\"`), []string{`-lextra fun arg\`}},
		{[]byte(`"     \r\n\      "`), []string{`     \r\n\      `}},
		{[]byte(`""`), []string{""}},
		{[]byte(``), nil},
		{[]byte(`"\\"`), []string{`\`}},
		{[]byte(`"\x"`), []string{`\x`}},
		{[]byte(`"\\x"`), []string{`\x`}},
		{[]byte(`'\\'`), []string{`\\`}},
		{[]byte(`'\x'`), []string{`\x`}},
		{[]byte(`"\\x"`), []string{`\x`}},
		{[]byte("\\\n"), nil},
		{[]byte(`-fPIC -I/test/include/foo -DQUOTED='"/test/share/doc"'`), []string{"-fPIC", "-I/test/include/foo", `-DQUOTED="/test/share/doc"`}},
		{[]byte(`-fPIC -I/test/include/foo -DQUOTED="/test/share/doc"`), []string{"-fPIC", "-I/test/include/foo", "-DQUOTED=/test/share/doc"}},
		{[]byte(`-fPIC -I/test/include/foo -DQUOTED=\"/test/share/doc\"`), []string{"-fPIC", "-I/test/include/foo", `-DQUOTED="/test/share/doc"`}},
		{[]byte(`-fPIC -I/test/include/foo -DQUOTED='/test/share/doc'`), []string{"-fPIC", "-I/test/include/foo", "-DQUOTED=/test/share/doc"}},
		{[]byte(`-DQUOTED='/te\st/share/d\oc'`), []string{`-DQUOTED=/te\st/share/d\oc`}},
		{[]byte(`-Dhello=10 -Dworld=+32 -DDEFINED_FROM_PKG_CONFIG=hello\ world`), []string{"-Dhello=10", "-Dworld=+32", "-DDEFINED_FROM_PKG_CONFIG=hello world"}},
		{[]byte(`"broken\"" \\\a "a"`), []string{"broken\"", "\\a", "a"}},
	} {
		got, err := splitPkgConfigOutput(test.in)
		if err != nil {
			t.Errorf("splitPkgConfigOutput on %#q failed with error %v", test.in, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("splitPkgConfigOutput(%#q) = %#q; want %#q", test.in, got, test.want)
		}
	}

	for _, test := range []struct {
		in   []byte
		want []string
	}{
		// broken quotation
		{[]byte(`"     \r\n      `), nil},
		{[]byte(`"-r:foo" "-L/usr/white space/lib "-lfoo bar" "-lbar baz"`), nil},
		{[]byte(`"-lextra fun arg\\`), nil},
		// broken char escaping
		{[]byte(`broken flag\`), nil},
		{[]byte(`extra broken flag \`), nil},
		{[]byte(`\`), nil},
		{[]byte(`"broken\"" "extra" \`), nil},
	} {
		got, err := splitPkgConfigOutput(test.in)
		if err == nil {
			t.Errorf("splitPkgConfigOutput(%v) = %v; haven't failed with error as expected.", test.in, got)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("splitPkgConfigOutput(%v) = %v; want %v", test.in, got, test.want)
		}
	}

}

// TestCheckCxxFlags checks the C++ flags that the plan wants (Phase 04
// README §2.9): Go's table has them, for the compile; the flags that run
// code at build time are refused.
func TestCheckCxxFlags(t *testing.T) {
	for _, f := range [][]string{
		{"-std=c++23"}, {"-std=gnu++20"}, {"-frtti"}, {"-fno-rtti"},
		{"-fexceptions"}, {"-fno-exceptions"}, {"-fsanitize=address,undefined"},
		{`-DMSG="hi"`}, {"-include", "config.h"}, {"-isystem", "/opt/include"},
	} {
		if err := checkCompilerFlags("CFLAGS", "test", f); err != nil {
			t.Errorf("unexpected error for %q: %v", f, err)
		}
	}
	for _, f := range [][]string{
		{"-fplugin=x.so"}, {"-fpass-plugin=x.so"}, {"-specs=x"},
		{"-wrapper", "x"}, {"@flags"}, {"-Xclang", "-load"}, {"-I"}, {"-fno-rtti=x"},
	} {
		if err := checkCompilerFlags("CFLAGS", "test", f); err == nil {
			t.Errorf("missing error for %q", f)
		}
	}
	for _, f := range [][]string{{"-lstdc++"}, {"-fsanitize=address"}} {
		if err := checkLinkerFlags("LDFLAGS", "test", f); err != nil {
			t.Errorf("unexpected linker error for %q: %v", f, err)
		}
	}
	for _, f := range [][]string{{"-Wl,-plugin,x.so"}, {"-lto_library"}, {"@flags"}} {
		if err := checkLinkerFlags("LDFLAGS", "test", f); err == nil {
			t.Errorf("missing linker error for %q", f)
		}
	}
	// The messages, like Go's.
	for _, c := range []struct {
		list []string
		want string
	}{
		{[]string{"-fplugin=x"}, "invalid flag in //gocpp:cflags: -fplugin=x"},
		{[]string{"-I", "-x"}, "invalid flag in //gocpp:cflags: -I -x"},
		{[]string{"-DA", "-I"}, "invalid flag in //gocpp:cflags: -I without argument"},
	} {
		if err := checkCompilerFlags("CFLAGS", "//gocpp:cflags", c.list); err == nil || err.Error() != c.want {
			t.Errorf("checkCompilerFlags(%q) = %v, want %q", c.list, err, c.want)
		}
	}
	t.Setenv("GOCPP_CFLAGS_ALLOW", "(")
	if err := checkCompilerFlags("CFLAGS", "test", []string{"-DA"}); err == nil || !regexp.MustCompile(`^parsing \$GOCPP_CFLAGS_ALLOW: error parsing regexp`).MatchString(err.Error()) {
		t.Errorf("a bad GOCPP_CFLAGS_ALLOW: %v", err)
	}
}

// TestRefusedFlags checks the flags that gocpp refuses on top of Go's
// tables (refused.go): the choices for the whole program (the system root,
// the toolchain, the linker, the target, the ABI and -stdlib=), in both lists, with
// the reason in the message; GOCPP_CFLAGS_ALLOW and GOCPP_LDFLAGS_ALLOW
// allow them. Other macros stay allowed.
func TestRefusedFlags(t *testing.T) {
	wholeProgram := [][]string{
		{"--sysroot=/x"}, {"--sysroot", "/x"}, {"-isysroot", "/x"}, {"-isysroot/x"},
		{"--gcc-toolchain=/x"}, {"-gcc-toolchain", "/x"}, {"--gcc-install-dir=/x"}, {"--prefix=/x"},
		{"--ld-path=/x"}, {"-fuse-ld=lld"}, {"-B/x"},
	}
	abi := [][]string{
		{"-m32"}, {"-m64"}, {"-target", "i686-linux-gnu"}, {"--target=i686-linux-gnu"}, {"-arch", "x86_64"},
		{"-mabi=lp64d"}, {"-mfloat-abi=soft"}, {"-msoft-float"}, {"-mdouble-float"}, {"-mms-bitfields"},
		{"-stdlib=libc++"}, {"--stdlib=libc++"},
		{"-D_GLIBCXX_DEBUG"}, {"-D", "_GLIBCXX_DEBUG"}, {"-D_GLIBCXX_DEBUG_PEDANTIC=1"}, {"-D_GLIBCXX_USE_CXX11_ABI=0"},
		{"-U_GLIBCXX_USE_CXX11_ABI"}, {"-D_LIBCPP_ABI_UNSTABLE"}, {"-D_LIBCPP_ABI_VERSION=2"}, {"-D_LIBCPP_DEBUG=1"},
		{"-Wp,-D_GLIBCXX_DEBUG"}, {"-Wp,-DX,-D_GLIBCXX_USE_CXX11_ABI=0"},
	}
	for _, f := range slices.Concat(wholeProgram, abi) {
		if err := checkCompilerFlags("CFLAGS", "test", f); err == nil {
			t.Errorf("missing error for %q", f)
		}
	}
	for _, f := range [][]string{{"--sysroot", "/x"}, {"-isysroot", "/x"}, {"-stdlib=libc++"}, {"-fuse-ld=gold"}, {"-target", "x"}, {"-arch", "x86_64"}} {
		if err := checkLinkerFlags("LDFLAGS", "test", f); err == nil {
			t.Errorf("missing linker error for %q", f)
		}
	}
	for _, f := range [][]string{
		{"-D_REENTRANT"}, {"-D_FILE_OFFSET_BITS=64"}, {"-D_GLIBCXX_ASSERTIONS"}, {"-D", "_GNU_SOURCE"},
		{"-DGLIBCXX_DEBUG"}, {"-U_FORTIFY_SOURCE"}, {"-march=x86-64-v3"}, {"-O2"}, {"-g3"},
	} {
		if err := checkCompilerFlags("CFLAGS", "test", f); err != nil {
			t.Errorf("unexpected error for %q: %v", f, err)
		}
	}
	for _, c := range []struct {
		list []string
		want string
	}{
		{[]string{"--sysroot", "/x"}, "invalid flag in //gocpp:cflags: --sysroot (a package can't choose the system root or the toolchain: they are choices for the whole program; set them in GOCPP_CFLAGS or GOCPP_LDFLAGS; $GOCPP_CFLAGS_ALLOW can allow it)"},
		{[]string{"-D", "_GLIBCXX_DEBUG"}, "invalid flag in //gocpp:cflags: -D_GLIBCXX_DEBUG (it changes the ABI, and the package's code would no longer match gocpp's runtime; $GOCPP_CFLAGS_ALLOW can allow it)"},
		{[]string{"-fuse-ld=lld"}, "invalid flag in //gocpp:cflags: -fuse-ld=lld (a package can't choose the linker: it is a choice for the whole program; set it in GOCPP_LDFLAGS; $GOCPP_CFLAGS_ALLOW can allow it)"},
		{[]string{"-stdlib=libc++"}, "invalid flag in //gocpp:cflags: -stdlib=libc++ (it selects another C++ standard library than the one gocpp's runtime is built with; $GOCPP_CFLAGS_ALLOW can allow it)"},
	} {
		if err := checkCompilerFlags("CFLAGS", "//gocpp:cflags", c.list); err == nil || err.Error() != c.want {
			t.Errorf("checkCompilerFlags(%q) = %v, want %q", c.list, err, c.want)
		}
	}
	t.Setenv("GOCPP_CFLAGS_ALLOW", `-m32|-D_GLIBCXX_DEBUG|--sysroot=/opt/root`)
	t.Setenv("GOCPP_LDFLAGS_ALLOW", `-stdlib=libc\+\+`)
	for _, f := range [][]string{{"-m32"}, {"-D", "_GLIBCXX_DEBUG"}, {"--sysroot=/opt/root"}} {
		if err := checkCompilerFlags("CFLAGS", "test", f); err != nil {
			t.Errorf("GOCPP_CFLAGS_ALLOW: unexpected error for %q: %v", f, err)
		}
	}
	if err := checkLinkerFlags("LDFLAGS", "test", []string{"-stdlib=libc++"}); err != nil {
		t.Errorf("GOCPP_LDFLAGS_ALLOW: unexpected error: %v", err)
	}
}

func TestAbsPaths(t *testing.T) {
	dir := filepath.Clean(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "pre.h"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	abs := filepath.FromSlash("/abs")
	j := func(p string) string { return filepath.Join(dir, p) }
	for _, c := range []struct {
		link     bool
		in, want []string
	}{
		{false,
			[]string{"-Iinclude", "-I", "sub/inc", "-I" + abs, "-I=/usr/include", "-I", "$SYSROOT/inc", "-Fframeworks", "-F", "fw",
				"-isystem", "inc", "-include", "pre.h", "-include", "config.h", "-D", "include", "-DX=include", "-x", "c", "-framework", "Cocoa"},
			[]string{"-I" + j("include"), "-I", j("sub/inc"), "-I" + abs, "-I=/usr/include", "-I", "$SYSROOT/inc", "-F" + j("frameworks"), "-F", j("fw"),
				"-isystem", j("inc"), "-include", j("pre.h"), "-include", "config.h", "-D", "include", "-DX=include", "-x", "c", "-framework", "Cocoa"}},
		{true,
			[]string{"-L../lib", "-L", "lib", "-Flib", "-l", "m", "-lm", "libfoo.a", "./x.o", abs + "/y.so", "-framework", "Cocoa",
				"-Wl,-rpath,lib", "-Wl,-rpath", "rel", "-Wl,-framework", "-Wl,Cocoa"},
			[]string{"-L" + j("../lib"), "-L", j("lib"), "-F" + j("lib"), "-l", "m", "-lm", j("libfoo.a"), j("x.o"), abs + "/y.so", "-framework", "Cocoa",
				"-Wl,-rpath,lib", "-Wl,-rpath", "rel", "-Wl,-framework", "-Wl,Cocoa"}},
	} {
		if got := absPaths(slices.Clone(c.in), dir, c.link); !slices.Equal(got, c.want) {
			t.Errorf("absPaths(%q, link %v) =\n%q\nwant\n%q", c.in, c.link, got, c.want)
		}
	}
}
