// Package modgraph is the requirement graph of a gocpp module: the require
// lines of its gocpp.mod and of the module files (gocpp.mod or go.mod) of the
// modules that it requires, with the main module's replace and exclude lines
// applied, as the go command reads them (docs/spec/modules.md "Versions").
//
// It is the mvs.Reqs of internal/mvs on top of the sources of
// internal/modfetch: a module cache, a module proxy or a git host, a local
// folder of a replace line.
//
// Semantics that follow Go (and are tested against the real go command):
//
//   - the main module is selected whatever other modules require of it;
//   - a replace line (for one version or all) changes where a module's
//     requirements and files come from, not its version in the build list;
//   - an exclude line of the main module makes a requirement on that version
//     count as nothing (Go 1.16 and later);
//   - the retract lines of dependencies play no part in the selection of the
//     build list (they matter to "gocpp get" and "gocpp mod graph" warnings),
//     but Previous (the versions a downgrade may fall back to) skips the
//     retracted ones, as Go's CheckAllowed does;
//   - a module that has no module file (a C library) requires nothing.
package modgraph

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/mvs"
)

// A Source says what the requirements of a module version are and which
// versions a module has. modfetch.Fetcher is one (FetcherSource); tests have
// their own.
type Source interface {
	// ModFile returns the module file of the version, or one with Name ""
	// for a module that has none.
	ModFile(ctx context.Context, path, version string) (*modfetch.ModFileInfo, error)

	// Versions returns the versions of the module in increasing order.
	Versions(ctx context.Context, path string) ([]string, error)
}

// A RetractionSource is a Source that also knows the retract lines of the
// highest version of a module. Reqs.Previous skips the retracted versions
// when the Source is one, as Go's CheckAllowed does.
type RetractionSource interface {
	Retractions(ctx context.Context, path string) ([]*gocppmod.Retract, error)
}

// A Config describes the main module and where the others come from.
type Config struct {
	// MainDir is the folder of the main module (gocpp.mod is in it). Relative
	// replace folders are relative to it.
	MainDir string

	// File is the main module's gocpp.mod. It may be nil (a module with no
	// requirements); then MainPath names the module.
	File *gocppmod.File

	// MainPath overrides the path in File's module line (tests).
	MainPath string

	Source Source

	// Latest, if set, is what Upgrade (gocpp get -u) asks for the latest
	// version of a module at or above the current one ("upgrade" or "patch"
	// of modfetch.Query).
	Latest func(ctx context.Context, path, current string) (string, error)
}

// Main returns the main module: its path, no version.
func (c *Config) Main() mvs.Module {
	if c.MainPath != "" {
		return mvs.Module{Path: c.MainPath}
	}
	if c.File != nil && c.File.Module != nil {
		return mvs.Module{Path: c.File.Module.Path}
	}
	return mvs.Module{Path: "main"}
}

// Reqs is the requirement graph; it implements mvs.Reqs. It is safe for
// concurrent use, and reads each module file once.
type Reqs struct {
	cfg *Config
	ctx context.Context

	excluded map[mvs.Module]bool

	mu     sync.Mutex
	loaded map[mvs.Module]*loadedMod
	vers   map[string]*loadedVers
	rets   map[string]*loadedRets
}

type loadedRets struct {
	once sync.Once
	rets []*gocppmod.Retract
	err  error
}

type loadedMod struct {
	once sync.Once
	file *gocppmod.File // nil for a module without a module file
	err  error
	// what is the place the file came from, for messages.
	what string
}

type loadedVers struct {
	once sync.Once
	vs   []string
	err  error
}

// New returns the requirement graph of the configuration.
func New(ctx context.Context, cfg *Config) *Reqs {
	r := &Reqs{
		cfg:      cfg,
		ctx:      ctx,
		excluded: map[mvs.Module]bool{},
		loaded:   map[mvs.Module]*loadedMod{},
		vers:     map[string]*loadedVers{},
		rets:     map[string]*loadedRets{},
	}
	if cfg.File != nil {
		for _, e := range cfg.File.Exclude {
			r.excluded[excludeKey(e.Path, e.Version)] = true
		}
	}
	return r
}

// Excluded reports whether the main module excludes the version.
func (r *Reqs) Excluded(path, version string) bool {
	return r.excluded[excludeKey(path, version)]
}

// excludeKey is the key of an exclude line, and of a version that is looked up in them: the
// version without +incompatible. v2.0.0 and v2.0.0+incompatible of a module with no /v2 in its path
// are one tree (the list of versions writes the second, "get" turns an explicit v2.0.0 into it),
// and Go insists on the second spelling in an exclude line, but gocpp.mod is read leniently, so
// an exclude line written the first way must still match.
func excludeKey(path, version string) mvs.Module {
	return mvs.Module{Path: path, Version: strings.TrimSuffix(version, "+incompatible")}
}

// Main returns the main module.
func (r *Reqs) Main() mvs.Module { return r.cfg.Main() }

// File returns the main module's file (nil if there is none).
func (r *Reqs) File() *gocppmod.File { return r.cfg.File }

