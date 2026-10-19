// Package modget is "gocpp get": it changes the requirements of a module
// (add, upgrade, downgrade, remove) with minimal version selection, downloads
// what changed, and writes gocpp.mod and gocpp.sum together.
//
// The semantics are those of "go get" for modules (docs/guides/modules.md):
//
//   - path@query asks for a version (latest, upgrade, patch, none, v1.2.3, a
//     prefix v1 or v1.2, a comparison, a branch, a tag or a commit); a query
//     that is lower than the version in the build list is a downgrade, which
//     also lowers every module that needs more than the downgrade allows;
//   - the build list is computed over the requirement graph of the module
//     files (gocpp.mod or go.mod) of the dependencies (internal/modgraph);
//   - gocpp.mod keeps one require line for every module of the build list: the
//     modules that the user named without "// indirect", the others with it;
//   - nothing is written unless everything worked, and the two files are
//     replaced by one rename each, under a lock that every gocpp that uses
//     the module cache respects.
package modget

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/modgraph"
	"github.com/huuhait/gocpp/internal/mvs"
)

// Options are the arguments of Run.
type Options struct {
	// Dir is the folder of the main module (gocpp.mod is in it).
	Dir string

	Fetcher *modfetch.Fetcher

	// Args are the modules: path or path@query.
	Args []string

	// Upgrade is "" for no -u, "all" for -u (the modules that Args name and
	// everything they need, or every module if there are no Args) and
	// "patch" for -u=patch (the latest patch release only).
	Upgrade string

	// Tests is -t. gocpp.mod's requirements are per module, not per package,
	// so the flag changes nothing; it is accepted for scripts that use it.
	Tests bool

	// DryRun computes everything and writes nothing.
	DryRun bool
}

// A Change is a module that the command added, upgraded, downgraded or removed.
type Change struct {
	Path string
	Old  string // "" if the module was not in the build list
	New  string // "" if it is not any more
}

// Verb is "added", "removed", "upgraded" or "downgraded".
func (c Change) Verb() string {
	switch {
	case c.Old == "":
		return "added"
	case c.New == "":
		return "removed"
	case gocppmod.CompareVersions(c.New, c.Old) < 0:
		return "downgraded"
	}
	return "upgraded"
}

// String is the line that gocpp get prints: "added example.com/a v1.2.0".
func (c Change) String() string {
	switch c.Verb() {
	case "added":
		return "added " + c.Path + " " + c.New
	case "removed":
		return "removed " + c.Path + " " + c.Old
	}
	return c.Verb() + " " + c.Path + " " + c.Old + " => " + c.New
}

// A Result says what Run did.
type Result struct {
	// Changes are the modules of the build list that changed, sorted by path.
	Changes []Change

	// Warnings are retracted versions and the like: the command worked.
	Warnings []string

	// Notes are things the user should know: a replaced module.
	Notes []string

	// ModChanged and SumChanged say which files were written (or would be).
	ModChanged, SumChanged bool

	// Downloaded is the modules that were fetched into the module cache.
	Downloaded []string
}

// An ArgError is an argument that can't be used.
type ArgError struct {
	Arg string
	Msg string
}

func (e *ArgError) Error() string { return fmt.Sprintf("%s: %s", e.Arg, e.Msg) }

type request struct {
	arg   string
	path  string
	query string

	version string // what the query resolved to; "none" removes
	named   bool
}

