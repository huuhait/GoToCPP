# Go 1.25.14's tests of io/fs; see convert_std.py.
FILES = ['all']
# The tests glob the source file of the package by name: here the folder holds the test files, which
# are .gp files.
patch(
    'glob_test.go',
    '"glob.go", "glob.go"}',
    '"glob_test.gp", "glob_test.gp"}',
    "the folder of the test has glob_test.gp (the test files of gocpp are .gp files), not Go's "
    'glob.go',
)
patch(
    'glob_test.go',
    '"gl?b.go", "glob.go"}',
    '"gl?b_test.gp", "glob_test.gp"}',
    'the folder of the test has glob_test.gp, not glob.go',
)
patch(
    'glob_test.go',
    '`gl\\ob.go`, "glob.go"}',
    '`gl\\ob_test.gp`, "glob_test.gp"}',
    'the folder of the test has glob_test.gp, not glob.go',
)
patch(
    'glob_test.go',
    '"*", "glob.go"}',
    '"*", "glob_test.gp"}',
    'the folder of the test has glob_test.gp, not glob.go',
)
patch(
    'glob_test.go',
    '"*/glob.go", "fs/glob.go"}',
    '"*/glob_test.gp", "fs/glob_test.gp"}',
    'the folder of the test has glob_test.gp, not glob.go',
)
expect(
    'TestCVE202230630',
    'divergence',
    'divergence: Glob recurses once for each of 10001 separators (the limit that fixed the CVE), '
    'and a goroutine of gocpp has a fixed 8 MiB stack (D24) that the unoptimized frames of the '
    'recursion exhaust: stack overflow; Go grows stacks to 1 GB',
)
