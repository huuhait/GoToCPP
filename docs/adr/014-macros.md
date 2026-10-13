# ADR-014: Macros are `macro` declarations used as `#Name(...)`

Status: accepted (2026-10-13)

## Context

C macros are text substitution: they evaluate arguments twice, capture names, and are invisible
to tools. gocpp wants macro power without breaking types, gofmt or the editor. The user chose
the style ([Q12](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)) and both
on-top styles ([Q13](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

- **Declare** a macro at package level: `macro Name(args) { ... }`. It looks like a func, is
  type-checked once where it is defined, and can be generic and return values.
- **Inside** a macro, `#file()`, `#line()`, `#func()` and `#text(x)` describe where it is used.
  A `lazy T` parameter runs only when the macro calls it; other parameters are evaluated once.
- **Use** it as `#Name(...)` in exactly three places:
  1. inside code, as a statement or an expression;
  2. on top of a func, in **start style** (it runs at the start, and a `func()` it returns runs
     at the end, even on a panic) or **wrap style** (the macro calls `#body()` where the
     function's own code should run);
  3. on top of a type. In v1 that is only the built-in `#Derive`: `String` (constant enums),
     `Equal`, `Clone` and `JSON` (structs only).
- **Compiled as functions** with a hidden call-site parameter: `lazy T` becomes `func() T`, and
  wrap style passes the function's code as a closure. In C++ they are inline templates in the
  package header, so they inline across packages. Parser patches #5 and #6 and a small
  `go/printer` fork add the syntax; elsewhere `macro` and `lazy` stay plain identifiers (D18).

```go
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() }) // #body() is the function's own code
}

#util.Trace()     // start style: prints "enter" at the start and "exit" at the end
#Once(&setupOnce) // wrap style: the code below runs only on the first call
func setup(path string) {
	#Assert(path != "", "path is empty") // inside code; msg runs only if the assert fails
	load(path)
}
```

## Alternatives

- **C-style text macros** (rejected): unsafe and invisible to the type checker and the editor.
  C macros stay usable for C APIs through header packages (`raylib.RAYWHITE`).
- **Rust-style `macro_rules!`** (rejected): its own syntax breaks gofmt and the editor.

## Consequences

Good: hygienic for free (a macro's variables can't clash with yours); errors appear in the
macro, not at every use; no runtime cost (a passing `#Assert` costs the same as an `if`); the
language server understands macros (completion, go-to-definition, "expand macro").

Bad (v1 limits):
- `#body()` gives no access to the function's results, so a `#Retry(3)` that looks at the error
  isn't possible yet; `err := #body()` is planned right after v1.
- No user-written type macros or code generators yet (backlog).
- A macro can't `return` or `break` out of its caller, its `defer`s run when the macro returns,
  and it is only called with `#` (never as a plain call or a value).

## Evidence

- Spikes 2026-10-13: a lazy message ran 0 times while its assert passed
  ([v1](../../spikes/2026-10-13/macros-v1-functions/)); under `-race`, `#Trace` printed "exit"
  also on a panic and `#Locked` was race-clean with 100 concurrent calls
  ([v2](../../spikes/2026-10-13/macros-v2-keyword-and-on-top/)); the wrap macros `#VM`,
  `#Recover`, `#InThread` and `#Once` behaved as designed ([v3](../../spikes/2026-10-13/macros-v3-wrap/)).

See: [Phase 10 §1](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#1-idea), [§2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#22-part-b-macros), [§2.3](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md#23-where-this-sits-in-the-pipeline), divergences [D13, D18](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
