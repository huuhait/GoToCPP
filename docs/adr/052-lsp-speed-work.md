# ADR-052: Language server speed work: no debounce after a fast check, the use-tied method family, per-package indexes

Status: accepted (2026-10-18)

## Context

[ADR-031](031-lsp-speed-gate.md) left the "within 1.5x of gopls" criterion open and named what was
behind it: a fixed 30 ms quiet time before every check, a references search that followed a method
through every interface it could satisfy, and searches whose cost was the size of the package and not
the number of results. The user asked for the server to be optimized as far as it goes
(the rows of [docs/benchmarks/lsp.md](../benchmarks/lsp.md), against gopls).

## Decisions

1. **Adaptive debounce.** The quiet time after a `didChange` is `min(typical check of the package / 2,
   30 ms)`, and none when that took 10 ms or less, where the typical time is the median of the last five runs
   (round 2: a single slow run, a garbage collection on a busy machine, no longer makes the next keystroke wait). Runs of a folder still never overlap
   and a newer change cancels the run under way, so a result for a stale snapshot is never published.
   Changes on disk keep the fixed 30 ms. (Keystroke p50: 31 ms to about 1.4 ms on the small and medium
   projects, where gopls takes 1.4-1.7 ms.)
2. **Rename follows gopls's family; references and highlights use the same use-tied family.** The
   family of a method is the set of methods that a *use* ties together (a value of one type passed,
   assigned, returned, sent, compared, converted or asserted where an interface with the method is
   needed). The interface may be named, an interface literal, or the constraint of a type parameter
   that a call instantiates (the type arguments are tied to the constraints), and the type may be
   unnamed (a struct that embeds a named type has its methods). A type that could satisfy an
   interface, with no use that needs it to, is not in the family: renaming `Lener.Len` leaves
   `Counterer.Len` alone when nothing converts a `Counterer` to a `Lener`. This is gopls's behaviour
   for rename and was the 11.8 decision's cost (0.7-0.95 s for a method that all types share, 600-3000
   locations where gopls answers 2). Ties to an interface outside the workspace (`error`,
   `fmt.Stringer`) are followed for a rename only, to refuse it; the references of the method do not
   join every other implementation of that interface. **An interface that a type
   assertion, a type switch or `errors.As` takes a value out as is the exception, and gopls does not
   follow it: the type that it reaches has no use that ties it to the interface (it may be unexported
   and in a package that the asserting code does not import, reached through `error` or `any`), and
   a rename that leaves its method behind makes the assertion fail at run time with no error from the
   compiler (`errors_cross_as`: the output changed `true` -> `false`; `named_containers`;
   `same_names`). So such an interface's method and the method of every type of the module that
   implements it are one family, from either side (`internal/lsp/server/dynamic.go`: the types are
   compared with the interface as method names and signature texts, where each type is declared,
   because the type and the interface are not in one check; when such an interface joins the family,
   every package of the module is searched, once). This is wider than gopls and than the use-tied
   family, only for these interfaces. Two more guards: an unexported method is searched in the
   importers for ties (`Describe(d)` in main ties `base.Dog.name` to `base.Animal.name`), and the check of
   the result also type-checks the importers of the edited folders; and `init`, and `Unwrap`, `Is` and
   `As` of an error (called by name by the `errors` package) are refused. **gopls's references are not the same thing:** it lists the declaration of the queried
   method and the uses of the methods of every implementing type (tied by a use or not), and not the
   declarations of those. We list the declarations and the uses of the tied methods; matching gopls
   exactly would need the all-implementations enumeration that made the old family slow, so it is
   left open. The 11.8 family stays behind `gocpp lsp -wide-family` (`server.Options.WideFamily`).
   **This is a change of semantics, made on the user's priority (match gopls on every row); the
   manager may revert the default by flipping the option.**
3. **Indexes per checked package** (`cache.Package.Memo`, shared by every request and snapshot that
   has the package, gone with it): the identifiers of the check by spelling (references), the ties
   of the package (family), the encoded semantic tokens of each file. The search of a symbol looks up
   the few spellings of its keys; it does not walk every name of the package.
4. **Per-file tables, not per-query work**: the workspace symbols of each file are kept for its content
   hash (the snapshot knows the hash without a parse), queries run a prepared byte matcher (a
   differential test pins it to `matchSymbol`), the line tables of files that are not open are kept by
   hash, and the folder listing names its test files once.
