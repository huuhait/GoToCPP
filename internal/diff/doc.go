// Package diff makes the diffs that "gocpp fmt -d" prints. It is a copy of
// Go's internal/diff package (from Go 1.25.14), which gofmt -d uses, so
// both print the same diffs. A package outside Go's standard library can't
// import internal/diff, so gocpp has its own copy.
//
// diff.go and the testdata folder are copied unchanged. diff_test.go has
// two changes, because it can't import Go's internal/txtar package either:
// the import of internal/txtar is gone, and it calls parseTxtarFile where
// it called txtar.ParseFile. parseTxtarFile is in txtar_test.go, a copy of
// the parser of internal/txtar. LICENSE is Go's license (the same file as
// internal/syntax/LICENSE).
package diff
