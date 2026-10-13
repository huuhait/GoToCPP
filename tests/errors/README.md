# tests/errors

An error test is one `.gp` file that must not compile. The harness checks it in this folder
with `gocpp check -e file.gp` (`-e` reports every error, not only the first 10; Go's own test
runner passes `-e` too). It passes when gocpp check reports exactly the errors that the
`// ERROR` comments ask for, on their lines, and no other error, and exits with 1. A file
without `// ERROR` comments must check without errors and exit with 0 (a "no false errors"
test).

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

gocpp check prints each error as `path/file.gp:LINE:COL: message` (the column may be missing).
A line that starts with a tab or a space continues the error above it (the regexp sees it
too), and a line that starts with `#` is skipped. Any other output fails the test.

Only the phase line, a `//go:build` line (see [tests/README.md](../README.md)) and the
`// ERROR` comments have a meaning here; `// Output:` and the other lines of
[run tests](../run/README.md) are errors.
