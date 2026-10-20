# Language server: the round of step 11.13 (`!`, macros and `#Derive`)

What step 11.13 cost the rows that existed, and what the new rows cost. Written by hand from runs of
`tools/lspbench` and of the server's own benchmarks; the generated tables are [lsp.md](lsp.md) (regenerate with
`make lsp-bench` on a quiet machine). The machine (192 logical cores, 503 GiB) was **not quiet**: the load average
was between 60 and 190 during the first 12 runs below and went up to 1,700 during the second set, because other
agents were running test sweeps. Read ratios between 0.8 and 1.25 as equal.

## A/B: the binary before the step against the binary after it

Two builds of `gocpp`: `base` is the parent commit (`5eb35c68`), `new` is the branch. Alternating runs
(`base`, `new`, then `new`, `base`, ...), six rounds each of two sets, `lspbench -runs 3` on the `medium`, `large`
and `macro` projects (so the median of 18 sessions per row and set). The table is the median over the rounds of the
median p50 and the p95 of a session, set 1 (load 60 to 190). The rows that this step can touch are
hover, semantic-tokens, completion-dot and completion-edit, keystroke, references and rename (they go through
`bang()`, `phase10Tokens`, `macroCompletion`, the hints of a call and `shownName`; for a package with none of the forms
each is a nil test or an empty loop). Every row is within the noise; the largest are the 0.04 to 0.06 ms rows, where one
scheduler tick is 20%.

| project | scenario | base p50 | new p50 | p50 new/base | base p95 | new p95 | p95 new/base | rounds |
|---|---|--:|--:|--:|--:|--:|--:|--:|
| large | api-change | 0.981 | 0.911 | 0.93 | 1.59 | 1.34 | 0.84 | 6 |
| large | api-fix | 1.02 | 0.689 | 0.68 | 1.45 | 1.16 | 0.80 | 6 |
| large | completion-dot | 0.0796 | 0.0885 | 1.11 | 0.182 | 0.176 | 0.97 | 6 |
| large | completion-edit | 1.41 | 1.33 | 0.94 | 1.76 | 1.62 | 0.92 | 6 |
| large | definition | 0.0368 | 0.039 | 1.06 | 0.0429 | 0.0494 | 1.15 | 6 |
| large | document-symbol | 0.0384 | 0.046 | 1.20 | 0.0479 | 0.0575 | 1.20 | 6 |
| large | format-1k | 3.52 | 3.76 | 1.07 | 7.44 | 5.45 | 0.73 | 6 |
| large | hover | 0.0494 | 0.0586 | 1.19 | 0.131 | 0.163 | 1.24 | 6 |
| large | initialize | 9.43 | 9.71 | 1.03 | 10.8 | 10.8 | 1.00 | 6 |
| large | keystroke | 0.587 | 0.63 | 1.07 | 1.5 | 3.04 | 2.04 | 6 |
| large | keystroke-imported | 0.882 | 0.423 | 0.48 | 1.74 | 0.681 | 0.39 | 6 |
| large | open-cold | 381 | 388 | 1.02 | 381 | 388 | 1.02 | 6 |
| large | open-warm | 126 | 136 | 1.08 | 126 | 136 | 1.08 | 6 |
| large | references-family | 0.644 | 0.739 | 1.15 | 0.958 | 1.01 | 1.05 | 6 |
| large | references-local | 0.468 | 0.526 | 1.12 | 0.621 | 0.696 | 1.12 | 6 |
| large | references-popular | 6.27 | 6.5 | 1.04 | 7.12 | 7.23 | 1.01 | 6 |
| large | rename | 4.04 | 4.14 | 1.02 | 4.82 | 4.88 | 1.01 | 6 |
| large | semantic-tokens | 0.0377 | 0.0449 | 1.19 | 0.0411 | 0.0575 | 1.40 | 6 |
| large | signature-help | 0.0414 | 0.0436 | 1.05 | 0.0539 | 0.0596 | 1.10 | 6 |
| large | switch-file | 1.2 | 1.14 | 0.94 | 1.74 | 1.45 | 0.83 | 6 |
| large | workspace-symbol | 0.651 | 0.746 | 1.15 | 0.854 | 1.14 | 1.34 | 6 |
| macro | api-change | 0.494 | 0.519 | 1.05 | 0.803 | 0.819 | 1.02 | 6 |
| macro | api-fix | 0.322 | 0.459 | 1.42 | 0.558 | 0.836 | 1.50 | 6 |
| macro | code-action | - | 0.123 | - | - | 0.262 | - | 6 |
| macro | completion-dot | 0.0987 | 0.112 | 1.13 | 0.403 | 0.441 | 1.09 | 6 |
| macro | completion-edit | 0.809 | 0.821 | 1.02 | 1.14 | 1.09 | 0.96 | 6 |
| macro | completion-macro | 1.82 | 0.698 | 0.38 | 2.42 | 0.902 | 0.37 | 6 |
| macro | definition | 0.0424 | 0.0429 | 1.01 | 0.0914 | 0.0511 | 0.56 | 6 |
| macro | document-symbol | 0.0559 | 0.06 | 1.07 | 0.0812 | 0.0991 | 1.22 | 6 |
| macro | format-1k | 3.75 | 3.56 | 0.95 | 6.85 | 6.04 | 0.88 | 6 |
| macro | hover | 0.0638 | 0.0667 | 1.05 | 0.185 | 0.204 | 1.10 | 6 |
| macro | hover-bang | 0.0689 | 0.0842 | 1.22 | 0.131 | 0.14 | 1.07 | 6 |
| macro | hover-macro | 0.0417 | 0.058 | 1.39 | 0.0745 | 0.0945 | 1.27 | 6 |
| macro | initialize | 10.1 | 10.5 | 1.04 | 10.6 | 11.7 | 1.10 | 6 |
| macro | inlay-hints-bang | 0.0849 | 0.113 | 1.33 | 0.112 | 0.15 | 1.34 | 6 |
| macro | keystroke | 0.232 | 0.254 | 1.10 | 0.98 | 1.11 | 1.14 | 6 |
| macro | keystroke-imported | 0.338 | 0.337 | 1.00 | 2.83 | 3.07 | 1.08 | 6 |
| macro | open-cold | 107 | 115 | 1.08 | 107 | 115 | 1.08 | 6 |
| macro | open-warm | 34.9 | 35.8 | 1.03 | 34.9 | 35.8 | 1.03 | 6 |
| macro | references-family | 0.166 | 0.164 | 0.99 | 0.219 | 0.227 | 1.04 | 6 |
| macro | references-local | 0.133 | 0.142 | 1.07 | 0.214 | 0.224 | 1.05 | 6 |
| macro | references-popular | 0.336 | 0.316 | 0.94 | 0.473 | 0.476 | 1.01 | 6 |
| macro | rename | 0.549 | 0.57 | 1.04 | 0.811 | 0.757 | 0.93 | 6 |
| macro | semantic-tokens | 0.0557 | 0.0595 | 1.07 | 0.0687 | 0.0807 | 1.18 | 6 |
| macro | semantic-tokens-macro | 0.044 | 0.0441 | 1.00 | 0.052 | 0.0493 | 0.95 | 6 |
| macro | signature-help | 0.0454 | 0.0471 | 1.04 | 0.0823 | 0.0616 | 0.75 | 6 |
| macro | switch-file | 0.698 | 0.665 | 0.95 | 0.893 | 0.891 | 1.00 | 6 |
| macro | workspace-symbol | 0.154 | 0.149 | 0.97 | 0.3 | 0.215 | 0.72 | 6 |
| medium | api-change | 0.84 | 0.636 | 0.76 | 1.11 | 1.21 | 1.09 | 6 |
| medium | api-fix | 0.74 | 0.541 | 0.73 | 1.39 | 0.816 | 0.59 | 6 |
| medium | completion-dot | 0.108 | 0.101 | 0.93 | 0.274 | 0.292 | 1.07 | 6 |
| medium | completion-edit | 0.992 | 0.87 | 0.88 | 1.53 | 1.2 | 0.79 | 6 |
| medium | definition | 0.0424 | 0.0447 | 1.05 | 0.0541 | 0.0534 | 0.99 | 6 |
| medium | document-symbol | 0.0587 | 0.0655 | 1.12 | 0.0903 | 0.121 | 1.34 | 6 |
| medium | format-1k | 3.46 | 3.52 | 1.02 | 4.92 | 4.16 | 0.85 | 6 |
| medium | hover | 0.0593 | 0.0546 | 0.92 | 0.149 | 0.143 | 0.96 | 6 |
| medium | initialize | 9.6 | 10.3 | 1.07 | 10.5 | 10.8 | 1.03 | 6 |
| medium | keystroke | 0.399 | 0.399 | 1.00 | 2.52 | 3.45 | 1.37 | 6 |
| medium | keystroke-imported | 0.402 | 0.353 | 0.88 | 2.82 | 1.54 | 0.55 | 6 |
| medium | open-cold | 190 | 198 | 1.05 | 190 | 198 | 1.05 | 6 |
| medium | open-warm | 67.4 | 67.9 | 1.01 | 67.4 | 67.9 | 1.01 | 6 |
| medium | references-family | 0.305 | 0.288 | 0.94 | 0.409 | 0.415 | 1.02 | 6 |
| medium | references-local | 0.208 | 0.216 | 1.04 | 0.31 | 0.283 | 0.91 | 6 |
| medium | references-popular | 1.1 | 1.15 | 1.05 | 1.47 | 1.5 | 1.02 | 6 |
| medium | rename | 1.27 | 1.26 | 1.00 | 1.74 | 1.6 | 0.92 | 6 |
| medium | semantic-tokens | 0.0594 | 0.0631 | 1.06 | 0.0738 | 0.0692 | 0.94 | 6 |
| medium | signature-help | 0.0459 | 0.0488 | 1.06 | 0.0809 | 0.0582 | 0.72 | 6 |
| medium | switch-file | 0.739 | 0.742 | 1.00 | 0.99 | 0.969 | 0.98 | 6 |
| medium | workspace-symbol | 0.221 | 0.25 | 1.13 | 0.296 | 0.315 | 1.07 | 6 |

