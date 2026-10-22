package probe

import (
	"os"
	"slices"
)

// Stamps remember the files that a Locate call read, so that a caller can tell, without a compiler
// run and without reading the files again, that its answers are still the answers for them (step
// 12.13e: the definitions that the sources of a package give are kept from one snapshot to the
// next while no file they read has changed).
//
// A file counts as unchanged when its identity, size, modification time and change time are the
// same as when they were captured, and it was captured only if no file was modified shortly before
// the call began or after it (the rule of racyDeps, the same that keeps the runner from caching an
// answer): a file that is older than that cannot have been changed since by a write that left its
// time as it was. A place where a file that the compiler looked for was not found stays empty
// (a header that shadows another one is a change).
type Stamps struct {
	files []indexDependency
	nodes *indexResolution
}

// Deps returns the files that the call read, and the places it looked for them, sorted.
func (l *Locations) Deps() []string {
	if l == nil {
		return nil
	}
	return slices.Clone(l.deps)
}

// CaptureStamps records the state of the files of the call, or returns nil when it cannot be
// trusted: there are none, one has changed since the call began, or one was modified so lately
// that a later write might leave its times alone.
func (l *Locations) CaptureStamps() *Stamps {
	if l == nil || len(l.deps) == 0 || racyDeps(l.deps, l.started) {
		return nil
	}
	var files []indexDependency
	for _, path := range l.deps {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue // a place that is empty: the resolution below sees it
		}
		files = append(files, indexDependency{path: path, info: info})
	}
	nodes, err := captureIndexResolution(append(slices.Clone(l.deps), l.dir))
	if err != nil {
		return nil
	}
	s := &Stamps{files: files, nodes: nodes}
	if racyDeps(l.deps, l.started) || !s.Unchanged() {
		return nil
	}
	return s
}

// Unchanged reports whether every file is as it was, by looking at their status only.
func (s *Stamps) Unchanged() bool {
	if s == nil {
		return false
	}
	for _, f := range s.files {
		info, err := os.Stat(f.path)
		if err != nil || !info.Mode().IsRegular() || !os.SameFile(f.info, info) ||
			f.info.Size() != info.Size() || !f.info.ModTime().Equal(info.ModTime()) {
			return false
		}
		oldCT, oldOK := changeTime(f.info)
		newCT, newOK := changeTime(info)
		if oldOK != newOK || oldOK && oldCT != newCT {
			return false
		}
	}
	return s.nodes.nodesUnchanged()
}

// Bytes is a conservative estimate of the memory of the stamps.
func (s *Stamps) Bytes() int64 {
	if s == nil {
		return 0
	}
	return int64(len(s.files))*160 + int64(len(s.nodes.nodes))*120
}
