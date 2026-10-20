# Editors

gocpp has a language server, `gocpp lsp`. It speaks the Language Server Protocol (LSP 3.17)
over standard input and output, so any editor with an LSP client can use it. The editor starts
`gocpp lsp` and talks to it; there is nothing else to run.

What the server does now: it shows the errors of `gocpp check` as you type, and it has hover,
go to definition and type definition, signature help, completion (with snippets for calls and
the imports it adds for you), formatting and organize imports, find references, rename,
document highlights, document and workspace symbols, semantic tokens, inlay hints, document
links, a quick fix for a missing import, and the features for `!`, macros and `#Derive` (below,
"`!`, macros and `#Derive`"). Some features come in later versions of the server (the ones for
C/C++ names), and they need no change in the editor setup below: the editor asks the server what
it can do.

**References and rename.** They work on the names that gocpp code declares, in all the packages
of the module that the file is in (one `gocpp.mod`; the other modules of the workspace are not
searched, and you get a warning when there are any), in the unsaved text of the open files, and
in `_test.gp` files (internal and external tests, also of packages that import the package only
in their tests).

*What a method stands for (the family).* A **rename** of a method changes the method you ask
about and the methods that tie to it, so that the program means what it meant before:

- a *use* ties a type's method to an interface's: wherever a value of one type is passed,
  assigned, returned, sent, put in a composite literal, compared or converted where an interface
  with the method is wanted (the interface may be named, written as a literal in a parameter, or the
  constraint of a type parameter that the call instantiates; the type may be a named type, a pointer,
  or an unnamed struct that embeds one), the type's method and the interface's method are one method
  and change together, also in the test files and in other packages. This is gopls's family. For an
  unexported method the importers are searched too (a `main` that passes a `base.Dog` to a
  `func(base.Animal)` ties the unexported `Dog.name` to `Animal.name`);
