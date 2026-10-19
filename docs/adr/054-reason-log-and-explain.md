# ADR-054: The lowering records its reasons in an optional log; `explain` is built from the compiler's own output

Status: accepted (2026-10-18).

## Context

Step 13.5 needs two things: `gocpp emit -annotate` (why is this variable boxed, where does the code check, which
conversion allocates) and `gocpp explain` (what a mechanism lowers to, what a "not available yet" message means).
The decisions are made deep in `internal/lower`. The emitted C++ must not change for anyone who does not ask.

## Decision

- **A reason log, `lower.Why`, in `lower.Options`.** Lowering hooks call `g.why(pos, kind, msg)`, or the
  `note...` helpers of `internal/lower/whynotes.go`. With a nil log a hook returns at once, and hooks that format
  a message first test `g.noting()`. No hook reads the log: it cannot change the C++. A test compares the output of
  `gocpp emit` with and without a log, for every example and for every program of `tests/run` and `tests/golden`
  (and the step's report compares the compiler before and after the step over all 1605 programs of `tests` and
  `examples`: identical, stderr and exit code included).
- **Each reason has a kind, and each kind is an `explain` topic** (`boxing`, `defer`, ...: `lower.AllKinds`,
  `internal/explain/topics.go`). A test keeps the two lists equal, so a reason always links to a topic.
- **The annotated view is for reading, not for compiling.** It prints the C++ grouped by the originating `.gp` line
  with the reasons, and replaces the `#line` directives with headers. `TestAnnotateKeepsTheCpp` checks that
  removing the annotations gives the normal emit back.
- **Topics have live examples.** Each topic embeds a small `.gp` program (`internal/explain/examples`); `gocpp
  explain TOPIC` emits it with the real compiler every time, and goldens and the guide are tested against that
  output, so the documentation cannot drift from the compiler.
- **Messages are a table with a completeness test.** `internal/explain/messages.go` has an entry for each "not
  available yet" / "(Phase NN)" message (the gates of the checker are read from `internal/check`); a test scans
  the sources of `internal/lower`, `internal/cppgen`, `internal/load` and `cmd/gocpp` and fails for a message with no
  entry. Catch-all entries (marked `Fallback`) answer only for fragments that the lowering completes with a name.

## Consequences

A new lowering decision needs one hook line to show up in `-annotate`; a new "not available yet" message needs one
table entry (the completeness test says so). The cost of the log when off is a nil check in the hooks.
