# ADR-035: `reflect` is a subset written in `.gp` over the type descriptors

Status: accepted (2026-10-18)

## Context

Phase 09 ports packages that call `reflect`: `sort.Slice` (`reflectlite.Swapper`), `fmt` (`Sscan`),
`flag`, `context`, `encoding/json` (settable values), `internal/fmtsort` and the ported tests
(`reflect.DeepEqual`). Divergence D6 says gocpp's `reflect` is a **subset**: no `MakeFunc`, no
calling methods by name, no `StructOf`/`FuncOf`, nothing that needs run-time code generation.

gocpp has no run-time type information beyond the **type descriptors** (ADR-016, ADR-028): one
`gp::type_desc` for every type that flows into an interface, with name, kind, size, method table
and (lazily, `gp_full`) fields, element, key and parameters; and `type_ops` (`gp_ops`), the small
functions that read a value of the C++ type without knowing it (ADR-028). `fmt` already walks values
through them. What a settable `reflect` needs on top is to **make** and **write** values: a zero
value of any type, a copy, a new slice, map or pointer, an assignment.

## Decision

1. **`reflect` is Go's API, written in `.gp` (`stdlib/reflect`) over native functions** (ADR-027,
   `runtime/include/gocpp/reflect.hpp`, `runtime/src/reflect.cpp`). `Value`, `Kind`, `StructTag`,
   `DeepEqual`, the panics and their texts are Go's code, adapted; the natives only read and write
   objects by descriptor. A **type** is the address of its descriptor (`uintptr`; the descriptor,
   never `gp_full`'s copy, is the identity, so `TypeOf(a) == TypeOf(b)` is pointer equality); `Type`
   is Go's interface over `*rtype`, one `rtype` for every descriptor (a table), so Types compare
   with `==`. A **Value** is `{typ uintptr, ptr unsafe.Pointer, flag}`: `ptr` is always the address
   of the C++ object itself (for a pointer, the address of the `gp::ptr` object, never "the word in
   the interface"), and its owner keeps the storage alive. One rule for every kind replaces Go's
   `flagIndir` cases.
2. **Lifetime operations in `type_ops`.** `gp_trivial` (zero value is all-zero bytes, a copy is
   `memcpy`), `gp_ctor` and `gp_dtor` (construct the zero value in raw memory, destroy it),
   `gp_share` (the type is exactly `gp::ptr<U>`: an interface that holds one shares the pointer's
   owner instead of boxing a copy). They are set by `ops_of<T>()` and `foreign_ops_of<U>()` from
   the C++ type. A C++ type that can't be default constructed or copy assigned has null `gp_ctor`:
   reflect panics `reflect: C++ type X is opaque (it can't be made or assigned)`.
3. **One pointer for the group of a kind, `gp_refl`.** `slice_refl` (make a slice from an array view,
   capacity), `ptr_refl` (make a pointer that shares an owner: the aliasing constructor), `map_refl`
   (new, len, find, set, erase, clear) and `chan_refl` (len, cap) are typed (a template instantiates
   them for the element, key and value types) and constant-initialized; a descriptor points at the
   group of its own kind, so a type pays one pointer, and a program that never calls `reflect`
   pays only for the constant tables that its descriptors already needed to reference (nothing the
   linker can drop, see Evidence for the numbers). `gp::cptr<T>` is read like a pointer, but
   reflect does not make one.
4. **Storage and aliasing.** Storage that reflect makes is one block owned by a `shared_ptr` whose
   deleter runs `gp_dtor`; a Value that came out of an interface shares the interface's (immutable)
   box and is not addressable; one that came out of a map is a copy; a field is the same storage at
   an offset (`v_offset`, offsets from `gp_full`), so `Addr` and `Set` write through, as in Go.
   `Interface()` copies (`v_interface`).
5. **`fmt` shares the machinery.** The engine's `print_arg` recognizes a `reflect.Value` operand
   by name (`pp::print_reflect_value`: the type, the object and the flags, in that field order) and
   prints the value it holds, as Go's `case reflect.Value` does, honoring the read-only flags (a
   Value taken from an unexported field prints without calling its methods). `reflect` has no printer
   of its own.
6. **Generated interface structs hide their method names.** An interface with a method named like a
   type that another method mentions (`Kind() Kind`, `Method(int) Method` in `reflect.Type`) made the
   C++ member hide the type in its own body and in the parameter lists of the wrapper
   definitions; `ifaceDef` and `ifaceImpl` now write those types qualified (`g.hide`, as
   named types already do). This is the only change to the lowerer.
7. **`internal/reflectlite` is an alias package** of the subset (`stdlib/internal/reflectlite`:
   `type Type = reflect.Type`, the kinds, `TypeOf`, `ValueOf`, `Swapper`, `DeepEqual`,
   `Value`'s methods as methods of the alias). It has no gate row (nobody outside the standard
   library can import an internal package). The Phase 05 `errors` keeps its three natives;
   Phase 09.4 may import `reflectlite` where Go's `errors`, `sort` and `context` do.
8. **C/C++ values.** reflect sees a foreign (`kind::Foreign`) value as the Go kind of the same
   shape: a number, a bool, a string, an `unsafe.Pointer` for every C pointer, a `Struct` without
   fields for a class; `Type.String()` is the C++ type's name; a class that is opaque panics as in
   item 2. `reflect.DeepEqual` on opaque values uses `==` where the C++ type has it and panics
   otherwise. A foreign value is a C++ object, never a gocpp one, so no native writes it through
   the Go layout of its Go kind: `SetString` on a `std::string` assigns the bytes (copy), on a
   `std::string_view` panics `reflect: SetString on a C++ value of type T is not supported` (nothing
   would keep the bytes alive); `SetInt`/`SetUint`/`SetFloat`/`SetBool` need a number of the Go
   size (1, 2, 4, 8 bytes; a 16-byte integer is a class, a struct without fields, so these panic in
   the `.gp` kind check); `SetComplex`, `Convert` between a foreign and another type, and `Slice`
   of a string_view panic the same way; `Set` and `SetZero` use the type's own copy (`gp_store`)
   or are opaque. `Value.Slice` of a `std::string` gives a copied `std::string`.
   `tests/interop/reflect_cpp_values_set.gp` runs every writer over std::string, string_view,
   numbers, enums, classes, unions, bit-field structs, wide integers, pointers and arrays.
9. **Not in the subset** (`.gp` doc comment of the package, `feature-matrix.md`): `MakeFunc`,
   `Value.Call`, `Value.Method`/`MethodByName` (a method is listed with `NumMethod`, `Method(i).Name`,
   `PkgPath` and `Type`, item 11; `Method(i).Func` is the zero Value), `StructOf`, `FuncOf`, `MapOf`, `ArrayOf`, `ChanOf`,
   `Select`, `Value.Send`/`Recv`/`Close`, `Value.Seq`. Each panics or is absent: a program that
   uses one is a compile error ("undefined: reflect.X"), never a silent difference.
   Also absent (compile error): `VisibleFields`, `SetIterKey`/`SetIterValue`, `SetPointer`,
   `FieldByIndexErr`, `TypeAssert`, `MakeChan`, `NewAt`/`SliceAt`, `Seq2`, `CallSlice`,
   `Value.TrySend`/`TryRecv`.
10. **Known divergences from Go** (found by the 9.3 review; (a) and (b) were fixed in a later
    pass): (a) `DeepEqual` walks with its own stack of frames on the heap, so a linked list or
    a nest of a million levels works (Go's 1 GB stack ends at about a million). (b) `MapIter`/`MapRange`
    take the keys of the map at the first `Next` (an entry added later is never produced, where Go
    may produce it); an entry deleted before `Next` reaches it is skipped and `Value` reads the
    current value (the zero value of the element type when the key was deleted after `Next`), as Go
    and the language-level `range` do. (c) `Type.In`/`Out` out of range panic
    with a string, not a `runtime.Error`. (d) `Value.Pointer()` of an empty string is not 0.
    (e) `DeepEqual` of two `reflect.Value`s: Go compares the `Value` struct (type, pointer,
    flags), and the rule behind its answers (checked with Go 1.25.7) is the identity of the
    pointer field: for a pointer, a map, a channel and a func it is the pointer itself, so two
    `Value`s of the same one are equal, and so are `ValueOf(&x).Elem()` and its `.Field(i)` taken
    twice, a copy of a `Value`, and two zero `Value`s; for any other kind it is the box that the
    conversion to an interface made, which is a new allocation for a number, a string, a float, a
    struct or a slice computed at run time (false), except where Go's conversion shares one: a bool
    and a one-byte integer, a value of 2, 4 or 8 bytes (aligned to its size, without pointers)
    below 256 (`runtime.staticuint64s`), `""` and a slice with a nil array, also as the one field
    of a struct or the one element of an array (`runtime.zeroVal`), a zero-size value
    (`runtime.zerobase`); and a value that Go keeps in the interface word itself (a pointer, a
    map, a channel, a func, or a struct or an array of one such part) is its own pointer field.
    gocpp follows the rule for those pointer-shaped types (the `Value`s compare the pointers they
    hold: `directPart`; test `reflect_deepequal_values`), for the storage of
    copies, `Elem` and `Field`, and for the shared boxes (`ValueOf` of such a value gives storage
    that is the same for equal values, never written: test `reflect_deepequal_shared_boxes`), so
    `DeepEqual(ValueOf(1), ValueOf(1))` is true as in Go. Not followed: Go also shares the
    read-only box of a constant (`ValueOf(1000)`, `"a"`, `1.5`, `P{1, 2}`, also `n := 1000` that
    the compiler folds), where gocpp boxes anew, so those rows say false where Go says true (it
    would need the emitter to give each constant converted to an interface a static box).
    (f) A C/C++ value in an
    interface (`any(C.int(1))`) has a descriptor of its own that is not the Go `int32` of a
    field `C.int` of a struct: `Set` between them panics, and the text names the C++ type
    (`int32 (C++ int)`) so that it does not read "int32 is not assignable to int32".
11. **`Method(i).Type`** is the method's func type, as in Go: with the receiver first for a type
    that is not an interface, without one for an interface. Each method table entry (`gp::method`,
    `gp::imethod`) has `gp_ftype`, a function that gives the func type of the method without its
    receiver (`gp::td_sig` of the parameters' and results' descriptors, written out in place, so it
    names no object of the package and costs nothing at program start); reflect's `m_type` puts
    the receiver first (`gp::sig_with_receiver`). The func types are descriptors without a C++ type
    of their own: every `gp::func<R(A...)>` has the layout and the operations of
    `gp::func<void()>`, and the interning gives the program's own descriptor when it has one. The
    functions cost code (one for each signature of a package, and one for each method of an
    instance of a generic type and of an unnamed interface) and keep the descriptors of all the
    types in the signatures, with their methods, which the linker would otherwise drop: a program
    with `encoding/json`, `regexp`, `time` and `bufio` grew by 4.2 % (5.30 to 5.52 MB stripped).
    So the emitter writes them only for a program in which a package other than reflect can get
    at a `reflect.Method` (`wantMethodTypes`, `usesReflectMethod`): it names the type, calls a
    method `Method` or `MethodByName` of package reflect, has an expression whose type is or holds
    `reflect.Method` (a generic `get[T interface{ Method(int) R }, R any]` called with a
    `reflect.Type`: R is inferred, the program never names it), or a type argument that is or
    holds it. In any other program `gp_ftype` is null, and the program has no `reflect.Method`
    value to read it from (what is left: a `reflect.Method` reached through `any` alone, such as a
    `reflect.Value` of the `Type` interface that calls `Method` through reflection, which the
    subset cannot do: `Value.Method` and `Call` are not in it). The
    standard packages' files then have two forms, and the object cache keeps both. The method
    tables of the runtime's interfaces (`error`, the `fmt` ones) always have theirs. What every
    program pays is the field (8 bytes a method table entry, and its store in the tables' dynamic
    initialization): 0.6 % of the same program without `reflect.Method` (5.28 to 5.32 MB, with
    item 10e's shared boxes).

## Alternatives

- **Port Go's `reflect` (`abi.Type`, `rtype`, `unsafe` arithmetic).** It needs Go's type layout
  (`abi.Type`, `uncommon`, `name` encoding, `typelinks`) and the Go runtime's `unsafe` view of
  memory; gocpp's objects are C++ objects (`gp::slice`, `gp::map`, `gp::ptr`) that are not laid out
  as Go's headers. Rejected: it would need a second set of descriptors next to ADR-016's.
- **Put the operations in the descriptor itself (more fields).** `type_desc` is 208 bytes
  and every program has dozens; the operations are only reached for slices, maps, pointers and
  channels, so they live in the group of `gp_refl` and cost one pointer.
- **Natives for everything (no `.gp`).** The panics, `StructTag`, `DeepEqual`'s traversal,
  `Value`'s flag logic are Go code; keeping it as Go code keeps the messages identical and the
  C++ small.

## Consequences

- `reflect` costs nothing in a program that does not import it (the natives are in
  `libgocpp_rt` and linked only when called); the descriptors grew by `type_ops` fields (Evidence).
- Generic types (Phase 07.3) need no change in `reflect`: their descriptors are descriptors.
- Every new `type_desc` or `type_ops` field must be reflected by `reflect.hpp`'s vocabulary and
  by `runtime/test/reflect_test.cpp`; `fmt` and `reflect` read the same fields.
- `reflect.Value` is not safe for concurrent writes to the same object, as in Go.
- Descriptors of types that the program never mentions do not exist: `PointerTo(t)` and
  `SliceOf(t)` make (and intern) a descriptor at run time when the program has none; `MapOf`,
  `ChanOf`, `ArrayOf` would need C++ templates instantiated at run time, which cannot be done.

## Evidence

- `tests/run/reflect_*.gp`: 104 programs, each compared with real Go's output
  (`GOCPP_RUN_VS_GO`), on g++ and clang++, normal and with `-asan`; `runtime/test/reflect_test.cpp`
  tests the natives over hand-written descriptors; mutation checks (see the step's report).
- Sizes: see the step's report and `docs/spec/cpp-mapping.md` ("reflect").

## See

[Phase 09 §2.2 and step 9.3](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md),
[ADR-016](016-type-descriptors.md), [ADR-027](027-native-std-functions.md),
[ADR-028](028-fmt-on-descriptors.md), [language.md D6](../spec/language.md#d6-a-reflect-subset-and-no-plugin).
