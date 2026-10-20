package cache

import "sync"

// keptVersions is how many checks of one package the check cache keeps. An
// edited package makes a new check at every keystroke (a key has the hash of its
// files), and the cache, which goes by the cost of what it holds, kept the old
// ones until it was full: a project of 90,000 lines that is edited has 250
// checks in memory for its 100 packages, and a resident set of 430 MiB. The last
// ones stay (an undo, or the text of an edit and its undo, which alternate, hit
// them); an older one is dropped from the cache, and checked again if it is
// asked for. (A snapshot that holds a package keeps its check anyway.)
const keptVersions = 3

// A versionKeeper remembers the keys of the latest checks of each package.
type versionKeeper struct {
	mu   sync.Mutex
	keys map[pkgID][]Hash // oldest first
}

// note records that the check with the key key exists for the package id,
// and drops the checks older than the last keptVersions from the cache of p.
func (v *versionKeeper) note(p *Pipeline, id pkgID, key Hash) {
	v.mu.Lock()
	if v.keys == nil {
		v.keys = make(map[pkgID][]Hash)
	}
	list := v.keys[id]
	for i, k := range list {
		if k == key {
			// Seen already: it is the latest now.
			list = append(list[:i], list[i+1:]...)
			break
		}
	}
	list = append(list, key)
	var drop []Hash
	if len(list) > keptVersions {
		drop = append(drop, list[:len(list)-keptVersions]...)
		list = append([]Hash(nil), list[len(list)-keptVersions:]...)
	}
	v.keys[id] = list
	v.mu.Unlock()
	for _, k := range drop {
		p.checks.ForgetIf(k, func(*Package, error) bool { return true })
	}
}
