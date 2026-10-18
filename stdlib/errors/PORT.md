# errors

Ported by hand from go1.25.14's `src/errors` (`errors.go`, `wrap.go`, `join.go`; BSD-3-Clause, see `../LICENSE`) in
Phase 05 step 5.7. Phase 09's `tools/portstd` will take over the port; these are the changes it must keep.

- `errors.go`: unchanged, except that the file has no `// Code generated` stub header.
- `wrap.go`: `internal/reflectlite` is not ported. Its three uses are native functions (`//gocpp:native`, ADR-027) that
  the runtime has in `gocpp/errors.hpp`:
  - `reflectlite.TypeOf(target).Comparable()` is `comparableType(target)` (`gp::errors::comparable`);
  - the checks at the start of `As` (`val.Type().Kind() != Ptr || val.IsNil()`, `targetType.Kind() != Interface &&
    !targetType.Implements(errorType)`) are `checkTarget(target)` (`gp::errors::check_target`), with Go's three panic
    texts; the nil test stays in Go;
  - `reflectlite.TypeOf(err).AssignableTo(targetType)` with `targetVal.Elem().Set(ValueOf(err))` is `assign(err,
    target)` (`gp::errors::assign`), and `as` no longer takes the reflect values.
- `join.go`: `unsafe.String(&b[0], len(b))` is `string(b)` (one more copy of the text).
