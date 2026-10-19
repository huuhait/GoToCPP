// Package toolchain finds, downloads and unpacks the C++ toolchain that builds programs for
// Windows (Phase 06 step 6.13, ADR-055, docs/guides/windows-cross.md), the way the go command's
// GOTOOLCHAIN fetches a Go toolchain: gocpp needs a MinGW-w64 compiler for windows/amd64, and
// when the machine has none, gocpp downloads the pinned llvm-mingw release.
//
// GOCPP_TOOLCHAIN chooses the rule (Mode): "auto" (the default: a MinGW-w64 compiler in PATH,
// else the pinned one), "local" (never download) or "pinned" (always the pinned one). The pinned
// archives, with the SHA-256 and the size of each, are compiled into the program (Archives): an
// archive is checked against them before anything is unpacked, and a host without an entry (or
// with an empty hash) fails closed. The download comes from llvm-mingw's GitHub releases or from a
// mirror (GOCPP_TOOLCHAIN_URL: a folder URL that holds the archives under their names, like
// GOPROXY), over HTTPS only. It is unpacked into <cache>/toolchain/llvm-mingw-<version>-<host>/
// through a temporary folder and one rename, with a lock file so that two builds that start at
// once download it once, and the files are read-only afterwards (Remove makes them writable).
//
// Nothing here runs in the editor's way: Resolve downloads only when Config.Download is true
// (gocpp build and gocpp run); gocpp check and the language server use a toolchain that is
// already there.
package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Release is the llvm-mingw release that this gocpp pins, and Variant its C runtime: UCRT, the one
// that Windows 10 has built in (the dev shell's MinGW-w64 is the older msvcrt flavor).
const (
	Release = "20261006"
	Variant = "ucrt"
)

// DefaultBaseURL is the folder of the release's archives on GitHub.
const DefaultBaseURL = "https://github.com/mstorsjo/llvm-mingw/releases/download/" + Release

// An Archive is a pinned toolchain archive of one host.
type Archive struct {
	Host   string // GOOS/GOARCH of the system that runs the compiler: "linux/amd64"
	Name   string // the file name in the release
	SHA256 string // lower-case hex; empty means "not set": the download fails closed
	Size   int64  // the exact size in bytes
	// Loader is the dynamic loader that the toolchain's programs need ("" when the host has none to
	// check): on a system without it (NixOS) they don't start, and gocpp says so before it downloads
	// 84 MB that can't run.
	Loader string
}

// Archives are the pinned archives of Release, from GitHub's release page (the digests shown there
// for each asset; the amd64 one was also computed by downloading the file; the aarch64 one only
// compared with the digest and size that the release API reports). The Windows host (Phase 14 step 14.2) is the zip.
//
// To check the table again (a new Release, or a doubt), compare it with what GitHub reports, then
// with the bytes of the file:
//
//	curl -sL https://api.github.com/repos/mstorsjo/llvm-mingw/releases/tags/$RELEASE |
//	    jq -r '.assets[] | select(.name|test("ucrt-(ubuntu|x86_64.zip)")) | "\(.name) \(.size) \(.digest)"'
//	curl -sLO $DefaultBaseURL/<name> && sha256sum <name> && stat -c %s <name>
//
// TestPinnedTable checks the shape of the entries (64 hex digits, the name's release and variant).
var Archives = []Archive{
	{Host: "linux/amd64", Name: "llvm-mingw-" + Release + "-" + Variant + "-ubuntu-22.04-x86_64.tar.xz",
		SHA256: "5f9c6ed95b2d4bdb2869a488c5fd5857fbdabcf288a0aa3eb1da43f6a08d8ab4", Size: 84003044,
		Loader: "/lib64/ld-linux-x86-64.so.2"},
	{Host: "linux/arm64", Name: "llvm-mingw-" + Release + "-" + Variant + "-ubuntu-22.04-aarch64.tar.xz",
		SHA256: "e77c4acfa44184f6b0c52b814d724ad63b2ecebb3c3f6f842c0846e717cbd23c", Size: 78002292,
		Loader: "/lib/ld-linux-aarch64.so.1"},
	// The Windows host (Phase 14 step 14.2): a zip with the compilers as .exe files, no symbolic
	// links. 190,836,485 bytes; the digest is GitHub's and was also computed over the downloaded file.
	{Host: "windows/amd64", Name: "llvm-mingw-" + Release + "-" + Variant + "-x86_64.zip",
		SHA256: "317492c456aa27ee607a5919f1d2d38dcdc1112516a24d0bf4b00d078f52d17a", Size: 190836485},
}

