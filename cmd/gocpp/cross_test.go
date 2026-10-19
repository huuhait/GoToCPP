package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/toolchain"
)

// The tests of the target flags (Phase 06 step 6.13, ADR-055): -os and -arch, GOCPP_OS and
// GOCPP_ARCH, GOOS and GOARCH, GOCPP_EXEC, and what a build for another target asks of the
// compiler (a fake one, so that they run without MinGW-w64 or Wine).

// noTargetEnv clears the variables that choose the target.
func noTargetEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"GOOS", "GOARCH", "GOCPP_OS", "GOCPP_ARCH", "GOCPP_EXEC"} {
		t.Setenv(k, "")
	}
}

func TestCrossResolve(t *testing.T) {
	for _, c := range []struct {
		name                 string
		flags                crossFlags
		env                  map[string]string
		goos, goarch, errMsg string
	}{
		{"the host", crossFlags{}, nil, runtime.GOOS, runtime.GOARCH, ""},
		{"flags", crossFlags{"windows", "amd64"}, nil, "windows", "amd64", ""},
		{"GOOS and GOARCH", crossFlags{}, map[string]string{"GOOS": "windows", "GOARCH": "arm64"}, "windows", "arm64", ""},
		{"GOCPP_OS wins over GOOS", crossFlags{}, map[string]string{"GOOS": "linux", "GOCPP_OS": "windows", "GOARCH": "amd64"}, "windows", "amd64", ""},
		{"GOCPP_ARCH wins over GOARCH", crossFlags{}, map[string]string{"GOOS": "linux", "GOARCH": "amd64", "GOCPP_ARCH": "arm64"}, "linux", "arm64", ""},
		{"flags win over the environment", crossFlags{"linux", "arm64"}, map[string]string{"GOCPP_OS": "windows", "GOCPP_ARCH": "amd64"}, "linux", "arm64", ""},
		{"only -os", crossFlags{os: "windows"}, map[string]string{"GOARCH": "amd64"}, "windows", "amd64", ""},
		{"a pair go does not know", crossFlags{"windows", "mips"}, nil, "", "", "unsupported GOOS/GOARCH pair windows/mips"},
		{"a bad name in the environment", crossFlags{}, map[string]string{"GOCPP_OS": "Windows", "GOCPP_ARCH": "amd64"}, "", "", "unsupported GOOS/GOARCH pair Windows/amd64"},
		// check accepts every pair that go knows, build checks it later (buildTarget).
		{"darwin/arm64 resolves", crossFlags{"darwin", "arm64"}, nil, "darwin", "arm64", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			noTargetEnv(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			goos, goarch, err := c.flags.resolve()
			if c.errMsg != "" {
				if err == nil || err.Error() != c.errMsg {
					t.Errorf("resolve() error = %v, want %q", err, c.errMsg)
				}
				return
			}
			if err != nil || goos != c.goos || goarch != c.goarch {
				t.Errorf("resolve() = %s/%s, %v; want %s/%s", goos, goarch, err, c.goos, c.goarch)
			}
		})
	}
}

func TestBuildTargetTable(t *testing.T) {
	for _, c := range []struct {
		goos, goarch string
		ok           bool
	}{
		{"linux", "amd64", true}, {"linux", "arm64", true}, {"windows", "amd64", true},
		{"windows", "arm64", false}, {"darwin", "arm64", false}, {"freebsd", "amd64", false},
	} {
		err := buildTarget(c.goos, c.goarch)
		if (err == nil) != c.ok {
			t.Errorf("buildTarget(%s, %s) = %v, want ok = %v", c.goos, c.goarch, err, c.ok)
		}
		if err != nil && !strings.Contains(err.Error(), "targets: linux/amd64, linux/arm64, windows/amd64") {
			t.Errorf("buildTarget(%s, %s) = %q: it should name the targets", c.goos, c.goarch, err)
		}
	}
}

func TestCrossRunner(t *testing.T) {
	noTargetEnv(t)
	if r, err := crossRunner(runtime.GOOS, runtime.GOARCH); r != nil || err != nil {
		t.Errorf("the host's runner = %v, %v; want none", r, err)
	}
	_, err := crossRunner("windows", "amd64")
	if runtime.GOOS != "windows" && (err == nil || !strings.Contains(err.Error(), "set GOCPP_EXEC")) {
		t.Errorf("crossRunner(windows) without GOCPP_EXEC = %v, want the hint about GOCPP_EXEC", err)
	}
	t.Setenv("GOCPP_EXEC", "wine --flag 'a b'")
	r, err := crossRunner("windows", "amd64")
	if err != nil || !slices.Equal(r, []string{"wine", "--flag", "a b"}) {
		t.Errorf("crossRunner with GOCPP_EXEC = %v, %v", r, err)
	}
	// GOCPP_EXEC applies to the host too, like go's -exec.
	if r, _ := crossRunner(runtime.GOOS, runtime.GOARCH); len(r) != 3 {
		t.Errorf("the host's runner with GOCPP_EXEC = %v", r)
	}
	t.Setenv("GOCPP_EXEC", "'wine")
	if _, err := crossRunner("windows", "amd64"); err == nil || !strings.HasPrefix(err.Error(), "GOCPP_EXEC='wine: ") {
		t.Errorf("crossRunner with a broken GOCPP_EXEC = %v", err)
	}
}

