# Go 1.25.14's tests of bufio; see convert_std.py.
FILES = ['all']
skip_file(
    'net_test.go',
    'its only test, TestCopyUnixpacket, copies across a unixpacket socket of package net, which '
    'gocpp does not have (tier 3)',
)
expect(
    'TestReadStringAllocs',
    'divergence',
    'divergence: it counts allocations with testing.AllocsPerRun, and gocpp allocates differently '
    'than Go (it counts C++ operator new); counting rule of the Phase 09 README, 2.8',
)
