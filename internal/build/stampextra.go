package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// AddOutputs records the files that a library mode wrote besides the stamp's main output (the header of
// a c-archive, the tree of a cpp-lib), so that a build finds them again, and builds when one is missing
// or was changed. It returns why the build can't be stamped, or "". Call it after NewStamp.
func (s *Stamp) AddOutputs(paths []string) string {
	if why := s.settleListings(paths); why != "" {
		return why
	}
	now := time.Now()
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() {
			return "an output file can't be looked at: " + p
		}
		rec := recOf(fi, now)
		if rec.Racy {
			data, err := os.ReadFile(p)
			if err != nil {
				return "an output file can't be read: " + p
			}
			rec.Hash = hashHex(data)
		}
		s.Extra = append(s.Extra, OutputRec{Path: p, Stat: rec})
	}
	return ""
}

// settleListings makes the listings of the folders that the loader read agree with the files that the
// build wrote into them: a header next to the sources (libx.h) or a folder (the tree of a cpp-lib) is
// new after the first build, and is not a change of the package. A listing is accepted, with the
// files as they are now, when it is the recorded one once the outputs are left out; anything else was
// changed during the build.
func (s *Stamp) settleListings(outs []string) string {
	for i := range s.Trace {
		op := &s.Trace[i]
		if op.Kind != "d" || !op.Exists {
			continue
		}
		skip := map[string]bool{}
		for _, o := range outs {
			if rel, err := filepath.Rel(op.Path, o); err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !hasDotDot(rel) {
				skip[firstElem(rel)] = true
			}
		}
		if len(skip) == 0 {
			continue
		}
		entries, err := os.ReadDir(op.Path)
		if err != nil {
			return op.Path + " can't be listed: " + err.Error()
		}
		var rest []fs.DirEntry
		for _, e := range entries {
			if !skip[e.Name()] {
				rest = append(rest, e)
			}
		}
		if listingHash(rest) != op.Hash && listingHash(entries) != op.Hash {
			return "the files of " + op.Path + " changed during the build"
		}
		op.Hash = listingHash(entries)
		if fi, err := os.Stat(op.Path); err == nil {
			op.Stat = recOf(fi, time.Now())
		}
	}
	return ""
}

func hasDotDot(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}

func firstElem(rel string) string {
	for i := 0; i < len(rel); i++ {
		if rel[i] == filepath.Separator {
			return rel[:i]
		}
	}
	return rel
}