- an interface that a type assertion, a type switch or `errors.As` takes a value out as is followed
  too, **which gopls does not do**, for a **rename** only (references and highlights say where the
  method you asked about is used, and do not follow it): a value that is converted to an interface
  anywhere in the module (passed, assigned, returned, sent, put in a composite literal, also to
  `any`: `fmt.Println(v)`) can come out of an assertion as the interface, so the interface's method
  and the methods of the types that implement it and are converted somewhere in the module change
  together. Without it the program compiles and the assertion fails at run time. The assertion may
  be in any package of the module, also one that does not import the type's package (`core.IsCloser(x
  any)` asserts `x.(Closer)` on what `main` passes it). A type is *reached* when a value of it (or of a struct, named or not, local, generic or an
  alias, that embeds it, or that holds it in a field, an element, a key, a pointee or a function
  result: `reflect` and `fmt` take those out as interfaces) flows to an interface in any form: it is
  converted, passed, assigned, returned, sent, put in a composite literal, ranged into a variable,
  used as the key of a map index, returned by a call whose values are passed on, or read with `v, ok`.
  A type that nothing reaches is left alone: no value of it can come out of
  an assertion, so a module of 30 packages that all have a `Close() error` does not become one
  family because `core` asserts `Closer`. The types are compared with the interface by the names and
  signatures of their methods. When the assertion would add more than 20 methods the rename is
  **refused**, with the count and the assertion, and you are told how to go on: it would change that
  many methods across the module on a guess about what flows where. The limit is the setting
  `dynamicLimit` (`gocpp.dynamicLimit` in the editor's server settings: the `initializationOptions`,
  or `workspace/didChangeConfiguration` with `{"gocpp": {"dynamicLimit": n}}`; `-1` for any number),
  or `gocpp lsp -dynamic-limit n` (in VS Code: `"-dynamic-limit", "n"` in `gocpp.lsp.args`). Test files are looked at
  too (an assertion or a conversion in a `_test.gp` file counts). Not looked at: values that reach an interface through `reflect` alone, and the types of other
  modules;
- a type that merely *could* be used as the interface, with no use that needs it to and no
  assertion that takes it, is left alone: renaming `Lener.Len` leaves `Counterer.Len` alone when
  nothing converts a `Counterer` to a `Lener`. It stops satisfying `Lener`. Nothing in the module
  can tell (it compiles, and no check sees the difference); only reflection and code outside the
  module could.

A field of an anonymous struct type (`struct{ x int }` written where a type is wanted) is renamed in
every identical struct type of the module, as the types would stop being identical otherwise.

A method that is renamed is also refused when a type that embeds its type already has the new name,
at any depth (a field, a method, or one that the rename would make ambiguous): the method set of the
embedder, made by promotion, would lose or change a method with no error from the build, as `W` in
`type W struct { a.A; b.B }` where `A.M` becomes `N` and `B` has an `N`.

A rename of a method or a field also warns when a string literal of the module is its name
(`MethodByName("Close")`, `FieldByName`): reflect and the encoders find them by that string, which a
rename does not change.

A rename tells you, with a message, how many other declarations it changed (and the first few), so
that you can undo it. These are refused instead, because the program would change without a
compile-time error: a method that implements a method of an interface that is not in the module
(`String` and `fmt.Stringer`, `Error` and `error`); `Unwrap`, `Is` and `As` of an error, which
the `errors` package calls by name; and an `init` function. The edit is also checked at the end by
type-checking the packages it changes and the importers of the packages whose methods it
changes: if an importer would no longer compile, it is refused.

**References and highlights** of a method use the same family, except for the interfaces outside
the module (the references of `Error` are those of that method, not of every `Error` method of the
module that returns as an `error`). This differs from gopls, which lists the declaration of the
method you asked about and the uses (calls) of the methods of every type that implements the
interface, tied by a use or not, but not the declarations of those methods: gocpp lists the
declarations and the uses of the tied methods. `gocpp lsp -wide-family` restores the older, wider
family for both (every method of the module with the name and shape of a method of an interface
the type satisfies, needed by a use or not; it is slow when many types share a method name, and
wider than gopls; it has none of the later guards above: the assertion follow-up, the importers of an
unexported method, the refusals of `Unwrap`, `Is`, `As` and `init`). Both modes are written down in
[ADR-052](../adr/052-lsp-speed-work.md).

*What a rename checks.* It is refused, with a message, when the new name would clash with another
name, hide a use, or be captured (also in the test files), when the name is used in other
packages and the new name is not exported, when the name belongs to the standard library or to a
package outside the module, when it is `func main`, when a method is related to a method of an
interface outside the module, when an importer of the package cannot see the package's names
because the package has an error (a syntax error, an import that fails: fix it first), and when
the edit would make a compile-time error that the code did not have (the edited packages and test
files are type-checked in a scratch copy). That is all that it guards: a rename that makes a
*new* implements relation (a method renamed to `String` or `Error`, so that `fmt` now calls it, or
a method that stops shadowing a promoted one) changes the behavior without a compile-time error,
as in gopls; gocpp warns about the common case, a method renamed to `String`, `Error`, `Read`,
`Write`, `Close` or `Len`/`Less`/`Swap` with the signature that makes its type satisfy the
standard interface. A file that the build leaves out (`x_windows.gp` on Linux) is not checked, so a
rename does not change it: if it mentions the name, you get a warning. The doc comments and
other text that mention the name are not changed. Renaming a package (its clause, or an import
path) is not supported yet; the references of a package clause are empty. When the declaring
package has an error that makes it unavailable to its importers, references and highlights
return what they know and say so in a warning.

*Speed.* On a warm server a request takes a few milliseconds (a 10,000-line package: references
p50 4 ms, rename p50 9 ms, measured at machine load 400 and more, so only an upper bound; see
`go test -bench 'References|Rename' ./internal/lsp/server`). Packages that were never opened cost
a check each the first time: a module of 40 packages, asked for a method (which looks in all of
them) on a busy machine, took about 0.6 s for references and 1.1 s for a rename, then 0.25-0.45 s
once they were checked (budget row: 300 ms for the warm case). The test files of a package are
loaded again from source when they mention the name (once for each package and change).

*Memory.* The server keeps the packages it has checked in a cache whose cap is 10% of the machine's
memory, at least 64 MB and at most 300 MB (an upper bound: it holds only what was looked at, and
the memory at start is the same). A request that looks at more packages than the cache holds (a
rename of a method that a large module shares) checks them again each time, which is slower and
allocates more, so a bigger cache can use less memory in total. Set it with `gocpp lsp
-check-cache MB`, or the setting `checkCacheMB` (`gocpp.checkCacheMB` in the editor's server
settings: `initializationOptions` or `workspace/didChangeConfiguration`); 0 is the default.

**C/C++ names** (`raylib.InitWindow`, `std.vector`, the members of C++ values, header imports)
are declared in headers, not in gocpp code: references and highlights show nothing for them,
and rename says that it does not rename them. The Go names around them (a variable that holds a
C value, a Go function that returns one) are ordinary names.

**Symbols.** The outline of a file (functions, methods, types with their fields, constants,
variables) comes from its syntax tree alone, so it works while the file has errors. Workspace
symbols search every `.gp` file of the modules of the workspace folders and of the open files
(for a folder without `gocpp.mod`, all the `.gp` files under it); the query matches the
characters of the name in order, ignoring case, and the best 100 come first.

**Semantic tokens.** The server colors the names by what they are: packages, types (structs and
interfaces too), type parameters, parameters, variables, constants (`readonly`), fields,
functions and methods, with the modifiers `declaration`, `defaultLibrary` (predeclared names and
the standard library) and `modification` (a variable that is assigned to). C/C++ names have
types of their own: `cppHeader` (the name of a header package, `raylib` in `raylib.InitWindow`),
`cppNamespace` (`std`, `fs`) and `cppName` (`InitWindow`, `vector`, `push_back`: a name after
one of those, or a member of a C/C++ value). The VS Code extension gives them colors; in other
editors, the highlight groups are named after the types (Neovim: `@lsp.type.cppHeader`,
`@lsp.type.cppNamespace`, `@lsp.type.cppName`), and a type without a group is left alone. Only
names are marked: keywords, strings and comments come from the editor's own highlighting. A
file that is not in its package (a `_test.gp` file, a file that the build leaves out) has no
semantic tokens.

**Inlay hints.** The name of the parameter before each argument of a call (`name:`, left out when
the argument says it already: `f(name)`, `f(x.name)`), and the type that `:=` and `var x = ...`
infer, after the name of the variable (also for the variables of `for i, v := range`). They are
of two kinds (parameter, type), so an editor can show one and not the other; a call of a C/C++
function and a variable that holds a C/C++ value get none, since their types are opaque to
gocpp. VS Code shows them with `editor.inlayHints.enabled`; Neovim 0.10 and later with
`vim.lsp.inlay_hint.enable()`. The hints of the `!` forms are in the next section.

**Document links.** A header import (`"<raylib.h>"`, `"./physics.h"`) is a link to the header
file, found the way the build finds it (the folder of the file for `"./x.h"`, the compiler's
include path for a system header; there is no link for a header that is not found, for example
when no C++ compiler is installed), and the addresses (`http://`, `https://`) in comments are
links too.

