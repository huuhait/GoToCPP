# Go 1.25.14's tests of io; see convert_std.py.
FILES = ['all']
drop(
    'multi_test.go',
    'TestMultiReaderFreesExhaustedReaders',
    'unsupported: it needs runtime.AddCleanup, which gocpp does not have yet (the garbage '
    'collector of Go is reference counting here, ADR-004)',
)
