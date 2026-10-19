# ADR-058: The built-in macros and `//go:embed` are source rewrites in the loader

Status: accepted (2026-10-18), step 10.8; the clause "only when the parser took the syntax of Phase 10" is superseded by [ADR-095](095-must-and-macros-are-always-available.md). (Numbers 053 to 057 belong to jobs that run in parallel.)

## Context

Phase 10's built-in macros `#file()`, `#line()`, `#func()`, `#text(x)` and `#embed("f")`, and Go's
`//go:embed`, all turn a place in the source into a constant: the file name, the line, the name of the
enclosing function, the text of an argument, the bytes of a file. The plan
([Phase 10 README](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md), "Built-in macros")
says what they give; the open questions were where the expansion runs and how the constants reach C++.

Whatever does it needs three things that only the loader has: the **text** of the file (for `#text`),
the **module** (for `#file`: the path from its root) and the **file system** (for the bytes of an
embedded file, found through the same `FileSystem` as the sources, so that the up-to-date check of a
build, which records the reads, sees a changed, added or removed file).

## Decision

1. **A rewrite in the loader, after the imports are classified** (`internal/load/builtins.go`,
   `embed.go`), only when the parser took the syntax of Phase 10 (`Config.Phase10Syntax`, set when the
   phase gates allow it). Each use is replaced, in place, by a **parenthesized Go constant** that
   keeps the span of the use: `#line()` becomes `(12)`, `#file()` becomes `("util/util.gp")`. The
   result is plain Go: `go/types` checks it (a const declaration or an array length may use `#line()`;
   `var n int = #file()` is the usual type error), the lowering writes a literal, and no later pass knows
   about macros. Every node keeps the position of the use, so errors, the `#line` directives of the
   C++ and the language server all point at the text the programmer wrote.
2. **`#file()`** is the path from the root of the module with slashes (`util/util.gp`), the file name
   for a program without `gocpp.mod`; a `//line` directive changes it (the name is the module-relative or the base name of what the
   directive says, `??` when it has none, so a relative or an absolute name in it can differ from
   what Go's runtime prints).
3. **`#func()`** is the name that Go's runtime prints for code that is not inlined: `main.f`,
   `main.(*T).M`, `main.T.M`, `main.f.func1`, `main.f.func2.1` for a literal in a literal,
   `main.init.func1` for literals in package-level initializers (counted across the files of the
   package), `main.init.0` for `func init()`, `main.F[...]` and `main.(*G[...]).M` for generics, the
   import path (with the linker's escapes) for packages other than main. It was compared with
   `runtime.FuncForPC(pc).Name()` of real Go built with `-gcflags=-l`; gc's inliner changes the names
   of closures it inlines, which no emitter can follow. A known limit: the closures of
   package-level variables are numbered in declaration order, while Go numbers them in
   initialization order (a variable that depends on a later one is initialized after it); the rewrite
   runs before `go/types`, which is where `InitOrder` comes from. A TODO. `runtime.Caller` also
   gives a package folder with a dot unescaped (`example.com/app/my.pkg.Run`), where `#func()` gives
   Go's `my%2epkg`.
4. **`#text(x)`** is the text of `x` from its first to its last token, as written. Where `x` spans
   lines, each line break with its indentation and each comment become one space (so an assertion
   message does not hold the layout of the code); strings, raw strings and every other token stay
   byte for byte. `x` is not evaluated and not checked: it may name things that do not exist.
5. **`#embed("f")`** and **`//go:embed`** give a constant string (`[]byte(...)` of it for a `[]byte`
   variable), with Go's rules for patterns (`path.Match`, no `.`, `..`, empty or absolute parts, a
   folder embeds the files in it without the names that start with `.` or `_` unless `all:`, no symbolic
   links, no folder that is another module, no special characters in names), Go's wording of the
   errors, and the same place for them (the pattern, or the start of the line). `#embed` takes one
   file name, not a pattern.
6. **Limits.** A file is at most 16 MiB and a package embeds at most 64 MiB, so a bigger file is an
   error that says so and not an out-of-memory in the C++ compiler. Measured in the review: a 10 MiB
   random file is about 30 MB of C++ and takes about 1 s and 270 to 300 MB of memory with both
   compilers (-O0 and -O2); a 16 MiB plain literal takes 0.8 s and 220 MB with g++, 1.15 s and 1.1 GB
   with clang++; a package at the 64 MiB limit builds in 6 s (g++, 1.8 GB peak) to 11 s (clang++,
   1.4 GB). The bytes stay a string literal. `.incbin` in `__asm__` (0.10 to 0.14 s for 16 MiB) or
   `ld -r -b binary` (about 0.01 s) are about ten times faster, but lose `constinit`/constant folding
   and are platform specific; C23 `#embed` would need a fallback for the other compiler. They are
   options for later (a TODO), not the decision. Prefer `var` to `const` for a big file: a constant
   is copied at every use that is not folded (six uses of a 16 MiB constant made 190 MB of C++).
   The files are opened without following links and sized on the open file, so a file swapped
   after the listing can't make the read grow without end.
   The stamp of a build records the folders that a pattern lists with every file in them, so a file
   added to such a folder rebuilds (or reports the new error).
7. **`embed.FS`** needs `io/fs`, which is Phase 09: a variable of that type with `//go:embed` is the
   error "embed.FS is not available yet (Phase 09: it needs io/fs)", and the package `embed` is the
   Phase-01 declaration stub (gated at Phase 10; its `FS` type is "not available yet" as before).
8. **`#body()`** is checked here (outside a macro declaration it is an error) and left to step 10.9.
   Inside a macro declaration the built-ins are only validated, not replaced, because there they name
   the place where the macro is **used**; step 10.9 expands them with the use site.

## Consequences

- Nothing downstream changes: a program without macros gets byte-identical C++ (`TestMacroEmitUnchanged`),
  and the rewrite costs a walk of the files of user packages only (the standard library has no macros
  and no `File.Src`).
- A type error that mentions a macro shows the expansion (`cannot use ("main.gp") (untyped string
  constant ...)`), not `#file()`. A later step can print the macro name from a side table.
- The embedded bytes are in the generated C++, so the object cache key (the C++ text) changes with the
  file, and no separate stamp is needed; the up-to-date check of step 6.5 sees the reads through the
  loader's `FileSystem`.
- `//go:embed` works for `string` and `[]byte` today, and `embed.FS` waits for `io/fs`: step 9.x
  replaces the notYet error by a read-only `FS` over the same constants.