**Code actions.** *Organize Imports* (`source.organizeImports`), the quick fix *Add import*
for `undefined: strings`, which offers each package of the standard library and of the module
with that name, *Handle the error of f with !* and *Show expansion* (next section).

### `!`, macros and `#Derive`

The checker rewrites every `!`, macro use and `#Derive` line into plain Go, so the server shows
you what they become, and where the user's words are, instead of the compiler's code.

- **Hover on `F!(x)`** (on the name or on the `!`): the signature of `F`, what the `!` hands back
  (the values besides the error), *what happens when the error is not nil in that place* (the
  function returns it, a closure returns it through its own results, `t.Fatal` in a test, a
  panic in a function without an error result, exit 1 in `main`, and the rules of `defer F!()`),
  and the Go code that stands for the statement (names that start with `must_` belong to the
  compiler). A call that a must function handles without a `!` in the text has the same hover,
  and says that it is handled for you. Hover on a **must function** (`func f!(...) T`) shows the
  signature that callers see, `func f(...) (T, error)`, and the declaration as written.
- **Hover on a macro use** (`#util.Assert(x > 1, "small")`, also on top of a func): the macro as
  declared (not the function with hidden parameters), the arguments the compiler passes for
  `#file()`, `#line()`, `#func()` and `#text(x)`, a note for each `lazy` parameter, and the call
  that the use becomes. Hover on `#Derive` lists the methods it writes.