(All times in milliseconds. A second set of six rounds, at load 600 to 1,700, gave the same picture with more noise:
the `macro` project's rows that this step does not touch moved by 0.9 to 1.6 in both directions.)

## The server's own benchmarks, pinned

The load makes the sessions noisy, so the same two builds were also run as test binaries, alternating, pinned to four
cores (`taskset -c 8-11`, `go test -c`, 3,000 iterations, six rounds each, median of the rounds), on the generated project of
`internal/lsp/server/bench_test.go`:

| benchmark | base ns/op | new ns/op | new / base |
|---|--:|--:|--:|
| Hover | 62,988 | 63,636 | 1.01 |
| InlayHints | 147,162 | 145,656 | 0.99 |
| SemanticTokens | 78,132 | 76,979 | 0.99 |
| SignatureHelp | 69,534 | 69,976 | 1.01 |

The unpinned runs of the same binaries (8 rounds, 200 iterations, `Completion`, `CompletionAfterEdit`, `Definition`, `Keystroke`
too) have ratios between 0.86 and 1.40 with distributions that overlap completely: the machine's load, not the code (the
keystroke bench includes the server's debounce and a check, and moved from 0.4 to 1.4 between rounds of the same binary).

## The new rows: the `macro` project

`macro` is a generated project (`internal/lsp/bench/gen.go`, seed 5): 71 files and 8,239 lines, gocpp only (gopls cannot
read it, so the Go copy has none of the macro packages). Its packages `mac01` to `mac07` have six files each (`mac00` declares the macros), and every file
has a `#Derive`, a must function whose dropped errors are handled, `!` calls (statement, assignment, `defer`), macros inside
code, macros on top of a func (start and wrap style) and a lazy argument. The scenarios run on `mac03/f02`. Median over the 12
sessions of the two sets (milliseconds):

| scenario | first | p50 | p95 | max | budget (p95) |
|---|--:|--:|--:|--:|--:|
| hover-macro (a macro use in code, on top of a func, `#Derive`) | 0.46 | 0.059 | 0.099 | 0.294 | 30 |
| hover-bang (`F!`, a handled call, a must function) | 0.44 | 0.077 | 0.123 | 0.150 | 30 |
| inlay-hints-bang (the whole file) | 0.25 | 0.122 | 0.163 | 0.257 | - |
| semantic-tokens-macro (the whole file; made once per package, then memoized) | 0.22 | 0.047 | 0.050 | 0.054 | - |
| completion-macro (`#` typed at the start of a line, asked at once) | 4.15 | 0.717 | 1.008 | 1.213 | 50 |
| code-action (at a `!` and at a macro use) | 0.72 | 0.132 | 0.246 | 0.326 | 30 |

Hover on a macro use and on `F!` is sub-millisecond at p50, p95 and max, as the step asked. The "first" figure is
the first request of its kind in the session (the index of the package is built then: one pass over the calls of the check).
Not measured here: "Show expansion" itself (it loads the package and its imports from source, like `gocpp expand`, a
second or two for a package that imports a few standard packages; it is a command, not a request on the keystroke path).

## Windows

`tests/windows_lsp_test.go` (`TestWindowsLSP`, step 14.5) and the new `TestWindowsLSPBang` (hover and hints on a CRLF file with
an emoji, the command of "Show expansion", the folder of the expansion on the PC while the server runs and its removal at
shutdown) were run on the real PC (`GOCPP_WINDOWS_SSH`) at the end of the step: both pass.

## Review round 1 (syntax-error repair, moved names, expansion files)

The fixes add work only for a file that has a syntax error (`newViewQuery`: a parse already in the cache, then one repaired check on request),
the callee record, and a wider search by place. Measured with the binary of the parent of the step (`base`) against the branch (`new`),
alternating, pinned to 16 cores (`taskset -c 8-23`), machine load 10 to 200.

Set 1: `lspbench -runs 3` on medium, large and macro, six rounds (medians over the rounds; ms). All the 70 rows are within 0.8 to 1.25 but
a few at 0.03 to 0.07 ms (hover-macro 1.47, inlay-hints-bang 1.35: 16 to 30 microseconds more, the cost of the hint records and of the
hover text for macros, which the base binary does not have; `medium` api-change/api-fix/switch-file 1.3 to 1.5 in this set only):

| project | scenario | base p50 | new p50 | p50 new/base | base p95 | new p95 | p95 new/base | rounds |
|---|---|--:|--:|--:|--:|--:|--:|--:|
| large | api-change | 0.429 | 0.485 | 1.13 | 0.782 | 0.86 | 1.10 | 6 |
| large | api-fix | 0.4 | 0.451 | 1.13 | 0.685 | 0.765 | 1.12 | 6 |
| large | completion-dot | 0.0724 | 0.0797 | 1.10 | 0.144 | 0.118 | 0.82 | 6 |
| large | completion-edit | 0.82 | 0.822 | 1.00 | 1.5 | 1.43 | 0.95 | 6 |
| large | definition | 0.034 | 0.0316 | 0.93 | 0.0408 | 0.0373 | 0.91 | 6 |
| large | document-symbol | 0.0348 | 0.0349 | 1.00 | 0.0454 | 0.0423 | 0.93 | 6 |
| large | format-1k | 3.49 | 3.49 | 1.00 | 4.17 | 4.04 | 0.97 | 6 |
| large | hover | 0.0461 | 0.0448 | 0.97 | 0.127 | 0.12 | 0.95 | 6 |
| large | initialize | 6.73 | 7.08 | 1.05 | 7.17 | 7.73 | 1.08 | 6 |
| large | keystroke | 0.393 | 0.352 | 0.90 | 1.17 | 1.23 | 1.05 | 6 |
| large | keystroke-imported | 0.318 | 0.345 | 1.09 | 0.45 | 0.569 | 1.26 | 6 |
| large | open-cold | 215 | 213 | 0.99 | 215 | 213 | 0.99 | 6 |
| large | open-warm | 63.8 | 65.4 | 1.03 | 63.8 | 65.4 | 1.03 | 6 |
| large | references-family | 0.476 | 0.438 | 0.92 | 0.652 | 0.589 | 0.90 | 6 |
| large | references-local | 0.355 | 0.341 | 0.96 | 0.443 | 0.436 | 0.98 | 6 |
| large | references-popular | 4.57 | 4.72 | 1.03 | 5.64 | 5.5 | 0.97 | 6 |
| large | rename | 2.86 | 2.87 | 1.00 | 4.13 | 3.92 | 0.95 | 6 |
| large | semantic-tokens | 0.0337 | 0.0346 | 1.03 | 0.0443 | 0.045 | 1.01 | 6 |
| large | signature-help | 0.0388 | 0.037 | 0.95 | 0.0491 | 0.0426 | 0.87 | 6 |
| large | switch-file | 0.601 | 0.769 | 1.28 | 1.08 | 1.17 | 1.08 | 6 |
| large | workspace-symbol | 0.704 | 0.653 | 0.93 | 0.887 | 0.805 | 0.91 | 6 |
| macro | api-change | 0.235 | 0.292 | 1.24 | 0.38 | 0.441 | 1.16 | 6 |
| macro | api-fix | 0.215 | 0.25 | 1.16 | 0.339 | 0.317 | 0.93 | 6 |
| macro | code-action | - | 0.109 | - | - | 0.137 | - | 6 |
| macro | completion-dot | 0.09 | 0.098 | 1.09 | 0.173 | 0.169 | 0.98 | 6 |
| macro | completion-edit | 0.569 | 0.587 | 1.03 | 0.979 | 0.946 | 0.97 | 6 |
| macro | completion-macro | 1.56 | 0.438 | 0.28 | 2.11 | 0.787 | 0.37 | 6 |
| macro | definition | 0.0362 | 0.0401 | 1.11 | 0.0483 | 0.0508 | 1.05 | 6 |
| macro | document-symbol | 0.0529 | 0.0572 | 1.08 | 0.0951 | 0.0756 | 0.80 | 6 |
| macro | format-1k | 3.59 | 3.49 | 0.97 | 4.77 | 5.11 | 1.07 | 6 |
| macro | hover | 0.05 | 0.0513 | 1.03 | 0.137 | 0.143 | 1.05 | 6 |
| macro | hover-bang | 0.052 | 0.0671 | 1.29 | 0.0926 | 0.109 | 1.17 | 6 |
| macro | hover-macro | 0.0341 | 0.0502 | 1.47 | 0.0572 | 0.0784 | 1.37 | 6 |
| macro | initialize | 7 | 6.77 | 0.97 | 7.25 | 7.14 | 0.98 | 6 |
| macro | inlay-hints-bang | 0.0766 | 0.104 | 1.35 | 0.0996 | 0.135 | 1.36 | 6 |
| macro | keystroke | 0.164 | 0.167 | 1.02 | 0.474 | 0.478 | 1.01 | 6 |
| macro | keystroke-imported | 0.226 | 0.241 | 1.07 | 1.56 | 1.79 | 1.14 | 6 |
| macro | open-cold | 76.2 | 75.7 | 0.99 | 76.2 | 75.7 | 0.99 | 6 |
| macro | open-warm | 27 | 27.6 | 1.02 | 27 | 27.6 | 1.02 | 6 |
| macro | references-family | 0.116 | 0.123 | 1.06 | 0.144 | 0.158 | 1.10 | 6 |
| macro | references-local | 0.11 | 0.107 | 0.97 | 0.151 | 0.139 | 0.92 | 6 |
| macro | references-popular | 0.199 | 0.194 | 0.98 | 0.324 | 0.332 | 1.02 | 6 |
| macro | rename | 0.401 | 0.399 | 0.99 | 0.571 | 0.529 | 0.93 | 6 |
| macro | semantic-tokens | 0.0511 | 0.0579 | 1.13 | 0.058 | 0.0817 | 1.41 | 6 |
| macro | semantic-tokens-macro | 0.0371 | 0.0422 | 1.14 | 0.0424 | 0.048 | 1.13 | 6 |
| macro | signature-help | 0.0407 | 0.0447 | 1.10 | 0.0538 | 0.0525 | 0.98 | 6 |
| macro | switch-file | 0.453 | 0.444 | 0.98 | 0.746 | 0.635 | 0.85 | 6 |
| macro | workspace-symbol | 0.132 | 0.141 | 1.07 | 0.188 | 0.193 | 1.03 | 6 |
| medium | api-change | 0.332 | 0.495 | 1.49 | 0.651 | 0.669 | 1.03 | 6 |
| medium | api-fix | 0.251 | 0.382 | 1.53 | 0.384 | 0.667 | 1.74 | 6 |
| medium | completion-dot | 0.0917 | 0.0934 | 1.02 | 0.162 | 0.166 | 1.03 | 6 |
| medium | completion-edit | 0.601 | 0.712 | 1.18 | 1.07 | 1.41 | 1.31 | 6 |
| medium | definition | 0.0375 | 0.0393 | 1.05 | 0.0454 | 0.047 | 1.04 | 6 |
| medium | document-symbol | 0.0537 | 0.0537 | 1.00 | 0.0714 | 0.0705 | 0.99 | 6 |
| medium | format-1k | 3.55 | 3.45 | 0.97 | 4.34 | 4 | 0.92 | 6 |
| medium | hover | 0.0462 | 0.0493 | 1.07 | 0.125 | 0.134 | 1.07 | 6 |
| medium | initialize | 6.78 | 6.77 | 1.00 | 7.29 | 7.17 | 0.98 | 6 |
| medium | keystroke | 0.256 | 0.251 | 0.98 | 1.01 | 1.13 | 1.12 | 6 |
| medium | keystroke-imported | 0.238 | 0.239 | 1.00 | 0.956 | 0.881 | 0.92 | 6 |
| medium | open-cold | 135 | 128 | 0.95 | 135 | 128 | 0.95 | 6 |
| medium | open-warm | 36.1 | 35.8 | 0.99 | 36.1 | 35.8 | 0.99 | 6 |
| medium | references-family | 0.226 | 0.207 | 0.91 | 0.312 | 0.281 | 0.90 | 6 |
| medium | references-local | 0.16 | 0.156 | 0.97 | 0.209 | 0.206 | 0.98 | 6 |
| medium | references-popular | 0.699 | 0.715 | 1.02 | 0.965 | 0.964 | 1.00 | 6 |
| medium | rename | 0.833 | 0.851 | 1.02 | 1.32 | 1.45 | 1.10 | 6 |
| medium | semantic-tokens | 0.0544 | 0.0541 | 1.00 | 0.0605 | 0.068 | 1.12 | 6 |
| medium | signature-help | 0.0415 | 0.0416 | 1.00 | 0.0519 | 0.0476 | 0.92 | 6 |
| medium | switch-file | 0.483 | 0.629 | 1.30 | 0.717 | 0.968 | 1.35 | 6 |
| medium | workspace-symbol | 0.214 | 0.213 | 0.99 | 0.25 | 0.256 | 1.03 | 6 |

Set 2: the rows that moved in set 1, again on `medium` alone, twelve rounds of `-runs 5`: they are equal (0.92 to 1.09 at p50):

| project | scenario | base p50 | new p50 | p50 new/base | base p95 | new p95 | p95 new/base | rounds |
|---|---|--:|--:|--:|--:|--:|--:|--:|
| medium | api-change | 0.307 | 0.284 | 0.92 | 0.432 | 0.447 | 1.04 | 12 |
| medium | api-fix | 0.253 | 0.276 | 1.09 | 0.413 | 0.358 | 0.87 | 12 |
| medium | completion-edit | 0.581 | 0.581 | 1.00 | 0.711 | 0.698 | 0.98 | 12 |
| medium | hover | 0.0509 | 0.0508 | 1.00 | 0.145 | 0.135 | 0.93 | 12 |
| medium | initialize | 6.94 | 7.09 | 1.02 | 7.62 | 7.46 | 0.98 | 12 |
| medium | keystroke | 0.257 | 0.254 | 0.99 | 0.904 | 1.05 | 1.17 | 12 |
| medium | open-cold | 145 | 136 | 0.94 | 145 | 136 | 0.94 | 12 |
| medium | open-warm | 40.2 | 39.6 | 0.98 | 40.2 | 39.6 | 0.98 | 12 |
| medium | switch-file | 0.452 | 0.457 | 1.01 | 0.544 | 0.549 | 1.01 | 12 |

In-process benchmarks, eight alternating rounds, pinned, 3,000 iterations (median ns/op, base / new): Hover 63,435 / 64,126, InlayHints 145,910 /
144,784, SemanticTokens 76,822 / 77,368, SignatureHelp 69,205 / 68,922.

A file with a syntax error (the macro project's file with a `)` missing, `TestBangSyntaxErrors`' state): a change with the hints and the tokens asked
takes about 20 ms (the repaired check), against 11 ms for a clean edit; a hover there takes 0.2 to 0.4 ms. Both are request costs of a file that is
being typed; the keystroke row (diagnostics) does not pay for it.

## Guard-recovery validation (2026-10-20)

**The earlier permissive-stale typing measurements are not evidence for the stricter guard. This step is not budget-complete.**
A repaired view must belong to the request's snapshot, or be reachable through a contiguous run of successful changes to this
file alone. Equal file text, signatures or line counts cannot prove that sibling/import inputs stayed unchanged. Reuse across
snapshots additionally requires an edit confined to the same lexically verified, blanked function body, preserving everything
outside it and its line count. Old partial hints/tokens inside that body are omitted; a hover inside it waits for a current check.
Signature edits, newlines, ambiguous braces/comments/backticks, disk invalidation and unknown snapshot events rebuild instead.

The guard regression (`TestStaleSemanticInputs`) first reproduced old `int` hovers after a provider became `string`, function
tokens after a function became a variable, and old definition columns, even though the target text had not changed. It now
compares hover, definition, hints and tokens with a fresh server after sibling/import API changes and a same-file edit/revert.
`TestStaleLexicalBoundaries` compares fresh answers after braces, comments, backticks, newlines and Unicode edits;
`TestStaleBodyNeverReturnsOldPartial` checks that a changed body's old classifications do not leak. The raw-token helper rejects
error/null replies, truncated tuples, overflowing deltas, overlaps, out-of-range positions and half-surrogate UTF-16 boundaries.
The lazy-partial/coalescing/synthetic-marker regression is in `internal/check/partial_test.go`; that recovery regression did not
change checker production code.

### Unique-edit, balanced typing runs

Two completed rounds, in `dev → branch`, then `branch → dev` order, with the **same harness**, toolchain, CPUs 8–15,
`GOMAXPROCS=8`, fresh work/cache directories per target/round, and the shared `perf.lock`. One session per project per round;
`-runs 1 -scale 1 -projects macro,bigpkg -only typing-clean,typing-broken,typing-broken-reqs`. Every scenario types **200 unique
prefixes**, with two warm-ups and 198 measured samples, 30 ms between edits. The clean scenario types a comment. The broken
scenario types an unfinished nested macro call inside `SumX`; bare uses `x`, requests uses `y`, so the requests scenario cannot
reuse the bare scenario's checked texts. The requests scenario asks rotating `!` hovers, whole-file hints and semantic tokens
after each edit; the validator checks nonempty replies, current-document positions, token deltas and retention of the `Open`
token before the damaged function. `macro` has 71 files / 8,239 lines spread over small packages; `bigpkg` has 53 files / 5,282
lines and puts 46 macro-heavy files into one checked package. The one-minute load ranged from 1.51 to 7.16.

Saved binary identities (SHA-256): dev `c06c24b6fcb484a06cc3f61b566ac3207d9518d6434ce23dd376a1d6de46e71c`,
branch `8c5ad86cc97ac5ff4c4ccfd10a109e279bf920946084f09ff597c81ba261b01a`,
harness `639ffb3eac7f106229be2401d2813836444bea3086c179a39adc842b8846b607`. These are the recovered
`gocpp-dev`, `gocpp-branch` and `lspbench` binaries from the semantic-input-guard fix, not the older `base` of the tables above.
The helper/doc edits at `42e49dc9` did not change those server or scenario implementations. Those binaries predate the
independent-review correctness corrections below; no timing rerun or performance-completion claim is made for the corrected server.

Times below are the range of the two per-session quantiles, in milliseconds, not pooled quantiles:

| Project | Scenario | dev p50 | branch p50 | dev p95 | branch p95 | Validity / budget |
|---|---|--:|--:|--:|--:|---|
| macro | typing-clean | 9.29–9.38 | 9.19–9.36 | 12.65–13.89 | 13.68–13.70 | all 198 samples valid per round |
| macro | typing-broken | 3.73–3.76 | 2.09–2.25 | 4.09–4.17 | 2.67–2.72 | all 198 valid; <100 ms |
| macro | typing-broken-reqs | invalid | 2.20–2.23 | invalid | 2.62–3.15 | branch 198/198 valid; <100 ms |
| macro | typing-broken-hover | invalid | 0.64–0.65 | invalid | 13.30–13.42 | branch 198/198 valid; <30 ms |
| bigpkg | typing-clean | 95.50–96.79 | 96.27–96.39 | 120.89–123.11 | 120.04–120.47 | **100 ms typing target unmet on both** |
| bigpkg | typing-broken | 25.52–25.83 | 6.74–6.77 | 30.03–34.80 | 9.07–9.80 | all 198 valid; <100 ms |
| bigpkg | typing-broken-reqs | invalid | 7.50–7.56 | invalid | 16.80–16.86 | branch 198/198 valid; <100 ms |
| bigpkg | typing-broken-hover | invalid | 1.13–1.44 | invalid | 147.42–148.02 | branch 198/198 valid; **30 ms hover budget unmet** |

The dev requests scenario fails **98 of 198 measured samples** on each project/round (`unexpected end of JSON input`, an
empty/missing feature payload); it has only 100 accepted request/hover samples. Its successful-subset latencies must not be
compared with the branch's complete series, and the extra hover series' zero failure count does not make the missing samples
valid. No validators or budgets were relaxed. The guarded branch has no failed typing samples. Its macro hover maximum is
23.14–50.74 ms and bigpkg hover maximum 173.53–177.94 ms: a low median is not evidence that all hovers are instant.

A `wait4` wrapper recorded total server CPU for the combined warm session (clean, bare and requests): macro branch 7.71–8.22 s
versus dev 6.61–6.78 s; bigpkg branch 50.86–50.88 s versus dev 40.77–41.03 s. Those CPU figures are not feature-equivalent
because dev loses replies, and cannot prove a CPU speedup. The wrapper is the process sampled by the harness's RSS collector,
so these runs' RSS figures are **not server memory measurements** and are not used. Lower broken-file diagnostic latency does
not settle the remaining big-package clean-check and guarded-hover costs. Independent correctness review and subsequent valid
performance measurements are still required before integration; no broad CI, corpus or sanitizer sweep was run for this recovery.

Final focused checks used `nix develop`, the shared test-slot wrapper, isolated temporary/cache folders,
`GOCPP_MODCACHE=off GOCPP_OFFLINE=1 GOCPP_TEST_TIMEOUT=6m`, and bounded timeouts. The race-enabled selection
`Test(TokenDataValidation|TypingAnswerValidation|RepairedAnswersAreValid|StaleBodyNeverReturnsOldPartial|StaleSemanticInputs|StaleLexicalBoundaries|BangSyntaxErrors)`
passed in `internal/lsp/server` (17.519 s) and `internal/lsp/bench` (1.011 s). The earlier focused
`TestLazyPartialMustSynthetic` race run also passed. `TestWindowsLSPBang` and `TestWindowsLSP` passed again over real Windows SSH
(24.866 s for the test package), including CRLF/UTF-16 positions, expansion text and shutdown cleanup. Wine was not used.
The Windows process/ownership fallback remains age-based: a session suspended for more than a day can lose its old expansion
files to another server's cleanup; see ADR-099. That limitation was documented, not expanded into a new ACL/process feature.

## Independent-review correctness follow-up

Review of `42e49dc9` found six confirmed feature/correctness gaps; the follow-up addresses each without relaxing reuse guards:

- `#Derive` sits before its type, so mark-containment alone never offered its expansion action. The action now uses the loader's
  `Derive.Decl`/`Spec` association. Metadata-only printer spans select the associated generated methods from directive or type cursors,
  including comments, stacked directives, CRLF and a supported single-spec group. Rejected multi-spec derives and nearby unmarked
  types do not gain an unrelated action/selection. On-top wrap directives also select their function's expansion. The existing expansion
  golden and insertion-span tests keep emitted text unchanged; the new span test compares the same edits with/without metadata.
- The insertion-only `!` quick action is suppressed for parenthesized callees (`(f)()`, `(pkg.F)()`, repeated parentheses and parenthesized
  generic bases): inserting after the identifier would create invalid `(f!)()`. `(v).Method()` stays supported, because the receiver's
  parentheses do not enclose the method name. Every offered edit in the regression is applied, parsed and type-checked.
- An ordinary library function named `main` no longer gets an exit explanation merely because unrelated errors prevented lowering.
  The partial policy checks the package name; actual main-package entry points and recorded exit-policy must functions keep their rule.
- Ordinary `gp_value` parameter hints are retained. Hidden macro arguments are identified by `MacroUse.Sig` slots, not by a `gp_` prefix.
- Imported user-written `must_r`/`must_err` parameter/result names survive inexact export-data positions. Synthetic identity or exact
  source evidence can hide generated names; an unknown origin conservatively retains the name rather than guessing from its spelling.
- A request holding an old repaired-view reference now takes an already-published current view. It cannot resume after the builder's
  refresh decision and strand incomplete stale body tokens with no pending refresh. A deterministic interleaving regression checks the
  current body tokens and absence of a stranded stale flag.

Focused server regressions and adjacent expansion/quick-action/macro-hint/stale-view tests passed (0.990 s), printer span tests passed
(0.003 s), and the existing `TestMustInfo*` checker selection passed (2.243 s). Seven directly affected server tests were repeated with
`-race -count=5` and passed (5.896 s). Commands used the test-slot wrapper, `nix develop`, isolated cache/temp paths, the offline/module-cache
settings above, bounded timeouts and one heavy job at a time. No broad tests, new performance runs or new Windows runs were requested
for this narrow follow-up. The earlier Windows evidence and unmet performance budgets remain as recorded, not upgraded to final-head
verification or budget completion. Independent whole-diff re-review is still required.

### Re-review of `b75ab54e`: two additional correctness fixes

The six findings above were independently verified fixed. The next whole-diff review found two further cases:

1. A partial `!` policy lost `testing.TB` and a closure's inherited testing context after an unrelated type error. Partial
   information now receives the partial `types.Info` and uses the lowerer's `newCtx`/`isTesting` context construction, including
   return-policy priority, named/nonblank testing parameters, exact package identity and nested closure inheritance. Conversion
   of those contexts to `MustKind` is shared with lowered calls. It performs no extra type-check rounds or lowering. The focused
   regression compares clean and broken hovers across TB, T/closure/nested-closure, renamed-import, own-closure-parameter,
   return-priority, unnamed/blank-parameter and impostor-package controls.
2. A column-one `func` inside a raw string was mistaken for a declaration boundary, causing repair to erase an intact function
   signature and lose healthy-caller hovers. Recovery boundaries now come from the Phase-10 scanner's column-one declaration
   tokens, not raw line prefixes. Strings/comments cannot create boundaries; an unterminated raw string/block comment consumes
   its uncertain tail instead of inventing declarations. Controls cover all declaration keywords and `#` in raw strings/comments,
   CRLF, unterminated tails, and healthy-caller/signature preservation for keyword-looking text versus a `faux` control.

The initial focused selection passed in the server (1.626 s) and checker (2.041 s), including existing `TestMustInfo*` and
lazy-partial synthetic-marker tests. A second named server selection covering these fixes and the earlier six findings passed
(4.648 s). The new testing-policy/lexical tests, main-policy controls, stale interleaving/lexical guards and lazy-partial regression
were repeated with `-race -count=5`: server 42.729 s, checker 2.062 s, all passed. Runs used the shared test-slot wrapper,
`nix develop`, isolated cache/temp directories, offline/module-cache settings, bounded timeouts and one heavy job at a time.

These are correctness fixes, not a new timing result. No broad tests, performance rerun or new Windows run was performed.
The earlier final-head and performance limitations remain unchanged, and independent whole-diff re-review is still required.

## Guarded-hover performance follow-up

The new comparison baseline is the independently reviewed correctness revision `2192a725`, **not** the older dev binary
that lost feature replies. The candidate changes only the server's hover/repair request path: when the current lexical repair
blanks the requested place, a hover goes straight to that snapshot's original partial check. The previous path first waited
for a repaired whole-package check, then discarded it at that place and selected the same original partial query. The current
partial type-check still completes synchronously; no old-body answer, empty reply or new delay/debounce policy is substituted.
Definitions, whole-file hints/tokens, repaired-view reuse guards and refresh decisions keep their existing paths. Core
load/cache/check code owned by the separate performance branch is untouched.

Focused regressions hold the repaired builder blocked while checking current body hovers after same-file/sibling/import API
changes and reverts, including UTF-16 column shifts. Another compares nonempty `!` and macro hover text/ranges against the
original query selected by the full repaired-view path. Existing stale-input, lexical-boundary, refresh, interleaving,
syntax-error, raw-declaration and testing-policy regressions also pass under race detection (32.028 s for the named selection).

### Profiling evidence (not the paired timing comparison)

A single instrumented `bigpkg` session per binary ran only the existing `typing-clean` and `typing-broken-reqs` workloads,
plus the latter's hover series. Both have 198/198 valid measured samples per series, no failed replies, and two additional
validated warm-ups. With CPU profiling, request tracing and the same slow-log threshold, hover p95 was 147.675 ms at the
baseline and 28.768 ms at the candidate; clean typing remained 121.874 / 122.514 ms. These pilot figures are diagnostic,
not the final performance claim.

The baseline's longest traced hover took 181.043 ms with only 0.134 ms of parsing on its request thread, consistent with its
wait for the repaired builder. The held-builder regression proves that the new current-partial route does not need that wait.
The combined clean/requests profile collected 47.66 CPU-samples seconds over 40.77 s at baseline and 39.23 over 35.41 s at
candidate. The candidate still attributes about 47.1% of samples cumulatively to GC drain and 38.5% to `check.Check`;
these cumulative categories are not additive. The remaining clean-check cost is in the checker/AST/allocation/GC path,
not a new reason to bypass checks or edit the separate core-performance work without integration review.

### Matched, uninstrumented pairs

Two balanced rounds (`baseline → candidate`, then `candidate → baseline`), same Go 1.25.14 toolchain, CPUs 8–15,
`GOMAXPROCS=8`, bounded shared `perf.lock`, and fresh work/cache directories for every target/round. Only `macro,bigpkg` and
`typing-clean,typing-broken-reqs` were selected (`-runs 1 -scale 1`); the rotating `!` hover series comes from the requests
scenario. The existing harness and validators were not changed. It still types 200 unique prefixes per scenario; requests use
`y`, distinct from the bare scenario's `x`, even though bare typing was not needed for this comparison.

**Validity before timing:** every reported scenario has zero failures/errors. Each project/target/round has 198/198 measured
samples in all three series. Server stats independently record exactly 200 hover, 200 inlay-hint and 200 semantic-token calls
per session, including the two validated warm-ups. Across the two rounds, that is 396 valid measured samples per series and
400 validated calls of each feature per project/binary. No missing-reply subset is used in any ratio.

Exact identities (SHA-256):
- baseline source `2192a725b3cbe020b9825cfa5eee17cdb700b19f`, binary
  `4ac23f21e3e44ace1e03093566d03be35542ed7d91950326382c01c90cefd3a7`;
- candidate binary `9b3757dd0ce9411d29fb73f3d40b6cbba71ef2f1e9737e60d294f6e033d54bc4`;
- frozen `2192a725` harness `639ffb3eac7f106229be2401d2813836444bea3086c179a39adc842b8846b607`.

Ranges below are the two individual session quantiles, **not pooled quantiles**, in milliseconds:

| Project | Series | baseline p50 | candidate p50 | baseline p95 | candidate p95 |
|---|---|--:|--:|--:|--:|
| macro | typing-clean | 9.350–9.427 | 8.570–9.409 | 13.033–14.813 | 12.461–13.314 |
| macro | typing-broken-reqs (diagnostics) | 2.252–2.275 | 2.251–2.266 | 2.626–2.763 | 2.632–2.702 |
| macro | typing-broken-hover | 0.692–0.704 | 0.699–0.708 | 14.026–14.115 | 4.857–4.875 |
| bigpkg | typing-clean | 96.406–96.713 | 96.609–96.676 | 115.722–123.766 | 119.096–121.377 |
| bigpkg | typing-broken-reqs (diagnostics) | 7.116–7.540 | 7.320–7.369 | 14.489–16.480 | 14.129–16.145 |
| bigpkg | typing-broken-hover | 1.142–1.612 | 1.057–1.098 | 129.300–147.087 | 28.882–31.436 |

Paired hover p95 improves 2.89× on `macro` and 4.48–4.68× on `bigpkg`. This does **not** finish the budget contract:
- bigpkg hover p95 misses 30 ms in round 2 (31.436 ms); measured maxima are 60.569 / 82.814 ms, versus baseline 167.048 / 155.308 ms;
- bigpkg clean typing remains above 100 ms in both candidate rounds (121.377 / 119.096 ms), with essentially unchanged p50;
- no full-project, gopls-relative or memory-budget pass is claimed. Direct-process stats still show high peak RSS: candidate
  macro 669.9–671.3 MiB and bigpkg 840.3–841.8 MiB (baseline 714.2–738.1 / 855.4–871.9 MiB). These are not wrapper-process measurements.

For the complete selected CLI sessions (both projects, required opens and both typing scenarios), elapsed time is baseline
55.307 / 56.294 s versus candidate 50.499 / 50.681 s. Total user+system CPU is 54.252 / 55.604 s versus 45.394 / 46.015 s.
The faster hover is therefore not merely compensated by more total CPU or a longer full-feature replay. Start/end one-minute
load observations span 1.44–7.81; two balanced rounds establish a narrow improvement, not a universal performance guarantee.

Raw artifacts are `paired-{1,2}-{base,cand}.{json,log,time,valid}`, with final call-count/sample validation in
`paired-validation-final.txt`; reproduction scripts preserve the frozen baseline/harness and isolated paths. Remaining clean
check/allocation/GC/cache costs require coordination with the separately reviewed core-performance work and a valid combined
measurement, not speculative edits to its load/cache/check paths here. This branch remains unmerged and not budget-complete;
independent whole-diff review of the hover shortcut is still required. No full benchmarks, broad tests or new Windows run
were performed for this performance follow-up.

## Committed-dev integration and Go 1.25 cohort

The authorized integration merge is `1aac3a448d908327e761c90d9357b9a3c56274f5`, with parents
`441d628fed073d08d186780c8695a522ccedd99e` and committed dev `e745f037546c9288a512cd52b9c4df07a1a53011`.
It brings the reviewed native fmt/Go 1.25 standard-library changes and round-6 import/version/memory/rename improvements
into this feature branch, not into dev. No other worker's uncommitted normal-loader changes were imported or edited.

Three conflicts were resolved by keeping both changes: round-6's stricter general budgets plus all macro typing budgets;
the fresh-edit/diagnostic tests plus strict typing-response tests; and generic ordinary/macro completion plus the numbered
fresh-text prefix and its correctly adjusted cursor. The provenance/reply validators and `fresh-edits-v2-go1.25` protocol
remain intact. `check.GoVersion` is `go1.25`, and generated Go modules declare `go 1.25.0`.

The initial focused integration checks found one generated-origin problem in two hover tests: the error result added to a
named must-function signature has `NoPos` before lowering, so rejecting invalid positions before checking its synthetic object
identity leaked `must_err1`/`must_err3` into partial signatures. Commit `4ed92e715654a047c67a2bdc0d73368a619772b9` gives positive
synthetic identity priority without treating unknown imported user names as generated. A bounded named-result regression
covers the exact case. An actual-request spy separately verifies unique fresh completion texts and cursor positions after
both `r.` and `#`, rather than merely repeating the helper's offset formula. No test expectations or validators were weakened.

Final named Linux selections passed: server 4.603 s, benchmark/provenance tests 0.872 s; race selections passed in server
10.495 s, benchmark 1.512 s, cache import/version tests 1.039 s and lazy-partial checker test 1.227 s. Initial focused cache,
checker and printer selections also passed (0.008 / 2.262 / 0.003 s). The initial feature selection included the existing
139-program must-form hover comparison, which reported the one named-result difference above; after the fix only the exact
regression and adjacent named tests were rerun, not that whole comparison. No full repository/phase/CI sweeps were run. The merge
preserves four upstream EOF-blank-line notices in gotest documentation/ported fixtures verbatim; their staged blobs were
verified identical to `e745f037`, rather than silently reformatting imported files.

### Real Windows LSP semantics with isolated harness cleanup

Both required tests passed on the real Windows PC: `TestWindowsLSPBang` 4.11 s and `TestWindowsLSP` 5.76 s, including CRLF,
UTF-16, alternate file spellings, disk invalidation, header import and expansion/shutdown behavior. Wine was not used.
With explicit authorization, a **host test-only Go overlay** omitted exactly the startup calls `removeStaleLocal(base)` and
`CleanStale(ctx, 24*time.Hour)` so earlier workers' temporary folders were not deleted. It did not alter tracked sources or
Windows LSP code. The actual executable built/uploaded by each test was captured and byte-compared with a reference built
from unmodified Windows sources; all have SHA-256 `f705aabdec73fcd8af5ceb7bacd8283ab7cb3911ee6bb2e02c95b17c723a999b`.
Overlay source/diff/hashes, the exact driver, compiler invocations and test logs are retained. This verifies Windows LSP
semantics with that isolated harness, **not** unmodified startup stale-cleanup behavior or the full Windows runner. An initial
host test launch used the wrong working directory and failed before reaching Windows; the corrected launch from `tests/`
is the passing result above.

The new performance cohort is absolute-budget evidence only. The standard library and harness/protocol differ from the
older measurements above, so no speedup ratios against those binaries are valid. Two fresh repeat sessions use one frozen
combined binary/harness, Go 1.25 workload semantics and complete response/provenance validation.

### Absolute results of the integrated cohort

Code was frozen at `4ed92e715654a047c67a2bdc0d73368a619772b9`; later documentation does not change those measured code inputs.
The two invocation IDs are `0113e56d2166ac88660107331534b240` and `19968f24ca7d6577955cfd0d47428967`.
Both used Go 1.25.14, CPUs 8–15, `GOMAXPROCS=8`, the bounded shared `perf.lock`, isolated fresh work/cache directories,
`-runs 1 -scale 1 -projects macro,bigpkg -only typing-clean,typing-broken,typing-broken-reqs`, and the unchanged strict
feature-response/provenance validators. Required initialize/open preparation also ran; no gopls or full reference-project
scenario matrix was requested. Start/end one-minute load observations were 0.06–1.79.

Every series below has **198/198 valid measured samples per invocation**, zero failures/errors, with two validated warm-ups
excluded from the quantiles. Each project/session independently records exactly 200 hover, 200 inlay-hint and 200 semantic-token
calls, plus 603 didChanges (600 prefix edits, 200 unique within each scenario, and their three cleanups). Across the two
repeats that is 396 measured samples per series/project and 400 validated calls of each feature. The strict cohort merge
accepted the two reports with identical project, binary, protocol and configuration identities, retaining both raw Sources.

All values are milliseconds. p50 ranges are the two session quantiles, not pooled quantiles; p95 columns remain separate:

| Project | Series | p50 range | p95 round 1 | p95 round 2 | Absolute p95 limit |
|---|---|--:|--:|--:|--:|
| macro | typing-clean | 8.223–8.231 | 10.593 | 10.339 | 100 |
| macro | typing-broken | 1.503–1.504 | 1.699 | 1.706 | 100 |
| macro | typing-broken-reqs (diagnostics) | 1.525–1.537 | 1.790 | 1.773 | 100 |
| macro | typing-broken-hover | 0.701–0.707 | 3.831 | 3.946 | 30 |
| bigpkg | typing-clean | 95.966–96.091 | 98.677 | 98.557 | 100 |
| bigpkg | typing-broken | 5.868–5.875 | 8.471 | 8.284 | 100 |
| bigpkg | typing-broken-reqs (diagnostics) | 6.610–6.614 | 13.518 | 14.852 | 100 |
| bigpkg | typing-broken-hover | 1.038–1.098 | 24.270 | 27.602 | 30 |

The requested **latency** limits are met in both measured repeats, but bigpkg clean typing has little margin. This is not a
maximum-latency claim: its clean maxima are 98.976 / 100.269 ms and hover maxima 47.000 / 46.841 ms. Elapsed time for each complete
selected invocation was 65.469 / 65.462 s; user+system CPU was 45.665 / 45.150 s. No ratios against the older incompatible
standard-library/harness cohorts are inferred.

Memory is still an honest limitation, not a pass inferred from the latency results. The harness's direct-process RSS/peak
observations are macro 174.605 / 166.781 MiB and bigpkg 416.465 / 398.734 MiB. The server's separate `gocpp/stats` high-water
observations are macro 174.645 / 166.785 MiB and bigpkg 419.910 / 398.738 MiB; the small sampling difference is not discarded to
choose a favorable result. Bigpkg exceeds the nominal 400 MiB budget in the first repeat. No overall memory-budget, full-project
or gopls-relative pass is claimed. No new latency profile was run because the requested typing/hover p95 limits were met;
normal-loader parallelization and any further core memory work remain separate jobs, not speculative edits in this branch.

Exact provenance:
- Linux server SHA-256: `62ac6d3f23d9fda6ea36c0ebbfd031659b240481cd72a402ab72ad24168205e6`;
- harness SHA-256: `8c8d19eb6bc7ec1e672d60963bb1f7aeb3c7116b28e8665f3cc42030a96f5ef6`;
- macro workload SHA-256: `b7d231fa819b8d5840704d193d6e9da4e6c6e46c9085bccca7ff64072751f48c`;
- bigpkg workload SHA-256: `01722ba388159baf9ef928612822b1ea00b0dbb4e23e10167aeae57054bd5c18`;
- standard-library tree: `226f82ca8a71397453066634847743b8f6737eb5`; runtime tree: `5e174f97acab2f1f184c6b96b855aa6cdc6c5947`;
- the JSON retains named configuration fingerprints, Go command/version, affinity, target configuration and raw samples.

Raw artifacts are `combined-{1,2}.{json,log,time,valid}`, `combined-validation.txt`, `combined-merged.json`, source/binary identity
files and the exact `measure-combined.sh` driver. Windows overlay, executable hashes and isolated-harness logs are separate.
The feature branch remains unmerged into dev; independent whole-diff review of the combined integration and the synthetic-origin
correction is required. These observations do not mark step 11.13 complete or approve shipment.

## Focused memory diagnosis after `30ae6890`

### The over-budget metric is real RSS

The 419.910 MiB observation above is **not** Go heap reservation renamed as memory. The server's `processMemory` reads Linux
`/proc/self/status` `VmRSS` and `VmHWM` in KiB and converts them to bytes. The harness reads the corresponding fields from
`/proc/<actual-server-pid>/status`; there was no wrapper process in the measured cohort. The few MiB between the two observations
are sampling time, not grounds to discard the larger peak. In that same first run, Go `HeapAlloc` was 375.063 MiB,
`HeapInuse` 382.148 MiB and `Sys` 404.487 MiB, versus actual RSS/high-water 419.910 MiB. The second run had HeapAlloc
337.627 MiB and actual RSS/high-water 398.738 MiB. The nominal 400 MiB RSS goal really was exceeded in the first run.

### Separate, live profiling runs — not latency measurements

A recorded, scratch-only Go overlay added a fixed stage-end capture to the existing 200-unique-prefix typing workload,
a read-only feature-view pointer inventory, and a live heap dump at the existing profiling signal-stop hook. The profiling
harness identifies itself as `memory-profile-only-v1-go1.25`. It captures stats and `/proc` values **before** stopping the server
with SIGTERM while its normal server/cache roots are still live, rather than profiling after normal shutdown has released them.
The profile hook's existing one terminal GC supplies the live-object heap/dump; no production GC policy or per-feature-request
GC was added. No GOGC/GOMEMLIMIT tuning, validators, budgets or tracked production code were changed.

Two bounded `bigpkg` captures used Go 1.25.14, CPUs 8–15, GOMAXPROCS 8, isolated caches, offline/module-cache settings,
the shared perf lock and one heavy job at a time:

| Live stop point | Completed unique prefixes in that scenario | Actual RSS / high-water before terminal GC | Check-cache entries / estimated bytes | Live snapshots / repair views |
|---|--:|--:|---|---|
| after clean typing, before cleanup | 200 | 132.848 / 132.848 MiB | 24 / 64,103,655 | 1 / none |
| after clean + bare + requests, before final cleanup | 200 | 414.293 / 414.293 MiB | 67 / 239,807,063 | 2 / one |

The request-stage counters are 602 didChanges (three 200-prefix scenarios and the first two cleanups) and exactly 200 hover,
200 hint and 200 token calls; their unchanged response validators completed before the capture. The signal-stop intentionally
ends profiling before ordinary scenario cleanup. The CLI then reports closed-stream/put-back errors for subsequent work; those
outputs and their post-exit zero RSS are **not** accepted benchmark measurements or a pass. Only the recorded pre-stop snapshot
and live profiles are used here. An initial driver expected exit 1, while this diagnostic CLI path returns 0 after recording
scenario failures; the marker, 200 unique edits, live snapshot and profile files establish the intentional stop independently.

The sampled live heap is about 38.45 MiB after clean typing and 259.65 MiB at the request stop. At the latter, roughly 240 MiB
(92.4% cumulative sampled in-use space) originates under `cache.(*Pipeline).check`; go/types type/value/use/scope tables,
parsed ASTs, must rewrites and synthetic maps dominate. These sampled cumulative categories are not additive. Full live heap
dumps independently record 44,885,912 versus 282,543,832 bytes of post-GC HeapAlloc, with 545,152 versus 3,513,663 dumped objects.
The inventory has one old repair view with refs=1 and only one additional live snapshot, not an unbounded list of feature views.

### Retainer proof and the core-cache dependency

A bounded reader for the Go 1.25 runtime's `go1.7 heap dump` format followed the dumped GC pointer bitmap, resolving interior
pointers to their containing objects. It traversed from the recorded pipeline pointer while explicitly blocking the server,
session and views objects, so a back-reference through the feature controller could not manufacture the result. It found
40.252 MiB reachable from that core root in the clean capture and **245.213 MiB** in the request capture. This is graph
reachability, not a claim that all those bytes are exclusive to the pipeline or that their deletion alone would equal RSS.

Both packages held by the last feature view are independently reachable from the core check cache:

- repaired package `0xc010898000`: pipeline → check store → LRU element → entry → completed promise → package;
- original partial package `0xc010899980`: pipeline → check store → entries map → entry → completed promise → package.

The raw object paths and offsets are preserved. Compiled DWARF identifies `Pipeline.checks` at offset 248; its actual pipeline
pointer is eight bytes into the heap allocation, matching the dumped first-edge offset 256. The store's LRU and entries fields
are at offsets 32 and 24, respectively, matching the two paths. Thus merely releasing the feature view's package references
cannot reclaim those same graphs while the cache independently retains them.

The relevant current policy is concrete: `cache/exports.go:153–193` creates `PackageWithFile` handles with `variant: true`,
while `cache/packages.go:870–875` applies the three-version keeper only when `!h.variant`. Repair/completion variants therefore
remain in the shared cost-capped check store instead of the ordinary package's three-version policy. That explains a supported
core follow-up: evaluate a separately bounded/lifetime-aware variant cohort or lease/release API, keeping complete original
input/API keys, concurrent users, completion behavior and current-snapshot semantics intact. It is not permission to drop
responses, reuse wrong input hashes, reduce the global cache blindly or force GC on requests. The allocation profile alone
would not prove this retention path; the live object graph and DWARF offsets supply the independent ownership evidence.

**No speculative production memory fix was made.** The check-store/variant policy belongs to the separately owned core area;
normal-parser worker `541db0ab` was neither merged nor edited. All current-partial/stale/API/cancellation/refresh behavior and
previous correctness fixes remain unchanged. Since there is no candidate memory fix, no three-session candidate timing run
or small-macro control was presented as if it could prove success. The prior complete joint latency/RSS baseline remains
valid, and still contains the real over-budget RSS observation. A core variant-retention change needs its own implementation,
focused equivalence/race review and then three valid joint bigpkg sessions plus a macro control before claiming the memory
and latency limits together.

Profiling artifacts include stage stats/inventories, `heap.pprof`/CPU profiles, full live `retainers.heapdump` files, decoded
pointer paths, DWARF layouts, source/overlay hashes and exact drivers. Profiling server SHA-256 is
`cb53801f87dbb52780a639b75818167d19f14c64cb8c4e4303fe58f2c46c208b`; diagnostic harness SHA-256 is
`23d66c124e4e692bcee3675796a6e80d250674565648d09bd294dec132767f02`. These instrumented binaries and their deliberate
signal-stop outputs are never mixed with the uninstrumented timing cohort. No production tests or Windows runs were repeated
for this documentation-only diagnosis; the branch remains unmerged and the memory/overall exit criteria remain unfulfilled.

## Authorized core variant-retention follow-up

The next assignment explicitly authorized the narrow core-cache change. First, automatic merge
`69d41666818f7b95b7130cf9e726481a28331422` integrated **exact committed** normal-parser revision
`9a2b08714c9ffd47b01d8cd2f5e5984749300c9a` into the clean feature branch. No receiver/rename worker changes or moving dev state
were imported. This post-parser revision was frozen before the fix; the historical pre-parser memory cohort above is not a
controlled timing comparator.

### Policy and ownership proof

Implementation `0963582008634ea90800c1974db57fd3c838bb3a` uses a second instance of the existing **hash-only version keeper**
for patched-file variants, not a duplicate result cache. Each package keeps up to its latest three ordinary versions and its
latest three variant versions in the same globally cost-capped check store. Variant attempts returning completed errors are
also noted so small cached error values cannot escape the cohort bound. Ordinary behavior, global cap and the existing
one-oversized-value rule are unchanged.

Keys are unchanged: `PackageWithFile` still includes the original input identity and complete replacement hash in its variant
namespace, then imports/API/configuration in the normal check key. No reuse based on spelling or stripped keys was introduced.
The keeper remembers hashes only. `Store.ForgetIf` removes a completed entry from the store/LRU, checking that it is still the
same entry; it does not cancel its promise, mutate its package or revoke live request/snapshot/view references. It refuses to
forget an in-flight promise. Active computations can temporarily outlive a keeper hash; their callers note completion again,
so the completed cache-owned cohort is bounded after requests quiesce. Eviction means recomputation on a later miss, not
invalidating an already returned value.

Deterministic tests cover 40 unique variants, recent reverts, active reads across eviction, ordinary/variant namespace and
quota isolation, original/replacement/configuration key distinctions, sibling/import API changes and reverts, concurrent old
and changed snapshots, one shared check after canceled attempts, a registered canceled waiter while another consumer owns an
in-flight promise, completed error bounds and unchanged global-cap behavior. The targeted cache selection passed initially;
then the named policy/version/import tests passed with `-race -count=10` (1.883 s). Adjacent feature guards, cancellation,
refresh/interleaving, current-partial/API/UTF-16 behavior, partial testing policy, expansion and strict benchmark/provenance
regressions passed under race: server 13.380 s and harness 1.658 s. No checker/load/rename redesign, per-request GC, GOGC or
GOMEMLIMIT tuning, validator weakening, Windows rerun or broad sweep was used.

### Matched post-parser profile proof — separate from timings

A fresh, scratch-only profiling cohort used the same diagnostic stage-stop mechanism described above, at the end of the
200-prefix request scenario, before cleanup. The baseline overlay restored only the four pre-policy cache source files from
`69d41666`; the candidate used `09635820`. Both include the same reviewed normal parser, stdlib, diagnostic instrumentation,
Go 1.25 workload and configuration. Each captured 602 didChanges and 200 complete hover/hint/token calls, with 200 unique
prefixes in the last scenario. These intentional signal-stop profiles are never mixed with latency samples.

| Diagnostic observation | Post-parser baseline | Candidate |
|---|--:|--:|
| actual pre-stop RSS / VmHWM | 413.813 / 413.813 MiB | 156.863 / 156.863 MiB |
| pre-stop HeapAlloc | 364.455 MiB | 106.654 MiB |
| post-GC live HeapAlloc | 259.855 MiB | 57.576 MiB |
| check-cache entries | 68 | 27 |
| check-cache estimated bytes | 243,897,162 | 76,378,221 |
| live graph reachable from pipeline, excluding feature roots | 257.114 MiB | 54.882 MiB |

The global cache cap remains 314,572,800 bytes in both captures. Active views/snapshots still exist: the candidate has two
live snapshots rather than freeing users to manufacture a low heap. These data corroborate a retention reduction, not a
renaming of heap reservation or moving a real RSS peak out of the measurement. Raw live dumps, pointer paths, inventories,
source overlays and their hashes are retained for independent checking.

### Uninstrumented joint budget checks

The candidate was measured with the frozen, unmodified Go 1.25 harness after the correctness/profile proof: three independent
fresh `bigpkg` sessions and one `macro` control, CPUs 8–15, GOMAXPROCS 8, shared bounded `perf.lock`, isolated caches,
`-runs 1 -scale 1`, only clean/bare/requests typing and its hover series. No runtime GC/memory overrides or profile flags were
used. All reported series have 198/198 measured samples and zero errors per session, plus two validated warm-ups. Each session
records 603 didChanges and exactly 200 calls each of hover, hints and semantic tokens. The three bigpkg reports have identical
binary/project/protocol/configuration identities and were accepted by the strict merger, retaining all raw Sources.

All values below are individual-session p95 milliseconds, not pooled quantiles:

| Series | bigpkg 1 | bigpkg 2 | bigpkg 3 | macro control | Required p95 |
|---|--:|--:|--:|--:|--:|
| typing-clean | 97.852 | 97.988 | 97.837 | 10.648 | <100 |
| typing-broken | 7.395 | 7.262 | 7.376 | 1.545 | <100 |
| typing-broken-reqs (diagnostics) | 7.841 | 9.521 | 10.040 | 1.709 | <100 |
| typing-broken-hover | 24.272 | 24.056 | 23.664 | 3.623 | <30 |

Actual resident-memory observations, in MiB:

| Collector/metric | bigpkg 1 | bigpkg 2 | bigpkg 3 | macro control |
|---|--:|--:|--:|--:|
| harness RSS | 160.617 | 153.027 | 163.254 | 119.855 |
| harness VmHWM | 160.617 | 154.594 | 163.254 | 119.855 |
| server RSS | 160.617 | 153.031 | 163.258 | 118.344 |
| server VmHWM | 160.617 | 154.594 | 163.258 | 118.344 |

The largest observation from **either actual-process collector** is used, not whichever is favorable: 163.258 MiB for bigpkg
and 119.855 MiB for the control. All three bigpkg sessions jointly meet the selected 400 MiB resident peak, 100 ms clean/typing
p95 and 30 ms hover p95 limits. Clean p95 still has only about 2 ms margin. This is not an every-request bound: bigpkg hover
maxima are 45.411 / 44.896 / 46.241 ms; clean maxima are 98.642 / 99.885 / 98.912 ms. The three complete invocation durations
are 42.338 / 42.340 / 42.277 s, with a 22.213 s macro control; start/end one-minute load observations span 0.42–0.93.

The three bigpkg sessions supply 594 valid measured samples per series and 600 validated calls of each feature; the control
adds 198 / 200 respectively. Raw per-run samples and high-water observations remain intact. No speedup ratio against the
pre-parser or earlier stdlib cohorts is inferred, and no gopls/full-project/all-exit-criteria acceptance is claimed.

Frozen identities:
- post-parser baseline source `69d41666818f7b95b7130cf9e726481a28331422`, server SHA-256
  `4fb95376b565ac3aec652e8f816342195fc78c5b421ed827a3a4cad0e204970a`;
- candidate code `0963582008634ea90800c1974db57fd3c838bb3a`, server SHA-256
  `7fcef09ffd5a02563f1f893a720462df3fbbfca7d84398147f2965f3d393a475`;
- unchanged harness SHA-256 `8c8d19eb6bc7ec1e672d60963bb1f7aeb3c7116b28e8665f3cc42030a96f5ef6`;
- bigpkg workload SHA-256 `01722ba388159baf9ef928612822b1ea00b0dbb4e23e10167aeae57054bd5c18`;
- macro workload SHA-256 `b7d231fa819b8d5840704d193d6e9da4e6c6e46c9085bccca7ff64072751f48c`;
- stdlib/runtime trees remain `226f82ca8a71397453066634847743b8f6737eb5` / `5e174f97acab2f1f184c6b96b855aa6cdc6c5947`.

Artifacts include `bigpkg-{1,2,3}.{json,log,time,valid}`, `macro-control.*`, strict merged JSON, correctness/race logs and separate
`proof-{baseline,candidate}` profiles/dumps. The feature branch stays unmerged into dev; independent whole-net review of the
policy, ownership/concurrency proof and measurement provenance is required before any integration or completion decision.

## Exact current-dev rename integration

Authorized automatic merge `b9cf59fbe2c82fb0bfc76811555f7df1ecd75235` brings **only exact committed**
`ea1005e4795c6f783ebfe06a94b1757611617d76` into the clean feature branch at `401a73f2`. There were no conflicts and no moving-dev,
main staged receiver or other working-tree changes were imported. The separate three-version variant policy, complete keys,
active-holder/error/cancellation behavior and strict benchmark validators remain unchanged.

### Conservative origin guard found by integration checking

Existing declaring-binding proof/equivalence tests passed after the merge, including their assertions that eligible plain,
complete Go packages perform zero declaring-package scratch checks versus a positive full-validator count. Macro/bang and
other unsupported forms still fell back. A focused additional probe nevertheless showed that a **complete plain patched-file
package** could pass `normalBindingFacts`, because the prior structural checks did not identify the input namespace itself.
The probe failed with `patched-file variant was accepted as normal binding provenance`; no proof condition was weakened to
make it pass.

The manager explicitly authorized narrow correction `58d610220454165d2a07a00b06a4295440f28c08`: `keyOf` records the immutable
variant origin of its actual input-key namespace, and `Pipeline.check` copies that fact into `Package` before cache publication.
The read-only `IsVariant` accessor is rejected before the ordinary binding-facts memo. This is not a caller hint, spelling
heuristic, snapshot guess or mutable global label; no key or retention policy changed. Cached/recomputed results, late
completions and active holders retain their origin, including byte-identical overrides.

Named provenance tests cover normal/exact positives, mismatched type-info/file-set/syntax views, identical-byte and complete
plain variants, cache hits and eviction/recomputation. The original full-validator positive and negative suites remain intact,
including zero-scratch assertions, Unicode/UTF-16, conflict/capture, missing-occurrence and cancellation checks. Focused race
verification passed: server 5.496 s, cache 1.109 s, benchmark/provenance tests 1.659 s. The variant package/reflection/ownership,
API/revert, NoPos user-name, refresh/interleaving and current-partial regressions passed in the same named selection. The initial
failing integration probe and final passing logs are both preserved.

Because production LSP rename behavior changed, both actual Windows checks were repeated: `TestWindowsLSPBang` 4.00 s and
`TestWindowsLSP` 5.88 s, including rename, alternate file spelling, CRLF/UTF-16, expansion and shutdown. They used the previously
authorized cleanup-only **host harness** overlay omitting the same two startup stale-directory cleanup calls, never Wine. Both
actual built/uploaded executables matched the unmodified Windows-source reference byte for byte, SHA-256
`44eaff365aae650b011b338be52fc39f248035d4d4e8d231284b49db52902dbc`. This is isolated-harness Windows LSP coverage, not unmodified
startup-cleanup or full-runner verification; only this run's files were cleaned.

### One post-merge joint session, separate from the earlier three-run cohort

The required single invocation (`ef4813e9da2b08d2ea7216f25785698d`) used the frozen Go 1.25.14 harness/configuration, CPUs 8–15,
GOMAXPROCS 8, bounded shared `perf.lock`, isolated fresh caches, `-runs 1 -scale 1`, and only macro/bigpkg clean/bare/requests
typing with the hover side-series. Each scenario still types 200 unique prefixes, with two validated warm-ups and **198/198
measured samples**, zero failures/errors. Each project independently records 200 hover, 200 hint, 200 token calls and 603
didChanges. No matching error/refusal or missing response is counted as a sample.

| Series (p95 ms) | macro control | bigpkg | Required p95 |
|---|--:|--:|--:|
| typing-clean | 10.454 | 97.755 | <100 |
| typing-broken | 1.589 | 7.095 | <100 |
| typing-broken-reqs (diagnostics) | 3.237 | 8.389 | <100 |
| typing-broken-hover | 3.665 | 22.409 | <30 |

Harness RSS/VmHWM is 116.785 / 116.785 MiB on macro and 160.563 / 160.563 MiB on bigpkg. The independent server counters read
116.801 / 116.801 and 160.566 / 160.566 MiB; using the larger actual-process observation yields **116.801 / 160.566 MiB**,
not heap reservation. Thus this single post-merge check jointly meets the selected 400 MiB, 100 ms and 30 ms limits. It does
not replace or get pooled with the separately labeled three-bigpkg/control cohort above. The clean bigpkg maximum was
100.205 ms and hover maximum 44.683 ms: a p95 budget is not an every-request guarantee. Total invocation time was 64.476 s,
with 44.247 s user+system CPU; start/end one-minute load observations were 0.74 / 1.09.

Measured code is `58d610220454165d2a07a00b06a4295440f28c08` (later documentation only):
- Linux server SHA-256 `1ab26c651e6a6fbfeb7bb0ee2471fd9fb9fbfe38fdcc6dbcc2220462c915b49c`;
- unchanged harness SHA-256 `8c8d19eb6bc7ec1e672d60963bb1f7aeb3c7116b28e8665f3cc42030a96f5ef6`;
- macro/bigpkg workload hashes remain `b7d231fa819b8d5840704d193d6e9da4e6c6e46c9085bccca7ff64072751f48c` /
  `01722ba388159baf9ef928612822b1ea00b0dbb4e23e10167aeae57054bd5c18`;
- raw JSON retains protocol, configuration fingerprints, actual-process memory, complete samples and exact invocation identity.

Artifacts are `current.{json,log,time,valid}`, source/binary hashes, initial/final rename/provenance race logs, and separate
Windows cleanup-only overlay/diff/hashes/drivers with actual executable comparisons. No gopls comparison, extra matrix,
broad sweep or manual plan-state edit was performed. The branch remains unmerged into dev; same-reviewer whole-current-net
review of the integration and explicitly authorized provenance guard is required before any landing or completion decision.
