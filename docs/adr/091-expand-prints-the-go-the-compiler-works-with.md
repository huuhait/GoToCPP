# ADR-091: `gocpp expand` prints the checked package's own Go, and copies what it did not change

Status: accepted (2026-10-20), step 10.11. Builds [ADR-037](037-must-rewrite.md) (`!` is a rewrite of the
Go AST), [ADR-075](075-user-macros-are-generic-functions.md) (macros are generic functions written by the
loader) and [ADR-080](080-derive-writes-methods-from-the-types.md) (`#Derive` writes methods), which all
said that `gocpp expand` would show their work.

## Context

Every Phase 10 form is plain Go by the time `go/types` and the lowering see it: the loader writes the macros
and the `#Derive` stubs, the check writes the `!` code and the derived bodies. A command that shows the
result needs no code of its own to expand anything; the question is how to print those trees so that a
person can read them. Three things get in the way:

- the trees carry other rewrites that are not an expansion: a use of `std.vector` is `C.std.vector`, a
  conversion of a C++ value is `gp_cgoh.As[int](x)`, a header import is the import `"C"`;
- the positions of the nodes that the passes wrote are the places of the text they stand for (so that errors and
  `#line` point at the user's line), or the lines of a generated file, or none; the printer takes layout and
  the place of comments from positions;
- `check.Config.NoDerivedBodies` (the editor's mode, ADR-080) leaves the derived methods as stubs.

## Decision

1. **The command is the check, then a printer.** `gocpp expand` loads and checks the package exactly like
   `gocpp check` (same errors, same exit status, the full check with the derived bodies), and prints the
   syntax trees that the passes have left in `load.File.AST`. There is no second implementation of any
   rewrite, so the output can not differ from what the compiler lowers. It lives in `cmd/gocpp/expand.go`; the
   only change elsewhere is a hook, `checkCommand.afterLoad`, called between the load and the check.

2. **The output is the source file with the expanded declarations replaced.** The file is parsed again with the
   syntax of Phase 10, and a declaration is expanded when `gpast.Leftovers` finds a `!`, a `#`, a `lazy`, a `macro`
   or a line of macros in it (a line of macros belongs to the declaration below it). Every declaration of the
   checked tree is matched with its text in the source (by the position of its closing brace or keyword, to the end of
   the line it ends on, or to the end of the declaration when another one is on that line). `gocpp expand` then makes a list of edits of the bytes of the file: the text of a declaration
   that has an expansion is replaced by the declaration printed from the tree (the comments before its code, the
   documentation and the ones on lines of macros, are copied above it, because the printer would put them in the
   parameters); the lines of `#Derive` are taken out, and the comments on them stay; two edits that overlap are an
   internal error, never text that is left out;
   the declarations that the check wrote (the methods of `#Derive`, the `main` that calls `must_main`) and the import
   of `gp_must` are put in after the declaration before them. Nothing else is touched, so everything between the
   edits is the user's file byte for byte: spacing, comments (also the ones between declarations and at the end of
   the file), blank lines, line ends (CRLF, mixed), a byte order mark, a last line without a line end. A file without
   Phase 10 syntax comes back identical (checked over the programs of `tests/run`), the interop rewrites show only in
   the declarations that have an expansion, and `//go:embed` variables stay as written. (The first version printed the
   file declaration by declaration and rebuilt the text around them; that lost comments after a function whose
   signature gained a result, and cut raw strings and block comments in files with CRLF line ends, because the parser
   takes the carriage returns out of their text and the ends computed from it are short.)

3. **An expanded declaration is printed by the fork of `go/printer`, with the comments of its lines.** The
   declaration is printed as the only declaration of a file that holds the comments from its first line to the end of
   its last, so the printer prints every comment it is given (a comment after the closing brace is on the same line).
   `settle` gives the nodes the positions the printer needs: the user's nodes keep theirs; the first token of a
   statement that the check wrote keeps its anchor (the lowering gives the code that stands for a statement its start
   or its end, and that decides which comments come before it), unless the statement goes on what the check wrote
   before it; every other node takes the position of the node before it. A name or a literal that is after one that
   follows it in the tree (a pass gave it the end of its function) is not taken for the user's. The rules were found
   by looking at the output of the must, macro and derive programs of `tests/run`; their effect is only layout, and
   the comments of an expanded declaration can move by a statement.

4. **Traces of other passes are removed in what is printed.** The uses of headers and namespaces are put back
   (`CppUse.Sel.X = Ident`), `check.Package.UserText` turns the `gp_cgoh` forms into the user's text, the
   parentheses that the loader puts around its constants (`(gp_file)`, `("main.gp")`, `(12)`) go, and the import
   of the compiler's own `gp_must` is printed (as its own `import` line) when the printed code uses it. The
   bytes that `#embed` puts into a declaration are shortened to 40 and a comment with the length (`-full` prints
   them): a user's own long string is never shortened (a literal that is not in the source at its place was
   made by the compiler).

