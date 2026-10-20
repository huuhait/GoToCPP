# Go 1.25.14's tests of unicode/utf8; see convert_std.py.
FILES = ['all']
expect(
    'TestRuneCountNonASCIIAllocation',
    'divergence',
    'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates differently '
    'than Go (it counts C++ operator new); counting rule of the Phase 09 README, 2.8',
)