// A Mode is the rule of GOCPP_TOOLCHAIN.
type Mode string

const (
	Auto   Mode = "auto"
	Local  Mode = "local"
	Pinned Mode = "pinned"
)

// DefaultMode is the rule when GOCPP_TOOLCHAIN is not set. Auto looks in PATH and, when a build or
// run finds no MinGW-w64 compiler there, downloads the pinned toolchain (one line on stderr, about
// 80 MiB). Local makes the download opt-in (GOCPP_TOOLCHAIN=auto or pinned): changing this one
// line flips the default (the tests that depend on it read DefaultMode, and docs/spec/build.md
// and docs/guides/windows-cross.md say which one is current).
const DefaultMode = Auto

// ModeOf parses the value of GOCPP_TOOLCHAIN ("" is DefaultMode).
func ModeOf(s string) (Mode, error) {
	switch Mode(s) {
	case "":
		return DefaultMode, nil
	case Auto, Local, Pinned:
		return Mode(s), nil
	}
	return "", fmt.Errorf("GOCPP_TOOLCHAIN=%s: want auto, local or pinned", s)
}

// A Config is what Resolve needs. FromEnv fills it from the environment.
type Config struct {
	Mode     Mode
	Cache    string       // gocpp's cache folder ($XDG_CACHE_HOME/gocpp); the toolchain goes to Cache/toolchain
	BaseURL  string       // the folder URL of the archives; empty is DefaultBaseURL
	Client   *http.Client // nil is a client with timeouts for the connection and the headers
	Host     string       // GOOS/GOARCH of this system; empty is runtime's
	Archives []Archive    // nil is Archives
	Download bool         // whether Resolve may start a download
	// Offline is GOCPP_OFFLINE: the network is never used, so nothing is downloaded (a toolchain
	// that is already in the cache is used).
	Offline bool
	// IdleTimeout is how long a download may go without receiving a byte; zero is 60 seconds.
	IdleTimeout time.Duration
	Progress    io.Writer // the lines about a download; nil is os.Stderr
	// LookPath finds a program in PATH; nil is exec.LookPath.
	LookPath func(string) (string, error)
	// What the search for an installed MinGW-w64 on a Windows host reads (detect.go); nil is the
	// real thing. The tests put fake layouts here.
	Getenv  func(string) string                   // os.Getenv
	IsFile  func(string) bool                     // a regular file exists
	IsDir   func(string) bool                     // a folder exists
	Glob    func(pattern string) []string         // filepath.Glob
	Machine func(compiler string) (string, error) // the compiler's -dumpmachine
}

// FromEnv returns the configuration of GOCPP_TOOLCHAIN, GOCPP_TOOLCHAIN_URL and the cache folder
// ($XDG_CACHE_HOME/gocpp), for a system that may or may not download.
func FromEnv(download bool) (Config, error) {
	mode, err := ModeOf(os.Getenv("GOCPP_TOOLCHAIN"))
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Mode: mode, BaseURL: os.Getenv("GOCPP_TOOLCHAIN_URL"), Download: download, Offline: envOn(os.Getenv("GOCPP_OFFLINE"))}
	if dir, err := os.UserCacheDir(); err == nil {
		cfg.Cache = filepath.Join(dir, "gocpp")
	}
	return cfg, nil
}

// envOn reports whether an on/off variable is on: anything but empty, 0 and false (the rule of
// GOCPP_OFFLINE for module downloads).
func envOn(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v != "" && v != "0" && v != "false"
}

func (c *Config) host() string {
	if c.Host != "" {
		return c.Host
	}
	return runtime.GOOS + "/" + runtime.GOARCH
}

func (c *Config) archive() (Archive, bool) {
	list := c.Archives
	if list == nil {
		list = Archives
	}
	for _, a := range list {
		if a.Host == c.host() {
			return a, true
		}
	}
	return Archive{}, false
}

func (c *Config) lookPath(prog string) (string, error) {
	if c.LookPath != nil {
		return c.LookPath(prog)
	}
	return exec.LookPath(prog)
}

