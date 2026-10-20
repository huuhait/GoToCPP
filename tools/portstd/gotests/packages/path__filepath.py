# Go 1.25.14's tests of path/filepath (package filepath_test, which imports the package: here the copy in the module root, example.com/m,
# so the export_test.go of package filepath works), and the examples; see convert.py.
FILES = ['export_test.go', 'match_test.go', 'path_test.go', 'example_test.go', 'example_unix_test.go', 'example_unix_walk_test.go']
SHARED = ['testenv']
patch('match_test.go', '\t. "path/filepath"\n', '\t. "example.com/m"\n', 'the package under test is the copy in the module root')
patch('match_test.go', '\t"internal/testenv"\n', '\t"example.com/m/testenv"\n', 'internal/testenv is a package of Go\'s tree that a program cannot import: ./testenv has Builder, MustHaveSymlink and SkipFlaky')
patch('path_test.go', '\t"path/filepath"\n', '\tfilepath "example.com/m"\n', 'the package under test is the copy in the module root')
patch('path_test.go', '\t"internal/testenv"\n', '\t"example.com/m/testenv"\n', 'internal/testenv is a package of Go\'s tree that a program cannot import: ./testenv has Builder, MustHaveSymlink and SkipFlaky')
for f in ['example_test.go', 'example_unix_test.go', 'example_unix_walk_test.go']:
    patch(f, '\t"path/filepath"\n', '\tfilepath "example.com/m"\n', 'the package under test is the copy in the module root')
drop('path_test.go', 'TestBug3486', 'it walks $GOROOT/src/unicode, and a gocpp program has no GOROOT')
# Not taken: path_windows_test.go and export_windows_test.go (Windows; step 14 runs them).
EXTRA_PORTS = [('internal/filepathlite', 'internal/filepathlite')]
PORT_PATCHES = [('//\n//gocpp:native gp::runtime::stack_room\nfunc stackRoom() int64\n', ''), ('|| stackRoom() < 0 {', '|| depth >= 3000 { // a program cannot declare the native: the copy stops at a depth that every build holds'), ('"internal/bytealg"', '"example.com/m/internal/bytealg"'), ('"internal/filepathlite"', '"example.com/m/internal/filepathlite"'), ('"internal/stringslite"', '"example.com/m/internal/stringslite"')] # Go's internal packages cannot be imported by a program: ./internal/filepathlite is a copy, bytealg and stringslite are small ones in Go
drop('match_test.go', 'TestGlob', 'it globs the source files of the package by name (match.go, ../filepath/match.go), and in the module they are .gp files in a folder with another name')
drop('match_test.go', 'TestNonWindowsGlobEscape', 'it globs the source file match.go by name (here match.gp)')
