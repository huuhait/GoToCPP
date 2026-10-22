// Command portstd ports packages of Go's standard library into stdlib/:
// Phase 09 step 9.1, Phase 09 README section 2.1, docs/guides/porting.md.
//
//	go run ./tools/portstd                    port every package of tools/portstd/ports.txt
//	go run ./tools/portstd unicode/utf8       port one package
//	go run ./tools/portstd -check             the committed ports are what the tool writes (make portstd-check)
//	go run ./tools/portstd -list strings      files, imports and blockers of a package and its closure
//	go run ./tools/portstd -list -tier tier1 -md   the same for a named list, as markdown
//	go run ./tools/portstd -pin go1.25.15     re-pin to another Go version (needs the network)
//
// The source is the pinned tarball of PINS, checked by SHA-256 and cached in
// $XDG_CACHE_HOME/gocpp/portstd, never the local GOROOT (NixOS patches it).
// Downloads happen only when the cache lacks the tarball and neither -offline
// nor GOCPP_OFFLINE=1 is set; -check never downloads unless -fetch is given
// (and still not offline). Without the tarball -check compares the files with
// the SHA-256 of stdlib/PORTED.sha256; GOCPP_REQUIRE_PORTSTD=1 (or -require)
// makes the missing tarball an error.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/envflag"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main for the tests: it returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("portstd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		out      = fs.String("o", "stdlib", "the folder that holds the ports (stdlib/)")
		check    = fs.Bool("check", false, "check that the committed ports are what the tool writes")
		list     = fs.Bool("list", false, "list files, imports and blockers of the packages and their imports")
		doc      = fs.String("doc", "", "with -list -md: write the table into the generated block of this file")
		md       = fs.Bool("md", false, "with -list: markdown, without the state of stdlib/")
		tier     = fs.String("tier", "", "with -list: a package list of tiers.txt")
		pin      = fs.String("pin", "", "re-pin PINS to this Go version (go1.25.15), from the official listing")
		osFlag   = fs.String("os", "", "GOOS list (default: ports.txt)")
		archFlag = fs.String("arch", "", "GOARCH for the file selection (default: ports.txt, generic)")
		tagsFlag = fs.String("tags", "", "build tags that are true (default: ports.txt, purego)")
		cache    = fs.String("cache", "", "the cache folder (default: $XDG_CACHE_HOME/gocpp/portstd)")
		cfgDir   = fs.String("config", "", "read PINS, ports.txt, natives.txt, ... from this folder (default: the tool's own)")
		fetch    = fs.Bool("fetch", false, "download the tarball when the cache lacks it (also with -check)")
		offline  = fs.Bool("offline", false, "never download (also GOCPP_OFFLINE=1)")
		require  = fs.Bool("require", requireFromEnv(), "with -check: fail instead of comparing by hash only when the cache has no tarball (default: GOCPP_REQUIRE_PORTSTD set)")
		force    = fs.Bool("force", false, "write a port that has blockers, or beside hand-written .gp files")
		listURL  = fs.String("listing", "https://go.dev/dl/?mode=json&include=all", "with -pin: the official download listing")
		dlBase   = fs.String("dl", "https://dl.google.com/go", "with -pin: where the tarballs are")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: go run ./tools/portstd [flags] [package...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "portstd: "+format+"\n", a...)
		return 1
	}

	cfg, err := loadConfig(*cfgDir)
	if err != nil {
		return fail("%v", err)
	}
	cacheDir := *cache
	if cacheDir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return fail("no cache folder (set XDG_CACHE_HOME or -cache): %v", err)
		}
		cacheDir = filepath.Join(base, "gocpp", "portstd")
	}
	noNet := *offline || envflag.Offline(os.Getenv("GOCPP_OFFLINE"))

	if *pin != "" {
		if noNet {
			return fail("-pin needs the network")
		}
		p, err := Repin(&http.Client{}, *listURL, *dlBase, *pin, cacheDir, stderr)
		if err != nil {
			return fail("%v", err)
		}
		text, err := pinText(*cfgDir, p)
		if err != nil {
			return fail("%v", err)
		}
		file := filepath.Join("tools", "portstd", "PINS")
		if *cfgDir != "" {
			file = filepath.Join(*cfgDir, "PINS")
		}
		if _, err := os.Stat(file); err != nil {
			fmt.Fprint(stdout, text)
			fmt.Fprintf(stderr, "portstd: %s: run from the repository root to have %s written; copy the lines above into it\n", *pin, file)
			return 0
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return fail("%v", err)
		}
		fmt.Fprintf(stdout, "portstd: %s written for %s (SHA-256 %s); now run go run ./tools/portstd, and review the diff of stdlib/\n", file, p.Version, p.SHA256)
		return 0
	}

	for _, w := range cfg.Warnings {
		fmt.Fprintln(stderr, "portstd: warning: "+w)
	}
	// -fetch allows the download in -check, but -offline and GOCPP_OFFLINE=1 still forbid it.
	src := &Source{Pin: cfg.Pin, Cache: cacheDir, Fetch: !noNet && (*fetch || !*check), Log: stderr}
	if *check && !src.Available() && !(*fetch && !noNet) {
		why := fmt.Sprintf("the cache %s has no %s.src.tar.gz (run go run ./tools/portstd -check -fetch once)", cacheDir, cfg.Pin.stem())
		if *require {
			fmt.Fprintf(stderr, "portstd: -check failed: %s; GOCPP_REQUIRE_PORTSTD or -require makes a missing source an error\n", why)
			return 1
		}
		// Without Go's source the check is by hash: a hand edit is still found.
		n, probs := checkSums(*out)
		for _, p := range probs {
			fmt.Fprintln(stderr, "portstd: "+p)
		}
		if len(probs) > 0 {
			fmt.Fprintf(stderr, "portstd: -check found %d problem(s) by hash (the byte comparison with Go's source was skipped: %s)\n", len(probs), why)
			return 1
		}
		fmt.Fprintf(stdout, "portstd: -check: the files of %d package(s) have their recorded SHA-256; the byte comparison with Go's source was skipped: %s\n", n, why)
		return 0
	}
	root, err := src.Root()
	if err != nil {
		return fail("%v", err)
	}
	porter := &Porter{Root: root, Cfg: cfg}

	optsFor := func(pkg string) Options {
		o := cfg.optionsFor(pkg)
		for _, ps := range cfg.Ports {
			if ps.Pkg == pkg {
				o = ps.Opts
			}
		}
		if *osFlag != "" {
			o.OS = splitList(*osFlag)
		}
		if *archFlag != "" {
			o.Arch = *archFlag
		}
		if *tagsFlag != "" {
			o.Tags = splitList(*tagsFlag)
		}
		return o
	}

	pkgs := fs.Args()
	if *tier != "" {
		l, ok := cfg.Tiers[*tier]
		if !ok {
			return fail("no package list %q in tiers.txt", *tier)
		}
		pkgs = append(slices.Clone(pkgs), l...)
	}

	if *list {
		if len(pkgs) == 0 {
			return fail("-list needs packages or -tier")
		}
		lr, err := porter.List(pkgs, optsFor)
		if err != nil {
			return fail("%v", err)
		}
		if *md {
			var b strings.Builder
			lr.WriteMarkdown(&b, cfg.Pin, optsFor(pkgs[0]))
			if *doc != "" {
				if err := updateDoc(*doc, b.String()); err != nil {
					return fail("%v", err)
				}
				fmt.Fprintf(stdout, "portstd: %s: table written\n", *doc)
			} else {
				fmt.Fprint(stdout, b.String())
			}
		} else {
			lr.WriteText(stdout, *out)
		}
		return 0
	}

	all := len(pkgs) == 0
	if all {
		for _, ps := range cfg.Ports {
			pkgs = append(pkgs, ps.Pkg)
		}
	}
	var results []*Result
	status := 0
	for _, pkg := range pkgs {
		res, err := porter.Port(pkg, optsFor(pkg))
		if err != nil {
			fmt.Fprintf(stderr, "portstd: %v\n", err)
			status = 1
			continue
		}
		if bl := res.Blockers(); len(bl) > 0 && !*force && !*check {
			fmt.Fprintf(stderr, "portstd: %s: not ported, %d blocker(s) (-force writes it anyway):\n", pkg, len(bl))
			for _, f := range bl {
				fmt.Fprintf(stderr, "    %s %s:%d: %s\n", f.Kind, f.File, f.Line, f.Detail)
			}
			status = 1
			continue
		}
		results = append(results, res)
	}
	if *check {
		var probs []string
		for _, r := range results {
			probs = append(probs, checkPort(*out, r)...)
		}
		probs = append(probs, checkIndex(*out, results, all && status == 0)...)
		probs = append(probs, checkSumsFile(*out, results, all && status == 0)...)
		if all && *cfgDir == "" {
			if l, ok := cfg.Tiers[defaultTier]; ok {
				if lr, err := porter.List(l, optsFor); err == nil {
					var b strings.Builder
					lr.WriteMarkdown(&b, cfg.Pin, optsFor(l[0]))
					probs = append(probs, checkDoc(defaultDoc, b.String())...)
				} else {
					probs = append(probs, err.Error())
				}
			}
		}
		for _, p := range probs {
			fmt.Fprintln(stderr, "portstd: "+p)
		}
		if len(probs) > 0 || status != 0 {
			return 1
		}
		fmt.Fprintf(stdout, "portstd: %d package(s) are what the port writes (%s)\n", len(results), cfg.Pin.Version)
		return 0
	}
	var written []*Result
	for _, r := range results {
		if err := prepareWrite(*out, r, cfg.Own[r.Pkg], *force, stderr); err != nil {
			fmt.Fprintf(stderr, "portstd: %v\n", err)
			status = 1
			continue
		}
		if err := writePort(*out, r); err != nil {
			return fail("%v", err)
		}
		written = append(written, r)
		fmt.Fprintf(stdout, "portstd: %s: %d file(s)%s\n", r.Pkg, len(r.Files), natives(r))
	}
	if err := updateIndex(*out, written); err != nil {
		return fail("%v", err)
	}
	if err := updateSums(*out, written); err != nil {
		return fail("%v", err)
	}
	return status
}

// requireFromEnv: GOCPP_REQUIRE_PORTSTD=1 (any value but empty and 0) makes -check fail without the source.
func requireFromEnv() bool {
	v := os.Getenv("GOCPP_REQUIRE_PORTSTD")
	return v != "" && v != "0"
}

func natives(r *Result) string {
	if len(r.Natives) == 0 {
		return ""
	}
	return fmt.Sprintf(", %d native(s)", len(r.Natives))
}

// loadConfig reads the configuration from dir, or the tool's embedded files.
func loadConfig(dir string) (*Config, error) {
	if dir != "" {
		return LoadConfig(os.DirFS(dir))
	}
	return LoadConfig(embeddedConfig)
}

// pinText returns the text of a PINS file for p: the explanation of the
// current file (from dir, or the embedded one) and the new three lines.
func pinText(dir string, p Pin) (string, error) {
	var old []byte
	var err error
	if dir != "" {
		old, err = os.ReadFile(filepath.Join(dir, "PINS"))
	} else {
		old, err = embeddedConfig.ReadFile("PINS")
	}
	if err != nil {
		return "", err
	}
	var header strings.Builder
	for _, line := range strings.SplitAfter(string(old), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			header.WriteString(line)
		}
	}
	return p.text(header.String()), nil
}
