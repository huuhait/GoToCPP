# io/fs

Step 9.6 (Phase 09) **copied** go1.25.14's `src/io/fs` (`fs.go`, `format.go`, `readdir.go`, `readfile.go`, `readlink.go`,
`stat.go`, `walk.go`; BSD-3-Clause, see `../../LICENSE`) because `os` needs `fs.FileMode`, `fs.FileInfo`, `fs.DirEntry`, `fs.PathError`,
`fs.ErrNotExist` and the rest (`os.PathError = fs.PathError`, `os.FileMode = fs.FileMode`). Changes:

- `readdir.go`: `slices.SortFunc` with `bytealg.CompareString` is `sortDirEntries` (a stable merge sort by name, in the file).
- `walk.go`: `path.Join` is `pathJoin` (a join for directory names without dot-dot).
- `glob.go` and `sub.go` are **not taken**: they need package `path`, which is not ported yet (`Glob`, `GlobFS`, `Sub`, `SubFS`
  are listed in `stubPackage.Missing` of `tools/stubgen`).

**Step 9.4 replaces these files with `tools/portstd`'s output** (with `path` ported first); the declarations do not change.
