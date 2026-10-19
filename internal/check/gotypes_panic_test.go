package check

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// go/types of Go 1.25 panics ("nil underlying") on a type that is declared from a selector of
// itself: type T T.x.y. gocpp check and the language server must report an error, not crash (found
// by the 15.8 fuzzer).
func TestGoTypesPanicIsAnError(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\ntype Deep Deep.A.A0\n\nfunc main() {}\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	got := errorText(t, prog, r, root, false)
	if !strings.Contains(got, "type checker failed") {
		t.Errorf("errors:\n%s", got)
	}
}
