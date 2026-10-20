# Go 1.25.14's tests of math/rand (package rand_test, which imports the package: here the copy in the module root, example.com/m, so the
# export_test.go of package rand works), and the examples; see convert.py.
FILES = ['auto_test.go', 'example_test.go', 'export_test.go', 'race_test.go', 'rand_test.go']
SHARED = ['testenv', 'iotest', 'internal/godebug']
PORT_PATCHES = [('//gocpp:native gp::runtime::runtime_rand\nfunc runtime_rand() uint64\n', ''), ('"internal/godebug"', '"example.com/m/internal/godebug"')] # internal/godebug is a package of Go\'s tree that a program cannot import: ./internal/godebug has a setting that is always empty
# Not taken: default_test.go (TestDefaultRace starts the test binary again with os/exec and an environment variable).
for f in ['auto_test.go', 'race_test.go', 'rand_test.go']:
    patch(f, '\t. "math/rand"\n', '\t. "example.com/m"\n', 'the package under test is the copy in the module root')
patch('example_test.go', '\t"math/rand"\n', '\t"example.com/m"\n', 'the package under test is the copy in the module root')
patch('rand_test.go', '\t"internal/testenv"\n', '\t"example.com/m/testenv"\n', 'internal/testenv is a package of Go\'s tree that a program cannot import: ./testenv has Builder')
patch('rand_test.go', '\t"testing/iotest"\n', '\t"example.com/m/iotest"\n', 'testing/iotest is not in gocpp: ./iotest has OneByteReader')
# Not taken: regress_test.go (TestRegress calls every method of a Rand through reflect.Value.Method and Call, which are not in the reflect subset, D6; rand_basic.gp prints the same methods with Go's results).
