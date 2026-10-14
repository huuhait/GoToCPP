package filecache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A fakeClock is a clock that the test moves.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// open opens a cache in dir with the version "test" and the clock, and no
// trim in the background (but Trim): trim.txt says that one ran just now.
func open(t *testing.T, dir string, clock *fakeClock, maxSize int64) *Cache {
	t.Helper()
	c := openAuto(t, dir, clock, maxSize)
	c.mu.Lock()
	c.manual = true
	c.mu.Unlock()
	return c
}

// openAuto is open with trims in the background.
func openAuto(t *testing.T, dir string, clock *fakeClock, maxSize int64) *Cache {
	t.Helper()
	writeTrimTime(filepath.Join(dir, trimFile), clock.now())
	c, err := Open(Options{Dir: dir, Version: "test", MaxSize: maxSize, MaxAge: 48 * time.Hour, TrimEvery: time.Hour, Now: clock.now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// newClock returns a fake clock at the real time, so that the files that
// the test doesn't age are new for it.
func newClock() *fakeClock { return &fakeClock{t: time.Now()} }

func key(s string) [32]byte { return sha256.Sum256([]byte(s)) }

// entries returns the names of the entry files under dir, without
// trim.txt.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() != trimFile {
			names = append(names, d.Name())
		}
		return nil
	})
	return names
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := open(t, dir, newClock(), 0)
	if _, ok := c.Get("summary", key("a")); ok {
		t.Fatal("hit in an empty cache")
	}
	for _, v := range [][]byte{[]byte("hello"), {}, bytes.Repeat([]byte{7}, 1<<20)} {
		if err := c.Set("summary", key("a"), v); err != nil {
			t.Fatal(err)
		}
		got, ok := c.Get("summary", key("a"))
		if !ok || !bytes.Equal(got, v) {
			t.Fatalf("Get after Set of %d bytes: %d bytes, %v", len(v), len(got), ok)
		}
	}
	if _, ok := c.Get("other", key("a")); ok {
		t.Error("a hit for another kind")
	}
	if err := c.Set("Bad/kind", key("a"), nil); err == nil {
		t.Error("Set accepted a bad kind")
	}
	// A second cache in the same folder sees the entry; one of another
	// version (another gocpp binary) doesn't.
	c2 := open(t, dir, newClock(), 0)
	if _, ok := c2.Get("summary", key("a")); !ok {
		t.Error("a second cache of the same version misses")
	}
	c3, err := Open(Options{Dir: dir, Version: "other", Now: newClock().now})
	if err != nil {
		t.Fatal(err)
	}
	defer c3.Close()
	if _, ok := c3.Get("summary", key("a")); ok {
		t.Error("a cache of another version hits")
	}
}

// TestExecutableVersion: without a Version, the version is the hash of the
// executable (here the test binary), and the cache works.
func TestExecutableVersion(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Set("summary", key("x"), []byte("v")); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.Get("summary", key("x")); !ok || string(v) != "v" {
		t.Fatalf("Get: %q, %v", v, ok)
	}
	sum, err := executableHash()
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%s-%x", layout, sum[:8]); filepath.Base(c.dir) != want {
		t.Errorf("version folder %s, want %s", filepath.Base(c.dir), want)
	}
}