func TestExeSuffixFor(t *testing.T) {
	if exeSuffixFor("windows") != ".exe" || exeSuffixFor("linux") != "" || isHostTarget("plan9", "386") || !isHostTarget(runtime.GOOS, runtime.GOARCH) {
		t.Error("exeSuffixFor or isHostTarget is wrong")
	}
}

func TestCheckOSFlag(t *testing.T) {
	noTargetEnv(t)
	win32, err := filepath.Abs("../../examples/win32")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(win32)
	t.Setenv("GOCPP_CXX", "")
	// -os works like GOOS: windows has the example's files, linux none.
	_, _, viaEnv := func() (int, string, string) {
		t.Setenv("GOOS", "windows")
		t.Setenv("GOARCH", "amd64")
		defer t.Setenv("GOOS", "")
		defer t.Setenv("GOARCH", "")
		return gocpp("check", ".")
	}()
	code, _, viaFlag := gocpp("check", "-os", "windows", "-arch", "amd64", ".")
	if code != 1 || viaFlag != viaEnv || !strings.Contains(viaFlag, "command-line-arguments") {
		t.Errorf("gocpp check -os windows -arch amd64: exit %d, stderr %q; want what GOOS=windows prints (%q)", code, viaFlag, viaEnv)
	}
	code, _, errOut := gocpp("check", "-os", "linux", "-arch", "amd64", ".")
	if code != 1 || !strings.Contains(errOut, "build constraints exclude all .gp files") {
		t.Errorf("gocpp check -os linux: exit %d, stderr %q", code, errOut)
	}
	// The flag beats the environment.
	t.Setenv("GOOS", "linux")
	code, _, errOut = gocpp("check", "-os", "windows", "-arch", "amd64", ".")
	if code != 1 || errOut != viaEnv {
		t.Errorf("gocpp check -os windows with GOOS=linux: exit %d, stderr %q", code, errOut)
	}
	t.Setenv("GOOS", "")
	code, _, errOut = gocpp("check", "-os", "beos", ".")
	if code != 2 || !strings.HasPrefix(errOut, "gocpp: unsupported GOOS/GOARCH pair beos/") {
		t.Errorf("gocpp check -os beos: exit %d, stderr %q", code, errOut)
	}
}

func TestBuildUnsupportedTarget(t *testing.T) {
	noTargetEnv(t)
	inDir(t, map[string]string{"hello.gp": helloSrc})
	for _, args := range [][]string{
		{"build", "-os", "darwin", "-arch", "arm64", "hello.gp"},
		{"run", "-os", "darwin", "-arch", "arm64", "hello.gp"},
	} {
		r := gocppRun("", false, args...)
		want := "gocpp " + args[0] + ": building for darwin/arm64 is not supported (targets: linux/amd64, linux/arm64, windows/amd64)\n"
		if r != (result{1, "", want}) {
			t.Errorf("gocpp %s: %+v, want exit 1 and %q", strings.Join(args, " "), r, want)
		}
	}
	// emit works for every target that go knows: it compiles nothing.
	if r := gocppRun("", false, "emit", "-os", "darwin", "-arch", "arm64", "hello.gp"); r.code != 0 {
		t.Errorf("gocpp emit -os darwin -arch arm64: %+v, want exit 0", r)
	}
}

func TestRunForeignTargetNeedsExec(t *testing.T) {
	noTargetEnv(t)
	if runtime.GOOS == "windows" {
		t.Skip("the foreign target is windows")
	}
	inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "run", "-os", "windows", "hello.gp")
	// Before any build: the message names the way out.
	if r.code != 1 || r.stdout != "" || !strings.HasPrefix(r.stderr, "gocpp run: a program for windows/amd64 does not run on ") || !strings.Contains(r.stderr, "GOCPP_EXEC=wine") {
		t.Errorf("gocpp run -os windows without GOCPP_EXEC: %+v", r)
	}
}

