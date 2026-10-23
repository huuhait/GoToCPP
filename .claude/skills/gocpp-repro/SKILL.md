---
name: gocpp-repro
description: Reproduce or check gocpp behavior against real Go in seconds - write a tiny .gp program and run scripts/gp-repro, which builds the worktree's gocpp, runs it and real Go, and diffs stdout and exit code. Use before coding a TODO claim (is it still open?), after a fix, and when reviewing.
---

# Reproduce against real Go

Don't hand-build gocpp or write your own compare script; use the repo's script from your worktree:

```sh
scripts/gp-repro .scratch/r1.gp                 # gocpp vs go: stdout + exit code, prints SAME or DIFFERENT
scripts/gp-repro -stderr .scratch/r1.gp         # also compare stderr (not for panics: addresses differ)
scripts/gp-repro -cxx clang++ .scratch/r1.gp    # the other compiler
scripts/gp-repro -flags "-release" .scratch/r1.gp   # or -asan, -race, -O2
scripts/gp-repro -nogo .scratch/r1.gp           # not valid Go (`!`, macros, C/C++ imports): gocpp only
scripts/gp-repro path/to/moduledir -- arg1      # a folder: `gocpp run .`, Go gets a .go copy
```

- It re-execs itself in the dev shell (ccache, pinned GCC 15 / Clang 21), builds `cmd/gocpp` from
  the worktree as it is now (seconds, go's build cache), and sets `GOCPP_GATE_PHASE=15`.
- Outputs stay in `.scratch/repro/<name>/` (`gocpp.out`, `gocpp.err`, `go.out`, `go.err`).
- Exit code: 0 same, 1 different, 2 usage/build error.
- Put repro programs in `.scratch/` (ignored by git). A repro that should become a regression
  test goes to `tests/run/` (with `// phase:` and `// Output:`) or `tests/diff/` (compared with Go);
  see tests/README.md.
- Big-memory or deep-recursion programs: wrap the call, e.g.
  `systemd-run --user --scope -q -p MemoryMax=8G timeout 5m scripts/gp-repro .scratch/deep.gp`.
