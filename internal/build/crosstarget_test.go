package build

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The tests of the target of a build (Phase 06 step 6.13): the flags, the names and the cache keys
// per target, and the choice of the cross compiler. They use fake compilers (cxx_test.go): no
// real MinGW-w64 or llvm-mingw is needed, so they run everywhere.

func TestTargetTable(t *testing.T) {
	if got, want := Targets(), []string{"linux/amd64", "linux/arm64", "windows/amd64"}; !slices.Equal(got, want) {
		t.Errorf("Targets() = %v, want %v", got, want)
	}
	// The returned list is a copy.
	Targets()[0] = "plan9/386"
	if SupportedTarget("plan9", "386") {
		t.Error("Targets() hands out the table itself")
	}
	for _, c := range []struct {
		goos, goarch string
		supported    bool
		triple       string
	}{
		{"linux", "amd64", true, "x86_64-linux-gnu"},
		{"linux", "arm64", true, "aarch64-linux-gnu"},
		{"windows", "amd64", true, "x86_64-w64-mingw32"},
		{"windows", "arm64", false, "aarch64-w64-mingw32"},
		{"darwin", "arm64", false, ""},
		{"linux", "riscv64", false, ""},
		{"Windows", "amd64", false, ""},
	} {
		if got := SupportedTarget(c.goos, c.goarch); got != c.supported {
			t.Errorf("SupportedTarget(%s, %s) = %v, want %v", c.goos, c.goarch, got, c.supported)
		}
		if got := TargetTriple(c.goos, c.goarch); got != c.triple {
			t.Errorf("TargetTriple(%s, %s) = %q, want %q", c.goos, c.goarch, got, c.triple)
		}
	}
	if !IsHost(runtime.GOOS, runtime.GOARCH) || IsHost("plan9", "386") {
		t.Error("IsHost is wrong")
	}
}

func TestTargetFileNames(t *testing.T) {
	for goos, want := range map[string][2]string{
		"windows": {".exe", ".dll"},
		"linux":   {"", ".so"},
		"darwin":  {"", ".so"},
	} {
		if got := [2]string{ExeSuffix(goos), SharedSuffix(goos)}; got != want {
			t.Errorf("ExeSuffix, SharedSuffix(%s) = %q, want %q", goos, got, want)
		}
	}
}