// Run is "gocpp get".
func Run(ctx context.Context, o Options) (*Result, error) {
	f := o.Fetcher
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	// Two gets in one project run one after the other: a lock on the project's
	// folder (also without a module cache), then one of the module cache.
	unlockProject, err := modfetch.LockProject(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer unlockProject()
	unlock, err := f.Lock(ctx, modfetch.LockName("get", dir))
	if err != nil {
		return nil, err
	}
	defer unlock()

	modPath := filepath.Join(dir, gocppmod.FileName)
	sumPath := filepath.Join(dir, modfetch.SumFileName)
	data, err := modfetch.ReadRegular(modPath, 16<<20)
	if err != nil {
		return nil, err
	}
	file, perrs := gocppmod.Parse(modPath, data, nil)
	if len(perrs) > 0 {
		var msgs []string
		for _, e := range perrs {
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("%s", strings.Join(msgs, "\n"))
	}
	for _, name := range []string{modPath, sumPath} {
		if err := modfetch.CheckWritable(name); err != nil && !o.DryRun {
			return nil, err
		}
	}

	pending := &modfetch.PendingSums{}
	src := &modgraph.FetcherSource{F: f, MainDir: dir, Pending: pending}
	cfg := &modgraph.Config{MainDir: dir, File: file, Source: src}
	reqs := modgraph.New(ctx, cfg)
	main := reqs.Main()
	allowed := func(path string) func(string) bool {
		return func(v string) bool { return !reqs.Excluded(path, v) }
	}
	cfg.Latest = func(ctx context.Context, path, cur string) (string, error) {
		kind := "upgrade"
		if o.Upgrade == "patch" {
			kind = "patch"
		}
		res, err := f.Query(ctx, dir, path, kind, modfetch.QueryOptions{Current: cur, Allowed: allowed(path), Pending: pending})
		if err != nil {
			return "", err
		}
		return res.Version, nil
	}

	// The build list now.
	old, oldGraph, err := reqs.BuildList()
	if err != nil {
		return nil, err
	}
	oldSel := map[string]string{}
	for _, m := range old[1:] {
		oldSel[m.Path] = m.Version
	}

	// The requests.
	res := &Result{}
	var rs []*request
	seen := map[string]bool{}
	for _, arg := range o.Args {
		path, query, _ := strings.Cut(arg, "@")
		if strings.Contains(arg, "@") && query == "" {
			return nil, &ArgError{arg, "missing version query after @"}
		}
		r := &request{arg: arg, path: path, query: query, named: true}
		if err := checkArg(file, main, r); err != nil {
			return nil, err
		}
		if seen[path] {
			return nil, &ArgError{arg, "module " + path + " is listed twice"}
		}
		seen[path] = true
		rs = append(rs, r)
	}
	for _, r := range rs {
		cur := oldGraph.Selected(r.path)
		if err := resolve(ctx, f, dir, file, reqs, pending, r, cur, res); err != nil {
			return nil, err
		}
	}

	// The edit, as "go get" does it (mvs.Edit): every named module has the version
	// asked for (or is gone, for none); with -u, what the named modules need (every
	// module, if none is named) is upgraded as far as that allows.
	var must, tryUp []mvs.Module
	named := map[string]string{}
	for _, r := range rs {
		named[r.path] = r.version
		cur := oldGraph.Selected(r.path)
		if r.version == mvs.None && cur == "" {
			res.Notes = append(res.Notes, fmt.Sprintf("%s is not required in %s: nothing to remove", r.path, gocppmod.FileName))
			continue
		}
		must = append(must, mvs.Module{Path: r.path, Version: r.version})
	}
	if o.Upgrade != "" {
		var seeds []mvs.Module
		if len(o.Args) == 0 {
			seeds = old[1:]
		} else {
			for _, u := range must {
				if u.Version == mvs.None {
					continue
				}
				rs, err := reqs.Required(u)
				if err != nil {
					return nil, err
				}
				seeds = append(seeds, rs...)
			}
		}
		more, err := latestClosure(reqs, seeds, func(path string) bool { return path == main.Path || named[path] != "" })
		if err != nil {
			return nil, err
		}
		tryUp = more
	}
	roots, err := mvs.Edit(reqs, main, old[1:], tryUp, must)
	if err != nil {
		return nil, err
	}
	list, err := mvs.BuildList([]mvs.Module{main}, &baseReqs{Reqs: reqs, main: main, list: roots})
	if err != nil {
		return nil, err
	}
	sel := map[string]string{}
	for _, m := range list[1:] {
		sel[m.Path] = m.Version
	}
	for _, r := range rs {
		got := sel[r.path]
		switch {
		case r.version == mvs.None && got != "":
			return nil, fmt.Errorf("%s@none: %s stays in the build list at %s", r.path, r.path, got)
		case r.version != mvs.None && got != r.version:
			return nil, fmt.Errorf("%s@%s: the build list selects %s at %q (another module requires a higher version)", r.path, r.query, r.path, got)
		}
	}

	// gocpp.mod.
	edited := edit(file, old, sel, named)
	newData, err := modfetch.FormatFile(modPath, edited)
	if err != nil {
		return nil, err
	}
	crlf := bytes.Contains(data, []byte("\r\n"))
	newFile, perrs := gocppmod.Parse(modPath, newData, nil)
	if len(perrs) > 0 {
		return nil, fmt.Errorf("internal error: the edited %s has errors: %v", gocppmod.FileName, perrs[0])
	}
	check := modgraph.New(ctx, &modgraph.Config{MainDir: dir, File: newFile, Source: src})
	got, _, err := check.BuildList()
	if err != nil {
		return nil, err
	}
	if len(got) != len(list) {
		return nil, fmt.Errorf("internal error: the new %s selects %d modules, not %d", gocppmod.FileName, len(got), len(list))
	}
	for _, m := range got[1:] {
		if sel[m.Path] != m.Version {
			return nil, fmt.Errorf("internal error: the new %s selects %s, not %s", gocppmod.FileName, m, mvs.Module{Path: m.Path, Version: sel[m.Path]})
		}
	}

	// What changed.
	for p, nv := range sel {
		if oldSel[p] != nv {
			res.Changes = append(res.Changes, Change{Path: p, Old: oldSel[p], New: nv})
		}
	}
	for p, ov := range oldSel {
		if _, ok := sel[p]; !ok {
			res.Changes = append(res.Changes, Change{Path: p, Old: ov})
		}
	}
	sort.Slice(res.Changes, func(i, j int) bool { return res.Changes[i].Path < res.Changes[j].Path })

	// Download what changed, and look at what the user named.
	for _, c := range res.Changes {
		if c.New == "" {
			continue
		}
		if err := download(ctx, f, dir, newFile, pending, c.Path, c.New, res); err != nil {
			return nil, err
		}
	}
	for _, r := range rs {
		if r.version == mvs.None || r.version == "" || file.FindReplace(r.path, r.version) != nil && file.FindReplace(r.path, r.version).Local() {
			continue
		}
		why, bad, err := f.RetractionOf(ctx, dir, r.path, r.version, pending)
		if err != nil {
			continue // a warning, not a reason to fail
		}
		if bad {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s@%s is retracted: %s", r.path, r.version, why))
		}
	}
	for _, r := range rs {
		if rep := newFile.FindReplace(r.path, sel[r.path]); rep != nil && r.version != mvs.None {
			res.Notes = append(res.Notes, fmt.Sprintf("%s is replaced by %s in %s, so the download is not used until the replace line is removed", r.path, replaceTarget(rep), gocppmod.FileName))
		}
	}

	if crlf {
		// A file with Windows line ends keeps them.
		newData = bytes.ReplaceAll(newData, []byte("\n"), []byte("\r\n"))
	}
	res.ModChanged = !bytes.Equal(data, newData)
	res.SumChanged = len(pending.Lines()) > 0
	if o.DryRun {
		return res, nil
	}

	// Write: gocpp.sum first (a gocpp.sum with lines that nothing uses is
	// harmless; a gocpp.mod without the lines of its modules is not), then
	// gocpp.mod, each by one rename.
	beforeWrite()
	changed, err := f.SavePending(dir, pending)
	if err != nil {
		return nil, err
	}
	res.SumChanged = changed
	if res.ModChanged {
		if err := writeBytes(modPath, newData); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// beforeWrite is called before the files are written; tests make the write fail.
var beforeWrite = func() {}

func replaceTarget(r *gocppmod.Replace) string {
	if r.Local() {
		return r.NewPath
	}
	return r.NewPath + "@" + r.NewVersion
}

// latestClosure returns the seed modules at their latest versions and, again at
// their latest, the modules that those versions require.
func latestClosure(reqs *modgraph.Reqs, seeds []mvs.Module, skip func(path string) bool) ([]mvs.Module, error) {
	done := map[string]bool{}
	var out []mvs.Module
	queue := slices.Clone(seeds)
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		if done[m.Path] || skip(m.Path) {
			continue
		}
		done[m.Path] = true
		u, err := reqs.Upgrade(m)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
		rs, err := reqs.Required(u)
		if err != nil {
			return nil, err
		}
		queue = append(queue, rs...)
	}
	return out, nil
}

// baseReqs is a Reqs in which the main module requires list.
type baseReqs struct {
	mvs.Reqs
	main mvs.Module
	list []mvs.Module
}

func (b *baseReqs) Required(m mvs.Module) ([]mvs.Module, error) {
	if m == b.main {
		return b.list, nil
	}
	return b.Reqs.Required(m)
}

// checkArg checks the module path of an argument.
func checkArg(file *gocppmod.File, main mvs.Module, r *request) error {
	path := r.path
	switch {
	case path == "":
		return &ArgError{r.arg, "want module[@version]"}
	case strings.Contains(path, "..."):
		return &ArgError{r.arg, "patterns are not supported: name a module (gocpp get works on modules)"}
	case path == main.Path:
		return &ArgError{r.arg, "can't get the main module " + path}
	}
	if msg := gocppmod.CheckPath(path); msg != "" {
		return &ArgError{r.arg, fmt.Sprintf("invalid module path %q: %s", path, msg)}
	}
	if rep := file.FindReplace(path, ""); rep != nil && rep.Local() {
		return nil
	}
	if first, _, _ := strings.Cut(path, "/"); !strings.Contains(first, ".") {
		return &ArgError{r.arg, fmt.Sprintf("malformed module path %q: missing dot in first path element (only a module with a host name can be downloaded; for a local module add a replace line)", path)}
	}
	return nil
}

// resolve turns the query of a request into a version.
func resolve(ctx context.Context, f *modfetch.Fetcher, dir string, file *gocppmod.File, reqs *modgraph.Reqs, pending *modfetch.PendingSums, r *request, cur string, res *Result) error {
	q := r.query
	if q == "" {
		q = "latest"
		if cur != "" {
			q = "upgrade"
		}
	}
	if q == "none" {
		r.version = mvs.None
		return nil
	}
	isRev := q == "latest" || q == "upgrade" || q == "patch"
	if rep := file.FindReplace(r.path, ""); rep != nil && rep.Local() {
		// A local folder has no versions: the version is only a name.
		switch {
		case gocppmod.ValidVersion(q):
			if reqs.Excluded(r.path, gocppmod.CanonicalVersion(q)) {
				return fmt.Errorf("%s@%s: excluded by an exclude line in %s", r.path, q, gocppmod.FileName)
			}
			r.version = gocppmod.CanonicalVersion(q)
			return nil
		case isRev && cur != "":
			r.version = cur
			return nil
		}
		return fmt.Errorf("%s: replaced by the local folder %s, which has no versions: name one (%s@v0.0.0)", r.path, rep.NewPath, r.path)
	}
	qr, err := f.Query(ctx, dir, r.path, q, modfetch.QueryOptions{
		Current: cur,
		Allowed: func(v string) bool { return !reqs.Excluded(r.path, v) },
		Pending: pending,
	})
	if err != nil {
		return err
	}
	r.version = qr.Version
	if qr.Module != nil {
		if err := f.NoteModule(dir, qr.Module, pending); err != nil {
			return err
		}
	}
	return nil
}

// download makes sure that the module is in the module cache and its hashes
// are pending for gocpp.sum.
func download(ctx context.Context, f *modfetch.Fetcher, dir string, file *gocppmod.File, pending *modfetch.PendingSums, path, version string, res *Result) error {
	p, v := path, version
	if rep := file.FindReplace(path, version); rep != nil {
		if rep.Local() {
			return nil
		}
		p, v = rep.NewPath, rep.NewVersion
	}
	m, err := f.Fetch(ctx, dir, p, v, modfetch.FetchOptions{Record: true, Pending: pending})
	if err != nil {
		return err
	}
	if m.Downloaded {
		res.Downloaded = append(res.Downloaded, p+"@"+v)
	}
	return nil
}

// edit changes the syntax tree of the file so that it requires the build list:
// a line for every module of it (the modules in named without "// indirect"),
// and no line for a module that left it.
func edit(file *gocppmod.File, old []mvs.Module, sel, named map[string]string) *gocppmod.File {
	have := map[string]*gocppmod.Require{}
	for _, r := range file.Require {
		have[r.Path] = r
	}
	for _, m := range old[1:] {
		if _, ok := sel[m.Path]; !ok {
			modfetch.DropRequire(file, m.Path)
		}
	}
	for p, v := range named {
		if v == mvs.None {
			modfetch.DropRequire(file, p)
		}
	}
	paths := make([]string, 0, len(sel))
	for p := range sel {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		v := sel[p]
		r := have[p]
		_, isNamed := named[p]
		switch {
		case r != nil && isNamed:
			modfetch.SetRequireMode(file, p, v, modfetch.Direct)
		case r != nil && r.Version != v:
			modfetch.SetRequireMode(file, p, v, modfetch.KeepIndirect)
		case r == nil && isNamed:
			modfetch.SetRequireMode(file, p, v, modfetch.Direct)
		case r == nil:
			modfetch.SetRequireMode(file, p, v, modfetch.Indirect)
		}
	}
	return file
}

func writeBytes(name string, data []byte) error { return modfetch.WriteFileAtomic(name, data) }
