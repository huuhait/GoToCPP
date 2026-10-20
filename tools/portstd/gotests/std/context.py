# Go 1.25.14's tests of context; see convert_std.py.
FILES = ['all']
drop(
    'example_test.go',
    'ExampleAfterFunc_connection',
    'it reads from a net.Conn, and gocpp does not have package net (tier 3)',
)
skip_file(
    'net_test.go',
    'its only test, TestDeadlineExceededIsNetError, asserts that context.DeadlineExceeded '
    'implements net.Error, and gocpp does not have package net (tier 3)',
)
expect(
    'TestAllocs',
    'divergence',
    'divergence: it counts allocations with testing.AllocsPerRun against exact numbers, and gocpp '
    'allocates differently than Go (it counts C++ operator new); counting rule of the Phase 09 '
    'README, 2.8',
)
expect(
    'TestCauseRace',
    'flaky',
    "flaky: Go 1.25.14's Cause reads the cause under the lock and then calls c.Err() when it is "
    'nil, so a cancel that lands between the two makes it return context.Canceled instead of the '
    "cause; the window is a few instructions in Go and wider in gocpp's unoptimized build (about 1 "
    'run in 300 fails); it can pass or fail',
)
