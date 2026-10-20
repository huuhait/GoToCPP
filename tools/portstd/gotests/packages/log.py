# Go 1.25.14's tests of log (the internal tests and the examples); see convert.py.
FILES = ['log_test.go']
# Not taken: example_test.go (its two examples print the file name, example_test.go for go test and example_test.gp for gocpp; log_basic.gp checks the flags).
SHARED = ['testenv', 'exec']
# The patches keep the number of lines of log_test.go: the tests match the output against the line numbers of two calls in the file (Rline).
patch('log_test.go', '\tRlongfile     = `.*/[A-Za-z0-9_\\-]+\\.go:` + Rline\n', '\tRlongfile     = `.*/[A-Za-z0-9_\\-]+\\.g[op]:` + Rline\n', 'the file is log_test.gp in gocpp and log_test.go for go test (the harness renames the files)')
patch('log_test.go', '\tRshortfile    = `[A-Za-z0-9_\\-]+\\.go:` + Rline\n', '\tRshortfile    = `[A-Za-z0-9_\\-]+\\.g[op]:` + Rline\n', 'the file is log_test.gp in gocpp and log_test.go for go test (the harness renames the files)')
drop('log_test.go', 'TestCallDepth', 'it starts the test binary again with os/exec to read what Fatal and Panic print, and gocpp has no os/exec')
for imp in ['bufio', 'errors', 'internal/testenv', 'os/exec']:
    patch('log_test.go', '\t"%s"\n' % imp, '\t_ "%s"\n' % ('example.com/m/testenv' if imp == 'internal/testenv' else ('example.com/m/exec' if imp == 'os/exec' else imp)), 'only TestCallDepth used it (dropped); a blank import keeps the number of lines')
EXTRA_PORTS = [('log/internal', 'internal/log')]
PORT_PATCHES = [('"log/internal"', '"example.com/m/internal/log"')] # log/internal is internal to log, and the module root is not under log/: ./internal/log is a copy
drop('log_test.go', 'TestDiscard', 'it counts allocations with testing.AllocsPerRun, and gocpp allocates differently than Go (counting rule of the Phase 09 README, 2.8)')
