package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/version"
)

// The up-to-date check of gocpp build (Phase 06 steps 6.5 and 6.6, ADR-036, internal/build/stamp.go).
//
// A build that succeeded leaves a stamp in the cache folder: what it was asked, what the loader and
// the compilers read, the libraries it linked, and the program it wrote. The next gocpp build with the
// same question reads the stamp first; when every file is as recorded it prints nothing, writes
// nothing and exits 0, in a few milliseconds, without loading the packages. Otherwise it says why
// (-x: "# not up to date: ...") and builds as before, and writes a new stamp when it succeeds.
//
// The stamp is for gocpp build: gocpp run builds into a folder that it removes, so it has no
// program to find again. A build that printed a warning (the checker's, or a compiler's) is not
// stamped, so the warning comes every time; neither is one that the object cache did not keep (a
// compile with warnings, flags that read files it can't see, -work), one with a pattern with "..."
// (the walk of the folders is not recorded), or one whose link flags name something the check can't
// follow (stamp.go, resolveLink). GOCPP_NOUPTODATE=1 turns the check off.

// A stampState is the up-to-date check of one gocpp build.
type stampState struct {
	key    string
	cache  string
	fsrec  *build.FSRecorder
	pkgLog *build.PkgConfigLog
	rec    *build.Record

	prev *build.Stamp // the stamp of the last build with this key, when there was one

	// What compile leaves for saving the stamp, once it succeeded.
	built     bool
	compilers []string
	pkgs      []build.PkgSummary
	warned    *atomic.Int64 // bytes the check wrote to stderr (warnings)
}

// countWriter counts the bytes that are written through it.
type countWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (c countWriter) Write(p []byte) (int, error) {
	c.n.Add(int64(len(p)))
	return c.w.Write(p)
}

// stampEnv returns the environment variables that the build reads, as sorted NAME=value entries:
// gocpp's own (GOCPP_*), the target, the compiler's and the flags' (CXX, CC, the include paths, the Nix
// wrapper's), and pkg-config's. HOME is not among them. PATH is: it decides which linker
// the compiler's -print-prog-name=ld names, and the programs the wrappers run (a changed PATH is a
// new question, so its stamp is a new one, and the answers are asked again).
func stampEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(name, "GOCPP_") && name != "GOCPP_CPUPROFILE" && !strings.HasPrefix(name, "GOCPP_TEST_") && !strings.HasPrefix(name, "GOCPP_BENCH"),
			name == "GOOS", name == "GOARCH", name == "CXX", name == "CC", name == "XDG_CACHE_HOME",
			name == "PATH", name == "LDEMULATION", name == "GNUTARGET",
			strings.HasPrefix(name, "PKG_CONFIG"), build.KeyEnvName(name), linkerEnv(name):
			env = append(env, kv)
		}
	}
	slices.Sort(env)
	return env
}

// linkerEnv reports whether the environment variable name is read by the compiler or linker
// wrappers or by the linker itself, so that it can change the program: every NIX_ variable (the
// Nix wrappers' flags, rpaths, hardening and switches; only NIX_BUILD_TOP, a random folder in every
// Nix shell, is left out), the folders the linker searches or puts in the rpath (LD_RUN_PATH,
// LD_LIBRARY_PATH, LIBRARY_PATH), what makes the compiler's output differ (SOURCE_DATE_EPOCH) and
// the launcher's (CCACHE_*).
func linkerEnv(name string) bool {
	switch name {
	case "NIX_BUILD_TOP":
		return false
	case "LD_RUN_PATH", "LD_LIBRARY_PATH", "LIBRARY_PATH", "SOURCE_DATE_EPOCH", "COMPILER_PATH", "GCC_EXEC_PREFIX":
		return true
	}
	return strings.HasPrefix(name, "NIX_") || strings.HasPrefix(name, "CCACHE_")
}

// exeIdentity returns what tells this gocpp from another build of it: the file's path, size and
// time, and the version line.
func exeIdentity() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown " + version.String()
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return exe + " " + version.String()
	}
	return fmt.Sprintf("%s %d %d %s", exe, fi.Size(), fi.ModTime().UnixNano(), version.String())
}

