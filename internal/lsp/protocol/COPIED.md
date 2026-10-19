# The copied gopls protocol code

This folder holds the LSP 3.17 types of `gocpp lsp`. They are not written by hand: they are
gopls's generated protocol code, copied together with the generator that writes them from the
LSP's official `metaModel.json`
([Phase 11](../../../plans/2026-10-13-gocpp/phase-11-language-server/README.md) §2.2 item 9).
gopls's `internal/protocol` can't be imported (it is internal), and the public Go LSP modules lag
behind LSP 3.17. The rest of the folder is gocpp's own code (§4).

| File | What it is |
|---|---|
| `tsprotocol.go`, `tsjson.go` | the generated code: every LSP type and constant, and the JSON code of the union ("or") types. Copied (§1) |
| `uri.go`, `form.go`, `tsdocument_changes.go`, `tsinsertreplaceedit.go` | gopls's hand-written code that the generated code needs: `DocumentURI` and `URI`, `FormField`, `DocumentChange`, `InsertReplaceEdit`'s JSON. Copied; `uri.go` was changed in Phase 14 step 14.5 (ADR-096): it calls `internal/lsp/fspath` for the names of the system (Windows drive letters, UNC, case), has `Key`, and `urimemo.go` is new |
| `uri_test.go`, `uri_windows_test.go` | gopls's tests of `uri.go`. Copied |
| `internal/pathutil/` | gopls's `internal/util/pathutil`, which `uri.go` imports. Copied |
| `generate/` | gopls's generator, with its README. Copied |
| `generate/staticcheck.conf` | turns off two staticcheck checks for the generator (§5) |
| `LICENSE` | gopls's license (§1) |
| `update.sh` | the script that copies the files again (§6) |
| `doc.go`, `jsonrpc.go`, `stream.go`, `conn.go`, `position.go` and their tests | gocpp's code (§4) |
| `COPIED.md` | this file |

## 1. Upstream

