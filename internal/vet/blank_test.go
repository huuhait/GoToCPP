package vet

import "testing"

// A parenthesized blank identifier has no type in go/types (neither does the
// bare one on the left of an assignment): no analyzer crashes on it (printf
// did, through refactor/satisfy: "no type for *ast.ParenExpr"). The shapes
// are those of tests/run/blank_label_paren.gp (step 15.10).
func TestParenBlankNoCrash(t *testing.T) {
	src := `package main

import "fmt"

func two() (int, int) { return 1, 2 }

func labels() {
_:
	fmt.Println("after the first blank label")
	{
	_:
	}
_:
}

func main() {
	labels()
	s := []int{1, 2, 3}
	x := 1
	(_) = x
	((_)) = 3
	(x) = 5
	(_), (x) = two()
	(_), x = 1, x+1
	for (_) = range s {
	}
	for (_), (x) = range s {
	}
	switch (_) = 7; {
	}
	var e error
	(_) = e
	fmt.Println(x, e)
	fmt.Printf("%d\n", x)
}
`
	p := loadProgram(t, map[string]string{"main.gp": src}, ".")
	rep := p.vet(t)
	if c := rep.Crashed(); len(c) > 0 {
		for _, n := range c {
			t.Errorf("analyzer %s crashed: %s", n.Analyzer, n.Text)
		}
	}
	for _, f := range rep.Findings {
		t.Logf("finding: [%s] %v: %s", f.Analyzer, f.Pos, f.Message)
	}
}
