# Draft upstream issue: gc constant shift count >= 1<<63 (not filed)

Found 2026-10-14 by gocpp's random differential tests (seed 5021, job 03-order5021). gocpp follows the
spec; gc 1.25.14 doesn't. Not filed: the user decides whether to file it at github.com/golang/go.

**Title:** cmd/compile: wrong result for ((x >> c) & d) << e when the constant shift count c does not fit in int64

What version of Go are you using? go1.25.14 linux/amd64. The rules below are unchanged on master (checked 2026-10-15).

The program below should print 0: x >> ^uint(0) is 0 for x >= 0 (Go spec: shifts by a count >= the width
give 0 or -1). On amd64 it prints 16 (checked). On arm64, f compiles to `UBFIZ $4, R0, $1`, which also gives 16 (checked from the assembly; not run). On 386 it prints 0, because uint is 32 bits there. With a
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

Checked by the manager on 2026-10-15 with go1.25.14: f(3) = 16 and f(-3) = 16 on amd64 (want 0 and 24);
on GOARCH=386, f(3) = 0 and f(-3) = 24, but x >> ^uint64(0) gives 16 for int, int32 and int8 x; unsigned x
gives 0; a count of 1<<63 gives 0. No existing Go issue was found in a quick search, but none was done
carefully: search the tracker before filing.

