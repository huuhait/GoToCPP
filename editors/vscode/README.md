# editors/vscode

The VS Code extension for gocpp. It comes in **Phase 11** (step 11.10) and will have:

- the `gocpp` language for `.gp` files, with a TextMate grammar derived from an MIT-licensed Go
  grammar, plus header imports, `name!(`, `func name!(`, `#Name`, `#body()` and `macro`;
- the LSP client for `gocpp lsp`, and format on save;
- `contributes.breakpoints`, so breakpoints in `.gp` files work without extra settings
  (debugging is Phase 13).

It is empty for now. The TextMate grammar needs nothing else, so it can be written any time.

`.gp` is also the file extension of gnuplot. To change which language VS Code picks, use
`files.associations`.
