# ADR-138: `GOCPP_OFFLINE` has one reader for the toolchain and the modules

Status: accepted (2026-10-22; job 06.lows).

## Context

Two packages read `GOCPP_OFFLINE` with different rules. The Windows toolchain download
(`internal/toolchain`) took every value except empty, `0` and `false` as on. The module download
(`internal/modfetch`) took only `1`, `true`, `yes` and `on`. So `GOCPP_OFFLINE=off` stopped the
toolchain download and not the module download, and `GOCPP_OFFLINE=y` the other way round.

## Decision

- One function, `envflag.Offline` ([internal/envflag](../../internal/envflag/envflag.go)), is the only
  reader. `1`, `true`, `yes` and `on` are on; empty, `0`, `false`, `no` and `off` are off (any case, spaces
  around the word ignored).
- Any other word is on. The variable is a safety switch: a typo must not be the spelling that goes online.
  This keeps the old rule of the toolchain (`2`, `y` stay on) and changes the modules only in the safe
  direction (`2`, `y` were off there).
- `no` and `off` are new off spellings for the toolchain (they were on). Documented in
  [docs/spec/modules.md](../spec/modules.md), the guides and `gocpp env`.
- The dev tools that read the variable themselves (`tools/gotest`, `tools/portstd`) use the same function.
- The other `any value but empty, 0 and false` variables of `gocpp env` keep their rule.

## Rejected

An error for an unknown word: it would break a script that sets `GOCPP_OFFLINE=y` today.
