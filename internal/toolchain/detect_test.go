package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The tests of step 14.2: finding a MinGW-w64 compiler on a Windows host, and the zip archive of
// that host. The layouts are fake (the Config's facts are functions), so they run on Linux.

func TestMingwMachine(t *testing.T) {
	for m, want := range map[string]bool{
		"x86_64-w64-mingw32":     true, // gcc
		"x86_64-pc-mingw32":      true,
		"x86_64-w64-windows-gnu": true, // clang
		"X86_64-W64-MINGW32\r\n": true,
		"x86_64-pc-windows-msvc": false, // Visual Studio's LLVM
		"x86_64-pc-msys":         false,
		"x86_64-pc-cygwin":       false,
		"i686-w64-mingw32":       false,
		"aarch64-w64-mingw32":    false,
		"x86_64-linux-gnu":       false,
		"":                       false,
	} {
		if got := MingwMachine(m); got != want {
			t.Errorf("MingwMachine(%q) = %v, want %v", m, got, want)
		}
	}
}

// drive starts the fake absolute paths of the layouts: "/msys64" is not absolute on Windows, where
// the detection ignores a variable that holds a path that is not absolute.
var drive = func() string {
	if runtime.GOOS == "windows" {
		return "C:"
	}
	return ""
}()

// A layout is a fake machine: the files that exist, the PATH, the environment, what each compiler
// says it builds.
type layout struct {
	files   []string          // the compilers and other files that exist (any case is not matched)
	dirs    []string          // folders that exist besides those of the files
	path    []string          // PATH, in order
	env     map[string]string // environment
	machine map[string]string // compiler path -> -dumpmachine
	asked   []string          // the compilers that were asked
}

func (l *layout) config() Config {
	has := func(set []string, p string) bool { return slices.Contains(set, filepath.ToSlash(p)) }
	norm := func(in []string) []string {
		var out []string
		for _, s := range in {
			out = append(out, filepath.ToSlash(s))
		}
		return out
	}
	files, dirs := norm(l.files), norm(l.dirs)
	for _, f := range files {
		dirs = append(dirs, filepath.ToSlash(filepath.Dir(f)))
	}
	return Config{
		Mode: Local, Host: "windows/amd64", Cache: "/nowhere",
		LookPath: func(name string) (string, error) {
			for _, d := range l.path {
				for _, ext := range []string{".exe"} {
					p := filepath.ToSlash(filepath.Join(d, name+ext))
					if has(files, p) {
						return p, nil
					}
				}
			}
			return "", errors.New("not found")
		},
		Getenv: func(k string) string { return l.env[k] },
		IsFile: func(p string) bool { return has(files, p) },
		IsDir:  func(p string) bool { return has(dirs, p) },
		Glob: func(pattern string) []string {
			var out []string
			for _, d := range dirs {
				if ok, _ := filepath.Match(filepath.ToSlash(pattern), d); ok && !slices.Contains(out, d) {
					out = append(out, d)
				}
			}
			return out
		},
		Machine: func(p string) (string, error) {
			p = filepath.ToSlash(p) // the layouts write '/', and a folder of the install comes with '\'
			l.asked = append(l.asked, p)
			if m, ok := l.machine[p]; ok {
				return m, nil
			}
			return "", errors.New("does not run")
		},
	}
}

