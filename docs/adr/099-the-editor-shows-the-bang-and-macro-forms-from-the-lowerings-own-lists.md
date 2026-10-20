# ADR-099: The editor shows `!`, macros and `#Derive` from the lowering's own lists

Status: accepted (2026-10-21), step 11.13. Builds [ADR-015](015-language-server.md) (the server),
[ADR-037](037-must-rewrite.md) (`!` is a rewrite of the Go tree), [ADR-075](075-user-macros-are-generic-functions.md)
(a macro is a generic function the loader writes), [ADR-080](080-derive-writes-methods-from-the-types.md) (`#Derive`
writes methods), [ADR-091](091-expand-prints-the-go-the-compiler-works-with.md) (`gocpp expand`) and
[ADR-095](095-must-and-macros-are-always-available.md) (the forms are always on).

## Context

By the time the server sees a package, every `F!(x)` is an `if err != nil` statement, every macro use is a call
with hidden arguments, and every `#Derive` line is a stub method with the place of the generator's name. Steps 10.9
and 10.10 already made the old features right on those trees (hover shows `macro Dbg[T any](v T) T`, definition of a
derived method lands on the `#Derive` line, no token or hint for a name of the lowering). What the user still could not
see is what the forms *do*: where the error of a `!` goes, which call a macro use becomes, what hides behind a must
function's signature. The constraints:

- a keystroke, an open, a hover, a references or a rename must not get slower for it (the server is held to gopls
  on every row, `docs/benchmarks/lsp.md`); a package without the forms must pay nothing;
- the answer must be what the compiler really does, not a second model of it (the lesson of ADR-091);
- positions: the lowering gives its nodes the places of the user's text, so a lookup by place finds the user's name
  or a name of the lowering, and the existing code tells them apart with `File.Synthetic`.

## Decision

