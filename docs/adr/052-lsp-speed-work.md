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

## Review round 3: the reach of the dynamic family

Round 2 followed an asserted interface to **every** type of the module with the interface's
methods. The reviewer's `falsefam` (30 packages with `Close() error`, one `core.Closer` asserted)
made a rename of one unrelated `Close` 32 edits in 31 files, and the large project (`core.Adder`
asserted in `cmd/app`) 3,004 edits in 1,502 files and 0.8-1.3 s: the explosion the narrowing was
meant to remove. And the search found the assertion only in packages that it was already looking at
(`core.IsCloser(x any)` asserts `x.(Closer)` on what `main` passes it, `core` does not import the
type's package): the output of the program changed with no error.

- **Reach.** A value can come out of an assertion as an interface only if it was converted to an
  interface (any interface, `any` too) somewhere: a type that is converted nowhere cannot be the
  dynamic value of anything. `scanTies` now records the types of the values converted (passed,
  assigned, returned, sent, put in a composite literal or converted, and the type arguments of an
  instantiation), per checked package (`tieInfo.converted`, memoized with the ties). A type of the
  module joins the dynamic family only if its methods have the shape of the asserted interface and
  it is converted somewhere. This is conservative (the value may be converted and never reach *this*
  assertion) and cheap; the finer test, the flow of the value through the import graph, was not
  worth its cost. Where a conversion can be: in the package of the type or in a package that imports it,
  directly or not (a package that does not import it has no expression of its type), so for a type
  of the family the views of its importers, which the search has, decide; the rest of the module is
  searched (once) only when the interface's method is in the family and the other implementers have to
  be found.
- **Rename only.** References and highlights do not follow assertions (they say where *this* method
  is used); the rename does.
- **Where the assertions are** (`dynindex.go`). A table of the files by content hash (the parse
  cache feeds it, as the workspace symbols do) says which files declare an interface with a method
  of the name and which have an assertion, a type switch or an `As` call. Only the packages that
  have an assertion and can see such an interface (the packages that declare it, and those that import
  them) are checked for the target. A rename of a method that no interface of the module has costs one
  pass over that table (about 1 ms on 1,500 files).
- **Limit.** When the dynamic rule would add more than 20 methods (`Options.DynamicLimit`,
  `gocpp lsp -dynamic-limit n`, `-1` for any number) the rename is refused with the count, the
  assertion's place and the flag, instead of editing across the module on a guess.
- Measured (`falsefam`, the large project `core.Adder`): 4 edits (the two converted types, the
  interface, the call) for `falsefam`; 2 edits and 12 ms warm for a rename of one `Add` of the
  large project (was 3,004 edits and 0.9 s); references 2 locations in 2 ms (was 3,004).
  `falsefam3` (the assertion in `core`): the 4 edits that dev makes.
- Fields of an anonymous struct type are renamed in every identical struct type (`anon.go`); a
  method of an interface literal is named `pkg.(interface literal).Name` in the messages.

## Review round 4: the reach test made sound, the cost of the check of the result

- **The reach test of round 3 was unsound** (twelve probes of the reviewer: a rename that gave the
  same output on dev changed it): it recorded a type as converted only for the single-value call,
  assignment, return and send, by the type's name. It missed the values of a call that returns several
  (`return f()`, `h(f())`, `a, err = f()`, `var a, err any = f()`), the comma-ok forms (`a, ok = m[k]`,
  `<-ch`), a range into an assigned variable, the key of a map index, and every type that is not a
  named type of a package scope (a struct that embeds the type, anonymous, local, generic or an alias).
  Now `scanTies` records the **types** (not their names) of the values that flow to an interface, in all
  of these forms and in the comparison, the switch on an interface and the type arguments of every
  instantiation (the generic code may convert them), per checked package (`tieInfo.converted`). The
  implementers of an asserted interface are exactly those types: a method lookup on a struct that embeds
  `T` finds `T`'s method, so the method that is renamed is `T`'s whatever carries it, and the types
  that are not at the package scope need no table of their own (`methodTypes` is gone). The twelve
  probes and the working ones (50 shapes: the reviewer's `c`/`m` series) are table tests
  (`family_probes_test.go`); the old rule fails exactly the twelve.
- **The check of the result** leaves out the packages that the rename cannot affect: for a method, a
  package that the search looked at and the edits do not touch can break only by using the new name
  (`mayBreak`); the others are checked. The search closes the family over every tie of the packages it
  looked at, so those packages have no tie to an interface that is not renamed with the type. This
  took a rename of 10 methods on the large project from 212 packages type-checked after the edit to 5.
- **A search of 100 packages thrashed the check cache** (`defaultMaxCheckBytes`, 100 MB: the packages of a
  large project that a rename of a shared method searches do not fit, so each search checks about 95 of
  them again): 266 ms warm for that rename, 57 ms with 300 MB. The cap is the memory budget of the
  server (README §2.1) and was not changed here; it is the knob.
- **Settings**: `dynamicLimit` is also read from the `initializationOptions` and `workspace/didChangeConfiguration`
  (`gocpp.dynamicLimit`), and the refusal says so. Test files take part in the syntax index.

## Review round 5: narrowing that was not conservative enough, again

The two blockers of the recheck were of one kind: a rule that left something out on the argument that
it cannot matter, where it can.

- **Promotion.** The method set of a type that embeds the renamed method's type is made by
  promotion (the shallowest name, an ambiguity at a depth hides all), so renaming `A.M` to `N` can make
  `W{a.A; b.B}.N` ambiguous, hide `W.N`, or let `A.N` hide a deeper `N`, in a package that never names
  `N`, and `W` stops implementing an interface (a compile error that the pruned check of the result
  missed) or calls another method (no error at all: the case that no check can see). Now the check of the
  result keeps every package that ties a value to an interface that has the old or the new name, as well
  as every package that uses the new name; and `checkEmbedders` refuses the rename when a type that holds
  the method by embedding (any depth, any package the search looked at, structs and interfaces,
  anonymous, local or generic) already has the new name at any depth (the embedder's own, a deeper one,
  or the same depth). The embedding itself is not a reason to check a package: it breaks nothing until
  a method is selected or a value is passed on (a use of the name, a tie).
- **Reflection.** The reach set of a converted type now includes what it is made of, as far as it goes:
  the fields (all of them), the elements of a slice, an array and a channel, the keys and values of a map,
  the pointee, and the parameters and results of a function; `reflect.ValueOf(S{&T{}}).Field(0).Interface()`
  and `fmt` of a nested struct reach `T`. A type of the standard library is not looked into. A comparison with an
  interface records nothing (it keeps nothing), and the type arguments need no code of their own (the
  constraint is an interface, so `add` records them): both were redundant.
- **Cache cap.** The default of the check cache follows the machine (10% of the memory, 64 to 300 MB)
  and is a setting (`gocpp.checkCacheMB`, `-check-cache`); with 100 MB a rename of a method that a large
  module shares thrashed it (95 packages checked again per request, 400-660 ms warm; with the new default
  8-15 ms) and the resident size under that load went from 500 MB to 360 MB; at start it is the same
  (88 MB). Under a long editing session the cache can hold more old versions before it drops them (up to
  the cap).
- **Strings.** A rename of a method or a field warns where a string literal of the module is its name
  (`MethodByName("Close")`): reflect finds it by the string, which a rename does not change. This is
  a warning, not a refusal; the program is not known to depend on it.
- Probes: the reviewer's c, m, mb1 and c50-c59 and 29 of mine (promotion at the same depth, shallower, deeper,
  a field with the new name, interface embedding, reflect over fields, slices, maps, arrays, channels, function
  results, named pointer types, nested structs, a walk over a struct tree, a swapper, a select send, a
  method expression, a defer of `any(t)`) run the program before and after: 96 probes, none changes
  its output unless refused. The shadowing and ambiguity ones are the silent changes that compile; they
  were accepted by every earlier version, dev's too.

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
