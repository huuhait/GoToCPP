package server

import (
	"context"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// keywordHint is the error for the uses of <sys/select.h> by its default name, select.
const keywordHint = "<sys/select.h> has no package name, because select is a Go keyword: give it a name: import cselect \"<sys/select.h>\""

// TestDiagnosticsAgreeWithBuild: the editor shows what gocpp check and gocpp build report for a
// program that a C/C++ name or header breaks (cmd/gocpp's TestCheckAndBuildAgree has the other
// two): the error of a const that holds a C variable, the hint for a keyword header name, and the
// hint for a header named like a declaration of the package or like no Go name. The server has a
// fake C++ compiler and a namespace probe that finds no namespace.
func TestDiagnosticsAgreeWithBuild(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{
		"gocpp.mod":        "module example.com/m\n",
		"const/const.gp":   "package main\n\nfunc main() {\n\tvar x C.int = 2\n\tconst d = x + 1\n\t_ = d\n}\n",
		"select/select.gp": "package main\n\nimport \"<sys/select.h>\"\n\nvar set select.fd_set\n",
		"uses/uses.gp": "package main\n\nimport \"<sys/select.h>\"\n\nfunc main() {\n\tvar set int\n\tselect.FD_ZERO(&set)\n" +
			"\tprintln(select.FD_SETSIZE)\n\tprintln(select.FD_SETSIZE)\n}\n",
		"mainh/main.gp": "package main\n\nimport \"./main.h\"\n\nfunc main() {\n\tprintln(main.mn())\n}\n",
		"mainh/main.h":  "int mn(void);\n",
		"d3/d3.gp":      "package main\n\nimport \"./3d.h\"\n\nfunc main() {\n\tprintln(d3.mn())\n}\n",
		"d3/3d.h":       "int mn(void);\n",
	}
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = func(_ context.Context, _ build.CXX, _ string, _, _, cands []string) (interop.Result, error) {
		r := interop.Result{Kinds: make(map[string]interop.Kind)}
		for _, c := range cands {
			r.Kinds[c] = interop.NotFound
		}
		return r, nil
	}
	c := startDiag(t, cfg, files, nil)
	for _, tc := range []struct{ file, want string }{
		{"const/const.gp", "4:11-4:16 error: x + 1 (C/C++ value) is not constant\n"},
		{"select/select.gp", "4:8-4:14 error: <sys/select.h> has no package name, because select is a Go keyword: " +
			"give it a name: import cselect \"<sys/select.h>\"\n"},
		// One error for each use, at the keyword, and none of the parser's follow-ups.
		{"uses/uses.gp", "6:1-6:7 error: " + keywordHint + "\n7:9-7:15 error: " + keywordHint + "\n8:9-8:15 error: " + keywordHint + "\n"},
		{"mainh/main.gp", "5:14-5:16 error: main.mn undefined (type func() has no field or method mn); " +
			"\"./main.h\" has no package name, because main is declared by the package: " +
			"give it a name: import cmain \"./main.h\"\n"},
		{"d3/d3.gp", "5:9-5:11 error: undefined: d3; \"./3d.h\" has no package name, because 3d is not a valid Go name: " +
			"give it a name: import d3 \"./3d.h\"\n"},
	} {
		c.open(tc.file, files[tc.file])
		// The probe answers in the background: the first publish may come without it, and
		// c.next fails the test when the right one does not come after the hang limit.
		for format(c.next(tc.file)) != tc.want {
		}
	}
}