func (c *Config) progress() io.Writer {
	if c.Progress != nil {
		return c.Progress
	}
	return os.Stderr
}

// Dir returns the folder of the pinned toolchain for this host ("" without a cache folder):
// <cache>/toolchain/llvm-mingw-<release>-<host>.
func (c *Config) Dir() string {
	if c.Cache == "" {
		return ""
	}
	return filepath.Join(c.Cache, "toolchain", "llvm-mingw-"+Release+"-"+strings.ReplaceAll(c.host(), "/", "-"))
}

// A Found is the compiler that Resolve chose.
type Found struct {
	Path   string // the program: x86_64-w64-mingw32-clang++ ...
	Source string // where it came from: "PATH", "pinned llvm-mingw 20261006 (cached)", "... (downloaded)"
}

// The compilers of a Windows target, in the order that auto looks for them in PATH.
func localNames(goos, goarch string) []string {
	if goos == "windows" && goarch == "amd64" {
		return []string{"x86_64-w64-mingw32-clang++", "x86_64-w64-mingw32-g++"}
	}
	return nil
}

// Resolve finds the C++ compiler for the target goos/goarch (windows/amd64) by the rule of the
// configuration. Its error says what to do about it.
func Resolve(ctx context.Context, cfg Config, goos, goarch string) (Found, error) {
	names := localNames(goos, goarch)
	if names == nil {
		return Found{}, fmt.Errorf("no toolchain is pinned for %s/%s", goos, goarch)
	}
	if cfg.Mode == "" {
		cfg.Mode = DefaultMode
	}
	target := goos + "/" + goarch
	if cfg.Mode != Pinned {
		if f, ok := cfg.local(ctx, names); ok {
			return f, nil
		}
		if cfg.Mode == Local {
			return Found{}, fmt.Errorf("no C++ compiler for %s: %s (GOCPP_TOOLCHAIN=local never downloads one%s; %s, or set GOCPP_CXX)",
				target, cfg.notFound(names), autoHint(), installHint(cfg.isWindowsHost()))
		}
	}
	f, err := cfg.pinned(ctx, names[0])
	if err != nil {
		if cfg.Mode == Auto {
			msg := fmt.Errorf("no C++ compiler for %s: %s, and the pinned toolchain is not usable: %w", target, cfg.notFound(names), err)
			if !strings.Contains(err.Error(), "GOCPP_CXX") {
				return Found{}, fmt.Errorf("%w (set GOCPP_CXX)", msg) // the offline message says it already
			}
			return Found{}, msg
		}
		return Found{}, fmt.Errorf("no C++ compiler for %s: the pinned toolchain is not usable: %w", target, err)
	}
	return f, nil
}

// notFound says where the search for a local compiler looked.
func (c *Config) notFound(names []string) string {
	if c.isWindowsHost() {
		return "no MinGW-w64 g++ or clang++ is in PATH, and none is in the folders of MSYS2, Chocolatey, Scoop or WinGet's WinLibs"
	}
	return "neither " + strings.Join(names, " nor ") + " is in PATH"
}

// installHint says how to get a compiler by hand.
func installHint(windows bool) string {
	if windows {
		return "install MinGW-w64 (WinLibs: winget install BrechtSanders.WinLibs.POSIX.UCRT; MSYS2: pacman -S mingw-w64-ucrt-x86_64-gcc) or llvm-mingw"
	}
	return "install MinGW-w64, or llvm-mingw"
}

// autoHint is the end of the message of mode local, when local is not the default: how to let
// gocpp download a toolchain.
func autoHint() string {
	if DefaultMode == Local {
		return "; GOCPP_TOOLCHAIN=auto lets gocpp download the pinned one"
	}
	return ""
}

