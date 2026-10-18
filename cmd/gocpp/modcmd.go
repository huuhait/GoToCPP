package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
)

// getUsage is the help text of "gocpp get".
const getUsage = `usage: gocpp get [module[@version] ...]

gocpp get adds modules to the require lines of gocpp.mod, downloads them into
the module cache and records their hashes in gocpp.sum. The module is a
repository on github.com, gitlab.com or codeberg.org (github.com/ocornut/imgui),
or any module that GOCPP_MODURL or GOCPP_MODPROXY says where to get. The
version is:

	latest         the newest release tag (vX.Y.Z or X.Y.Z); with no tags, the
	               head of the default branch. The default.
	v1.2.3         that tag (or a pseudo-version v0.0.0-yyyymmddhhmmss-commit12)
	v1, v1.2       the newest tag of that line
	<branch>       the head of a branch, as a pseudo-version
	<commit>       a commit (7 to 40 hex digits), as a pseudo-version
	none           remove the require line

With no module, gocpp get downloads every module that gocpp.mod requires
(like gocpp mod download). The first download of a module is trusted and its
hash is recorded in gocpp.sum; a later download that differs is a security
error. Downloaded code is never run. See docs/spec/modules.md for the
environment variables (GOCPP_OFFLINE, GOCPP_MODCACHE, GOCPP_MODPROXY,
GOCPP_MODURL, GOCPP_MODAUTH, ...).
`

// modUsage is the help text of "gocpp mod".
const modUsage = `usage: gocpp mod <command> [arguments]

Module maintenance. The commands are:

	download [module ...]   download the required modules into the module cache
	                        and record their hashes in gocpp.sum
	verify                  hash the modules in the module cache (or in vendor/)
	                        again and compare them with gocpp.sum
	vendor [-v]             copy every required module into vendor/ and write
	                        vendor/modules.txt, for builds without a network

Run "gocpp mod <command> -h" for a command. See docs/spec/modules.md.
`

// runGet is "gocpp get" (Phase 06 job 6.1c, docs/spec/modules.md).
func runGet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp get", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, getUsage) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	m, code := openMainModule("gocpp get", stderr)
	if m == nil {
		return code
	}
	if flags.NArg() == 0 {
		return m.download("gocpp get", nil)
	}
	f, ok := m.fetcher("gocpp get")
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	changed := false
	var failed bool
	for _, arg := range flags.Args() {
		path, query, _ := cutLast(arg, "@")
		if path == "" {
			fmt.Fprintf(stderr, "gocpp get: %q: want module[@version]\n", arg)
			return 2
		}
		if query == "none" {
			if modfetch.DropRequire(m.file, path) {
				fmt.Fprintf(stderr, "gocpp: removed %s\n", path)
				changed = true
			} else {
				fmt.Fprintf(stderr, "gocpp get: %s is not required in %s\n", path, gocppmod.FileName)
				failed = true
			}
			continue
		}
		old := m.file.FindRequire(path)
		if old != nil && old.Path != path {
			old = nil
		}
		mod, err := f.Get(ctx, m.dir, path, query)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp get: %v\n", err)
			failed = true
			continue
		}
		switch {
		case old == nil:
			fmt.Fprintf(stderr, "gocpp: added %s %s\n", mod.Path, mod.Version)
		case old.Version != mod.Version:
			verb := "upgraded"
			if modfetch.CompareVersions(mod.Version, old.Version) < 0 {
				verb = "downgraded"
			}
			fmt.Fprintf(stderr, "gocpp: %s %s %s => %s\n", verb, mod.Path, old.Version, mod.Version)
		}
		if rep := m.file.FindReplace(path, mod.Version); rep != nil {
			fmt.Fprintf(stderr, "gocpp: note: %s is replaced by %s in %s, so the download is not used until the replace line is removed\n", path, rep.NewPath, gocppmod.FileName)
		}
		if old == nil || old.Version != mod.Version {
			modfetch.SetRequire(m.file, path, mod.Version)
			changed = true
		}
	}
	if failed {
		fmt.Fprintf(stderr, "gocpp get: %s was not changed\n", gocppmod.FileName)
		return 1
	}
	if changed {
		if err := m.write(); err != nil {
			fmt.Fprintf(stderr, "gocpp get: %v\n", err)
			return 1
		}
		if _, err := os.Stat(filepath.Join(m.dir, "vendor", gocppmod.VendorFile)); err == nil {
			fmt.Fprintf(stderr, "gocpp: note: vendor/ is not updated; run \"gocpp mod vendor\"\n")
		}
	}
	return 0
}

// runMod is "gocpp mod".
func runMod(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, modUsage)
		return 2
	}
	switch args[0] {
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, modUsage)
		return 0
	case "download":
		return runModDownload(args[1:], stdout, stderr)
	case "verify":
		return runModVerify(args[1:], stdout, stderr)
	case "vendor":
		return runModVendor(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "gocpp mod %s: unknown command\nRun 'gocpp mod -h' for usage.\n", args[0])
	return 2
}

func modFlags(name string, args []string, stderr io.Writer, usage string, setup func(*flag.FlagSet)) (*flag.FlagSet, int, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usage)
		flags.PrintDefaults()
	}
	if setup != nil {
		setup(flags)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, 0, false
		}
		return nil, 2, false
	}
	return flags, 0, true
}

