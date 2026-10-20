# Go 1.25.14's tests of hash/fnv; see convert.py.
FILES = ['fnv_test.go']
EXTRA = [('internal/testhash', 'testhash'), ('internal/byteorder', 'internal/byteorder')]
PORT_PATCHES = [('"internal/byteorder"', '"example.com/m/internal/byteorder"')] # internal/byteorder is a package of Go's tree that a program cannot import: ./internal/byteorder is a copy
patch('fnv_test.go', '\t"internal/testhash"\n', '\t"example.com/m/testhash"\n', 'internal/testhash is a package of Go\'s tree that a program cannot import: ./testhash is a copy')
patch('testhash/hash.go', '\tseed := time.Now().UnixNano()\n', '\tseed := int64(0x5EED) // a fixed seed: the time makes the log lines of -v differ from run to run\n', 'the seed is the time, which is printed with t.Logf: the output of go test -v and gocpp test can only be compared with a fixed one')
patch('testhash/hash.go', '\t"time"\n', '', 'time is no longer used')
