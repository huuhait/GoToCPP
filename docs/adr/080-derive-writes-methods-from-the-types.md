# ADR-080: `#Derive` writes ordinary Go methods, in two steps, and imports nothing

Status: accepted (2026-10-20), step 10.10. Builds [ADR-014](014-macros.md) (`#Derive` on a type) and
[ADR-075](075-user-macros-are-generic-functions.md) (the loader rewrites, then `go/types` sees plain Go).

## Context

The plan (README §2.2) has `#Derive(String)`, `Equal`, `Clone` and `JSON` on top of a type, built in two
steps because a generator needs the types: the constants of an enumeration may be in other files, and
`Equal`/`Clone` need the kind of every field. Building it settled what the plan left open: where the stubs
and the bodies are made, how the generated code is named in positions and messages, which imports it may
use, and what "deep" means for each kind of field.

## Decision

1. **Step 1, the loader (`internal/load/derive.go`).** `#Derive(A, B)` above `type T ...` becomes, for each
   generator, a method **stub** after the type: the final signature (`String() string`, `Equal(other T)
   bool`, `Clone() T`, with the receiver `t` of the type's first letter, or another name when that one is
   used in the declaration or at the package level) and a body that only panics. `go/types` therefore checks
   every caller, in the package and, through the types of the package, in the packages that import it.
   Generic types get the receiver `T[K, V]`. Syntax-only errors are reported here: an unknown or repeated
   generator, a group of types, an alias, an interface/func/channel/pointer type, a method of that name that
   the package declares already. All the nodes of a stub have the place of the generator's **name in the
   `#Derive` line**; the method name is a real identifier (hover and go to definition on `e.Equal(f)` land on
   `Equal` in the line), the others are synthetic (`File.Synthetic`), so the editor skips them.

2. **Step 2, `internal/check/derive.go`.** When `go/types` is done and the package has no errors, a generator
   writes the **source text** of each method from the types of the fields, formats it, parses it and puts
   its body in the stub; the package is checked once more (as the lowering of `!` does). The text carries a
   `//line` directive before every line, so every position in it (errors, panics, the `#line` of the C++, the
   debugger) is the generator's name in the `#Derive` line, and the file of its tokens is named
   `gocpp-derive-N.go` (`check.DerivedFilePrefix`) so that a tool can skip it: `gocpp vet` drops the findings
   inside generated code (the finding about the signature, at the `#Derive` line, stays). The text of every
   method is kept in `check.Package.Derived` for `gocpp expand` (step 10.11).
   **A caller that does not lower the program does not pay for the second check:** `check.Config.NoDerivedBodies`
   (the language server, `gocpp check`, `gocpp vet`) runs the generators for their errors only (no formatting, no
   parsing, nothing kept, the stubs stay). Measured with the didChange harness of the review (a 3155-line package,
   one derived type; and a 705-line package of 50 types with 100 derived methods against its 3155-line
   hand-written equivalent): per keystroke 197 -> 92 ms and 243 -> 25 ms, against 90 ms for the hand-written one
   (`docs/benchmarks/derive.md`). `gocpp build`, `run` and `emit` check twice, as before.