func runModDownload(args []string, _, stderr io.Writer) int {
	flags, code, ok := modFlags("gocpp mod download", args, stderr, `usage: gocpp mod download [module ...]

Downloads the modules that gocpp.mod requires (or only the named ones) into the
module cache, checks them against gocpp.sum and adds the hashes of the ones it
does not list yet. A module that is replaced by a local folder is not downloaded.
`, nil)
	if !ok {
		return code
	}
	m, code := openMainModule("gocpp mod download", stderr)
	if m == nil {
		return code
	}
	return m.download("gocpp mod download", flags.Args())
}

func runModVerify(args []string, stdout, stderr io.Writer) int {
	_, code, ok := modFlags("gocpp mod verify", args, stderr, `usage: gocpp mod verify

Hashes the required modules in the module cache again (or in vendor/ for a
vendored project) and compares the hashes with gocpp.sum and with the hashes
recorded when the modules were downloaded. A module that changed is reported and
the exit status is 1. Modules that are not in the cache are not checked.
`, nil)
	if !ok {
		return code
	}
	m, code := openMainModule("gocpp mod verify", stderr)
	if m == nil {
		return code
	}
	f, ok := m.fetcher("gocpp mod verify")
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	res, err := f.Verify(ctx, m.dir, m.file)
	if err != nil {
		fmt.Fprintf(stderr, "gocpp mod verify: %v\n", err)
		return 1
	}
	bad, skipped := 0, 0
	for _, r := range res {
		switch {
		case r.Skipped:
			skipped++
		case r.Problem != "":
			bad++
			fmt.Fprintf(stdout, "%s %s: %s\n", r.Path, r.Version, r.Problem)
		}
	}
	if skipped > 0 {
		fmt.Fprintf(stdout, "(%d module(s) are not in the module cache and were not checked)\n", skipped)
	}
	if bad > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "all modules verified")
	return 0
}

func runModVendor(args []string, _, stderr io.Writer) int {
	var verbose bool
	_, code, ok := modFlags("gocpp mod vendor", args, stderr, `usage: gocpp mod vendor [-v]

Copies every module that gocpp.mod requires, replaced or not, with all of its
files, into vendor/<module path> and writes vendor/modules.txt (Go's format),
so that the project builds without a network and without the module cache. The
modules that are not replaced are downloaded first, and checked against
gocpp.sum. A vendor folder that gocpp mod vendor did not make (it has no
modules.txt) is never replaced. The flags are:
`, func(fs *flag.FlagSet) { fs.BoolVar(&verbose, "v", false, "print the modules that are vendored") })
	if !ok {
		return code
	}
	m, code := openMainModule("gocpp mod vendor", stderr)
	if m == nil {
		return code
	}
	f, ok := m.fetcher("gocpp mod vendor")
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	if err := f.Vendor(ctx, m.dir, m.file); err != nil {
		fmt.Fprintf(stderr, "gocpp mod vendor: %v\n", err)
		return 1
	}
	if verbose {
		for _, r := range m.file.Require {
			fmt.Fprintf(stderr, "# %s %s\n", r.Path, r.Version)
		}
	}
	return 0
}

// interruptContext returns a context that the first Ctrl-C cancels. The
// second Ctrl-C then ends the process the usual way: the signal handler is
// removed as soon as the first one arrives.
func interruptContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}

// A mainModule is the module of the current folder.
type mainModule struct {
	dir    string
	file   *gocppmod.File
	stderr io.Writer
}

// openMainModule finds and reads gocpp.mod in the current folder or above. It
// prints the problem and returns the exit code if there is none or it has
// errors.
func openMainModule(cmd string, stderr io.Writer) (*mainModule, int) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return nil, 1
	}
	dir := wd
	for {
		if st, err := os.Stat(filepath.Join(dir, gocppmod.FileName)); err == nil && st.Mode().IsRegular() {
			break
		}
		up := filepath.Dir(dir)
		if up == dir {
			fmt.Fprintf(stderr, "%s: %s file not found in %s or any parent directory\n", cmd, gocppmod.FileName, wd)
			return nil, 1
		}
		dir = up
	}
	name := filepath.Join(dir, gocppmod.FileName)
	data, err := modfetch.ReadRegular(name, 16<<20)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return nil, 1
	}
	file, errs := gocppmod.Parse(name, data, nil)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(stderr, e.Error())
		}
		return nil, 1
	}
	return &mainModule{dir: dir, file: file, stderr: stderr}, 0
}

func (m *mainModule) fetcher(cmd string) (*modfetch.Fetcher, bool) {
	cfg, err := modfetch.ConfigFromEnv(os.Getenv, m.stderr)
	if err == nil {
		var f *modfetch.Fetcher
		if f, err = modfetch.New(cfg); err == nil {
			return f, true
		}
	}
	fmt.Fprintf(m.stderr, "%s: %v\n", cmd, err)
	return nil, false
}

// download downloads the required modules (all, or the named ones).
func (m *mainModule) download(cmd string, only []string) int {
	f, ok := m.fetcher(cmd)
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	if _, err := f.DownloadAll(ctx, m.dir, m.file, only); err != nil {
		fmt.Fprintf(m.stderr, "%s: %v\n", cmd, err)
		return 1
	}
	return 0
}

// write writes gocpp.mod back, atomically, with the mode it had.
func (m *mainModule) write() error {
	name := filepath.Join(m.dir, gocppmod.FileName)
	if err := modfetch.CheckWritable(name); err != nil {
		return err
	}
	data, err := modfetch.FormatFile(name, m.file)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(name); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(m.dir, ".gocpp.mod-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}

func cutLast(s, sep string) (before, after string, found bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}