func TestDetectOrder(t *testing.T) {
	const gcc, clang, msvc = "x86_64-w64-mingw32", "x86_64-w64-windows-gnu", "x86_64-pc-windows-msvc"
	for _, c := range []struct {
		name    string
		l       layout
		want    string // the compiler found, "" for none
		source  string // part of Found.Source
		notAsk  []string
		mustAsk []string
	}{
		{name: "prefixed clang in PATH wins, nothing is run",
			l:    layout{files: []string{drive + "/llvm/bin/x86_64-w64-mingw32-clang++.exe", drive + "/llvm/bin/g++.exe"}, path: []string{drive + "/llvm/bin"}, machine: map[string]string{drive + "/llvm/bin/g++.exe": gcc}},
			want: drive + "/llvm/bin/x86_64-w64-mingw32-clang++.exe", source: "PATH", notAsk: []string{drive + "/llvm/bin/g++.exe"}},
		{name: "WinLibs: plain g++ in PATH",
			l:    layout{files: []string{drive + "/winlibs/mingw64/bin/g++.exe"}, path: []string{drive + "/winlibs/mingw64/bin"}, machine: map[string]string{drive + "/winlibs/mingw64/bin/g++.exe": gcc}},
			want: drive + "/winlibs/mingw64/bin/g++.exe", source: "PATH"},
		{name: "plain clang++ of Visual Studio's LLVM is refused, g++ after it is taken",
			l: layout{files: []string{drive + "/vs/llvm/clang++.exe", drive + "/w/bin/g++.exe"}, path: []string{drive + "/vs/llvm", drive + "/w/bin"},
				machine: map[string]string{drive + "/vs/llvm/clang++.exe": msvc, drive + "/w/bin/g++.exe": gcc}},
			want: drive + "/w/bin/g++.exe", source: "PATH", mustAsk: []string{drive + "/vs/llvm/clang++.exe"}},
		{name: "MSYS2's msys g++ in PATH is refused, the folder of ucrt64 is used",
			l: layout{files: []string{drive + "/msys64/usr/bin/g++.exe", drive + "/msys64/ucrt64/bin/g++.exe"}, path: []string{drive + "/msys64/usr/bin"},
				machine: map[string]string{drive + "/msys64/usr/bin/g++.exe": "x86_64-pc-msys", drive + "/msys64/ucrt64/bin/g++.exe": gcc}, env: map[string]string{"MSYS2_ROOT": drive + "/msys64"}},
			want: drive + "/msys64/ucrt64/bin/g++.exe", source: "MSYS2 ucrt64"},
		{name: "MSYS2 prefers ucrt64, then clang64, then mingw64",
			l: layout{files: []string{drive + "/m/mingw64/bin/g++.exe", drive + "/m/clang64/bin/clang++.exe", drive + "/m/ucrt64/bin/g++.exe"},
				machine: map[string]string{drive + "/m/mingw64/bin/g++.exe": gcc, drive + "/m/clang64/bin/clang++.exe": clang, drive + "/m/ucrt64/bin/g++.exe": gcc}, env: map[string]string{"MSYS2_ROOT": drive + "/m"}},
			want: drive + "/m/ucrt64/bin/g++.exe", source: "MSYS2 ucrt64"},
		{name: "MSYS2 clang64 alone",
			l:    layout{files: []string{drive + "/m/clang64/bin/clang++.exe"}, machine: map[string]string{drive + "/m/clang64/bin/clang++.exe": clang}, env: map[string]string{"MSYS2_ROOT": drive + "/m"}},
			want: drive + "/m/clang64/bin/clang++.exe", source: "MSYS2 clang64"},
		{name: "Chocolatey's mingw",
			l: layout{files: []string{drive + "/choco/lib/mingw/tools/install/mingw64/bin/g++.exe"}, env: map[string]string{"ChocolateyInstall": drive + "/choco"},
				machine: map[string]string{drive + "/choco/lib/mingw/tools/install/mingw64/bin/g++.exe": gcc}},
			want: drive + "/choco/lib/mingw/tools/install/mingw64/bin/g++.exe", source: "Chocolatey mingw"},
		{name: "Scoop's mingw-winlibs",
			l: layout{files: []string{drive + "/u/scoop/apps/mingw-winlibs/current/bin/g++.exe"}, env: map[string]string{"USERPROFILE": drive + "/u"},
				machine: map[string]string{drive + "/u/scoop/apps/mingw-winlibs/current/bin/g++.exe": gcc}},
			want: drive + "/u/scoop/apps/mingw-winlibs/current/bin/g++.exe", source: "Scoop mingw-winlibs"},
		{name: "Scoop's llvm-mingw has the prefixed name: no question asked",
			l:    layout{files: []string{drive + "/s/apps/llvm-mingw/current/bin/x86_64-w64-mingw32-clang++.exe"}, env: map[string]string{"SCOOP": drive + "/s"}},
			want: drive + "/s/apps/llvm-mingw/current/bin/x86_64-w64-mingw32-clang++.exe", source: "Scoop llvm-mingw"},
		{name: "WinGet's WinLibs",
			l: layout{files: []string{drive + "/l/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_x/mingw64/bin/g++.exe"}, env: map[string]string{"LOCALAPPDATA": drive + "/l"},
				machine: map[string]string{drive + "/l/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_x/mingw64/bin/g++.exe": gcc}},
			want: drive + "/l/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_x/mingw64/bin/g++.exe", source: "WinGet WinLibs"},
		{name: "a known folder with a compiler of another target is refused",
			l:    layout{files: []string{drive + "/m/mingw64/bin/g++.exe"}, machine: map[string]string{drive + "/m/mingw64/bin/g++.exe": "i686-w64-mingw32"}, env: map[string]string{"MSYS2_ROOT": drive + "/m"}},
			want: ""},
		{name: "a compiler that does not run is skipped",
			l:    layout{files: []string{drive + "/w/bin/g++.exe"}, path: []string{drive + "/w/bin"}},
			want: ""},
		{name: "nothing at all", l: layout{}, want: ""},
		{name: "PATH wins over a known folder",
			l: layout{files: []string{drive + "/p/bin/g++.exe", drive + "/m/ucrt64/bin/g++.exe"}, path: []string{drive + "/p/bin"}, env: map[string]string{"MSYS2_ROOT": drive + "/m"},
				machine: map[string]string{drive + "/p/bin/g++.exe": gcc, drive + "/m/ucrt64/bin/g++.exe": gcc}},
			want: drive + "/p/bin/g++.exe", source: "PATH", notAsk: []string{drive + "/m/ucrt64/bin/g++.exe"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := c.l.config()
			got, err := Resolve(context.Background(), cfg, "windows", "amd64")
			if c.want == "" {
				if err == nil {
					t.Fatalf("found %+v, want none", got)
				}
				for _, w := range []string{"MinGW-w64", "GOCPP_CXX", "GOCPP_TOOLCHAIN=local"} {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error %q lacks %q", err, w)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if filepath.ToSlash(got.Path) != c.want || !strings.Contains(got.Source, c.source) {
				t.Errorf("found %+v, want %s from %q", got, c.want, c.source)
			}
			for _, n := range c.notAsk {
				if slices.Contains(c.l.asked, n) {
					t.Errorf("%s was asked what it builds", n)
				}
			}
			for _, n := range c.mustAsk {
				if !slices.Contains(c.l.asked, n) {
					t.Errorf("%s was not asked what it builds", n)
				}
			}
		})
	}
}

// On a Linux host only the prefixed names count: g++ and clang++ build for Linux.
func TestDetectLinuxHostKeepsPrefixedNames(t *testing.T) {
	l := layout{files: []string{"/usr/bin/g++.exe", "/usr/bin/clang++.exe"}, path: []string{"/usr/bin"},
		machine: map[string]string{"/usr/bin/g++.exe": "x86_64-w64-mingw32"}}
	cfg := l.config()
	cfg.Host = "linux/amd64"
	if f, err := Resolve(context.Background(), cfg, "windows", "amd64"); err == nil {
		t.Fatalf("found %+v on a Linux host", f)
	}
	if len(l.asked) != 0 {
		t.Errorf("a Linux host asked %v", l.asked)
	}
}

// GOCPP_TOOLCHAIN=pinned never looks at the machine.
func TestDetectPinnedIgnoresLocal(t *testing.T) {
	l := layout{files: []string{"/w/bin/g++.exe"}, path: []string{"/w/bin"}, machine: map[string]string{"/w/bin/g++.exe": "x86_64-w64-mingw32"}}
	cfg := l.config()
	cfg.Mode = Pinned
	cfg.Download = false
	if _, err := Resolve(context.Background(), cfg, "windows", "amd64"); err == nil {
		t.Fatal("pinned found a compiler without the download")
	}
	if len(l.asked) != 0 {
		t.Errorf("pinned asked %v", l.asked)
	}
}

func TestWindowsArchiveIsPinned(t *testing.T) {
	var a Archive
	for _, x := range Archives {
		if x.Host == "windows/amd64" {
			a = x
		}
	}
	if !strings.HasSuffix(a.Name, ".zip") || !strings.Contains(a.Name, "ucrt") || a.Loader != "" || a.Size < 1<<27 {
		t.Errorf("windows/amd64 archive = %+v", a)
	}
}

func zipOf(t *testing.T, entries map[string]string, order []string, modes map[string]os.FileMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if m, ok := modes[name]; ok {
			h.SetMode(m)
		} else if strings.HasSuffix(name, "/") {
			h.SetMode(os.ModeDir | 0o755)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, entries[name])
	}
	zw.Close()
	return buf.Bytes()
}

