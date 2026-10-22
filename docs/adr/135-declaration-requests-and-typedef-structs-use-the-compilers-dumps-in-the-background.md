# ADR-135: `textDocument/declaration`, definitions in package sources, and the fields of typedef'd structs come from the compiler's dumps, in the background

Status: accepted (2026-10-21; job 12.13e). Builds on [ADR-122](122-declaration-locations-of-header-names-come-from-clang-lookup-dumps.md)
(declaration locations) and [ADR-126](126-time-to-c-cpp-editor-data-is-measured-and-its-independent-compiler-runs-go-together.md)
(no request waits for the compiler).

## Context

Two things were left open after step 12.13c2 (ADR-122):

1. The server had no `textDocument/declaration`. Go to definition of `raylib.InitWindow` jumps into the
   header, which is the declaration; a C or C++ function that a package source (`geo.cpp`) defines has
   a second place, its body, and the editor had no way to ask for either one.
2. A C library names its structs with a typedef, `typedef struct Vector2 { float x; float y; } Vector2;`
   (raylib: `Vector2`, `Color`, `Rectangle`). The lookup of the name (`#pragma clang __debug dump Vector2`)
   finds the typedef, whose dump has the name of the record and none of its fields; C++ hides a tag that a
   typedef of the same name names, so no pragma finds the record. Hover showed `typedef struct Vector2
   Vector2;`, and the hover of a variable of that type had no fields either.

The rules of 12.13c2 and 12.13d stay: no request starts or waits for a compiler, a missing fact is the old
answer, nothing slows the cold open, a stale answer is a bug.

## Decision

**Declaration.** `textDocument/declaration` is advertised (`declarationProvider`) and answered by the code
of definition with one switch. A Go name has one place that declares it, so both give it (as gopls does).
A C/C++ name gives the declaration of the header for declaration; definition gives it too, except for a
function or a variable that a C or C++ source of the package folder defines (below), where it gives the
body. Implementation and type definition of C/C++ names are not offered: the data would be other
lookups (derived classes, the type of a variable), not these.

**Definitions in the sources of the package.** The compiler's lookup answers with the latest declaration
of a name. After the worker published the declarations, it looks the functions and variables among them
(those that the header does not define itself) up again with the include of each `.c`, `.cc`, `.cpp` and
`.cxx` file of the package folder added to the probe file (`"/abs/geo.cpp"`, inside `extern "C" { }` for a
`.c` file, which is C code: a header that declares the function with C++ linkage is not what it defines),
one compiler run for each file, four at a time. The latest declaration is the definition when it has a body
(`Declaration.Defined`: a `CompoundStmt` child of a function, a variable that is not `extern`) and the
compiler took it for another declaration of the entity that the header declares (`Declaration.Redeclares`,
the word `prev` of the dump). A static function, or a function in an unnamed namespace, is another function
whatever its name is, and has no `prev` when the source defines it without declaring it first; so is an
overload of another type. The body may be in a file that the source includes (a unity build, a generated
`.inc`).

*A function that the source declares first.* `static int sp(char c); static int sp(char c) { ... }` has a
`prev` of its own, the prototype in the source, so `prev` alone does not say that the header's function is
meant. Two rules separate it. The dump line of a static declaration says `static`, and such a declaration
never counts as a redeclaration of the header's (`Redeclares = prev && !static`; the prototype and the
definition are one chain of their own). A function of an unnamed namespace has no mark in the lookup dump
(the namespace is not printed), so the pairing below decides: a header declaration is paired with the
definition that has the same parameter types (`Declaration.Params`: the canonical type of each
`ParmVarDecl`, joined by `;`, with `...` last for a variadic function). `int sp2(int);` in the header and
`namespace { int sp2(double); int sp2(double d) { } }` in the source have `int` against `double`, so the
header is shown. The types of the whole function (`Declaration.Type`) are not compared: the dump keeps the
typedef names of the declaration (`size_t (size_t)` against `unsigned long (unsigned long)`), and the
return type is not part of the identity of a function. There is no rule for a single declaration and a
single definition of a name any more: a definition of other parameters is another function, and the header
is shown. A variable is paired by its name (its type may be spelled `int[]` in the header and `int[3]` in
the source).

