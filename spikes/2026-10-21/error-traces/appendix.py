#!/usr/bin/env python3
# Writes the appendix of ADR-124 (the scenario programs with their output) from scenarios/ and expected/.
# Usage: python3 -I appendix.py > appendix.md   (the ADR holds the result after its "## Appendix" line)
import os

here = os.path.dirname(os.path.abspath(__file__))

SCENARIOS = [
    ("chain", "A fresh error through two must functions",
     "The hops are the lines of the two `!` that handled the error. The call in `main` is plain Go, so "
     "it records nothing, and the second call has no error and no trace."),
    ("shared_value", "One value, two propagations",
     "`ErrNotFound` is returned by `lookup` for both requests. Request 1 goes `serve`, `fetch`, `lookup`; "
     "request 2 goes `outer`, `direct`, `lookup`. The first `!` of request 2 is in `direct`, which is "
     "deeper than where request 1 ended (`serve`): the frame test restarts the trace. `err == ErrNotFound` "
     "is true both times: the register never touches the value."),
    ("siblings", "Sibling paths of one value (the layout does not matter)",
     "`a2` and `b1` are called from `main` and fail with the same `ErrX`. Whether the frame of `b1`'s "
     "hop is older than the frame of `a2`'s hop depends on the frame sizes of the two functions, which "
     "change with the optimization level. The test that decides is the link between the two hops: "
     "`a2`'s hop returns into `main`, `b1`'s hop is in `b1`, so the trace of path 2 is `b1` alone, "
     "whatever the frames look like."),
    ("replaced", "Handled, dropped and replaced",
     "Two fresh errors; the second is made while the program still holds the first. The last printed "
     "line says they are two different objects with the same text."),
    ("wrapped", "Wrapping: `Wrapf`, `%w`, `Join`",
     "`load` has `defer errors.Wrapf(&err, \"load %s\", path)` and calls `open!`, which returns the "
     "sentinel `ErrPerm`; `boot` calls `load!`. `Wrapf` replaces the error during `load`'s exit and takes "
     "over the identity of the register, so `boot`'s `!` continues the trace through the wrapper (the "
     "wrapper is held by the named-result cell of `load`). `errors.Is(err, ErrPerm)` is true and `err == ErrPerm` is "
     "false, which is Go's rule for wrapped errors, unchanged. `fmt.Errorf(\"%w\")` and `errors.Join` do "
     "the same: the error that is returned has the trace, and the one it wraps does not."),
    ("foreign", "A wrapper the runtime did not make; no user code in a hop; a typed nil",
     "`decorate` wraps the error of `step` in a user type that has `Unwrap`. The runtime does not own that "
     "type, so the next `!` (in `run`) sees an error it cannot match, and the trace restarts there: `step`'s "
     "hop is gone from it. The counter shows that no hop called `Unwrap`. `errors.As` still works. The last "
     "part passes a typed nil pointer through `!`: it is a non-nil `error` without an identity, nothing "
     "is claimed for it, and its `Error` method is never called."),
    ("passthrough", "A function without `!` in the middle",
     "`adapter` returns the error of `lower` without a `!`. The hop of `upper` is not made in the function "
     "that `lower`'s hop returned into (`adapter`), so the link is cut and the trace begins at `upper`: the origin "
     "side (`lower`) is gone. With `!` in `adapterBang` the chain is complete. This is the `-O0` output; at `-O2` "
     "the call in `adapter` is a tail call, `lower` returns into `upper`, and the trace is `lower, upper`."),
    ("closure_defer_recover", "Deferred `Close!`, closures, panic and recover",
     "`use`: the deferred `c.Close!()` becomes the function's error and records its hop in the deferred "
     "closure; the site is named after `use` and the closure passes the return address that `use` saved when "
     "it started, so `useUp`'s hop links to it. `parseAll` is a closure: its `!` leaves through the "
     "closure's result. `mustStep` has no error result, so its `!` records the hop and panics; `safe` recovers. "
     "Inside the handler the trace is complete. After `safe` has returned the error, `outer`'s `!` is not in "
     "the function that `mustStep`'s hop returned into (`safe`), so the trace begins at `outer`: read the "
     "trace in the handler if the whole path matters."),
    ("stored", "Stored in a struct, returned later",
     "`task.run` stores the error; `finish` returns it without a `!`; `report!` passes it up. The hop in "
     "`step` is not linked to `report`'s, so the trace of the late return is `report` alone, with or "
     "without other errors in between. `runTraced` stores `errors.Traced(err)` instead: the snapshot "
     "keeps the history."),
    ("goroutines", "Goroutines and channels",
     "Two workers run `job(\"\")`, which fails through `step` and `job`. One sends the error as it is, the "
     "other sends `errors.Traced(err)`. Then `report!` in the receiving goroutine passes the traced error "
     "up. `Traced` of a traced error and `Traced` read in the goroutine that made it show each hop once: the "
     "wrapper takes the hops, the register is empty afterwards."),
    ("limits", "Limits: earlier hops in front, and the ring",
     "`caller` takes the error of `inner` (a hop is recorded and the error is dropped), then passes the same "
     "sentinel up again through `plainPass!`, a callee that records no hop. `inner`'s hop returned into `caller` "
     "and the new hop is in `caller`, so the link holds and `inner`'s hop stays in the trace. The same happens "
     "across calls: `handle(-1, false)` calls `lookupB`, which hops and returns the sentinel into `handle`; "
     "`handle(-1, true)` puts the sentinel up with its own `!`, and the register cannot tell the two calls of "
     "`handle` apart. This is the one shape in which a trace has hops of an earlier propagation. Then a recursion "
     "of depth 40 fails: 41 hops, of which the first 16, a gap and the last 16 are kept (33 frames, the gap at index 16)."),
    ("macros", "Names in macros (not simulated)",
     "A `!` in the body of a value macro (`Parse`) is in the macro function, at the line of the macro text, and the "
     "site is named `main.Parse`; it has no error result, so it panics, and the handler reads the trace. A `!` in a "
     "closure in a function with a wrap macro (`Around`) is named as the source numbers it, `main.viaWrap.func1`. "
     "Both names are static and agree with the file and line next to them. A panic trace of the same code prints "
     "`main.Parse` at the same line, but `main.viaWrap.func1.1.1` for the closure (the wrap macro's body runs in two "
     "lowered closures), and `#func()` in a value macro says the function of the use; the hop uses neither. Checked "
     "with real gocpp programs: the panic traces, and `#func()` printed in the same places."),
    ("logwrap", "COST: a wrapper made only for a log line",
     "`step`'s deferred function makes `fmt.Errorf(\"step failed: %w\", err)` only to log it, and the real error goes "
     "up. The wrapper is a wrapper of the error in flight, so it takes the identity; `top`'s `!` sees the real error, "
     "which no longer matches, and the trace restarts at `top`: shorter, and `step` is lost. This is a named cost of the "
     "single-identity rule (section 6)."),
    ("iface", "A hop made through an interface",
     "`S.A` is called through the interface `Svc`. The real hop returns into a compiler-made thunk, whose name says "
     "nothing about the caller, so the link after it cannot be checked and is a cut: the trace begins at `run` and the "
     "origin side (`A`) is gone (the program marks `A` for the simulation with `// sim: thunk`). The second part is the "
     "reviewer's case: `viaPlain` calls `A` through the interface, the hop stays in the register, and a later "
     "unrelated `!` on the same sentinel in `h2` has no old hop in front."),
    ("steal", "An unrelated wrapper of a shared sentinel",
     "`load` wraps the sentinel `ErrPerm` with `Wrapf`, and a deferred function that runs after it makes `fmt.Errorf(\"audit: %w\", ErrPerm)`. The sentinel is not the "
     "identity (the `Wrapf` wrapper is), so the audit wrapper changes nothing and `boot`'s `!` continues the trace. The second part is the cost: a log-only wrapper "
     "of the error takes the identity first, so the `Wrapf` that follows wraps an error that is not the identity, and the trace restarts at `bootLog`."),
    ("stale", "A handled `Wrapf` wrapper is not revived by a wrapper of its sentinel",
     "`a` wraps `ErrPerm` with `Wrapf` (the identity is the wrapper `W1`), `f2` handles the error and drops it, then calls `g`, which "
     "returns `fmt.Errorf(\"g: %w\", ErrPerm)` and never calls `open`. The wrapper of the sentinel is not `W1`, so `f2`'s `!` starts a trace of its "
     "own. The second part does the same with the bare sentinel from `g2`. This holds only because the handled error was itself a `Wrapf` wrapper: "
     "when `a` passes the bare sentinel up with a plain `!`, the sentinel is the identity and the next scenario shows the opposite."),
    ("nowrapf", "A handled bare sentinel is revived by its wrapper, by itself and by a `Join`",
     "The limit of scenario 11 through the rekey. `a` passes the sentinel `ErrPerm` up with a plain `!`, so the identity is the sentinel itself; `f2` handles the "
     "error and drops it, then calls `g`, which returns `fmt.Errorf(\"g: %w\", ErrPerm)` and never calls `open`. `Errorf` makes a wrapper of the identity, "
     "the identity becomes the wrapper, and the wrapper **inherits the earlier hops**: `f2`'s `!` is in the function the last hop (`a`) returned into, so the link holds and the trace "
     "is `a, f2`. The trace begins with a hop of an earlier propagation, which `g` never made. Part 2 returns the bare sentinel from `g2` (the same, scenario 11 itself); part 3 returns "
     "`errors.Join(ErrOther, ErrPerm)` from `j` (the same through `Join`). Each hop is a real `!` of an object of the identity chain and each link is a caller and a callee; "
     "the two propagations are not told apart."),
]

