package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/modfetch"
)

// cleanUsage is the help text of "gocpp clean". The command is minimal until step 6.12 (the CLI) gives
// it the rest of go clean's flags.
const cleanUsage = `usage: gocpp clean [-cache] [-gen] [-stamp] [-modcache] [-n] [-x]

gocpp clean removes what gocpp keeps in its cache folder ($XDG_CACHE_HOME/gocpp, ~/.cache/gocpp on
Linux; gocpp env prints it): the files that a build makes again when it needs them. Without flags it
removes nothing (the programs are written where -o says; gocpp leaves no object files in the project).

	-cache     remove everything except the module cache: the object cache (obj), the runtime and its
	           precompiled headers (rt), the generated C++ of the programs (gen), the records of
	           finished builds (stamp), what the compilers are (cxxid) and the probe results (probe)
	-gen       remove only the generated C++ (a program that is being built keeps its folder)
	-stamp     remove only the records of finished builds: every program is built (with the help of the
	           object cache) once more before gocpp build says it is up to date again
	-modcache  remove the module cache with the downloaded modules (they are downloaded again by the
	           next build that needs them)

A build that runs while the cache is removed can fail with a message about a file that is gone; run it
again. The cache is only an optimization: no build gives another result because of it.

The flags are:
`

// runClean is "gocpp clean".
func runClean(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var cache, gen, stamp, mod, dry, show bool
	flags := flag.NewFlagSet("gocpp clean", flag.ContinueOnError)
	flags.BoolVar(&cache, "cache", false, "remove the build cache (all but the module cache)")
	flags.BoolVar(&gen, "gen", false, "remove the generated C++")
	flags.BoolVar(&stamp, "stamp", false, "remove the records of finished builds")
	flags.BoolVar(&mod, "modcache", false, "remove the module cache")
	flags.BoolVar(&dry, "n", false, "print what would be removed, and remove nothing")
	flags.BoolVar(&show, "x", false, "print what is removed")
	if code, done := parseFlags("clean", cleanUsage, flags, args, stderr); done {
		return code
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "gocpp clean: unexpected argument %s (gocpp clean takes no packages)\n", flags.Arg(0))
		return 2
	}
	root := cacheDir()
	var names []string
	switch {
	case cache:
	case gen || stamp:
		if gen {
			names = append(names, "gen")
		}
		if stamp {
			names = append(names, "stamp")
		}
	case mod:
		names = []string{} // nothing of the build cache
	default:
		return 0
	}
	code := 0
	if cache || gen || stamp {
		if root == "" {
			fmt.Fprintln(stderr, "gocpp clean: there is no cache folder (neither XDG_CACHE_HOME nor HOME is set)")
			return exitErrors
		}
		// A folder of generated files whose lock is held belongs to a build that is running.
		inUse := func(folder, entry string) bool {
			if folder != "gen" {
				return false
			}
			if strings.HasSuffix(entry, ".lock") {
				return true // the locks stay: a process may hold one, and the file is tiny
			}
			l, busy, err := lockFile(filepath.Join(root, "gen", entry+".lock"))
			if err == nil && !busy {
				l.unlock()
			}
			return busy
		}
		removed, err := build.CleanCache(root, names, dry, inUse)
		for _, u := range removed {
			switch {
			case dry:
				fmt.Fprintf(stdout, "rm -rf %s # %d files, %s\n", filepath.Join(root, u.Name), u.Files, byteSize(u.Bytes))
			case show:
				fmt.Fprintf(stdout, "rm -rf %s # %d files, %s\n", filepath.Join(root, u.Name), u.Files, byteSize(u.Bytes))
			}
		}
		if err != nil {
			fmt.Fprintf(stderr, "gocpp clean: %v\n", err)
			code = exitErrors
		}
	}
	if mod {
		cfg, err := modfetch.ConfigFromEnv(os.Getenv, stderr)
		switch {
		case err != nil:
			fmt.Fprintf(stderr, "gocpp clean: %v\n", err)
			return exitErrors
		case cfg.CacheDir == "":
			fmt.Fprintln(stderr, "gocpp clean: there is no module cache")
			return exitErrors
		}
		files, size := build.UsageOf(cfg.CacheDir)
		if dry || show {
			fmt.Fprintf(stdout, "rm -rf %s # %d files, %s\n", cfg.CacheDir, files, byteSize(size))
		}
		if !dry {
			if err := modfetch.CleanModCache(cfg.CacheDir); err != nil {
				fmt.Fprintf(stderr, "gocpp clean: %v\n", err)
				code = exitErrors
			}
		}
	}
	return code
}

// byteSize writes a number of bytes the way du -h does.
func byteSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
