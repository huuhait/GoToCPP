// Gotest runs Go's own test/ folder (the run, errorcheck and compile tests
// of the Go repository) through gocpp: the conformance suite of step 15.10.
//
// Usage (in the dev shell, from the repository root):
//
//	go run ./tools/gotest fetch                  # download and extract the pinned test/ folder
//	go run ./tools/gotest manifest [-check]      # classify every test: tools/gotest/manifest.txt
//	go run ./tools/gotest run [flags]            # run applicable tests, write results JSON
//	go run ./tools/gotest report [flags]         # counts, pass rate, failure clusters
//	go run ./tools/gotest gate [-update]         # the regression gate (make conformance-check)
//	go run ./tools/gotest triage                 # map failure clusters to owner steps (expect.txt)
//
// docs/guides/conformance.md describes the directives, the reason codes
// and the workflow. Go's test files are never copied into the repository:
// they are read from the pinned tarball in the cache.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/huuhait/gocpp/internal/envflag"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usageText = `usage: gotest <command> [flags]

commands:
  fetch      download and extract the pinned Go test/ folder into the cache
  manifest   classify every test (tools/gotest/manifest.txt); -check compares with the file
  run        run the applicable tests with gocpp; write a results JSON file
  report     print counts and failure clusters of results files; write docs/conformance/go-test.md
  gate       the regression gate: the tests of passing.txt must still pass
  triage     assign owner steps (expect.txt) to the failing tests by cluster rules (clusters.txt)
  list       print the manifest as a table, filtered
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "fetch":
		err = cmdFetch(rest, stdout, stderr)
	case "manifest":
		err = cmdManifest(rest, stdout, stderr)
	case "run":
		err = cmdRun(rest, stdout, stderr)
	case "report":
		err = cmdReport(rest, stdout, stderr)
	case "gate":
		err = cmdGate(rest, stdout, stderr)
	case "triage":
		err = cmdTriage(rest, stdout, stderr)
	case "list":
		err = cmdList(rest, stdout, stderr)
	case "-h", "-help", "help":
		fmt.Fprint(stdout, usageText)
		return 0
	default:
		fmt.Fprintf(stderr, "gotest: unknown command %q\n%s", cmd, usageText)
		return 2
	}
	if err != nil {
		if e, ok := err.(exitError); ok {
			if e.msg != "" {
				fmt.Fprintln(stderr, "gotest:", e.msg)
			}
			return e.code
		}
		fmt.Fprintln(stderr, "gotest:", err)
		return 1
	}
	return 0
}

// An exitError ends the tool with a code (the regression gate fails with 1,
// a usage error with 2).
type exitError struct {
	code int
	msg  string
}

func (e exitError) Error() string { return e.msg }

func usageError(format string, args ...any) error {
	return exitError{2, fmt.Sprintf(format, args...)}
}

// repoRoot finds the folder with go.mod above the current folder.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "tools", "gotest")); err == nil {
				return dir, nil
			}
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", fmt.Errorf("no repository with tools/gotest above the current folder: run from the repository, or pass -config")
		}
		dir = up
	}
}

// userCache returns $XDG_CACHE_HOME or ~/.cache (on Linux), the base of
// gocpp's caches.
func userCache() (string, error) {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return x, nil
	}
	return os.UserCacheDir()
}

// defaultTarget is the machine the tests are for: the one that runs them.
func defaultTarget(pin Pin) Target {
	return Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Minor: pin.minor()}
}

// commonFlags are the flags several commands share.
type commonFlags struct {
	config string // the folder with PINS, manifest.txt, ...
	cache  string // the folder of the tarball and the extracted tree
	fetch  bool
	root   string // use this folder (with test/) instead of the cache
}

// configDir returns the configuration folder.
func (c *commonFlags) configDir() (string, error) {
	if c.config != "" {
		return c.config, nil
	}
	r, err := repoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "tools", "gotest"), nil
}

// readConfig reads a file of the configuration folder; a missing file is
// empty text when optional.
func (c *commonFlags) readConfig(name string, optional bool) (string, error) {
	dir, err := c.configDir()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if optional && os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// loadPin reads PINS.
func (c *commonFlags) loadPin() (Pin, error) {
	text, err := c.readConfig("PINS", false)
	if err != nil {
		return Pin{}, err
	}
	return parsePin(text)
}

// sourceRoot returns the folder that has test/, from -root or the cache.
func (c *commonFlags) sourceRoot(pin Pin, log io.Writer) (string, error) {
	if c.root != "" {
		return c.root, nil
	}
	cache := c.cache
	if cache == "" {
		u, err := userCache()
		if err != nil {
			return "", err
		}
		cache = filepath.Join(u, "gocpp", "portstd")
	}
	s := &Source{Pin: pin, Cache: cache, Fetch: c.fetch, Log: log}
	return s.Root()
}

// register adds the shared flags to fs.
func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&c.config, "config", "", "the folder with PINS, manifest.txt, overrides.txt, ... (default: tools/gotest of the repository)")
	fs.StringVar(&c.cache, "cache", "", "the cache folder of the Go tarball (default: $XDG_CACHE_HOME/gocpp/portstd, shared with tools/portstd)")
	fs.StringVar(&c.root, "root", "", "a folder that has test/ (default: extracted from the pinned tarball in the cache)")
	fs.BoolVar(&c.fetch, "fetch", !envflag.Offline(os.Getenv("GOCPP_OFFLINE")), "download the tarball when the cache lacks it (default off when GOCPP_OFFLINE is set)")
}

// newFlagSet returns a flag set that prints its errors to w.
func newFlagSet(name string, w io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(w)
	return fs
}