func TestUnpackZip(t *testing.T) {
	top := "llvm-mingw-x/"
	data := zipOf(t, map[string]string{top + "bin/x86_64-w64-mingw32-clang++.exe": "MZ", top + "include/a.h": "// h\n"},
		[]string{top, top + "bin/", top + "bin/x86_64-w64-mingw32-clang++.exe", top + "include/a.h"}, nil)
	root := filepath.Join(t.TempDir(), "root")
	os.Mkdir(root, 0o755)
	if err := unpackZip(bytes.NewReader(data), int64(len(data)), root); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "bin", "x86_64-w64-mingw32-clang++.exe")); err != nil || string(b) != "MZ" {
		t.Errorf("compiler = %q, %v", b, err)
	}
	if p, ok := binFile(root, compilerName); !ok || !strings.HasSuffix(p, ".exe") {
		t.Errorf("binFile = %q, %v", p, ok)
	}
}

func TestUnpackZipRejects(t *testing.T) {
	for _, c := range []struct {
		name  string
		names []string
		modes map[string]os.FileMode
		want  string
	}{
		{"dotdot", []string{"t/../evil"}, nil, ".. element"},
		{"absolute", []string{"/etc/evil"}, nil, "not a relative path"},
		{"backslash", []string{`t\evil`}, nil, "not a relative path"},
		{"two tops", []string{"a/f", "b/g"}, nil, "more than one top-level folder"},
		{"symlink", []string{"t/l"}, map[string]os.FileMode{"t/l": os.ModeSymlink | 0o777}, "not accepted"},
		{"empty", nil, nil, "empty"},
	} {
		t.Run(c.name, func(t *testing.T) {
			entries := map[string]string{}
			for _, n := range c.names {
				entries[n] = "x"
			}
			data := zipOf(t, entries, c.names, c.modes)
			root := filepath.Join(t.TempDir(), "root")
			os.Mkdir(root, 0o755)
			err := unpackZip(bytes.NewReader(data), int64(len(data)), root)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v, want %q", err, c.want)
			}
			if _, err := os.Lstat(filepath.Join(filepath.Dir(root), "evil")); err == nil {
				t.Error("a file was written outside the root")
			}
		})
	}
}

