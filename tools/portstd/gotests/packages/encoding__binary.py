# Go 1.25.14's tests of encoding/binary; see convert.py.
FILES = ['binary_test.go', 'varint_test.go', 'example_test.go']
SHARED = ['asan']
patch('binary_test.go', '\t"internal/asan"\n', '\t"example.com/m/asan"\n', 'internal/asan is a package of Go\'s tree that a program cannot import: ./asan has the constant Enabled (false)')
drop('binary_test.go', 'TestAppendAllocs', 'counts allocations with testing.AllocsPerRun, and gocpp allocates differently than Go (counting rule of the Phase 09 README, 2.8)')
drop('binary_test.go', 'TestSizeAllocs', 'counts allocations with testing.AllocsPerRun (counting rule of the Phase 09 README, 2.8)')
