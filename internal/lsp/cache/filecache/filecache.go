// Package filecache is the language server's cache on disk (Phase 11
// README §2.2 item 4, step 11.3b): a map from a kind and a key (a SHA-256
// hash of what a value was made from) to the bytes of the value, kept
// between runs of the server and shared by the servers that run at the same
// time. The memoized pipeline keeps the summaries of checked packages in it
// (export data and what goes with it), so that a server that starts again
// loads the packages it doesn't show from disk instead of checking them
// from source.
//
// The ideas come from gopls's filecache (see COPIED.md): one file per entry,
// in a folder per version of the program (the format of the values is the
// program's own, so a new gocpp binary never reads the entries of an old
// one), a check of the content on every read, and a trim in the background
// by age and by size, which also takes away the entries of old versions.
//
// The layout under the root folder (Options.Dir):
//
//	<root>/v1-<version>/<kk>/<key>-<kind>   an entry (kk: the first byte of the key, in hex)
//	<root>/trim.txt                          when the last trim ran (Unix seconds)
//
// (version: 16 hex digits; key: 64 hex digits; kind: a-z and 0-9). The
// cache owns only the files and folders of these names, and the temporary
// files of its writes (an entry or trim.txt with ".tmp-" and a suffix): a
// trim never removes anything else, so a root folder that has other files
// (by mistake, GOCPP_LSP_CACHE=~) keeps them.
//
// An entry is written to a temporary file in the same folder, which is then
// renamed over the entry: a reader sees the old file or the new one, never
// half of one. Every entry starts with a header that has the length and the
// SHA-256 hash of the value, so a file that is cut short or damaged is
// noticed on read: Get then removes it and reports a miss. Nothing in the
// cache can make the server fail: every error is a miss.
//
// The trim policy (the disk may be small): an entry that was not used for
// Options.MaxAge (default 5 days) goes, and one of another version (an old
// gocpp binary) after a day; then, while the files take more than
// Options.MaxSize (default 1 GiB), the ones used least recently go. Get marks
// an entry as used by setting its modification time, at most once an hour
// (like the go command's cache, so that a read is rarely a write). A trim
// runs in the background when the cache is opened and after writes, at
// most once per Options.TrimEvery (default 1 hour, for all the servers
// together: trim.txt says when the last one ran), and also when one server
// has written more than half of MaxSize since its last trim.
package filecache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The defaults of Options.
const (
	DefaultMaxSize   = 1 << 30
	DefaultMaxAge    = 5 * 24 * time.Hour
	DefaultTrimEvery = time.Hour
)

const (
	// layout is the version of the layout of the folders and of the
	// header of an entry: it is part of the name of each version folder.
	layout = "v1"
	// touchAfter is the age of the modification time of an entry after
	// which Get sets it again (to mark the entry as used).
	touchAfter = time.Hour
	// tempMaxAge is the age after which a trim removes a temporary file:
	// the writer died before it renamed it.
	tempMaxAge = time.Hour
	// trimFile is the file that says when the last trim ran.
	trimFile = "trim.txt"
	// tempMark is in the name of every temporary file.
	tempMark = ".tmp-"
	// otherMaxAge caps the age of the entries of the other versions (old
	// gocpp binaries, which are rarely run again).
	otherMaxAge = 24 * time.Hour
	// maxEntrySize caps the size of a value: Set refuses a bigger one, and
	// Get reads no file bigger than an entry of that size (export data
	// are a few MB at most).
	maxEntrySize = 256 << 20
)

// magic starts every entry.
var magic = [8]byte{'g', 'o', 'c', 'p', 'p', 'f', 'c', '1'}

// headerSize is the size of the header of an entry: magic, the length of
// the value (8 bytes, little endian) and its SHA-256 hash.
const headerSize = len(magic) + 8 + sha256.Size

// Options say where a Cache is and how it is trimmed.
type Options struct {
	// Dir is the root folder of the cache. It is made if it is missing.
	Dir string

	// Version separates the entries of different programs: the entries of
	// one version are never read by another. "" means the SHA-256 hash of
	// the running executable (computed in the background: the first Get
	// or Set waits for it), so a new gocpp binary starts with an empty
	// cache. The tests set it.
	Version string

	// MaxSize caps the size of the files of the cache, all versions
	// together, and MaxAge the time since an entry was last used. 0 means
	// the default (DefaultMaxSize, DefaultMaxAge).
	MaxSize int64
	MaxAge  time.Duration

	// TrimEvery is the least time between two trims in the background
	// (0: DefaultTrimEvery).
	TrimEvery time.Duration

	// Now is the clock (nil: time.Now). The tests use a fake one.
	Now func() time.Time

	// Logf, if not nil, logs why the cache doesn't work: the executable
	// can't be hashed (then every Get and Set is a miss). It is called at
	// most once.
	Logf func(format string, args ...any)
}

