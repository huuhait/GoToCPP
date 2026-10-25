package main

// The committed output: a stable JSON file per target and the README that
// summarises it. Both are sorted (package, file, line) so that a later Go
// version can be diffed against this one.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const schemaVersion = 1

// The numbers of Phase 16 README 2.7 and 7.1 for the net/http + crypto/tls
// closure at go1.25 on linux/amd64. They are the comparison, not a target:
// the tool reports what it measures.
const (
	planVersion             = "go1.25"
	planTargets             = 166
	planRuntimeTargets      = 149
	planSynctestOnly        = 9
	planOtherTargets        = 17
	planWindowsOnlyBodyless = 17
)

// A Package is the dry run of one package.
type Package struct {
	Path             string         `json:"path"`
	Name             string         `json:"name"`
	Dir              string         `json:"dir"` // relative to GOROOT
	InClosure        bool           `json:"in_closure"`
	Files            []string       `json:"files"`
	Excluded         []ExcludedFile `json:"excluded"`
	DroppedByGocpp   []string       `json:"dropped_by_gocpp"`
	AddedByGocpp     []string       `json:"added_by_gocpp"`
	SFiles           []string       `json:"s_files"`
	CgoFiles         []string       `json:"cgo_files"`
	TestFiles        int            `json:"test_files"`
	NoPureGoFallback bool           `json:"no_pure_go_fallback"`
	Bodyless         []Bodyless     `json:"bodyless"`
	Linknames        []Linkname     `json:"linknames"`
	Intrinsics       []Intrinsic    `json:"intrinsics"`
}

// A KindCount is the number of linkname directives of one kind.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// Counts is the summary of one package subset.
type Counts struct {
	Packages                      int         `json:"packages"`
	Bodyless                      int         `json:"bodyless"`
	Linknames                     int         `json:"linknames"`
	LinknameKinds                 []KindCount `json:"linkname_kinds"`
	DistinctTargets               int         `json:"distinct_targets"`
	RuntimeHookTargets            int         `json:"runtime_hook_targets"`      // pushes from the runtime + pulls of runtime symbols
	RuntimePushTargets            int         `json:"runtime_push_targets"`      // the first half of RuntimeHookTargets
	RuntimePullTargets            int         `json:"runtime_pull_targets"`      // the second half
	RuntimeNamespaceTargets       int         `json:"runtime_namespace_targets"` // targets named runtime.*, from any package
	BodylessTargets               int         `json:"bodyless_targets"`          // targets whose symbol has no body in its package
	SynctestOnlyTargets           int         `json:"synctest_only_targets"`
	BodylessTargetsOutsideRuntime int         `json:"bodyless_targets_outside_runtime"` // not provided by the runtime
	BodylessTargetsUnprovided     int         `json:"bodyless_targets_unprovided"`      // not provided by any push either
	AssemblyPackages              []string    `json:"assembly_packages"`
	CgoPackages                   []string    `json:"cgo_packages"`
	GocppDroppedFiles             int         `json:"gocpp_dropped_files"`
	GocppAddedFiles               int         `json:"gocpp_added_files"`
}

// Summary is the whole run: every analyzed package, and the closure subset.
type Summary struct {
	All     Counts `json:"all"`
	Closure Counts `json:"closure"`
}

// Result is one dry run, the content of one JSON file.
type Result struct {
	Schema        int           `json:"schema"`
	Tool          string        `json:"tool"`
	GoVersion     string        `json:"go_version"`
	GoMinor       string        `json:"go_minor"`
	GOROOT        string        `json:"goroot"`
	Tags          *TagSet       `json:"tags"`
	ToolDefaults  *ToolDefaults `json:"tool_defaults"`
	PackageSet    []string      `json:"package_set"`
	ClosureRoots  []string      `json:"closure_roots"`
	ClosureWalked bool          `json:"closure_walked"`
	Packages      []Package     `json:"packages"`
	Summary       Summary       `json:"summary"`
}

// writeJSON writes res as stable, indented JSON.
func writeJSON(path string, res *Result) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// readJSON reads a dry run file.
func readJSON(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var res Result
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Schema != schemaVersion {
		return nil, fmt.Errorf("%s: schema %d, want %d", path, res.Schema, schemaVersion)
	}
	return &res, nil
}

// normalize turns nil slices into empty ones, so that the JSON says [] and not
// null: the file is compared with a later version's file.
func normalize(res *Result) {
	if res.PackageSet == nil {
		res.PackageSet = []string{}
	}
	if res.ClosureRoots == nil {
		res.ClosureRoots = []string{}
	}
	if res.Packages == nil {
		res.Packages = []Package{}
	}
	for i := range res.Packages {
		p := &res.Packages[i]
		for _, s := range []*[]string{&p.Files, &p.DroppedByGocpp, &p.AddedByGocpp, &p.SFiles, &p.CgoFiles} {
			if *s == nil {
				*s = []string{}
			}
		}
		if p.Excluded == nil {
			p.Excluded = []ExcludedFile{}
		}
		if p.Bodyless == nil {
			p.Bodyless = []Bodyless{}
		}
		if p.Linknames == nil {
			p.Linknames = []Linkname{}
		}
		if p.Intrinsics == nil {
			p.Intrinsics = []Intrinsic{}
		}
	}
	normalizeCounts(&res.Summary.All)
	normalizeCounts(&res.Summary.Closure)
}