The copy is from **gopls v0.21.1** (the module `golang.org/x/tools/gopls`, git commit
`5c4433be420451410e8cfd968eda32a818dac087` of <https://go.googlesource.com/tools>). Its
generated code comes from `protocol/metaModel.json` of
<https://github.com/microsoft/vscode-languageserver-node> at the tag
`release/protocol/3.17.6-next.14` (commit `66a087310eea0d60495ba3578d78f70409c403d9`), whose
`metaData.version` is **3.17.0**. The header of each generated file says so.

| Upstream, in the gopls module | Here |
|---|---|
| `internal/protocol/tsprotocol.go`, `tsjson.go` | `tsprotocol.go`, `tsjson.go` |
| `internal/protocol/uri.go`, `form.go`, `tsdocument_changes.go`, `tsinsertreplaceedit.go` | the same names |
| `internal/protocol/uri_test.go`, `uri_windows_test.go` | the same names |
| `internal/protocol/generate/*` (7 Go files and README.md) | `generate/` |
| `internal/util/pathutil/util.go` | `internal/pathutil/util.go` |
| `LICENSE` | `LICENSE` |

- `LICENSE` is the license of the gopls module: Go's BSD-3-Clause license. Every copied file
  says that its license "can be found in the LICENSE file". The copyright headers of the copied
  files stay as they are.
- The copied generator, run on the same `metaModel.json`, writes `tsprotocol.go` and
  `tsjson.go` byte for byte as they are here (checked on 2026-10-14 with `update.sh -gen`).
- The generated names are gopls's: for example `ParamInitialize` (the params of `initialize`),
  `TextDocumentContentChangeEvent` (an alias of `TextDocumentContentChangePartial`), and
  `Or_…` structs for the unions that gopls keeps. `generate/README.md` explains them.

## 2. The import paths

`update.sh` changes two import paths, only on the import lines of three files:

| Upstream | Here | In |
|---|---|---|
| `"golang.org/x/tools/gopls/internal/util/pathutil"` | `"github.com/huuhait/gocpp/internal/lsp/protocol/internal/pathutil"` | `uri.go` |
| `"golang.org/x/tools/gopls/internal/protocol"` | `"github.com/huuhait/gocpp/internal/lsp/protocol"` | `uri_test.go`, `uri_windows_test.go` |

Nothing else in the copied files changes. After the copy, `update.sh` checks that no package
here imports anything of `golang.org/x/tools`.

## 3. What is not copied, and why

| Upstream file | Why not |
|---|---|
| `tsclient.go`, `tsserver.go` (generated) | the `Client` and `Server` interfaces and their dispatch need gopls's `internal/jsonrpc2` and its event packages. gocpp dispatches with its own small `conn.go` instead. The generator still writes these two files; `update.sh -gen` drops them |
| `doc.go` | its `//go:generate go run ./generate` would write `tsclient.go` and `tsserver.go` here. gocpp's `doc.go` replaces it |
| `mapper.go`, `span.go`, `edits.go`, `protocol.go`, `context.go`, `log.go`, `enums.go`, `command/`, `semtok/` | gopls's own helpers on top of the types; they import other gopls packages. gocpp's `position.go` does the position conversion |
| `json_test.go`, `tsinsertreplaceedit_test.go`, `mapper_test.go` | they need `github.com/google/go-cmp` or the files that are not copied |

## 4. gocpp's own code

| File | What it is |
|---|---|
| `doc.go` | the package doc |
| `jsonrpc.go` | the JSON-RPC 2.0 messages (`Request`, `Response`, `ID`), `ResponseError` and `Errorf`, `DecodeMessage` and `EncodeMessage`. The error codes are the generated `ErrorCodes` and `LSPErrorCodes` |
| `stream.go` | the base protocol's framing: `Content-Length` headers (`Reader`, `Writer`) |
| `conn.go` | `Conn`: reads the messages, hands each request and notification to a `Handler` in order, writes the responses; `Notify`; `Request` (the server's own requests to the client, such as `client/registerCapability`, with IDs `gocpp-N`: the client's answers are matched to them, an error is logged, a result dropped) |
| `position.go` | `Mapper`: LSP positions (lines and UTF-16 units) to byte offsets and back |

These files say at the top that they are gocpp's code, and `update.sh` never touches them.

## 5. Rules

- Don't change a copied file. A change in them is lost on the next `update.sh`, and it makes
  the next update harder. Write gocpp's code in its own files (§4).
- When a copied file must change after all, the change goes into `update.sh` and into this
  file, with the reason, like the import paths in §2.
- `generate/staticcheck.conf` turns off two checks for the generator, because they report
  upstream code, which stays as it is: SA1019 (`generate.go` uses the deprecated
  `strings.Title`) and ST1005 (two error strings in `main.go` start with a capital letter). All
  other checks still run. The generated files themselves need no exceptions.

## 6. Update with update.sh

Run it to move to a newer gopls (which may bring a newer LSP), or to check the copy:

    nix develop -c internal/lsp/protocol/update.sh
    GOPLS=v0.22.0 nix develop -c internal/lsp/protocol/update.sh

It downloads the gopls module with `go mod download` (into the module cache, without changing
`go.mod`), copies the files of §1, rewrites the paths of §2, and runs gofmt, `go vet` and the
tests of this folder. It stops when a copied file has changes that are not committed (`-f`
overwrites them). With the same gopls, a second run changes nothing. After an update, change
the versions in §1 and look at `git diff`.

With `-gen`, it then runs the copied generator: it clones vscode-languageserver-node at the tag
in `generate/main.go` (`lspGitRef`) and writes `tsprotocol.go` and `tsjson.go` again. That needs
git and the network. `LSP_REF` chooses another tag, for an LSP version that gopls doesn't use
yet; the generator then runs from a temporary copy with that `lspGitRef`, and `generate/` stays
as upstream:

    LSP_REF=release/protocol/3.17.6 nix develop -c internal/lsp/protocol/update.sh -gen

Then the generated files are no longer gopls's: write the tag into §1. The generator's tables
(`generate/tables.go`) are made for gopls's tag, and a newer `metaModel.json` can need changes to
them (the generator then stops with an error); usually it is simpler to wait for a gopls release
that has the new LSP version.
