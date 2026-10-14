// Package lsp is the gocpp language server ("gocpp lsp"), built like gopls:
// every edit makes a new immutable snapshot, work is cached by content hash,
// and dependencies come from export-data summaries, never from source. Its
// speed budgets (for example under 100 ms from a keystroke to new
// diagnostics) are checked in CI.
//
// Phase 11 fills it. The early version (v0: diagnostics, hover,
// go-to-definition, completion, formatting) needs only Phase 01. So far
// (steps 11.1 to 11.4) there are:
//   - protocol: the LSP 3.17 types (copied from gopls, see
//     protocol/COPIED.md), JSON-RPC 2.0 over the Content-Length framing
//     (notifications in order on the read loop, requests on their own
//     goroutines, cancellation), and the conversion between positions and
//     byte offsets;
//   - cache: the session and its immutable snapshots of the open
//     documents, and the memoized pipeline (step 11.3): parsed files and
//     checked packages cached by content hash, each package checked alone
//     with its imports from export data, and its importers checked again
//     only when its API changed; the C++ compiler lookup and the
//     namespace probe run in the background (step 11.4), so a check
//     never waits for them; the summaries of the packages (export data,
//     with a side table of the C/C++ types that export data can't hold)
//     are also kept on disk (step 11.3b), so a server that starts again
//     checks from source only the packages it shows;
//   - cache/filecache: that cache on disk: in gocpp/lsp in the user's cache
//     folder (GOCPP_LSP_CACHE sets another absolute folder, or "off"), one
//     folder per gocpp binary, every entry checked when it is read, and a
//     trim in the background, of the cache's own files only, by age
//     (GOCPP_LSP_CACHE_AGE, default 5 days; a day for other binaries) and
//     size (GOCPP_LSP_CACHE_SIZE, default 1 GiB);
//   - memoize: the promises and the LRU store that the caches are made of;
//   - debounce: work that runs after a quiet time per key (diagnostics,
//     step 11.4);
//   - server: the lifecycle (initialize, shutdown, exit), the open
//     documents kept up to date with incremental changes, the requests,
//     each on the snapshot of its time, and the diagnostics as you type
//     (step 11.4: textDocument/publishDiagnostics per file of the open
//     packages, and workspace/didChangeWatchedFiles).
package lsp