// A Cache is a cache on disk. Its methods are safe for concurrent use, and
// several Caches (in this process or in others) may use the same folder at
// the same time.
type Cache struct {
	root      string
	maxSize   int64
	maxAge    time.Duration
	trimEvery time.Duration
	now       func() time.Time
	logf      func(format string, args ...any)
	maxEntry  int64 // maxEntrySize; the tests make it smaller

	ready  chan struct{} // closed when dir and dirErr are set
	dir    string        // the folder of the version
	dirErr error

	mu       sync.Mutex
	closed   bool
	manual   bool // no trims in the background (the tests)
	trimming bool
	written  int64 // the bytes written since the last trim of this Cache
	wg       sync.WaitGroup
}

// Open opens the cache in the folder opts.Dir, making the folder if it is
// missing, and starts a trim in the background if one is due.
func Open(opts Options) (*Cache, error) {
	if opts.Dir == "" {
		return nil, errors.New("filecache: no folder")
	}
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	c := &Cache{
		root:      root,
		maxSize:   opts.MaxSize,
		maxAge:    opts.MaxAge,
		trimEvery: opts.TrimEvery,
		now:       opts.Now,
		logf:      opts.Logf,
		maxEntry:  maxEntrySize,
		ready:     make(chan struct{}),
	}
	if c.maxSize <= 0 {
		c.maxSize = DefaultMaxSize
	}
	if c.maxAge <= 0 {
		c.maxAge = DefaultMaxAge
	}
	if c.trimEvery <= 0 {
		c.trimEvery = DefaultTrimEvery
	}
	if c.now == nil {
		c.now = time.Now
	}
	if opts.Version != "" {
		c.setVersion(versionHash([]byte(opts.Version)), nil)
	} else {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.setVersion(executableHash())
		}()
	}
	c.maybeTrim(false)
	return c, nil
}

// versionHash returns the hash of a version string.
func versionHash(v []byte) [sha256.Size]byte { return sha256.Sum256(v) }

// executableHash returns the SHA-256 hash of the running executable (about
// 20 ms for a 50 MB binary). On Linux it reads /proc/self/exe, which is the
// file the process runs even when another binary was put at its path after
// the start (go install while the editor runs): the path that
// os.Executable returns would give that one's hash.
func executableHash() ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := openExecutable()
	if err != nil {
		return sum, fmt.Errorf("filecache: can't open the executable: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, fmt.Errorf("filecache: can't read the executable: %w", err)
	}
	h.Sum(sum[:0])
	return sum, nil
}

// openExecutable opens the running executable.
func openExecutable() (*os.File, error) {
	if runtime.GOOS == "linux" {
		if f, err := os.Open("/proc/self/exe"); err == nil {
			return f, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.Open(exe)
}

func (c *Cache) setVersion(sum [sha256.Size]byte, err error) {
	if err != nil {
		c.dirErr = err
		if c.logf != nil {
			c.logf("no cache on disk: %v", err)
		}
	} else {
		// 16 hex digits are enough: this is no defense against an attacker.
		c.dir = filepath.Join(c.root, layout+"-"+hex.EncodeToString(sum[:8]))
	}
	close(c.ready)
}

// Dir returns the root folder of the cache.
func (c *Cache) Dir() string { return c.root }

// Close waits for the background work of the cache (the hash of the
// executable and a trim that runs) to end. The cache can still be used
// after Close, but it starts no more trims.
func (c *Cache) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.wg.Wait()
}

// validKind reports whether kind can be a part of a file name.
func validKind(kind string) bool {
	if kind == "" || len(kind) > 32 {
		return false
	}
	for _, r := range kind {
		if !('a' <= r && r <= 'z' || '0' <= r && r <= '9') {
			return false
		}
	}
	return true
}

// name returns the file name of an entry.
func (c *Cache) name(kind string, key [sha256.Size]byte) (string, error) {
	if !validKind(kind) {
		return "", fmt.Errorf("filecache: bad kind %q", kind)
	}
	<-c.ready
	if c.dirErr != nil {
		return "", c.dirErr
	}
	k := hex.EncodeToString(key[:])
	return filepath.Join(c.dir, k[:2], k+"-"+kind), nil
}

// Get returns the value of the entry kind and key, if the cache has it and
// it is whole. A damaged entry is removed. The caller may keep the value
// but must not change it.
func (c *Cache) Get(kind string, key [sha256.Size]byte) ([]byte, bool) {
	name, err := c.name(kind, key)
	if err != nil {
		return nil, false
	}
	data, fi, err := c.read(name)
	if err != nil {
		return nil, false
	}
	value, ok := decode(data)
	if !ok {
		// Cut short, damaged or too big (or a newer one is being renamed
		// over it right now: then only that entry is lost).
		os.Remove(name)
		return nil, false
	}
	if now := c.now(); now.Sub(fi.ModTime()) > touchAfter {
		os.Chtimes(name, now, now) // the entry is used; an error doesn't matter
	}
	return value, true
}

// read returns the content of the entry file name, and its information. A
// file that is bigger than an entry can be has no content (decode then
// reports it as damaged); something that is not a regular file (a folder)
// is an error.
func (c *Cache) read(name string) ([]byte, fs.FileInfo, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("filecache: %s is not a file", name)
	}
	if fi.Size() > int64(headerSize)+c.maxEntry {
		return nil, fi, nil
	}
	data := make([]byte, fi.Size())
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, nil, err
	}
	return data, fi, nil
}

