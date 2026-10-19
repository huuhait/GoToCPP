# The standard library

Most of the standard library is Go's own source, ported to `.gp` files by a tool and compiled with
your program. This lesson reads lines, counts words and sorts them.

> **Experimental today.** The ported packages (`strings`, `strconv`, `sort`, `bufio`, `os` and the
> rest) are phase 09. Run this lesson with `GOCPP_GATE_PHASE=09 gocpp run .`. The
> [standard library reference](../stdlib/index.html) lists the packages that exist.

<!-- program: 09 id=wordfreq go -->
```go
package main

import (
	"bufio"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const text = `the quick brown fox
jumps over the lazy dog
the end`

func main() {
	counts := map[string]int{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		for _, w := range strings.Fields(sc.Text()) {
			counts[strings.ToUpper(w)]++
		}
	}

	words := make([]string, 0, len(counts))
	for w := range counts {
		words = append(words, w)
	}
	// Most frequent first, then alphabetical.
	sort.Slice(words, func(i, j int) bool {
		if counts[words[i]] != counts[words[j]] {
			return counts[words[i]] > counts[words[j]]
		}
		return words[i] < words[j]
	})
	for _, w := range words[:3] {
		fmt.Printf("%-6s %d\n", w, counts[w])
	}

	n, err := strconv.Atoi("12x")
	fmt.Println(n, err)
	fmt.Println(slices.Contains(words, "FOX"), strings.Repeat("=", 5))
}
```

<!-- output -->
```text
THE    3
BROWN  1
DOG    1
0 strconv.Atoi: parsing "12x": invalid syntax
true =====
```

Without the switch the compiler says which package waits for its phase:

<!-- rejected: wordfreq -->
```text
./main.gp:4:2: package bufio is not available yet (Phase 09)
```

## What to notice

- `strconv.Atoi` returns Go's error with Go's text, because it is Go's code.
- `os` gives files, directories, the environment and `os.Args`; `regexp`, `time`, `context`,
  `io` and `io/fs` are there too.
- **The standard library is a subset of Go's: 37 packages today.** The
  [reference](../stdlib/index.html) lists them. `flag` and `encoding/json` are declaration stubs for
  now: a program that imports them type-checks, but there is no implementation yet (the port of `flag`
  is in progress). So the programs [wc](../../examples/wc/main.gp) and
  [grep-lite](../../examples/grep-lite/main.gp) in `examples/`, which use `flag`, are still marked
  pending in the test suite.
- A package that gocpp does not have is reported by `gocpp check`: `log`, `math/rand`,
  `path/filepath`, `encoding/hex`, `text/template`, `net/http` and many others fail with
  `package ... is not available yet`. The [Go developers guide](../guides/go-developers.md#what-is-not-there)
  lists the usual ones, and C++ libraries can fill gaps ([lesson 9](09-cpp-containers.md)).

Next: [C++ containers](09-cpp-containers.md).
