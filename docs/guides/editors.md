# Editors

gocpp has a language server, `gocpp lsp`. It speaks the Language Server Protocol (LSP 3.17)
over standard input and output, so any editor with an LSP client can use it. The editor starts
`gocpp lsp` and talks to it; there is nothing else to run.

What the server does now: it shows the errors of `gocpp check` as you type, and it has hover,
go to definition and type definition, signature help, completion (with snippets for calls and
the imports it adds for you), formatting and organize imports. The other features (rename,
references, symbols, ...) come in later versions of the server, and they need no change in the
editor setup below: the editor asks the server what it can do.

All the setups need the `gocpp` binary in `PATH` (or its full path in the config). Build it in
the gocpp repository with `make build`; that writes `bin/gocpp`.

| Editor | Setup | Syntax highlighting | Tested |
|---|---|---|---|
| VS Code | the extension in [editors/vscode](../../editors/vscode/README.md) | a gocpp grammar | the extension's tests run in Node.js, not in VS Code itself |
| Neovim | a few lines of Lua, below | Go's, if you want it | yes, Neovim 0.11.6 |
| Helix | `languages.toml`, below | Go's | not tested yet |
| Emacs | eglot, below | Go's, with `go-mode` | not tested yet |
| Zed | not supported yet | | |

**`.gp` files.** Other programs use `.gp` too: gnuplot, and PARI/GP (Neovim calls `.gp` files
`gp`, for PARI/GP). So each setup below says that `.gp` is gocpp. If you also edit gnuplot or
PARI/GP files, give those another extension (gnuplot scripts are also called `.gnuplot`,
`.plt` or `.gpi`).

**Highlighting.** Only VS Code has a gocpp grammar for now. The other editors can use their
Go highlighting: gocpp is Go syntax with a few additions (`name!(...)`, `#Macro(...)`, header
imports), and those additions may show as errors in a Go grammar. A tree-sitter grammar for
gocpp may come later.

## VS Code

See [editors/vscode/README.md](../../editors/vscode/README.md): how to install the extension
from source, its settings, and how to see the server log.

## Neovim

Neovim 0.11 and later have the LSP setup built in. Put this in your `init.lua`:

```lua
-- .gp files are gocpp (Neovim's default is "gp", for PARI/GP).
vim.filetype.add({ extension = { gp = 'gocpp' } })

vim.lsp.config('gocpp', {
  cmd = { 'gocpp', 'lsp' },
  filetypes = { 'gocpp' },
  root_markers = { 'gocpp.mod', '.git' },
})
vim.lsp.enable('gocpp')

-- Optional: tabs, as gocpp fmt writes them, and Go's highlighting. The
-- treesitter line needs the Go parser (:TSInstall go with nvim-treesitter).
vim.api.nvim_create_autocmd('FileType', {
  pattern = 'gocpp',
  callback = function(args)
    vim.bo[args.buf].expandtab = false
    vim.bo[args.buf].commentstring = '// %s'
    pcall(vim.treesitter.start, args.buf, 'go')
  end,
})
```