// pinned returns the pinned compiler: from the cache, or downloaded when the configuration allows.
func (c *Config) pinned(ctx context.Context, name string) (Found, error) {
	dir := c.Dir()
	if dir == "" {
		return Found{}, errors.New("there is no cache folder (neither XDG_CACHE_HOME nor HOME is set)")
	}
	source := "pinned llvm-mingw " + Release + " (cached)"
	if p, ok := installed(dir, name); ok {
		return Found{Path: p, Source: source}, nil
	}
	if c.Offline {
		return Found{}, fmt.Errorf("it is not downloaded (%s) and GOCPP_OFFLINE is set, so gocpp does not use the network: %s, or set GOCPP_CXX", dir, installHint(c.isWindowsHost()))
	}
	if !c.Download {
		return Found{}, fmt.Errorf("it is not downloaded yet: gocpp build and gocpp run download it on first use (to %s)", dir)
	}
	a, ok := c.archive()
	if !ok {
		return Found{}, fmt.Errorf("no toolchain archive is pinned for a %s host", c.host())
	}
	if a.SHA256 == "" {
		return Found{}, fmt.Errorf("pinned toolchain hash not set for a %s host (gocpp was built without the archive's SHA-256, so it will not download one)", c.host())
	}
	if a.Loader != "" {
		if _, err := os.Stat(a.Loader); err != nil {
			return Found{}, fmt.Errorf("its programs need %s, which this system does not have (NixOS?): install MinGW-w64 and use GOCPP_TOOLCHAIN=local, or a loader such as nix-ld", a.Loader)
		}
	}
	if err := c.fetch(ctx, a, dir); err != nil {
		return Found{}, err
	}
	p, ok := installed(dir, name)
	if !ok {
		return Found{}, fmt.Errorf("the unpacked toolchain in %s has no bin/%s", dir, name)
	}
	return Found{Path: p, Source: "pinned llvm-mingw " + Release + " (downloaded)"}, nil
}

// markerName is the file that a finished unpack leaves in the toolchain's folder.
const markerName = ".gocpp-toolchain"

// installed returns the compiler of the unpacked toolchain in dir.
func installed(dir, name string) (string, bool) {
	if _, err := os.Stat(filepath.Join(dir, markerName)); err != nil {
		return "", false
	}
	return binFile(dir, name)
}

// binFile returns the program name of the folder dir/bin: on a Windows host the file has ".exe"
// after its name (the archive of a Windows host holds x86_64-w64-mingw32-clang++.exe).
func binFile(dir, name string) (string, bool) {
	for _, n := range []string{name, name + ".exe"} {
		p := filepath.Join(dir, "bin", n)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

// Installed reports whether the pinned toolchain of this host is unpacked, and where.
func (c *Config) Installed() (string, bool) {
	dir := c.Dir()
	if dir == "" {
		return "", false
	}
	_, ok := installed(dir, "x86_64-w64-mingw32-clang++")
	return dir, ok
}

// Remove deletes the pinned toolchain of this host (gocpp clean -toolchain): its files are
// read-only, so the folders are made writable first. A missing toolchain is not an error. It
// returns the folder that it removed ("" when there was none).
func (c *Config) Remove() (string, error) {
	dir := c.Dir()
	if dir == "" {
		return "", nil
	}
	// The lock file and what an interrupted download left go too, and the folder of the toolchains when it is empty.
	defer func() {
		os.Remove(dir + ".lock")
		sweepStale(filepath.Dir(dir))
		os.Remove(filepath.Dir(dir))
	}()
	if _, err := os.Lstat(dir); err != nil {
		return "", nil
	}
	MakeWritable(dir)
	return dir, os.RemoveAll(dir)
}

// Exists reports whether the folder of the pinned toolchain is there, finished or not (Remove
// removes it either way: a folder that holds no compiler must not stay).
func (c *Config) Exists() (string, bool) {
	dir := c.Dir()
	if dir == "" {
		return "", false
	}
	_, err := os.Lstat(dir)
	return dir, err == nil
}

// MakeWritable gives the owner write permission to the folders of root, so that RemoveAll works.
func MakeWritable(root string) {
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o755)
		}
		return nil
	})
}

// client returns the HTTP client: the configured one, or one that gives up on a server that
// doesn't answer (the transfer itself has no limit but the context's).
func (c *Config) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 60 * time.Second
	return &http.Client{Transport: t}
}

// archiveURL returns the address of the archive: the mirror's folder (GOCPP_TOOLCHAIN_URL) or
// GitHub's. Only https is accepted: the hash protects the contents, and the scheme keeps the
// download private and a proxy honest.
func (c *Config) archiveURL(a Archive) (string, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("GOCPP_TOOLCHAIN_URL=%s: %w", base, err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("GOCPP_TOOLCHAIN_URL=%s: want an https:// folder URL", base)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + a.Name
	return u.String(), nil
}
