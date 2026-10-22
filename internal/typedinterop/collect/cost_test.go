package collect

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The cost of the collector grows with the size of the program, not faster: the variables of a package
// were each looked up in all of Defs (quadratic). (The flat chain, which was cubic, is tested through
// the check in cmd/gocpp: TestTypedCheckOfAFlatChainAllocatesLittle.)

func collectCost(t *testing.T, src string) time.Duration {
	t.Helper()
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program does not check: %v", errs[0])
	}
	start := time.Now()
	_ = Collect(p, fset)
	return time.Since(start)
}

func TestCollectCostOfManyVariablesIsNotQuadratic(t *testing.T) {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n" + stdImports + ")\n\nfunc main() {\n\tn := 0\n")
	const vars = 40000
	for i := range vars {
		fmt.Fprintf(&b, "\tv%d := cstdlib.abs(%d)\n\t_ = v%d\n", i, i, i)
	}
	b.WriteString("\tfmt.Println(n)\n}\n")
	took := collectCost(t, b.String())
	t.Logf("%d variables: %v", vars, took)
	if took > 4*time.Second {
		t.Errorf("%d variables take %v", vars, took)
	}
}
