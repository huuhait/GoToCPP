package lower

import (
	"strings"
	"testing"
)

// A package variable that owns more than maxConstinitLeaves objects is gp::lazy storage (clang++ can't
// build it by constant evaluation), and a small one stays gp::keep storage and constinit: a threshold
// that drifted down would make every small array of strings a dynamic start-up cost.
func TestLargePackageVarsAreLazy(t *testing.T) {
	cpp, err := externFilesCPP(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

type Rec struct {
	Name string
	Tags []string
}

var bigS [1 << 21]string
var bigP [1 << 20]*int
var grid [1 << 10][1 << 10]string
var smallR [1 << 12]Rec
var smallS [1 << 15]string
var plain [1 << 20]int
var bigC [1 << 20]complex128

func main() {}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := cpp["example.com/m"]
	// (In the package's header: the extern declarations of the variables' storage.)
	has := func(decl string) bool { return strings.Contains(text, decl) }
	for _, name := range []string{"bigS", "bigP", "grid"} {
		if !has("& "+name+";") || has("gp_keep_"+name) {
			t.Errorf("%s is not gp::lazy storage (extern T& %s), or it is gp::keep storage that clang++ can't build:\n%s", name, name, text)
		}
	}
	for _, name := range []string{"smallR", "smallS"} {
		if !has("gp_keep_"+name+";") || has("& "+name+";") {
			t.Errorf("%s is not gp::keep storage: it became gp::lazy (the threshold %d is too low)", name, maxConstinitLeaves)
		}
	}
	for _, name := range []string{"plain", "bigC"} {
		if !has("> "+name+";") || has("gp_keep_"+name) || has("& "+name+";") {
			t.Errorf("%s (trivially destructible) is not a plain object (extern T %s)", name, name)
		}
	}
}
