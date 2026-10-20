# Go 1.25.14's tests of strings: the internal tests, package strings_test and the examples; see
# convert_std.py.
FILES = [
    'export_test.go',
    'builder_test.go',
    'clone_test.go',
    'compare_test.go',
    'example_test.go',
    'iter_test.go',
    'reader_test.go',
    'replace_test.go',
    'search_test.go',
    'strings_test.go',
]
SHIMS = ['internal/testenv', 'internal/asan']

ALLOC = (
    'divergence: it counts allocations with testing.AllocsPerRun or testing.AllocsPerRun-based '
    'expectations, and gocpp allocates differently than Go (counting rule of the Phase 09 README, '
    '2.8)'
)
for name in [
    'TestBuilderGrow',
    'TestBuilderAllocs',
    'TestBuilderGrowSizeclasses',
    'TestIndexRune',
    'TestReplace',
]:
    expect(name, 'divergence', ALLOC)