3. **The generated code imports nothing.** `Equal` and `Clone` need no package, and an enumeration's `String`
   writes the number of an unknown value with a short loop. This also keeps the code readable in `gocpp expand`
   and the language server's package metadata (made from the header of each file, `parser.ImportsOnly`) the same
   with and without `#Derive`. (A first version added hidden blank imports in the loader; the server failed with "the
   import of strconv was not resolved with the metadata".) **`String` is for integer types with constants only**
   (the plan's table): a struct's `String` would need `fmt`, which `fmt.Println` already provides (it prints the
   field names with the plus flag), and it is an error with that explanation.

4. **`String`** writes a `switch` over the constants of the type that the package declares, in the order of
   their declarations, the first name for a value that two constants share (Go's `stringer`), and
   `Type(n)` for another value. A type without constants is an error.

5. **`Equal(other T) bool`** is deep equality: a field of a type that has an `Equal(T) bool` method calls it
   (`time.Time`, another derived type, a type with a method that you wrote; a method of that name with another
   signature is an error); a "flat" field (no pointers) is compared with `==`, also when its type has a derived `Equal`
   (the same result, faster); arrays, slices and maps are compared element by element (a nil slice or map equals an
   empty one, as `slices.Equal` and `maps.Equal` say); a pointer is equal to the same pointer, and otherwise to a
   pointer whose pointee is equal (nil only to nil), except a pointer to a type of another package that has no
   `Equal`, which is compared **by identity**; an interface field is compared with `==` (it panics for values that cannot
   be compared, like `==` on the struct would), or with its `Equal` method when the interface type has `Equal(I) bool`;
   a field of a type parameter uses `==` for `comparable`, its `Equal` method, or `any(a) == any(b)` (which panics at run time
   when the type argument is a slice or a map). A func field, a C or C++ type, a type of another package that cannot
   be compared, and a type of this package that holds pointers, slices or maps and has no `Equal` method are errors
   that name the field and say to add `#Derive(Equal)` to that type.

6. **`Clone() T`** is a deep copy: the struct is copied by assignment and the fields that refer to memory are
   copied again (nil stays nil, an empty slice or map stays empty); **a type that has a `Clone() T` method of its
   own gets it called** for a field, for the elements of a slice, array or map and for what a pointer points at,
   also when the type holds no pointers (the type's Clone says what a copy is; a method of that name with another
   signature is an error); pointers are copied with their pointees, **except a pointer to a type of another package
   without a `Clone` method, which is shared** (as `Equal` compares it by identity, so a copy equals its original:
   `*time.Location`, `*sync.Mutex`, `*bytes.Buffer`); channels, funcs, type-parameter values and interface values are
   shared (an interface with `Clone() I` is cloned through it; a `V` that is a slice is shared too). A **value** of
   another package is copied as a value (`time.Time`, `sync.Mutex`), unless it shows a slice, map or pointer in an
   exported field or is a slice or map type itself and has no `Clone` method: then it is an error that names the field
   and asks for a `Clone` method (a copy would share the slice). Sharing is not preserved (a node reached twice becomes
   two nodes). `Equal` and `Clone` on a struct with a `sync.Mutex` copy and compare the lock state; `gocpp vet`
   reports "Clone passes lock by value" at the `#Derive` line. Aliases to types of packages that the file does not import
   are written as the alias.

7. **References that point back to themselves are not supported** (the plan says so for `Clone`; the same holds
   for `Equal` of two different cyclic values). Like the hand-written code, the generated code overflows the
   stack, with Go's `fatal error: stack overflow`. The same pointer is equal to itself at once. (The test
   `derive_cycle` shows the cases that work; the overflow cannot be a test, because a sanitizer run reports it
   in its own words.)

8. **`JSON` is not built.** It needs `encoding/json` (`stdlib/encoding/json` is a declaration stub until step 9.11):
   a faithful `MarshalJSON` must match `encoding/json` exactly (escaping, number formats, tags, embedded structs).
   `#Derive(JSON)` is an error that says so (`explain`: `loader/derive-json`); on a type that is not a struct it is an error
   anyway. *Design note for later (from the review of 10.10):* the blocker is encoding/json, not the language server's
   header-only metadata: `internal/check` already has a precedent for an implicit hidden package (`gocpp/cgoh`), so an
   implicit import is feasible, and what is missing is a package to call. A smaller design: a runtime-helper package
   plus hidden-import plumbing for `MarshalJSON`/`UnmarshalJSON`, with a differential test against `encoding/json`.

## Consequences

- A package that derives is checked **twice** by `gocpp build`/`run`/`emit` (the cost of the second check is the
  package's, about the cost of the first); the editor, `gocpp check` and `gocpp vet` check it once (decision 2). A
  package without `#Derive` does not change.
- Derived methods are real methods: summaries, export data, `gocpp doc` (it shows them from fixed tables, ADR-063),
  hover, definition, references and rename work; renaming the type regenerates the stubs.
- `vet` still finds what is wrong with the type from the signature (`Clone passes lock by value` for a struct
  with a `sync.Mutex`, at the `#Derive` line).
- Messages: `internal/explain` has an entry for every error (`loader/derive-*`, `checker/derive-*`); the errors of
  `go/types` about a stub are rewritten in the words of the line (`invalid receiver type` -> "is an interface or a
  pointer type", "field and method with the same name" -> "has a field ... already").
- Cost at `-release` (g++ 15 / clang++ 21, 2 million calls, `docs/benchmarks/derive.md`): derived `Equal`, `Clone`
  and `String` are at the speed of the hand-written Go-style methods (within the noise; a derived `Equal`
  of a struct of numbers is the same instructions as `==`).

## Alternatives

- **Generate the bodies before `go/types`** from the syntax: impossible for enumerations (constants may come
  from any file) and wrong for fields whose type is a name.
- **Typed AST for the bodies**: the text is checked by `go/types` and so cannot be wrong in ways the type checker
  does not know; building `types.Info` by hand would have to be kept in step with every change of the checker.
- **A reflection helper** (`reflect.DeepEqual`) for `Equal`: needs an import (decision 3), and differs from the
  rules above (nil and empty slices).
