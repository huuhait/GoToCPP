package mvs

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// A Conflict says why a requested version can't be selected: Path is a chain of
// requirements from a root to a module version that is higher than the
// Constraint allows.
type Conflict struct {
	Path       []Module
	Constraint Module
}

func (c Conflict) String() string {
	if len(c.Path) == 0 {
		return "(internal error: no path)"
	}
	last := c.Path[len(c.Path)-1]
	var b strings.Builder
	b.WriteString(c.Path[0].String())
	for _, m := range c.Path[1:] {
		b.WriteString(" requires " + m.String())
	}
	if len(c.Path) == 1 {
		return fmt.Sprintf("%s is requested, but %s@%s is also requested", b.String(), last.Path, c.Constraint.Version)
	}
	return fmt.Sprintf("%s, but %s is requested", b.String(), c.Constraint)
}

// A ConstraintError is the result of Edit when the requested versions can't
// all be selected.
type ConstraintError struct{ Conflicts []Conflict }

func (e *ConstraintError) Error() string {
	var b strings.Builder
	b.WriteString("version constraints conflict:")
	for _, c := range e.Conflicts {
		b.WriteString("\n\t" + c.String())
	}
	return b.String()
}

// Edit changes the requirements of the main module as "go get" does since Go
// 1.17 (cmd/go/internal/modload.editRequirements, for a module graph that is not
// pruned): initial is the build list now (without the main module, each module
// being a root), and the result is the list of roots after the edit, so that
//
//  1. every module in mustSelect has that version ("none" removes it);
//  2. every module in tryUpgrade is upgraded toward its version as far as 1
//     allows (other upgrades follow from what the modules require);
//  3. every other module of initial is downgraded or upgraded from its version
//     only as far as 1 and 2 need.
//
// A requirement on a module version higher than the one in mustSelect makes the
// module that has it unusable; if that is a root it is replaced by its previous
// version, again and again (or by "none", which removes it); a root whose version
// is not the selected one is upgraded to the selected one if that is possible,
// after the downgrades are done.
func Edit(reqs Reqs, main Module, initial, tryUpgrade, mustSelect []Module) ([]Module, error) {
	gt := func(p, a, b string) bool { return a != b && reqs.Max(p, a, b) == a && reqs.Max(p, b, a) == a }

	origSel := map[string]string{}
	selectedRoot := map[string]string{}
	for _, m := range initial {
		origSel[m.Path] = m.Version
		selectedRoot[m.Path] = m.Version
	}
	for _, r := range tryUpgrade {
		if v, ok := selectedRoot[r.Path]; ok && !gt(r.Path, r.Version, v) {
			continue
		}
		selectedRoot[r.Path] = r.Version
	}
	var conflicts []Conflict
	must := map[string]string{}
	for _, r := range mustSelect {
		if v, ok := must[r.Path]; ok && v != r.Version {
			lo, hi := r, Module{Path: r.Path, Version: v}
			if gt(r.Path, v, r.Version) {
				lo, hi = hi, lo
			}
			conflicts = append(conflicts, Conflict{Path: []Module{lo}, Constraint: hi})
			continue
		}
		must[r.Path] = r.Version
		selectedRoot[r.Path] = r.Version
	}
	violates := func(m Module) bool {
		max, ok := must[m.Path]
		return ok && gt(m.Path, m.Version, max)
	}

	rejected := map[Module]bool{}
	// Every round rejects a version or accepts an upgrade, so there are few; the
	// bound is a guard against a Reqs that does not behave.
	for round, dirty := 0, true; dirty && len(conflicts) == 0; round++ {
		if round > 10000 {
			return nil, fmt.Errorf("version selection does not settle (internal error)")
		}
		var roots []Module
		for p, v := range selectedRoot {
			if v != None {
				roots = append(roots, Module{p, v})
			}
		}
		sort.Slice(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
		g, err := NewGraph(&override{target: main, list: roots, Reqs: reqs}, []Module{main})
		if err != nil {
			return nil, err
		}

		// The modules that can't be in the graph: the ones that are too high, and
		// the ones that require one (a requirement edge from m to r with r
		// disqualified disqualifies m).
		reason := map[Module]Module{} // m -> the module that is the reason
		requiring := map[Module][]Module{}
		var queue []Module
		disq := func(m, why Module) {
			if m == main {
				return
			}
			if _, ok := reason[m]; ok {
				return
			}
			reason[m] = why
			queue = append(queue, m)
		}
		for _, m := range g.Modules() {
			if m == main {
				continue
			}
			if violates(m) {
				disq(m, m)
				continue
			}
			for _, r := range g.Required(m) {
				if violates(r) {
					disq(m, r)
					break
				}
				requiring[r] = append(requiring[r], m)
			}
		}
		for len(queue) > 0 {
			m := queue[0]
			queue = queue[1:]
			for _, p := range requiring[m] {
				disq(p, m)
			}
		}
		path := func(m Module) []Module {
			var p []Module
			for {
				why, ok := reason[m]
				if !ok {
					return nil
				}
				p = append(p, m)
				if why == m || len(p) > len(reason)+1 {
					return p
				}
				m = why
			}
		}

		dirty = false
		for _, m := range roots {
			p := path(m)
			if len(p) == 0 {
				continue
			}
			last := p[len(p)-1]
			if v, ok := must[m.Path]; ok && v == m.Version {
				conflicts = append(conflicts, Conflict{Path: p, Constraint: Module{Path: last.Path, Version: must[last.Path]}})
				continue
			}
			if v := g.Selected(m.Path); v != m.Version {
				u := Module{m.Path, v}
				if _, in := reason[u]; !in && !rejected[u] {
					continue // the upgrade to the selected version resolves it
				}
			}
			origV := origSel[m.Path]
			rejected[m] = true
			prev := m
			for steps := 0; ; steps++ {
				if steps > 100000 {
					return nil, fmt.Errorf("%s: no previous version settles (internal error)", m)
				}
				pv, err := reqs.Previous(prev)
				if err != nil {
					return nil, err
				}
				prev = pv
				if origV != "" && gt(m.Path, m.Version, origV) && !gt(m.Path, prev.Version, origV) && prev.Version != origV {
					// Previous skipped over the version of before the edit: try it.
					prev.Version = origV
				}
				if rejected[prev] {
					continue
				}
				if _, bad := reason[prev]; bad {
					rejected[prev] = true
					continue
				}
				break
			}
			selectedRoot[m.Path] = prev.Version
			dirty = true
		}
		if dirty {
			continue
		}
		// Accept the upgrades that the graph needs.
		for p, v := range selectedRoot {
			if _, ok := must[p]; ok {
				continue
			}
			if actual := g.Selected(p); actual != v && actual != "" {
				selectedRoot[p] = actual
				dirty = true
			}
		}
	}
	if len(conflicts) > 0 {
		return nil, &ConstraintError{Conflicts: conflicts}
	}
	var out []Module
	for p, v := range selectedRoot {
		if v != None {
			out = append(out, Module{p, v})
		}
	}
	slices.SortFunc(out, func(a, b Module) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}
