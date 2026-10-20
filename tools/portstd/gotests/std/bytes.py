# Go 1.25.14's tests of bytes: the internal tests, package bytes_test and the examples; see
# convert_std.py.
FILES = ['all']
skip_file(
    'boundary_test.go',
    'it maps pages with syscall.Mmap and Mprotect to put the data next to an inaccessible page, '
    'and the syscall subset of gocpp has neither (step 9.6)',
)

ALLOC = (
    'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates differently '
    'than Go (it counts C++ operator new: bytes <-> string conversions copy); counting rule of the '
    'Phase 09 README, 2.8'
)
for name in [
    'TestNewBufferShallow',
    'TestWriteAppend',
    'TestEqual',
    'TestIndex',
    'TestLastIndex',
    'TestIndexRune',
    'TestReplace',
]:
    expect(name, 'divergence', ALLOC)
