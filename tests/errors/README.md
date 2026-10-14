# tests/errors

An error test is one `.gp` file that must not compile. The harness checks it in this folder
with `gocpp check -e file.gp` (`-e` reports every error, not only the first 10; Go's own test
runner passes `-e` too). It passes when gocpp check reports exactly the errors that the
`// ERROR` comments ask for and the warnings that the `// WARNING` comments ask for, on their
lines, and no other error or warning, and exits with 1. A file without `// ERROR` comments
must check without errors and exit with 0 (a "no false errors" test); warnings don't change
the exit status, so a file with only `// WARNING` comments exits with 0 too.

[undefined_name.gp](undefined_name.gp) (the message is the one `go/types` gives):

```go
// phase: 01

package main

import "fmt"

func main() {
	fmt.Println(count) // ERROR "undefined: count"
}
```

The comments follow Go's own `test/` folder:

- `// ERROR "regexp"` goes at the end of the line that must have the error. The regexp is
  unanchored and matched against the message, the text after `file.gp:LINE:COL: `.
- The regexp is written between double quotes and used as written: no escapes are processed,
  so `"cannot use x \(variable"` has the regexp escape `\(`. It can't contain a double quote;
  write `\x22` for one.
- `// ERROR "rx1" "rx2"` wants two errors on that line, one for each regexp.
- One regexp uses up every error on its line that it matches.
- `LINE` in a regexp stands for `file.gp:N` of its own line, `LINE-1` and `LINE+2` for a line
  before or after it: for an error that points at another line.
- A line that contains `////` is skipped, so `//// ERROR "x"` turns an expectation off.

`// WARNING "regexp"` works exactly like `// ERROR`, for a warning: gocpp check prints a
warning as `path/file.gp:LINE:COL: warning: message`, and the regexp is matched against the
message after `warning: `. An `ERROR` regexp matches only errors and a `WARNING` regexp only
warnings. Both can be on one line, one after the other, each with its own regexps:
`// ERROR "rx1" // WARNING "rx2"`. A `// WARNING` comment can also end a comment line, for
example one that looks like a broken directive:

```go
// gocpp:link m // WARNING "is an ordinary comment, not a directive"
package main
```

gocpp check prints each error as `path/file.gp:LINE:COL: message` (the column may be missing).
A line that starts with a tab or a space continues the error above it (the regexp sees it
too), and a line that starts with `#` is skipped. Any other output fails the test.

Only the phase line, a `//go:build` line (see [tests/README.md](../README.md)) and the
`// ERROR` and `// WARNING` comments have a meaning here; `// Output:` and the other lines of
[run tests](../run/README.md) are errors.

The tests here check with the phase gates of the last finished phase
([internal/version](../../internal/version/version.go)): a test of a gocpp-specific error in
code that uses a later phase's feature (a header import, a C++ namespace, `C.int`) also gets
that feature's gate error ("header imports are not supported yet (Phase 04)"), with its own
`// ERROR` comment. When that phase is finished, its gate opens and those comments go (the
step that lifts the gate updates them). The front-end without the gates is tested by the
"no false errors" corpus in
[internal/check/testdata/corpus](../../internal/check/testdata/corpus/README.md).