// fakeTargetCXX writes a compiler that records its arguments and fails, and returns its path: the
// build's first compile and link commands show the target's flags.
func fakeTargetCXX(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	script := `#!/bin/sh
echo "$*" >> "$0.log"
for a in "$@"; do
	case "$a" in
	--version) echo "x-g++ (GCC) 99.1.0"; exit 0;;
	-dM) echo '#define __GNUC__ 99'; echo '#define __VERSION__ "99.1.0"'; exit 0;;
	esac
done
echo "fake: no real compile" >&2
exit 3
`
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildWindowsAsksTheCompilerForTheTarget(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	inDir(t, map[string]string{"hello.gp": helloSrc})
	cxx := fakeTargetCXX(t, t.TempDir(), "x86_64-w64-mingw32-g++")
	t.Setenv("GOCPP_CXX", cxx)
	r := gocppRun("", false, "build", "-os", "windows", "-arch", "amd64", "-o", "hello.exe", "hello.gp")
	if r.code == 0 {
		t.Fatalf("the fake compiler made a program: %+v", r)
	}
	// A runtime that does not compile is an internal error that keeps the work folder.
	if _, rest, ok := strings.Cut(r.stderr, "The work folder is kept: "); ok {
		dir, _, _ := strings.Cut(rest, "\n")
		os.RemoveAll(dir)
	}
	log, err := os.ReadFile(cxx + ".log")
	if err != nil {
		t.Fatalf("the compiler never ran: %v (%+v)", err, r)
	}
	if !strings.Contains(string(log), "-D_WIN32_WINNT=0x0A00") {
		t.Errorf("no compile of the build had -D_WIN32_WINNT=0x0A00:\n%s", log)
	}
}

func TestCleanToolchain(t *testing.T) {
	noTargetEnv(t)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("GOCPP_TOOLCHAIN", "")
	cfg, err := toolchain.FromEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	dir := cfg.Dir()
	// A finished unpack: the marker and the compiler, read-only like the real one.
	write := func(rel, content string, mode os.FileMode) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(".gocpp-toolchain", "llvm-mingw test\n", 0o444)
	write("bin/x86_64-w64-mingw32-clang++", "#!/bin/sh\n", 0o555)
	toolchain.MakeWritable(dir) // the helper below makes the folders read-only again
	for _, d := range []string{filepath.Join(dir, "bin"), dir} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { toolchain.MakeWritable(dir) })

	// -n prints and removes nothing.
	code, out, errOut := gocpp("clean", "-toolchain", "-n")
	if code != 0 || errOut != "" || !strings.HasPrefix(out, "rm -rf "+dir+" # 2 files, ") {
		t.Errorf("gocpp clean -toolchain -n: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, ok := cfg.Installed(); !ok {
		t.Fatal("-n removed the toolchain")
	}
	// -cache leaves it alone (like the module cache).
	if code, _, errOut := gocpp("clean", "-cache"); code != 0 {
		t.Errorf("gocpp clean -cache: exit %d, stderr %q", code, errOut)
	}
	if _, ok := cfg.Installed(); !ok {
		t.Fatal("-cache removed the toolchain")
	}
	code, _, errOut = gocpp("clean", "-toolchain")
	if code != 0 || errOut != "" {
		t.Errorf("gocpp clean -toolchain: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("the toolchain is still there after gocpp clean -toolchain: %v", err)
	}
	// Nothing to remove is not an error.
	if code, out, errOut := gocpp("clean", "-toolchain"); code != 0 || out != "" || errOut != "" {
		t.Errorf("gocpp clean -toolchain without one: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

// TestCheckPerTarget checks one package for several targets: the file name suffixes (_windows.gp,
// _linux.gp, _arm64.gp) and //go:build lines choose the files, so each target sees the error of
// its own file only.
func TestCheckPerTarget(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("GOCPP_CXX", "")
	inDir(t, map[string]string{
		"main.gp":        "package main\n\nfunc main() {}\n",
		"w_windows.gp":   "package main\n\nvar onWindows int = \"windows\"\n",
		"l_linux.gp":     "package main\n\nvar onLinux int = \"linux\"\n",
		"a_arm64.gp":     "package main\n\nvar onArm int = \"arm64\"\n",
		"tag.gp":         "//go:build windows && amd64\n\npackage main\n\nvar tagged int = \"tagged\"\n",
		"neg.gp":         "//go:build !windows\n\npackage main\n\nvar notWindows int = \"not windows\"\n",
		"unix_only.gp":   "//go:build unix\n\npackage main\n\nvar unixOnly int = \"unix\"\n",
		"other_plan9.gp": "package main\n\nvar plan int = \"plan9\"\n",
	})
	for _, c := range []struct {
		goos, goarch string
		want         []string // the variables with errors
	}{
		{"windows", "amd64", []string{"onWindows", "tagged"}},
		{"windows", "arm64", []string{"onArm", "onWindows"}},
		{"linux", "amd64", []string{"notWindows", "onLinux", "unixOnly"}},
		{"linux", "arm64", []string{"notWindows", "onArm", "onLinux", "unixOnly"}},
		{"plan9", "amd64", []string{"notWindows", "plan"}},
	} {
		t.Run(c.goos+"/"+c.goarch, func(t *testing.T) {
			code, _, errOut := gocpp("check", "-e", "-os", c.goos, "-arch", c.goarch, ".")
			var got []string
			for _, name := range []string{"onWindows", "onLinux", "onArm", "tagged", "notWindows", "unixOnly", "plan"} {
				if strings.Contains(errOut, "cannot use \""+map[string]string{
					"onWindows": "windows", "onLinux": "linux", "onArm": "arm64", "tagged": "tagged",
					"notWindows": "not windows", "unixOnly": "unix", "plan": "plan9"}[name]+"\"") {
					got = append(got, name)
				}
			}
			slices.Sort(got)
			if code != 1 || !slices.Equal(got, c.want) {
				t.Errorf("gocpp check -os %s -arch %s: exit %d, errors for %v, want %v\n%s", c.goos, c.goarch, code, got, c.want, errOut)
			}
		})
	}
}

func TestBuildLibraryForWindowsIsRefused(t *testing.T) {
	noTargetEnv(t)
	inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "build", "-os", "windows", "-buildmode=c-shared", "hello.gp")
	want := "gocpp build: -buildmode=c-shared for windows is not supported yet (Phase 14: .dll and import libraries)\n"
	if r != (result{1, "", want}) {
		t.Errorf("gocpp build -os windows -buildmode=c-shared = %+v, want exit 1 and %q", r, want)
	}
}

// The link line that -n (the plan) prints is the one the build runs: it has the target's link flags.
func TestDryRunShowsTheWindowsLinkFlags(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	inDir(t, map[string]string{"hello.gp": helloSrc})
	cxx := fakeTargetCXX(t, t.TempDir(), "x86_64-w64-mingw32-g++")
	t.Setenv("GOCPP_CXX", cxx)
	r := gocppRun("", false, "build", "-n", "-os", "windows", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp build -n -os windows = %+v", r)
	}
	var link string
	for _, l := range strings.Split(r.stdout+r.stderr, "\n") {
		if strings.Contains(l, " -o ") && strings.Contains(l, ".exe") {
			link = l
		}
	}
	if !strings.Contains(link, " -static ") || !strings.Contains(link, "-Wl,--stack,8388608") {
		t.Errorf("the planned link line lacks the Windows link flags:\n%s\nfull output:\n%s", link, r.stdout+r.stderr)
	}
}

// The plan that the exports read (link.Pre of recordLinkPlan) has the target's link flags too.
func TestExportCMakeHasTheWindowsLinkFlags(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	t.Setenv("GOCPP_CXX", fakeTargetCXX(t, t.TempDir(), "x86_64-w64-mingw32-g++"))
	r := gocppRun("", false, "export", "cmake", "-os", "windows", "-o", "cm", "hello.gp")
	if r.code != 0 {
		t.Fatalf("gocpp export cmake -os windows = %+v", r)
	}
	text, err := os.ReadFile(filepath.Join(dir, "cm", "CMakeLists.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "-static") || !strings.Contains(string(text), "--stack,8388608") {
		t.Errorf("CMakeLists.txt lacks the Windows link flags:\n%s", text)
	}
}

func TestBuildOfflineNeverDownloadsTheToolchain(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "pinned")
	t.Setenv("GOCPP_TOOLCHAIN_URL", "https://127.0.0.1:1/x")
	inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "build", "-os", "windows", "hello.gp")
	if r.code == 0 || strings.Contains(r.stderr, "downloading") || !strings.Contains(r.stderr, "GOCPP_OFFLINE is set") {
		t.Errorf("gocpp build with GOCPP_OFFLINE=1 and GOCPP_TOOLCHAIN=pinned = %+v: want an error about GOCPP_OFFLINE and no download", r)
	}
}

func TestCleanToolchainRemovesAnUnfinishedFolder(t *testing.T) {
	noTargetEnv(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_TOOLCHAIN", "")
	cfg, err := toolchain.FromEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	dir := cfg.Dir()
	// A folder without the compiler and the marker, read-only like a finished one.
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "bin", "junk"), []byte("x"), 0o444)
	os.Chmod(filepath.Join(dir, "bin"), 0o555)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { toolchain.MakeWritable(dir) })
	if code, _, errOut := gocpp("clean", "-toolchain"); code != 0 || errOut != "" {
		t.Errorf("gocpp clean -toolchain: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("the unfinished folder is still there: %v", err)
	}
}
