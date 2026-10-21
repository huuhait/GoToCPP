# Reading the generated C++: `gocpp emit -annotate` and `gocpp explain`

gocpp compiles to readable C++, but readable is not the same as obvious. Why is `c` a `gp::ptr<Counter>`
and `x` a plain `int64_t`? Why does that `defer` become a `gp_df.gp_push_call(...)`? Where does the
program check for nil? Two commands answer these questions about *your* program, and a third
form answers them about gocpp's messages.

| Command | What it does |
|---|---|
| `gocpp emit -annotate file.gp` | prints the C++, each run of lines headed by the `.gp` line it comes from and the reasons for the lowering's choices |
| `gocpp explain TOPIC` | says what a mechanism is (`boxing`, `defer`, `select`...), with an example that the real compiler emits each time you ask |
| `gocpp explain file.gp:LINE` | the annotated C++ of one line or a range of lines |
| `gocpp explain "MESSAGE"` | what an error message means and what to do about it |
| `gocpp explain -list` | the topics (`-list -messages`: the messages that have an explanation) |

The annotated view is for *reading*: it is not C++ that compiles (the `#line` directives are replaced
by the headers), and the normal `gocpp emit` and `gocpp build` are not changed by it: the emitter
writes the same bytes with the reasons collected and without them (a test checks it for every example
and for every program of `tests/run` and `tests/golden`).

## `gocpp emit -annotate`

Take this program (`boxing.gp`, also the example of `gocpp explain boxing`):

```go
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() { c.n++ }

func newCounter() *Counter {
	c := Counter{n: 1} // its address is returned: boxed
	return &c
}

func main() {
	var local Counter // a pointer method is called on it: not boxed
	local.Inc()
	x := 41 // nothing sees x after the function: it stays on the stack
	x++
	p := newCounter()
	p.Inc()
	fmt.Println(local.n, x, p.n)
}
```

`gocpp emit -annotate boxing.gp` prints the files of the program as `gocpp emit` does (`//// path`
lines), and in each file, before each run of C++ lines, a header and the reasons. This is the part for
`newCounter` and `main` (lines 9 to 17 of the program):

<!-- gocpp: boxing explain boxing.gp:9-17 -->
```
// ---- ./boxing.gp:9: func newCounter() *Counter {
gp::ptr<Counter> newCounter() {

// ---- ./boxing.gp:10: c := Counter{n: 1} // its address is returned: boxed
// gp[boxing] col 2: c is boxed: its address (or the address of a part of it) is taken, so it lives
//     in a gp::ptr heap box that every pointer shares and that never dangles
    gp::ptr<Counter> c = gp::new_<Counter>(Counter{.n = 1});

// ---- ./boxing.gp:11: return &c
// gp[refcount] col 9: &c is the box of c itself: the pointer shares its reference count, so the
//     variable lives as long as any pointer to it
    return c;

// ---- ./boxing.gp:12: }
}

// ---- ./boxing.gp:14: func main() {
void main() {

// ---- ./boxing.gp:15: var local Counter // a pointer method is called on it: not boxed
// gp[boxing] col 6: local stays on the C++ stack: its address is taken, but the escape analysis
//     found that the address does not escape, so a pointer to it is gp::borrow(&local) or a
//     gp::stack_ptr value (no heap box, no counts)
    Counter local{};

// ---- ./boxing.gp:16: local.Inc()
// gp[refcount] col 2: lent to Inc as the raw address of a part of an object that stays alive during
//     the call: no aliasing pointer, no reference count (gp::borrow)
// gp[methods] col 2: pointer method Inc: a static call T::Inc(p, ...), the receiver pointer is the
//     first argument; no dynamic dispatch
    Counter::Inc(gp::borrow(&local));

// ---- ./boxing.gp:17: x := 41 // nothing sees x after the function: it stays on the stack
    int64_t x = 41;

// gocpp explain boxing, gocpp explain methods, gocpp explain refcount explains the mechanisms
```

Each header is `// ---- file.gp:LINE: <the line>`; the lines under it, up to the next header, are the C++ that this
line became. A line of the program that needs several chunks (a `select`, a `defer` of a literal) is
shown again with `(continued)`. The lines `// gp[kind] col N: reason` are the reasons: `kind` is the name of
a `gocpp explain` topic, `N` the column of the construct in the line. C++ that has no `.gp` line (the
prototypes, the type descriptors, `main`) has no header.

**Statements on several lines.** A reason about a continuation line (an argument on the third line of a
call, a field of a composite literal) is shown under the header of the line that starts the statement,
and says `line N`: `// gp[methods] line 14 col 3: ...`. `gocpp explain main.gp:14` shows that statement.

