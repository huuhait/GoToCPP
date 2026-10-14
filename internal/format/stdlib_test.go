package format

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// TestStdlib formats every Go file of the Go standard library with Source:
// all .go files in $GOROOT/src outside testdata folders, as in the
// round-trip test of internal/syntax. The result must be what the real
// gofmt prints for each file. gofmt changes only a few of them (4 in Go
// 1.25.14, in spacing), so "the file stays the same" is not the right
// test. The test does it the fast way: gofmt -l lists the files that gofmt
// changes (and the errors). Every other file must come out of Source
// byte for byte as it went in. For a file in the list, Source must print
// what gofmt prints for it, and for a file with errors (there are none in
// Go 1.25.14), Source must give the same errors.
//
// gofmt -l runs while Source formats the files, both in parallel: the
// test takes about 1.2 seconds on 16 cores and 10 on one.
func TestStdlib(t *testing.T) {
	gofmt := realGofmt(t)
	if gofmt == "" {
		t.Skip("there is no real gofmt in $GOROOT/bin")
	}
	src := filepath.Join(goroot(t), "src")
	files := stdlibFiles(t, src)
	// A wrong folder or a broken walk would make this test pass without
	// checking anything. Go 1.25 has 6240 such files.
	if len(files) < 1000 {
		t.Fatalf("found only %d Go files in %s; the Go standard library has thousands", len(files), src)
	}

	// gofmt -l over all of src also lists the testdata files, and prints
	// the syntax errors of some of them: it exits with status 2.
	type listing struct {
		stdout, stderr []byte
		code           int
		err            error
	}
	listed := make(chan listing, 1)
	go func() {
		var l listing
		l.stdout, l.stderr, l.code, l.err = gofmtRun(gofmt, "-l", src)
		listed <- l
	}()
	results := formatFiles(files)
	l := <-listed
	if l.err != nil || l.code != 0 && l.code != 2 {
		t.Fatalf("gofmt -l %s: %v, exit status %d\n%s", src, l.err, l.code, l.stderr)
	}

	inTest := make(map[string]bool, len(files))
	for _, f := range files {
		inTest[f] = true
	}
	changed := make(map[string]bool)
	for line := range strings.Lines(string(l.stdout)) {
		if name := strings.TrimSuffix(line, "\n"); inTest[name] {
			changed[name] = true
		}
	}
	gofmtErrors := make(map[string]string) // the errors of each file, as gofmt prints them
	for line := range strings.Lines(string(l.stderr)) {
		// An error is "file.go:line:col: message".
		name, _, ok := strings.Cut(line, ".go:")
		name += ".go"
		switch {
		case ok && inTest[name]:
			gofmtErrors[name] += line
		case ok && strings.Contains(filepath.ToSlash(name), "/testdata/"):
		default:
			t.Fatalf("gofmt -l %s printed something that is not an error in a testdata file:\n%s", src, line)
		}
	}

	var failures int
	fail := func(format string, args ...any) {
		failures++
		if failures <= 10 {
			t.Errorf(format, args...)
		}
	}
	for i, name := range files {
		r := &results[i]
		switch {
		case gofmtErrors[name] != "":
			if text := errorText(r.err); text != gofmtErrors[name] {
				fail("%s: Source gives the errors\n%swant gofmt's:\n%s", name, text, gofmtErrors[name])
			}
		case r.err != nil:
			fail("%s: Source: %v", name, r.err)
		case changed[name]:
			want, errOut, code := runGofmt(t, gofmt, name)
			if code != 0 {
				fail("gofmt %s: exit status %d\n%s", name, code, errOut)
			} else if !bytes.Equal(r.out, want) {
				fail("%s: Source does not print what gofmt prints:\n%s", name, showDiff(want, r.out))
			}
		case r.out != nil:
			fail("%s: Source changes the file, gofmt does not:\n%s", name, showDiff(readFile(t, name), r.out))
		}
	}
	if failures > 10 {
		t.Errorf("too many failures: %d in all, the first 10 are shown", failures)
	}
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range changed {
			if !yield(strings.TrimPrefix(filepath.ToSlash(name), filepath.ToSlash(src)+"/")) {
				return
			}
		}
	})
	t.Logf("%s: formatted %d files; gofmt changes %d of them, and Source makes the same changes: %s",
		src, len(files), len(changed), strings.Join(names, ", "))
}

// stdlibFiles returns the .go files under src, outside testdata folders,
// in the order of filepath.WalkDir. The names start with src.
func stdlibFiles(t *testing.T, src string) []string {
	var files []string
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() && strings.HasSuffix(d.Name(), ".go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A formatResult is what Source does with one file. out is nil when the
// result is the same as the file, so the test doesn't keep a copy of the
// whole standard library.
type formatResult struct {
	out []byte
	err error
}

// formatFiles runs Source on the files, in parallel.
func formatFiles(files []string) []formatResult {
	results := make([]formatResult, len(files))
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range next {
				r := &results[i]
				data, err := ReadFile(files[i], nil)
				if err != nil {
					r.err = err
					continue
				}
				out, err := Source(files[i], data)
				if err != nil {
					r.err = err
				} else if !bytes.Equal(out, data) {
					r.out = out
				}
			}
		})
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()
	return results
}