// A zip download goes the whole way: the hash is checked, the zip is unpacked, the .exe is found.
func TestDownloadZip(t *testing.T) {
	const name = "t.zip"
	top := "llvm-mingw-x/"
	body := zipOf(t, map[string]string{top + "bin/x86_64-w64-mingw32-clang++.exe": "MZ"},
		[]string{top, top + "bin/", top + "bin/x86_64-w64-mingw32-clang++.exe"}, nil)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Host = "windows/amd64"
	cfg.Archives = []Archive{{Host: "windows/amd64", Name: name, SHA256: sum(body), Size: int64(len(body))}}
	f, err := resolve(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.Source, "downloaded") || !strings.HasSuffix(f.Path, "x86_64-w64-mingw32-clang++.exe") {
		t.Errorf("found %+v", f)
	}
	again, err := resolve(t, cfg)
	if err != nil || !strings.Contains(again.Source, "cached") || s.hits.Load() != 1 {
		t.Errorf("second resolve = %+v, %v (hits %d)", again, err, s.hits.Load())
	}
	// A wrong hash unpacks nothing.
	bad := config(t, s, name, body)
	bad.Host = "windows/amd64"
	bad.Archives = []Archive{{Host: "windows/amd64", Name: name, SHA256: strings.Repeat("0", 64), Size: int64(len(body))}}
	if _, err := resolve(t, bad); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Errorf("wrong hash: %v", err)
	}
}

