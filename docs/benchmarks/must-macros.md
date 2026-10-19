# `!` and macros against hand-written code (step 10.13)

`!` and the macros of Phase 10 are rewrites, so the plan says they cost what the code that you write by hand costs. This page
measures it: each program of `bench/phase10/CASE/VARIANT/` is built by gocpp at `-release` (g++ 15 and clang++ 21), and the
variants of a case print the same. The tool is `go run ./tools/phase10bench` (`-reps`, `-cxx`, `-cases`, `-keep DIR`): it builds
every variant, then runs rounds (one round runs each binary once, in a fixed order, so a change of the machine's load hits all
variants alike; the first round is dropped) and prints the median CPU time (user plus system, from the kernel's accounting: a busy
machine moves wall time more than CPU time), the wall time, the MAD of the CPU time in percent, and the CPU time over that of
the variant `hand`.

| Case | Variants | What the loop does |
|---|---|---|
| `must_call` | `bang`, `hand` | 300 million `acc = step!(i, acc)` against `v, err := step(i, acc); if err != nil { return 0, err }; acc = v` |
| `must_func` | `bang`, `hand` | 100 million calls of a must function with three dropped errors, called with `F!`, against the same checks by hand |
| `assert_const` | `macro`, `hand` | a passing `#Assert(i >= 0, "i is negative")` (constant message) in a hot loop, against an `if` |
| `assert` | `macro`, `hand` | a passing `#Assert(sum >= 0, fmt.Sprint("sum ", sum, " at ", i))`: the lazy message captures two variables, one of them changed in the loop |
| `wrap` | `macro`, `hand`, `func` | a wrap macro (`#Tally()`: count, then `#body()`) on a small function called 300 million times; `hand` writes the count inline, `func` is what Go without macros needs: a function that takes the code as a func value |
| `cross` | `macro`, `hand`, `func` | the same two macros (`#util.Assert`, `#util.Tally`) declared in **another package**, 200 million iterations; `func` has them as functions of `util` that take func values |

Results (2026-10-19, 192-core machine shared with other jobs, load average 100 to 1000 while it ran, so read the MAD column; 12 measured rounds
after one dropped round, and 25 rounds for `assert`, `cross` and `must_call`, which were run again):

## g++

| case | variant | CPU ms | wall ms | MAD % | CPU over hand |
|---|---|---:|---:|---:|---:|
| assert | macro | 82 | 82 | 6.9 | 1.01 |
| assert | hand | 81 | 82 | 10.5 | 1.00 |
| assert_const | macro | 71 | 72 | 8.2 | 0.93 |
| assert_const | hand | 76 | 77 | 9.8 | 1.00 |
| cross | macro | 71 | 71 | 17.1 | 1.00 |
| cross | hand | 71 | 72 | 18.2 | 1.00 |
| cross | func | 8337 | 8378 | 0.7 | 116.96 |
| must_call | bang | 319 | 322 | 3.5 | 1.00 |
| must_call | hand | 320 | 322 | 1.0 | 1.00 |
| must_func | bang | 930 | 937 | 8.2 | 0.75 |
| must_func | hand | 1248 | 1257 | 11.0 | 1.00 |
| wrap | macro | 82 | 82 | 7.3 | 1.00 |
| wrap | hand | 82 | 83 | 7.0 | 1.00 |
| wrap | func | 5781 | 5825 | 3.4 | 70.86 |

## clang++

| case | variant | CPU ms | wall ms | MAD % | CPU over hand |
|---|---|---:|---:|---:|---:|
| assert | macro | 75 | 76 | 7.9 | 1.00 |
| assert | hand | 75 | 75 | 6.7 | 1.00 |
| assert_const | macro | 68 | 68 | 0.8 | 1.01 |
| assert_const | hand | 67 | 68 | 2.2 | 1.00 |
| cross | macro | 72 | 73 | 12.0 | 1.27 |
| cross | hand | 57 | 57 | 14.0 | 1.00 |
| cross | func | 8392 | 8458 | 2.0 | 147.64 |
| must_call | bang | 290 | 292 | 10.1 | 1.02 |
| must_call | hand | 282 | 284 | 8.5 | 1.00 |
| must_func | bang | 128 | 129 | 5.9 | 0.95 |
| must_func | hand | 135 | 136 | 1.8 | 1.00 |
| wrap | macro | 79 | 79 | 1.1 | 1.00 |
| wrap | hand | 79 | 79 | 1.2 | 1.00 |
| wrap | func | 7291 | 7337 | 1.3 | 92.52 |

## What the numbers say

- **`!`** is at the hand-written speed: `must_call` is 1.00 (g++) and 1.02 (clang++). `must_func` is 0.95 with clang++ and **0.75** with g++: the C++ of
  the two is the same except that `!` gives every call a new variable (`gp::error must_e2 = std::get<1>(gp_t1)`), where the hand-written code
  reuses `err` (`err = std::get<1>(gp_t1)`, an assignment of a reference-counted value that g++ does not optimize away). No helper call is left in
  the C++ (`TestPhase10NoHelpers`, `tests/golden/must.gp`).
- **Macros** (`assert_const`, `assert`, `wrap`) are at the hand-written speed, 0.93 to 1.01. `cross`, the same macros from **another package**, is
  1.00 with g++. With clang++ one build measured 1.27: the loop in `main` is the same nine instructions in the macro and in the hand-written build
  (checked with `objdump -d`: the same code, the same registers), and in the slow build it straddles a 64-byte boundary (the code in front of it moved
  by 16 bytes), so it is a layout effect of that binary and not the macro; an earlier build of the same program measured 0.99 to 1.02.
- **The hidden arguments.** A first measurement of `assert` gave 2.9 to 3.1 times the hand-written loop with g++. The cause was not the lambda's reference
  to `sum` (`sum` and `i` stay in registers): every iteration built four `gp::string` arguments (the file, the function and the text of the condition),
  which are not trivially destructible, so they had to be kept and destroyed around the lazy message that may throw. Since step 10.13 the lowering
  passes them as `gp::static_view` (a pointer and a length; `viewParam` in `internal/lower/generics.go`) and makes a `gp::string` only where the macro's
  code reads one, in the panic call ([ADR-075](../adr/075-user-macros-are-generic-functions.md), "Test notes and a correction"): 1.01 now.
- `func` is what Go needs without macros: a function that takes a func value. It costs 70 to 150 times more here (an allocated closure on every call),
  which is what the macro form saves.
