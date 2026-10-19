package fspath

import (
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Canon returns the one spelling that the server uses for the file named p, an absolute name. On a flavor that
// does not fold case it is p itself. On one that does, it is the clean name with the drive letter in upper
// case, and every part of the name that exists on disk has the case it has there: the editor's name is what the
// user typed when the folder was opened (`c:\users\me\Proj`), the files found by listing a folder have the case
// the file system stores, and the two must be one key. Parts below the first that does not exist stay as they
// are (a file that is not saved yet).
//
// The answers are remembered; Invalidate forgets them (a file or folder was created, deleted or renamed). Work
// is bounded: a folder is listed once at a time however many names ask for it, a folder that lacks a name is
// not listed again for missTTL, and a listing that takes more than listTimeout (a share that does not answer) is
// given up and remembered as empty, so one dead share does not stall every request.
func (f Flavor) Canon(p string) string {
	if !f.FoldCase || p == "" {
		return p
	}
	gen := generation.Load()
	raw := canonKey{f, p}
	if v, ok := canonMemo.Load(raw); ok && v.(canonValue).gen == gen { // the name as it came: no work at all
		return v.(canonValue).out
	}
	p = f.Clean(p)
	if !f.IsAbs(p) {
		return p
	}
	k := canonKey{f, p}
	if v, ok := canonMemo.Load(k); ok && v.(canonValue).gen == gen {
		canonMemo.Store(raw, v)
		return v.(canonValue).out
	}
	out := f.resolve(p)
	if generation.Load() != gen {
		return out // Invalidate came while this was worked out: the answer may be old, so it is not kept
	}
	if canonCount.Add(2) > maxCanonMemo {
		canonMemo.Clear() // a bound on the memory: the names of a long session are all different
		canonCount.Store(2)
	}
	v := canonValue{gen, out}
	canonMemo.Store(k, v)
	canonMemo.Store(raw, v)
	return out
}

// Canon is Current().Canon.
func Canon(p string) string { return Current().Canon(p) }

type canonKey struct {
	f Flavor
	p string
}

type canonValue struct {
	gen uint64
	out string
}

var (
	canonMemo  sync.Map // canonKey -> canonValue
	canonCount atomic.Int64
	generation atomic.Uint64 // Invalidate adds one: what was remembered under an older number is old
)

// maxCanonMemo bounds the memo (a var for the test).
var maxCanonMemo int64 = 200_000

// Generation returns a number that changes whenever Invalidate is called: a caller that remembers an answer of Canon
// keeps the number with it, and forgets the answer when the number is another.
func Generation() uint64 { return generation.Load() }

// resolve gives the parts of the clean absolute name p the case they have on disk.
func (f Flavor) resolve(p string) string {
	if f.Windows && strings.Contains(p, "~") {
		if long, ok := expandShortNames(p); ok {
			p = f.Clean(long) // the long names, then the case of each (once: no name is expanded twice)
		}
	}
	vl := f.volumeLen(p)
	vol, rest := p[:vl], p[vl:]
	if f.Windows && len(vol) == 2 { // C: -> upper case, the drive letter is never case sensitive
		vol = strings.ToUpper(vol)
	}
	sep := string(f.Sep())
	var b strings.Builder
	b.WriteString(vol)
	dir := vol + sep
	parts := strings.Split(strings.TrimPrefix(rest, sep), sep)
	if rest == "" || rest == sep {
		return vol + sep
	}
	for i, part := range parts {
		name, ok := f.lookup(dir, part)
		if !ok {
			// It does not exist (or the folder cannot be listed): the rest stays as given.
			for _, q := range parts[i:] {
				b.WriteString(sep + q)
			}
			return b.String()
		}
		b.WriteString(sep + name)
		dir = dir + name + sep
	}
	return b.String()
}

// lookup finds the entry of the folder dir that is named like part, and returns it as the file system stores it.
func (f Flavor) lookup(dir, part string) (string, bool) {
	l := listing(f, dir, false)
	if n, ok := l.find(part); ok {
		return n, true
	}
	// The listing may be older than the file: list again, but not for every name that is missing.
	if now().Sub(l.at) < max(missTTL, l.ttl) {
		return "", false
	}
	l = listing(f, dir, true)
	return l.find(part)
}

// A folderListing is the entries of a folder by their folded names.
type folderListing struct {
	names map[string]string // folded name -> name; nil when the folder could not be listed
	// ambiguous holds the folded names that two entries have (`Foo` and `foo`: a case-sensitive folder, such as a
	// WSL share seen from Windows): only the exact spelling finds one of them.
	ambiguous map[string][]string
	at        time.Time
	ttl       time.Duration // when it says nothing more than missTTL: how long a listing that failed is believed
	gen       uint64
}

func (l *folderListing) find(part string) (string, bool) {
	if l.names == nil {
		return "", false
	}
	k := fold(part)
	if all, ok := l.ambiguous[k]; ok {
		if slices.Contains(all, part) {
			return part, true
		}
		return "", false
	}
	n, ok := l.names[k]
	return n, ok
}

var (
	listMu    sync.Mutex
	listCache = map[canonKey]*folderListing{}
	inflight  = map[canonKey]*call{}
	lister    = osLister
	now       = time.Now
	// missTTL is how long a listing is believed when it lacks a name; listTimeout how long a listing may take.
	missTTL       = time.Second
	listTimeout   = 5 * time.Second
	deadFolderTTL = 30 * time.Second
)

// A call is a listing in progress, which the others that want it wait for.
type call struct {
	done chan struct{}
	l    *folderListing
}

// osLister lists a folder on the system the process runs on.
func osLister(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, err
}

// SetLister replaces the function that lists a folder (the names of its entries), for tests of the logic with a
// file system that is not the one of the process. It returns the function that puts the old one back.
func SetLister(fn func(dir string) ([]string, error)) (restore func()) {
	listMu.Lock()
	old := lister
	lister = fn
	listMu.Unlock()
	Invalidate()
	return func() {
		listMu.Lock()
		lister = old
		listMu.Unlock()
		Invalidate()
	}
}

// SetTimes sets missTTL and listTimeout, for tests; it returns the function that puts the old ones back.
func SetTimes(miss, timeout time.Duration) (restore func()) {
	listMu.Lock()
	om, ot := missTTL, listTimeout
	missTTL, listTimeout = miss, timeout
	listMu.Unlock()
	return func() {
		listMu.Lock()
		missTTL, listTimeout = om, ot
		listMu.Unlock()
	}
}

// listing returns the listing of the folder dir: the remembered one, or (refresh, or none) a new one. Callers that
// want the same folder at the same time share one call of the lister.
func listing(f Flavor, dir string, refresh bool) *folderListing {
	k := canonKey{f, dir}
	gen := generation.Load()
	listMu.Lock()
	if l, ok := listCache[k]; ok && l.gen == gen && !refresh {
		listMu.Unlock()
		return l
	}
	if c, ok := inflight[k]; ok {
		listMu.Unlock()
		<-c.done
		return c.l
	}
	c := &call{done: make(chan struct{})}
	inflight[k] = c
	fn, timeout := lister, listTimeout
	listMu.Unlock()

	type result struct {
		names []string
		err   error
	}
	ch := make(chan result, 1) // buffered: a listing that comes too late is dropped
	go func() {
		names, err := fn(dir)
		ch <- result{names, err}
	}()
	l := &folderListing{at: now(), gen: gen}
	timer := time.NewTimer(timeout)
	select {
	case r := <-ch:
		timer.Stop()
		if r.err == nil || len(r.names) > 0 {
			l.names = make(map[string]string, len(r.names))
			for _, n := range r.names {
				fk := fold(n)
				if old, dup := l.names[fk]; dup && old != n {
					if l.ambiguous == nil {
						l.ambiguous = map[string][]string{}
					}
					if len(l.ambiguous[fk]) == 0 {
						l.ambiguous[fk] = []string{old}
					}
					l.ambiguous[fk] = append(l.ambiguous[fk], n)
				}
				l.names[fk] = n
			}
		}
	case <-timer.C:
		// A folder that does not answer is an empty one for now (names stay as given), and is not asked again for a while:
		// a dead share must not stall every request by the time it takes to give up.
		l.ttl = deadFolderTTL
	}
	listMu.Lock()
	if generation.Load() == gen {
		listCache[k] = l
	}
	delete(inflight, k)
	c.l = l
	listMu.Unlock()
	close(c.done)
	return l
}

// Invalidate forgets the canonical names and the listings of folders. The server calls it when a file or folder
// was created, deleted or renamed outside the editor (didChangeWatchedFiles). It takes no time: what was remembered
// carries the number of its generation, and the new number makes it old.
func Invalidate() {
	generation.Add(1)
	canonCount.Store(0)
	canonMemo.Clear()
	listMu.Lock()
	clear(listCache)
	listMu.Unlock()
}

// longName turns a name that exists into its long form (the 8.3 short names `PROGRA~1` written out); the system's
// function on Windows (GetLongPathName), none elsewhere. SetLongName replaces it in tests.
var longName = nativeLongName

// SetLongName replaces the function that expands short names, for tests. It returns the function that puts the old one back.
func SetLongName(fn func(path string) (string, bool)) (restore func()) {
	old := longName
	longName = fn
	Invalidate()
	return func() {
		longName = old
		Invalidate()
	}
}

// expandShortNames returns p with its short names written out, and whether that changed anything. The part of p that
// does not exist (a file that is not saved yet) is kept as it is: the longest existing folder is expanded.
func expandShortNames(p string) (string, bool) {
	if longName == nil {
		return p, false
	}
	for prefix := p; ; {
		if long, ok := longName(prefix); ok {
			if long == prefix {
				return p, false
			}
			return long + p[len(prefix):], true
		}
		i := strings.LastIndexByte(prefix, '\\')
		if i <= 2 {
			return p, false
		}
		prefix = prefix[:i]
	}
}
