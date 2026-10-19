# ADR-074: Typed interop reads declarations by id from filtered dumps, and maps C++ to go/types

Status: accepted (2026-10-19). Builds on [ADR-060](060-typed-interop-probes.md); the contract is
[docs/spec/typed-interop.md](../spec/typed-interop.md) §5 and §6.

## Context

Steps 12.1 and 12.2 give the type of every C/C++ expression as text. Step 12.3 has to say what the
members of those types are (fields, methods, bases, enumerators, overloads), and step 12.4 has to say
what the types are in gocpp's terms, so that the interop checker (12.5) can type expressions that
`go/types` leaves invalid. Three things were not obvious: how to get a declaration out of clang
(its AST dump can only be filtered by a substring of a name), what a "foreign type" is when `go/types`
is not forked, and what to do with the types that have no foreign view.

## Decision

1. **Declarations are found by id.** A run's file has the package's includes and an alias per
   asked type, named so that the dump filter (the type's last name) selects both the real declaration
   and the alias. The alias's type tree holds the id of the declaration it stands for, and the id
   selects the declaration in the dump (by name it would be ambiguous: a typedef of an unnamed
   struct has no name, a class template instance is one of many, and `Item` can be in three namespaces).
   `sizeof`, `alignof`, the canonical spelling and the enum's underlying type are asked as aliases of
   the same run, so the compiler's answers are used, not guessed (the JSON does not give an
   enumerator's implicit value, the underlying type of an unscoped enum, or the size).
2. **Functions: one run for all.** `namespace gp_f0_fopen { using ::fopen; }` makes the dump list
   every overload with its type in the using-declaration's shadows, with a unique prefix to filter on.
3. **One run per last name, parallel, bounded.** Names that share a last part share a run; at most
   64 runs a call, 8 at a time; a dump over 32 MB (`std::basic_string<char>` has 40 MB because the
   bodies of its member functions are in it) is a refusal that is remembered, not a failure. This
   keeps the common case (user headers, C libraries, the common containers) at 0.1 to 1 s a name, and it is
   what step 12.7's disk cache will absorb.
4. **A structured type (`ctype`) is parsed from clang's text** with a real declarator parser,
   tested for being the inverse of the printer over 7,000 spellings of the standard library; the text is
   kept next to it, so an unreadable type is an `Opaque` field, not a lost declaration. The "written"
   type (typedef names kept) comes from the type tree of the probe alias, not from a second probe.
5. **The foreign view is `go/types`.** No fork: pseudo-package `C` with named types `C.int` ... over
   Go's basic types, records as named structs (public fields, public bases as embedded fields, so the
   standard `LookupFieldOrMethod` promotes), methods and overloads in a side table (`Members`),
   enums as named integers with `types.Const`s, stand-ins as `string`/`[]T`/`[N]T`/`map[K]V`,
   references as a flag on the mapped type and on the value (`Assignable`). One `types.Named` per canonical
   type. This is what 12.5 needs to reuse `types.Identical`, `AssignableTo`, struct promotion and
   `TypeString`, and nothing in it needs a different `go/types`.
6. **Fallback is opaque with a reason, and the mapping never guesses.** A relative name is resolved
   from the innermost scope outwards among the names the declarations know; if none is known it
   is opaque and `NeedsMore` asks for the candidates in the next round. The compatibility rule is a
   function (`Value.Compat`) that says `No` only when no C++ conversion exists.
7. **Off by default.** `Options.Decls` (and `gocpp __typed-probe -decls`) turn the step on; `Analyze`
   without it is unchanged.

## Consequences

- A record of the library costs a compiler run of its own; the memo makes the second call free, and
  12.7 must put the same key (`PreKey` of the run text and the headers' contents) on disk.
- Member types of namespaced records are resolved in the mapper, not in the reader: `Record.Field.Type.T`
  can hold a relative name (`Vec`), and only `Record.Scope` and the known names tell which one.
- `explicit` constructors, locations, parameter names of free functions and variables are not extracted
  (12.13 and 12.5 may need them; the JSON does not carry `explicit`).
- The long double has no Go type: it is a float64 in the view, and `Mapped.C` has the truth.
- `LongBits` (64 or 32) is the target's data model; the probes of a Windows build (Phase 14) must be
  mapped with `ftype.LLP64`.