Open a `.gp` file: `:checkhealth vim.lsp` shows the running server, and the errors show as
diagnostics. Format on save, when the server can format (the server cannot format yet, so
until it can, this does nothing; without the check, Neovim would print "Format request
failed" on each save):

```lua
vim.api.nvim_create_autocmd('BufWritePre', {
  pattern = '*.gp',
  callback = function(args)
    local client = vim.lsp.get_clients({ bufnr = args.buf, name = 'gocpp' })[1]
    if client and client:supports_method('textDocument/formatting') then
      vim.lsp.buf.format({ bufnr = args.buf, id = client.id })
    end
  end,
})
```

With an older Neovim (0.10) and [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig),
add gocpp as a custom server (not tested yet):

```lua
vim.filetype.add({ extension = { gp = 'gocpp' } })

local configs = require('lspconfig.configs')
if not configs.gocpp then
  configs.gocpp = {
    default_config = {
      cmd = { 'gocpp', 'lsp' },
      filetypes = { 'gocpp' },
      root_dir = require('lspconfig.util').root_pattern('gocpp.mod', '.git'),
    },
  }
end
require('lspconfig').gocpp.setup({})
```

The client's log is `:lua vim.cmd.edit(vim.lsp.get_log_path())`. The server writes its own log
to standard error, and Neovim puts each line of it in that log too, as an `ERROR` line with
`"stderr"` (that is how Neovim logs standard error; it is not an error of the server). To get
the server log in a file of its own, use `cmd = { 'gocpp', 'lsp', '-logfile', '/tmp/gocpp-lsp.log' }`.

## Helix

Not tested yet. Put this in `~/.config/helix/languages.toml` (or `languages.toml` in the
`.helix` folder of a project):

```toml
[language-server.gocpp]
command = "gocpp"
args = ["lsp"]

[[language]]
name = "gocpp"
scope = "source.gocpp"
file-types = ["gp"]
roots = ["gocpp.mod"]
comment-token = "//"
block-comment-tokens = { start = "/*", end = "*/" }
indent = { tab-width = 4, unit = "\t" }
language-servers = ["gocpp"]
grammar = "go"
# auto-format = true   # format on save, when the server can format
```

`grammar = "go"` parses `.gp` files with Helix's Go grammar. Helix looks for the highlighting
rules under the language's name, so also make the file
`~/.config/helix/runtime/queries/gocpp/highlights.scm` with this one line, which takes Go's
rules:

```scheme
; inherits: go
```

`hx --health gocpp` checks the setup: it should find the `gocpp` binary. The log, with the
server's messages, is in `~/.cache/helix/helix.log` (start Helix with `hx -v` for more).

## Emacs

Not tested yet. This uses eglot, which is part of Emacs since Emacs 29. Put this in your init
file:

```elisp
;; A major mode for .gp files. With go-mode installed, use go-mode as the
;; parent instead of prog-mode, to get Go's highlighting.
(define-derived-mode gocpp-mode prog-mode "gocpp"
  "Major mode for gocpp (.gp) files."
  (setq-local comment-start "// ")
  (setq-local comment-end "")
  (setq-local indent-tabs-mode t)
  (setq-local tab-width 4))
(add-to-list 'auto-mode-alist '("\\.gp\\'" . gocpp-mode))

(with-eval-after-load 'eglot
  (add-to-list 'eglot-server-programs '(gocpp-mode . ("gocpp" "lsp"))))
(add-hook 'gocpp-mode-hook #'eglot-ensure)
```

Eglot sends the language id `gocpp` (from the mode name). Format on save, when the server can
format: add `(add-hook 'before-save-hook #'eglot-format-buffer nil t)` to `gocpp-mode-hook`.
`M-x eglot-events-buffer` shows the messages, and the server's standard error (its log) is in
a buffer whose name starts with `*EGLOT` and ends with `stderr*`.

## Zed

Not supported yet. Zed adds a new language only through a Zed extension, which needs:

- a tree-sitter grammar (Go's could do at first, as in Helix), with highlighting queries;
- the language (`languages/gocpp/config.toml`: the name, `path_suffixes = ["gp"]`, comments,
  tabs);
- a small Rust program compiled to WebAssembly that tells Zed how to start `gocpp lsp`.

This may come with the tree-sitter grammar. Until then, Zed can at least highlight `.gp` files
as Go, with `"file_types": { "Go": ["gp"] }` in its settings; but then Zed also starts gopls for
them, which does not know gocpp.

## Other editors

Any LSP client works: the command is `gocpp lsp`, over standard input and output, for files
with the extension `.gp`. The language id is `gocpp`, and the root of a project is the folder
with `gocpp.mod`. `gocpp lsp -h` lists the flags.