def body_start(lines):
    for i, line in enumerate(lines):
        if line.startswith("func show("):
            for j in range(i, len(lines)):
                if lines[j] == "}":
                    return j + 2
    raise SystemExit("no show helper")

out = []
out.append("## Appendix: the scenarios\n")
out.append("The complete programs are `spikes/2026-10-21/error-traces/scenarios/*.gp`, each with a helper `show` "
           "(`file:line function` per frame, base names) that is left out below. `run.sh` there runs every "
           "program but the macro one through a simulation of the register and compares the output with `expected/`. "
           "Line numbers are those of the files. They become `tests/run/etrace_*.gp` in 10.7d, with the real "
           "runtime in place of the simulation. The outputs are those of the simulation, valid for programs without unused locals "
           "(a Go local dies at its last use, a C++ local at the end of its block); 10.7e regenerates them from the real build.\n")
for n, (name, title, text) in enumerate(SCENARIOS, 1):
    src = open(os.path.join(here, "scenarios", "etrace_%s.gp" % name)).read().split("\n")
    start = body_start(src)
    while src and src[-1] == "":
        src.pop()
    code = "\n".join(("%-3d %s" % (i + 1, src[i])).rstrip() for i in range(start, len(src)))
    got = open(os.path.join(here, "expected", "etrace_%s.txt" % name)).read().rstrip("\n")
    out.append("### %d. %s (`etrace_%s.gp`)\n" % (n, title, name))
    out.append(text + "\n")
    out.append("```text\n" + code + "\n```\n")
    out.append("Output:\n")
    out.append("```text\n" + got + "\n```\n")
print("\n".join(out))
