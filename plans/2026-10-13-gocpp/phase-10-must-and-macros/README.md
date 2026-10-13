# Phase 10: Beyond Go: `!` (must) & Macros

> Two features Go doesn't have. First, **`!` (must)**: write it on a function
> (`func loadConfig!(path string) Config`) and every error inside it that you don't take yourself
> is returned for you; write it on one call (`os.ReadFile!(path)`) and that call's error is
> returned. `!` only does something where you write it. Second, **Go-style macros**: declared
> with `macro Name(args) { ... }` and used with `#Name(...)` inside code or on top of a function
> (start style, or wrap style with `#body()`), plus `#Derive(...)` code generation for types. Both
> are compile-time rewrites, so they cost nothing at runtime and the language server understands
> them.

| | |
|---|---|
| Depends on | Phase 05 (errors, panics, single-exit function shape), Phase 06 (package summaries for macros from other packages), Phase 09 (`fmt`/`strconv` for `errors.Wrapf` and `#Derive`; `encoding/json` for `#Derive(JSON)`) |
| Unlocks | Phase 11 (editor support for `!` and `#`), nicer code everywhere after this |
| Size | medium-large (about 6–7 weeks: `!` ≈ 2.5–3, macros ≈ 2–2.5, `#Derive` ≈ 1–2) |
| Note | The core of part A (`!`) only needs Phase 05, so it can move earlier; `errors.Wrapf` then waits for `fmt` |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

**Errors.** Go's error handling is explicit, but noisy. The Go team spent years on it: the 2018
`check`/`handle` draft, the 2019 `try` proposal, the 2024–25 `?` proposal. In 2025 they
decided not to change Go's error-handling syntax at all. gocpp is its own language, so it can
ship what people keep asking for on Reddit: **must**, written `!`. Your decision (2026-10-13):
`!` works in two places and nowhere else.
- **On a function**, `func loadConfig!(...)`: inside it you skip the error handling, and every
  error you don't take yourself is returned automatically.
- **On one call**, `os.ReadFile!(path)`, inside any function: that call's error is returned.
- **No `!`**: plain Go, `cfg, err := loadConfig(path)`.

**Macros.** C macros are text substitution. They are unsafe (double evaluation, name clashes) and
invisible to tools. gocpp macros are **Go-style** instead. You declare one with
`macro Name(args) { ... }`: it looks like a func and is type-checked like one. You use it with
`#Name(...)` either **inside** code, or **on top of a function** in one of two styles (your
decision, 2026-10-13):
- **start style**: it runs at the start and, if it returns a `func()`, again at the end (tracing,
  timing, locking, checks);
- **wrap style**: it gets the function's own code as `#body()` and decides how to run it, for
  example "run this function under a VM", "on its own thread", "recover its panics", "only once".

Inside a macro, `#file()`, `#line()`, `#func()` and `#text(arg)` describe the place where it is
used, and `lazy` parameters only run when needed. `#Derive(String)` (enums) or `#Derive(JSON)`
(structs) on top of a type generates methods (like Go's `stringer` tool, but built in), and C/C++
macros stay fully usable through their header packages (`raylib.RAYWHITE`).

Every design here was checked with spikes on 2026-10-13 against the **stock** `go/types` (no fork
needed): must calls, must functions, start-style and wrap-style macros. The audit then tested
the edge cases. Results are in §2.1 and §2.2.

## 2. How it works

### 2.1 Part A: `!` (must)

```go
// Go today
func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

```go
// gocpp: ! on the function (a must function)
func loadConfig!(path string) Config {
	var cfg Config
	data := os.ReadFile(path)  // error → returned for you
	json.Unmarshal(data, &cfg) // error → returned for you
	if cfg.Port == 0 {
		return Config{}, errors.New("port is missing") // one extra value: fail on purpose
	}
	return cfg
}

// a normal function: ! on one call
func port(path string) (int, error) {
	cfg := loadConfig!(path) // error → returned for you
	return cfg.Port, nil
}

// no ! anywhere: plain Go
func portByHand(path string) (int, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return 0, err
	}
	return cfg.Port, nil
}