- **Inlay hints.** After a call that a `!` handles, where its error goes: `→ return err`,
  `→ panic`, `→ exit 1`, `→ t.Fatal`, `→ deferred: ...`; a call that a must function handles
  without a `!` in the text has a `!` in front of the arrow (`!→ return err`), the `!` that you
  did not write. Before the argument of a lazy parameter of a macro: `lazy msg:`. They have no
  kind, so they are on with the other hints. The tooltip says what happens.
- **Semantic tokens.** `macro` for the name of a macro (declaration and uses) and for the `#` of
  a use (`#Derive` too), `operator` for the `!` of a call and of a must function, `keyword` for
  `lazy`.
- **Completion** after a `#` (the server also asks for it with `#` as a trigger character):
  the built-in macros, the macros of the package and of dot imports, and the packages that
  declare macros; after `#util.` the macros of `util`. A macro inserts as a call with a
  placeholder for each parameter.
- **Definition, references, rename.** A macro is found by its name (`#util.Dbg` goes to the
  macro); a method that `#Derive` writes goes to the `#Derive` line. Rename of a must function
  changes the name and leaves every `!` as it is (`func load!(` and `load!(x)` become
  `func fetch!(` and `fetch!(x)`; a new name typed with a `!` is accepted for a must function and
  an error for another); rename of a macro keeps the `#`. Document symbols show `f!` and `#Name`.
- **Handle the error of f with `!`** (code action, kind `refactor.rewrite`): on the name of a call
  that drops its error, it puts a `!` after the name: `os.Remove(p)` becomes `os.Remove!(p)`, and
  `n := strconv.Atoi(s)`, which the checker rejects, becomes `n := strconv.Atoi!(s)`. It is offered
  where a `!` is accepted: the last result is exactly `error`, the call is a statement (also after
  `defer`) of a function with the one result, or the only value of an assignment or `var` that wants the
  others; it has no `!` yet, and no must function handles it already.
- **Show expansion** (code action, kind `refactor.inline`, offered on a declaration that has a
  `!`, a macro use or a `#Derive` in it; Ctrl+. in VS Code). It prints what `gocpp expand` prints
  for the file, from the text in the editor, and shows it in a document of its own
  (`name.gpexpand`, colored as gocpp, in a folder of its own for each source file inside a folder of the server's, private to you; the server removes it when it ends, and the next server removes the ones that a killed server left, when they are more than a day old),
  with the declaration you were in selected. It loads the package and what it imports from
  source and checks it again with the bodies of `#Derive` written, so it takes as long as
  `gocpp expand` (a second or two for a package that imports a few standard packages) and
  needs a package without errors (it says the first error otherwise). A client that cannot show
  documents gets the text in the command's result (`gocpp.showExpansion`, a
  `workspace/executeCommand`).

Not covered: a `!` inside `#text(...)` (that is the text of the expression, not a call), the files `_test.gp` (the server checks
a package without its test files, and loading them for each keystroke is not cheap: they get no hover, hints or tokens at all,
`!` or not), and "make `f` a must function". While the package has an error the checker does not lower the `!` forms: you still
get the hover text, the hints and the tokens (what happens on an error is then guessed from the signature of the function: a last
result `error`, a `*testing.T` parameter, `main`), but not the code that stands for the statement, and "Show expansion" says the
first error. A **syntax** error does not take the features of the rest of the file either: the server blanks the body of the function that the first
errors are in (the parser loses the rest of the file after an unclosed parenthesis; the signature stays, so callers still check;
a broken signature blanks the whole declaration), checks that text, and answers from it for everything outside the blanked
parts; inside them it answers from the check of the text as it is. While you type, the answers come from the last such check
and are moved across the keystrokes since (the characters just typed have no hover, token or hint until the next check, which
takes a few tens of milliseconds in a big package; a client that supports it is then asked to refresh the tokens and the hints).
`GOCPP_LSP_NOSTALE=1` turns that off. (An error in the package clause or the imports cannot be repaired this way.) References and
rename do not use the repaired text: a rename that did not see the blanked parts would be wrong.

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

## Windows

