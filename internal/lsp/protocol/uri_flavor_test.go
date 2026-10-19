package protocol_test

import (
	"encoding/json"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The URI rules of a Windows server, run on any system with the Windows flavor of fspath (step 14.5, ADR-096).
func windows(t *testing.T, files ...string) {
	t.Helper()
	set := map[string][]string{}
	for _, f := range files {
		for {
			dir, name := path.Split(f)
			dir = strings.TrimSuffix(dir, "/")
			if dir == "" {
				dir = "/"
			}
			set[strings.ToLower(dir)] = append(set[strings.ToLower(dir)], name)
			if dir == "/" {
				break
			}
			f = dir
		}
	}
	t.Cleanup(fspath.SetLister(func(dir string) ([]string, error) {
		dir = strings.ReplaceAll(dir, `\`, "/")
		if len(dir) >= 2 && dir[1] == ':' {
			dir = "/" + dir
		}
		if dir != "/" {
			dir = strings.TrimSuffix(dir, "/")
		}
		names, ok := set[strings.ToLower(dir)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return names, nil
	}))
	t.Cleanup(fspath.Use(fspath.Windows))
}

func TestWindowsParseDocumentURI(t *testing.T) {
	windows(t)
	for _, c := range []struct{ in, want, path string }{
		{`file:///c:/Users/Me/a%20b/x.gp`, `file:///C:/Users/Me/a%20b/x.gp`, `C:\Users\Me\a b\x.gp`},
		{`file:///C%3A/Users/Me/x.gp`, `file:///C:/Users/Me/x.gp`, `C:\Users\Me\x.gp`},
		{`file:///c%3a/Users/Me/x.gp`, `file:///C:/Users/Me/x.gp`, `C:\Users\Me\x.gp`},
		{`file://c:/Users/Me/x.gp`, `file:///C:/Users/Me/x.gp`, `C:\Users\Me\x.gp`},
		{`file://localhost/c:/x.gp`, `file:///C:/x.gp`, `C:\x.gp`},
		{`file:///C:/`, `file:///C:/`, `C:\`},
		{`file:///C%3A`, `file:///C:/`, `C:\`},
		{`file:///c:/%E6%97%A5%E6%9C%AC/x.gp`, `file:///C:/%E6%97%A5%E6%9C%AC/x.gp`, `C:\日本\x.gp`},
		{`file:///c:/%F0%9F%98%80/x.gp`, `file:///C:/%F0%9F%98%80/x.gp`, "C:\\\U0001F600\\x.gp"},
		{`file:///c:/a+b/x%2Bc.gp`, `file:///C:/a+b/x+c.gp`, `C:\a+b\x+c.gp`},
		{`file:///c:/a%25b/x.gp`, `file:///C:/a%25b/x.gp`, `C:\a%b\x.gp`},
		{`file:///c:/a#b/x.gp`, `file:///C:/a%23b/x.gp`, `C:\a#b\x.gp`},
		// UNC: the authority is the host.
		{`file://srv/share/d/x.gp`, `file://srv/share/d/x.gp`, `\\srv\share\d\x.gp`},
		{`file://SRV/Share/d%20e/x.gp`, `file://SRV/Share/d%20e/x.gp`, `\\SRV\Share\d e\x.gp`},
		{`file:////srv/share/d/x.gp`, `file://srv/share/d/x.gp`, `\\srv\share\d\x.gp`},
		{`file://wsl%24/Ubuntu/home/x.gp`, `file://wsl$/Ubuntu/home/x.gp`, `\\wsl$\Ubuntu\home\x.gp`},
	} {
		uri, err := protocol.ParseDocumentURI(c.in)
		if err != nil {
			t.Errorf("ParseDocumentURI(%q): %v", c.in, err)
			continue
		}
		if string(uri) != c.want {
			t.Errorf("ParseDocumentURI(%q) = %q, want %q", c.in, uri, c.want)
		}
		if got := uri.Path(); got != c.path {
			t.Errorf("Path of %q = %q, want %q", c.in, got, c.path)
		}
		// The URI of the path is the URI again (the form is canonical).
		if back := protocol.URIFromPath(c.path); back != uri {
			t.Errorf("URIFromPath(%q) = %q, want %q", c.path, back, uri)
		}
	}
	if _, err := protocol.ParseDocumentURI("https://x/y"); err == nil {
		t.Error("a URI that is not a file URI was taken")
	}
}

func TestWindowsURIFromPath(t *testing.T) {
	windows(t)
	for _, c := range []struct{ path, want string }{
		{`C:\Windows\System32`, `file:///C:/Windows/System32`},
		{`c:\Go\src\bob george\x.go`, `file:///C:/Go/src/bob%20george/x.go`},
		{`C:/Mixed\slashes/x.gp`, `file:///C:/Mixed/slashes/x.gp`},
		{`C:\a\..\b\.\x.gp`, `file:///C:/b/x.gp`},
		{`\\?\C:\long\x.gp`, `file:///C:/long/x.gp`},
		{`\\srv\share\x y.gp`, `file://srv/share/x%20y.gp`},
		{`\\?\UNC\srv\share\x.gp`, `file://srv/share/x.gp`},
		{`C:\日本語\café.gp`, `file:///C:/%E6%97%A5%E6%9C%AC%E8%AA%9E/caf%C3%A9.gp`},
		{`C:\100%\a#b\c?d.gp`, `file:///C:/100%25/a%23b/c%3Fd.gp`},
		{`C:\`, `file:///C:/`},
		{``, ``},
	} {
		if got := protocol.URIFromPath(c.path); string(got) != c.want {
			t.Errorf("URIFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestWindowsPathCase: a path is in the case of the disk, however the editor spelled it, and the Key of two spellings is one.
func TestWindowsPathCase(t *testing.T) {
	windows(t, "/C:/Users/Me/Proj Dir/Lib/Util.gp", "/C:/Users/Me/Proj Dir/Main.gp")
	a, _ := protocol.ParseDocumentURI(`file:///c%3A/users/me/proj%20dir/main.gp`)
	b, _ := protocol.ParseDocumentURI(`file:///C:/Users/Me/Proj%20Dir/Main.gp`)
	if a == b {
		t.Fatalf("the spellings should differ before Key: %s", a)
	}
	want := `C:\Users\Me\Proj Dir\Main.gp`
	if a.Path() != want || b.Path() != want {
		t.Errorf("Path: %q, %q; want %q", a.Path(), b.Path(), want)
	}
	if a.Key() != b.Key() || a.Key() != "file:///C:/Users/Me/Proj%20Dir/Main.gp" {
		t.Errorf("Key: %q, %q", a.Key(), b.Key())
	}
	if a.Base() != "Main.gp" || a.DirPath() != `C:\Users\Me\Proj Dir` || a.Dir() != protocol.URIFromPath(`C:\Users\Me\Proj Dir`) {
		t.Errorf("Base %q DirPath %q Dir %q", a.Base(), a.DirPath(), a.Dir())
	}
	root, _ := protocol.ParseDocumentURI(`file:///C:/USERS/me/proj%20dir`)
	if !root.Encloses(a) || root.Encloses(root) || a.Encloses(root) {
		t.Error("Encloses")
	}
	// The JSON form goes through UnmarshalText.
	var v struct {
		URI protocol.DocumentURI `json:"uri"`
		M   map[protocol.DocumentURI]int
	}
	if err := json.Unmarshal([]byte(`{"uri":"file:///c%3A/Users/Me/x.gp","M":{"file:///d%3A/y.gp":1}}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.URI != "file:///C:/Users/Me/x.gp" || v.M["file:///D:/y.gp"] != 1 {
		t.Errorf("decoded %+v", v)
	}
}

// On a flavor that is case-sensitive nothing changes: Key is the URI, Path is the path.
func TestPosixKeyIsIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a Windows process has no Posix paths: filepath.Abs makes them drive paths")
	}
	t.Cleanup(fspath.Use(fspath.Posix))
	u := protocol.URIFromPath("/Home/Me/X.gp")
	if u.Key() != u || u.Path() != "/Home/Me/X.gp" {
		t.Errorf("%q %q", u.Key(), u.Path())
	}
}

func TestWindowsRejectsUserInfo(t *testing.T) {
	windows(t)
	for _, s := range []string{`file://user@srv/share/x.gp`, `file://user:pw@srv/share/x.gp`, `file://us%40er/share/x.gp`} {
		if u, err := protocol.ParseDocumentURI(s); err == nil {
			t.Errorf("ParseDocumentURI(%q) = %q, want an error (Path would panic on it)", s, u)
		}
	}
	// A URI made by hand, not by ParseDocumentURI, with a user name: Path panics (as for any invalid URI), it does not guess.
	defer func() {
		if recover() == nil {
			t.Error("Path of file://user@srv/share/x did not panic")
		}
	}()
	_ = protocol.DocumentURI(`file://user@srv/share/x`).Path()
}

func TestPathOfNonFileURI(t *testing.T) {
	for _, f := range []fspath.Flavor{fspath.Windows, fspath.Posix, fspath.CaseFold} {
		restore := fspath.Use(f)
		func() {
			defer restore()
			defer func() {
				if recover() == nil {
					t.Errorf("%+v: Path of https://x/y did not panic", f)
				}
			}()
			_ = protocol.DocumentURI(`https://x/y`).Path()
		}()
	}
}

func TestWindowsURIFromRelativePath(t *testing.T) {
	windows(t)
	got := protocol.URIFromPath(`rel\x.gp`)
	if !strings.HasPrefix(string(got), "file:///") || !strings.HasSuffix(string(got), "rel/x.gp") {
		t.Errorf("URIFromPath of a relative name = %q", got)
	}
}

// The answers of Path and Key are remembered, and forgotten when fspath is invalidated.
func TestWindowsPathMemo(t *testing.T) {
	windows(t, "/C:/P/Main.gp")
	u, _ := protocol.ParseDocumentURI(`file:///c:/p/main.gp`)
	if u.Path() != `C:\P\Main.gp` || u.Key() != "file:///C:/P/Main.gp" {
		t.Fatalf("%q %q", u.Path(), u.Key())
	}
	// The folder listing changes (the file is renamed in case) and the server says so.
	t.Cleanup(fspath.SetLister(func(dir string) ([]string, error) {
		if strings.EqualFold(strings.TrimRight(dir, `\`), `C:`) {
			return []string{"P"}, nil
		}
		return []string{"MAIN.GP"}, nil
	}))
	if u.Path() != `C:\P\MAIN.GP` || u.Key() != "file:///C:/P/MAIN.GP" {
		t.Errorf("after Invalidate: %q %q", u.Path(), u.Key())
	}
}