func main!() {
	p := port("app.json") // error → printed, exit 1
	fmt.Println("listening on", p)
}
```

#### Must functions: `func f!(…) T`

- **What is handled.** Inside a must function, every call whose `error` result you **drop** is
  handled for you: if the error isn't nil, the function returns it (the other results become
  zero values; defers still run). An error is dropped when the call is a statement
  (`json.Unmarshal(data, &cfg)`), or when its results are used as fewer values than it returns
  (`data := os.ReadFile(path)`, `n := strconv.Atoi(s) * 2`). If you take the error
  (`data, err := os.ReadFile(path)`, or `_ = f.Close()` to drop it on purpose), nothing changes.
- **The signature.** A must function gets an extra `error` result, so callers see
  `func loadConfig(path string) (Config, error)`. `return cfg` means success (`return cfg, nil`);
  `return cfg, err` (one extra value) returns an error on purpose. `func run!()` with no results
  becomes `func run() error`, and its `return` means `return nil`.
- **Declaring the error yourself.** If the last declared result is already `error`, nothing is
  added and the returns are normal Go. That's how you name it, for example to add context:
  `func loadConfig!(path string) (cfg Config, err error) { defer errors.Wrapf(&err, "load config %s", path); … }`.
- **`func main!()` and `func init!()`**: an error is printed as `<program>: <error>` (`<program>`
  is the base name of `os.Args[0]`) and the program exits with code 1 (decided, Q11). For
  `main!`, `main`'s defers run first; for `init!`, the program stops before `main` starts.
- **Methods** can be must functions too: `func (s *Server) Start!() { … }`. An interface spells the
  real signature (`Start() error`), and the `!` method satisfies it.
- **`defer f.Close()`** inside a must function: if `Close` fails while the function is returning
  a nil error, `Close`'s error is returned (the same as `defer f.Close!()` below). This holds for
  any deferred call whose last result is `error` (e.g. `defer fmt.Println(…)`), because a
  deferred call drops its results like a statement does.
- **Closures** inside a must function are separate functions, so nothing is handled inside them
  automatically; use `!` on calls there.
- **`go f()`**: the error can't come back from another thread, so it isn't handled; `gocpp vet`
  warns.
- **Every dropped error means every one**, including `fmt.Println(...)` statements: their error is
  returned too. That's harmless (writing to stdout rarely fails) and keeps the rule simple. The
  editor shows an inlay hint on each handled call (Phase 11), so nothing is invisible.

#### Must calls: `F!(…)`

`F!` accepts any named function or method whose **last result is exactly the type `error`**. The
whole Go-style stdlib works unchanged: `os.Open!(p)`, `strconv.Atoi!(s)`, `w.Write!(b)`,
`f.Close!()`, must functions (`loadConfig!(path)`), and generic functions with explicit type
arguments (`parse![int](s)`). A concrete error type as the last result (`*MyErr`,
`syscall.Errno`) is a compile error with a clear message, because Go's typed-nil rule would turn a
nil `*MyErr` into a non-nil `error` and make success look like failure (the audit showed exactly
this). The same rule holds inside must functions: such a call isn't handled automatically, and if
its error is dropped, the compiler asks you to take it yourself. `F!` gives back every result
except the error:

| `F` returns | Use `F!` as |
|---|---|
| `(T, error)` | `x := F!(...)`, or inline: `g(F!(...))` |
| `(A, B, error)` | `a, b := F!(...)` |
| `(A, B, C, error)` | `a, b, c := F!(...)` |
| `error` | a statement: `F!(...)` |

**What happens when the error is not nil** (for `F!`, and for handled calls inside a must
function, whose error result always exists):

| Where the call is | Behavior |
|---|---|
| a function whose last result is `error` (including every must function) | other results become zero values, the error result becomes `F`'s error, and the function returns. Defers still run, so they can wrap the error |
| `main.main` (`func main()` with `F!`, or `func main!()`) | defers in `main` run, then `<program>: <error>` is printed to stderr and the program exits with code 1 |
| a function with a `*testing.T`, `*testing.B` or `*testing.F` parameter and no error result | `t.Fatal(err)` (the test fails and stops, as a Go developer expects) |
| any other function with no error result (including `init` and package-level `var` initializers) | panic with the error. This is Go's `Must` convention (`regexp.MustCompile`, `template.Must`) |
| `defer f.Close!()` | if `Close` fails and the function was about to return a nil error, it returns `Close`'s error instead (the classic Go pattern, automated). In a function without an error result the failure panics; in `main` it is printed and exits 1; if a panic is already running, the `Close` error is dropped and the panic continues |
| inside a closure | propagates out of the closure (its own results), not the outer function |
| `go F!()` | compile error: nobody can receive the error |
| inside a `lazy` macro argument | compile error: the argument becomes a closure without an error result, so `!` would silently turn into a panic |

`go func() error { … F!() … }()` drops the error just like `go F!()` would; `gocpp vet` warns
about it. Not in v1: `!` on calls of arbitrary expressions (`fns[i]!()`, `fns![i]()`; write
`f := fns[i]; f!()`), `!` on function literals, and `!` on C functions (they don't return Go
errors).

**Adding context** uses a Go idiom (named result + `defer`) and a small helper in gocpp's
`errors` package. It works in a normal function with `F!` calls and in a must function that
declares its error:

```go
func loadConfig!(path string) (cfg Config, err error) {
	defer errors.Wrapf(&err, "load config %s", path)
	data := os.ReadFile(path)
	json.Unmarshal(data, &cfg)
	return cfg, nil
}