// cacheDir returns the cache folder of gocpp ($XDG_CACHE_HOME/gocpp), or "".
func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "gocpp")
}

// beginStamp prepares the up-to-date check of a build of patterns: it makes the key, and makes
// the loader's reads and the runs of pkg-config go through the recorders. It returns nil, with
// the reason in d.noStamp, when the build can't use the check.
func (d *driver) beginStamp(patterns []string) *stampState {
	why := ""
	switch {
	case d.name != "build":
		return nil
	case build.EnvOff("GOCPP_NOUPTODATE"):
		why = "GOCPP_NOUPTODATE is set"
	case d.flags.work:
		why = "-work keeps the work folder"
	case testHookCPP != nil:
		why = "a test changes the generated C++"
	case slices.ContainsFunc(patterns, func(p string) bool { return strings.Contains(p, "...") }):
		why = "a pattern with ... (the walk of the folders is not recorded)"
	case build.ObjectCacheOff():
		why = "the object cache is off"
	case cacheDir() == "":
		why = "there is no cache folder"
	}
	if why != "" {
		d.noStamp = why
		return nil
	}
	out := d.flags.output
	outKind := "default"
	switch {
	case out == "":
	case strings.HasSuffix(out, "/") || strings.HasSuffix(out, string(filepath.Separator)) || isDir(out):
		outKind = "dir"
	default:
		outKind = "file"
	}
	parts := []string{"gocpp build", exeIdentity(), d.check.goos + "/" + d.check.goarch, d.check.dir, strings.Join(patterns, "\x00"),
		fmt.Sprint(d.flags.asan, d.flags.nochecks), out, outKind}
	if d.flags.mode != "" {
		if d.flags.mode == build.ModeCPPLib {
			parts[len(parts)-1] = "tree" // -o is the folder of the tree, which exists after the first build
		}
		parts = append(parts, "buildmode="+string(d.flags.mode))
	}
	parts = append(parts, stampEnv()...)
	st := &stampState{key: build.StampKey(parts...), cache: cacheDir(), fsrec: build.NewFSRecorder(), pkgLog: &build.PkgConfigLog{}, rec: &build.Record{}}
	d.check.fs = st.fsrec
	d.check.pkgConfig = st.pkgLog.Wrap(d.check.pkgConfig)
	st.warned = new(atomic.Int64)
	d.check.stderr = countWriter{d.check.stderr, st.warned}
	return st
}

// upToDate reports whether the program of the last build with this key is current: the check
// of the stamp (build.Stamp.Check). It prints the reason under -x.
func (d *driver) upToDate() bool {
	st := d.stamp
	s := build.LoadStamp(st.cache, st.key)
	st.prev = s
	if s == nil {
		d.traceStamp("no record of an earlier build")
		return false
	}
	cxx, err := build.FindCXXFor(d.ctx, d.check.goos, d.check.goarch)
	if err != nil {
		return false // the build says why
	}
	ids := []string{build.CompilerID(cxx)}
	if len(s.Compilers) > 1 {
		cc, err := build.FindCC(d.ctx, cxx)
		if err != nil {
			return false
		}
		ids = append(ids, build.CompilerID(cc))
	}
	if why := s.Check(d.ctx, st.cache, ids, nil); why != "" {
		d.traceStamp(why)
		return false
	}
	if s.Dirty() {
		s.Save(st.cache) // the files that were touched have their new stat
	}
	s.Touch()
	d.upToDateBuild = true
	if d.flags.x {
		fmt.Fprintf(d.stderr, "# up to date: %s\n", s.Output.Path)
	}
	return true
}

// traceStamp prints why the build is not up to date, under -x.
func (d *driver) traceStamp(why string) {
	if d.flags.x {
		fmt.Fprintf(d.stderr, "# not up to date: %s\n", why)
	}
}