// A MissingModError is a module whose module file can't be read.
type MissingModError struct {
	Module mvs.Module
	What   string // where the file is: "replaced by ../x", or ""
	Err    error
}

func (e *MissingModError) Error() string {
	// The error of a download names the module too: say it once.
	msg := strings.TrimPrefix(e.Err.Error(), e.Module.String()+": ")
	if e.What != "" {
		return fmt.Sprintf("%s (%s): %s", e.Module, e.What, msg)
	}
	return fmt.Sprintf("%s: %s", e.Module, msg)
}

func (e *MissingModError) Unwrap() error { return e.Err }

// Required implements mvs.Reqs.
func (r *Reqs) Required(m mvs.Module) ([]mvs.Module, error) {
	if m.Version == "" && m.Path == r.Main().Path {
		return r.mainRequired(), nil
	}
	lm := r.load(m)
	if lm.err != nil {
		return nil, &MissingModError{Module: m, What: lm.what, Err: lm.err}
	}
	if lm.file == nil {
		return nil, nil
	}
	var out []mvs.Module
	for _, q := range lm.file.Require {
		if r.Excluded(q.Path, q.Version) {
			continue
		}
		out = append(out, mvs.Module{Path: q.Path, Version: q.Version})
	}
	return out, nil
}

// sortReqs orders requirements by path and version. The go command does that
// for the main module's (it rewrites go.mod, which sorts them), and shows the
// others in the order of their files, in the order of "go mod graph".
func sortReqs(l []mvs.Module) {
	slices.SortStableFunc(l, func(a, b mvs.Module) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return gocppmod.CompareVersions(a.Version, b.Version)
	})
}

func (r *Reqs) mainRequired() []mvs.Module {
	if r.cfg.File == nil {
		return nil
	}
	var out []mvs.Module
	for _, q := range r.cfg.File.Require {
		if r.Excluded(q.Path, q.Version) {
			continue
		}
		out = append(out, mvs.Module{Path: q.Path, Version: q.Version})
	}
	sortReqs(out)
	return out
}

// Max implements mvs.Reqs. The empty version, the main module's, is higher
// than every other; of two equal versions (v2.0.0 and v2.0.0+incompatible) the
// first one stays, as in Go.
func (r *Reqs) Max(_, v1, v2 string) string { return maxVersion(v1, v2) }

func maxVersion(v1, v2 string) string {
	switch {
	case v1 == v2:
		return v1
	case v2 == "":
		return v2
	case v1 == "":
		return v1
	case gocppmod.CompareVersions(v1, v2) < 0:
		return v2
	}
	return v1
}

// Previous implements mvs.Reqs: the version of the module before m.Version in
// its list of versions (not counting the excluded and the retracted ones), or
// "none".
func (r *Reqs) Previous(m mvs.Module) (mvs.Module, error) {
	vs, err := r.versions(m.Path)
	if err != nil {
		return mvs.Module{}, err
	}
	rets, err := r.retractions(m.Path)
	if err != nil {
		return mvs.Module{}, err
	}
	prev := mvs.Module{Path: m.Path, Version: mvs.None}
	for _, v := range vs {
		if r.Excluded(m.Path, v) {
			continue
		}
		if _, bad := modfetch.Retraction(rets, v); bad {
			continue
		}
		if gocppmod.CompareVersions(v, m.Version) >= 0 {
			break
		}
		prev.Version = v
	}
	return prev, nil
}

// Upgrade returns m at the latest version that
// Config.Latest says. A module that is replaced by a local folder has no other
// version.
func (r *Reqs) Upgrade(m mvs.Module) (mvs.Module, error) {
	if r.cfg.Latest == nil {
		return m, nil
	}
	if f := r.cfg.File; f != nil {
		if rep := f.FindReplace(m.Path, ""); rep != nil && rep.Local() {
			return m, nil
		}
	}
	v, err := r.cfg.Latest(r.ctx, m.Path, m.Version)
	if err != nil {
		return mvs.Module{}, err
	}
	if v == "" || gocppmod.CompareVersions(v, m.Version) < 0 {
		return m, nil
	}
	return mvs.Module{Path: m.Path, Version: v}, nil
}

func (r *Reqs) versions(path string) ([]string, error) {
	r.mu.Lock()
	lv := r.vers[path]
	if lv == nil {
		lv = &loadedVers{}
		r.vers[path] = lv
	}
	r.mu.Unlock()
	lv.once.Do(func() {
		if f := r.cfg.File; f != nil {
			if rep := f.FindReplace(path, ""); rep != nil && rep.Local() {
				return // a local folder has no versions
			}
		}
		lv.vs, lv.err = r.cfg.Source.Versions(r.ctx, path)
	})
	return lv.vs, lv.err
}

// retractions returns the retract lines that decide which versions of the module
// are retracted: none if the Source does not know them, or if a replace line
// covers all versions of the module (Go does not load them then).
func (r *Reqs) retractions(path string) ([]*gocppmod.Retract, error) {
	rs, ok := r.cfg.Source.(RetractionSource)
	if !ok {
		return nil, nil
	}
	if f := r.cfg.File; f != nil && f.FindReplace(path, "") != nil {
		return nil, nil
	}
	r.mu.Lock()
	lr := r.rets[path]
	if lr == nil {
		lr = &loadedRets{}
		r.rets[path] = lr
	}
	r.mu.Unlock()
	lr.once.Do(func() { lr.rets, lr.err = rs.Retractions(r.ctx, path) })
	return lr.rets, lr.err
}

