package main

// known returns the name of the known engine bug that explains the mismatch m, or "". The
// mismatches of a known bug are counted apart and do not fail the campaign, so that the bug does not
// hide others. There is none now: add an entry (with the bug's repro in a comment) only while a bug
// is open, and remove it when the bug is fixed.
func known(g *Gen, m Mismatch) string {
	return ""
}
