// Package lsp will be the gocpp language server ("gocpp lsp"), built like
// gopls: every edit makes a new immutable snapshot, work is cached by content
// hash, and dependencies come from export-data summaries, never from source.
// Its speed budgets (for example under 100 ms from a keystroke to new
// diagnostics) are checked in CI.
//
// Phase 11 fills it. The early version (v0: diagnostics, hover,
// go-to-definition, completion, formatting) needs only Phase 01 and starts
// right after milestone M1.
package lsp
