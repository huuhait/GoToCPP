// Package fuzz holds the fuzz targets of the whole compile pipeline (Phase 15 step 15.8): parse,
// format, check, lower and emit, the sampled C++ compile of what is emitted, the language server,
// and the target that feeds the structure-aware program generator (tools/fuzz/gen). The targets
// are in the _test.go files, so that "go test" runs their seed corpora and "go test -fuzz" fuzzes
// them; tools/fuzz/campaign runs them for a time each (make fuzz-short, make fuzz-long), and
// docs/guides/fuzzing.md says how.
//
// The other fuzz targets are in their packages: FuzzCppErrors (cmd/gocpp), FuzzParse,
// FuzzParseRecipe and FuzzParseVendor (internal/load/gocppmod, the gocpp.mod parser), and the
// ones of internal/modfetch and of the parser and printer.
package fuzz