*A definition that a redeclaration follows.* `int g2(int a) { ... }` and then `int g2(int a);` in the same
source: the lookup shows the latest declaration only, which has no body. When the latest declaration of a
name in a source is a redeclaration that is not the header's own (`Redeclares`, not `Defined`), one more run
asks for the chain: the filtered AST dump (`-ast-dump-filter=g2`) prints every declaration of the name under
its own heading with its own body (`Runner.LocateChain`, cached like the others, key
`locate-v6-chain/NAME`), and the declaration that has a body (and `Redeclares`) is the definition. The run
is made only for such names, and a failure of it fails the source's run (retried once like any other).

The editor's file system lists only `.gp` files, so the folder is read by the worker. Build constraints are not applied: a source that does not compile for this target
fails its run and gives nothing, and the other sources still do. The locate runs are cached on disk like
the others, and their dependency lists contain the source, so an edit of it is noticed.

*Which sources.* At most 16 sources are read for each include set, none of them over 512 KiB; a source over
the limit is skipped and does not use one of the places. The sources are ranked before the cut: first those
whose name has the stem of a header of the include set (`geo.cpp` for `geo.h`), then those that include a
header of the set in their first 32 KiB, then the others, each group in name order. A folder with more than
16 readable sources therefore keeps the ones that belong to the headers in use. What was left out is counted,
and the hover of a C/C++ function or variable that no source was found to define says so ("Go to definition may
show this header: N C/C++ source files of the package were not read for definitions"), so that go to definition
showing the header is explained; a function that a source that was read defines has no note. A run of the includes of
raylib and one source took 72 ms on the development machine (measured, cache off), and 16 sources four at a
time 0.27 s; a C++ standard header set costs more per run, and the limit is a count of runs for that reason.
They are cached and run in the background after the declarations.

*What a snapshot costs.* Every edit of a `.gp` file makes a snapshot, and the worker of a snapshot asked every
source again. A locate question is cached by the runner, but the runner validates a cached answer by reading
every file that the run read (hundreds of headers of the standard library) and, when a file was modified
shortly before (the racy window), by running the compiler again: three runs for each source (`-fsyntax-only
-MD`, `-E -MD`, `-E -dI -MD`), 48 for 16 sources, for an edit that had nothing to do with them. The worker
now keeps what each source gave from one snapshot to the next (`sourceMemo`, in `foreignmemo.go`): the
definitions found for the names that were asked, those names, and the stamps (`probe.Stamps`) of the files the
run read: identity, size, modification time and change time of each file (and of the places that the compiler
looked in and did not find a file), read with `stat` and nothing else. Stamps are taken only when no file of
the run was modified shortly before or after it (the racy rule of the runner, so that a write which leaves the
times alone cannot be missed), and the memo is kept only with stamps. A new snapshot uses the memo of a source
while the stamps are unchanged, for the same compiler, flags, includes and source; it asks the source only for
the names that the memo has not been asked (an edit that writes a new name costs one run of that source, with
that name), and asks again when a file changed. At most 128 memos and 16 MiB are kept, the least recently used
dropped first. Measured with twenty sources, sixteen read, by the questions that the worker asks the
compiler layer (`TestSourceMemoQuestionsPerEdit`) and by a wrapper that counts the compiler's runs
(`TestSourceMemoCompilerRunsPerEdit`):

| edit | questions before | questions now |
|---|---|---|
| first snapshot | 16 | 16 |
| an edit of a `.gp` file that asks no new name | 16 | 0 |
| an edit of a `.gp` file that asks a new name | 16 | 16, for that name only |
| a source file (saved on disk) | 16 | 1 |
| a header | 16 | 16 |

The compiler runs, with the runner's cache warm and the files older than the racy window: 60 for the first
snapshot (48 of them for the 16 sources), 0 for an edit of a `.gp` file (before: 0, but each of the 16
questions read the dependency files of its run to validate the answer), 3 for an edit of one source (the
same before). When the runner cannot answer from its cache (a cold cache, or a file modified within the
racy window) each question is three runs, so the 16 questions of every snapshot cost up to 48 runs before,
and the memo is what takes them away. The memo does not decide more than the runner does: the runner trusts
a file by its content hash, the memo only when the status of every file is as it was and could not have
hidden a write.

*When a source changes.* The client watches `*.{c,cc,cpp,cxx}` as it does the headers. A change reaches
`Pipeline.HeadersChanged`, which asks the workers again (a source whose files are unchanged is served by its
memo, so only the source that changed runs). The
definitions' own runs are tried once more after two seconds when one fails, because a source saved inside
the runner's racy window (a file modified less than 2 s before the run) is refused for that time, as the
header index does.

**Bodies of typedef'd tags.** The dump of a typedef (`TypedefDecl ... 'struct Vector2'`) becomes a
`Declaration.Tag` (kind, tag name, typedef name, the range of the typedef). For these `Runner.LocateBodies`
runs `clang -fsyntax-only -Xclang -ast-dump -Xclang -ast-dump-filter=NAME` once for each typedef: the
filtered dump prints the definition of every declaration whose name has NAME in it, with fields,
enumerators, comments and nested records, and it finds a tag that has no name of its own (the typedef's
name stands in), where the pragma cannot find the record. The matching node is the definition of the
tag with that qualified name (the filter lists every `Thing` of every namespace, under the heading
`Dumping a::Thing:`; the qualified name comes from the typedef's type with its sugar removed, `b::Thing`
for `struct Thing` written inside `namespace b`), or, for an unnamed tag, the one inside the range of the
typedef. It is written as
`typedef struct Vector2 { ... } Vector2;` (one declaration, as the header has it) or, when the tag has
another name than the typedef's, as the typedef and the struct after it. Fields are `type name;` (the
compiler's spelling, a function pointer as `void (*cb)(int)`, an array as `int table[4]`, a bit-field with
` : 3`), followed by `// text` when the member has a one-line comment on its line or in a documentation
comment (a plain comment above is the heading of a group in most headers and is not used). A comment
at the end of a line that several members end (`int a; int b; // text`, `int x, y; // text`) may be
about any of them and is shown for none. An unnamed
union or struct is written where its member is. At most 40 fields (nested included) or enumerators, then
`// ...`; a body over 8 KiB, a failed run and a missing definition leave the plain typedef. The run
belongs to the typedef (its key has the tag name, the file and the range), so two typedefs with one tag
name do not share an answer. A name shorter than three letters is not asked: the filter is a substring of the qualified
name, so one or two letters would match most of the header, and the dump of that would be larger than the
limit on the run; those typedefs keep their plain text. Enumerators have the value the compiler computed.

