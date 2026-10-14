# Code copied or adapted from Go and gopls

Most of this folder is gocpp's own code. These parts come from the Go project, whose license
(BSD-3-Clause, the same as gocpp's) is in `../protocol/LICENSE` (gopls's) and in Go's own
`LICENSE` file; it is the same license text.

| Here | From | How |
|---|---|---|
| `builtin.go.txt` | Go 1.25.14, `src/builtin/builtin.go` | copied as it is (its copyright header stays): the docs of the predeclared names, for hover and signature help (`builtin.go`) |
| `hover.go`: `hoverResult`, `hoverResult.format`, `objectHover`, `through`, `packageHover`, `docMarkdown` | gopls v0.21.1, `internal/golang/hover.go` (`hoverResult`, `formatHover`, `hover`, `hoverPackageName`) and `comment.go` (`DocCommentToMarkdown`) | rewritten for gocpp's snapshots, after gopls's code: the same sections and text, without the links to pkg.go.dev, the size and offset notes and the promoted-field table |
| `navigation.go`: `objectString`, `constString`, `declaringNames`, `docOf`, `preferMarkdown` | gopls v0.21.1, `internal/golang/hover.go` (`objectString`, `findDeclInfo`, `chooseDocComment`) and `internal/settings` (the preferred content format) | rewritten after gopls's code |
| `builtin.go`: `builtinHover`, `builtinSignature`, `builtinTypes` | gopls v0.21.1, `internal/golang/hover.go` (`hoverBuiltin`) and `types_format.go` (`NewBuiltinSignature`, `replacer`) | rewritten after gopls's code |
| `signature.go`: `signature`, `activeParameter`, `signatureInfo.label` | gopls v0.21.1, `internal/golang/signature_help.go` and `types_format.go` (`signature.Format`) | rewritten after gopls's code |
| `completer.go`: the scores (`stdScore`, `highScore`, `unimportedScore`), `rank`, `methodsAndFields`, `eachField`, `lexical`, `structLit`, `structFields`, `expectedType` (its cheap cases), `keywords`, `hasDefault` | gopls v0.21.1, `internal/golang/completion/completion.go` (`lexical`, `selector`, `methodsAndFields`, `enclosingCompositeLiteral`, `structLiteralFieldName`, `expectedCandidate`, `matchingCandidate`), `deep_completion.go` (`addCandidate`), `keywords.go` and `util.go` (`eachField`) | rewritten after gopls's code, without deep completion, postfix snippets and literal candidates |
| `completion.go`: `fixDangling`; `autoimport.go`: `importEdits` (adds the import, then organizes the imports and diffs, as gopls's `ComputeImportFixEdits` does with goimports) | gopls v0.21.1, `internal/cache/parsego/parse.go` (`fixDanglingSelector`, `isPhantomUnderscore`) and `internal/golang/format.go` (`ComputeImportFixEdits`) | rewritten after gopls's code: the `_` takes the place of a space or tab, so the offsets of the text stay; parsed and fixed again up to 10 times, like gopls's loop in `Parse` |
| `completion.go`: `sortCandidates`, the items' fields (sortText, filterText, preselect, insert/replace ranges), `format`, `typeDetail`, `callSnippet`, `fieldSnippet`, `needsTypeArgs`, `snippetBuilder` | gopls v0.21.1, `internal/golang/completion/completion.go` (`sortItems`), `format.go` (`item`, `formatBuiltin`, `inferableTypeParams`), `snippet.go` (`functionCallSnippet`, `structFieldSnippet`), `snippet/snippet_builder.go`, `internal/golang/types_format.go` (`FormatType`) and `internal/server/completion.go` (`toProtocolCompletionItems`) | rewritten after gopls's code |

The tests compare the hover texts with what gopls prints for the same code as `.go`, except for
the "// Embedded fields:" table of a struct type, which gopls adds and gocpp doesn't, and for two
differences in generic functions, which wait for Phase 07 (generics): `typeParamsString` prints
each type parameter with its constraint (`[T any, U any]`, where go/types and gopls print
`[T, U any]`), and at a call site gopls shows the signature with the inferred type arguments,
gocpp the generic one.
