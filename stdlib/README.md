# stdlib

The gocpp standard library: `fmt`, `strings`, `strconv`, `os`, `errors`, `sort` and the rest, as
`.gp` files, plus native `.cpp` files where a package needs the system. Like the runtime, it is
embedded in the `gocpp` binary, and the loader finds the standard packages here. From Phase 01,
a small Go package in this folder holds the `//go:embed` directive, because an embed pattern
can't reach a parent folder.

It is empty for now.

- **Phase 01** adds declaration stubs for every standard package the target programs in
  `examples/` import, so they type-check.
- **Phases 02 to 08** add the first real code, for example `errors` and a basic `fmt` in
  Phase 05, `cmp`, `slices`, `maps` and `iter` in Phase 07, and `sync` in Phase 08.
- **Phase 09** fills it. Most packages are **ported from Go's own source** (BSD-3-Clause) with
  `tools/portstd`. The tool takes the pinned source tarball of the dev shell's Go version (now
  `go1.25.14.src.tar.gz`, checked by SHA-256), not the local GOROOT, which Nix patches. It picks
  the files for the target OS with the `purego` tag (no assembly), renames `.go` to `.gp`, turns
  `//go:linkname` declarations into native functions, applies a small patch set, and writes
  `stdlib/<pkg>/PORT.md` (upstream version, files, patches). Moving to a new Go version means
  running the tool again. The system parts are **native**: function declarations without a body
  in a `.gp` file, and their bodies in a `.cpp` file next to it.
- **Phase 14** runs `tools/portstd` again for Windows.
