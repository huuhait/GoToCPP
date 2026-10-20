# Go 1.25.14's tests of math/rand/v2 (package rand_test, which imports the package: here the copy in the module root, example.com/m), and the
# examples; see convert.py.
FILES = ['auto_test.go', 'example_test.go', 'export_test.go', 'pcg_test.go', 'race_test.go', 'rand_test.go']
SHARED = ['testenv', 'internal/goarch']
EXTRA_PORTS = [('internal/chacha8rand', 'internal/chacha8rand')]
EXTRA = [('internal/byteorder', 'internal/byteorder')]
# Not taken: chacha8_test.go (its test of the stream hashes the output with crypto/sha256, which gocpp does not have; the stream is checked by
# the output of rand_basic.gp, TestChaCha8 of the internal/chacha8rand fixture and regress_test.go).
for f in ['auto_test.go', 'pcg_test.go', 'race_test.go', 'rand_test.go']:
    patch(f, '\t. "math/rand/v2"\n', '\t. "example.com/m"\n', 'the package under test is the copy in the module root')
patch('example_test.go', '\t"math/rand/v2"\n', '\t"example.com/m"\n', 'the package under test is the copy in the module root')
patch('rand_test.go', '\t"internal/testenv"\n', '\t"example.com/m/testenv"\n', 'internal/testenv is a package of Go\'s tree that a program cannot import: ./testenv has Builder')
PORT_PATCHES = [('//gocpp:native gp::runtime::runtime_rand\nfunc runtime_rand() uint64\n', ''), ('"internal/byteorder"', '"example.com/m/internal/byteorder"'), ('"internal/chacha8rand"', '"example.com/m/internal/chacha8rand"'), ('"internal/goarch"', '"example.com/m/internal/goarch"')]
# Not taken: regress_test.go (TestRegress calls every method of a Rand through reflect.Value.Method and Call, which are not in the reflect subset, D6; rand_basic.gp prints the same methods with Go's results).