// TestCorrupt: an entry that is cut short, damaged, empty or of another
// format is a miss, and is removed.
func TestCorrupt(t *testing.T) {
	dir := t.TempDir()
	c := open(t, dir, newClock(), 0)
	value := []byte(strings.Repeat("export data ", 100))
	damage := map[string]func([]byte) []byte{
		"truncated":   func(b []byte) []byte { return b[:len(b)-10] },
		"header only": func(b []byte) []byte { return b[:headerSize] },
		"short":       func(b []byte) []byte { return b[:5] },
		"empty":       func([]byte) []byte { return nil },
		"flipped":     func(b []byte) []byte { b[len(b)/2] ^= 1; return b },
		"magic":       func(b []byte) []byte { b[0] = 'x'; return b },
		"longer":      func(b []byte) []byte { return append(b, 'x') },
	}
	for name, f := range damage {
		k := key(name)
		if err := c.Set("summary", k, value); err != nil {
			t.Fatal(err)
		}
		file, _ := c.name("summary", k)
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, f(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if v, ok := c.Get("summary", k); ok {
			t.Errorf("%s: a hit (%d bytes)", name, len(v))
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("%s: the damaged entry is still there (%v)", name, err)
		}
		// It can be written again.
		if err := c.Set("summary", k, value); err != nil {
			t.Fatal(err)
		}
		if _, ok := c.Get("summary", k); !ok {
			t.Errorf("%s: a miss after the entry was written again", name)
		}
	}
	// A folder in the place of an entry is a miss too (no crash).
	k := key("folder")
	file, _ := c.name("summary", k)
	if err := os.MkdirAll(file, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("summary", k); ok {
		t.Error("a folder is a hit")
	}
}

// TestConcurrentWriters: two caches in the same folder write and read the
// same entries from many goroutines at once: every read is a miss or a
// whole value, and no temporary file is left.
func TestConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	clock := newClock()
	caches := []*Cache{open(t, dir, clock, 0), open(t, dir, clock, 0)}
	values := func(i int) []byte { return bytes.Repeat([]byte{byte(i)}, 64<<10+i) }
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := caches[g%2]
			for i := range 50 {
				k := key(fmt.Sprint(i % 5))
				v := values(i % 5)
				if err := c.Set("summary", k, v); err != nil {
					t.Errorf("Set: %v", err)
				}
				if got, ok := c.Get("summary", k); ok && !bytes.Equal(got, v) {
					t.Errorf("entry %d: a value of %d bytes, want %d", i%5, len(got), len(v))
				}
			}
		}()
	}
	wg.Wait()
	for i := range 5 {
		if got, ok := caches[0].Get("summary", key(fmt.Sprint(i))); !ok || !bytes.Equal(got, values(i)) {
			t.Errorf("entry %d after the writers: %d bytes, %v", i, len(got), ok)
		}
	}
	for _, name := range entries(t, dir) {
		if strings.Contains(name, tempMark) {
			t.Errorf("temporary file left: %s", name)
		}
	}
}

// setAt writes an entry and sets its modification time to t.
func setAt(t *testing.T, c *Cache, k string, size int, at time.Time) string {
	t.Helper()
	if err := c.Set("summary", key(k), bytes.Repeat([]byte("x"), size)); err != nil {
		t.Fatal(err)
	}
	file, _ := c.name("summary", key(k))
	if err := os.Chtimes(file, at, at); err != nil {
		t.Fatal(err)
	}
	return file
}

// has reports whether the cache has the entry k (and marks it as used).
func has(c *Cache, k string) bool {
	_, ok := c.Get("summary", key(k))
	return ok
}

// exists reports whether the entry file of k is there, without a Get.
func exists(c *Cache, k string) bool {
	file, _ := c.name("summary", key(k))
	_, err := os.Stat(file)
	return err == nil
}

