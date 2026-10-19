package main

import "testing"

// TestApostropheInFileName builds a program with a file name that has an apostrophe. The same
// name was a fixture in tests/pkgs/odd_file_names, but a file name with an apostrophe is not allowed
// in a Go module zip, so a repository that has one can't be installed with go install (step 15.13,
// TestGoInstallOfATaggedVersion in internal/packaging): the file is made here, in a temporary folder.
func TestApostropheInFileName(t *testing.T) {
	anyCXX(t)
	t.Setenv("GOCPP_TEST_PHASE", "06")
	inDir(t, map[string]string{
		"gocpp.mod":     "module example.com/quote\n",
		"main.gp":       "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/quote/lib\"\n)\n\nfunc main() { fmt.Println(lib.Q()) }\n",
		"lib/q'uote.gp": "package lib\n\nfunc Q() int { return 7 }\n",
	})
	if r := gocppRun("", false, "run", "."); r != (result{0, "7\n", ""}) {
		t.Errorf("gocpp run . = %+v, want exit 0 and output 7", r)
	}
}