// Remove removes the entry kind and key, if the cache has it: its value is
// bad for its user (the cache can only tell that it is whole).
func (c *Cache) Remove(kind string, key [sha256.Size]byte) error {
	name, err := c.name(kind, key)
	if err != nil {
		return err
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// decode returns the value of the content of an entry file, and whether
// the content is whole.
func decode(data []byte) ([]byte, bool) {
	if len(data) < headerSize || !bytes.Equal(data[:len(magic)], magic[:]) {
		return nil, false
	}
	n := binary.LittleEndian.Uint64(data[len(magic):])
	value := data[headerSize:]
	if uint64(len(value)) != n {
		return nil, false
	}
	sum := sha256.Sum256(value)
	if !bytes.Equal(sum[:], data[len(magic)+8:headerSize]) {
		return nil, false
	}
	return value, true
}

// Set writes the value of the entry kind and key: to a temporary file, which
// is then renamed over the entry, so that readers never see half of it.
// When two writers write the same entry, the last rename wins; both values
// must be good (the key is the hash of what the value was made from).
func (c *Cache) Set(kind string, key [sha256.Size]byte, value []byte) error {
	if int64(len(value)) > c.maxEntry {
		return fmt.Errorf("filecache: a value of %d bytes is too big", len(value))
	}
	name, err := c.name(kind, key)
	if err != nil {
		return err
	}
	var header [headerSize]byte
	copy(header[:], magic[:])
	binary.LittleEndian.PutUint64(header[len(magic):], uint64(len(value)))
	sum := sha256.Sum256(value)
	copy(header[len(magic)+8:], sum[:])

	f, err := createTemp(filepath.Dir(name), filepath.Base(name)+tempMark+"*")
	if err != nil {
		return err
	}
	_, err = f.Write(header[:])
	if err == nil {
		_, err = f.Write(value)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		// On Windows, the rename fails while another process has the
		// entry open; then the entry stays as it was.
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	c.mu.Lock()
	c.written += int64(headerSize + len(value))
	big := c.written > c.maxSize/2
	c.mu.Unlock()
	c.maybeTrim(big)
	return nil
}

// mkdirTries is how many times createTemp makes the folder again.
const mkdirTries = 10

// createTemp makes a temporary file in the folder dir of an entry, making
// the folder (and the version folder) when it is missing: it is new, or a
// trim has removed it as empty. A trim, in this process or in another one,
// may also remove the folders just after they were made, before the file is
// in them: then createTemp makes them again, a few times. Once the file is
// in the folder, no trim can remove the folder (os.Remove fails for a folder
// that is not empty), so the rename of the file into the entry can't fail
// for a missing folder.
func createTemp(dir, pattern string) (*os.File, error) {
	for try := 0; ; try++ {
		f, err := os.CreateTemp(dir, pattern)
		if err == nil || !errors.Is(err, fs.ErrNotExist) || try == mkdirTries {
			return f, err
		}
		// A trim can also make MkdirAll fail: with ErrNotExist when it
		// removes the version folder after MkdirAll made it and before
		// MkdirAll made the key folder in it, and with ErrExist when another
		// writer made the key folder first and the trim removed it before
		// MkdirAll looked at it. Then the next try makes the folders again.
		if err := os.MkdirAll(dir, 0o700); err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
}

// maybeTrim starts a trim in the background when one is due: the last trim
// (of any process) is older than trimEvery, or force is set (this Cache
// wrote much since its last trim). Only one trim of a Cache runs at a time.
func (c *Cache) maybeTrim(force bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.manual || c.trimming {
		return
	}
	if !force && !c.due() {
		return
	}
	c.trimming = true
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.Trim()
		c.mu.Lock()
		c.trimming = false
		c.mu.Unlock()
	}()
}

// due reports whether the last trim, as trim.txt says, is older than
// trimEvery (or trim.txt is missing or bad).
func (c *Cache) due() bool {
	data, err := os.ReadFile(filepath.Join(c.root, trimFile))
	if err != nil {
		return true
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	last := time.Unix(sec, 0)
	now := c.now()
	return now.Sub(last) >= c.trimEvery || last.After(now.Add(time.Minute))
}

// Trim trims the cache now (see the package documentation): it removes the
// entries not used for MaxAge (a day for the other versions) and the
// temporary files of writers that died, then the entries used least
// recently while the entries take more than MaxSize, then the empty folders
// of the cache. It looks only at the files and folders that the cache owns
// (see the package documentation), and follows no symbolic link. It writes
// the time into trim.txt. Errors (a file that another process removed) are
// ignored: the next trim tries again.
func (c *Cache) Trim() {
	now := c.now()
	<-c.ready
	current := c.dir // "" when the version is unknown: then no folder is old
	type file struct {
		path  string
		mtime time.Time
		size  int64
	}
	var files []file
	var total int64
	var keyDirs, versionDirs []string
	removeOld := func(path string, d fs.DirEntry, maxAge time.Duration) bool {
		fi, err := d.Info()
		if err != nil {
			return true // gone
		}
		if now.Sub(fi.ModTime()) > maxAge {
			os.Remove(path)
			return true
		}
		return false
	}
	tops, _ := os.ReadDir(c.root)
	for _, top := range tops {
		path := filepath.Join(c.root, top.Name())
		if top.Type().IsRegular() && strings.HasPrefix(top.Name(), trimFile+tempMark) {
			removeOld(path, top, tempMaxAge)
			continue
		}
		if !top.IsDir() || !isVersionDir(top.Name()) {
			continue
		}
		maxAge := c.maxAge
		if current != "" && path != current {
			maxAge = min(maxAge, otherMaxAge)
		}
		versionDirs = append(versionDirs, path)
		subs, _ := os.ReadDir(path)
		for _, sub := range subs {
			if !sub.IsDir() || !isHex(sub.Name(), 2) {
				continue
			}
			kk := filepath.Join(path, sub.Name())
			keyDirs = append(keyDirs, kk)
			ents, _ := os.ReadDir(kk)
			for _, e := range ents {
				if !e.Type().IsRegular() {
					continue
				}
				entry, temp := entryName(e.Name(), sub.Name())
				p := filepath.Join(kk, e.Name())
				switch {
				case temp:
					removeOld(p, e, tempMaxAge)
				case !entry:
					// not the cache's
				case !removeOld(p, e, maxAge):
					if fi, err := e.Info(); err == nil {
						files = append(files, file{p, fi.ModTime(), fi.Size()})
						total += fi.Size()
					}
				}
			}
		}
	}
	if total > c.maxSize {
		slices.SortFunc(files, func(a, b file) int { return a.mtime.Compare(b.mtime) })
		for _, f := range files {
			if total <= c.maxSize {
				break
			}
			if os.Remove(f.path) == nil {
				total -= f.size
			}
		}
	}
	// The empty folders of the cache go, the key folders first (a Set that
	// finds its folder gone makes it again). os.Remove fails for a folder
	// that is not empty.
	for _, d := range keyDirs {
		os.Remove(d)
	}
	for _, d := range versionDirs {
		os.Remove(d)
	}
	writeTrimTime(filepath.Join(c.root, trimFile), now)
	c.mu.Lock()
	c.written = 0
	c.mu.Unlock()
}

// isHex reports whether s is n lowercase hex digits.
func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range len(s) {
		if b := s[i]; !('0' <= b && b <= '9' || 'a' <= b && b <= 'f') {
			return false
		}
	}
	return true
}

// isVersionDir reports whether name is the name of a version folder:
// layout, "-" and 16 hex digits (setVersion).
func isVersionDir(name string) bool {
	v, ok := strings.CutPrefix(name, layout+"-")
	return ok && isHex(v, 16)
}

// entryName reports whether name, in the key folder kk, is the name of an
// entry (<key>-<kind>), or of a temporary file of one (<key>-<kind>.tmp-*).
func entryName(name, kk string) (entry, temp bool) {
	base, _, temp := strings.Cut(name, tempMark)
	k, kind, ok := strings.Cut(base, "-")
	if !ok || !isHex(k, 2*sha256.Size) || k[:2] != kk || !validKind(kind) {
		return false, false
	}
	return !temp, temp
}

// writeTrimTime writes the time of a trim into the file name, by a rename,
// so that a reader never sees half of it.
func writeTrimTime(name string, t time.Time) {
	f, err := os.CreateTemp(filepath.Dir(name), trimFile+tempMark+"*")
	if err != nil {
		return
	}
	_, err = fmt.Fprintf(f, "%d\n", t.Unix())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		os.Remove(f.Name())
	}
}
