# ADR-098: Promoted members of embedded C++ classes are decided by the C++ compiler in a second pass, without a second go/types run

Status: accepted (2026-10-20). Builds on [ADR-074](074-typed-interop-declarations-and-foreign-types.md) (the foreign
view), [ADR-079](079-typed-interop-checker-reports-only-what-the-compiler-reports.md) (the checker reports what the
compiler reports), [ADR-087](087-typed-interop-code-generation.md) (typed code generation) and
[ADR-088](088-typed-interop-caches-every-compiler-answer-per-reference-under-the-headers-contents.md) (the cache).
Phase 5 step 5.8 made the one-class case work in opaque mode (`internal/check/promoted.go`). The contract is
[docs/spec/typed-interop.md](../spec/typed-interop.md) §11.

## Context

Gap E2 of Phase 12: `type S struct{ std.vector[int] }` then `s.push_back(1)`. go/types cannot know the members of
a C++ class: in `FakeImportC` mode every C/C++ type is invalid, and a field of an invalid type has no members.
Phase 5 treated a selector that go/types cannot find on a struct with exactly one embedded class as that class's
member (the C++ compiler checks it later), and made several classes, or a Go member at the depth of a class, a
gate: check cannot know which class has the member, or whether the class has one at all. README §2.4 planned
"look up the member in the probed record, rewrite the selector through the field, check again". Three questions:
who knows the members, what a second pass is, and where its answers go.

## Decision

1. **The C++ compiler knows the members, one question per class and name.** `decltype((std::declval<C&>().name))`
   is the access itself: clang's lookup covers bases, using-declarations, anonymous unions and the standard
   library's internal base classes (`std::string` is too big for the declaration extraction of ADR-074 to list,
   and `MembersOf` has no marker for using-declarations or ambiguity in bases), and the *words of the error* say
   what the name is (a member function: "must be called"; none: "no member named"; private, protected, deleted,
   ambiguous bases, a type, a macro). The answers are ordinary probe results, so ADR-088's cache keeps them under
   the headers' contents. A class with an `operator->` and no member is undecided, because typed code generation
   (ADR-087) writes `x->name` for the pointee's member; a failed or unclear probe is undecided too. Undecided is
   the first pass's result, so typed mode never rejects what opaque mode accepted.
2. **The second pass is a second *decision*, not a second go/types run.** Check calls a hook
   (`Config.CppMembers`) for the candidates the first pass could not decide, and applies Go's rule of selectors
   (smallest depth wins, several at that depth are ambiguous) to the Go members and the answers together, where
   the first pass looked: `PromotedPath`, the false errors that stay, the opaque pass and the gates. The class is
   a member table in check, not go/types objects, because synthesizing a struct with methods for a C++ class would
   make method sets, assignability and the types of the expressions around each use depend on a declaration that
   stock go/types never reads, and the C/C++ values around it are invalid in go/types by design. (The plan's
   wording, "synthesized methods and fields in the go/types model", and README §2.4's rewrite of the selector
   to `mv.vector.push_back(1)` and re-check, both end in opaque expressions: nothing in go/types would type
   `mv.size()`. Keeping go/types stock is the rule of Phase 12 §2.4.) The cost is nothing without candidates
   and one hook call per package with them.
3. **The cases where Go's rule picks a C++ member over a Go one are errors, not a guess.** go/types has
   already selected the Go member (`Info.Selections`) and typed the expressions around it by that choice. Writing the
   class's member while the types say the Go field would be wrong code; deleting the selection would leave
   the neighbours typed by a type they do not have. So `s.size` where a class at depth 1 has `size` and a Go field
   `size` is at depth 2 says "name the field" (the error that opaque mode's gate also gives, but only where the
   class has the member).
4. **A promoted selector is collected as a member of an lvalue of the field's C++ type.** `collect` makes the
   reference `sel(arg&(C)|name)` and the class reference of the field under the selector's name node, so the same
   probes, the same checker rules and the same typed lowering (`.`/`->`, `T&` assignability, smart-pointer
   arrows) apply as for a variable of the type; the lowering reads the object's kind from the field's class, and
   writes the path of embedded fields as opaque mode does. A class whose type expression is in another package (the
   struct is imported), a type parameter, or an alias to something check cannot spell stays undecided.
5. **Where the hook is set.** `GOCPP_TYPED_INTEROP=1` and the hidden typed commands set it (`cmd/gocpp`); the
   default switch is 12.11's, the language server's is 12.13's. Without clang++ the hook returns `ErrNoClang`.

## Consequences

- Structs with several embedded classes, a Go member beside a class and every kind of missing, private,
  protected, deleted, ambiguous or macro name get a precise error at the selector in typed mode; opaque mode is
  unchanged (the gates stay), and the programs of `tests/interop` that embed classes print the same in both.
- A cold check with candidates runs the compiler once more for the member probes: measured 1.5 s against 1.15 s for the same
  small std example with explicit field paths (about 0.35 s more, one compiler run over the same headers; a larger header costs more), and 0.07 s warm either way.
  A package without candidates pays nothing: the hook is not called and the opaque pass is not run twice.
- Limits, listed in the spec: operators are not promoted; explicit template arguments of a member
  (`s.conv[int](x)`) are a limit of the lowering that opaque mode shares; interfaces satisfied by promoted C++
  methods are not seen by go/types; embedded classes of imported structs are undecided.

See: [docs/spec/typed-interop.md](../spec/typed-interop.md) §11, [Phase 12 README](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md) §2.4 and step 12.9.
