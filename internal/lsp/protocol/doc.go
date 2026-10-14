// Package protocol is the Language Server Protocol (LSP 3.17) for
// "gocpp lsp": its types, its JSON-RPC 2.0 messages and their framing.
//
// The types (tsprotocol.go, tsjson.go) are gopls's generated code, copied
// with the generator that writes them from the LSP's metaModel.json
// (generate/), and a few files of gopls's own code that the types need
// (uri.go, form.go, tsdocument_changes.go, tsinsertreplaceedit.go).
// COPIED.md says from where and how to update them; those files are not
// changed here.
//
// The rest is gocpp's: the messages and the error codes (jsonrpc.go), the
// Content-Length framing (stream.go), a connection that hands each message
// to a Handler (conn.go) and the conversion between positions and byte
// offsets (position.go).
package protocol