5. **Rename's check of the result** clones the state of the snapshot (`State.Clone`, the same function
   that an edit uses) instead of starting a session that rereads and rechecks the workspace; only the
   packages the edits touch, and their importers, are checked again.
6. **Small costs**: a plain absolute path becomes a `file://` URI without `net/url`; a references answer
   is encoded by hand (`protocol.LocationList`, byte-identical to `encoding/json`, tested); the formatter's
   safety parse runs while the edits are computed; `metadata.valid` compares reads instead of hashing
   their contents.

## Review round 2 (what changed, and what is not guaranteed)

- What a rename guarantees: the edited program type-checks (the edited packages, their importers
  and the checks with test files that the search loaded are checked again; a new error refuses
  the rename), and the implements relation that a *use or a dynamic check in the module* depends on is
  kept (above). It does **not** keep the relation of a type that could satisfy an interface and is
  neither converted to it nor taken out as it: that type stops satisfying it (no check can see
  it; reflection and code outside the module can). `-wide-family` keeps that relation for every
  type, at the old cost, and has none of the guards of this round (it is dev's behaviour).
  Not looked at: types declared inside functions, generic types, types of other modules (the
  warning of the other modules stays), and a test file that ties without mentioning the name.
- Measured over the 43 programs of `tests/pkgs` (5,260 renames, every identifier, each edit set applied and
  the program run again: `go run` before and after): renames that change the output of the program
  by a change of the dynamic behaviour (not the printed name of a type or field): dev 10 (the same as
  before this round, and 6 more with `Unwrap`/`Is`, `init`: all of them also on dev), now 0.
- Rename's check of the result type-checks the edited package and its importers together (they were
  one after the other), and the selector tables of the conflict check are kept for each checked package.
- A panic in a goroutine of the server no longer ends it: the work of a request (before), the
  handler of a notification and `Handler.Call` (on the reading goroutine), the checks and parses
  (`memoize.Promise` computes on a goroutine of its own: the panic is the error of the promise), the
  goroutines that a search or a symbol query starts (the panic is raised again in the request),
  the debounced diagnostics and the background walks (logged). `Package.Memo` keeps nothing
  when its function panics (it was a spent `sync.Once`, so the later callers got nil and
  failed on every request until the package changed).
- A bogus `internal error: the import of fmt was not resolved with the metadata of ...` (and
  `package ... is not in module`) in a file whose import follows a syntax error or another
  declaration: the metadata is made from the header of the files (up to the last import of the
  `ImportsOnly` parse), and the check parses the whole file, which finds imports later. The file is its own
  header now when it has an error before its imports end or an `import` after them
  (`cache.lateImport`). This was also the cause of `TestRandomEdits` failures under load
  (the incremental and the fresh server disagreed about such a file, depending on the history).
- The root cause of the rest of the `TestRandomEdits` failures (bogus `package ... is not in module`
  and missing importers in a search, in the incremental server or in the fresh one; pre-existing, found
  by dumping the handle of the package when a comparison failed): `state.handle` took the error of
  the header parse for a header when the request's context was done, so the metadata key of a
  cancelled request was the same for every text of the file, and the metadata made for one text
  (its imports) was found by the requests for any other until the package changed. The adaptive
  debounce cancels runs more often (a newer keystroke cancels the run under way), which is why the
  branch hit it more than dev. A request that gives up now returns its error and keeps nothing
  (`TestCanceledHandleKeepsNothing`).

## Not done

- GC tuning (`GOGC`): measured at 200 and 400, it lowers CPU per request by 10-20 % and costs memory; the
  server's resident size is a feature (55 / 160 / 350 MiB against gopls's 600-700), so it is left to
  the default.
- A precomputed cross-package reference index: the search is now small enough on the benchmark
  projects (3 ms on 200 files, 30 ms on 1,500 files and 1,500 locations); an index that must be kept
  consistent under edits is the part of the fuzz test that would catch bugs, and is not needed yet.

## Consequences

- `references-family` is off the `Known` list of the gate (it meets its budget).
- Tests: the differential random-edit test, the family tests (both modes), `TestSymQueryMatchesMatchSymbol`,
  `TestLocationListJSON`, `TestURIFromPlainPath`, the adaptive-debounce tests, `BenchmarkGen`
  (the lsp-bench rows in process, with CPU time per request) and `BenchmarkFormatBig`.

## Limits

- A server panic in the work of one request is answered with an InternalError and logged
  (`protocol.Conn`); it does not end the process.
- `docs/benchmarks/lsp.md` is not regenerated by this work (the machine was loaded during every run).
