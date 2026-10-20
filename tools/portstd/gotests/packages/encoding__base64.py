# Go 1.25.14's tests of encoding/base64 (the internal tests and the examples); see convert.py.
FILES = ['base64_test.go', 'example_test.go']
EXTRA = [('internal/byteorder', 'internal/byteorder')]
PORT_PATCHES = [('"internal/byteorder"', '"example.com/m/internal/byteorder"')] # internal/byteorder is a package of Go's tree that a program cannot import: ./internal/byteorder is a copy