**Where it runs.** The header index worker does, after `locateNames` has published the declarations:
the definitions and the bodies, at the same time. Each publishes its own result when the slot is still the
current one, within the memory budget of the index. The member analysis (the variables) asks for the
declaration of the receiver's own type with its second step (a plain or qualified name only, not a
template), and runs the bodies as a third step after the second step is published. Hover on a variable
of a struct, union, enum or class type then shows the native type, the comment of the type, and the
definition in a second C++ block (`hoverResult.typeBody`); a type that is only declared shows no block.
Nothing in a request runs or waits.

**Cache key.** `locateKeyVersion` is `locate-v6`: a declaration has its compiler type, the canonical types of its parameters, the tag of a typedef (with its qualified name) and the flags `Defined` and `Redeclares` (not set for a static function). A body run has its own key with the version, the tag name, the file and the range.

## Alternatives and why not

- **Members through the existing member machinery** (names from the member probe, one lookup for each
  field in a class derived from the record): the names do not come in declaration order, an unnamed tag has
  no receiver to derive from, the fields of nested anonymous unions are `IndirectFieldDecl`s,
  which the lookup does not print, and enumerators have no members. The filtered dump has everything in order.
- **The unfiltered AST dump, or one filter for a batch**: the filter takes one string, and an unfiltered
  dump of the standard headers is tens of megabytes (ADR-122). One run for each typedef of a tag, for the
  few types that a program writes, is cheap (0.1 to 0.2 s for raylib) and is cached.
