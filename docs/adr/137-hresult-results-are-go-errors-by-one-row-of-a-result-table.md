# ADR-137: HRESULT results are Go errors, by one row of a result table

Status: accepted (2026-10-21; user decision: C/C++ functions and methods that return the typedef `HRESULT` are usable from gocpp as returning Go `error`)

## Context

A COM or Direct3D call returns an `HRESULT`, a 32-bit code: negative is a failure, anything else is a success, and not only
`S_OK`. The idiom of the demos was `if hr != 0` (wrong for `S_FALSE` and `DXGI_STATUS_OCCLUDED`, which are successes) and
most calls were statements whose failure nobody saw (`swapChain.GetBuffer(...)`, `device.CreateRenderTargetView(...)`).
Go's answer to "this call can fail" is `error`. The user decided that a function or method that returns `HRESULT` is used
from gocpp as one that returns `error`:

- `nil` is success, which is `hr >= 0` (`SUCCEEDED(hr)`), `S_FALSE` included.
- a concrete error type carries the code, with a message like `HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)`, so
  `errors.As` works.
- `-interop=opaque` keeps today's behavior.

[ADR-130](130-argument-adaptations-are-one-table-over-the-parameter-type-and-the-go-argument.md) listed this as its first
"later": the argument table is over the parameters, a result needs a table of its own, and the rule needs the typedef
*name* (`HRESULT` is `LONG` is `long`: the type alone says nothing). This ADR is that table, with one row.

## Decision

### (a) The detection rule: the exact typedef name `HRESULT`

A call is an error result when **the type that the declaration wrote for the result is exactly the name `HRESULT`**, and
the compiler's own type for it is a signed `int` or `long`. Clang gives both for the probe of the call: `Written`
(the sugar, `HRESULT`) and `Type` (`long`). The name is written without a qualifier (a `typedef long HRESULT` of another namespace, unqualified in its own, is not told from
the global one: see the table), has no template arguments, and is the outermost type (`HRESULT*` and `const HRESULT&` are not it). Not detected, on purpose:

| Type | Why not |
|---|---|
| `LRESULT` | a window procedure's result: a pointer-sized value, not a status |
| `NTSTATUS` | its success range and its codes are another table (a possible next row) |
| `DWORD`, `BOOL`, `int`, `long` | numbers; `GetLastError()` is a `DWORD` and `RegisterClassExW` a `BOOL` |
| a typedef of `HRESULT` (`typedef HRESULT MYRESULT`) | the written name decides, and it is `MYRESULT`: the rule has no way to see through a user's typedef that does not say what it means; a known gap, see below |

The rule is on the type of the **call**, so a method through a vtable, a function-pointer member or global, and a lambda
are covered as a free function is. The two cases in the table whose compiler type has lost the typedef (a function pointer
held in a **local variable**, and `std::function`) are the exceptions: they stay the raw number, which is accepted and
documented, not a bug waiting for a fix. The rule needs no list of functions and no header knowledge.

What the compiler's sugar of the call can and cannot say, measured (review round 1):

