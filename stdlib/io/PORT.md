# io

Step 9.6 (Phase 09) **copied** go1.25.14's `src/io` (`io.go`, `multi.go`, `pipe.go`; BSD-3-Clause, see `../LICENSE`) without
changes, because `os` needs `io.EOF`, `io.ErrShortWrite`, `io.ErrUnexpectedEOF`, `io.Copy` and the interfaces. `io` imports
`errors` and `sync`, both real. **Step 9.4 (`tools/portstd`) replaces these files with the tool's output**: the files are
Go's, so the replacement is `go run ./tools/portstd io` and a check that `make portstd-check` is clean; the declarations
(and the identity of `io.EOF`) do not change.