func TestTargetFlags(t *testing.T) {
	for _, c := range []struct {
		name string
		cxx  CXX
		want []string
	}{
		{"host gcc", CXX{Family: GCC, Path: "/bin/g++"}, nil},
		{"host clang", CXX{Family: Clang, Path: "/bin/clang++"}, nil},
		{"mingw g++", CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"},
			[]string{"-D_WIN32_WINNT=0x0A00"}},
		// llvm-mingw's clang++ names its target itself: no --target.
		{"llvm-mingw clang++", CXX{Family: Clang, Path: "/bin/x86_64-w64-mingw32-clang++", GOOS: "windows", GOARCH: "amd64"},
			[]string{"-D_WIN32_WINNT=0x0A00"}},
		// A plain clang++ (GOCPP_CXX=clang++) is told the target.
		{"plain clang++ for windows", CXX{Family: Clang, Path: "/bin/clang++", GOOS: "windows", GOARCH: "amd64"},
			[]string{"--target=x86_64-w64-mingw32", "-D_WIN32_WINNT=0x0A00"}},
		{"plain clang++ for arm64", CXX{Family: Clang, Path: "/bin/clang++", GOOS: "linux", GOARCH: "arm64"},
			[]string{"--target=aarch64-linux-gnu"}},
		{"arm64 gcc", CXX{Family: GCC, Path: "/bin/aarch64-linux-gnu-g++", GOOS: "linux", GOARCH: "arm64"}, nil},
		{"arm64 clang of Debian", CXX{Family: Clang, Path: "/bin/aarch64-linux-gnu-clang++", GOOS: "linux", GOARCH: "arm64"}, nil},
	} {
		if got := targetFlags(c.cxx); !slices.Equal(got, c.want) {
			t.Errorf("%s: targetFlags = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLinkFlags(t *testing.T) {
	if got := LinkFlags(CXX{Family: GCC}); got != nil {
		t.Errorf("the host's link flags = %v, want none", got)
	}
	if got := LinkFlags(CXX{Family: GCC, GOOS: "linux", GOARCH: "arm64"}); got != nil {
		t.Errorf("linux/arm64's link flags = %v, want none", got)
	}
	gcc := LinkFlags(CXX{Family: GCC, GOOS: "windows", GOARCH: "amd64"})
	clang := LinkFlags(CXX{Family: Clang, GOOS: "windows", GOARCH: "amd64"})
	for _, f := range [][]string{gcc, clang} {
		if !slices.Contains(f, "-static") || !slices.Contains(f, "-Wl,--stack,8388608") {
			t.Errorf("the Windows link flags %v lack -static or the 8 MiB stack", f)
		}
	}
	// -pthread: GCC's MinGW-w64 with a posix thread model needs it; llvm-mingw's libc++ does not.
	if !slices.Contains(gcc, "-pthread") || slices.Contains(clang, "-pthread") {
		t.Errorf("-pthread: gcc %v, clang %v: want it for gcc only", gcc, clang)
	}
}

func TestFlagsCarryTheTarget(t *testing.T) {
	win := CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"}
	host := CXX{Family: GCC, Path: "/bin/g++"}
	got := Flags(win, false)
	if last := got[len(got)-1]; last != "-D_WIN32_WINNT=0x0A00" {
		t.Errorf("Flags(windows) = %v: the target's flag should end it", got)
	}
	if hostFlags := Flags(host, false); slices.ContainsFunc(hostFlags, func(f string) bool { return strings.Contains(f, "WIN32") }) {
		t.Errorf("Flags(host) = %v has a Windows flag", hostFlags)
	}
	// The compile flags of generated code and of the runtime are the common ones plus the target's.
	if !slices.Contains(CompileFlags(win, false, false), "-D_WIN32_WINNT=0x0A00") {
		t.Error("CompileFlags(windows) lacks -D_WIN32_WINNT")
	}
}

func TestCompilerIDTarget(t *testing.T) {
	// The target is part of every cache key (stamp and object cache), and the host's keys do not
	// change when a field is added: a compiler of the host has no target.
	host := CXX{Family: GCC, Path: "/bin/g++", Version: "99"}
	win := host
	win.GOOS, win.GOARCH = "windows", "amd64"
	arm := host
	arm.GOOS, arm.GOARCH = "linux", "arm64"
	ids := map[string]string{"host": CompilerID(host), "windows": CompilerID(win), "arm64": CompilerID(arm)}
	if ids["host"] == ids["windows"] || ids["host"] == ids["arm64"] || ids["windows"] == ids["arm64"] {
		t.Errorf("the compiler ids do not tell the targets apart: %q", ids)
	}
	if strings.Contains(ids["host"], "target") {
		t.Errorf("the host's id %q has a target", ids["host"])
	}
	// The same compiler program for two architectures has two ids.
	win2 := win
	win2.GOARCH = "arm64"
	if CompilerID(win) == CompilerID(win2) {
		t.Error("windows/amd64 and windows/arm64 have the same compiler id")
	}
}

func TestFindCXXForTargetFields(t *testing.T) {
	dir := t.TempDir()
	mingw := fakeCXX(t, dir, "x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-g++ (GCC) 99.1.0", gccMacros...)
	mine := fakeCXX(t, dir, "my-c++", "my c++ 1.0", gccMacros...)
	aarch := fakeCXX(t, dir, "aarch64-linux-gnu-g++", "aarch64-linux-gnu-g++ (GCC) 99.1.0", gccMacros...)
	host := fakeCXX(t, dir, "g++", "g++ (GCC) 99.1.0", gccMacros...)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	for _, c := range []struct {
		name         string
		goos, goarch string
		gocppCXX     string
		path         string
		hostTarget   bool
	}{
		{"mingw g++", "windows", "amd64", "", mingw, false},
		{"GOCPP_CXX names the target's compiler", "windows", "arm64", "my-c++", mine, false},
		{"arm64's GNU cross g++", "linux", "arm64", "", aarch, false},
		{"the host", runtime.GOOS, runtime.GOARCH, "", host, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !c.hostTarget && c.goos == runtime.GOOS && c.goarch == runtime.GOARCH {
				t.Skip("the target is the host")
			}
			setEnv(t, dir, c.gocppCXX, "")
			got, err := FindCXXFor(context.Background(), c.goos, c.goarch)
			if err != nil || got.Path != c.path {
				t.Fatalf("FindCXXFor(%s, %s) = %+v, %v; want %s", c.goos, c.goarch, got, err, c.path)
			}
			wantOS, wantArch := c.goos, c.goarch
			if c.hostTarget {
				wantOS, wantArch = "", "" // the host's compiler carries no target
			}
			if got.GOOS != wantOS || got.GOARCH != wantArch {
				t.Errorf("target of the compiler = %q/%q, want %q/%q", got.GOOS, got.GOARCH, wantOS, wantArch)
			}
		})
	}
}

func TestFindCXXForWindowsRule(t *testing.T) {
	// windows/amd64 is chosen by GOCPP_TOOLCHAIN: local never downloads, pinned never looks in PATH,
	// auto looks in PATH first and falls back to the pinned toolchain (not cached here, and a
	// context that does not allow the download).
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	withMingw := t.TempDir()
	clang := fakeCXX(t, withMingw, "x86_64-w64-mingw32-clang++", "clang version 99.0.0", clangMacros...)
	fakeCXX(t, withMingw, "x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-g++ (GCC) 99.1.0", gccMacros...)
	without := t.TempDir()
	for _, c := range []struct {
		name, mode, path string
		want             string // the compiler, or a part of the error
		wantErr          bool
	}{
		{"auto prefers llvm-mingw's clang in PATH", "auto", withMingw, clang, false},
		{"default is auto", "", withMingw, clang, false},
		{"local finds PATH", "local", withMingw, clang, false},
		{"local without one", "local", without, "GOCPP_TOOLCHAIN=local never downloads", true},
		{"auto without one and without download", "auto", without, "the pinned toolchain is not usable", true},
		{"pinned ignores PATH", "pinned", withMingw, "the pinned toolchain is not usable", true},
		{"a mode that does not exist", "sometimes", withMingw, "GOCPP_TOOLCHAIN", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("GOCPP_TOOLCHAIN", c.mode)
			setEnv(t, c.path, "", "g++")
			got, err := FindCXXFor(context.Background(), "windows", "amd64")
			switch {
			case c.wantErr && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Errorf("FindCXXFor error = %v, want one with %q", err, c.want)
			case !c.wantErr && (err != nil || got.Path != c.want || got.GOOS != "windows"):
				t.Errorf("FindCXXFor = %+v, %v; want %s for windows", got, err, c.want)
			}
		})
	}
}

func TestToolchainConfigDownloadOnlyForABuild(t *testing.T) {
	t.Setenv("GOCPP_TOOLCHAIN", "pinned")
	t.Setenv("GOCPP_TOOLCHAIN_URL", "http://127.0.0.1:1/mirror")
	cfg, err := ToolchainConfig(context.Background())
	if err != nil || cfg.Download || cfg.BaseURL != "http://127.0.0.1:1/mirror" {
		t.Errorf("ToolchainConfig(plain context) = %+v, %v: want no download", cfg, err)
	}
	cfg, err = ToolchainConfig(WithToolchainDownload(context.Background(), nil))
	if err != nil || !cfg.Download {
		t.Errorf("ToolchainConfig(download context) = %+v, %v: want a download", cfg, err)
	}
	t.Setenv("GOCPP_TOOLCHAIN", "never")
	if _, err := ToolchainConfig(context.Background()); err == nil {
		t.Error("ToolchainConfig accepted GOCPP_TOOLCHAIN=never")
	}
}

// TestBigObjFlag: MinGW's GCC gets -Wa,-mbig-obj (a COFF object holds 32767 sections, which a program
// with many generic instances goes over), but not when the objects are LTO objects (-lto, or -flto from
// -release or the environment): GCC's linker plugin does not claim a big-object file, and
// `gocpp build -release -os windows` stopped at the link with "plugin needed to handle lto object".
func TestBigObjFlag(t *testing.T) {
	win := CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"}
	cases := []struct {
		name string
		b    *Builder
		want bool
	}{
		{"mingw g++", &Builder{CXX: win}, true},
		{"mingw g++ -lto", &Builder{CXX: win, LTO: true}, false},
		{"mingw g++ -release", &Builder{CXX: win, Env: EnvFlags{}.Layered([]string{"-O2", "-flto"}, []string{"-O2", "-flto"}, nil, nil, nil)}, false},
		{"mingw g++ GOCPP_CXXFLAGS=-flto", &Builder{CXX: win, Env: EnvFlags{CXXFlags: []string{"-flto=auto"}}}, false},
		{"mingw clang++", &Builder{CXX: CXX{Family: Clang, Path: "/bin/x86_64-w64-mingw32-clang++", GOOS: "windows", GOARCH: "amd64"}}, false},
		{"host g++", &Builder{CXX: CXX{Family: GCC, Path: "/bin/g++"}}, false},
	}
	for _, c := range cases {
		got := slices.Contains(c.b.compileFlags(), "-Wa,-mbig-obj")
		if got != c.want {
			t.Errorf("%s: -Wa,-mbig-obj in the compile flags = %v, want %v", c.name, got, c.want)
		}
	}
}