// TestTrimAge: a trim removes the entries not used for MaxAge, those of
// other versions after a day, and old temporary files of dead writers.
func TestTrimAge(t *testing.T) {
	dir := t.TempDir()
	clock := newClock()
	c := open(t, dir, clock, 0)
	now := clock.now()
	setAt(t, c, "old", 10, now.Add(-49*time.Hour))
	setAt(t, c, "young", 10, now.Add(-47*time.Hour))
	other, err := Open(Options{Dir: dir, Version: "old gocpp", Now: clock.now})
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	setAt(t, other, "other", 10, now.Add(-25*time.Hour))
	setAt(t, c, "day old", 10, now.Add(-25*time.Hour))
	young, _ := c.name("summary", key("young"))
	deadTemp := young + tempMark + "1"
	liveTemp := young + tempMark + "2"
	deadTrimTemp := filepath.Join(dir, trimFile+tempMark+"3")
	for _, f := range []string{deadTemp, liveTemp, deadTrimTemp} {
		if err := os.WriteFile(f, []byte("half"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	os.Chtimes(deadTemp, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	os.Chtimes(deadTrimTemp, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	os.Chtimes(liveTemp, now.Add(-time.Minute), now.Add(-time.Minute))

	c.Trim()
	if has(c, "old") || !has(c, "young") || !has(c, "day old") || has(other, "other") {
		t.Errorf("after the trim: old %v, young %v, day old %v, other version a day old %v; want false, true, true, false",
			has(c, "old"), has(c, "young"), has(c, "day old"), has(other, "other"))
	}
	for _, f := range []string{deadTemp, deadTrimTemp} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("the old temporary file %s is still there", filepath.Base(f))
		}
	}
	if _, err := os.Stat(liveTemp); err != nil {
		t.Error("the trim removed a temporary file that is being written")
	}
	if _, err := os.Stat(other.dir); !os.IsNotExist(err) {
		t.Error("the empty folder of the other version is still there")
	}
	// A Get marks an entry as used: "young" is 47 hours old, so the Get
	// above set its time to now, and it outlives 2 more hours.
	clock.add(2 * time.Hour)
	c.Trim()
	if !has(c, "young") {
		t.Error("an entry used 2 hours ago was trimmed")
	}
}

// TestTrimSize: a trim removes the entries used least recently while the
// files take more than MaxSize.
// TestTrimForeign: a trim removes nothing that the cache doesn't own, in a
// root folder that has other files (GOCPP_LSP_CACHE=~ or a project folder):
// old files, big files, empty folders, files with names like those of the
// cache in the wrong places, and symbolic links.
func TestTrimForeign(t *testing.T) {
	dir := t.TempDir()
	clock := newClock()
	c := open(t, dir, clock, 1<<10)
	now := clock.now()
	long := now.Add(-30 * 24 * time.Hour)
	setAt(t, c, "mine", 10, long)
	kk := filepath.Base(filepath.Dir(must(c.name("summary", key("x")))))
	k := fmt.Sprintf("%x", key("foreign"))
	foreign := map[string]int{ // name: size
		"notes.txt":        10,
		"big.iso":          4 << 10,
		"rt/abc/runtime.o": 10,
		"trim.txt.bak":     10,
		"v1-notahexversion/" + kk + "/" + k + "-summary":                                           10,
		filepath.Base(c.dir) + "/notes.txt":                                                        10,
		filepath.Base(c.dir) + "/zz/" + k + "-summary":                                             10,
		filepath.Base(c.dir) + "/" + kk + "/" + k:                                                  10, // no kind
		filepath.Base(c.dir) + "/" + kk + "/" + k + "-Summary":                                     10,
		filepath.Base(c.dir) + "/" + kk + "/notes" + tempMark + "1":                                10,
		filepath.Base(c.dir) + "/" + kk + "/" + fmt.Sprintf("%x", key("a"))[:62] + kk + "-summary": 10,
	}
	for name, size := range foreign {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, bytes.Repeat([]byte("u"), size), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(path, long, long)
	}
	emptyDirs := []string{"empty", "v1-old", filepath.Join(filepath.Base(c.dir), "empty")}
	for _, d := range emptyDirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A link to a folder outside, with an old entry-like file in it.
	outside := t.TempDir()
	outFile := filepath.Join(outside, kk, k+"-summary")
	os.MkdirAll(filepath.Dir(outFile), 0o755)
	os.WriteFile(outFile, []byte("u"), 0o644)
	os.Chtimes(outFile, long, long)
	linked := filepath.Join(dir, "v1-0123456789abcdef")
	if err := os.Symlink(outside, linked); err != nil {
		t.Logf("no symbolic link: %v", err)
	}

	c.Trim()
	if exists(c, "mine") {
		t.Error("the old entry of the cache is still there")
	}
	for name := range foreign {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Errorf("the trim removed %s: %v", name, err)
		}
	}
	for _, d := range emptyDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("the trim removed the empty folder %s: %v", d, err)
		}
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("the trim followed a link: %v", err)
	}
	// The cache still works, and its own empty folders went.
	if err := c.Set("summary", key("new"), []byte("v")); err != nil || !has(c, "new") {
		t.Errorf("Set after the trim: %v", err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// TestGetTooBig: a file bigger than an entry can be is not read: a miss,
// and it is removed; Set refuses a value that big.
func TestGetTooBig(t *testing.T) {
	c := open(t, t.TempDir(), newClock(), 0)
	c.maxEntry = 100
	if err := c.Set("summary", key("big"), make([]byte, 101)); err == nil {
		t.Error("Set of a value too big")
	}
	if err := c.Set("summary", key("ok"), make([]byte, 100)); err != nil || !has(c, "ok") {
		t.Errorf("Set of a value at the cap: %v", err)
	}
	file := must(c.name("summary", key("big")))
	os.MkdirAll(filepath.Dir(file), 0o700)
	if err := os.WriteFile(file, make([]byte, headerSize+101), 0o600); err != nil {
		t.Fatal(err)
	}
	if has(c, "big") || exists(c, "big") {
		t.Error("a file too big is a hit, or is still there")
	}
}

// TestHashError: when the executable can't be hashed, every Get and Set is a
// miss, and the cache says why once.
func TestHashError(t *testing.T) {
	var logs []string
	c := &Cache{ready: make(chan struct{}), logf: func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}}
	c.setVersion([32]byte{}, fmt.Errorf("filecache: can't open the executable: %w", fs.ErrPermission))
	if c.Set("summary", key("a"), []byte("v")) == nil {
		t.Error("Set without a version")
	}
	if _, ok := c.Get("summary", key("a")); ok {
		t.Error("Get without a version")
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "can't open the executable") {
		t.Errorf("the log: %q", logs)
	}
}

func TestTrimSize(t *testing.T) {
	dir := t.TempDir()
	clock := newClock()
	const size = 1000
	c := open(t, dir, clock, int64(3*(size+headerSize)+10))
	now := clock.now()
	for i, k := range []string{"a", "b", "c", "d", "e"} {
		setAt(t, c, k, size, now.Add(time.Duration(i-10)*time.Minute)) // a is the oldest
	}
	c.Trim()
	got := ""
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		if has(c, k) {
			got += k
		}
	}
	if got != "cde" {
		t.Errorf("entries after the trim: %q, want \"cde\"", got)
	}
}

// TestTrimEvery: a trim in the background runs when the cache opens or
// writes, at most once per TrimEvery (trim.txt), or when one cache wrote
// more than half of MaxSize since its last trim.
func TestTrimEvery(t *testing.T) {
	dir := t.TempDir()
	clock := newClock()
	c := openAuto(t, dir, clock, 1<<20)
	old := clock.now().Add(-72 * time.Hour)
	setAt(t, c, "old", 10, old) // the Set doesn't trim: trim.txt is new
	c.maybeTrim(false)
	c.wg.Wait()
	if !exists(c, "old") {
		t.Fatal("a trim ran before TrimEvery passed")
	}
	clock.add(2 * time.Hour)
	c.maybeTrim(false)
	c.wg.Wait()
	if exists(c, "old") {
		t.Fatal("no trim ran after TrimEvery passed")
	}
	// trim.txt has the new time: no trim for another hour, but a cache
	// that writes more than MaxSize/2 trims at once.
	setAt(t, c, "old", 10, old)
	c.wg.Wait()
	if !exists(c, "old") {
		t.Fatal("a second trim ran within TrimEvery")
	}
	if err := c.Set("summary", key("big"), make([]byte, 600<<10)); err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	if exists(c, "old") {
		t.Error("no trim after writing more than half of MaxSize")
	}
	// A new cache in the folder trims when it opens, if a trim is due.
	setAt(t, c, "old", 10, old)
	c.wg.Wait()
	clock.add(2 * time.Hour)
	c2, err := Open(Options{Dir: dir, Version: "test", MaxAge: 48 * time.Hour, Now: clock.now})
	if err != nil {
		t.Fatal(err)
	}
	c2.Close()
	if exists(c, "old") {
		t.Error("Open didn't trim when a trim was due")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"100": 100, "4K": 4 << 10, "512M": 512 << 20, "1G": 1 << 30, "2GiB": 2 << 30, "3mb": 3 << 20} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Errorf("ParseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "G", "-1", "0", "1T", "x"} {
		if _, err := ParseSize(in); err == nil {
			t.Errorf("ParseSize(%q) accepted", in)
		}
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(EnvDir, "off")
	if _, ok, err := FromEnv(); ok || err != nil {
		t.Errorf("off: %v, %v", ok, err)
	}
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	t.Setenv(EnvSize, "64M")
	t.Setenv(EnvAge, "72h")
	opts, ok, err := FromEnv()
	if !ok || err != nil || opts.Dir != dir || opts.MaxSize != 64<<20 || opts.MaxAge != 72*time.Hour {
		t.Errorf("FromEnv: %+v, %v, %v", opts, ok, err)
	}
	t.Setenv(EnvAge, "soon")
	if _, _, err := FromEnv(); err == nil {
		t.Error("a bad age is accepted")
	}
	t.Setenv(EnvAge, "")
	for _, rel := range []string{"cache", "./cache", "~/.cache/gocpp"} {
		t.Setenv(EnvDir, rel)
		if _, ok, err := FromEnv(); ok || err == nil || !strings.Contains(err.Error(), "absolute") {
			t.Errorf("%s=%s: %v, %v", EnvDir, rel, ok, err)
		}
	}
}