// error text: "load config app.json: open app.json: no such file or directory"
```

`errors.Wrapf` wraps like `fmt.Errorf` with `%w`, so `errors.Is(err, fs.ErrNotExist)` and
`errors.As` still find the original error.

**Error return traces (optional step).** The idea: each `!` that passes an error up records its
`.gp` call site, so `errors.Trace(err)` or `%+v` can show the path the error took, like Zig's
error return traces, without wrapping the error value (`err == io.EOF` must keep working). The
audit showed the hard parts: shared sentinel errors like `io.EOF`, errors that are handled and
dropped, and errors sent over channels. So this step starts with a short design note that defines
when a trace starts, how it is attached and when it is cleared. It is the first thing to move
after v1 if time is short.

#### How `!` is implemented (validated on 2026-10-13)

1. **Parser (patch #4 to the Phase 01 fork).** A **non-keyword** identifier immediately followed by
   `!` and then `(` (or `[` for explicit type arguments) is one name: `ReadFile!`. The same rule
   covers declarations: `func loadConfig!(`, `func (s *Server) Start!(`, `func parse![T any](`.
   Keywords are excluded on purpose: `if!(a && b)`, `return!(x)`, `case!(b):` and `for!(a)` are
   valid Go (the audit checked them with the stock parser) and must keep their meaning. `x != y`
   and `!ok` are unaffected, because `!=` is a different token and a prefix `!` never directly
   follows an identifier. The stock `go/printer` prints `os.ReadFile!(path)` unchanged (spike).
2. **Must functions, before `go/types`.** The signature gets its `error` result (unless it's
   already declared), and each `return` of the function itself (not of closures inside it) with
   exactly the declared number of values gets `nil` appended.
3. **Must calls, before `go/types`.** The arity (number of values) is chosen from the syntax:

   | Use | Rewritten to | Hidden helper (package `gocpp/must`) |
   |---|---|---|
   | `x := F!(a)`, or inside an expression | `gp_must.M1(F(a))` | `func M1[T any](v T, err error) T` |
   | `a, b := F!(a)` | `gp_must.M2(F(a))` | `func M2[A, B any](a A, b B, err error) (A, B)` |
   | `a, b, c := F!(a)` | `gp_must.M3(F(a))` | `func M3[A, B, C any](a A, b B, c C, err error) (A, B, C)` |
   | statement `F!(a)` | `gp_must.Any(F(a))` | `func Any(results ...any)` |

   Go lets `f(g())` pass all of `g`'s results to `f`, so stock `go/types` infers everything.
   Spike: `os.ReadFile!` → `[]byte`, `strconv.Atoi!` → `int`, `os.Create!` → `*os.File`. After
   type checking, a **post-check runs on every form**: the inner call's last result must be
   `types.Identical` to `error`. `M1` alone would accept any type *assignable* to `error`, such as
   a concrete `*MyErr`, which is exactly the typed-nil trap. The post-check also covers the
   statement form: `f.Write(data)` → `(n int, err error)` OK, `f.Close()` → `error` OK,
   `lookup()` → `(int, bool)` rejected.
4. **Finding dropped errors needs types, so must functions take two checks.** Whether
   `data := os.ReadFile(path)` drops an error depends on what `ReadFile` returns, which only
   `go/types` knows. So `go/types` checks the package once (inside must functions, a dropped error
   is still a type error at this point, e.g. "assignment mismatch"); then, using the callee
   signatures from that check, every call that drops its error is rewritten into the step-3 form;
   then `go/types` checks again, and that check must be clean. Only packages with must functions
   pay for the second check, and the editor caches it like every other step (Phase 11).
   **Spike** (`spikes/2026-10-13/must-functions`): the first check had 5 expected errors and the
   second 0; the generated program then ran correctly for a good config (exit 0), a config
   without a port (the explicit error, exit 1) and a missing file (`open missing.json: no such
   file or directory`, exit 1). The nested case `strconv.Atoi(s) * 2` and the multi-value
   forwarding case `f(g())` (all results passed on, so nothing is dropped) were both handled.
5. **Lowering (after type checking)** replaces each helper with real control flow, using the
   Phase 05 single-exit function shape (results live in `gp_r0`, `gp_r1`, …). No helper call is
   left in the C++:

   ```cpp
   auto gp_t1 = ::gpkg_os::ReadFile(path);
   if (std::get<1>(gp_t1) != nullptr) {     // error path (the type is exactly `error`)
       gp_r0.set(Config{});                  // zero value for the other result
       gp_r1.set(std::get<1>(gp_t1));        // the error result
       goto gp_exit;                         // defers run, then return
   }
   gp::slice<uint8_t> data = std::get<0>(gp_t1);
   ```

   (If a named error result is boxed, for example because `errors.Wrapf(&err, …)` takes its
   address, the lowering writes through the box.)
6. **Where the early return goes, per context.** "Move the call before the statement" is only
   right for plain statements. Each other context gets its own shape, with a differential test
   for each:
   - **inside an expression** (`g(h(), F!(a))`): every call to the *left* is moved first, in Go's
     order, then `F!(a)`, so `h()` still runs before `F`;
   - **loop condition or post statement** (`for i < n!(x); i = next!(i)`): the loop becomes
     `for { …; if !cond { break }; body; post }`, so the call runs on **every** iteration, and
     `continue` jumps to the post statement;
   - **`else if F!(x)`**: moved into the `else` block, so it only runs when the first branch is
     not taken;
   - **`case F!(x):`**: switches already compute the matching case lazily, in source order
     (Phase 02), so each case's `!` runs only when that case is tried;
   - **`&&` / `||` right side**: the Phase 03 `if` chain, so short-circuiting is kept.
7. **`defer F!(…)`** (and `defer f.Close()` inside a must function) is lowered to a deferred
   closure whose receiver and arguments are evaluated at the `defer` statement (Go's rule) and
   whose call runs at exit:
   `{ auto gp_r = f; gp_df.push([gp_r]{ gp::must::defer_err(gp_r0_err, ::gpkg_os::File::Close(gp_r)); }); }`.
   A plain `defer gp_must.Any(f.Close())` would be wrong: deferred call arguments are evaluated
   immediately, so `Close` would run at the `defer` line (the audit saw it run before the body).
8. **Friendly errors.** `go/types` messages that mention the hidden helpers are rewritten, e.g.
   `strconv.Itoa!(n): Itoa does not return an error, so "!" can't be used`,
   `os.Remove!(p) returns only an error; use it as a statement`, or
   `F!: the last result of F is *MyErr, not error; convert it explicitly`.

### 2.2 Part B: macros

**Declaring a macro** looks like declaring a function, with the keyword `macro`:

```go
package util

import (
	"fmt"
	"os"
	"sync"
)

// Inside a macro, #file(), #line(), #func() and #text(arg) describe the place where the macro is USED.

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	fmt.Fprintf(os.Stderr, "[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}
```

- `lazy string` means the argument is **not** evaluated at the call. The macro calls `msg()` when
  (and only if) it needs the value. Other parameters are evaluated once, like normal function
  arguments, so there are no C-style double-evaluation surprises.
- A macro can be generic (`Dbg[T any]`) and can return values.

**Using a macro: `#Name(...)`, in exactly three places**

```go
#util.Trace() // on top of a func: runs at the start; the func() it returns runs at the end
func loadConfig!(path string) Config {
	#util.Assert(path != "", "path is empty") // inside: as a statement
	data := os.ReadFile(path)
	return parse(data)
}

#util.Locked(&s.mu) // on top of a method: lock on entry, unlock on exit
func (s *Store) Put(k string, v int) {
	s.m[k] = v
}

#Derive(String) // on top of a type: generate methods
type Color int

func main!() {
	cfg := loadConfig("app.json")
	port := #util.Dbg(cfg.Port) // inside: as an expression; also prints "[file:line] cfg.Port = 8080"
	fmt.Println("listening on", port)
}
```

| Where | What the compiler does |
|---|---|
| **inside** a function (statement or expression) | replaces it with a normal call that also gets the use site: `util.Assert(&gp_site_2, path != "", func() string { return "path is empty" })` |
| **on top of a func/method**, start style (package level only) | inserts the call as the function's **first statement**. If the macro returns `func()`, it becomes `if f := util.Trace(&gp_site_1); f != nil { defer f() }`, so the macro runs code at the start **and** at the end, even when the function panics, and a nil result is simply skipped. Its arguments are evaluated inside the function, so they can use the parameters and receiver (`&s.mu`, `id > 0`) |
| **on top of a func/method**, wrap style (the macro uses `#body()`) | moves the function's code into a closure and hands it to the macro, which runs it where `#body()` is (next section) |
| **on top of a type** (package level only) | `#Derive(...)` runs built-in code generators that add methods next to the type (user-written type macros come later, see below) |

Several macros on top of one func apply top to bottom: the first one listed is the outermost,
like stacked decorators, and start style and wrap style mix freely. A doc comment goes **above**
the `#` lines, so `go/doc` still attaches it to the function. Inside a function body,
`#Name(...)` is always a use inside code.

**Spike (2026-10-13), start style.** The plain Go that this rewrite produces was run with `-race`:

```
enter main.loadConfig
exit  main.loadConfig
config: port=8080
store size: 10                      ← 100 concurrent Put calls, race detector clean
enter main.loadConfig
exit  main.loadConfig               ← the exit part still ran when loadConfig panicked
recovered: main.gp:5: assertion failed: path != "": path is empty
```

An earlier spike showed the generic `Dbg` returning a typed `int`, and a lazy message being
evaluated **0 times** while its assert passed. The exit part of an on-top macro may call
`recover()`; that works because Phase 05 gives every deferred function value a deferred entry.

**Wrap style: `#body()`.** A macro used on top of a function can take the function's own code and
decide how to run it: when, how often, on which thread, under which context. It calls the
built-in `#body()` where that code should run:

```go
macro VM() {
	vmEnter(#func())
	defer vmExit(#func())
	#body() // the function's own code runs here
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}
```

```go
#util.Trace() // start style: runs first; its func() runs last
#util.VM()    // wrap style: the code below runs inside VM
func secret(key string) (int, error) {
	if key == "" {
		return 0, errors.New("empty key")
	}
	return len(key) * 42, nil
}
```

- A macro whose body uses `#body()` is a **wrap macro** and can only be used on top of a function.
  `#body()` runs the function's code; a `return` in that code sets the function's results and ends
  `#body()`.
- The macro decides how the code runs: once (`VM`), maybe never (`Once` on later calls: the
  results stay zero values), on another thread (`InThread`, which waits for it), several times
  (a retry loop), or inside `recover` (`Recover`: a panic stops there and the function returns
  zero values).
- `defer`s in the function's code run when that code finishes, that is when `#body()` returns,
  before the macro continues. A macro that starts `#body()` on another thread must wait for it
  before returning (as `InThread` does); `gocpp vet` warns when `#body()` runs in a `go` statement
  that nothing waits for.
- **How it compiles.** The function's code moves into a closure; each `return x, y` in it becomes
  "set the results, then return" (unnamed results get hidden names). The macro gets the closure as
  a hidden `gp_body func()` parameter, and `#body()` calls it. In C++ the closure is a lambda
  passed straight into the macro's inline template, like a `lazy` argument, so nothing is
  allocated.
- **v1 limit:** `#body()` gives the macro no access to the function's results, so a `#Retry(3)`
  that looks at the returned error isn't possible yet. Planned next: `err := #body()` for
  functions whose last result is `error`.

**Spike (2026-10-13), wrap style** (`spikes/2026-10-13/macros-v3-wrap`, run with `-race`):

```
enter main.secret
  vm enter: main.secret
  secret: body defer          ← the code's own defer runs when #body() returns
  vm exit:  main.secret
exit  main.secret
126 <nil>
...
recovered in main.risky: runtime error: integer divide by zero
risky(0) = 0                  ← Recover: the panic stops, the result is the zero value
[job 0 done job 1 done job 2 done job 3 done]   ← InThread from 4 threads, race detector clean
setup: 1 0 calls: 1           ← Once: the second call skips the code
```

**Built-in macros**

| Macro | Gives |
|---|---|
| `#file()`, `#line()`, `#func()` | where it is written; inside a macro body, the **outermost** use site (your code). `#file()` is the path relative to the module root (`util/util.gp`), or just the file name (`main.gp`) for a program without `gocpp.mod` |
| `#text(x)` | the source text of `x`; for a macro parameter, the text the caller wrote (`"path != \"\""`) |
| `#body()` | inside a macro used on top of a function: runs that function's code (wrap style) |
| `#embed("logo.png")` | a string constant with the file's bytes (Go's `//go:embed` works too) |
| `#Derive(...)` with `String`, `Equal`, `Clone` or `JSON` | on top of a type: generated methods (table below; `String` is for enums, `JSON` for structs) |

**Nested macros keep both sites.** If macro `Check(v int)` uses `#Assert(v > 0, …)`, then inside
`Assert` `#text(cond)` must be `"v > 0"` (the inner call's text) while `#file()`/`#line()` should
point at the user's call of `Check`. One static object can't hold both, so `macro.Site` has an
`Outer *Site` field: `#file()`, `#line()` and `#func()` walk to the outermost site, and `#text()`
reads the innermost one.

**How macros work (no `go/types` fork needed)**
- **Parser (patches #5 and #6):** `#Name` is one token (`#` is illegal in Go, so there's no
  ambiguity). At package level, `macro Name(` starts a macro declaration. Inside functions `macro`
  stays an ordinary identifier, so old code that uses `macro` as a variable name still works.
  `lazy T` is a modifier in macro parameter lists (a macro parameter can't be *named* `lazy`, so
  `macro M(lazy string)` is a clear error, not a puzzle). `#Name(...)` lines directly above a
  package-level `func` or `type` attach to it.
- **AST:** a macro is stored as a normal `*ast.FuncDecl`, marked "macro" in a side table. On-top
  macros are recorded per declaration. `go/types` only ever sees plain Go code.
- **Rewrite before `go/types`:**
  1. every macro gets a hidden first parameter `gp_site *gp_macro.Site` (the hidden import is
     aliased `gp_macro`, so a user's own package-level `macro` name can't clash); `#file()`,
     `#line()`, `#func()` and `#text(param)` in its body read that site; `lazy T` becomes
     `func() T`; a wrap macro also gets a last hidden parameter `gp_body func()`, and `#body()`
     becomes `gp_body()`;
  2. every use `#util.Dbg(a * 2)` becomes `util.Dbg(&gp_site_N, a*2)`, with one static
     `macro.Site{File, Line, Func, Args, Outer}` per use (zero runtime cost); lazy arguments are
     wrapped as `func() T { return <argument> }`;
  3. a start-style on-top macro becomes the function's first statement, and a wrap-style one
     becomes `util.VM(&gp_site_N, func() { <the function's code> })` (§ table above).
- **C++ output:** macros are emitted as **inline templates in the package header**, and each
  `lazy` parameter (and `gp_body`) becomes a template parameter that takes the lambda directly.
  So the C++ compiler inlines macros **across packages** (no LTO needed), and a lazy argument
  never goes through `gp::func` (`std::function` may allocate). A benchmark checks that an
  `#Assert` that passes costs the same as an `if`.
- **Printer fork:** the stock `go/printer` can't print `macro`, `lazy` or on-top macros, so this
  phase forks `go/printer` too (small patch, same patch-file + re-fork procedure as the parser),
  and `gocpp fmt` switches to it (keeping gofmt's number-literal normalization, Phase 01).
- **A macro is a real function**, so it is type-checked once where it's defined (errors appear in
  the macro, not at every use), hygienic for free (its variables can't clash with yours), and
  visible to the language server (completion inside it, go-to-definition from `#util.Dbg`).
- **Rules (v1)**, each checked with a clear error:
  - `macro` declarations only at package level;
  - lazy parameter types must be concrete and spellable at every use site: predeclared or
    exported types, not `lazy T` and not an unexported type of another package;
  - a macro is only called with `#`: a plain call `util.Dbg(x)` or a macro used as a value
    (`f := util.Dbg`) is an error, because it would expose the hidden site parameter;
  - a macro that returns `func()` can't be used as a statement inside code (`#util.Locked(&mu)` on
    its own line would drop the unlock); use it on top of a function, or call the result;
  - `#body()` only in macros, and a macro that uses it only on top of a function;
  - a macro can't `return`/`break` out of the code that uses it (use `!` for errors);
  - a `defer` inside a macro body runs when the **macro** returns, not when the caller returns
    (the macro is a function); this is documented, and the on-top `func()` form is the way to run
    code at the caller's exit;
  - on types, only the built-in `#Derive` exists in v1.
- **Why not C-style text macros:** they break type checking and the editor, evaluate arguments
  twice, and capture names. C macros remain available for C APIs through their header
  packages.
- **Later (backlog):** `err := #body()` for wrap macros, user-written macros on types (generators
  that run at compile time and add methods, like `#Derive` but your own), and package-level
  macros that generate declarations.

**`#Derive`** generates methods next to a type:

```go
#Derive(String)
type Color int

const (
	Red Color = iota
	Green
	Blue
)
```

generates (`gocpp expand` shows it):

```go
func (c Color) String() string {
	switch c {
	case Red:
		return "Red"
	case Green:
		return "Green"
	case Blue:
		return "Blue"
	}
	return "Color(" + strconv.Itoa(int(c)) + ")"
}
```

**`#Derive` runs in two steps**, because generators need type information that only exists
after type checking (the constants of `Color` may be in other files, two constants can share one
value, and `Equal`/`Clone`/`JSON` need the kind of every field):
1. **before `go/types`**: only the method *stubs* are added. Their signatures follow from the
   derive name (`String() string`, `Equal(T) bool`, `MarshalJSON() ([]byte, error)`), so code that
   calls `c.String()` type-checks;
2. **after `go/types`**: the bodies are generated from `types.Info`. Constants that share a value
   use the first name in declaration order, as Go's `stringer` does. A name-based switch over
   `const (Debug Level = iota - 1; Info; Warn; Default = Info)` would not even compile
   (`duplicate case`), which the audit reproduced.

| Derive | Generates |
|---|---|
| `String` | `String()` for constant enums (same output style as Go's `stringer`) |
| `Equal` | `Equal(other T) bool`, deep equality (also compares slices and maps) |
| `Clone` | `Clone() T`, deep copy (reference cycles are not supported; documented) |
| `JSON` | for **structs**: `MarshalJSON`/`UnmarshalJSON` from struct tags (`json:"name,omitempty"`), reflection-free; `encoding/json` (Phase 09) picks them up through its `Marshaler` interface. On a non-struct type it is an error (`encoding/json` would print an enum's number, which surprises people; derive `String` and implement `MarshalText` instead) |

`Clone` and `JSON` are the first derives to move after v1 if time is short.

### 2.3 Where this sits in the pipeline

```
parse (patches 1–6) → load → expand: macro decls, #uses (inside, on top: start + wrap style),
     #Derive stubs → rewrite F!() and must-function signatures → go/types
     → must functions: rewrite dropped errors, go/types again → #Derive bodies
     → opaque pass → lower (! per-context shapes) → C++
```

Expansion is cached per file, but the cache key includes everything the file depends on: the
**package summaries** (Phase 06) of every imported package that declares macros or must functions
(signatures, which parameters are `lazy`, which macros use `#body()`), the same package's macro
and constant declarations, and the bytes of every `#embed`ded file. So making a macro parameter
`lazy` in `util` correctly re-expands every caller. Errors inside generated or expanded code point
at the call site **and** at the macro or derive that produced it.

### 2.4 Tooling

- `gocpp expand file.gp` prints the code after macro, derive and `!` rewriting.
- In the editor: `!`, `#Name`, `#body()` and `macro` get their own highlighting; inside a must
  function, an **inlay hint** marks every call whose error is handled automatically; hover on a
  must function shows its real signature (`func loadConfig(path string) (Config, error)`); hover
  on `F!` says what happens on error *here* (return / panic / exit 1 / `t.Fatal`); hover on
  `#util.Dbg` shows the macro, and on an on-top macro it shows what it inserts or wraps. Code
  actions: "add `!`" where an error result is ignored, "make this a `!` function", "expand `!`
  into an explicit `if err != nil`", "expand macro". All of this is Phase 11.
- `gocpp vet` (Phase 13) warns about `!` that would **panic** inside a library package (libraries
  should return errors), about `go func() error { … F!() … }()` and `go f()` in must functions,
  about `#body()` in a `go` statement nobody waits for, and about lazy macro parameters that are
  never used.

## 3. Steps

1. **10.1 Parser + printer**: patch #4 (`name!(` / `name![` for non-keyword identifiers, in calls
   and in `func` declarations), #5 (`#Name`), #6 (`macro` declarations, `lazy T`, package-level
   on-top `#Name(...)`), the `go/printer` fork (`gocpp fmt` switches to it), tests incl.
   `if!(a)`/`return!(x)`, round-trip.
2. **10.2 Hidden packages** `gocpp/must` (`M1`, `M2`, `M3`, `Any`) and `gocpp/macro` (`Site` with
   `Outer`), imported as `gp_must` / `gp_macro`.
3. **10.3 `F!` call rewrite** (arity from syntax) + post-check on every form (`Identical` to
   `error`) + friendly error messages.
4. **10.4 Must functions**: signature and `return` rewrite; the two-check detection of dropped
   errors; the declared-error form; `main!`/`init!`; methods; `defer` inside; closures and `go`.
5. **10.5 `!` lowering**: propagate / panic / `main` exit 1 / `t.Fatal` in tests / package-level
   initializers; the per-context shapes (left-hoisting, loop condition and post, `else if`, `case`,
   `&&`/`||`); the `defer F!()` closure; closures; reject `go F!()` and `!` in lazy arguments.
6. **10.6 `errors.Wrapf(&err, ...)`** helper (a gocpp extension in the `errors` package).
7. **10.7 (optional) Error return traces**: design note first, then `errors.Trace` / `%+v`.
8. **10.8 Built-in macros** (`#file`, `#line`, `#func`, `#text`, `#body`, `#embed`) + `//go:embed`.
9. **10.9 User macros**: `macro` declarations → inline header templates with the hidden site
   parameter; uses inside code → calls with one static `Site` each; `lazy` → template callables;
   on-top macros: start style (first statement, nil-safe `defer` form) and wrap style (the code
   moved into a closure, returns rewritten, `gp_body` as a template callable); nested sites via
   `Outer`; macros from other packages via package summaries; the v1 rules with clear errors.
10. **10.10 `#Derive(...)`** in two steps (stubs before `go/types`, bodies after): `String`,
    `Equal`, then `Clone` and struct-only `JSON` (both may move after v1).
11. **10.11 `gocpp expand`.**
12. **10.12 Lift the gates; update the spec** (`docs/spec/language.md`, divergences D12, D13,
    D17, D18; feature matrix).
13. **10.13 Tests**: run + error tests for every rule in §2.1 and §2.2, and for each lowering
    context. **Differential tests:** `!` is pure sugar, so each `!` test program (must calls and
    must functions) has a hand-expanded `if err != nil` twin that is valid Go (its
    `errors.Wrapf` is a small Go function written in the twin), and each wrap-macro test has its
    closure-based twin, like the spikes; the gocpp output must equal real `go run` output of the
    twin. Plus golden tests showing no helper calls remain in the C++, and benchmarks proving
    `!`, a passing `#Assert` and a wrap macro cost the same as hand-written code, also across
    packages.

## 4. Deliverables

- Parser patches #4–#6 + the `go/printer` fork, the `!` rewrite (calls and must functions) +
  lowering, the macro expander (start and wrap style), `#Derive` generators, `gocpp expand`
- Packages `gocpp/must`, `gocpp/macro`, `errors.Wrapf`
- Target programs `must-errors` and `macros` passing

## 5. Exit criteria

- `must-errors` and `macros` examples pass on GCC and Clang.
- Every rule in §2.1 has run and error tests; every lowering context (expression, loop condition
  and post, `else if`, `case`, `&&`/`||`, `defer`) has a differential test; every `!` program
  (must calls and must functions) prints the same output as its hand-written Go twin under real
  `go run`.
- The must-function spike's three runs (good config, missing port, missing file) pass as gocpp
  programs.
- The golden C++ for `!` contains no helper calls, and the benchmarks show no overhead versus
  hand-written code (also for macros used across packages).
- Macro definitions are type-checked at their definition, and errors in expansions point at
  both the call site and the macro.
- On-top macros work like the spikes: a `Trace`-style macro runs its exit part even when the
  function panics; a `Locked`-style macro passes a concurrent test under the race detector; the
  wrap macros `VM`, `Recover`, `InThread` and `Once` behave as in the wrap spike, race-clean.
- `gocpp fmt` (with the printer fork) round-trips `macro`, `lazy`, `func f!()` and on-top macros
  unchanged.
- `#Derive(String)` handles constants that share a value; `#Derive(JSON)` output matches
  `encoding/json` for the same structs (differential test).

## 6. Risks

| Risk | Mitigation |
|---|---|
| `!` hides control flow | `!` is written on every must function and must call, so it's visible where it acts; inlay hints mark each handled call inside a must function; hover explains it; vet flags panicking `!` in libraries |
| Arity guessed from syntax is wrong | `go/types` checks the value forms, the post-check covers every form, and errors are translated to clear messages |
| The second type check of must functions costs time | Only packages with must functions pay it; it's cached per package like every other step; the editor budget is measured with must functions in the reference projects |
| A lowering context is missed and `!` runs at the wrong time | One written shape and one differential test per context (§2.1, point 6) |
| A wrap macro returns before the code it started has finished | Documented rule; the `InThread` pattern waits; `gocpp vet` warns about `#body()` in an unwaited `go` |
| People want Rust-style `macro_rules!` | v1 is deliberately small (functions + call site + lazy arguments + on-top hooks and wraps); compile-time generators are in the backlog with a written design first |
| Too many ways to use macros | Exactly three places in v1: inside code, on top of a func (start or wrap style), `#Derive` on top of a type |
| Forking `go/printer` | Small patch kept as a patch file with the same re-fork script as the parser |
| 6–7 weeks slips | Error traces, `Clone` and `JSON` derive are the first things to move after v1 |