Installing gocpp, the C++ compiler and the paths gocpp writes are on [gocpp on Windows](windows.md#editing-on-windows).
`gocpp.exe lsp` is the same server, and every editor above starts it the same way (the command is
`gocpp.exe`, or `gocpp` when the folder of the binary is in `PATH`). What differs is how names of
files are written and compared, and the server takes care of it ([ADR-096](../adr/096-language-server-on-windows-one-spelling-per-file.md)):

- **URIs.** `file:///c:/Users/Me/p.gp`, VS Code's `file:///c%3A/Users/Me/p.gp`, `file://C:/Users/Me/p.gp`
  (two slashes, which some clients send), `file://localhost/C:/...` and a UNC name
  (`file://server/share/p.gp`, also `file:////server/share/p.gp`) all name the file they say. Names with
  spaces and characters outside the ANSI code page (`Wörk Dir 日本語`) work: the server reads the files with
  the wide API, and hands the C++ compiler, which does not, a short name (this is [gocpp
  build's rule](../adr/081-windows-host-build-driver.md) too). The server answers with an upper-case drive letter
  (`file:///C:/...`); clients treat drive letters as case-insensitive.
- **One spelling per file.** Windows compares names without case, and editors spell a folder as the user typed
  it (`c:\users\me\proj`) while the disk says `C:\Users\Me\Proj`. The server gives every name from the editor
  the spelling that the disk has (drive letter in upper case, each folder and file in its stored case, 8.3 short names such as
  `ADMINI~1` written out) before it uses it as a key, so the packages, open documents, diagnostics and caches
  of two spellings are the same. The answer about a document that is open uses the editor's own URI for it
  (the editor knows the document by that), every other file the disk's spelling. A file that is created later
  with another case than the server first saw is found when the watcher reports it (below).
  Not handled: two names of one folder through a symbolic link or a junction (two folders here, as in gopls), the
  administrative share `\\localhost\c$` (it is another name of `C:\`, so a file opened through it has its own
  package), a path with a trailing dot or space, a case-sensitive folder on a Windows file system (a WSL share
  `\\wsl$\...` or a folder with `fsutil file setCaseSensitiveInfo`): two names that differ in case only are one name to the
  server there.
- **Line ends and columns.** Sources with `\r\n` work everywhere: a position counts UTF-16 units of the
  line without its line end, an edit that the server answers (rename, organize imports) uses the line end of the file, and
  format keeps `\r\n` (the file is not rewritten with `\n`). A UTF-8 byte order mark at the start of a file on
  disk (Notepad writes one) is not counted: the editor shows the file without it, and the server's columns on
  the first line must agree.
- **File watching.** The server asks the editor to watch `**/*.gp`, `**/gocpp.mod` and the headers
  (`workspace/didChangeWatchedFiles`, like gopls); it has no watcher of its own, so with an editor that does not
  watch files (Helix, Zed, eglot) does not see a change on disk of a file that is not open (a `git checkout`) until a watcher event or a
  restart: a file is read again only when an event names it, as in gopls. An event may name the file in any
  of the spellings above, and a created, deleted or renamed file or folder makes the server forget what it knew of
  names (the case and the listings behind it) before it looks at the event. A file that another program holds so that it cannot be read (a lock,
  no sharing) for more than half a second is read again in the background (the log and `window/logMessage` say so once) and the packages are
  diagnosed when it can be read.
- **C++ compiler and cache.** The server needs the same MinGW-w64 as `gocpp build` for the header imports (`gocpp env`
  shows what it found; without a compiler each header import gets the error "cannot check the header imports", and the rest works), and it runs the
  compiler with the environment of the build (`TEMP` and `LOCALAPPDATA` replaced by names the compiler can read when
  the user's own cannot be). The cache of summaries is in `%LocalAppData%\gocpp\lsp` (`GOCPP_LSP_CACHE`).
- **Speed.** The budgets of the language server are measured on Windows with the same harness as on Linux
  (`GOCPP_WINDOWS_LSPBENCH=1 GOCPP_WINDOWS_SSH=user@host go test -run TestWindowsLSPBench ./tests`: tools/lspbench
  runs on the PC); the figures are in [the benchmarks](../benchmarks/lsp-windows.md). The clock of a Windows process ticks every
  half millisecond or so, so a figure under a millisecond is a multiple of that.

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

## When the server is slow: the slow log, traces and profiles

The server is meant to be as fast as gopls, with written budgets (about 30 ms of waiting after
the last keystroke, then the check; hover under 30 ms; completion under 50 ms; the table is in
[docs/benchmarks/lsp.md](../benchmarks/lsp.md)). When it is not, these tools say where the time
goes. All of them are flags of `gocpp lsp`, so in an editor they go into the command, for
example `{"gocpp", "lsp", "-slow", "200ms", "-logfile", "/tmp/gocpp-lsp.log"}`.

**The slow log.** A request, a notification or a diagnostics run that takes `-slow` (500 ms
by default; a negative value turns the log off) or longer is written to the log (standard error,
or `-logfile`) with what it was about and where its time went:

```
slow: textDocument/completion took 612ms (over 500ms) uri=file:///w/pkg/a.gp snapshot=41 (now: snapshot=41 live=1 overlays=3 checks=17); phases: package 598ms, check 597ms, typecheck 520ms, load 61ms, imports 9ms, export 4ms
```

The phases are the time spent in the steps of the work, summed by name: `package` (getting the
checked package of the file: from the cache, or by checking it), `check` (one package check)
with its parts `queue` (waiting for a free core), `imports` (reading the export data of the
imports), `load` (reading and parsing the package's files), `typecheck`, `export` (writing the
export data for the importers) and `apihash`; `parse` (parsing one file for a feature), `disk-read`
(a package summary from the disk cache). Phases of different names can overlap (`check` holds
`typecheck`). A request that waits for a check that another piece of work started shows `package`
without the parts: they are in the line of the work that did the check. A line of the log is
what to put in a bug report, with the output of `gocpp version`.

**`gocpp/stats`.** The request `gocpp/stats` (also the command `gocpp.stats` of
`workspace/executeCommand`, which every client can send) answers with JSON: the snapshots (the
current one, how many are alive, how many documents are open), the work of the pipeline (files
parsed, packages checked, summaries read from the disk cache), each cache's entries, estimated
memory, hits, misses and hit rate (`parse`, `check`, `summary`), the count, p50, p95 and maximum
of the last 256 of each kind of request (and of the diagnostics runs), and the memory of the
process. In Neovim: `:lua vim.print(vim.lsp.get_clients()[1]:request_sync("gocpp/stats", nil, 5000).result)`.

**Chrome trace.** `gocpp lsp -trace spans.json` writes every piece of work and its steps as a
Chrome trace when the server ends (or is stopped with SIGINT or SIGTERM: then the process exits
at once, so the summaries being written to the disk cache are lost, which is harmless). The server
keeps up to a million spans in memory (about 150 MB) until then, so use it for a short session. Open it in
`chrome://tracing`, [Perfetto](https://ui.perfetto.dev) or speedscope: each request, each
notification, each diagnostics run and each namespace probe is a row, with its steps below.

**Profiles.** `gocpp lsp -profile dir` writes `cpu.pprof` and `heap.pprof` to the folder `dir`
when the server ends (the heap profile is taken after a garbage collection: it shows what is
alive), and `-exec-trace` adds the Go execution trace, `exec.trace`. Read them with
`go tool pprof -http=: dir/cpu.pprof` and `go tool trace dir/exec.trace`. Stop the editor (or
send SIGTERM to the server) after the slow action, so that the profile has it.

**Benchmarks.** `make lsp-bench` replays scripted editor sessions against generated reference
projects, for gocpp and for gopls on the same code, and writes the table; `make lsp-budgets` checks
the speed budgets (a Go test that is not part of `make ci`, because it takes a minute and a half
and needs a quiet machine). `go run ./tools/lspbench -h` lists the flags (`-only hover -scale 20
-gocpp-args "-profile /tmp/p"` profiles one scenario).

## Debugging

gdb (and, with fewer features, LLDB) shows gocpp values as Go values and stops at `.gp` lines; `gocpp debug`
starts it. VS Code runs it through the C/C++ extension: breakpoints in `.gp` files come with the gocpp
extension, and [debugging.md](debugging.md) has the `launch.json` and `tasks.json` to copy.