**Method values.** `f := t.Get` is not a call: it binds the receiver now, and its note says `method value`.

**fmt calls.** The note about how fmt takes its operands appears once for each fmt function in each
function (or generic instance), not for every call.

**Generic functions.** The instances of a generic function (`Show[int]`, `Show[Big]`) are written from the same
`.gp` lines, and what is true for one is not true for another: a captured `y` of type `Big` (512 bytes) is boxed,
and an `int` is copied into the lambda. So each instance's chunks have their own reasons, and a header says which
instance it is: `// ---- show.gp:8: var y T = x  [in gp_inst_4296dcb5_Show]`. The comment above each instance's C++
(`// Show[Big]`) and the `generics` note at the call say which `gp_inst_...` is which.

Flags:

- `-kinds=boxing,defer` keeps the reasons of those kinds only (the C++ stays).
- `-nochecks` writes the notes about the run-time checks as for `gocpp build -nochecks`.
- `-json` prints the same view as JSON (`files[].chunks[]` with `gp`, `reasons` and `cpp`).
- `-std` adds the standard library's packages (errors, fmt's `Errorf` wrappers...): the view is of the program's own
  packages unless you ask.
- `-o file` writes the view to a file.

## What the reasons say

The kinds, and the decision each one records:

- `boxing`: why a variable is a `gp::ptr` box (its address escapes, a closure that can outlive the call
  shares it, it is too big) and not a plain C++ variable, and why a variable whose address is taken
  still stays on the C++ stack (the escape analysis of
  [ADR-114](../adr/114-escape-analysis-keeps-non-escaping-objects-on-the-stack.md)).
- `closures`: a func literal is a lambda; each captured name is copied, shared or (for a literal that is only
  called where it is made) used by reference, and the note says which and why.
- `defer`, `recover`: the defer frame and the single exit of a function with `defer`; the ticket that a
  function that calls `recover` takes.
- `checks`, `panics`: the operations that check at run time (index, nil, division, type assertion...),
  and what `-nochecks` removes.
- `refcount`: where a pointer shares an owner, so that the variable lives as long as the last pointer.
- `interfaces`, `allocation`: a conversion to an interface, and the operations that allocate
  (`make`, `new`, `append`, a conversion that boxes). A `new(T)` or `&T{...}` that the escape analysis
  keeps on the stack says so.
- `generics`: which type arguments an instance `gp_inst_...` is for.
- `select`, `goroutines`, `channels`, `range`: the lowering of those statements.
- `order`: a `gp_t0` temporary that keeps Go's order of evaluation.
- `interop`: a call or a value of C or C++.
- `embedding`, `methods`, `names`, `strings`, `slices`, `maps`, `initialization`: the other mappings of
  `docs/spec/cpp-mapping.md`.

A note is made where the lowering decides, so it tells what *this* gocpp did. The reasons are
about readability, not a promise: the escape analysis keeps more variables on the stack as it learns
more, and the notes change with it. It does not analyse generic functions (their instances box as
before), which is why the example of `gocpp explain generics` below still boxes a captured variable.

## Why is it on the stack, or boxed? `-gp:m`

The escape analysis (ADR-114) decides for each variable whose address is taken, each `&T{...}` and `new(T)`, and
each func literal whether it can stay on the C++ stack. `gocpp build -gp:m` (also `run` and `emit`) prints the
decisions in the form of `go build -gcflags=-m`, on standard error, and `-gp:m=2` adds the reason of each
escape. For `boxing.gp`:

<!-- gocpp: boxing stderr emit -gp:m=2 boxing.gp -->
```
# command-line-arguments
./boxing.gp:7:7: c does not escape
./boxing.gp:10:2: moved to heap: c: it is returned
./boxing.gp:11:9: &c escapes to heap: it is returned
./boxing.gp:16:2: &local does not escape
```

`c` does not escape in `Inc` (the receiver is borrowed), and `local` stays on the stack, but `c` in `newCounter` is
returned, so it is moved to the heap. A variable that nothing but local closures capture (a literal called where it
is made) is not boxed either (a parameter or a result still is); a closure that is stored, returned, passed or that
calls `recover` is a `gp::func` that owns its captures, and a literal that `go` or `defer` runs directly is a lambda
with its captures copied into it. A variable is kept on the stack only when it is small: 2 KiB for
one object and 16 KiB for all of them in a function. `-gp:noescape` turns the analysis off, so that every variable whose
address is taken is boxed (`gp::lend(local)` in this example), which tells whether a bug is the analysis's.

## `gocpp explain TOPIC`

