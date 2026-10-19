# internal/filepathlite

Step 9.6 (Phase 09): go1.25.14's `src/internal/filepathlite` for Linux (`path.go`, `path_unix.go`, `path_nonwindows.go`), copied;
the only change is `slices.Insert(b.buf, 0, prefix...)` written out (the generic `slices` function is not ported yet).
`tools/portstd` (step 9.1) can take the package over; `make portstd-check` has no entry for it yet.
