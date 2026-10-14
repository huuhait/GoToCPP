# Draft upstream issue: gc constant shift count >= 1<<63 (not filed)

Found 2026-10-14 by gocpp's random differential tests (seed 5021, job 03-order5021). gocpp follows the
spec; gc 1.25.14 doesn't. Not filed: the user decides whether to file it at github.com/golang/go.

**Title:** cmd/compile: wrong result for ((x >> c) & d) << e when c is a constant shift count >= 1<<63

What version of Go are you using? go1.25.14 linux/amd64 (the rule is the same on master).

The program below should print 0: x >> ^uint(0) is 0 for x >= 0 (Go spec: shifts by a count >= the width
give 0 or -1). On amd64 and arm64 it prints 16. On 386 it prints 0, because uint is 32 bits there. With a
uint64 count (`x >> ^uint64(0)`), 386 also prints 16, and so do int32 and int8 x. An unsigned x is not
affected.

```go
package main

import "fmt"

//go:noinline
func f(x int) int { return ((x >> ^uint(0)) & 3) << 3 }

func main() { fmt.Println(f(3)) } // want 0, got 16 (f(-3): want 24, got 16)
```

The cause seems to be these rules in src/cmd/compile/internal/ssa/_gen/generic.rules (and the same rules
for 32, 16 and 8 bits):

```
(Lsh64x64 (And64 (Rsh(64|64U)x64 <t> x (Const64 <t2> [c])) (Const64 [d])) (Const64 [e])) && c >= e => (And64 (Rsh(64|64U)x64 <t> x (Const64 <t2> [c-e])) (Const64 <t> [d<<e]))
(Lsh64x64 (And64 (Rsh(64|64U)x64 <t> x (Const64 <t2> [c])) (Const64 [d])) (Const64 [e])) && c < e  => (And64 (Lsh64x64 <t> x (Const64 <t2> [e-c])) (Const64 <t> [d<<e]))
```

The shift counts are unsigned, but c and e are compared as signed int64. The count 1<<64-1 is read as
c = -1, so `c < e` holds and x>>c becomes x<<(e+1): (3<<4) & 24 = 16. A possible fix is to compare
uint64(c) and uint64(e), or to require that c and e are in [0, 64) (and the same for the narrower widths).
We found it through differential testing: an index like `s[(x >> (1<<n - 2)) & 3:]` with n folded to 0.
