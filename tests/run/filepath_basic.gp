// phase: 09

//go:build !windows

// path/filepath on Linux: the lexical functions, Match, Glob, Walk, WalkDir, Rel, EvalSymlinks, Abs on a temporary tree.

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	for _, p := range []string{"", ".", "..", "/", "//", "a", "a/", "a//b", "a/./b", "a/../b", "/a/../..", "../../a", "a/b/../../..", "/abs/./path//x/", "a/b/c.txt", "a.b/c", ".hidden", "x/.hidden.go"} {
		d, f := filepath.Split(p)
		fmt.Printf("%-16q clean=%-10q base=%-9q dir=%-10q ext=%-5q split=%q,%q abs=%v local=%v\n", p, filepath.Clean(p), filepath.Base(p), filepath.Dir(p), filepath.Ext(p), d, f, filepath.IsAbs(p), filepath.IsLocal(p))
	}
	fmt.Println(filepath.Join(), filepath.Join(""), filepath.Join("a"), filepath.Join("a", ""), filepath.Join("", "b"), filepath.Join("a", "../b", "c"), filepath.Join("/", "x"), filepath.Join("a/", "/b"))
	fmt.Println(filepath.SplitList("/a:/b::/c"), len(filepath.SplitList("")), filepath.ToSlash("a/b"), filepath.FromSlash("a/b"), filepath.VolumeName("/a"), string(filepath.Separator), string(filepath.ListSeparator))
	for _, c := range [][2]string{{"/a/b", "/a/b/c/d"}, {"/a/b", "/a"}, {"/a", "/b/c"}, {"a", "b"}, {"a/b", "a/b"}, {"/a", "b"}, {"a/b/c", "a/d"}, {"..", "a"}, {"a", ".."}} {
		r, err := filepath.Rel(c[0], c[1])
		fmt.Printf("Rel(%q, %q) = %q %v\n", c[0], c[1], r, err)
	}
	for _, c := range [][2]string{{"*.go", "main.go"}, {"*.go", "dir/main.go"}, {"a/*/c", "a/b/c"}, {"a?c", "abc"}, {"a?c", "a/c"}, {"[a-c]x", "bx"}, {"[^a-c]x", "bx"}, {"[]a]", "]"}, {"\\*", "*"}, {"a[", "a"}, {"[z-a]", "m"}, {"*", ""}, {"**", "ab"}, {"[a-", "a"}} {
		m, err := filepath.Match(c[0], c[1])
		fmt.Printf("Match(%q, %q) = %v %v\n", c[0], c[1], m, err)
	}

	root, err := os.MkdirTemp("", "fp")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	mk := func(p string, data string) {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			panic(err)
		}
	}
	mk("a.txt", "A")
	mk("b.go", "B")
	mk("sub/c.txt", "C")
	mk("sub/deep/d.txt", "D")
	mk("sub/deep/e.go", "E")
	mk("empty/.keep", "")
	if err := os.Symlink("sub/deep", filepath.Join(root, "link")); err != nil {
		panic(err)
	}
	rel := func(p string) string {
		r, _ := filepath.Rel(root, p)
		return r
	}

	var walked []string
	err = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		s := rel(path)
		if info.IsDir() {
			s += "/"
		}
		walked = append(walked, s)
		return nil
	})
	fmt.Println("Walk:", strings.Join(walked, " "), err)

	walked = nil
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "deep" {
			return filepath.SkipDir
		}
		s := rel(path)
		if d.Type()&fs.ModeSymlink != 0 {
			s += "@"
		}
		walked = append(walked, s)
		return nil
	})
	fmt.Println("WalkDir:", strings.Join(walked, " "), err)

	walked = nil
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d.Name() == "c.txt" {
			return filepath.SkipAll
		}
		walked = append(walked, rel(path))
		return nil
	})
	fmt.Println("SkipAll:", strings.Join(walked, " "), err)

	err = filepath.Walk(filepath.Join(root, "missing"), func(path string, info fs.FileInfo, err error) error {
		fmt.Println("walk missing:", rel(path), info == nil, errors.Is(err, fs.ErrNotExist))
		return err
	})
	fmt.Println(errors.Is(err, fs.ErrNotExist))

	for _, pat := range []string{"*.txt", "*.go", "sub/*", "sub/*/*.txt", "*/", "[ab].*", "nomatch*", "sub/[", "link/*.go"} {
		ms, err := filepath.Glob(filepath.Join(root, pat))
		var out []string
		for _, m := range ms {
			out = append(out, rel(m))
		}
		fmt.Printf("Glob(%q) = %v %v\n", pat, out, err)
	}

	ev, err := filepath.EvalSymlinks(filepath.Join(root, "link", "d.txt"))
	resolved, _ := filepath.EvalSymlinks(root)
	r2, _ := filepath.Rel(resolved, ev)
	fmt.Println("EvalSymlinks:", r2, err)
	_, err = filepath.EvalSymlinks(filepath.Join(root, "nothing"))
	fmt.Println(errors.Is(err, fs.ErrNotExist))

	wd, _ := os.Getwd()
	abs, err := filepath.Abs("x/../y.txt")
	fmt.Println(abs == filepath.Join(wd, "y.txt"), err)
	os.Chdir(root)
	a2, _ := filepath.Abs("sub")
	r3, _ := filepath.EvalSymlinks(a2)
	r4, _ := filepath.EvalSymlinks(filepath.Join(root, "sub"))
	fmt.Println(r3 == r4)
	loc, err := filepath.Localize("a/b")
	fmt.Println(loc, err)
	_, err = filepath.Localize("../x")
	fmt.Println(err)
}

