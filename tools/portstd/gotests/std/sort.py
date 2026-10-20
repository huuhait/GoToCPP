# Go 1.25.14's tests of sort; see convert_std.py.
FILES = ['all']
expect(
    'TestSearchWrappersDontAlloc',
    'divergence',
    'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates differently '
    'than Go (it counts C++ operator new); counting rule of the Phase 09 README, 2.8 (the test is '
    'skipped when GOMAXPROCS > 1, as in Go)',
)
