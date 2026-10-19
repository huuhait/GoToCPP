package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDedupLinkFlags(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lib twice keeps the last", "-lfoo -lm -lbar -lm", "-lfoo -lbar -lm"},
		{"-l name is one item", "-l m -lfoo -l m", "-lfoo -l m"},
		{"-l m and -lm are the same library", "-l m -lfoo -lm", "-lfoo -lm"},
		{"dir twice keeps the first", "-L/a -L/b -L/a", "-L/a -L/b"},
		{"-L dir is one item", "-L /a -L/b -L/a", "-L /a -L/b"},
		{"dir spelled differently", "-L/a/ -L/a", "-L/a/"},
		{"pthread once", "-pthread -lfoo -pthread", "-pthread -lfoo"},
		{"framework pair stays whole", "-framework Cocoa -framework IOKit -framework Cocoa", "-framework Cocoa -framework IOKit"},
		{"wl framework", "-Wl,-framework,Cocoa -lx -Wl,-framework,Cocoa", "-Wl,-framework,Cocoa -lx"},
		{"file keeps the last", "a.o libx.a b.o a.o", "libx.a b.o a.o"},
		{"file spelled with ./", "./a.o b.o a.o", "b.o a.o"},
		{"argument pairs stay whole", "-arch arm64 -lm -arch arm64", "-arch arm64 -lm -arch arm64"},
		{"-Xlinker pair", "-Xlinker -z -Xlinker -z", "-Xlinker -z -Xlinker -z"},
		{"as-needed repeated in one state", "-Wl,--as-needed -lx -Wl,--as-needed", "-Wl,--as-needed -lx"},
		{"as-needed toggled", "-Wl,--as-needed -lx -Wl,--no-as-needed -ly -Wl,--as-needed", "-Wl,--as-needed -lx -Wl,--no-as-needed -ly -Wl,--as-needed"},
		{"static and dynamic -l are different libraries", "-Wl,-Bstatic -lz -Wl,-Bdynamic -lz", "-Wl,-Bstatic -lz -Wl,-Bdynamic -lz"},
		{"same library in the same state", "-Wl,-Bstatic -lz -lm -lz", "-Wl,-Bstatic -lm -lz"},
		{"push and pop restore the state", "-lz -Wl,--push-state -Wl,-Bstatic -lz -Wl,--pop-state -lz", "-Wl,--push-state -Wl,-Bstatic -lz -Wl,--pop-state -lz"},
		{"group keeps libraries in place", "-Wl,--start-group -la -lb -la -Wl,--end-group -pthread -pthread", "-Wl,--start-group -la -lb -la -Wl,--end-group -pthread"},
		{"group dedups folders", "-L/a -Wl,--start-group -la -Wl,--end-group -L/a", "-L/a -Wl,--start-group -la -Wl,--end-group"},
		{"whole-archive is a group", "-Wl,--whole-archive -la -Wl,--no-whole-archive -la", "-Wl,--whole-archive -la -Wl,--no-whole-archive -la"},
		{"unknown flags as they come", "-Wl,-z,foo -Wl,-z,foo -fuse-ld=lld", "-Wl,-z,foo -Wl,-z,foo -fuse-ld=lld"},
		{"-z now twice", "-z now -z now -lm", "-z now -z now -lm"},
		{"-u sym twice", "-u sym -lm -u sym", "-u sym -lm -u sym"},
		{"-T script twice", "-T a.ld -lfoo -T a.ld", "-T a.ld -lfoo -T a.ld"},
		{"-rpath dir", "-rpath /x -lm -rpath /x", "-rpath /x -lm -rpath /x"},
		{"-e entry", "-e main -e main", "-e main -e main"},
		{"-soname and -h", "-soname libx.so -h libx.so", "-soname libx.so -h libx.so"},
		{"an unknown option's argument is not a file", "-weird x.o -lm -weird x.o", "-weird x.o -lm -weird x.o"},
		{"a file after the option is deduplicated again", "x.o -fno-foo x.o -lm x.o", "-fno-foo x.o -lm x.o"},
		{"a dependent before its dependency", "-lpng -lz -lm -lz", "-lpng -lm -lz"},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := strings.Fields(c.in)
			got := strings.Join(dedupLinkFlags(slices.Clone(in)), " ")
			if got != c.want {
				t.Errorf("dedupLinkFlags(%q) = %q, want %q", c.in, got, c.want)
			}
			// It is idempotent: a deduplicated line stays as it is.
			if again := strings.Join(dedupLinkFlags(strings.Fields(got)), " "); again != got {
				t.Errorf("dedupLinkFlags is not idempotent: %q -> %q", got, again)
			}
		})
	}
}

func TestDedupFiles(t *testing.T) {
	got := dedupFiles([]string{"/w/a.o", "/w/b.o", "/w/./a.o", "/w/c.a", "/w/b.o"})
	want := []string{"/w/./a.o", "/w/c.a", "/w/b.o"}
	if !slices.Equal(got, want) {
		t.Errorf("dedupFiles = %q, want %q", got, want)
	}
	if got := dedupFiles(nil); len(got) != 0 {
		t.Errorf("dedupFiles(nil) = %q", got)
	}
}