// Output:
// ""               clean="."        base="."       dir="."        ext=""    split="","" abs=false local=false
// "."              clean="."        base="."       dir="."        ext="."   split="","." abs=false local=true
// ".."             clean=".."       base=".."      dir="."        ext="."   split="",".." abs=false local=false
// "/"              clean="/"        base="/"       dir="/"        ext=""    split="/","" abs=true local=false
// "//"             clean="/"        base="/"       dir="/"        ext=""    split="//","" abs=true local=false
// "a"              clean="a"        base="a"       dir="."        ext=""    split="","a" abs=false local=true
// "a/"             clean="a"        base="a"       dir="a"        ext=""    split="a/","" abs=false local=true
// "a//b"           clean="a/b"      base="b"       dir="a"        ext=""    split="a//","b" abs=false local=true
// "a/./b"          clean="a/b"      base="b"       dir="a"        ext=""    split="a/./","b" abs=false local=true
// "a/../b"         clean="b"        base="b"       dir="."        ext=""    split="a/../","b" abs=false local=true
// "/a/../.."       clean="/"        base=".."      dir="/"        ext="."   split="/a/../",".." abs=true local=false
// "../../a"        clean="../../a"  base="a"       dir="../.."    ext=""    split="../../","a" abs=false local=false
// "a/b/../../.."   clean=".."       base=".."      dir="."        ext="."   split="a/b/../../",".." abs=false local=false
// "/abs/./path//x/" clean="/abs/path/x" base="x"       dir="/abs/path/x" ext=""    split="/abs/./path//x/","" abs=true local=false
// "a/b/c.txt"      clean="a/b/c.txt" base="c.txt"   dir="a/b"      ext=".txt" split="a/b/","c.txt" abs=false local=true
// "a.b/c"          clean="a.b/c"    base="c"       dir="a.b"      ext=""    split="a.b/","c" abs=false local=true
// ".hidden"        clean=".hidden"  base=".hidden" dir="."        ext=".hidden" split="",".hidden" abs=false local=true
// "x/.hidden.go"   clean="x/.hidden.go" base=".hidden.go" dir="x"        ext=".go" split="x/",".hidden.go" abs=false local=true
//   a a b b/c /x a/b
// [/a /b  /c] 0 a/b a/b  / :
// Rel("/a/b", "/a/b/c/d") = "c/d" <nil>
// Rel("/a/b", "/a") = ".." <nil>
// Rel("/a", "/b/c") = "../b/c" <nil>
// Rel("a", "b") = "../b" <nil>
// Rel("a/b", "a/b") = "." <nil>
// Rel("/a", "b") = "" Rel: can't make b relative to /a
// Rel("a/b/c", "a/d") = "../../d" <nil>
// Rel("..", "a") = "" Rel: can't make a relative to ..
// Rel("a", "..") = "../.." <nil>
// Match("*.go", "main.go") = true <nil>
// Match("*.go", "dir/main.go") = false <nil>
// Match("a/*/c", "a/b/c") = true <nil>
// Match("a?c", "abc") = true <nil>
// Match("a?c", "a/c") = false <nil>
// Match("[a-c]x", "bx") = true <nil>
// Match("[^a-c]x", "bx") = false <nil>
// Match("[]a]", "]") = false syntax error in pattern
// Match("\\*", "*") = true <nil>
// Match("a[", "a") = false syntax error in pattern
// Match("[z-a]", "m") = false <nil>
// Match("*", "") = true <nil>
// Match("**", "ab") = true <nil>
// Match("[a-", "a") = false syntax error in pattern
// Walk: ./ a.txt b.go empty/ empty/.keep link sub/ sub/c.txt sub/deep/ sub/deep/d.txt sub/deep/e.go <nil>
// WalkDir: . a.txt b.go empty empty/.keep link@ sub sub/c.txt <nil>
// SkipAll: . a.txt b.go empty empty/.keep link sub <nil>
// walk missing: missing true true
// true
// Glob("*.txt") = [a.txt] <nil>
// Glob("*.go") = [b.go] <nil>
// Glob("sub/*") = [sub/c.txt sub/deep] <nil>
// Glob("sub/*/*.txt") = [sub/deep/d.txt] <nil>
// Glob("*/") = [a.txt b.go empty link sub] <nil>
// Glob("[ab].*") = [a.txt b.go] <nil>
// Glob("nomatch*") = [] <nil>
// Glob("sub/[") = [] syntax error in pattern
// Glob("link/*.go") = [link/e.go] <nil>
// EvalSymlinks: sub/deep/d.txt <nil>
// true
// true <nil>
// true
// a/b <nil>
// invalid path
