---
name: gocpp-merge
description: Manager-only checklist to integrate a gocpp job after a clean review - focused re-check, added-file hygiene, squash merge into dev with TODO.md claims closed, push origin dev, remove the merged worktree and branch. Use when a reviewer returns clean.
---

# Integrating a clean job (manager only)

Workers never merge, push or remove worktrees. Worktrees live at `~/worktrees/GoToCPP/<job>`.

1. **Clean review in hand** (no blocker, no should-fix) for the job's latest commit.
2. **Re-run the focused checks yourself** in the job worktree: `scripts/gp-focused -x` (and
   `-cxx clang++` for runtime/emitter changes). Read the passed/skipped counts.
3. **File hygiene**: `git diff --name-status --diff-filter=A dev...job/<job>` and
   `git diff --stat dev...job/<job>`. Drop/send back binaries, `.scratch` content, helper scripts,
   logs, files over ~1 MB, out-of-scope edits.
4. **Squash merge** in the main checkout `/home/huuhait/GoToCPP` on `dev`:
   `git merge --squash job/<job>`; edit `plans/2026-10-13-gocpp/TODO.md` in the same commit
   (tick `[x] (done: <commit>)` or delete fixed claims, add new claims from the review), and the
   phase STATE.md if a step changed. Commit message: `<job>: <what changed>; TODO: <claims closed>`,
   with the session's attribution lines.
5. **Push**: `git push origin dev` (never force). If dev moved, rebase the squash, re-run 2.
6. **Remove**: `git worktree remove ~/worktrees/GoToCPP/<job>` (refuses when dirty: look first,
   never `--force` over unknown work) and `git branch -D job/<job>`. Only merged jobs, only yours.
7. Update TODO.md "Now" / HANDOFF.md and start the next queued job in the free slot.