- **Definitions by a text search in the sources**: a function in a macro, in a namespace, an overload, a
  static function of the same name in another file are all wrong answers the search cannot tell from
  right ones. The compiler's lookup after the include does not guess, and the dump says whether the
  definition redeclares the header's declaration: a definition that does not is not used.
- **One probe file with all sources included**: two sources that define the same static function would
  fail together. One run for each file is independent.
- **Reading the folder in the request**: no request reads the disk for this.

## Consequences

- Hover on `raylib.Vector2`, `raylib.Color`, `raylib.ConfigFlags` shows the fields or the enumerators;
  hover on `v` of such a type shows the type and its definition; go to definition of a function that
  `geo.cpp` defines goes to its body, and go to declaration to the header.
- More compiler runs after the declarations (one for each source file, up to 16, and one for each typedef
  of a tag), in the background, cached; none is on the path of the names, the declarations or the
  members. The budget of ADR-126 is for the data that was there before and is not changed.
- A cold start with many typedefs of tags in one file (up to 16 are asked) has up to 16 more runs, bounded
  by the same compiler slots as the other runs.

## Gaps

- Member functions that a source defines go to the header; so do the members of a class.
- A function of an unnamed namespace that the source declares first and defines after that, with the same
  parameter types as the header's function of the same name, is taken for the header's function: the lookup
  dump does not print the namespace, and telling it would take one more run for each source (`-ast-dump-lookups`).
  Such code cannot call either function without an ambiguity. With other parameter types, and a `static`
  function whatever its parameters, the header is shown. A header function that is `static` is not asked for
  a definition.
- A source whose chain run fails (the second run for a name that the source redeclares) fails as a whole, and
  is asked once more after two seconds; the definitions of the other sources stay.
- A header edit asks every source again (the stamps of all of them include the header); a source or header
  modified less than 2 s before a run is not kept by the memo.
- Implementation and type definition of C/C++ names; signature help and completion details for C/C++
  functions (they carry no declaration).
- A tag that a macro or a header-local typedef names (`typedef Vector2 Point;` is a typedef of a typedef
  and keeps its text); a typedef of a class template instance.
- Windows paths through real clang are unverified (the PC has no `clang++`); the parser and path
  functions are unchanged.
- Sources are found by the extension and not by the build constraints of the package.

## Tests

- `probe/locate_tag_clang_test.go` (real clang, and raylib through pkg-config when it is there):
  struct, unnamed struct with bit-field, array, function pointer, nested unnamed union and struct,
  anonymous enum with computed values, a named enum behind a typedef, a typedef of another name, a
  forward-declared tag, a function pointer typedef, a plain typedef, the 40 field bound, and the doc and
  place of the typedef kept. `probe/locate_fake_test.go`: the fake compiler's answers, a warm start with
  no compiler run, an edited header asked again, no definition, a failed compiler, a stopped request.
- `cache/foreigndefs_test.go`: definitions of a function and of a variable, none for a function no source
  defines or for a type, a source with an error gives none and loses no declaration, no run without a
  source, the bodies of the typedefs, a failed body step leaves the typedef.
- `server/declaration_test.go`: declaration against definition of a function and a variable, header for
  both when no source defines, types, a Go name, no answer without facts, hover of struct, enum and
  function pointer typedefs, hover of variables (Markdown and plain); the slow-clang tests ask
  `textDocument/declaration` while the compiler is blocked.
