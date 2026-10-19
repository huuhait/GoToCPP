package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modget"
)

// getUsage is the help text of "gocpp get".
const getUsage = `usage: gocpp get [-u[=patch]] [-t] [module[@version] ...]

gocpp get changes the requirements of gocpp.mod, downloads the modules that
changed into the module cache and records their hashes in gocpp.sum. The files
are written together, and only if everything worked. The module is a
repository on github.com, gitlab.com or codeberg.org (github.com/ocornut/imgui),
or any module that GOCPP_MODURL or GOCPP_MODPROXY says where to get. The
version is:

	latest         the newest release tag (vX.Y.Z or X.Y.Z); with no tags, the
	               head of the default branch. The default for a module that
	               gocpp.mod does not require yet.
	upgrade        the newest release at or above the version in use. The
	               default for a module that gocpp.mod requires.
	patch          the newest patch release of the version in use
	v1.2.3         that tag (or a pseudo-version v0.0.0-yyyymmddhhmmss-commit12);
	               a version below the one in use is a downgrade, and the
	               modules that need more than it allows go down too
	v1, v1.2       the newest tag of that line
	<v1.5.0 ...    the newest tag below (<, <=) or the oldest above (>, >=) a version
	<branch>       the head of a branch, as a pseudo-version
	<commit>       a commit (7 to 40 hex digits), as a pseudo-version
	none           remove the module (and what only it needs, to the extent that
	               version selection can say)

The versions of every module that the requirements need are chosen by minimal
version selection, as the go command does: the highest version that any
gocpp.mod or go.mod in the graph requires. gocpp.mod gets one require line for
each module of the result; a module that you did not name has "// indirect".

The flags are:

	-u             also upgrade the modules that the named modules need, to
	               their latest version (every module, if none is named)
	-u=patch       the same, to the latest patch release
	-t             accepted for compatibility with go get; requirements are
	               per module, so it changes nothing

With no module and no -u, gocpp get downloads every module that gocpp.mod
requires (like gocpp mod download). The first download of a module is trusted
and its hash is recorded in gocpp.sum; a later download that differs is a
security error. Downloaded code is never run. See docs/guides/modules.md and
docs/spec/modules.md for the environment variables (GOCPP_OFFLINE,
GOCPP_MODCACHE, GOCPP_MODPROXY, GOCPP_MODURL, GOCPP_MODAUTH, ...).
`

// upgradeFlag is -u, which may be given a value: -u=patch.
type upgradeFlag struct{ v *string }

func (u upgradeFlag) String() string {
	if u.v == nil {
		return ""
	}
	return *u.v
}

func (u upgradeFlag) IsBoolFlag() bool { return true }

func (u upgradeFlag) Set(s string) error {
	switch s {
	case "true", "1", "all":
		*u.v = "all"
	case "patch":
		*u.v = "patch"
	case "false", "0":
		*u.v = ""
	default:
		return fmt.Errorf("want -u or -u=patch")
	}
	return nil
}

// runGet is "gocpp get" (Phase 06 job 6.1c, Phase 15 job 15.14).
func runGet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp get", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, getUsage) }
	var upgrade string
	var tests bool
	flags.Var(upgradeFlag{&upgrade}, "u", "upgrade the modules that the named modules need")
	flags.BoolVar(&tests, "t", false, "accepted for compatibility")
	// Flags may follow the modules, as in go get.
	var mods []string
	for {
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if flags.NArg() == 0 {
			break
		}
		mods = append(mods, flags.Arg(0))
		args = flags.Args()[1:]
	}
	m, code := openMainModule("gocpp get", stderr)
	if m == nil {
		return code
	}
	if len(mods) == 0 && upgrade == "" {
		return m.download("gocpp get", nil)
	}
	f, ok := m.fetcher("gocpp get")
	if !ok {
		return 1
	}
	ctx, stop := interruptContext()
	defer stop()
	res, err := modget.Run(ctx, modget.Options{Dir: m.dir, Fetcher: f, Args: mods, Upgrade: upgrade, Tests: tests})
	if err != nil {
		fmt.Fprintf(stderr, "gocpp get: %v\n", err)
		fmt.Fprintf(stderr, "gocpp get: %s was not changed\n", gocppmod.FileName)
		return 1
	}
	for _, c := range res.Changes {
		fmt.Fprintf(stderr, "gocpp: %s\n", c)
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stderr, "gocpp: warning: %s\n", w)
	}
	for _, n := range res.Notes {
		fmt.Fprintf(stderr, "gocpp: note: %s\n", n)
	}
	if res.ModChanged {
		if _, err := os.Stat(filepath.Join(m.dir, "vendor", gocppmod.VendorFile)); err == nil {
			fmt.Fprintf(stderr, "gocpp: note: vendor/ is not updated; run \"gocpp mod vendor\"\n")
		}
	}
	return 0
}