// saveStamp writes the stamp of the build that wrote the program out, when the build can be
// stamped. why, under -x, says when it can't.
func (d *driver) saveStamp(out string) {
	st := d.stamp
	if st == nil {
		if d.noStamp != "" && d.flags.x {
			fmt.Fprintf(d.stderr, "# not stamped: %s\n", d.noStamp)
		}
		return
	}
	why := ""
	switch {
	case !st.built:
		why = "the build did not finish"
	case st.warned.Load() > 0:
		why = "the check printed warnings, which are shown by every build"
	}
	var s *build.Stamp
	if why == "" {
		s, why = build.NewStamp(st.key, st.fsrec, st.pkgLog, st.rec, st.compilers, out, st.pkgs)
		if s != nil && d.lib != nil {
			if w := s.AddOutputs(d.lib.extra); w != "" {
				s, why = nil, w
			}
		}
	}
	if s == nil {
		if d.flags.x {
			fmt.Fprintf(d.stderr, "# not stamped: %s\n", why)
		}
		return
	}
	if err := s.Save(st.cache); err != nil && d.flags.x {
		fmt.Fprintf(d.stderr, "# not stamped: %v\n", err)
	}
}

// stampSummaries returns the summaries of the packages of the build: the hash of the sources
// (from the loader's reads) and the API hash, the hash of pkg.hpp.
func (d *driver) stampSummaries(set *buildSet, files *cppgen.ProgramFiles) []build.PkgSummary {
	headers := make(map[string][]byte)
	for _, f := range files.Files {
		if strings.HasSuffix(f.Path, "/pkg.hpp") {
			headers[f.Path] = f.Data
		}
	}
	var sums []build.PkgSummary
	for _, p := range set.pkgs {
		h := sha256.New()
		for _, f := range p.Load.Files {
			fmt.Fprintf(h, "%s\x00%s\n", filepath.Base(f.Path), d.stamp.fsrec.FileHash(f.Path))
		}
		api := sha256.Sum256(headers["gpkg/"+p.Load.ImportPath+"/pkg.hpp"])
		sums = append(sums, build.PkgSummary{Path: p.Load.ImportPath, Src: hex.EncodeToString(h.Sum(nil))[:16], API: hex.EncodeToString(api[:])[:16]})
	}
	return sums
}

// traceSummaries prints the summaries of the packages under -x: for each package the hash of the
// API (pkg.hpp) and, when an earlier build with the same key left a stamp, which packages have
// other sources or another API than then. A package whose API did not change does not recompile
// the units of the packages that import it (their headers are the same), whatever changed in its
// bodies; one whose API changed does.
func (d *driver) traceSummaries(sums []build.PkgSummary) {
	if !d.flags.x {
		return
	}
	for _, p := range sums {
		fmt.Fprintf(d.stderr, "# api %s %s\n", p.Path, p.API)
	}
	if d.stamp.prev == nil {
		return
	}
	src, api := build.ChangedPackages(d.stamp.prev.PkgSummaries(), sums)
	if len(src) > 0 {
		fmt.Fprintf(d.stderr, "# sources changed: %s\n", strings.Join(src, " "))
	}
	if len(api) > 0 {
		fmt.Fprintf(d.stderr, "# api changed: %s\n", strings.Join(api, " "))
	}
}

// watchHeaders records the header files that the packages import by a path in their folder or in
// a module ("./physics.h", "github.com/x/lib/include/x.h"): the loader does not read them (the C++
// compiler does), so their content is not in its trace; the stamp has it, so a header that changed
// is never missed, whatever the compiles read.
func (d *driver) watchHeaders(set *buildSet) {
	st := d.stamp
	for _, p := range set.pkgs {
		for _, f := range p.Load.Files {
			if f.Info == nil {
				continue
			}
			for _, inc := range f.Info.Includes {
				switch {
				case inc.File != "":
					st.fsrec.ReadFile(filepath.FromSlash(inc.File))
				case inc.Kind == load.LocalHeader:
					st.fsrec.ReadFile(filepath.Join(p.Load.Dir, filepath.FromSlash(inc.Path)))
				}
			}
		}
	}
	// The checksums of the main module's downloads.
	if m := set.main.Load.Module; m != nil && m.Dir != "" {
		st.fsrec.ReadFile(filepath.Join(m.Dir, "gocpp.sum"))
	}
}
