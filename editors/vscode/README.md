# editors/vscode

The VS Code extension for gocpp. It adds the `gocpp` language for `.gp` files:

- syntax highlighting, with a TextMate grammar (`syntaxes/gocpp.tmLanguage.json`);
- the editor settings for comments, brackets, auto-closing pairs and indentation
  (`language-configuration.json`: VS Code's settings for Go, plus a word pattern);
- tabs for indentation in `.gp` files, as `gocpp fmt` writes them.

The extension has no code. It needs no build step and starts nothing. The language server
client, format on save and `contributes.breakpoints` come later (Phase 11 step 11.10, and
Phase 13 for debugging).

## Try it

Start VS Code with the extension loaded, from the root of the repository:

```sh
code --extensionDevelopmentPath=$PWD/editors/vscode examples/macros/main.gp
```

This opens a new window ("Extension Development Host") with the extension. Or install it with
a link, then restart VS Code:

```sh
ln -sfn $PWD/editors/vscode ~/.vscode/extensions/gocpp
```

With `-sfn`, running it again replaces the link. (A plain `ln -s` would put a second link
inside `editors/vscode`.) To remove the extension, delete the link. Both commands work in bash,
zsh and fish.

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

The grammar is derived from an MIT-licensed Go grammar.
[syntaxes/README.md](syntaxes/README.md) says where it comes from and what gocpp changed.

## Tests

```sh
make vscode-test
```

The first time, this runs `npm ci`: it downloads the test tools into
`editors/vscode/node_modules`. Then it runs `npm test`, which has two parts:

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

Both use the same tokenizer as VS Code 1.109: vscode-textmate 9.3.2 and vscode-oniguruma
1.7.0. The versions are pinned in `package.json` and `package-lock.json`. Node.js and npm come
from the dev shell (`nix develop`).

`make vscode-test` is not part of `make ci`, because it downloads packages the first time,
like `make staticcheck`. It runs `npm ci` again only when `package-lock.json` changes.

## gnuplot

`.gp` is also the file extension of gnuplot. If a gnuplot extension takes your `.gp` files,
choose the language in your settings with `files.associations`:

```json
"files.associations": {
	"*.gp": "gocpp"
}
```