// A zip whose headers say less than the entry holds (a damaged archive, or one made to overflow a size
// check) is refused, and so is one that says more.
func TestUnpackZipLyingSize(t *testing.T) {
	for _, delta := range []int{-3, +5} {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.CreateHeader(&zip.FileHeader{Name: "top/f", Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, "0123456789")
		zw.Close()
		data := buf.Bytes()
		// Store with a data descriptor: the sizes are in the central directory (and the descriptor); patch every
		// 4-byte little-endian 10 that follows the CRC... simpler: patch the central directory's sizes.
		cd := bytes.Index(data, []byte("PK\x01\x02"))
		if cd < 0 {
			t.Fatal("no central directory")
		}
		for _, off := range []int{cd + 20, cd + 24} { // compressed and uncompressed size
			data[off] = byte(10 + delta)
		}
		root := filepath.Join(t.TempDir(), "root")
		os.Mkdir(root, 0o755)
		if err := unpackZip(bytes.NewReader(data), int64(len(data)), root); err == nil {
			t.Errorf("a zip that says %d bytes for a 10 byte entry was unpacked", 10+delta)
		}
	}
}

// gocpp clean -toolchain leaves nothing: not the lock file, not what a killed download left, and not the empty folder.
func TestRemoveCleansLockAndDownloads(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(cfg.Dir())
	if _, err := os.Stat(cfg.Dir() + ".lock"); err != nil {
		t.Fatalf("no lock file after the download (the test needs one): %v", err)
	}
	stale := filepath.Join(parent, ".download-2147483000-123456") // the number of a process that is not there
	os.MkdirAll(filepath.Join(stale, "root"), 0o755)
	os.WriteFile(filepath.Join(stale, "archive"), []byte("half"), 0o644)
	if _, err := cfg.Remove(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(parent); err == nil {
		t.Errorf("clean -toolchain left %d entries in %s: %v", len(entries), parent, entries)
	}
}

// A download that was killed leaves a .download-* folder; the next download, which holds the lock, removes it at once
// (it was kept for a day before, and is a gigabyte).
func TestFetchSweepsADeadDownload(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	parent := filepath.Dir(cfg.Dir())
	stale := filepath.Join(parent, ".download-2147483000-987654")
	running := filepath.Join(parent, fmt.Sprintf(".download-%d-111", os.Getpid()))
	os.MkdirAll(stale, 0o755)
	os.MkdirAll(running, 0o755)
	t.Cleanup(func() { MakeWritable(parent) })
	os.WriteFile(filepath.Join(stale, "archive"), []byte("half"), 0o644)
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("the folder of the dead download is still there")
	}
	if _, err := os.Stat(running); err != nil {
		t.Error("the folder of a download whose process lives was removed")
	}
}

// The message of a build without a compiler says GOCPP_CXX once, offline and not.
func TestNoCompilerMessageSaysGOCPPCXXOnce(t *testing.T) {
	for _, offline := range []bool{true, false} {
		l := layout{}
		cfg := l.config()
		cfg.Mode = Auto
		cfg.Offline = offline
		cfg.Cache = t.TempDir()
		cfg.Download = false
		_, err := Resolve(context.Background(), cfg, "windows", "amd64")
		if err == nil {
			t.Fatal("a compiler was found")
		}
		if n := strings.Count(err.Error(), "GOCPP_CXX"); n != 1 {
			t.Errorf("offline=%v: GOCPP_CXX is in the message %d times: %v", offline, n, err)
		}
	}
}

// The debuggers of a Windows host (step 14.6): gdb and lldb are found in PATH first, then in the
// folders of the known installs, then in gocpp's own llvm-mingw (lldb only).
func TestFindTool(t *testing.T) {
	for _, c := range []struct {
		name   string
		l      layout
		tool   string
		want   string // "" for none
		source string
	}{
		{name: "gdb in PATH", l: layout{files: []string{"/w/bin/gdb.exe"}, path: []string{"/w/bin"}},
			tool: "gdb", want: "/w/bin/gdb.exe", source: "PATH"},
		{name: "PATH wins over a known folder",
			l: layout{files: []string{"/p/gdb.exe", "/m/ucrt64/bin/gdb.exe"}, path: []string{"/p"},
				env: map[string]string{"MSYS2_ROOT": "/m"}},
			tool: "gdb", want: "/p/gdb.exe", source: "PATH"},
		{name: "gdb of MSYS2 ucrt64 when PATH has none",
			l: layout{files: []string{"/m/ucrt64/bin/gdb.exe", "/m/mingw64/bin/gdb.exe"},
				env: map[string]string{"MSYS2_ROOT": "/m"}},
			tool: "gdb", want: "/m/ucrt64/bin/gdb.exe", source: "MSYS2 ucrt64"},
		{name: "gdb of WinGet's WinLibs",
			l: layout{files: []string{"/l/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_x/mingw64/bin/gdb.exe"},
				env: map[string]string{"LOCALAPPDATA": "/l"}},
			tool:   "gdb",
			want:   "/l/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_x/mingw64/bin/gdb.exe",
			source: "WinGet WinLibs"},
		{name: "lldb of Scoop's llvm-mingw",
			l:    layout{files: []string{"/s/apps/llvm-mingw/current/bin/lldb.exe"}, env: map[string]string{"SCOOP": "/s"}},
			tool: "lldb", want: "/s/apps/llvm-mingw/current/bin/lldb.exe", source: "Scoop llvm-mingw"},
		{name: "none", l: layout{}, tool: "gdb"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := c.l.config()
			got, source, ok := cfg.FindTool(c.tool)
			if ok != (c.want != "") || filepath.ToSlash(got) != c.want || !strings.Contains(source, c.source) {
				t.Errorf("FindTool(%s) = %q, %q, %v; want %q from %q", c.tool, got, source, ok, c.want, c.source)
			}
		})
	}
}

