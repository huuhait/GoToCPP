# Go 1.25.14's tests of flag; see convert_std.py.
FILES = ['all']
skip_file(
    'example_textvar_test.go',
    'its only example, ExampleTextVar, uses net.IP, and gocpp does not have package net (tier 3)',
)
skip_file(
    'example_value_test.go',
    'its only example, ExampleValue, parses a net/url.URL, and gocpp does not have package net/url',
)
drop(
    'example_func_test.go',
    'ExampleFunc',
    'it parses an address with net.ParseIP, and gocpp does not have package net (tier 3)',
)
drop(
    'flag_test.go',
    'TestExitCode',
    'it starts the test binary again with os/exec to read the exit status of flag.ExitOnError, and '
    'gocpp has no os/exec',
)
patch(
    'flag_test.go',
    'flag_test.go:.* before being defined',
    'flag_test.g[op]:.* before being defined',
    'the test file is flag_test.gp in gocpp (the message names the file of the call)',
)
