// Package analysis holds the analysis of Phase 16 job 16.1f2-plan: what the GC engine's
// emitter must support to build Go's container/list, strings, strconv and sort and the Go
// version of binarytrees, with file:line evidence from Go 1.25's source.
//
//	emitter-needs.md    the construct table, the closure order, today's support and the
//	                    recommended split of step 16.1f
//	programs/           one small program per construct group (valid Go, deterministic output),
//	                    manifest.json (name, constructs, imports, expected output, today's
//	                    gocpp result) and programs_test.go, which runs each one with go run
//
// The programs are the test corpus of the 16.1f jobs: a construct is done when its program
// compiles under gocpp run -gc-engine and prints the expected output.
package analysis
