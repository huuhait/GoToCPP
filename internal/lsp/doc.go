// Package lsp is the gocpp language server ("gocpp lsp"), built like gopls:
// every edit makes a new immutable snapshot, work is cached by content hash,
// and dependencies come from export-data summaries, never from source. Its
// speed budgets (for example under 100 ms from a keystroke to new
// diagnostics) are checked in CI.
//
// Phase 11 fills it. The early version (v0: diagnostics, hover,
// go-to-definition, completion, formatting) needs only Phase 01. So far
// (step 11.1) there are:
//   - protocol: the LSP 3.17 types (copied from gopls, see
//     protocol/COPIED.md), JSON-RPC 2.0 over the Content-Length framing,
//     and the conversion between positions and byte offsets;
//   - server: the lifecycle (initialize, shutdown, exit) and the open
//     documents, kept up to date with incremental changes.
package lsp
