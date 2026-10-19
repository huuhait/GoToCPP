// Package mvs is minimal version selection (Russ Cox's algorithm, as the go
// command uses it): given the requirement graph of a module, it picks, for
// every module in the graph, the highest version that anything requires, and
// (Edit) it computes the requirements that "go get" writes when versions are
// changed.
//
// The package knows nothing about files, networks or the syntax of gocpp.mod:
// a Reqs says what a module version requires, and what "higher" and
// "previous" mean. internal/modgraph is the Reqs that reads gocpp.mod and
// go.mod files, replace and exclude lines and the module cache.
//
// The semantics follow cmd/go/internal/mvs. They are checked against the
// real go command on random graphs (internal/modget, TestDifferential*).
package mvs

import (
	"slices"
	"sort"
	"sync"
)

// A Module is a module path at a version. The version "none" stands for "not
// in the build list"; the version "" is a main module (it has no version).
type Module struct {
	Path, Version string
}

func (m Module) String() string {
	if m.Version == "" {
		return m.Path
	}
	return m.Path + "@" + m.Version
}

// None is the version of a module that is not in the build list.
const None = "none"

// Reqs describes the requirement graph.
type Reqs interface {
	// Required returns the direct requirements of m, in file order. It is
	// called concurrently for different modules.
	Required(m Module) ([]Module, error)

	// Max returns the higher of two versions of the module path. The empty
	// version is lower than every other; "none" is lower than every real
	// version.
	Max(path, v1, v2 string) string

	// Previous returns the version of m.Path that comes before m.Version, or
	// "none" if there is none (Edit uses it to lower a module).
	Previous(m Module) (Module, error)
}

// A Graph is the requirement graph from some root modules, and the version of
// each module that is selected.
type Graph struct {
	reqs     Reqs
	roots    []Module
	selected map[string]string
	required map[Module][]Module
	order    []Module // the modules in the order they were first reached
}

// workers is the number of Required calls that run at the same time.
const workers = 8

// NewGraph reads the whole graph below the roots: every version of every
// module that any requirement names, not only the selected ones (a lower
// version may require something that no higher one does not, and then it still
// counts: this is what makes the selection minimal and stable).
func NewGraph(reqs Reqs, roots []Module) (*Graph, error) {
	return newGraph(reqs, roots)
}

// A RequiredError is an error of Reqs.Required for a module, with the chain of
// requirements that led to it (the first element is a root).
type RequiredError struct {
	Module Module
	Err    error
}

func (e *RequiredError) Error() string { return e.Err.Error() }
func (e *RequiredError) Unwrap() error { return e.Err }

func newGraph(reqs Reqs, roots []Module) (*Graph, error) {
	g := &Graph{
		reqs:     reqs,
		roots:    slices.Clone(roots),
		selected: map[string]string{},
		required: map[Module][]Module{},
	}
	seen := map[Module]bool{}
	var level []Module
	visit := func(m Module) {
		if m.Version == None || seen[m] {
			return
		}
		seen[m] = true
		g.order = append(g.order, m)
		if v, ok := g.selected[m.Path]; !ok {
			g.selected[m.Path] = m.Version
		} else {
			g.selected[m.Path] = reqs.Max(m.Path, v, m.Version)
		}
		level = append(level, m)
	}
	for _, r := range roots {
		visit(r)
	}
	for len(level) > 0 {
		cur := level
		level = nil
		type result struct {
			reqs []Module
			err  error
		}
		res := make([]result, len(cur))
		var wg sync.WaitGroup
		sem := make(chan struct{}, workers)
		for i, m := range cur {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				r, err := reqs.Required(m)
				res[i] = result{r, err}
			}()
		}
		wg.Wait()
		for i, m := range cur {
			if res[i].err != nil {
				return nil, &RequiredError{Module: m, Err: res[i].err}
			}
			rs := res[i].reqs
			g.required[m] = rs
			for _, r := range rs {
				visit(r)
			}
		}
	}
	return g, nil
}

// Selected returns the selected version of the module path, or "" if the
// graph has none.
func (g *Graph) Selected(path string) string { return g.selected[path] }

// Required returns the requirements of m as the graph read them.
func (g *Graph) Required(m Module) []Module { return g.required[m] }

// Modules returns the modules of the graph in the order they were reached,
// breadth first from the roots.
func (g *Graph) Modules() []Module { return slices.Clone(g.order) }

// BuildList returns the selected module versions: the roots first, then the
// others sorted by path.
func (g *Graph) BuildList() []Module {
	isRoot := map[string]bool{}
	out := make([]Module, 0, len(g.selected))
	for _, r := range g.roots {
		isRoot[r.Path] = true
		out = append(out, r)
	}
	var rest []Module
	for p, v := range g.selected {
		if !isRoot[p] && v != None {
			rest = append(rest, Module{p, v})
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Path < rest[j].Path })
	return append(out, rest...)
}

// BuildList is the build list of the graph below the target modules.
func BuildList(targets []Module, reqs Reqs) ([]Module, error) {
	g, err := NewGraph(reqs, targets)
	if err != nil {
		return nil, err
	}
	return g.BuildList(), nil
}

// An override is a Reqs in which the requirements of one module are replaced.
type override struct {
	target Module
	list   []Module
	Reqs
}

func (r *override) Required(m Module) ([]Module, error) {
	if m == r.target {
		return r.list, nil
	}
	return r.Reqs.Required(m)
}
