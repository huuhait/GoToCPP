# encoding/json

Ported by `tools/portstd` from go1.25.14's `src/encoding/json` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/encoding__json.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd encoding/json` |

## Files taken

- `decode.gp`, from `decode.go`
- `encode.gp`, from `encode.go`
- `fold.gp`, from `fold.go`
- `indent.gp`, from `indent.go`
- `scanner.gp`, from `scanner.go`
- `stream.gp`, from `stream.go`
- `tables.gp`, from `tables.go`
- `tags.gp`, from `tags.go`

## Files not taken

- `v2_decode.go`: //go:build goexperiment.jsonv2 is false
- `v2_encode.go`: //go:build goexperiment.jsonv2 is false
- `v2_indent.go`: //go:build goexperiment.jsonv2 is false
- `v2_inject.go`: //go:build goexperiment.jsonv2 is false
- `v2_options.go`: //go:build goexperiment.jsonv2 is false
- `v2_scanner.go`: //go:build goexperiment.jsonv2 is false
- `v2_stream.go`: //go:build goexperiment.jsonv2 is false
- 13 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `decode.go`: Go's goroutine stacks grow; gocpp's are fixed: stackRoom asks the runtime how much is left (see the note), and a nesting that does not fit is an error.
- `decode.go`: err is a result of unmarshal now (the recover sets it): it is assigned, not declared.
- `decode.go`: The recursion value -> object/array -> value must stop when the stack is full.
- `decode.go`: The recursion valueInterface -> arrayInterface/objectInterface -> valueInterface must stop when the stack is full.
- `decode.go`: A type with #Derive(JSON) is decoded as the struct it stands for, without calling its UnmarshalJSON (which would scan the document again at each level).
- `encode.go`: The encoders that recurse (pointers, slices, maps, interfaces) must stop when the stack is full.
- `encode.go`: Same: the slice encoder recurses into its elements.
- `encode.go`: Same: the map encoder recurses into its values.
- `encode.go`: Same: the interface encoder recurses into the value it holds; tooDeep is defined here.
- `encode.go` (4 places): Same: a method of the value (MarshalJSON, MarshalText) can call Marshal again, which is how a type with #Derive(JSON) nests: the recursion goes through these encoders and no other.
- `encode.go`: A type with #Derive(JSON) is encoded as the struct it stands for, without calling its MarshalJSON (which would write each level to a buffer and scan it again): the encoder of the type is chosen here, once.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `stackRoom` (`decode.gp`) | (none: no body upstream) | `//gocpp:native gp::runtime::stack_room` |

## Findings

- `format`, decode.gp: the port is not gofmt-clean: a patch or a rewrite left odd spacing
- `reflect`, decode.gp:16: imports reflect: needs the reflect subset (step 9.3)
- `linkname-export`, decode.gp:1238: unquoteBytes: only marks the function as used by other packages; dropped
- `format`, encode.gp: the port is not gofmt-clean: a patch or a rewrite left odd spacing
- `reflect`, encode.gp:54: imports reflect: needs the reflect subset (step 9.3)
- `linkname-export`, encode.gp:757: isValidNumber: only marks the function as used by other packages; dropped
- `linkname-export`, encode.gp:1244: typeFields: only marks the function as used by other packages; dropped

## Known divergences from Go

- Nesting depth. Go's decoder refuses a document nested over 10000 deep before it builds anything (`Valid` and `Unmarshal` into a `RawMessage` do exactly that here, in every build), but building the value recurses on the C++ stack, which is fixed (8 MiB for the main thread and for each goroutine, divergence D24), so what a build can nest below 10000 depends on the compiler and the mode: see the stack guard note below, which turns the rest into an error.
- Reference cycles. `Marshal` finds a cycle of pointers, maps or slices as Go does (an error after 1000 levels), but a value that is a cycle is never freed (D1), like any cyclic value. A cycle through types that have `#Derive(JSON)` is found the same way (a patch repeats `ptrEncoder`'s detection for them).
- Types with `#Derive(JSON)`. They get two methods besides `MarshalJSON` and `UnmarshalJSON` (`GoCPPJSONPlain`, `GoCPPJSONPlainPtr`: the value, and its address, as a struct of the same fields with no methods), and the decoder (`indirect`) and the encoder (`marshalerEncoder`, `addrMarshalerEncoder`) use them to decode and encode the value as the struct it stands for, without calling the two methods: those write each level to a buffer and scan it again, which makes a nested value cost the square of its depth (ADR-102). The options of the `Decoder` and the `Encoder` reach such a value, as they reach a struct without methods.
- Speed. About 3 to 7 times Go's time at `-O2`, and the stack guard costs less than 2% (ADR-102 has the numbers): the encoder and the decoder are reflection and interface heavy, and a counted pointer or slice costs an atomic operation where Go's collector costs none.
- Stack guard. Go's goroutine stack grows to 1 GB, so its decoder and encoder recurse as deep as the data goes (the decoder refuses documents nested over 10000 deep; the encoder has no limit). gocpp's stacks are fixed (8 MiB for the main thread and for each goroutine, D24) and a frame of the reflective code is several KB at `-O0`, so a hostile document or a deep value could end the program with `fatal error: stack overflow`. The patches ask the runtime how much stack is left (`stackRoom`: the bytes between the stack pointer and a reserve of 1/16 of the stack, 128 to 512 KiB, below which nothing else needs to run) at every level of the recursion and give an error when there is none: the decoder (`value`, `valueInterface`) a `*SyntaxError` "exceeded max depth (...)", the encoder (`ptrEncoder`, `sliceEncoder`, `mapEncoder`, `interfaceEncoder` and the four that call a `MarshalJSON` or `MarshalText` method) an `*UnsupportedValueError` "exceeded max depth (...)". Go's own limit of 10000 for the decoder stays and is checked first by the scanner. What a build can nest depends on the compiler and the mode: documents nested over 10000 deep are always a clean error; below that, with the 8 MiB stack, `Unmarshal` into `any` reaches about 9100 (g++ -O0), 6800 (clang++ -O0), 4700 (-asan) and 10000 (-release), into a recursive struct about 1400, 1000, 680 and 2200 to 4100, and `Marshal` of a linked list about 3200, 2600, 1600 and 5100 to 7000 (measured before the guard; the reserve takes about 6% of these, and the exact figure is the stack's). The guard costs one call into the runtime for each value that can recurse.