func normalizeCounts(c *Counts) {
	if c.LinknameKinds == nil {
		c.LinknameKinds = []KindCount{}
	}
	if c.AssemblyPackages == nil {
		c.AssemblyPackages = []string{}
	}
	if c.CgoPackages == nil {
		c.CgoPackages = []string{}
	}
}

// summarize computes the counts of a subset of the packages.
func summarize(pkgs []Package) Counts {
	c := Counts{}
	known := packageSet(pkgs)
	bodyless := map[string]map[string]bool{} // package -> function name -> bodyless
	targets := map[string]bool{}
	runtimePush := map[string]bool{}
	runtimePull := map[string]bool{}
	runtimeNamespace := map[string]bool{}
	allPush := map[string]bool{}
	synctest := map[string]bool{}
	kinds := map[string]int{}
	for _, p := range pkgs {
		c.Packages++
		c.Bodyless += len(p.Bodyless)
		c.Linknames += len(p.Linknames)
		c.GocppDroppedFiles += len(p.DroppedByGocpp)
		c.GocppAddedFiles += len(p.AddedByGocpp)
		if len(p.SFiles) > 0 {
			c.AssemblyPackages = append(c.AssemblyPackages, p.Path)
		}
		if len(p.CgoFiles) > 0 {
			c.CgoPackages = append(c.CgoPackages, p.Path)
		}
		if bodyless[p.Path] == nil {
			bodyless[p.Path] = map[string]bool{}
		}
		for _, b := range p.Bodyless {
			bodyless[p.Path][b.Name] = true
		}
		for _, ln := range p.Linknames {
			kinds[ln.Kind]++
			if ln.Target == "" {
				continue
			}
			targets[ln.Target] = true
			tp := targetPackage(ln.Target, known)
			if tp == "runtime" {
				runtimeNamespace[ln.Target] = true
			}
			if ln.Kind == "push" {
				allPush[ln.Target] = true
			}
			if ln.Kind == "push" && p.Path == "runtime" && inSet(pkgs, tp) {
				runtimePush[ln.Target] = true
			}
			if ln.Kind == "pull" && tp == "runtime" {
				runtimePull[ln.Target] = true
			}
			if ln.Kind == "push" && p.Path == "runtime" && tp == "internal/synctest" {
				synctest[ln.Target] = true
			}
		}
	}
	c.DistinctTargets = len(targets)
	c.RuntimePushTargets = len(runtimePush)
	c.RuntimePullTargets = len(runtimePull)
	c.RuntimeNamespaceTargets = len(runtimeNamespace)
	hook := map[string]bool{}
	for t := range runtimePush {
		hook[t] = true
	}
	for t := range runtimePull {
		hook[t] = true
	}
	c.RuntimeHookTargets = len(hook)
	// The runtime provides a target when it declares the push or when the
	// symbol lives in the runtime's namespace (internal/runtime/maps pushes
	// mapaccess1 and its variants into it, for example).
	provided := map[string]bool{}
	for t := range runtimePush {
		provided[t] = true
	}
	for t := range runtimeNamespace {
		provided[t] = true
	}
	// A push from any package of the subset provides its target, not only a
	// push from the runtime (crypto/sha3 pushes sha3Unwrap, os pushes
	// net.newUnixFile, and so on).
	anyProvided := map[string]bool{}
	for t := range allPush {
		anyProvided[t] = true
	}
	for t := range runtimeNamespace {
		anyProvided[t] = true
	}
	c.SynctestOnlyTargets = len(synctest)
	bl := map[string]bool{}
	outside := map[string]bool{}
	unprovided := map[string]bool{}
	for t := range targets {
		tp := targetPackage(t, known)
		if !bodyless[tp][targetSymbol(t, tp)] {
			continue
		}
		bl[t] = true
		if !provided[t] {
			outside[t] = true
		}
		if !anyProvided[t] {
			unprovided[t] = true
		}
	}
	c.BodylessTargets = len(bl)
	c.BodylessTargetsOutsideRuntime = len(outside)
	c.BodylessTargetsUnprovided = len(unprovided)
	sort.Strings(c.AssemblyPackages)
	sort.Strings(c.CgoPackages)
	for _, k := range []string{"one-argument", "pull", "push", "variable", "linker", "unknown"} {
		if n := kinds[k]; n > 0 {
			c.LinknameKinds = append(c.LinknameKinds, KindCount{Kind: k, Count: n})
		}
	}
	return c
}

// inSet reports whether the package path p is one of pkgs.
func inSet(pkgs []Package, p string) bool {
	if p == "" {
		return false
	}
	for _, pkg := range pkgs {
		if pkg.Path == p {
			return true
		}
	}
	return false
}

// targetPackage returns the package part of a linkname target:
// "runtime.mapaccess1" -> "runtime", "runtime.funcInfo.entry" -> "runtime",
// "reflect.(*rtype).Align" -> "reflect", "go:fipsinfo" -> "" (a linker
// symbol). A dotted method symbol (runtime.funcInfo.entry) has no marker of
// its own, so the known package paths decide: the longest one that is a prefix
// followed by a dot. For a target package outside the analyzed set the last
// dot (or the text before the first parenthesis) is the best available guess.
func targetPackage(target string, known map[string]bool) string {
	best := ""
	for p := range known {
		if strings.HasPrefix(target, p+".") && len(p) > len(best) {
			best = p
		}
	}
	if best != "" {
		return best
	}
	if i := strings.Index(target, "("); i >= 0 {
		if j := strings.LastIndex(target[:i], "."); j >= 0 {
			return target[:j]
		}
		return ""
	}
	j := strings.LastIndex(target, ".")
	if j < 0 {
		return ""
	}
	return target[:j]
}

