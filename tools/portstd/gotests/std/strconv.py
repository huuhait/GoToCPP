# Go 1.25.14's tests of strconv; see convert_std.py.
FILES = ['all']
TESTDATA = True
ALLOC = (
    'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates differently '
    'than Go (it counts C++ operator new, and string(bytes) copies where Go does not allocate); '
    'counting rule of the Phase 09 README, 2.8'
)
for name in ['TestAllocationsFromBytes', 'TestCountMallocs']:
    expect(name, 'divergence', ALLOC)