1. **The check keeps lists, and the server reads them.** `check.MustInfo` (already there for `gocpp vet`) gets what an
   editor needs and `vet` does not read: the place of the `!` and of the callee's name for each written call
   (`Bang`, `Name`), the calls that a must function handles without a `!` in the text (`Handled`), and for each
   source statement that the lowering rewrote the statements that stand for it (`Stmts`, recorded in `lowerList`, with
   the span of the statement as written). Nothing is computed from them until a request needs it: the lists cost one
   append per rewritten statement in the build, and an index of them (sorted by place, a map by the callee's name) is
   made once for each checked package with `cache.Package.Memo`. A package without a `!` form has no list and no
   index; its requests pay a `nil` test.
2. **The code that stands for a statement is printed from the tree the compiler lowers.** Hover on `F!(x)` prints
   the `MustStmt` that holds the call with `go/format` and the package's `UserText` (the same clean-up `gocpp expand` uses),
   so the hover shows `p, must_e2 := port(name); if must_e2 != nil { return 0, must_e2 }` and not a template. The
   names of the lowering (`must_e2`) are shown as they are, with a note; hiding them would need a renaming that can
   clash in a statement with two `!`. What a failing `!` does (return, `t.Fatal`, panic, exit 1; the rules of `defer`; a
   closure) comes from `MustCall.Kind`, which the lowering computes from the types, and is said in a sentence.
3. **A package with errors is not lowered, and the editor still has the places.** The rewrite stops at the first error of
   the package, so `Must` is nil for it, and every `!` feature would blink off with each keystroke that leaves an unused
   variable. `check.Package.MustPartial` is made then (`mustPass.partialInfo`): the places of the calls, the callee
   names and a `Kind` from the partial type check, using the lowerer's `newCtx`/`isTesting` rules without rewriting code.
   This includes named `testing.TB` parameters, exact testing-package identity, return-policy priority and a closure's
   inherited testing context; an unrelated error must not change Fatal to panic. Main/init exit handling uses the same
   recorded must-function/entry-point conditions, and the conversion to `MustKind` is shared with lowered calls.
   Hover, hints and tokens use it; the code that stands for the statement, and the expansion, need a package without
   errors. `vet` still sees `Must == nil` for such a package.
3a. **Macro uses** are read from `load.Package.MacroUses` (the callee and the call the rewrite wrote), the hidden
   arguments by their order (file, line, func, the texts, the code of a wrap macro, then the user's). The hover prints the
   call, with a func literal argument shortened to `func() T { ... }`; a lazy parameter is found by the position of the
   argument (the instantiated signature has lost the `gp_L_` type parameter). Hints never name a `gp_` parameter, and
   never a synthetic argument.
4. **Semantic tokens** add the standard types `macro` (the name of a macro, the `#` of a use, `#Derive`), `operator` (the `!`) and
   `keyword` (`lazy`), at the end of the legend, so a client that knew the old legend is unchanged. The `!` and the `lazy`
   are not names of the tree: they come from the places in the lists and from the text.
5. **Completion** after `#` and `#pkg.` is its own small function (`macrocomplete.go`), run before the normal completion
   when the text before the cursor says so: the parser reads `#Assert` as a name with a `#` in it, which a scope lookup
   does not know. It uses the package's macros (and the imports', which the summaries keep) and works while the file
   has the stray `#`. The `#` is a trigger character.
6. **Rename** never changes a `!` (the identifier of the tree has no `!`, so the edit does not reach it) and accepts a new
   name typed with one for a must function, or with a `#` for a macro, by dropping it; for any other symbol it is an
   error that says what the mark means.
7. **"Show expansion" is a code action whose command runs `gocpp expand`'s printer.** The printer moved from
   `cmd/gocpp/expand.go` into `internal/expand` (`File(fset, pkg, file, known, full, readSrc)`; the command keeps its flags,
   its output and its tests, and calls it); it also returns where each printed declaration stands in the result. The
   server loads and checks the package from the snapshot with the bodies of `#Derive` written and the nodes of the loader
   recorded (`Snapshot.CheckExpand`, like `CheckTests`: from source, uncached, one semaphore place), prints the file and writes it
   to `<name>.gpexpand` in a folder of the server's (removed when it ends), and asks the client to show it with
   `window/showDocument` and the declaration selected. The action is offered by an index of the marks of the package
   (sorted places of `!` calls, must functions, macro uses and declarations, `#Derive`): the declaration around the cursor
   is expandable when a mark is in it, and a package with none answers at once, before a query is made. The file has its
   own extension, which the VS Code extension colors as gocpp, and which the server makes no package of. A `#Derive` line
   precedes the type, so its action uses the loader's exact `Decl`/`Spec` association rather than requiring its mark to lie
   inside the declaration. The directive, intervening comments and type select that type's generated methods; a supported
   one-spec group works, while a rejected multi-spec derive does not acquire another type's action. Printer span metadata
   associates inserted methods with their source type without changing the emitted expansion text. An on-top wrap directive
   also selects the function's expansion, not an empty selection.

8. **One quick action edits**: "Handle the error of f with !" (`bangactions.go`, `refactor.rewrite`) inserts the `!` after the
   name of a call that drops its error where the rule of the spec allows one (a last result that is exactly `error`; a
   statement or the value of an assignment that wants the other results). The edit is one insertion; the checker
   decides after it. A parenthesized callee such as `(f)()` or `(pkg.F)()` is not offered this insertion: `(f!)()` is not
   valid `!` syntax, and safely removing parentheses/comments would need a different rewrite. Parentheses around a method's
   receiver, `(v).Method()`, do not enclose its name and remain supported. "Make `f` a must function" (change the signature and every `return`) is not built: it needs the whole
   rewrite of a function and is better done by hand with the hover's help.
9. **A syntax error does not take the `!` features of the rest of the file** (review round 1). The parser loses every declaration
   after an unclosed call (it takes the tokens after the missing `)` for more arguments, up to a `)` in some later function), so
   the tree of a file that is being typed often ends in the middle. The packages with load errors get the `!` pass too
   (`checkPartial`: the places of the forms in the files that parsed). For the rest, `repair.go` blanks the declarations the first
   syntax errors are in (every byte but the line ends becomes a space; a declaration is found by a column-one lexical token,
   `func`, `type`, `var`, `const`, `macro` or `#`, never by declaration-looking text inside strings/comments; an unterminated raw
   string or block comment conservatively consumes its uncertain tail; an error on the first line of a declaration blames the one before, for an
   unclosed brace), up to six, parses the result, and checks it with `Snapshot.PackageWithFile` (the check completion uses for a
   fixed text). Hover, definition, hints and tokens (`newViewQuery`) answer from that check outside the blanked declarations and
   from the partial check inside them; the repaired check is a cached check of its own text, made on request. References and rename
   keep the partial check (renaming without the blanked declarations would be wrong). The test deletes tokens of the macro
   project's file and requires every hint and `!` token outside the damaged paragraph to stay.
10. **Names that the lowering moves** (a loop's condition, the second operand of `&&`, a chain, a deferred call, a var group, a
   package variable's initializer) keep the callee's node in the call's record (`MustCall.Callee`), so the hover is found by the
   place of the name in the source, not of the node in the tree. `main!` and `init!` are found by `MustFunc`. A `!` inside
   `#text(...)` is text, not a call. `identAt` takes in the nodes that a wrap macro makes: its call ends before its last argument
   (the code of the function), and the func literal that holds the code starts at the function's brace, after the uses of the
   macros below the first.
11. **Show expansion files** are `<hash of the source>/<name>.gpexpand` (two `f.gp` in two packages do not overwrite each other), in
   a folder with a marker file; a server that is killed leaves it, and the next one that writes an expansion removes folders
   named like ours, with the marker, owned by the user and older than a day. The folder is not made again after the end of the
   server.

12. **The cost of a keystroke in a file with a syntax error** (review round 2). Round 1 made the partial check of a package with
   load errors do the `!` pass and up to eight more type checks, on the diagnostics path, for every keystroke: 2 to 5 times
   dev's cost in a package that uses `!`, and the repaired check on top for every request. Now:
   - the partial check is **lazy** (`check.Config.LazyPartial`, `Package.PartialResult`): the diagnostics of a package with
     load errors are the load errors and cost no type check at all (dev made one), the partial check is made by the first
     request that needs it, once, and without the extra rounds (the written `!` forms are in its list, the calls a must
     function handles without one are not: they come from the repaired check).
   - a function with a syntax error in its **body** is blanked down to the body (`func f(...) T { panic("") }`): its signature
     stays and its callers check. Only a broken signature blanks the whole declaration. A load error that is not a syntax
     error (a macro that is not there) blanks the body that has it. A hover that comes out null or `invalid type` falls back to the
     check of the text as it is.
   - the repaired check is a **view** (`stale.go`) built in the background, one at a time per file, of the newest text,
     at most every 100 ms (or 1.5 times the previous build duration) and at once for a request that cannot wait.
     Its identity includes the snapshot, not just the file hash: changing a sibling or imported API must invalidate answers
     even when this file's spelling is unchanged. Cross-snapshot reuse requires a verified contiguous run of successful edits
     to this file alone; an unknown event, open/close, disk invalidation or recheck conservatively forces a current build.
     Reuse is allowed only for an edit wholly inside the same blanked **function body**, with its lexical/declaration boundary,
     every character outside the body, and its line count unchanged. Signatures, macro bodies, newlines and uncertain boundaries
     rebuild. The old partial hints/tokens of the edited body are omitted rather than mapped to unrelated new text; a hover
     inside that body waits for its current partial check. It goes directly to that snapshot's original partial query: the
     repaired-view path would discard the repaired check at that place and use the same original query anyway. It therefore
     does not build/wait for an unused whole-package repair before answering. Other features keep their repaired-view policy;
     whole-file hints/tokens still request repairs and refreshes, and definitions still require a current view. Unaffected
     answers outside the body are mapped to the current offsets.
     The completed build asks supporting clients to refresh hints and tokens. A request that retained an old candidate
     while another request finished the current build takes that current view instead; publication and stale-serving
     decisions share the view lock, so a stale flag cannot be stranded after the refresh decision. Definition always requests a current view;
     references, rename and code actions never use repaired or stale views. Off with `GOCPP_LSP_NOSTALE=1`.
   The recovery validation, unique-edit harness, exact timings and remaining limits are recorded in
   [lsp-step-11.13.md](../benchmarks/lsp-step-11.13.md); earlier permissive-stale measurements are not evidence for these stricter guards.
13. **The folder of the expansions** has a marker with the process id, touched at every write and every hour. On Unix, a later
   server removes a folder only if its marker is older than a day, the process is gone and the folder is the user's. On Windows
   (and the other non-Unix fallback), cleanup relies on the per-user temporary folder, our marker and its age; it does not check
   process liveness or ACL ownership. A live Windows session suspended for more than 24 hours can therefore lose its expansion
   files when another server cleans up; writing an expansion again recreates them. The hourly touch protects an awake session,
   not a suspended one. Process/ACL-aware Windows cleanup is not part of this step.
14. **Ordinary user names and partial policies stay ordinary** (independent correctness review). A library function named
   `main` returns an error or panics by the same rule as another library function; only the actual main-package entry point
   or a recorded exit-policy must function gets the exit explanation, even when an unrelated error prevents lowering.
   Parameter hints skip macro hidden slots using `MacroUse.Sig`, not a `gp_` name prefix. Result-name elision requires a
   synthetic identifier or exact source-position evidence. Positive synthetic object identity is checked before position
   validity: a named error result added by `prepareFunc` has `NoPos` until lowering anchors it, but its origin is known.
   Unknown/inexact export-data origins conservatively retain their
   names. A legitimate imported `F(must_r int) (must_err int)` therefore keeps both user names.

## Alternatives

- **Print the expansion of the one declaration** instead of the file: the printer works on edits of the source file; a
  declaration alone loses the imports and comments that a reader needs. The file with the declaration selected is what
  `gocpp expand` shows and is easier to diff.
- **A virtual document in the VS Code extension** (a `TextDocumentContentProvider`): nicer in one editor, nothing in the
  others. The file and `window/showDocument` work in every client that can show a document, and the extension needed
  only the extension in its language list.
- **Compute the `!` info in the editor from the text** (a scan for names that end in `!`): it would show tokens while a file
  has errors, but it knows no kinds and no types, and is a second parser. The partial list is the same idea with the checker's
  own places.
- **Rename the temporaries to readable names in the hover** (`err`, `v`): not unique in a statement with two calls.
- **Put the lowered statements into the cache's summary or the disk cache:** they are needed for the open packages only, and
  are rebuilt with the check.

## Consequences

- A package without `!`, macros or `#Derive` pays: a `nil` map test in hover, a loop over zero derives, a loop over the file's
  declarations for the macro `lazy` keyword (a map lookup each) in a semantic-token walk that is memoized per file, and a
  prefix test of the text in each completion request (`macroCompletion` returns at once). The benchmark rows of the existing projects
  did not move beyond the noise (A/B of 12 rounds and pinned microbenchmarks: [lsp-step-11.13.md](../benchmarks/lsp-step-11.13.md)).
- The new rows (the macro project: 71 files, 8,000 lines; every file has `!`, macros and `#Derive`): hover on a macro use,
  hover on a `!` call, inlay hints for a whole file, the tokens, completion after `#` and the code actions are all
  sub-millisecond at p50 and under 3 ms at the maximum; they have budgets (30 ms; 50 ms for completion).
- The files `_test.gp` get no hover, hints or tokens (the pipeline checks a package without its test files; `CheckTests` loads
  from source and is not cached across edits). That is where `!` and `t.Fatal` matter most, so it is the first thing to do
  when the pipeline learns to check test files.
- Partial failure policies use the type information that the partial check could recover, not a second signature-spelling
  approximation. If a damaged signature's types cannot be recovered, the editor cannot promise its intended policy; it does
  not invent a testing context from another package's `T`/`TB` spelling. Unrelated errors retain the compiler's known policy.
- "Show expansion" costs a load of the package and its imports from source, one to three seconds for a package that imports
  a few standard packages; it holds a place of the check semaphore meanwhile. It is a command, not a request on the keystroke
  path.

## Evidence

- `internal/lsp/server/bang_test.go`: hover (every place a `!` can be, the must function, the handled call, markdown),
  inlay hints (all kinds, a range, the tooltips, no hint for the lowering's own code), semantic tokens, completion after `#`,
  rename of `f!` and `#Name`, "Show expansion" (the action where it is offered and where not, the text equal to the golden
  expansion, the selection, an unsaved edit, a package with an error, the file removed when the server ends), the `!` features
  with an error in the package; `macros_test.go` and `must_test.go` for what was already there.
- `internal/check/mustinfo_test.go` (`TestMustInfoEditorData`), `internal/expand` (spans), `TestExpand*` of the command unchanged.
- Benchmarks: `internal/lsp/bench` has the `macro` project and six rows; A/B rounds against the binary of the parent commit
  are in `docs/benchmarks/lsp-step-11.13.md`.
- Mutants (33, each made in a copy of the tree, each fails a test of the packages above): the lists of the check (a handled call
  not recorded, the `!` one byte off, the statements not recorded, the callee's name not recorded, the guessed kind of a package with errors,
  `MustPartial` not set), the texts and labels (the kind of a failing `!`, the `!` before the arrow of an unwritten one, the deferred
  rule, the lazy argument, the count of hidden arguments, the call chosen for a statement, a hidden name in a hint, an argument that the
  lowering wrote, `shownName`), the lookups (the callee's place, `#Derive`), rename (the `!` and the `#` stripped, the must function
  test), the tokens (the `!`, the `#` of a use), completion (the `#pkg.` prefix, the test for a comment, the trigger character), "Show
  expansion" (the declaration chosen, the offered test, the nodes of the loader, the folder removed at the end, the offsets of a span),
  and the quick action (the type of the last result, the number of values on the left, a statement with two results).

## See

[Phase 11 README, step 11.13](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md),
[the editors guide](../guides/editors.md), `internal/lsp/server/bang.go`, `expansion.go`, `macrocomplete.go`.
