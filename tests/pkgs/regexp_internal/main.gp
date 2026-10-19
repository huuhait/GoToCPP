// phase: 09
// Go 1.25's internal tests of package regexp (all_test.go, find_test.go, exec_test.go, exec2_test.go, onepass_test.go), which name unexported
// functions and fields, run in a copy of the package (regexp/, the files that tools/portstd writes, plus the converted tests); a Go test
// compares the copy with stdlib/regexp. See ADR-083.

package main

import "example.com/rxtest/regexp"

func main() {
	regexp.RunAll()
}

// Output:
// ok TestGoodCompile subtests: 0 checks: 34
// ok TestBadCompile subtests: 0 checks: 24
// ok TestMatch subtests: 0 checks: 425
// ok TestMatchFunction subtests: 0 checks: 85
// ok TestCopyMatch subtests: 0 checks: 340
// ok TestReplaceAll subtests: 0 checks: 216
// ok TestReplaceAllLiteral subtests: 0 checks: 261
// ok TestReplaceAllFunc subtests: 0 checks: 9
// ok TestQuoteMeta subtests: 0 checks: 22
// ok TestLiteralPrefix subtests: 0 checks: 34
// ok TestSubexp subtests: 0 checks: 86
// ok TestSplit subtests: 0 checks: 84
// ok TestParseAndCompile subtests: 0 checks: 6
// ok TestOnePassCutoff subtests: 0 checks: 3
// ok TestSwitchBacktrack subtests: 0 checks: 0
// ok TestDeepEqual subtests: 0 checks: 4
// ok TestMinInputLen subtests: 0 checks: 12
// ok TestUnmarshalText subtests: 1 checks: 137
// ok TestFind subtests: 0 checks: 221
// ok TestFindString subtests: 0 checks: 68
// ok TestFindIndex subtests: 0 checks: 68
// ok TestFindStringIndex subtests: 0 checks: 68
// ok TestFindReaderIndex subtests: 0 checks: 68
// ok TestFindAll subtests: 0 checks: 320
// ok TestFindAllString subtests: 0 checks: 194
// ok TestFindAllIndex subtests: 0 checks: 194
// ok TestFindAllStringIndex subtests: 0 checks: 194
// ok TestFindSubmatch subtests: 0 checks: 353
// ok TestFindStringSubmatch subtests: 0 checks: 260
// ok TestFindSubmatchIndex subtests: 0 checks: 260
// ok TestFindStringSubmatchIndex subtests: 0 checks: 260
// ok TestFindReaderSubmatchIndex subtests: 0 checks: 260
// ok TestFindAllSubmatch subtests: 0 checks: 606
// ok TestFindAllStringSubmatch subtests: 0 checks: 448
// ok TestFindAllSubmatchIndex subtests: 0 checks: 448
// ok TestFindAllStringSubmatchIndex subtests: 0 checks: 448
// ok TestRE2Search subtests: 0 checks: 66070
// ok TestFowler subtests: 0 checks: 15265
// ok TestLongest subtests: 0 checks: 3
// ok TestProgramTooLongForBacktrack subtests: 0 checks: 2
// ok TestRE2Exhaustive subtests: 0 checks: 1
// ok TestMergeRuneSet subtests: 0 checks: 26
// ok TestCompileOnePass subtests: 0 checks: 111
// ok TestRunOnePass subtests: 0 checks: 3
// tests: 44 failed: 0
