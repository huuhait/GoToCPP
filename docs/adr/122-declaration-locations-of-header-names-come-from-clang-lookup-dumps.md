# ADR-122: Declaration locations of header names come from clang's lookup dumps

Status: accepted (2026-10-21)

## Context

Step 12.13c2 gives hover and go to definition for the names that Go source writes after a header
package or a namespace (`raylib.InitWindow`, `std.string`) and for the members of a variable whose
native type is proven (`win.size()`): the declaration as the compiler reads it, with the header's
comment, and a jump into the header. The existing bounded header index of step 12.13b2 (`-Xclang -ast-list`) has names only. The JSON dump with a filter has locations, but one dump of a
filtered name per name is a run each, and an unfiltered dump of `<string>` is tens of megabytes.
Member declarations of a class template (`std::string::size`) and inherited members need the
compiler's own lookup, not a text search.

The constraints of the job: no request probes or waits, nothing slows the cold open, a stale
answer after an edit or a header change is a bug, an overload set is shown whole or not at all,
and typed mode never rejects what opaque accepts.

## Decision

- **The mechanism is `#pragma clang __debug dump NAME`.** It prints the text AST dump of every
  declaration an unqualified lookup of NAME finds at that place, each overload included, with
  locations. One probe file has one pragma per name (`namespace A { namespace B { #pragma ... }}`
  for a qualified one) and ends with a pragma for a variable that the file declares, which proves
  that the dump reached its end. Members are looked up inside
  `struct gocpp_loc_class_N : public Receiver { void gocpp_loc_method() { #pragma ... } };`, which
  finds inherited members and the instantiated members of a template, with the locations of the
  pattern. A compiler error inside such a span fails that lookup only (a base that could not be
  derived from, a `final` class, would make the lookup find a name of an outer scope); an error
  elsewhere fails the run.
- **The dump is read as a stream** (`probe.dumpReader`): the dump of a class template is large, so
  only the lines that the renderers read are kept (limits: 8192 nodes a lookup, 4096 lookups, 48 MB
  of output), but every line moves the state of the elided places (`file:line:col`, then `line:`
  and `col:`), so every line is parsed. A file name may contain colons, spaces and commas: the
  end of the name is the place that names a file of the run's dependency list.
- **A place is trusted only after checks:** the file is one of the run's dependencies (absolute,
  cleaned; a `..` goes through `EvalSymlinks`, because a link before it makes the lexical parent
  another folder), and the text of the file at the compiler's line and byte column starts with the
  name. The byte column becomes a UTF-16 column in the worker. A declaration that no file spells
  (`<scratch space>`, a token paste) fails its overload group.
- **The text of a declaration is rendered from the dump**, not copied from the file: macros,
  attributes and bodies are gone, typedef names stay, names are qualified as the Go source writes
  them, non-literal defaults print as `= ...`, records show their public members (24) and enums
  their enumerators (16). Using declarations whose target is written from the global scope
  (`std::fopen`) are followed in up to three runs; a relative target fails the group. Macros come
  from `-E -dD`: the `# N "file"` markers give the line of each `#define`, validated against the
  text; the last `#define` wins, `#undef` deletes.
- **Only the latest declaration is known.** The lookup answers with the latest declaration of a
  name and the dump says only that an earlier one exists (`prev`). A class, class template or enum
  that is not a definition but has a `prev` is therefore refused: it may follow its definition,
  which is somewhere the dump does not say (`std::vector` once `<format>` has redeclared it). A
  declaration without `prev` is shown as it is (`struct S;`). A set of overloads larger than 64, or
  whose dump was cut at the node limit, is refused: complete or absent.
- **Doc comments** use `-fparse-all-comments` and `-fretain-comments-from-system-headers` (clang
  keeps no comment of a system header otherwise, and a nix wrapper makes every `-I` of the store
  a system directory, so libstdc++, glibc and sqlite3.h would have no doc): a documentation comment (`///`, `//!`, `/** */`,
  `///<`) wins, then a plain comment on the last line of the declaration, then the plain comment
  above. raylib puts a group heading (`// Window-related functions`) above the first function of
  each group, so a trailing comment must beat a plain leading one.
- **The facts live where the names live.** The worker of a header slot (`cache.headerIndexer`)
  asks for the names that the files of the slot write (`foreignNames`, from the rewrite's
  provenance, so a shadowed `raylib` is no use) right after its index is published, and keeps the
  answers in the slot (`headerEntry.decls`), after it has let go of its probe slot. The member
  worker publishes the member names and types first, and then asks for the members that the
  program writes on bindings of each receiver (a second step that gives the entry a copy of the
  facts with the declarations, while the entry is still the same). Both read
  through snapshot-scoped, exact-binding lookups (`ForeignDeclarations`,
  `ForeignMemberDeclarations`) that return copies and nothing for variants, repaired or moved
  views, a canceled or replaced slot or an over-budget cache. The slot is dropped on
  `HeadersChanged` and read again in the background.
- **The cache kind is `l`**, with the key prefix `locate-v3`, in the runner's variant files with
  dependency-hash validation, so no old entry is read as an answer, and no old reader sees it.
  Its content is the answers of one run; the dependencies are those of the run.

## Consequences

- The cost is background only. Measured with the compiler of the dev shell on the headers of the
  library tests: raylib (10 names) 71 ms, sqlite3 (8) 73 ms, SDL2 (6) 482 ms, `std::string`
  265 ms, a member group of `std::basic_string<char>` under 1 s; each result is cached on disk.
  Hover and definition read a map under a mutex.
- A C struct behind a typedef (raylib `Vector2`) shows `typedef struct Vector2 Vector2;` without
  its fields: the pragma cannot name the hidden tag. A declaration from a header that a macro
  generates in a way no file spells has no answer.
- The text of the dump is clang 21's. A different format is not guessed at: a heading that does not
  match, a node whose parent is missing or an unknown shape fails the run, and the answers fall back
  to the earlier hover.
- There is no `textDocument/declaration`: the server has only definition, which goes to the
  declaration in the header.