5. **The output is Go to read, and for most programs Go to run.** A program without C or C++ and without
   `main!` expands to Go that real Go builds and runs with the same output (`main!` needs the helper
   `gp_must.Exit`; `errors.Wrapf` is gocpp's own). That is how the tests define the output being right
   (`TestExpandRunsAsGo`, which also covers 152 programs of `tests/run`), and it is why the command prints
   the compiler's work instead of a model of it. With more than one file the files follow each other under
   `//// name` lines, in the order of the loader; one file has no header, so the output can be saved and diffed.

## Alternatives

- **Expand on the source text** (a macro processor in front of the loader): a second implementation of the
  macro, `!` and derive passes, which would drift from the real ones, and which can not know the types that `!`,
  `#Derive` and a typed macro need.
- **Print the whole tree with `go/printer` as it is:** the first version. It shows `C.std.vector` and
  `gp_cgoh.As` for every file with interop, loses the user's spacing, and puts the comments of the end of a
  function into the first derived method (the generated file has higher positions than the user's).
- **Keep every position and print the file at once:** comments land inside the code that `!` wrote, and a
  blank line appears wherever an anchor is two lines below the node before it.
- **A `-diff` or a `-changed` flag** that prints only the expanded declarations: easy to add (the item list
  knows which ones changed); not asked for.

## Consequences

- The comments inside an expanded function can move by a statement (a comment goes to the start of the code that
  stands for its statement), and the blank lines inside one are a guess; the code is right (the real-Go tests), the layout is gofmt's.
- `settle` knows how the lowering of `!` and the macro rewrite place positions. A new pass that writes nodes
  must give them positions in the same way, or its output is laid out by the "node before" rule, which is
  always valid and sometimes ugly. The golden files in `cmd/gocpp/testdata/expand` show the layout.
- A package is checked fully by the command (the derived bodies need the second check), which is the cost of
  `gocpp build`, not of `gocpp check`.
- `main!`'s `gp_must.Exit` and the names `gp_*` and `must_*` stay visible: they are what the compiler lowers.

## Evidence

- `cmd/gocpp/expand_test.go`: golden files for the must forms in every context, macros (inside code, start
  and wrap style, lazy, nested, across packages), derive, interop mixed with macros, embed, a tricky file with
  comments in odd places and a package of several files; the expansion of 152 must, macro and derive programs of
  `tests/run` built and run with real Go, with the output and exit code of the program's `// Output:`; a seventh
  of the other programs of `tests/run` come back byte for byte; errors, usage, line endings, blank lines.
- A sweep over all 1944 programs of `tests/run` and 77 package folders of `tests/pkgs` and `examples`: no crash;
  1784 programs unchanged, the 150 that use Phase 10 forms changed.
- Mutants (each fails a test): the overlap test always true or false, the source copy, the blank-line counts, the
  line-ending and package-line handling, each of the `settle` rules, the hidden import, the derived bodies (the
  editor's mode), the snapshot of the loader's nodes, the interop clean-up, the paren removal, the shortening of
  an embedded file and its exemption for the user's own strings.

## See

[Phase 10 README §2.4 and step 10.11](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md),
[the guide](../guides/expand.md), `cmd/gocpp/expand.go`.