func (r *Reqs) load(m mvs.Module) *loadedMod {
	r.mu.Lock()
	lm := r.loaded[m]
	if lm == nil {
		lm = &loadedMod{}
		r.loaded[m] = lm
	}
	r.mu.Unlock()
	lm.once.Do(func() { lm.file, lm.what, lm.err = r.read(m) })
	return lm
}

// ErrDotless is the problem of a module path with no dot in its first element
// that has to be downloaded.
var ErrDotless = errors.New("missing dot in first path element")

// read reads the module file of m: from the replacement, or from the source.
func (r *Reqs) read(m mvs.Module) (*gocppmod.File, string, error) {
	path, version := m.Path, m.Version
	var rep *gocppmod.Replace
	if r.cfg.File != nil {
		rep = r.cfg.File.FindReplace(m.Path, m.Version)
	}
	what := ""
	var mi *modfetch.ModFileInfo
	var err error
	declared := m.Path
	switch {
	case rep != nil && rep.Local():
		dir := filepath.FromSlash(rep.NewPath)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(r.cfg.MainDir, dir)
		}
		what = "replaced by " + rep.NewPath
		if st, serr := statDir(dir); serr != nil {
			return nil, what, fmt.Errorf("replacement directory %s: %v", rep.NewPath, serr)
		} else if !st {
			return nil, what, fmt.Errorf("replacement directory %s is not a directory", rep.NewPath)
		}
		mi, err = modfetch.ReadModFileDir(dir)
	case rep != nil:
		what = "replaced by " + rep.NewPath + "@" + rep.NewVersion
		path, version, declared = rep.NewPath, rep.NewVersion, rep.NewPath
		mi, err = r.fetch(path, version)
	default:
		mi, err = r.fetch(path, version)
	}
	if err != nil {
		return nil, what, err
	}
	if mi.Name == "" {
		return nil, what, nil
	}
	file, err := modfetch.ParseModFile(mi.Name, mi.Data)
	if err != nil {
		return nil, what, fmt.Errorf("parsing %s: %v", mi.Name, err)
	}
	if file.Module != nil && file.Module.Path != m.Path && file.Module.Path != declared {
		return nil, what, fmt.Errorf("parsing %s: module declares its path as: %s\n\t\tbut was required as: %s", mi.Name, file.Module.Path, m.Path)
	}
	return file, what, nil
}

func (r *Reqs) fetch(path, version string) (*modfetch.ModFileInfo, error) {
	if first, _, _ := strings.Cut(path, "/"); !strings.Contains(first, ".") {
		return nil, fmt.Errorf("malformed module path %q: %w (only a module with a dot in its first path element can be downloaded: replace it with a local folder, or give it a host name)", path, ErrDotless)
	}
	return r.cfg.Source.ModFile(r.ctx, path, version)
}

// BuildList returns the build list of the main module (minimal version
// selection) and the graph it came from.
func (r *Reqs) BuildList() ([]mvs.Module, *mvs.Graph, error) {
	g, err := mvs.NewGraph(r, []mvs.Module{r.Main()})
	if err != nil {
		return nil, nil, err
	}
	return g.BuildList(), g, nil
}

// An Edge is a line of "gocpp mod graph": From requires To.
type Edge struct{ From, To mvs.Module }

// Edges returns the edges of the graph in the order of "go mod graph": the
// modules breadth first from the main module, and for each the modules it
// requires in the order of its file.
func Edges(g *mvs.Graph) []Edge {
	var out []Edge
	for _, m := range g.Modules() {
		for _, q := range g.Required(m) {
			out = append(out, Edge{m, q})
		}
	}
	return out
}

// Why returns the shortest chain of requirements from the main module to the
// module path in the graph, looking only at the selected versions (as "go mod
// why -m" would say how a module gets in); nil if the module is not in the
// build list.
func Why(g *mvs.Graph, main mvs.Module, path string) []mvs.Module {
	if path == main.Path {
		return []mvs.Module{main}
	}
	sel := g.Selected(path)
	if sel == "" || sel == mvs.None {
		return nil
	}
	prev := map[mvs.Module]mvs.Module{}
	seen := map[mvs.Module]bool{main: true}
	queue := []mvs.Module{main}
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		for _, q := range g.Required(m) {
			// Walk the versions that were selected, not the lower ones that were
			// only read.
			if q.Path == main.Path || q.Version != g.Selected(q.Path) {
				continue
			}
			if seen[q] {
				continue
			}
			seen[q] = true
			prev[q] = m
			if q.Path == path {
				chain := []mvs.Module{q}
				for c := q; c != main; {
					c = prev[c]
					chain = append(chain, c)
				}
				slices.Reverse(chain)
				return chain
			}
			queue = append(queue, q)
		}
	}
	return nil
}
