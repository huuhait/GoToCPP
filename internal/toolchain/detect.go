package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Finding a MinGW-w64 compiler on a Windows host (Phase 14 step 14.2, ADR-081). On Linux the
// compilers for windows/amd64 are the prefixed ones, x86_64-w64-mingw32-clang++ and -g++, which no
// other compiler is called. On Windows the usual installs of MinGW-w64 put the compilers under their
// plain names, clang++ and g++, in a folder of their own, and the same names also belong to
// compilers that do not make a MinGW program: the Visual Studio LLVM's clang++ targets the MSVC
// ABI, MSYS2's own g++ (the msys environment, not mingw64) builds programs that need msys-2.0.dll,
// Cygwin's too. So a plain name is accepted after the compiler says what it builds (-dumpmachine),
// and the folders where the known installs live are looked at when PATH has none:
//
//	PATH, prefixed names      x86_64-w64-mingw32-clang++, x86_64-w64-mingw32-g++     (llvm-mingw, Linux distributions)
//	PATH, plain names         clang++, g++ whose -dumpmachine is x86_64 MinGW           (WinLibs, w64devkit, MSYS2's shell)
//	MSYS2                     <root>\ucrt64\bin, clang64\bin, mingw64\bin               (root: MSYS2_ROOT, C:\msys64, C:\msys2)
//	Chocolatey                <ChocolateyInstall>\lib\mingw\tools\install\mingw64\bin and \bin (its shims)
//	Scoop                     <SCOOP>\apps\{mingw,mingw-winlibs,llvm-mingw,gcc}\current\bin and \shims
//	WinGet                    %LOCALAPPDATA%\Microsoft\WinGet\Packages\*WinLibs*\mingw64\bin
//	common unpack folders     C:\mingw64\bin, C:\w64devkit\bin, C:\winlibs\mingw64\bin, C:\Program Files\mingw-w64\*\mingw64\bin
//
// A compiler of PATH always wins over a folder that PATH does not name: the user's PATH is their
// choice. The folders are listed by knownFolders, the whole decision is in Config.local, and both
// take their facts (environment, file system, the compiler's answer) from the Config, so the tests
// run it on fake layouts.

// compilerNamesWindows are the compilers of a folder in the order that they are preferred: the
// prefixed names (never another target), clang before gcc, then the plain names.
var compilerNamesWindows = []string{"x86_64-w64-mingw32-clang++", "x86_64-w64-mingw32-g++", "clang++", "g++"}

// A folder is a place where a MinGW-w64 install keeps its compilers.
type folder struct {
	dir    string
	source string // for Found.Source: "MSYS2 ucrt64"
}

// knownFolders returns the folders of the known installs (see the table above) that exist, in the
// order that they are tried. getenv, glob and isDir are the Config's.
func knownFolders(getenv func(string) string, glob func(string) []string, isDir func(string) bool) []folder {
	var out []folder
	add := func(dir, source string) {
		if dir != "" && isDir(dir) {
			out = append(out, folder{filepath.Clean(dir), source})
		}
	}
	// MSYS2: UCRT first (the C runtime that the pinned toolchain has too), then clang, then msvcrt.
	roots := []string{getenv("MSYS2_ROOT"), getenv("MSYS64"), `C:\msys64`, `C:\msys2`}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" || seen[strings.ToLower(root)] {
			continue
		}
		seen[strings.ToLower(root)] = true
		for _, env := range []string{"ucrt64", "clang64", "mingw64"} {
			add(filepath.Join(root, env, "bin"), "MSYS2 "+env)
		}
	}
	if choco := firstOf(getenv("ChocolateyInstall"), `C:\ProgramData\chocolatey`); choco != "" {
		add(filepath.Join(choco, "lib", "mingw", "tools", "install", "mingw64", "bin"), "Chocolatey mingw")
		add(filepath.Join(choco, "bin"), "Chocolatey shims")
	}
	home := firstOf(getenv("USERPROFILE"), getenv("HOME"))
	scoop := firstOf(getenv("SCOOP"), filepath.Join(home, "scoop"))
	if scoop != "" && scoop != "scoop" {
		for _, app := range []string{"mingw", "mingw-winlibs", "mingw-winlibs-llvm-ucrt", "llvm-mingw", "gcc"} {
			add(filepath.Join(scoop, "apps", app, "current", "bin"), "Scoop "+app)
		}
		add(filepath.Join(scoop, "shims"), "Scoop shims")
	}
	if local := getenv("LOCALAPPDATA"); local != "" {
		for _, d := range glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*WinLibs*", "mingw64", "bin")) {
			add(d, "WinGet WinLibs")
		}
	}
	for _, d := range []string{`C:\mingw64\bin`, `C:\w64devkit\bin`, `C:\winlibs\mingw64\bin`} {
		add(d, "MinGW-w64 in "+filepath.Dir(d))
	}
	for _, d := range glob(`C:\Program Files\mingw-w64\*\mingw64\bin`) {
		add(d, "MinGW-w64 in "+filepath.Dir(d))
	}
	return out
}

