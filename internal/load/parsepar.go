package load

import (
	"go/ast"
	"go/token"
	"runtime"
	"sync"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// parallelParseBytes is the least total size of the files of a package that
// are worth parsing on several goroutines (a goroutine costs a few
// microseconds, a file of 100 lines parses in about 100).
const parallelParseBytes = 6 << 10

type parseFileFunc func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)

// parseFiles parses captured sources into the loader's file set. Results and
// positions have source order, regardless of worker scheduling. File-system
// hooks and later package processing remain on the caller's goroutine.
func (l *loader) parseFiles(
	names []string, srcs [][]byte, mode parser.Mode,
) ([]*ast.File, []error) {
	return parseFilesUsing(l.fset, names, srcs, mode,
		runtime.GOMAXPROCS(0), parallelParseBytes, l.parseFile)
}

// parseFilesWith is the parser helper with explicit parallelism and threshold,
// for tests comparing the sequential and parallel paths.
func parseFilesWith(
	fset *token.FileSet, names []string, srcs [][]byte, mode parser.Mode, workers, minBytes int,
) ([]*ast.File, []error) {
	return parseFilesUsing(fset, names, srcs, mode, workers, minBytes, parser.ParseFile)
}

func parseFilesUsing(
	fset *token.FileSet, names []string, srcs [][]byte, mode parser.Mode,
	workers, minBytes int, parseFile parseFileFunc,
) ([]*ast.File, []error) {
	count := len(srcs)
	files := make([]*ast.File, count)
	errs := make([]error, count)
	total := 0
	for _, src := range srcs {
		total += len(src)
	}
	workers = min(count, workers)
	if workers < 2 || total < minBytes {
		for i := range srcs {
			files[i], errs[i] = parseFile(fset, names[i], srcs[i], mode)
		}
		return files, errs
	}

	// Reserve the complete position range atomically. A caller sharing fset
	// may add other files while these workers parse; they must not overlap.
	reservation := fset.AddFile("", -1, total+count-1)
	bases := make([]int, count)
	nextBase := reservation.Base()
	for i, src := range srcs {
		bases[i] = nextBase
		nextBase += len(src) + 1
	}
	tokenFiles := make([]*token.File, count)
	panics := make([]any, count)
	var wg sync.WaitGroup
	jobs := make(chan int, count)
	for i := range count {
		jobs <- i
	}
	close(jobs)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				func() {
					// A parser panic belongs to the caller's recovery boundary,
					// not an unprotected worker that would end the server.
					defer func() { panics[i] = recover() }()
					private := token.NewFileSet()
					if bases[i] > 1 {
						private.AddFile("", 1, bases[i]-2)
					}
					files[i], errs[i] = parseFile(private, names[i], srcs[i], mode)
					tokenFiles[i] = private.File(files[i].FileStart)
				}()
			}
		}()
	}
	wg.Wait()

	// Remove the reservation before publishing. A failing batch rethrows on
	// the caller after every worker has joined; no partial ASTs are returned.
	fset.RemoveFile(reservation)
	for _, value := range panics {
		if value != nil {
			panic(value)
		}
	}
	fset.AddExistingFiles(tokenFiles...)
	return files, errs
}

// forEachFile calls fn for each file of a package, one after the other, or on several
// goroutines when the files are big enough to pay for them (parallelParseBytes of source,
// as for parsing). fn works on one file's tree and reports its errors to the list it is
// given; the lists are added to errs afterwards in the order of the files, so the diagnostics
// come in the order that a loop over the files gives them. A panic in fn is rethrown on the
// caller once every worker has joined, as for the parser.
func forEachFile(files []*File, errs *diag.List, fn func(i int, f *File, errs *diag.List)) {
	forEachFileWith(files, errs, runtime.GOMAXPROCS(0), parallelParseBytes, fn)
}

// forEachFileWith is forEachFile with the parallelism and the threshold given, for tests that
// compare the serial and the parallel runs.
func forEachFileWith(
	files []*File, errs *diag.List, workers, minBytes int, fn func(i int, f *File, errs *diag.List),
) {
	total := 0
	for _, f := range files {
		if f.AST != nil {
			total += int(f.AST.FileEnd - f.AST.FileStart)
		}
	}
	workers = min(len(files), workers)
	if workers < 2 || total < minBytes {
		for i, f := range files {
			fn(i, f, errs)
		}
		return
	}

	lists := make([]diag.List, len(files))
	panics := make([]any, len(files))
	jobs := make(chan int, len(files))
	for i := range files {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				func() {
					defer func() { panics[i] = recover() }()
					fn(i, files[i], &lists[i])
				}()
			}
		}()
	}
	wg.Wait()
	for _, value := range panics {
		if value != nil {
			panic(value)
		}
	}
	for i := range lists {
		errs.AddAll(&lists[i])
	}
}