`gocpp explain defer` prints the text of the topic, the section of `docs/spec/` that has the details, the
related topics, and an example program with the annotated C++ that the compiler writes for it now. The
example is made when you ask: it is not a stored copy that can fall behind. (The examples of the topics of
a later phase, such as `select`, are emitted with the phase gates open, and the output says so: `gocpp build`
rejects such a program until its phase is built.) A topic has aliases: `gocpp explain lambda` is
`closures`. A topic can be named with several words (`gocpp explain multiple results`). A name that is no topic gets suggestions:

```
$ gocpp explain defr
gocpp explain: unknown topic "defr". Did you mean: defer?
Run 'gocpp explain -list' for the topics.
```

The topics that match the kinds of reasons (`boxing`... `initialization`) are what the notes of the view are about.
Eight more are reference pages for fixed shapes of the mapping, which have no note of their own: `types` (the type
table and why `int` and `int64` are one C++ type), `arrays`, `results` (multiple results are a `std::tuple`),
`complex`, `named` (named types over `gp::newtype`), `fmt` (fmt and the builtins), `files` (the generated files) and
`linemap` (`#line` and debugging). Each has an example like the others.

## `gocpp explain file.gp:LINE`

Takes one line or a range (`main.gp:12`, `main.gp:9-17`) and prints the chunks of the annotated view
that come from it, then the topics to read. This is the same as the lines of the full view; the file is
compiled like `gocpp emit file.gp` does, so its errors are the compiler's. The lines of a
declaration without C++ of its own (a comment, a blank line) say so. A file of a package that is not `main` is
explained in the program of its module (the package at the `gocpp.mod` above the file), so `gocpp explain
geom/geom.gp:30` works inside a module; `gocpp explain geom/geom.gp` without a line says that a line is needed.

<!-- gocpp: defer explain defer.gp:9-12 -->
```
// ---- ./defer.gp:9: defer func() {
// gp[defer] col 2: defer: the function and its arguments are evaluated now and stored in the frame;
//     the call runs when the function exits: a func literal: its lambda goes into the frame
//     (gp_push_call), so the stack trace names it func1, func2...
// gp[closures] col 8: func literal #1 is a C++ lambda (the stack trace names it func1); what it
//     captures is copied into the lambda (a box is shared), and a lambda that is used as a func
//     value is wrapped in gp::func, whose copies share the captured state
        gp_df.gp_push_call([fail, err]<int = 1>() mutable -> void {

// ---- ./defer.gp:10: if fail {
// gp[closures] col 6: captures fail by copy into the lambda: it never changes after the closure
//     exists and is at most 128 bytes
            if (fail) {

// ---- ./defer.gp:11: err = fmt.Errorf("work failed")
// gp[closures] col 4: captures err by sharing its box (it changes after capture)
// gp[interfaces] col 10: fmt.Errorf: the runtime's gp::fmt functions take each operand with the
//     descriptor of its Go type (gp::fmt::as), not boxed into an interface first (an operand of a
//     non-pointer type with an Error method is, see the allocation note)
                (*err) = gp::fmt::Errorf("work failed"_gs);

// ---- ./defer.gp:12: }
            }

// ---- ./defer.gp:9: defer func() {  (continued)
        );

// gocpp explain closures, gocpp explain defer, gocpp explain interfaces explains the mechanisms
```

(That is the closure of a `defer` in `defer.gp`, the example of `gocpp explain defer`.)

## `gocpp explain "MESSAGE"`

gocpp has no error codes: the text of a message is its name. Paste the message, with or without the
`file:line:col:` in front:

<!-- gocpp: defer explain ./main.gp:5:12: interfaces are not supported yet (Phase 05) -->
```
interfaces are not supported yet (Phase 05)

reported by the checker

The checker rejects interfaces because the phase that builds it, Phase 05 (interfaces and
errors), is not finished in this gocpp. Each phase of the plan adds a part of the language;
the checker keeps a program inside the part that is built, so that nobody gets broken C++ or
a wrong program, and the message names the phase that brings what is missing.

What the gate covers: Interface types, conversions to them, interface embedding.

What to do:
  Write the program without it, or wait for Phase 05. docs/spec/feature-matrix.md has the
  whole table of features and phases.

The mechanism: gocpp explain interfaces
Specification: docs/spec/feature-matrix.md
```

The messages that have an explanation are the "not supported yet (Phase NN)" and "not available yet
(Phase NN)" family (the phase gates of the checker, each one with what it covers; the messages of the
lowering, the loader and the driver), and the messages about the name `C` and about headers that are not
found. `gocpp explain -list -messages` lists them. A test scans the source of gocpp for messages of the
family and fails for one that has no explanation, so a new "not available yet" arrives with its entry.

`--json` is `-json` for every form.
