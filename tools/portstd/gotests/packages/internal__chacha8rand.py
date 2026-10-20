# Go 1.25.14's tests of internal/chacha8rand (package chacha8rand_test, which dot-imports the package: here the copy in the module root, so the
# export_test.go of package chacha8rand works); see convert.py.
FILES = ['export_test.go', 'rand_test.go']
EXTRA = [('internal/byteorder', 'internal/byteorder')]
PORT_PATCHES = [('"internal/byteorder"', '"example.com/m/internal/byteorder"')] # internal/byteorder is a package of Go's tree that a program cannot import: ./internal/byteorder is a copy
patch('rand_test.go', '\t. "internal/chacha8rand"\n', '\t. "example.com/m"\n', 'the package under test is the copy in the module root')
SHARED = ['internal/goarch']
PORT_PATCHES.append(('"internal/goarch"', '"example.com/m/internal/goarch"'))
