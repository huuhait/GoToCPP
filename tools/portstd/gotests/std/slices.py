# Go 1.25.14's tests of slices; see convert_std.py.
FILES = ['all']
for name in ['TestInsert', 'TestGrow', 'TestConcat']:
    expect(
        name,
        'divergence',
        'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates '
        'differently than Go (it counts C++ operator new; append(s, make(S, n)...) and '
        'string(bytes) allocate a temporary); counting rule of the Phase 09 README, 2.8',
    )
ZERO = (
    'bug: it makes a slice of zero-size elements (struct{}) with a length up to math.MaxInt, and '
    'gocpp slices cost one byte for each element (the C++ size of an empty struct is 1), so '
    'makeslice panics with len out of range; Go allocates nothing'
)
for name in ['TestConcat_too_large', 'TestRepeat']:
    expect(name, 'bug', ZERO)
