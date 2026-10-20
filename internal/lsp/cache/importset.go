package cache

import (
	"bytes"
	"fmt"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
)

// An importSet is the imports of one package decoded from the export data of
// their summaries: the packages (all into one map, so that a package that
// two of them mention is one *types.Package), the fake files that hold the
// positions of their objects, and the package of each import in the order of
// the check's deps.
//
// Reading the export data takes about a seventh of the time of a check
// and a fifth of what it allocates, and an edit in a function body leaves the
// imports of the package as they were, so the next check of the package
// uses the same decoded packages. They are only read once decoded
// (go/types reads imported packages from several checks at the same
// time; so does gopls); a change to the summary of any import is a new
// summary (a new pointer), so the old set is not used for it.
type importSet struct {
	deps     []*summary // the summaries decoded, in the order of the check's deps
	imported map[string]*types.Package
	files    []*token.File    // the fake files of the positions, in the order of their bases
	pkgs     []*types.Package // by dep: nil for a dep without export data
	cost     int64            // the estimated memory of the decoded packages
}

// decodeImports decodes the export data of the deps (nil entries are skipped)
// into fset, which must be empty, or takes the decoded packages of the
// last check of the package id when its deps are the same summaries; fset
// then gets the files of that decode. The set is kept for the next check.
func (p *Pipeline) decodeImports(id pkgID, fset *token.FileSet, deps []*summary) (*importSet, error) {
	if set := p.importSets.get(id, deps); set != nil {
		fset.AddExistingFiles(set.files...)
		return set, nil
	}
	set := &importSet{deps: deps, imported: make(map[string]*types.Package), pkgs: make([]*types.Package, len(deps))}
	for i, s := range deps {
		if s == nil || s.export == nil {
			continue
		}
		pkg, err := gcexportdata.Read(bytes.NewReader(s.export), fset, set.imported, s.importPath)
		if err != nil {
			return nil, fmt.Errorf("cache: the export data of %s: %v", s.importPath, err)
		}
		set.pkgs[i] = pkg
		set.cost += exportFactor * int64(len(s.export))
	}
	fset.Iterate(func(f *token.File) bool {
		set.files = append(set.files, f)
		return true
	})
	p.importSets.put(id, set)
	return set, nil
}

// maxImportSets is how many packages keep their last imports, and
// maxImportSetBytes the estimated memory they may take together: the packages
// of the open files are the ones that are edited, and their sets are the
// ones used again.
const (
	maxImportSets     = 16
	maxImportSetBytes = 48 << 20
)

// importSets keeps the last importSet of up to maxImportSets packages, the
// least recently used going first.
type importSets struct {
	mu    sync.Mutex
	sets  map[pkgID]*importSet
	order []pkgID // oldest first
	bytes int64
}

func (c *importSets) get(id pkgID, deps []*summary) *importSet {
	c.mu.Lock()
	defer c.mu.Unlock()
	set := c.sets[id]
	if set == nil || len(set.deps) != len(deps) {
		return nil
	}
	for i, s := range deps {
		if set.deps[i] != s {
			return nil
		}
	}
	return set
}

func (c *importSets) put(id pkgID, set *importSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sets == nil {
		c.sets = make(map[pkgID]*importSet)
	}
	if old, ok := c.sets[id]; ok {
		c.bytes -= old.cost
		for i, o := range c.order {
			if o == id {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	}
	c.sets[id] = set
	c.bytes += set.cost
	c.order = append(c.order, id)
	for len(c.order) > 1 && (len(c.order) > maxImportSets || c.bytes > maxImportSetBytes) {
		c.bytes -= c.sets[c.order[0]].cost
		delete(c.sets, c.order[0])
		c.order = c.order[1:]
	}
}