// linkGolden is the link command that the build makes for a program, for each target: a golden
// text (the order of the words is the contract of 6.9).
func TestLinkArgsGolden(t *testing.T) {
	objs := []string{"/w/a.o", "/w/b.o"}
	libObjs := []string{"/w/lib1.o", "/w/b.o"} // b.o twice: once
	rt := []string{"/c/rt/lib/k/libgocpprt.a"}
	link := []string{"-lfoo", "-L/opt/x", "-lm", "-lfoo", "-pthread"}
	for _, c := range []struct {
		name string
		cxx  CXX
		goos string
		asan bool
		lto  bool
		want string
	}{
		{"g++ linux", CXX{Family: GCC}, "linux", false, false,
			"-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fstack-clash-protection -g -gno-column-info -O0 -ffunction-sections -fdata-sections " +
				"-Wl,--gc-sections /w/a.o /w/lib1.o /w/b.o /c/rt/lib/k/libgocpprt.a -L/opt/x -lm -lfoo -pthread -o /w/out"},
		{"clang linux asan", CXX{Family: Clang}, "linux", true, false,
			"-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fstack-clash-protection -g -gno-column-info -O0 -ffunction-sections -fdata-sections " +
				"-fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all -Qunused-arguments " +
				"-Wl,--gc-sections /w/a.o /w/lib1.o /w/b.o /c/rt/lib/k/libgocpprt.a -L/opt/x -lm -lfoo -pthread -o /w/out"},
		{"g++ lto", CXX{Family: GCC}, "linux", false, true,
			"-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fstack-clash-protection -g -gno-column-info -O0 -ffunction-sections -fdata-sections -flto=auto " +
				"-Wl,--gc-sections /w/a.o /w/lib1.o /w/b.o /c/rt/lib/k/libgocpprt.a -L/opt/x -lm -lfoo -pthread -o /w/out"},
		{"clang lto", CXX{Family: Clang}, "linux", false, true,
			"-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fstack-clash-protection -g -gno-column-info -O0 -ffunction-sections -fdata-sections -Qunused-arguments -flto=thin " +
				"-Wl,--gc-sections /w/a.o /w/lib1.o /w/b.o /c/rt/lib/k/libgocpprt.a -L/opt/x -lm -lfoo -pthread -o /w/out"},
		{"darwin: no gc-sections", CXX{Family: Clang}, "darwin", false, false,
			"-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fstack-clash-protection -g -gno-column-info -O0 -ffunction-sections -fdata-sections -Qunused-arguments " +
				"/w/a.o /w/lib1.o /w/b.o /c/rt/lib/k/libgocpprt.a -L/opt/x -lm -lfoo -pthread -o /w/out"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := &Builder{CXX: c.cxx, GOOS: c.goos, ASan: c.asan, LTO: c.lto, Work: t.TempDir()}
			args, err := b.linkArgs(objs, libObjs, rt, link, "/w/out")
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(args, " "); got != c.want {
				t.Errorf("link command:\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestLinkArgsLDFlagsLastAndDeterministic(t *testing.T) {
	b := &Builder{CXX: CXX{Family: GCC}, GOOS: "linux", Work: t.TempDir(), Env: EnvFlags{LDFlags: []string{"-Wl,-z,relro", "-lextra"}}}
	objs := []string{"/w/z.o", "/w/a.o", "/w/m.o"}
	first, err := b.linkArgs(objs, nil, []string{"/rt.a"}, []string{"-lx"}, "/w/out")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		again, _ := b.linkArgs(slices.Clone(objs), nil, []string{"/rt.a"}, []string{"-lx"}, "/w/out")
		if !slices.Equal(first, again) {
			t.Fatalf("the link command changes between calls:\n%q\n%q", first, again)
		}
	}
	// The objects stay in the order given (the order of the packages), not sorted.
	i, j, k := slices.Index(first, "/w/z.o"), slices.Index(first, "/w/a.o"), slices.Index(first, "/w/m.o")
	if !(i < j && j < k) {
		t.Errorf("the objects are reordered: %q", first)
	}
	// GOCPP_LDFLAGS come after the package flags, before -o.
	n := len(first)
	if first[n-4] != "-Wl,-z,relro" || first[n-3] != "-lextra" || first[n-2] != "-o" || slices.Index(first, "-lx") > n-5 {
		t.Errorf("the environment's link flags are not after the package's: %q", first)
	}
	// The runtime comes after the objects and before the libraries.
	if rt, lx := slices.Index(first, "/rt.a"), slices.Index(first, "-lx"); !(k < rt && rt < lx) {
		t.Errorf("order objects < runtime < -l broken: %q", first)
	}
}

func TestResponseArgs(t *testing.T) {
	work := t.TempDir()
	b := &Builder{GOOS: "linux", Work: work}
	args := []string{"-g", "-O0", "/w/a.o", "/w/b c.o", "-lm", "-o", "/w/out"}

	t.Run("short command stays", func(t *testing.T) {
		got, err := b.responseArgs(slices.Clone(args), 2)
		if err != nil || !slices.Equal(got, args) {
			t.Errorf("responseArgs = %q, %v; want the command as it was", got, err)
		}
		if _, err := os.Stat(filepath.Join(work, "link.rsp")); err == nil {
			t.Error("a response file was written for a short command")
		}
	})
	t.Run("long command goes to a response file", func(t *testing.T) {
		old := responseThreshold
		responseThreshold = 10
		t.Cleanup(func() { responseThreshold = old })
		got, err := b.responseArgs(slices.Clone(args), 2)
		if err != nil {
			t.Fatal(err)
		}
		rsp := filepath.Join(work, "link.rsp")
		want := []string{"-g", "-O0", "@" + rsp, "-o", "/w/out"}
		if !slices.Equal(got, want) {
			t.Errorf("responseArgs = %q, want %q", got, want)
		}
		data, err := os.ReadFile(rsp)
		if err != nil {
			t.Fatal(err)
		}
		if want := "/w/a.o\n\"/w/b c.o\"\n-lm\n"; string(data) != want {
			t.Errorf("response file = %q, want %q", data, want)
		}
	})
	t.Run("nothing between the driver flags and -o", func(t *testing.T) {
		old := responseThreshold
		responseThreshold = 1
		t.Cleanup(func() { responseThreshold = old })
		short := []string{"-g", "-o", "x"}
		got, err := b.responseArgs(slices.Clone(short), 1)
		if err != nil || !slices.Equal(got, short) {
			t.Errorf("responseArgs = %q, %v", got, err)
		}
	})
}

func TestRspQuote(t *testing.T) {
	for _, c := range []struct {
		in      string
		windows bool
		want    string
	}{
		{"/w/a.o", false, "/w/a.o"},
		{"/w/a b.o", false, `"/w/a b.o"`},
		{`/w/a"b.o`, false, `"/w/a\"b.o"`},
		{`/w/a\b.o`, false, `"/w/a\\b.o"`},
		{`C:\w\a.o`, true, "C:/w/a.o"},
		{`C:\w x\a.o`, true, `"C:/w x/a.o"`},
		{"", false, `""`},
		{"it's", false, `"it's"`},
	} {
		if got := rspQuote(c.in, c.windows); got != c.want {
			t.Errorf("rspQuote(%q, %v) = %q, want %q", c.in, c.windows, got, c.want)
		}
	}
}

func TestCompilerPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/g++":                           "",
		"/usr/bin/clang++":                       "",
		"/usr/bin/g++-15":                        "",
		"/usr/bin/clang++-21":                    "",
		"/x/x86_64-w64-mingw32-g++":              "x86_64-w64-mingw32-",
		"/x/aarch64-linux-gnu-g++-14":            "aarch64-linux-gnu-",
		"/x/x86_64-w64-mingw32-clang++":          "x86_64-w64-mingw32-",
		`C:\llvm\bin\x86_64-w64-mingw32-g++.exe`: "",
		"/x/c++":                                 "",
		"/x/myprog":                              "",
	} {
		// (a Windows path has no folder on Linux: the base is the whole string)
		if strings.Contains(in, `\`) {
			continue
		}
		if got := compilerPrefix(CXX{Path: in}); got != want {
			t.Errorf("compilerPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArchiveFlags(t *testing.T) {
	for goos, want := range map[string]string{"linux": "rcsD", "windows": "rcsD", "darwin": "rcs", "freebsd": "rcsD"} {
		if got := archiveFlags(goos); got != want {
			t.Errorf("archiveFlags(%q) = %q, want %q", goos, got, want)
		}
	}
}

func TestLTOFlags(t *testing.T) {
	if got := LTOFlags(CXX{Family: GCC}); !slices.Equal(got, []string{"-flto=auto"}) {
		t.Errorf("g++: %q", got)
	}
	if got := LTOFlags(CXX{Family: Clang}); !slices.Equal(got, []string{"-flto=thin"}) {
		t.Errorf("clang++: %q", got)
	}
	if got := LTOFlags(CXX{}); got != nil {
		t.Errorf("unknown family: %q", got)
	}
	// LTO goes to the compiles and to the link, and the flags without it are unchanged.
	plain := &Builder{CXX: CXX{Family: GCC}}
	lto := &Builder{CXX: CXX{Family: GCC}, LTO: true}
	if slices.Contains(plain.compileFlags(), "-flto=auto") || slices.Contains(plain.linkFlags(), "-flto=auto") {
		t.Error("-flto without LTO")
	}
	if !slices.Contains(lto.compileFlags(), "-flto=auto") || !slices.Contains(lto.linkFlags(), "-flto=auto") {
		t.Error("no -flto with LTO")
	}
	if !slices.Contains(lto.pchFlags(), "-flto=auto") {
		t.Error("the precompiled header does not know about LTO: it has the flags in its key")
	}
}