func firstOf(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// MingwMachine reports whether the answer of a compiler to -dumpmachine names an x86-64 MinGW-w64
// target: GCC says x86_64-w64-mingw32 (or x86_64-pc-mingw32), clang x86_64-w64-windows-gnu. The
// answers that say no: x86_64-pc-windows-msvc (Visual Studio's LLVM), x86_64-pc-msys and
// x86_64-pc-cygwin (programs that need the msys or cygwin DLL), i686-w64-mingw32 (32 bit).
func MingwMachine(machine string) bool {
	m := strings.ToLower(strings.TrimSpace(machine))
	if !strings.HasPrefix(m, "x86_64-") && !strings.HasPrefix(m, "amd64-") {
		return false
	}
	return strings.Contains(m, "mingw") || strings.HasSuffix(m, "windows-gnu")
}

// machineOf asks the compiler prog what it builds: its -dumpmachine, with a time limit. It runs
// with a clean LC_ALL, like the other probes of the compiler.
func machineOf(ctx context.Context, prog string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, "-dumpmachine")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s -dumpmachine: %w", prog, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Config) getenv(k string) string {
	if c.Getenv != nil {
		return c.Getenv(k)
	}
	return os.Getenv(k)
}

func (c *Config) isFile(p string) bool {
	if c.IsFile != nil {
		return c.IsFile(p)
	}
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func (c *Config) isDir(p string) bool {
	if c.IsDir != nil {
		return c.IsDir(p)
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func (c *Config) glob(pattern string) []string {
	if c.Glob != nil {
		return c.Glob(pattern)
	}
	m, _ := filepath.Glob(pattern)
	return m
}

func (c *Config) machine(ctx context.Context, prog string) (string, error) {
	if c.Machine != nil {
		return c.Machine(prog)
	}
	return machineOf(ctx, prog)
}

// isWindowsHost reports whether the system that runs gocpp is Windows.
func (c *Config) isWindowsHost() bool { return strings.HasPrefix(c.host(), "windows/") }

// local looks for a MinGW-w64 compiler that is not gocpp's own download: the prefixed names in
// PATH (every host), and on a Windows host the plain names in PATH and the known folders.
func (c *Config) local(ctx context.Context, names []string) (Found, bool) {
	for _, n := range names {
		if p, err := c.lookPath(n); err == nil {
			return Found{Path: p, Source: "PATH"}, true
		}
	}
	if !c.isWindowsHost() {
		return Found{}, false
	}
	for _, n := range compilerNamesWindows {
		if slices.Contains(names, n) {
			continue // looked up above
		}
		if p, err := c.lookPath(n); err == nil && c.targetsMingw(ctx, p) {
			return Found{Path: p, Source: "PATH"}, true
		}
	}
	for _, f := range knownFolders(c.getenv, c.glob, c.isDir) {
		for _, n := range compilerNamesWindows {
			p := filepath.Join(f.dir, n+".exe")
			if !c.isFile(p) {
				continue
			}
			if strings.HasPrefix(n, "x86_64-w64-mingw32-") || c.targetsMingw(ctx, p) {
				return Found{Path: p, Source: f.source + " (" + f.dir + ")"}, true
			}
		}
	}
	return Found{}, false
}

// targetsMingw asks the compiler p what it builds, and says yes when that is x86-64 MinGW-w64.
func (c *Config) targetsMingw(ctx context.Context, p string) bool {
	m, err := c.machine(ctx, p)
	return err == nil && MingwMachine(m)
}

// FindTool looks for a program that comes with a MinGW-w64 install but is not a compiler (gdb, lldb,
// lldb-dap) on a Windows host (Phase 14 step 14.6): in PATH first, then in the folders of the known
// installs (knownFolders: MSYS2, Chocolatey, Scoop, WinGet's WinLibs, the usual unpack folders), then
// in the bin folder of gocpp's own llvm-mingw when it is unpacked (it has lldb, no gdb). The user's
// PATH always wins. source says where it was found ("PATH", "MSYS2 ucrt64 (C:\msys64\ucrt64\bin)");
// ok is false when there is none. On other hosts only PATH is looked at.
func (c *Config) FindTool(name string) (path, source string, ok bool) {
	if p, err := c.lookPath(name); err == nil {
		return p, "PATH", true
	}
	if !c.isWindowsHost() {
		return "", "", false
	}
	for _, f := range knownFolders(c.getenv, c.glob, c.isDir) {
		p := filepath.Join(f.dir, name+".exe")
		if c.isFile(p) {
			return p, f.source + " (" + f.dir + ")", true
		}
	}
	if dir := c.Dir(); dir != "" {
		if p, found := installed(dir, name); found {
			return p, "pinned llvm-mingw " + Release + " (cached)", true
		}
	}
	return "", "", false
}