// On a Linux host only PATH is looked at: the folders are Windows ones.
func TestFindToolLinuxHost(t *testing.T) {
	l := layout{files: []string{"/m/ucrt64/bin/gdb.exe"}, env: map[string]string{"MSYS2_ROOT": "/m"}}
	cfg := l.config()
	cfg.Host = "linux/amd64"
	if p, source, ok := cfg.FindTool("gdb"); ok {
		t.Errorf("found %s (%s) in a Windows folder on a Linux host", p, source)
	}
}

// gocpp's own llvm-mingw has lldb: it is used when it is fully unpacked (the marker file is there),
// and only after PATH and the known folders.
func TestFindToolPinnedToolchain(t *testing.T) {
	cfg := (&layout{}).config()
	cfg.Cache = t.TempDir()
	bin := filepath.Join(cfg.Dir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	lldb := filepath.Join(bin, "lldb.exe")
	if err := os.WriteFile(lldb, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if p, _, ok := cfg.FindTool("lldb"); ok {
		t.Errorf("found %s in a toolchain that is not fully unpacked", p)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir(), markerName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p, source, ok := cfg.FindTool("lldb")
	if !ok || p != lldb || !strings.Contains(source, "pinned llvm-mingw") {
		t.Errorf("FindTool(lldb) = %q, %q, %v; want %s from the pinned toolchain", p, source, ok, lldb)
	}
}

// On a real Windows machine: with a PATH that names no gdb, FindTool finds the one of the install
// that the machine has (WinGet's WinLibs, MSYS2, ...), and it runs. Skipped elsewhere, and where gdb
// is installed in a folder that is not one of the known ones. The Windows suite runs it on the real PC
// (TestWindowsNativeUnits).
func TestFindToolReal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a Windows machine's folders")
	}
	inPath, err := exec.LookPath("gdb")
	if err != nil {
		t.Skip("gdb is not installed")
	}
	cfg, err := FromEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LookPath = func(string) (string, error) { return "", errors.New("PATH names no gdb") }
	got, source, ok := cfg.FindTool("gdb")
	if !ok {
		t.Skipf("gdb is in %s, which is not one of the folders of the known installs", inPath)
	}
	t.Logf("gdb found in %s (%s); PATH has %s", got, source, inPath)
	out, err := exec.Command(got, "--version").Output()
	if err != nil || !strings.HasPrefix(string(out), "GNU gdb") {
		t.Errorf("%s --version: %v %q", got, err, out)
	}
}