// targetSymbol returns the symbol part of a linkname target.
func targetSymbol(target, pkg string) string {
	if pkg != "" && strings.HasPrefix(target, pkg+".") {
		return target[len(pkg)+1:]
	}
	j := strings.LastIndex(target, ".")
	if j < 0 {
		return target
	}
	return target[j+1:]
}

// packageSet returns the paths of pkgs as a set.
func packageSet(pkgs []Package) map[string]bool {
	set := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		set[p.Path] = true
	}
	return set
}

// writeMarkdown writes the README for res. siblings are the other dry runs of
// the same Go version in the same directory; the Windows-only bodyless
// functions are computed from the linux and windows runs.
func writeMarkdown(w io.Writer, res *Result, siblings []*Result) error {
	byTarget := map[string]*Result{}
	for _, r := range siblings {
		byTarget[r.Tags.GOOS+"/"+r.Tags.GOARCH] = r
	}
	// The fresh run wins over a stale JSON for the same target.
	byTarget[res.Tags.GOOS+"/"+res.Tags.GOARCH] = res
	// The plan's numbers and the tag table are for linux/amd64. Use that run
	// when it exists, so the README does not depend on which target ran last.
	ref := byTarget["linux/amd64"]
	if ref == nil {
		ref = res
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# Go standard library dry run: %s\n\n", res.GoVersion)
	fmt.Fprintf(&b, "Generated by `tools/gostddryrun` from the Go source tree at `%s`.\n", res.GOROOT)
	b.WriteString("Do not edit by hand: rerun the commands at the end of this file.\n\n")
	b.WriteString("The `goroot` field of each JSON records the path that was read. It is provenance only: the\n")
	b.WriteString("same Go version at another path changes it, so ignore it when diffing two Go versions.\n\n")

	b.WriteString("## What this is\n\n")
	b.WriteString("The input of steps 16.9 to 16.11: everything in Go's standard library that the GC engine\n")
	b.WriteString("must replace under the build constraints of Phase 16 README 2.7. For each package in the\n")
	b.WriteString("default set, the tool reports the bodyless functions (with their signatures), every\n")
	b.WriteString("`//go:linkname` of each kind, the compiler intrinsics, the `.s` files and the packages whose\n")
	b.WriteString("assembly has no pure-Go fallback, the files that the `gocpp` tag drops or adds compared with\n")
	b.WriteString("`gc`, and the cgo files. It reads only the source tree: no `go` command, no network.\n\n")
	b.WriteString("The default package set is the list of step 16.1 (runtime, reflect, internal/reflectlite,\n")
	b.WriteString("internal/runtime/maps, sync, internal/sync, time, os, syscall, net, net/http, crypto/tls,\n")
	b.WriteString("testing) plus the import closure of `net/http` and `crypto/tls`. The closure is walked by\n")
	b.WriteString("reading the selected files' imports and resolving them in `GOROOT/src` (a vendored package\n")
	b.WriteString("keeps its `vendor/` path, as Phase 16 README 2.3 rule 4 says).\n\n")

	b.WriteString("## Build constraints\n\n")
	fmt.Fprintf(&b, "The table is for %s/%s; each JSON records its exact set under `tags`.\n", ref.Tags.GOOS, ref.Tags.GOARCH)
	if len(byTarget) > 1 {
		b.WriteString("A different target differs in its `GOOS`/`GOARCH` and, on Linux, in the `unix` tag.\n")
	}
	b.WriteString("\n")
	b.WriteString("| kind | tags |\n|---|---|\n")
	fmt.Fprintf(&b, "| GOOS, GOARCH | `%s`, `%s` |\n", ref.Tags.GOOS, ref.Tags.GOARCH)
	fmt.Fprintf(&b, "| compiler tag | `%s` (neither `gc` nor `gccgo`) |\n", ref.Tags.Compiler)
	b.WriteString("| cgo | `CGO_ENABLED=0`: cgo files are dropped and reported |\n")
	fmt.Fprintf(&b, "| release tags | %s (`%s` to `%s`) |\n", codeList(ref.Tags.ReleaseTags), ref.Tags.ReleaseTags[0], ref.Tags.ReleaseTags[len(ref.Tags.ReleaseTags)-1])
	fmt.Fprintf(&b, "| tool tags | %s |\n", codeList(ref.Tags.ToolTags))
	fmt.Fprintf(&b, "| build tags | %s |\n", codeList(ref.Tags.BuildTags))
	b.WriteString("\nEvery one of these is set explicitly on `go/build.Context` (`Compiler`, `BuildTags`,\n")
	b.WriteString("`ToolTags`, `ReleaseTags`, `GOOS`, `GOARCH`, `CgoEnabled`); the tool builds the context from\n")
	b.WriteString("scratch and never reads `build.Default`. It does not use `ImportDir`, which refuses a compiler\n")
	b.WriteString("other than `gc` or `gccgo`: it walks the directory and asks `Context.MatchFile`, the go/build\n")
	b.WriteString("rule for one file name.\n\n")

	b.WriteString("### Where the tool tags came from\n\n")
	b.WriteString("Read from the target tree, never from the Go that built the tool (whose `runtime.Version()`\n")
	b.WriteString("and whose compiled-in `go/build` defaults are ignored; only `runtime.GOROOT()` supplies the\n")
	b.WriteString("default value of `-goroot`). `GOEXPERIMENT`, `GOAMD64` and `GOARM64` in the environment are\n")
	b.WriteString("ignored too.\n\n")
	b.WriteString("| file:line | what |\n|---|---|\n")
	for _, s := range res.ToolDefaults.Sources {
		fmt.Fprintf(&b, "| `%s:%d` | %s |\n", s.File, s.Line, mdEscape(s.What))
	}
	b.WriteString("\n")
	if res.ToolDefaults.Verified {
		fmt.Fprintf(&b, "The explicit table entry `%s` in `tools/gostddryrun/tags.go` pins these values, and the\n", res.ToolDefaults.Table)
		b.WriteString("tree agreed with it (a disagreement is an error).\n\n")
	} else {
		fmt.Fprintf(&b, "No table entry exists for `%s`: the values above were derived from the tree and are not\n", res.GoMinor)
		b.WriteString("checked against a pinned set. Add one to `tools/gostddryrun/tags.go` before relying on them.\n\n")
	}
	fmt.Fprintf(&b, "Enabled experiments: %s.\n", codeList(res.ToolDefaults.Experiments))
	fmt.Fprintf(&b, "`defaultGOEXPERIMENT` = %s, `DefaultGOAMD64` = %s, `DefaultGOARM64` = %s.\n\n",
		quote(res.ToolDefaults.DefaultGOEXPERIMENT), quote(res.ToolDefaults.DefaultGOAMD64), quote(res.ToolDefaults.DefaultGOARM64))

	b.WriteString("## Counts\n\n")
	var targets []string
	for t := range byTarget {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	rows := []struct {
		name string
		f    func(Counts) string
	}{
		{"packages", func(c Counts) string { return fmt.Sprint(c.Packages) }},
		{"bodyless functions", func(c Counts) string { return fmt.Sprint(c.Bodyless) }},
		{"`//go:linkname` directives", func(c Counts) string { return fmt.Sprint(c.Linknames) }},
		{"distinct linkname targets", func(c Counts) string { return fmt.Sprint(c.DistinctTargets) }},
		{"  bodyless in their target package", func(c Counts) string { return fmt.Sprint(c.BodylessTargets) }},
		{"runtime hook targets (push + pull)", func(c Counts) string { return fmt.Sprint(c.RuntimeHookTargets) }},
		{"  pushed by the runtime", func(c Counts) string { return fmt.Sprint(c.RuntimePushTargets) }},
		{"  pulled from the runtime", func(c Counts) string { return fmt.Sprint(c.RuntimePullTargets) }},
		{"  of them only for internal/synctest", func(c Counts) string { return fmt.Sprint(c.SynctestOnlyTargets) }},
		{"targets in the runtime namespace", func(c Counts) string { return fmt.Sprint(c.RuntimeNamespaceTargets) }},
		{"bodyless targets not provided by the runtime", func(c Counts) string { return fmt.Sprint(c.BodylessTargetsOutsideRuntime) }},
		{"  of them provided by no push at all", func(c Counts) string { return fmt.Sprint(c.BodylessTargetsUnprovided) }},
		{"packages with `.s` files", func(c Counts) string { return fmt.Sprint(len(c.AssemblyPackages)) }},
		{"packages with cgo files", func(c Counts) string { return fmt.Sprint(len(c.CgoPackages)) }},
		{"files dropped by the `gocpp` tag", func(c Counts) string { return fmt.Sprint(c.GocppDroppedFiles) }},
		{"files added by the `gocpp` tag", func(c Counts) string { return fmt.Sprint(c.GocppAddedFiles) }},
	}
	b.WriteString("| count |")
	for _, t := range targets {
		fmt.Fprintf(&b, " %s |", t)
	}
	b.WriteString("\n|---|")
	for range targets {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s |", row.name)
		for _, t := range targets {
			fmt.Fprintf(&b, " %s |", row.f(byTarget[t].Summary.Closure))
		}
		b.WriteString("\n")
	}
	b.WriteString("\nThe counts are for the closure subset (the plan's numbers are for the net/http + crypto/tls\n")
	b.WriteString("closure). The JSON also has the counts of every analyzed package under `summary.all`.\n")
	if !res.ClosureWalked {
		b.WriteString("\nThe import closure was not walked (`-closure=false`): the closure subset is the named set.\n")
	}
	b.WriteString("\n")

	b.WriteString("### Linkname kinds\n\n")
	b.WriteString("| kind |")
	for _, t := range targets {
		fmt.Fprintf(&b, " %s |", t)
	}
	b.WriteString("\n|---|")
	for range targets {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, kind := range []string{"pull", "push", "one-argument", "variable", "linker", "unknown"} {
		fmt.Fprintf(&b, "| %s |", kind)
		for _, t := range targets {
			n := 0
			for _, kc := range byTarget[t].Summary.Closure.LinknameKinds {
				if kc.Kind == kind {
					n = kc.Count
				}
			}
			fmt.Fprintf(&b, " %d |", n)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n`pull`: a bodyless function that names another package's symbol. `push`: a function with a\n")
	b.WriteString("body that names another package's symbol. `one-argument`: exports the local symbol only.\n")
	b.WriteString("`variable`: a var, not a func. `linker`: a symbol with no package (`go:fipsinfo`).\n\n")

	// The runtime-provided targets and the synctest-only set of the reference
	// target (linux/amd64 when both JSONs are present).
	fmt.Fprintf(&b, "### The runtime's hooks (%s/%s)\n\n", ref.Tags.GOOS, ref.Tags.GOARCH)
	fmt.Fprintf(&b, "Distinct runtime hook targets: %d (%d pushed by the runtime into packages of the closure,\n",
		ref.Summary.Closure.RuntimeHookTargets, ref.Summary.Closure.RuntimePushTargets)
	fmt.Fprintf(&b, "%d pulled from the runtime). %d of them are only for `internal/synctest`.\n",
		ref.Summary.Closure.RuntimePullTargets, ref.Summary.Closure.SynctestOnlyTargets)
	b.WriteString("Targets in the runtime's own namespace (`runtime.*`, including the pushes from\n")
	fmt.Fprintf(&b, "`internal/runtime/maps`): %d.\n", ref.Summary.Closure.RuntimeNamespaceTargets)
	b.WriteString("`internal/synctest` itself uses the one-argument form, so the runtime's pushes into it are\n")
	b.WriteString("the whole set; they are the synctest hooks of Phase 16 README 2.7 and step 16.10.\n\n")

	bt := bodylessTargetsOf(ref)
	unprovidedSet := map[string]bool{}
	for _, t := range bt.Unprovided {
		unprovidedSet[t] = true
	}
	fmt.Fprintf(&b, "### Bodyless targets and who provides them (%s/%s)\n\n", ref.Tags.GOOS, ref.Tags.GOARCH)
	fmt.Fprintf(&b, "%d linkname targets in the closure are bodyless in their target package. %d are\n",
		ref.Summary.Closure.BodylessTargets, ref.Summary.Closure.BodylessTargets-ref.Summary.Closure.BodylessTargetsOutsideRuntime)
	fmt.Fprintf(&b, "provided by the runtime (a symbol in its namespace or a push from it); %d are not.\n\n",
		ref.Summary.Closure.BodylessTargetsOutsideRuntime)
	b.WriteString("Of the targets the runtime does not provide, these are provided by a push from another\n")
	b.WriteString("package of the closure, so they need no native of their own:\n\n")
	for _, t := range bt.Outside {
		if unprovidedSet[t] {
			continue
		}
		if ps := bt.Providers[t]; len(ps) > 0 {
			fmt.Fprintf(&b, "- `%s` (pushed by %s)\n", t, codeList(ps))
		} else {
			fmt.Fprintf(&b, "- `%s`\n", t)
		}
	}
	b.WriteString("\nThese are provided by no push at all and need a native or an overlay:\n\n")
	for _, t := range bt.Unprovided {
		fmt.Fprintf(&b, "- `%s`\n", t)
	}
	b.WriteString("\n")
	if win, ok := byTarget["windows/amd64"]; ok && win != ref {
		wc := win.Summary.Closure
		fmt.Fprintf(&b, "The windows/amd64 JSON has the same data: %d bodyless targets, %d provided by the runtime,\n",
			wc.BodylessTargets, wc.BodylessTargets-wc.BodylessTargetsOutsideRuntime)
		fmt.Fprintf(&b, "%d not, %d of those provided by another package and %d unprovided.\n\n",
			wc.BodylessTargetsOutsideRuntime, wc.BodylessTargetsOutsideRuntime-wc.BodylessTargetsUnprovided, wc.BodylessTargetsUnprovided)
	}

	b.WriteString("## Compiler intrinsics\n\n")
	var intrinsics []Intrinsic
	for _, p := range ref.Packages {
		intrinsics = append(intrinsics, p.Intrinsics...)
	}
	if len(intrinsics) == 0 {
		b.WriteString("None found.\n\n")
	} else {
		b.WriteString("| package | file:line | name |\n|---|---|---|\n")
		for _, in := range intrinsics {
			fmt.Fprintf(&b, "| `%s` | `%s:%d` | `%s` |\n", in.Package, in.File, in.Line, in.Name)
		}
		b.WriteString("\n")
	}
	b.WriteString("These are bodyless functions whose implementation is the compiler, not a linkname. They are\n")
	b.WriteString("natives in the GC engine (Phase 16 README 2.7).\n\n")

	b.WriteString("## Packages with `.s` files and no pure-Go fallback\n\n")
	b.WriteString("The constraints include `purego` and `math_big_pure_go`, so a bodyless function that remains\n")
	b.WriteString("in a package with `.s` files has no pure-Go variant under gocpp's constraints. Each such\n")
	b.WriteString("package needs an overlay or a native for its bodyless functions; the overlay list of README\n")
	b.WriteString("2.7 is a subset (a package whose bodyless functions are all natives, such as\n")
	b.WriteString("`internal/runtime/atomic`, needs no overlay). The bodyless functions themselves, with their\n")
	b.WriteString("signatures, are in the JSON.\n\n")
	b.WriteString("| target | packages |\n|---|---|\n")
	for _, t := range targets {
		var names []string
		for _, p := range byTarget[t].Packages {
			if p.NoPureGoFallback {
				names = append(names, p.Path)
			}
		}
		fmt.Fprintf(&b, "| %s | %s |\n", t, codeList(names))
	}
	b.WriteString("\n")

	b.WriteString("## cgo files\n\n")
	b.WriteString("Files that a cgo-enabled build would compile for the target (a context with `CgoEnabled=true`\n")
	b.WriteString("and every other constraint the same). The dry run drops them: `CGO_ENABLED=0`, as Phase 16\n")
	b.WriteString("README 2.7 says.\n\n")
	b.WriteString("| target | packages and files |\n|---|---|\n")
	for _, t := range targets {
		var parts []string
		for _, p := range byTarget[t].Packages {
			if len(p.CgoFiles) > 0 {
				parts = append(parts, fmt.Sprintf("`%s` (%s)", p.Path, codeList(p.CgoFiles)))
			}
		}
		if len(parts) == 0 {
			parts = []string{"none"}
		}
		fmt.Fprintf(&b, "| %s | %s |\n", t, strings.Join(parts, ", "))
	}
	b.WriteString("\n")

	b.WriteString("## Files the `gocpp` tag drops and adds\n\n")
	b.WriteString("Compared with the `gc` compiler tag, with every other constraint the same. A file with\n")
	b.WriteString("`//go:build gc` is dropped under gocpp; a file that gocpp adds has `//go:build gocpp` or\n")
	b.WriteString("`//go:build !gc`. These are the packages that need an overlay for the file, as Phase 16 README 2.7\n")
	b.WriteString("describes for `internal/goroot`, `x/sys/cpu` and `x/sys/unix`. The default package set is the\n")
	b.WriteString("net/http + crypto/tls closure, and `internal/goroot` (the package `go/build` imports) is not in\n")
	b.WriteString("it, so a zero here is expected. To see those overlays, name the packages:\n")
	b.WriteString("`-packages internal/goroot,vendor/golang.org/x/sys/cpu`.\n\n")
	for _, t := range targets {
		r := byTarget[t]
		fmt.Fprintf(&b, "### %s\n\n", t)
		found := false
		for _, p := range r.Packages {
			if len(p.DroppedByGocpp) == 0 && len(p.AddedByGocpp) == 0 {
				continue
			}
			found = true
			fmt.Fprintf(&b, "- `%s`:", p.Path)
			if len(p.DroppedByGocpp) > 0 {
				fmt.Fprintf(&b, " dropped %s", codeList(p.DroppedByGocpp))
			}
			if len(p.AddedByGocpp) > 0 {
				fmt.Fprintf(&b, " added %s", codeList(p.AddedByGocpp))
			}
			b.WriteString("\n")
		}
		if !found {
			b.WriteString("No file in the analyzed packages changes with the compiler tag.\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Windows-only bodyless functions\n\n")
	if win, lin := byTarget["windows/amd64"], byTarget["linux/amd64"]; win != nil && lin != nil {
		only := bodylessOnlyIn(win, lin)
		var outside []string
		for _, name := range only {
			if !strings.HasPrefix(name, "runtime.") {
				outside = append(outside, name)
			}
		}
		fmt.Fprintf(&b, "%d bodyless functions exist only in the windows/amd64 run; %d of them are outside\n",
			len(only), len(outside))
		b.WriteString("the `runtime` package (which the GC engine replaces as a whole, so its functions are not\n")
		b.WriteString("natives). The plan's 17 (README 2.7) matches the outside-the-runtime count.\n\n")
		b.WriteString("| package:function |\n|---|\n")
		for _, name := range outside {
			fmt.Fprintf(&b, "| `%s` |\n", name)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("Both the linux/amd64 and windows/amd64 JSON files must be present for this section.\n\n")
	}

	b.WriteString("## Comparison with the plan\n\n")
	if ref.GoMinor != planVersion {
		fmt.Fprintf(&b, "Phase 16 README 2.7 and 7.1 give their numbers for %s; this run is %s, so the numbers are\n",
			planVersion, ref.GoVersion)
		b.WriteString("not comparable. The JSON has the measured counts.\n\n")
	} else if ref.Tags.GOOS != "linux" || ref.Tags.GOARCH != "amd64" {
		fmt.Fprintf(&b, "Phase 16 README 2.7 and 7.1 give their numbers for linux/amd64; the reference target of\n")
		fmt.Fprintf(&b, "this README is %s/%s, so the plan comparison is not shown. The JSON has the measured counts.\n\n",
			ref.Tags.GOOS, ref.Tags.GOARCH)
	} else {
		c := ref.Summary.Closure
		known := packageSet(ref.Packages)
		closureSet := map[string]bool{}
		for _, p := range ref.Packages {
			if p.InClosure {
				closureSet[p.Path] = true
			}
		}
		// Runtime pushes by target package, and the legacy aliases the runtime
		// pushes into its own namespace.
		rtPushByPkg := map[string]map[string]bool{}
		rtSelf := map[string]bool{}
		mapsIntoRuntime := map[string]bool{}
		for _, p := range ref.Packages {
			if !p.InClosure {
				continue
			}
			for _, ln := range p.Linknames {
				if ln.Kind != "push" || ln.Target == "" {
					continue
				}
				tp := targetPackage(ln.Target, known)
				if p.Path == "runtime" && closureSet[tp] {
					if rtPushByPkg[tp] == nil {
						rtPushByPkg[tp] = map[string]bool{}
					}
					rtPushByPkg[tp][ln.Target] = true
					if tp == "runtime" {
						rtSelf[ln.Target] = true
					}
				}
				if p.Path == "internal/runtime/maps" && tp == "runtime" {
					mapsIntoRuntime[ln.Target] = true
				}
			}
		}
		byPkg := func(pkg string) int { return len(rtPushByPkg[pkg]) }

		b.WriteString("Phase 16 README 2.7 and 7.1 give one number for the net/http + crypto/tls closure at\n")
		fmt.Fprintf(&b, "go1.25 on linux/amd64: %d distinct `//go:linkname` targets, %d \"in the runtime\" (%d of\n",
			planTargets, planRuntimeTargets, planSynctestOnly)
		fmt.Fprintf(&b, "them only for `internal/synctest`) and %d \"between other standard packages or to the\n", planOtherTargets)
		b.WriteString("linker\". The tool does not adjust the plan; it measures three different definitions, none\n")
		b.WriteString("of which is that set:\n\n")
		b.WriteString("| measured definition | value |\n|---|---|\n")
		fmt.Fprintf(&b, "| distinct targets of two-argument directives, both directions | %d (%d when testing is included) |\n",
			c.DistinctTargets, ref.Summary.All.DistinctTargets)
		fmt.Fprintf(&b, "| targets bodyless in their target package | %d (%d provided by the runtime, %d not, %d of those by no push) |\n",
			c.BodylessTargets, c.BodylessTargets-c.BodylessTargetsOutsideRuntime, c.BodylessTargetsOutsideRuntime, c.BodylessTargetsUnprovided)
		fmt.Fprintf(&b, "| runtime hook targets (pushes from the runtime into closure packages + pulls of runtime symbols) | %d = %d + %d |\n\n",
			c.RuntimeHookTargets, c.RuntimePushTargets, c.RuntimePullTargets)
		linkerTargets := map[string]bool{}
		for _, p := range ref.Packages {
			if !p.InClosure {
				continue
			}
			for _, ln := range p.Linknames {
				if ln.Kind == "linker" && ln.Target != "" {
					linkerTargets[ln.Target] = true
				}
			}
		}
		fmt.Fprintf(&b, "The plan's %d is close to the %d runtime pushes (%d more here), but its %d cannot be\n",
			planRuntimeTargets, c.RuntimePushTargets, c.RuntimePushTargets-planRuntimeTargets, planOtherTargets)
		fmt.Fprintf(&b, "mapped to a single measured set: the closest exact %d is the %d pushes from\n", planOtherTargets, len(mapsIntoRuntime))
		b.WriteString("`internal/runtime/maps` into the runtime, yet the plan also says \"or to the linker\", and the\n")
		fmt.Fprintf(&b, "tree has %d linker targets. The plan does not state its counting method, so its definitions\n", len(linkerTargets))
		b.WriteString("cannot be reproduced from the plan text; the manager should pick one and update 2.7/7.1.\n\n")
		b.WriteString("| check | plan | measured |\n|---|---|---|\n")
		fmt.Fprintf(&b, "| only for internal/synctest | %d | %d (%s) |\n",
			planSynctestOnly, c.SynctestOnlyTargets, sameWord(planSynctestOnly, c.SynctestOnlyTargets))
		if win, lin := byTarget["windows/amd64"], byTarget["linux/amd64"]; win != nil && lin != nil {
			n := 0
			for _, name := range bodylessOnlyIn(win, lin) {
				if !strings.HasPrefix(name, "runtime.") {
					n++
				}
			}
			fmt.Fprintf(&b, "| Windows-only bodyless functions outside the runtime | %d | %d (%s) |\n",
				planWindowsOnlyBodyless, n, sameWord(planWindowsOnlyBodyless, n))
		}
		b.WriteString("\n### Other plan claims\n\n")
		b.WriteString("These are the rest of 2.7 and 7.1 that the dry run can check. They are reported so the\n")
		b.WriteString("manager can update the plan in one pass; none of them is adjusted here.\n\n")
		b.WriteString("| claim (2.7 / 7.1) | measured |\n|---|---|\n")
		fmt.Fprintf(&b, "| the net/http + crypto/tls closure is 178 packages (7.1) | %d (`go list -deps` gives %d, because it counts `unsafe`, which is not a package) |\n", c.Packages, c.Packages+1)
		fmt.Fprintf(&b, "| `internal/runtime/maps` pushes 19 functions into the runtime (2.7) | %d |\n", len(mapsIntoRuntime))
		fmt.Fprintf(&b, "| the runtime has 154 such functions for net/http + crypto/tls (2.7) | %d runtime pushes, %d runtime hook targets |\n",
			c.RuntimePushTargets, c.RuntimeHookTargets)
		b.WriteString("| 187 with testing and its test main (2.7) | not measured: the default set does not walk testing's closure (`internal/fuzz`, `os/signal`, `runtime/pprof`) |\n")
		fmt.Fprintf(&b, "| runtime pushes by package: reflect 37, sync 18, syscall 15, internal/poll 12, time 7 | reflect %d, sync %d, syscall %d, internal/poll %d, time %d |\n",
			byPkg("reflect"), byPkg("sync"), byPkg("syscall"), byPkg("internal/poll"), byPkg("time"))
		b.WriteString("\nThe runtime pushes include ")
		var selfAliases []string
		for t := range rtSelf {
			selfAliases = append(selfAliases, targetSymbol(t, "runtime"))
		}
		sort.Strings(selfAliases)
		fmt.Fprintf(&b, "%d pushes into its own namespace (the legacy aliases %s).\n\n", len(rtSelf), codeList(selfAliases))
	}

	b.WriteString("## Reproduce\n\n")
	b.WriteString("```sh\n")
	b.WriteString("GOROOT=$(scripts/in-devenv go env GOROOT)\n")
	for _, t := range targets {
		parts := strings.Split(t, "/")
		fmt.Fprintf(&b, "GOMAXPROCS=16 go run ./tools/gostddryrun -goroot \"$GOROOT\" -goos %s -goarch %s \\\n", parts[0], parts[1])
		fmt.Fprintf(&b, "    -out gostd/dryrun/%s/%s-%s.json -md gostd/dryrun/%s/README.md\n", res.GoVersion, parts[0], parts[1], res.GoVersion)
	}
	b.WriteString("```\n\n")
	b.WriteString("The README is written by the last command and reads the JSON files next to it for the\n")
	b.WriteString("two-target table and the Windows-only section.\n")
	return writeAll(w, b.String())
}

// bodylessOnlyIn returns the package.function names of a's bodyless functions
// that b does not have, sorted.
func bodylessOnlyIn(a, b *Result) []string {
	inB := map[string]bool{}
	for _, p := range b.Packages {
		for _, f := range p.Bodyless {
			inB[p.Path+"."+f.Name] = true
		}
	}
	var out []string
	for _, p := range a.Packages {
		for _, f := range p.Bodyless {
			name := p.Path + "." + f.Name
			if !inB[name] {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// A bodylessTargetInfo splits the closure's bodyless linkname targets by who
// provides them.
type bodylessTargetInfo struct {
	Outside    []string            // not provided by the runtime, sorted
	Unprovided []string            // not provided by any push either, sorted
	Providers  map[string][]string // target -> the closure packages that push it
}

// bodylessTargetsOf returns the bodyless targets of the closure and their
// providers. A target is provided by the runtime when its symbol lives in the
// runtime's namespace or the runtime pushes it; it is provided by another
// package when any package of the closure pushes it.
func bodylessTargetsOf(res *Result) bodylessTargetInfo {
	var closure []Package
	for _, p := range res.Packages {
		if p.InClosure {
			closure = append(closure, p)
		}
	}
	known := packageSet(res.Packages)
	// The bodyless symbols are those of the closure, the same subset that
	// summarize() counts, so the lists and the JSON counts agree.
	bodyless := map[string]map[string]bool{}
	for _, p := range closure {
		if bodyless[p.Path] == nil {
			bodyless[p.Path] = map[string]bool{}
		}
		for _, f := range p.Bodyless {
			bodyless[p.Path][f.Name] = true
		}
	}
	runtimeNamespace := map[string]bool{}
	runtimePush := map[string]bool{}
	anyPush := map[string]bool{}
	providers := map[string][]string{}
	for _, p := range closure {
		for _, ln := range p.Linknames {
			if ln.Target == "" {
				continue
			}
			tp := targetPackage(ln.Target, known)
			if tp == "runtime" {
				runtimeNamespace[ln.Target] = true
			}
			if ln.Kind == "push" {
				anyPush[ln.Target] = true
				providers[ln.Target] = appendUnique(providers[ln.Target], p.Path)
			}
			if ln.Kind == "push" && p.Path == "runtime" && inSet(closure, tp) {
				runtimePush[ln.Target] = true
			}
		}
	}
	runtimeProvided := map[string]bool{}
	for t := range runtimeNamespace {
		runtimeProvided[t] = true
	}
	for t := range runtimePush {
		runtimeProvided[t] = true
	}
	anyProvided := map[string]bool{}
	for t := range runtimeNamespace {
		anyProvided[t] = true
	}
	for t := range anyPush {
		anyProvided[t] = true
	}
	outside := map[string]bool{}
	unprovided := map[string]bool{}
	for _, p := range closure {
		for _, ln := range p.Linknames {
			if ln.Target == "" {
				continue
			}
			tp := targetPackage(ln.Target, known)
			if !bodyless[tp][targetSymbol(ln.Target, tp)] {
				continue
			}
			if !runtimeProvided[ln.Target] {
				outside[ln.Target] = true
			}
			if !anyProvided[ln.Target] {
				unprovided[ln.Target] = true
			}
		}
	}
	info := bodylessTargetInfo{Outside: sortedKeys(outside), Unprovided: sortedKeys(unprovided), Providers: map[string][]string{}}
	for _, t := range info.Outside {
		ps := append([]string{}, providers[t]...)
		sort.Strings(ps)
		info.Providers[t] = ps
	}
	return info
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

func codeList(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	var b strings.Builder
	for i, s := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "`%s`", s)
	}
	return b.String()
}

func sameWord(a, b int) string {
	if a == b {
		return "matches"
	}
	return "differs"
}

func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func writeAll(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
