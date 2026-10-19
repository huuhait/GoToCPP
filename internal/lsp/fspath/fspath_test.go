package fspath

import (
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWindowsNames checks the name functions of the Windows flavor against the rules of Go's path/filepath on
// Windows (the expected values are what filepath returns there).
func TestWindowsNames(t *testing.T) {
	w := Windows
	clean := []struct{ in, want string }{
		{``, `.`},
		{`C:\`, `C:\`},
		{`C:`, `C:.`},
		{`c:/Users/me//proj/./x`, `c:\Users\me\proj\x`},
		{`C:\a\..\..\b`, `C:\b`},
		{`C:\a\b\`, `C:\a\b`},
		{`C:a\b`, `C:a\b`},
		{`\a\b`, `\a\b`},
		{`a/b/../c`, `a\c`},
		{`\\srv\share`, `\\srv\share`},
		{`\\srv\share\d\..\e`, `\\srv\share\e`},
		{`//srv/share/d`, `\\srv\share\d`},
		{`\\?\C:\a\b`, `C:\a\b`},
		{`\\?\UNC\srv\share\x`, `\\srv\share\x`},
		{`\\?\unc\srv\share\x`, `\\srv\share\x`},
	}
	for _, c := range clean {
		if got := w.Clean(c.in); got != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	dir := []struct{ in, want string }{
		{`C:\a\b.gp`, `C:\a`},
		{`C:\a`, `C:\`},
		{`C:\`, `C:\`},
		{`C:a`, `C:.`},
		{`\\srv\share\d\x`, `\\srv\share\d`},
		{`\\srv\share\x`, `\\srv\share\`},
		{`\\srv\share`, `\\srv\share`},
		{`a\b`, `a`},
		{`b`, `.`},
	}
	for _, c := range dir {
		if got := w.Dir(c.in); got != c.want {
			t.Errorf("Dir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	base := []struct{ in, want string }{
		{`C:\a\b.gp`, `b.gp`},
		{`C:\a\`, `a`},
		{`C:\`, `\`},
		{`\\srv\share\x y`, `x y`},
		{`c:/a/b`, `b`},
	}
	for _, c := range base {
		if got := w.Base(c.in); got != c.want {
			t.Errorf("Base(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := w.Join(`C:\a`, `b/c`, ``, `..\d`); got != `C:\a\b\d` {
		t.Errorf("Join = %q", got)
	}
	if got := w.Join(`C:`, ``, `a`); got != `C:a` {
		t.Errorf("Join of a bare drive = %q, want C:a", got)
	}
	if got := w.Base(``); got != `.` {
		t.Errorf("Base of nothing = %q", got)
	}
	if got := w.Volume(`//srv/share/d`); got != `\\srv\share` {
		t.Errorf("Volume of a UNC name with slashes = %q", got)
	}
	for in, want := range map[string]bool{`C:\a`: true, `C:/a`: true, `C:a`: false, `C:`: false, `\a`: false, `a\b`: false, `\\srv\share\x`: true, `\\?\C:\a`: true} {
		if got := w.IsAbs(in); got != want {
			t.Errorf("IsAbs(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestPosixIsPath: the other flavors are the slash path functions.
func TestPosixIsPath(t *testing.T) {
	for _, f := range []Flavor{Posix, CaseFold} {
		for _, p := range []string{"/", "/a/b/../c", "a//b", "", "/a/b/"} {
			if f.Clean(p) != path.Clean(p) || f.Dir(p) != path.Dir(p) || f.Base(p) != path.Base(p) {
				t.Errorf("%+v: %q differs from package path", f, p)
			}
		}
	}
}

func TestKeyEqualWithin(t *testing.T) {
	w := Windows
	for _, c := range []struct {
		a, b string
		eq   bool
	}{
		{`C:\Users\Me\P.gp`, `c:\users\me\p.GP`, true},
		{`C:\Users\Me\P.gp`, `c:/users/me//p.gp`, true},
		{`C:\Users\Me\P.gp`, `C:\Users\Me\Q.gp`, false},
		{`C:\Ünï\x`, `c:\ünï\X`, true},
		{`C:\日本\x`, `C:\日本\X`, true},
		{`\\SRV\Share\x`, `\\srv\share\X`, true},
		{`C:\a`, `D:\a`, false},
	} {
		if got := w.Equal(c.a, c.b); got != c.eq {
			t.Errorf("Equal(%q, %q) = %v, want %v", c.a, c.b, got, c.eq)
		}
	}
	if Posix.Equal("/a/B", "/a/b") || !Posix.Equal("/a/b", "/a/b") {
		t.Error("the Posix flavor is case-sensitive")
	}
	if !CaseFold.Equal("/a/B", "/a/b") {
		t.Error("the CaseFold flavor is not")
	}
	for _, c := range []struct {
		dir, file string
		in        bool
	}{
		{`C:\Proj`, `c:\proj\x.gp`, true},
		{`C:\Proj`, `C:\PROJ`, true},
		{`C:\Proj`, `C:\Project\x.gp`, false},
		{`C:\Proj\`, `C:\proj\sub\x.gp`, true},
		{`C:\`, `c:\a`, true},
		{`C:\Proj`, `D:\Proj\x`, false},
	} {
		if got := w.Within(c.dir, c.file); got != c.in {
			t.Errorf("Within(%q, %q) = %v, want %v", c.dir, c.file, got, c.in)
		}
	}
	if !w.HasPrefixDir(`C:\Proj\sub\x.gp`, `c:\proj\`) || w.HasPrefixDir(`C:\Projx\a`, `C:\Proj\`) || Posix.HasPrefixDir("/A/b", "/a/") {
		t.Error("HasPrefixDir")
	}
}

func TestURIParts(t *testing.T) {
	w := Windows
	for _, c := range []struct{ name, host, p string }{
		{`C:\Users\Me\a b\x.gp`, ``, `/C:/Users/Me/a b/x.gp`},
		{`c:\x`, ``, `/C:/x`},
		{`C:\`, ``, `/C:/`},
		{`\\srv\share\d\x.gp`, `srv`, `/share/d/x.gp`},
		{`\\srv\share`, `srv`, `/share`},
		{`\\?\C:\x`, ``, `/C:/x`},
		{`\\?\UNC\srv\share\x`, `srv`, `/share/x`},
	} {
		host, p := w.URIParts(c.name)
		if host != c.host || p != c.p {
			t.Errorf("URIParts(%q) = %q, %q; want %q, %q", c.name, host, p, c.host, c.p)
		}
	}
	for _, c := range []struct{ host, p, want string }{
		{``, `/C:/Users/Me/a b/x.gp`, `C:\Users\Me\a b\x.gp`},
		{``, `/c:/x`, `C:\x`},
		{``, `/c:`, `C:\`},
		{`localhost`, `/C:/x`, `C:\x`},
		{`localhost`, `/c$/Users/x.gp`, `\\localhost\c$\Users\x.gp`},
		{`srv`, `/share/d/x.gp`, `\\srv\share\d\x.gp`},
		{``, `//srv/share/d`, `\\srv\share\d`},
		{``, `/x/y`, `\x\y`},
	} {
		if got := w.FromURIParts(c.host, c.p); got != c.want {
			t.Errorf("FromURIParts(%q, %q) = %q, want %q", c.host, c.p, got, c.want)
		}
	}
	if got := Posix.FromURIParts("wsl$", "/Ubuntu/x"); got != "/wsl$/Ubuntu/x" {
		t.Errorf("Posix FromURIParts with a host = %q", got)
	}
}

// fakeFS is a Windows-like file system: it lists a folder under any spelling of its name.
type fakeFS struct {
	dirs  map[string][]string // folded folder name -> entries
	lists atomic.Int64
}

func newFakeFS(entries ...string) *fakeFS {
	fs := &fakeFS{dirs: map[string][]string{}}
	for _, e := range entries {
		fs.add(e)
	}
	return fs
}

// add makes the file or folder e, and the folders above it.
func (fs *fakeFS) add(e string) {
	e = strings.TrimSuffix(e, "/")
	for {
		dir, name := path.Split(e)
		dir = strings.TrimSuffix(dir, "/")
		if dir == "" {
			dir = "/"
		}
		k := strings.ToLower(dir)
		found := false
		for _, n := range fs.dirs[k] {
			found = found || n == name
		}
		if !found {
			fs.dirs[k] = append(fs.dirs[k], name)
		}
		if dir == "/" {
			return
		}
		e = dir
	}
}

func (fs *fakeFS) remove(e string) {
	dir, name := path.Split(e)
	k := strings.ToLower(strings.TrimSuffix(dir, "/"))
	var keep []string
	for _, n := range fs.dirs[k] {
		if n != name {
			keep = append(keep, n)
		}
	}
	fs.dirs[k] = keep
}

func (fs *fakeFS) list(dir string) ([]string, error) {
	fs.lists.Add(1)
	dir = strings.ReplaceAll(dir, `\`, "/")
	// C:/ -> /C: (the drive is a folder of the fake root)
	if len(dir) >= 2 && dir[1] == ':' {
		dir = "/" + dir
	}
	if dir != "/" {
		dir = strings.TrimSuffix(dir, "/")
	}
	names, ok := fs.dirs[strings.ToLower(dir)]
	if !ok {
		return nil, os.ErrNotExist
	}
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out, nil
}

func TestCanonWindows(t *testing.T) {
	fs := newFakeFS("/C:/Users/Me/Proj Dir/Lib/Util.gp", "/C:/Users/Me/Proj Dir/Main.gp", "/C:/Users/Me/Ünï Ça/x.gp", "/D:/Data/a")
	defer SetLister(fs.list)()
	for _, c := range []struct{ in, want string }{
		{`c:\users\me\proj dir\main.GP`, `C:\Users\Me\Proj Dir\Main.gp`},
		{`C:/USERS/ME/PROJ DIR/lib/util.gp`, `C:\Users\Me\Proj Dir\Lib\Util.gp`},
		{`c:\users\me\ünï ça\X.gp`, `C:\Users\Me\Ünï Ça\x.gp`},
		{`C:\Users\Me\Proj Dir\New.gp`, `C:\Users\Me\Proj Dir\New.gp`},                 // not there yet: as given
		{`c:\users\me\proj dir\lib\Sub\New.gp`, `C:\Users\Me\Proj Dir\Lib\Sub\New.gp`}, // the part that exists is fixed
		{`d:\data\A`, `D:\Data\a`},
		{`C:\`, `C:\`},
		{`c:\users\me\proj dir\..\PROJ DIR\main.gp`, `C:\Users\Me\Proj Dir\Main.gp`},
		{`\\?\c:\users\me`, `C:\Users\Me`},
		{`relative\path`, `relative\path`},
	} {
		if got := Windows.Canon(c.in); got != c.want {
			t.Errorf("Canon(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// A Posix flavor keeps the name as it is.
	if got := Posix.Canon(`/a/../B`); got != `/a/../B` {
		t.Errorf("Posix Canon = %q", got)
	}
}

func TestCanonMemoAndInvalidate(t *testing.T) {
	fs := newFakeFS("/C:/p/Main.gp")
	defer SetLister(fs.list)()
	defer SetTimes(0, 5*time.Second)() // a missing name lists the folder again at once
	f := Windows
	if got := f.Canon(`c:\P\main.gp`); got != `C:\p\Main.gp` {
		t.Fatalf("Canon = %q", got)
	}
	n := fs.lists.Load()
	for i := 0; i < 5; i++ {
		f.Canon(`c:\P\main.gp`)
	}
	if fs.lists.Load() != n {
		t.Errorf("a repeated name listed folders again: %d -> %d", n, fs.lists.Load())
	}
	// A file created later is found without Invalidate for a name that was never asked, because a miss lists the folder again.
	fs.add("/C:/p/Later.gp")
	if got := f.Canon(`c:\p\LATER.gp`); got != `C:\p\Later.gp` {
		t.Errorf("a new file: Canon = %q", got)
	}
	// A name that was asked while the file was missing is remembered as given, until the server says a file changed.
	if got := f.Canon(`c:\p\later2.gp`); got != `C:\p\later2.gp` {
		t.Fatalf("Canon of a missing file = %q", got)
	}
	fs.add("/C:/p/Later2.gp")
	if got := f.Canon(`c:\p\later2.gp`); got != `C:\p\later2.gp` {
		t.Errorf("the answer for a missing file changed without Invalidate: %q", got)
	}
	Invalidate()
	if got := f.Canon(`c:\p\later2.gp`); got != `C:\p\Later2.gp` {
		t.Errorf("after Invalidate: Canon = %q", got)
	}
	// A case-only rename.
	fs.remove("/C:/p/Main.gp")
	fs.add("/C:/p/MAIN.gp")
	Invalidate()
	if got := f.Canon(`c:\p\main.gp`); got != `C:\p\MAIN.gp` {
		t.Errorf("after a rename of the case: Canon = %q", got)
	}
}

func TestCanonCaseFoldPosix(t *testing.T) {
	fs := newFakeFS("/Work/Proj/Main.gp")
	defer SetLister(func(dir string) ([]string, error) { return fs.list(dir) })()
	if got := CaseFold.Canon("/work/PROJ/main.gp"); got != "/Work/Proj/Main.gp" {
		t.Errorf("Canon = %q", got)
	}
}

func TestUnlistableFolder(t *testing.T) {
	defer SetLister(func(string) ([]string, error) { return nil, errors.New("access denied") })()
	if got := Windows.Canon(`c:\secret\x`); got != `C:\secret\x` {
		t.Errorf("Canon of a folder that cannot be listed = %q", got)
	}
}

func TestUse(t *testing.T) {
	restore := Use(Windows)
	if !Current().Windows || Sep() != '\\' || Dir(`C:\a\b`) != `C:\a` {
		t.Error("Use(Windows)")
	}
	restore()
	if reflect.DeepEqual(Current(), Windows) && Sep() != '\\' {
		t.Error("restore")
	}
}

func TestCanonShortNames(t *testing.T) {
	fs := newFakeFS("/C:/Program Files/Gocpp/Bin/gocpp.exe", "/C:/Users/Administrator/Proj/Main.gp")
	defer SetLister(fs.list)()
	long := map[string]string{
		`C:\PROGRA~1`:                          `C:\Program Files`,
		`C:\PROGRA~1\GOCPP~1`:                  `C:\Program Files\Gocpp`,
		`C:\PROGRA~1\GOCPP~1\Bin`:              `C:\Program Files\Gocpp\Bin`,
		`C:\Users\ADMINI~1`:                    `C:\Users\Administrator`,
		`C:\Users\ADMINI~1\Proj`:               `C:\Users\Administrator\Proj`,
		`C:\Users\ADMINI~1\Proj\Main.gp`:       `C:\Users\Administrator\Proj\Main.gp`,
		`C:\Program Files\Gocpp\Bin\gocpp.exe`: `C:\Program Files\Gocpp\Bin\gocpp.exe`,
		`C:\Users\Administrator\Proj`:          `C:\Users\Administrator\Proj`,
		`C:\Users\Administrator\Proj\Main.gp`:  `C:\Users\Administrator\Proj\Main.gp`,
		`C:\Users\Administrator`:               `C:\Users\Administrator`,
	}
	folded := map[string]string{}
	for k, v := range long {
		folded[strings.ToLower(k)] = v // the system does not care about case
	}
	defer SetLongName(func(p string) (string, bool) { l, ok := folded[strings.ToLower(p)]; return l, ok })()
	for _, c := range []struct{ in, want string }{
		{`c:\progra~1\gocpp~1\bin\gocpp.exe`, `C:\Program Files\Gocpp\Bin\gocpp.exe`},
		{`C:\PROGRA~1\GOCPP~1\bin\GOCPP.EXE`, `C:\Program Files\Gocpp\Bin\gocpp.exe`},
		{`C:\Users\ADMINI~1\Proj\main.gp`, `C:\Users\Administrator\Proj\Main.gp`},
		{`C:\Users\ADMINI~1\Proj\New.gp`, `C:\Users\Administrator\Proj\New.gp`}, // a file that is not there: the folder is expanded
		{`C:\Users\a~b\x`, `C:\Users\a~b\x`},                                    // a name with ~ that is not a short name
	} {
		if got := Windows.Canon(c.in); got != c.want {
			t.Errorf("Canon(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A folder that tells two names apart by case (a WSL share): a name is found only in its own spelling.
func TestCanonCaseSensitiveFolder(t *testing.T) {
	fs := newFakeFS("/C:/share/Foo/x.gp", "/C:/share/foo/y.gp", "/C:/share/Bar/z.gp")
	defer SetLister(fs.list)()
	for _, c := range []struct{ in, want string }{
		{`c:\share\Foo\x.gp`, `C:\share\Foo\x.gp`},
		{`c:\share\foo\y.gp`, `C:\share\foo\y.gp`},
		{`c:\share\FOO\x.gp`, `C:\share\FOO\x.gp`}, // matches neither: as given
		{`c:\SHARE\bar\z.gp`, `C:\share\Bar\z.gp`}, // the other names in the folder are still found
	} {
		if got := Windows.Canon(c.in); got != c.want {
			t.Errorf("Canon(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanRelativeDriveLike(t *testing.T) {
	if got := Windows.Clean(`./c:`); got != `.\c:` {
		t.Errorf("Clean(./c:) = %q", got)
	}
	if got := Windows.Clean(Windows.Clean(`./c:`)); got != `.\c:` {
		t.Errorf("Clean is not stable: %q", got)
	}
}

// A name is not looked for again in a folder that lacks it until missTTL has passed.
func TestCanonMissTTL(t *testing.T) {
	fs := newFakeFS("/C:/p/Main.gp")
	defer SetLister(fs.list)()
	clock := time.Unix(1000, 0)
	old := now
	now = func() time.Time { return clock }
	defer func() { now = old }()
	defer SetTimes(time.Second, 5*time.Second)()
	f := Windows
	f.Canon(`c:\p\a.gp`) // missing: the folder is listed once
	n := fs.lists.Load()
	for i := 0; i < 10; i++ {
		f.Canon(fmt.Sprintf(`c:\p\missing%d.gp`, i))
	}
	if got := fs.lists.Load(); got != n {
		t.Errorf("ten missing names listed the folder %d more times, want none", got-n)
	}
	fs.add("/C:/p/Later.gp")
	clock = clock.Add(2 * time.Second)
	if got := f.Canon(`c:\p\LATER.gp`); got != `C:\p\Later.gp` {
		t.Errorf("after the TTL, a new file: %q", got)
	}
}

// Callers that want the same folder at the same time share one listing.
func TestCanonOneListing(t *testing.T) {
	fs := newFakeFS("/C:/p/Main.gp")
	var calls atomic.Int64
	release := make(chan struct{})
	defer SetLister(func(dir string) ([]string, error) {
		calls.Add(1)
		<-release
		return fs.list(dir)
	})()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := Windows.Canon(`c:\p\main.gp`); got != `C:\p\Main.gp` {
				t.Errorf("Canon = %q", got)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	// The folders on the way are C: and p: two listings, whatever the number of callers.
	if got := calls.Load(); got != 2 {
		t.Errorf("8 callers made %d listings, want 2", got)
	}
}

// A folder that does not answer is given up after the timeout, remembered, and the names stay as given.
func TestCanonDeadFolder(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	var calls atomic.Int64
	defer SetLister(func(dir string) ([]string, error) {
		calls.Add(1)
		<-block
		return nil, nil
	})()
	defer SetTimes(0, 20*time.Millisecond)()
	start := time.Now()
	if got := Windows.Canon(`\\dead\share\a\b.gp`); got != `\\dead\share\a\b.gp` {
		t.Errorf("Canon = %q", got)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("Canon took %v", time.Since(start))
	}
	n := calls.Load()
	for i := 0; i < 5; i++ {
		Windows.Canon(fmt.Sprintf(`\\dead\share\a\c%d.gp`, i))
	}
	if got := calls.Load(); got != n {
		t.Errorf("a dead folder was asked again %d times", got-n)
	}
}

// What Invalidate says happens while a name is worked out is not kept.
func TestCanonInvalidateDuringResolve(t *testing.T) {
	fs := newFakeFS("/C:/p/Main.gp")
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer SetLister(func(dir string) ([]string, error) {
		once.Do(func() { close(started); <-release })
		return fs.list(dir)
	})()
	done := make(chan string)
	go func() { done <- Windows.Canon(`c:\p\main.gp`) }()
	<-started
	Invalidate()
	close(release)
	<-done
	canonMemo.Range(func(k, v any) bool {
		if v.(canonValue).gen == generation.Load() {
			t.Errorf("%v was kept although Invalidate came while it was worked out", k)
		}
		return true
	})
}

func TestCanonMemoBound(t *testing.T) {
	fs := newFakeFS("/C:/p/x")
	defer SetLister(fs.list)()
	old := maxCanonMemo
	maxCanonMemo = 40
	defer func() { maxCanonMemo = old }()
	for i := 0; i < 500; i++ {
		Windows.Canon(fmt.Sprintf(`c:\p\n%d`, i))
	}
	n := 0
	canonMemo.Range(func(_, _ any) bool { n++; return true })
	if n > 45 {
		t.Errorf("%d names are remembered, the bound is 40", n)
	}
}

// A name with ~ that the system returns as it is does not make Canon loop.
func TestCanonShortNameUnchanged(t *testing.T) {
	fs := newFakeFS("/C:/a~b/x.gp")
	defer SetLister(fs.list)()
	defer SetLongName(func(p string) (string, bool) { return p, true })()
	done := make(chan string)
	go func() { done <- Windows.Canon(`c:\A~B\X.gp`) }()
	select {
	case got := <-done:
		if got != `C:\a~b\x.gp` {
			t.Errorf("Canon = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Canon does not end")
	}
}
