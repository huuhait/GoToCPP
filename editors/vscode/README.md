# editors/vscode

The VS Code extension for gocpp. It adds the `gocpp` language for `.gp` files:

- syntax highlighting, with a TextMate grammar (`syntaxes/gocpp.tmLanguage.json`);
- the editor settings for comments, brackets, auto-closing pairs and indentation
  (`language-configuration.json`: VS Code's settings for Go, plus a word pattern);
- the gocpp language server: it starts `gocpp lsp` when you open a `.gp` file (with
  [vscode-languageclient](https://github.com/microsoft/vscode-languageserver-node)). Today the
  server shows the errors of `gocpp check` as you type, and has hover, go to definition and declaration,
  signature help, completion, formatting and organize imports, references, rename, symbols,
  semantic highlighting, inlay hints, links to header files, a quick fix for a missing
  import, and the features for `!`, macros and `#Derive` (the list is in [docs/guides/editors.md](../../docs/guides/editors.md)); the features
  of later versions of the server need no change in the extension;
- format on save and organize imports on save, and tabs for indentation, in `.gp` files (the
  same defaults as the Go extension);
- breakpoints in `.gp` files, for debugging: start the program with the C/C++ extension (cppdbg and gdb) or
  CodeLLDB or lldb-dap; `gocpp debug -vscode` prints the `launch.json` (for Linux and Windows, `-os windows`), and the files to copy, with gocpp's printers, are in
  [docs/guides/debugging.md](../../docs/guides/debugging.md#vs-code).

The extension is plain JavaScript (`src/`), so it has no build step. The other editors are in
[docs/guides/editors.md](../../docs/guides/editors.md).

## Install from source

You need Node.js and npm (the dev shell, `nix develop`, has them) and the `gocpp` binary. In
the root of the repository:

```sh
make build                          # writes bin/gocpp
cd editors/vscode && npm ci         # the extension's packages, in node_modules
```

The extension needs `node_modules` (vscode-languageclient) at run time, so run `npm ci` again
after a `git pull` that changes `package-lock.json`. Then load the extension in one of these
ways:

- **Try it** in a new window ("Extension Development Host"), from the root of the repository:

  ```sh
  code --extensionDevelopmentPath=$PWD/editors/vscode examples/macros/main.gp
  ```

- **Install it** with a link, then restart VS Code:

  ```sh
  ln -sfn $PWD/editors/vscode ~/.vscode/extensions/gocpp
  ```

  With `-sfn`, running it again replaces the link. (A plain `ln -s` would put a second link
  inside `editors/vscode`.) To remove the extension, delete the link. Both commands work in
  bash, zsh and fish.

- Or run **Developer: Install Extension from Location...** from the Command Palette and pick
  the `editors/vscode` folder.

`code --install-extension` takes only a Marketplace id or a `.vsix` file, not a folder. There is
no `.vsix` of the extension yet.

If `gocpp` is not in the `PATH` of VS Code, set `gocpp.path` to its full path (for example
`/home/me/gocpp/bin/gocpp`). VS Code started from a desktop menu may have a shorter `PATH` than
your shell.

## Settings

| Setting | Default | What it does |
|---|---|---|
| `gocpp.path` | `"gocpp"` | The gocpp binary that runs the server. A name is looked up in `PATH`. A path can start with `~` (your home folder); a relative path is relative to the first workspace folder. On Windows, `.exe` can be left out, the value may be the folder of `gocpp.exe`, a UNC name, a path with spaces (no quoting needed; quotes around the whole path are taken off, as are quotes around an entry of `PATH`), and `/` or `\\` both work. A `.bat` or `.cmd` wrapper does not work: VS Code starts the binary without a shell, and Node.js refuses to run those without one. |
| `gocpp.lsp.args` | `[]` | More arguments for the server, after `gocpp lsp`. For example `["-logfile", "/tmp/gocpp-lsp.log"]` writes the server log to that file. `gocpp lsp -h` lists the flags. |
| `gocpp.lsp.env` | `{}` | Environment variables for the server, on top of the environment of VS Code, for example `{ "PATH": "/opt/gocpp/bin:/usr/bin" }`. A `PATH` here is also used to find `gocpp.path`. |
| `gocpp.trace.server` | `"off"` | `"messages"` or `"verbose"` writes the messages between VS Code and the server to the **gocpp** output channel. The change takes effect at once. |

After a change of `gocpp.path`, `gocpp.lsp.args` or `gocpp.lsp.env`, the extension asks to
restart the server. The command **gocpp: Restart Language Server** (in the Command Palette)
restarts it at any time.

**C and C++ names.** The interop mode is chosen by `-interop=typed|opaque|auto` or the variable `GOCPP_INTEROP`
(`typed` asks clang++ for the types of the C/C++ names and needs it; `auto`, the default, uses it when it is
installed; `opaque` leaves everything to the C++ compiler). A build or `gocpp check` task you start from VS Code takes
the variable from its `env`. The language server reads `GOCPP_INTEROP` from `gocpp.lsp.env` and runs its C/C++
completion and hover index in the background (a missing clang++ leaves the other answers as they are); set
`GOCPP_INTEROP=opaque` there to turn the background clang runs off. The extension needs no new setting.

The extension sets these defaults for `.gp` files, as the Go extension does for `.go` files:

```json
"[gocpp]": {
	"editor.insertSpaces": false,
	"editor.formatOnSave": true,
	"editor.formatOnSaveMode": "file",
	"editor.codeActionsOnSave": { "source.organizeImports": "explicit" }
}
```

You can change them in your own settings under `"[gocpp]"`. Format on save does something once
the server can format (`gocpp fmt`); until then, saving a file does not change it.

In Restricted Mode (a folder that you have not trusted yet), the server does not start, since
it runs the C++ compiler on the headers that the project imports. Highlighting still works.
When you trust the folder, the server starts. In Restricted Mode, the three settings above that
start the server are read from your user settings only, not from the folder's settings.

## Troubleshooting

- **The server log.** Open the Output panel (**View: Toggle Output**, or `Ctrl+Shift+U`) and
  pick **gocpp** in the list on its right. It shows how the extension starts the server, the
  server's own log (its standard error), and with `gocpp.trace.server` the messages.
- **"Cannot start the gocpp language server: "gocpp" is not in PATH."** Install gocpp
  (`make build` writes `bin/gocpp`) and set `gocpp.path` to the full path of the binary, or add
  its folder to `PATH` (or to `PATH` in `gocpp.lsp.env`). The **Open Settings** button of the
  message opens the setting.
- **`.gp` files open as gnuplot** (or as plain text). `.gp` is also the file extension of
  gnuplot. If a gnuplot extension takes your `.gp` files, pick the language in your settings
  with `files.associations`:

  ```json
  "files.associations": {
  	"*.gp": "gocpp"
  }
  ```

  The language of the open file is shown on the right of the status bar; click it to change it
  for that file.
- **Nothing happens on save.** The server cannot format yet (see above), or another formatter
  is set for gocpp in `editor.defaultFormatter`.

## What it highlights

All of Go (the grammar starts from the Go grammar that VS Code itself uses), plus the gocpp
syntax:

| gocpp syntax | Example | Scopes (for color settings) |
|---|---|---|
| header imports | `"<raylib.h>"`, `"<SDL2/SDL.h>"`, `"./physics.h"` | `meta.preprocessor.include.gocpp`, `entity.name.import.header.gocpp` |
| namespace imports | `fs "::std::filesystem"` | `entity.name.namespace.gocpp`, `punctuation.separator.namespace.gocpp` |
| must calls | `os.ReadFile!(path)`, `parse![int](s)` | the name stays a function call, the `!` is `keyword.operator.must.gocpp` |
| must functions | `func loadConfig!(path string) Config`, `func (s *Server) Start!()` | the name stays a function name, the `!` is `keyword.operator.must.gocpp` |
| macro uses | `#Assert(n > 0, "msg")`, `#util.Dbg(x)`, `#Trace()` on top of a func | `entity.name.function.macro.gocpp` (the `#` too) |
| built-in macros | `#body()`, `#file()`, `#line()`, `#func()`, `#text(x)`, `#embed("f")`, `#Derive(String)` | `entity.name.function.macro.builtin.gocpp` |
| macro declarations | `macro Dbg[T any](v T) T {` | `keyword.macro.gocpp`, `entity.name.function.macro.gocpp` |
| lazy parameters | `macro Assert(cond bool, msg lazy string)` | `storage.modifier.lazy.gocpp` |
| Go keywords after a dot | `std.map[std.string, int]{}`, `node.type()`, `posix.select(...)` | a name, not a keyword |
| C++ types and C helpers | `std.array[int, 5]`, `var d std.chrono.seconds`, `std.vector[int].iterator`, `C.int`, `C.cstr(s)` | like Go types and calls |

Some details:

- `macro` is a keyword only at the start of a package-level declaration (`macro Name(`).
  `lazy` is a keyword only in the parameters of a macro. Everywhere else they are normal
  names, as the language says (D18 in [language.md](../../docs/spec/language.md)).
- After a Go keyword, `!` means "not", as in Go: in `if!(a && b)`, the `!` is not a must.
- A name after a dot is always a name: `s.append(x)`, `std.max(a, b)` and `p.string()` are
  method and function calls, not Go's built-in `append`, `max` and `string`.
- `#`, `!` and `!=` inside strings and comments are not changed.
- Double-click on `#Dbg` or `ReadFile!` selects only the name (`Dbg`, `ReadFile`): `#` and `!`
  are not part of a word.

The colors come from your color theme. In the default themes (Dark Modern, Light Modern),
header paths are blue, namespaces have the type color, and Go import paths have the string
color. Some themes have no color for `meta.preprocessor`; then a header path looks like a
Go import path. You can give any scope your own color in the settings, for example the `!`
of must:

```json
"editor.tokenColorCustomizations": {
	"textMateRules": [
		{ "scope": "keyword.operator.must.gocpp", "settings": { "foreground": "#E06C75" } }
	]
}
```

**Semantic highlighting.** On top of the grammar, the server colors names by what they are
(packages, types, parameters, fields, functions, ...). For C/C++ names it has three token
types, which the extension declares (`semanticTokenTypes`) and maps to scopes
(`semanticTokenScopes`) so that the theme colors them: `cppHeader` (the name of a header
package, `raylib`) is `entity.name.type.module.header.gocpp`, `cppNamespace` (`std`, `fs`) is
`entity.name.namespace.cpp.gocpp`, and `cppName` (`InitWindow`, `vector`) is
`support.function.cpp.gocpp`. Give them your own colors like any scope:

```json
"editor.semanticTokenColorCustomizations": {
	"[*]": { "rules": { "cppHeader": "#4EC9B0", "cppName": { "foreground": "#DCDCAA", "italic": true } } }
}
```

Inlay hints (the names of parameters, the types that `:=` infers) show when
`editor.inlayHints.enabled` is `on` (or `onUnlessPressed`).

The grammar is derived from an MIT-licensed Go grammar.
[syntaxes/README.md](syntaxes/README.md) says where it comes from and what gocpp changed.

## Tests

```sh
make vscode-test
```

The first time, this runs `npm ci`: it downloads the extension's packages and the test tools
into `editors/vscode/node_modules`. Then it runs `npm test`, which has three parts:

1. **Assertion tests** with [vscode-tmgrammar-test](https://github.com/PanAeon/vscode-tmgrammar-test):
   the files in `test/testdata/` are gocpp code with assertion lines. A line like
   `//    ^^^^ entity.name.function.go` says that the characters above the `^` marks must
   have this scope. With a `-` before the scopes, they must not have them.
2. **The examples check**, `test/check-examples.mjs`: it highlights every `.gp` file in
   `examples/`. It fails when a token gets an `invalid.*` scope, when a rule stays open
   after the end of a top-level declaration (then the rest of the file would be highlighted
   wrongly), or when a gocpp form (`#`, `name!(`, a keyword after a dot, a header import,
   `macro Name(`, a `lazy` parameter) is not highlighted as gocpp. It also fails when `macro`
   or `lazy` is highlighted as a keyword in a place where it is an ordinary name.
3. **The tests of the extension's code**, `test/*.test.js`, with Node.js's test runner
   (`node --test`). They need no VS Code:
   - `server.test.js`: how the settings become the command that starts the server (the
     lookup in `PATH`, `~`, Windows names, the environment) and the error messages;
   - `extension.test.js`: `src/extension.js` with a fake `vscode` module and a fake client:
     the client it starts, the restart command, the "not found" error, Restricted Mode;
   - `lsp.test.js`: builds `gocpp` from the repository (`go build`, so it needs Go: run it in
     the dev shell), takes the server options that `src/extension.js` gives the client, and
     starts `gocpp lsp` with the real vscode-languageclient code that spawns it, so it gets
     the same command line as in VS Code. Then it does what VS Code does over stdio:
     initialize, open a file, wait for its diagnostics, shutdown, exit. Set `GOCPP_TEST_BIN`
     to test a gocpp binary that is already built;
   - `manifest.test.js`: `package.json` against what VS Code needs and what the code and this
     README say (the main file, the activation event, the commands and settings, format on
     save, breakpoints, pinned versions).

The grammar tests use the same tokenizer as VS Code 1.109: vscode-textmate 9.3.2 and
vscode-oniguruma 1.7.0. All the versions are pinned in `package.json` and
`package-lock.json`. Node.js and npm come from the dev shell (`nix develop`).

`make vscode-test` is not part of `make ci`, because it downloads packages the first time,
like `make staticcheck`. It runs `npm ci` again only when `package-lock.json` changes.
