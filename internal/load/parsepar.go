package load

import (
	"go/ast"
	"go/token"
	"runtime"
	"sync"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// parallelParseBytes is the least total size of the files of a package that
// are worth parsing on several goroutines (a goroutine costs a few
// microseconds, a file of 100 lines parses in about 100).
const parallelParseBytes = 6 << 10

// parseFiles parses the sources srcs, named names, into fset, and returns
// the syntax trees and the parse errors, in the order of the sources. It gives
// the same trees as calling parser.ParseFile on each in turn, positions
// included: the files go into fset in that order with the bases that the
// sequence gives them, however the goroutines are scheduled. Each file is
// parsed into a file set of its own whose next base is the one the file
// gets, and then the file is added to fset (token.FileSet.AddExistingFiles).
func parseFiles(fset *token.FileSet, names []string, srcs [][]byte, mode parser.Mode) ([]*ast.File, []error) {
	return parseFilesWith(fset, names, srcs, mode, runtime.GOMAXPROCS(0), parallelParseBytes)
}

// parseFilesWith is parseFiles with at most workers goroutines, and the
// parallel parse only from minBytes of source on.
func parseFilesWith(fset *token.FileSet, names []string, srcs [][]byte, mode parser.Mode, workers, minBytes int) ([]*ast.File, []error) {
	n := len(srcs)
	files := make([]*ast.File, n)
	errs := make([]error, n)
	total := 0
	for _, s := range srcs {
		total += len(s)
	}
	workers = min(n, workers)
	if workers < 2 || total < minBytes {
		for i := range srcs {
			files[i], errs[i] = parser.ParseFile(fset, names[i], srcs[i], mode)
		}
		return files, errs
	}
	// The base of each file: token.FileSet.AddFile gives the next free base,
	// and the one after a file of size n is base+n+1.
	bases := make([]int, n)
	next := fset.Base()
	for i, s := range srcs {
		bases[i] = next
		next += len(s) + 1
	}
	tokenFiles := make([]*token.File, n)
	var wg sync.WaitGroup
	jobs := make(chan int, n)
	for i := range n {
		jobs <- i
	}
	close(jobs)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				private := token.NewFileSet()
				if bases[i] > 1 {
					// Fills the base space before this file, so that the file is
					// added at its base (a file of size s takes s+1 bases).
					private.AddFile("", 1, bases[i]-2)
				}
				files[i], errs[i] = parser.ParseFile(private, names[i], srcs[i], mode)
				tokenFiles[i] = private.File(files[i].FileStart)
			}
		}()
	}
	wg.Wait()
	fset.AddExistingFiles(tokenFiles...)
	return files, errs
}