| Call | Detected |
|---|---|
| a function, a method, a virtual method (vtable), `obj.fn(x)` with `fn` a function-pointer member, a global function pointer, a lambda, `f()(x)` (a call of a call's function-pointer result), `operator()` and a function template | yes |
| a call through a **function pointer held in a local variable** (`fn := edge.get_fn(); fn(4)`) | **no, the number**: the probe names the local by `std::decay_t<decltype(...)>` and `std::declval`, which return the compiler's canonical type, and the typedef name is gone |
| a call of a **`std::function<HRESULT(int)>`** (a global, a member or a local) | **no, the number**: `std::function::operator()` returns the `R` of its canonical template argument |
| `auto f() { return neg(2); }` and `decltype(auto)` results | yes, although the function never writes the name: the deduced type is the sugar of the call inside. It is `HRESULT` for the compiler, which is what the rule asks for |
| `typedef long HRESULT` inside `namespace app` | yes, by mistake: the written type is `HRESULT` there too, and the probe's text has no namespace to tell it from the global one. A header that has two meanings of the name is not expected |
| `const HRESULT f()` | yes (the qualifier is skipped; `edge.const_hr()` prints `HRESULT 0xFFFFFFFD` in the tests; corrected after review round 2, which found this row wrong) |

A local that holds a function pointer is the one that matters; the program calls it where the pointer is declared, or
through the member or the global. `gocpp` does not say anything where a number silently stays: such a call compiles as the
number it is in C++. The fix is a different probe for locals (their type without `declval`), which changes
every program's probe file and cache keys, and was not done for this round.

### (b) How it composes with the argument table: a second, independent table

`internal/typedinterop/adapt/results.go` has `ResultTable` of `ResultRow`s, as `adapt.go` has the argument `Table`. The
two are independent: the lowering writes a call with the rows of its arguments (out-parameters copied back, strings, enums)
and, outermost, the row of its result. `RowHResult` is the only row. A call with `&v` for an `int*` parameter and an
`HRESULT` result is `gp::hresult_error(gp::cgo::call_out<int>(&v, ...))`: the copy back happens before the code is looked
at.

### (c) Success codes, and ignoring the result

- Every code that is not negative is `nil`: `S_OK`, `S_FALSE`, `DXGI_STATUS_OCCLUDED`. That loses the difference between
  them. A program that needs it takes the HRESULT itself with a conversion to a number: `int32(sc.Present(1, 0))` or
  `C.long(...)`. A call that is the operand of such a conversion is the C++ call as it is, with no error. The test is on
  the syntax (`collect.KeepsHResult`): the parent of the call is a conversion to a Go number type (`int32`, `uint32`, ...) or
  to a C number type (`C.long`, `C.int`, ...), parentheses skipped.
- A call that is a statement, or the operand of `defer` or `go`, drops the result as in C++: not wrapped, no cost. (A
  `vet` check for a dropped HRESULT is the next step, below.)
- The code is reachable from the error: it has the methods `Error() string` and `HResult() int32`.
  `var h interface{ HResult() int32 }; errors.As(err, &h)` finds it. Two errors with the same code are equal
  (`err == other`, `errors.Is`).

One call gives either the error or the number, never both: `int32(err)` of the error is refused (an error is not a number),
and `nil` hides `S_FALSE` and the other success codes. A program that needs the success code takes the code from the
call itself (`int32(call)`), and the failure code is in the error (`errors.As` with `HResult()`). Assigning the call to `_`
(`_ = call`) is a statement in this sense: not wrapped, no error is made for nothing.

The type is `runtime.HResultError`, native to the runtime like `runtime.CppException`, and **not nameable from Go**. A
nameable type (`syscall.Errno`-like) needs a std package and an implicit import for a type the program never otherwise
imports; that is an open question (below), and the interface above is enough for `errors.As`. The message is
`HRESULT 0x` + eight hex digits, then ` (NAME)` for the 25 well-known codes of `runtime/src/hresult.cpp` (`E_FAIL`,
`E_INVALIDARG`, `E_OUTOFMEMORY`, `DXGI_ERROR_DEVICE_REMOVED`, `D3D11_ERROR_FILE_NOT_FOUND`, ...) or ` (Win32 error N)` for
`HRESULT_FROM_WIN32(N)` (facility 7). An unknown code has no suffix.

### (d) The checker, the editor and hover

The checker needs nothing of its own: the call is an `error` for go/types (see (g)), so a misuse such as `hr != 0`, `hr + 1`,
`var n int32 = f()` is go/types' own message, in Go's words, plus one related line with the way out:

```
main.gp:11:11: cannot convert 0 (untyped int constant) to type interface{Error() string}
	the call returns an HRESULT, which is an error here (nil when the code is not negative): compare it with nil; the code is int32(call) on the call itself, or HResult() through errors.As
```

- **The hint** is added by a post-pass over go/types' errors (`check/hresults.go`, `hintHResults`): an error inside a
  statement (the header of a compound one, the cases of a `switch` on the error) that has a wrapped call, or a variable
  made from one, gets the line. Messages say what the user wrote (`hresult.f(x)`), not the loader's `C.f(x)` or the checker's
  `gp_cgoh.As[error](...)` (`userTexts`, `unwrapErrorConversions`).
- **It is an error with g++ too.** go/types' errors are errors for either compiler. The one rule that is a warning for the
  rest of the typed checker under g++ (the probe is clang++ and g++ may accept it) is gocpp's rule here: a C number that goes
  into an error (`hr = hresult.nt_result(1)`) is an error for either compiler (`icheck`: the target `error` is always an
  error), since g++ accepts `err != 0` as a compare with `nullptr` and the build would pass with the wrong meaning.
- **`gocpp check` and `gocpp build` agree** for the targets go/types has no type for (`var n C.long = hr`, `n = f()` with `n` a
  C variable, `return f()` in a function that returns a C type): the check reports them (`checkErrorTargets`), with the same
  words as the lowering (`cguard.go`). `TestTypedHResultMisuseIsTheSameInCheckAndBuild` runs both over the same programs.
  Not covered, as for any Go value: an `error` passed as the **argument of a C/C++ function** that wants a number is
  C++'s error at the build (`cannot convert 'error' to 'int'`), as a Go slice there is; the check has no parameter type.
- **The language server** does not run the hook (it does not use `typedinterop.Hooks`: the hook asks the compiler before
  the check is done, and the editor's check is on every keystroke, its typed answers coming later from the background
  analysis). **The rule there is: a valid program shows no diagnostic.** Without the hook the checker sees the call as a
  C/C++ value, and go/types goes on only with an operand it can type: a map literal whose key it cannot assign
  (`map[error]string{e: "dev"}`) skips the value next to it. `check/mapvalue.go` does for that value what go/types would
  have, by asking go/types itself: the value is checked as the one element of `map[int]ELEM{0: value}`, with ELEM the
  element *type* of the map under a name that no program can write (a scope of its own round the value, which the completer never lists), so no name, import, alias, tag, embedded field, unexported name or shadowing is looked up again, and the words
  are Go's own (`cannot use "x" (untyped string
  constant) as int value in map literal`, `(overflows)`, `(truncated)`, `(value of struct type W)`, `int does not implement
  fmt.Stringer (missing method String)`, `invalid composite literal element type int`), a constant that does not fit a
  float32 or a type parameter is refused as Go refuses it, and an elided literal (`map[error]T{e: {1, 2}}`, `[]int`, `*T`,
  `struct{}`, nested maps, two levels of them, a named map type `type M map[any]T; M{cv: {1}}`) gets the element type of its
  map, which `info.Types` keeps for the lowering too (the build runs such a program: `hresult_mapkey.gp`, with both compilers). This holds for any C/C++ value as the
  key (`map[any]int{cv: 1}`, `map[C.long]T{cv: {1}}`), so `gocpp check` is right for them too. A difference to Go that is
  left: go/types reports the first error of the value only (`CheckExpr` has no error list), where Go lists every one. So an error
  that comes after the first one in the same value is not listed, and **a variable that is declared and not used inside a
  function literal in such a value may go unreported**: `declared and not used` of a variable that only a skipped nested value uses
  is false and is dropped, and when it is the first error of the value, a genuinely unused variable that comes after it in the same
  value is lost with it (`map[any]func(){k: func() { y := 1; z := 4; _ = map[any]int{k: y} }}`: Go says `z`, gocpp says nothing).
  A missed diagnostic, never a false one; it is reported once the value is edited so that `y` is not the first. The
  levels of nested literals are walked once (each value is checked once). The editor tests open the whole valid program of the typed run test and the map-literal
  shapes (key, value, mixed, elided, named map, plain C key) and wait for the end of the background analysis
  (`hookAdapted`), and a wrong value under a C key is an error there, with the same words.
  **What the editor does not report**: the editor does not run the HResult checks at all, since they need the type `error`
  of the call. Only the map-value check above works there, and the errors that do not depend on that type (`declared and
  not used`). Verified silent in the editor, and errors in `gocpp check` and `gocpp build`: `hr += 1`, `hr++`, `hr[0]`,
  `hr + 1`, `hr != 0`, `if f() < 0`, `switch hr { case 0: }`, `hr = "x"`, `hr = 5`, `hr = hresult.nt_result(1)`,
  `var n int32 = hr`, `var n C.long = hr`, `n = hr`, a return of a number where `error` is wanted, an `error` passed where
  a number is wanted, a wrong key type as in `map[string]int{hr: 1}`, and the hint on each of them. Running the hook in the
  editor is a possible next step (a compiler analysis per edited package; with the probe cache warm about 0.2 s on the demo).
- **Hover** on a C++ function whose declaration writes `HRESULT` for the result adds a line: "In typed mode a call returns
  an `error`: `nil` when the HRESULT is not negative, `int32(call)` is the code itself."
- `bindgen` (`gocpp doc`-style views of a header) writes `error` for an `HRESULT` result of a function or a function-pointer
  type: `func create_device(flags C.int) error`.

#### A variable made from the call

`hr := call` is an `error` variable because the call **is** an `error` expression for go/types: the variable is typed once,
at its declaration, and every use is Go's. There is no per-use code for it. What Go does with an `error` works as in Go:

- `hr.Error()` is a string (concatenation, `strings` functions, `var t string = hr.Error()`, `hr.Error()[:4]`, `range
  hr.Error()`), `hr = nil`, `append(xs, hr)` and `append(xs, f())` with `xs []error` (also `box.items = append(...)`),
  `map[error]string{hr: "dev"}` and `map[error]int{f(): 1}` (the hash is the code's), `errors.Is/As/Unwrap/Join`,
  `fmt.Errorf("%w", hr)`, `panic`, type assertions and type switches, `switch hr { case nil: ... }`, and every `error`
  variable, field, parameter and result. Go errors are assigned to it (`hr = errors.New(...)`).
- What it does not do is refused with go/types' own words: a number (`cannot use hr (variable of type error) as int32
  value in assignment`), `hr += 1` and `hr++` (`operator + not defined on hr (variable of interface type error)`), `hr[0]`, a
  case of a switch (`invalid case 0 in switch on hr (mismatched types untyped int and error)`), and an assignment of a
  number (`cannot use 5 (untyped int constant) as error value in assignment`). No message names an internal stand-in.
- The hint does not tell to convert the variable (`int32(hr)` is refused for an error): the code is `int32(call)` on the
  call itself, or `HResult()` through `errors.As`.
- A variable made from a call that the probe cannot type (`&p` for a `void**`) is the same, the call being decided by the
  declarations (`adapt.SigsReturnHResult`).

### (e) The generated C++

```cpp
auto hr = gp::hresult_error(swapChain->ResizeBuffers(0, w, h, 0, 0));
```

`gp::hresult_error(T)` (`runtime/include/gocpp/hresult.hpp`) is an inline template for integral `T`: it tests
`static_cast<int32_t>(code) >= 0` and returns the null `gp::error` interface, so **a success allocates nothing and costs a
compare**. A failure makes one boxed `HResultError{code}`. Truncating to 32 bits is the HRESULT definition (on a 64-bit
`long` the high bits are not part of the code; the demo's header and the tests run with `typedef long HRESULT` on Linux to
show it).

### (f) COM vtable calls

A COM method is a virtual member function: `swapChain.Present(1, 0)` is `swapChain->Present(1, 0)` and the rule is on the
type of the call, so nothing is special. A call that has an argument the probe cannot spell (`&p` of a pointer variable for
a `void**` parameter such as `GetBuffer`; `&n` of a Go number variable) is not probed as a whole, so its type comes from the
declarations: the callee's overloads all return `HRESULT` (`adapt.SigsReturnHResult`, with the typedef asked for from the
method's result by `ftype.NeedsMore`). The declarations are asked for only when some call has no probe answer
(`Report.hasUnprobedCall`), because they cost more than the probe when its answers are cached.

### (g) How the program gets the type: the check wraps the call in `error(...)`, once

The first design of this ADR typed the error in many places: a second probe round with a stand-in `error` in the probe file,
an IR node, use sites of the variable, and a guard in the lowering for each place an error variable goes. Review round 2
found each place that was forgotten (`e.Error()` as a string, `append`, `e = nil`, a map key), so the design was replaced by
one that cannot forget one: **give the call the type `error` once, and let go/types do the rest.**

1. `check.Check` runs go/types on the package as always, and rewrites nothing yet.
2. If the configuration has a `HResults` hook (`check.Config.HResults`, set by typed mode with a compiler), the hook
   (`typedinterop.HResultsHook`) analyses the package and returns the calls of C/C++ functions that return the typedef
   `HRESULT` (the probe row `adapt.RowHResult`, else the declarations, see (f)).
3. Check wraps each call, in the syntax tree, in the conversion `error(call)` (`wrapHResults`) and checks again. The
   existing machinery for a conversion of an opaque value makes it `gp_cgoh.As[gp_cgoh.Error](call)`; `gp_cgoh.Error` is
   Go's predeclared `error` (a type of the hidden package), not the name `error`, so a program that declares its own
   `error` in the scope of the call (`error := "x"`, `type error int`) is not broken (`hresult_shadow.gp`). go/types types that as `error`.
   A variable declared from it is `error` everywhere.
4. The lowering writes `gp::hresult_error(call)` for the conversion (`lower/typedresult.go`).

The hook runs **after the first check and before any rewrite loop**: the rewrites of the uses of values that go/types has no
type for (a slice, a comparison, a conversion) are irreversible, and made for the variable that holds the call they would
be wrong. `collect.KeepsHResult` says which calls are not wrapped: the call is a statement, `defer` or `go`, is `_ = f()`,
or is the operand of a conversion to a Go or C number (`int32(f())`, `C.long(f())`). A call already under `error(...)` is not
wrapped twice.

Consequences:

- **Cost.** The hook runs `typedinterop.Analyze`, and the build runs it again at the end (its probes are the same, so the
  compiler runs of the second are cache hits). A program without such a call pays only the first analysis it paid before, and
  a program without any C/C++ include does not run it. There is no second probe round, no stand-in in the probe file, and no
  change of any probe file or cache key. Measured: see below.
- **Generic functions.** A package that has generic functions is lowered without the typed view, so the lowering trusts
  `Package.IsHResultCall`, which check sets, as well as the typed report (test `hresult_generic.gp`).
- **No per-use code.** `lower/typedresult.go` is the one place of the lowering (43 lines); the collector, the probe, `icheck`,
  `ftype` and `adapt` have no error-variable code. The first design's `errormisuse`, `goTypeOfOpaque` and use sites were deleted.

### (h) Opt-out, and the breaking change

- `-interop=opaque` (and a machine without clang) is untouched: an `HRESULT` is the number the C++ function returns.
- In typed mode, `hr := f(); if hr != 0` is an error with the hint above, and `var n int32 = f()` too. There is no
  transitional rule (a warning, or a mode in which the old code still works): the only users are the demos in this
  repository, which this commit updates (`demos/imgui-dx11`). A program that wants the number writes `int32(f())`.
  The opaque mode and typed mode now compile different programs for code that uses HRESULTs; the demo README says so.

## What is built

| Part | Where |
|---|---|
| runtime error and helper | `runtime/include/gocpp/hresult.hpp`, `runtime/src/hresult.cpp`, `runtime/test/hresult_test.cpp` |
| the result table | `internal/typedinterop/adapt/results.go` (`RowHResult`, `ResultRowOf`, `SigsReturnHResult`) |
| which calls | `internal/typedinterop/hresults.go` (`HResultsHook`, `Report.HResultCalls`, `ReturnsHResult`), `collect/hresult.go` (`KeepsHResult`) |
| the type `error` | `internal/check/hresults.go` (`Config.HResults`, `wrapHResults`, the hint, `checkErrorTargets`, `IsHResultCall`), `check.go` (where the hook runs) |
| the error rules for C values | `icheck/rules.go`, `ftype/value.go` (`Compat("error")`), `lower/cguard.go` |
| lowering | `internal/lower/typedresult.go` (`hresultError`), `cvalues.go` |
| bindgen, hover | `bindgen/types.go`, `internal/lsp/server/foreignhover.go` |
| tests | `tests/typed/hresult*.gp` (with C++ twins), `err_hresult_*.gp`, `internal/check/hresults_test.go`, `cmd/gocpp/typedhresult_test.go` (check and build agree), and the unit tests of each package |
| the demo | `demos/imgui-dx11/main.gp` |

## Measured

On the development machine (128 cores), `gocpp check`, typed mode, a fresh probe cache for "cold" and the same cache again
for "warm". The binaries of the two sides were run in turn, three rounds, so that the load of the machine (other jobs ran
during the first rounds) falls on both; the table is the second set of rounds, with the machine quiet (load average 1 to 4).
"Base" is a binary of `job/12.check` with the demo as it was before this ADR.

| Program | Binary | Cold | Warm |
|---|---|---|---|
| `demos/imgui-dx11`, `-os windows` (`<windows.h>`, `<d3d11.h>`, `<dxgi.h>`, ImGui) | base | 16.6 to 16.9 s | 0.41 to 0.43 s |
| the same, with its HRESULT calls | this branch | 17.3 to 17.7 s | 0.37 s |
| `tests/typed/adapt_voidout.gp` (no HRESULT) | base | 5.9 to 6.1 s | 0.07 to 0.08 s |
| the same | this branch | 6.2 to 6.8 s | 0.07 to 0.08 s |

A cold build pays the hook's analysis once more (about 0.4 to 1 s on top of 6 s and 17 s: the compiler runs of the build's
analysis that follows are cache hits, but the hook's first analysis is the one that makes them). **A warm run costs no
more than before**, and the demo's is faster. That needed one change outside this ADR's own code: the first measurement of
this design had the warm demo at 0.62 s (against 0.42 s), because the hook asks for the declarations of a call that the probe
cannot type (`&p` for a `void**`), and every cache entry of that run then hashed the same list of hundreds of headers again
(`probe.DepsHash`, one `stat` of each per entry, tens of entries, twice per build). A `probe.Runner` of a command of the
command line now says `StableHeaders` (a command reads the headers once), and hashes one list once
(`Runner.depsHash`, `TestStableHeadersHashAListOnce`); the language server's Runner does not, because its user edits
headers. This also takes the base's own warm cost down (the 0.42 s above is the base without it).

**The cost of `gocpp check` on large sources** (wall time of `gocpp check`, best of three, a 192-thread machine; dev is the base):

| source | dev | round 7 | round 8 |
|---|---|---|---|
| one function with 2000 map literals that have a C/C++ key | 0.10 s | 0.18 s | 0.17 s |
| 2000 functions, each with one such literal | 0.14 s | 0.34 s | 0.24 s |
| a flat chain of 500 terms `int32(f()) + int32(f()) + ...` | 0.10 s | 0.49 s | 0.17 s |
| the same, 1000 terms | 0.19 s | 2.5 s (43 s CPU) | 0.51 s (about 5 s CPU) |
| the same, 2000 terms | 0.51 s | 12.4 s (210 s CPU) | 1.6 s (about 12 s CPU) |
| 8000 functions with one such literal each | 0.4 s (dev refuses the key) | 2.1 s | 0.9 s |

Where it came from: the chain was cubic in the collector, not in the check. The HRESULT hook runs the collector with the calls
typed, so each of the N nodes of the chain became a reference, and the key of each was built from its leaves (N nodes of N
leaves of a few hundred bytes: 3.3 GB allocated for 1000 terms, all of it garbage for the collector's 192 GC workers). A reference of
an expression of more than 1000 source nodes now gets a key from its place instead of its text and is "expression too large",
as the ones with a key of more than 4000 characters always were (`maxRefNodes`, `TestTypedCheckOfAFlatChainAllocatesLittle`:
378 MB for 1000 terms). The other part was `nameAt` looking up each variable in all of `Defs` (quadratic in the variables of a package):
one reverse map now. What is left is the work itself: a map-literal value next to a C/C++ key is a `types.CheckExpr` of its own,
about 20 microseconds each (2000 of them are 40 ms of the 170 ms above), and the hook's analysis of the package. The language server's
keystroke benchmark does not move (p50 5.5 ms, p95 36 ms, dev the same).

## Known gaps and open questions

- **No nameable Go type (decided).** There is no `windows.HResult`: `windows` is already the package name of `<windows.h>`
  in the programs that use these calls, and `errors.As` with `interface{ HResult() int32 }` works everywhere. A named type
  can come later as a std package type with an explicit import (`syscall.HResult`-like), after the variable and map-key
  cases of the review round, which are done.
- **A typedef of `HRESULT`** (`typedef HRESULT MYRESULT`) is not detected; the clang sugar of a call keeps one name only.
  A header that does that gets numbers.
- **An `error` as the argument of a C/C++ function** that wants a number is C++'s error at the build (`cannot convert 'error'
  to 'int'`), not the check's: the check has no parameter type, and a Go slice there is the same. Not specific to HRESULT.
- **The editor's gaps**: valid programs show no diagnostic there, but the misuse errors of an HRESULT call (`hr != 0`,
  `var n int32 = f()`, `hr += 1`, `var n C.long = hr`, `hr = "x"`, `map[string]int{hr: 1}`) are shown by `gocpp check` and
  `gocpp build` only; see (d).
- **An elided literal of a C/C++ type next to a C/C++ key** (`map[any]hresult.Surface{k: {7}}`, `map[any][]hresult.NTSTATUS{k: {1, 2}}`,
  `[2]NTSTATUS`, `map[any]map[any]hresult.Surface{k: {k: {7}}}`, k a C/C++ value) is refused by `gocpp check`, the editor and the
  build alike: `a composite literal without its type ({…}) for a C/C++ type next to a C/C++ map key is not supported yet: write
  the type`. The lowering has no Go type to write the literal with. Spelled with the type (`hresult.Surface{7}`) it builds and runs.
  A Go struct that has a C/C++ field (`type W struct{ S hresult.Surface; N int }`, `map[any]W{k: {N: 1}}`) is a Go type and works:
  go/types gives its elided literal the type and only the field is invalid, so the value is checked like any other (a wrong value
  `{N: "x"}` is `cannot use "x" (untyped string constant) as int value in struct literal`), also as `*W`, `[]W`, `[2]W`,
  `map[string]W`, nested in another such struct, and two levels deep (`tests/typed/hresult_gostruct.gp` builds and runs it with both
  compilers and on Windows). The rule is on the element type itself: only an element type that is C/C++ in its own shape (a C/C++
  struct, a slice, array or pointer of C/C++ numbers) is refused for an elided literal; a Go type with an invalid part is not.
  An elided literal of **an instance of a generic Go type with a C/C++ type argument** (`map[any]Gen[hresult.NTSTATUS]{k: {N: 1}}`)
  gets the words of that limit, `an instance of a generic type with a C/C++ type argument (Gen[C.NTSTATUS]) is not supported yet:
  the type arguments of a generic type are Go types`, which are also the build's words for the full form
  `Gen[hresult.NTSTATUS]{...}` (`gocpp check` does not refuse the full form yet: check and build disagree there, a Phase 04 gap).
  A `type D hresult.Surface` is refused at its declaration (Phase 04). Not specific to HRESULT (any C/C++ key does it).
  An **anonymous** Go struct with a C/C++ field (`map[any]struct{ S hresult.Surface; N int }{k: {N: 1}}`, also behind a pointer, slice or
  array) is not a named Go type here: the elided form is refused by `check`, the editor and the build with the C/C++ literal message above,
  and the full form passes `check` and fails in the build (`not supported yet: this type with C/C++ parts`). It is the Phase 04 limit
  of anonymous types with C/C++ parts, the same on dev; an anonymous struct that holds a named Go struct with a C field works.
- **`%d` and `%#v` of the HRESULT error** show `{}` without the code (`%v` and `%s` show the message; the code is
  `HResult()`).
- **Function pointers in locals and `std::function`** stay the raw number (decided; see (a)).
- **Libraries of later phases.** `log` and `strings` take the error and its text (checked by hand against the gate of
  phase 15); the typed test suite runs below those phases, so `tests/typed` covers `errors` and `fmt`.
- **A `vet` check for a dropped HRESULT (decided: later).** A `gocpp vet` analyzer only (never `check`): statement calls
  only; `_ = f()`, `int32(f())` and `defer`/`go` are the explicit opt-outs. Not built in this ADR (a TODO of the report).
- **`NTSTATUS` (decided: later)** as a second row of the result table, with its success range and its own name table.
- **Windows PC.** The updated `demos/imgui-dx11` was cross-built and run on the real PC over ssh (no desktop session): the
  device creation failed there, and the new path printed `D3D11CreateDeviceAndSwapChain failed: HRESULT 0x887A0022`
  (`DXGI_ERROR_NOT_CURRENTLY_AVAILABLE`, which is why that name is in the table). The success path (the window, the frame loop,
  `Present`'s `DXGI_STATUS_OCCLUDED` through `int32(...)`) needs the console desktop and has not been run.
- **`-interop=opaque` vs typed** compile different programs for HRESULT-using code. Intended, and documented.
