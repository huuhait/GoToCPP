# The gocpp grammar

`gocpp.tmLanguage.json` is the TextMate grammar for `.gp` files. Its scope is `source.gocpp`.

## Where it comes from

It is derived from the Go grammar of go-syntax, by Furkan Ozalp:
`syntaxes/go.tmLanguage.json` in <https://github.com/worlpaker/go-syntax>, at commit
`6e8421faf8f1445512825f63925e54a62106bcf1`. VS Code 1.109 ships this commit as its own Go
grammar (`extensions/go/syntaxes/go.tmLanguage.json`).

go-syntax has the MIT license. Its license text is in [LICENSE-go-syntax](LICENSE-go-syntax),
copied from the same commit. The gocpp changes have gocpp's license (BSD-3-Clause, the
[LICENSE](../../../LICENSE) file in the root of the repository).

## How it was made

The file is a copy of the upstream file with the changes below written into it. It keeps the
upstream formatting (JSON with tabs, no newline at the end), so a plain `diff` shows every
change. Run this in the root of the repository (it leaves no file behind):

```sh
curl -sSL https://raw.githubusercontent.com/worlpaker/go-syntax/6e8421faf8f1445512825f63925e54a62106bcf1/syntaxes/go.tmLanguage.json |
	diff -u - editors/vscode/syntaxes/gocpp.tmLanguage.json
```

## What gocpp changed

New rules, at the end of the `repository`. Each has a `comment` that starts with `gocpp:`.

- `cpp_imports`: header imports (`"<raylib.h>"`, `"./physics.h"`, `rl "<raylib.h>"`) and
  namespace imports (`fs "::std::filesystem"`). `imports` tries it first.
- `macro_declaration`: `macro Name(args) T {` and `macro Name[T any](args) T {`. It works like
  `function_declaration`: only at the start of a line (package level), so `macro` is a normal
  name inside a function. It is in `group-functions`, after `function_declaration`.
- `macro_param_types`: the parameters of a macro. Only there, `lazy` is a modifier
  (`msg lazy string`).
- `macro_uses`: `#Name(...)`, `#pkg.Name(...)` and the built-in macros. It is the first rule of
  `type-declarations` and of `type-declarations-without-brackets`, the lists of Go's own
  tokens, so a macro use is highlighted wherever they are. That includes an index before a
  call, `handlers[#Dbg(i)](x)`, which upstream reads as a call with type arguments.

Changed rules:

- `function_declaration`, begin: `func name!(` and `func (s *S) Name!(` are must functions.
  The `!` is the new capture 4.
- `function_declaration`, end: it also ends before a `{` that comes after a space. Upstream
  needs a `)` or a simple result type before the `{`. After a result type like `func()`
  (`func f() func() {`), `iter.Seq2[int, T]` or `interface{ M() }`, the rule never ended, and
  everything after such a function was highlighted as part of its declaration. This is a fix
  for plain Go too. `macro_declaration` has the same end.
- `support_functions`: `name!(` and `name![T](` are must calls. The `!` is the new capture 3,
  and the old capture 3 (the type arguments) is now capture 4. After a Go keyword, `!` stays
  "not" (`if!(x)` is `if !(x)`).
- A Go keyword or a predeclared name right after a single `.` is a name: `std.map`,
  `node.type()`, `posix.select(...)`, `x.range`, `s.append(...)`, `p.string()`, `C.int`. In
  these rules, the word now has `(?<!(?<!\.)\.)` ("not right after one dot") before it: `keywords`,
  `storage_types`, `language_constants`, `built_in_functions` (also `new` and `make`),
  `map_types`, `functions`, `functions_inline`, `struct_variables_types`,
  `interface_variables_types`, `multi_types`, `package_name` and `import`. After `...`
  (`...func()`, `...string`) the words stay Go keywords and types. The type switch
  `x.(type)` does not change, because there the `.` is followed by `(`.
- The header: the name is `gocpp`, the scope is `source.gocpp`, and
  `information_for_contributors` and `version` say where the file comes from, as in VS Code's
  copy.

All new scopes end in `.gocpp`. The list is in [../README.md](../README.md#what-it-highlights).

## Known limits

- A keyword after a dot with a space or a comment in between (`x. type()`, `x./*c*/type()`)
  is still highlighted as a keyword. The gocpp parser reads it as a name (Phase 01, patch 1).
  gofmt removes the space, so this only shows in code that is not formatted.
- A must call in an index before a call (`handlers[F!(i)](x)`) is not highlighted as a must
  call: upstream reads `x[...](` as a call with type arguments, so `F` gets the type scope and
  `!` the "not" scope.

## Moving to a newer go-syntax

1. Download the new upstream file and the old one (the commit above) into a temporary folder.
2. Apply the upstream changes (old → new) to `gocpp.tmLanguage.json`, or apply the gocpp
   changes (the `diff` above) to the new file.
3. Run `make vscode-test`.
4. Change the commit here, in `version` in the grammar, and in `LICENSE-go-syntax` if the
   license changed.

Newer go-syntax commits fix slow regular expressions in the struct rules (`c74e22eb`,
`4a9b120a`, 2026). The upstream declaration bug above is still there in `0e873ee9`
(September 2026).
