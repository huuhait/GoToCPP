package server

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func localProofFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/local\n",
		"a/a.gp": `package a

import "strings"

var total = 100

func Sum(items []int) int {
	sum := 0
	for _, it := range items {
		sum += it
	}
	return sum
}

func Shadow(items []int) int {
	outer := 1
	{
		inner := 2
		outer += inner
	}
	return outer
}

func Param(count int) int {
	return count + 1
}

func ParamTotal(count int) int {
	return count + total
}

func Closure() int {
	acc := 0
	add := func(n int) { acc += n }
	add(1)
	return acc
}

func Later() int {
	x := 1
	_ = x
	return total
}

func Named() (result int) {
	result = 4
	return
}

func Fields() int {
	type pair struct{ other, extra int }
	p := pair{other: 1}
	n := 2
	return p.other + n
}

func Both(word string) string {
	return strings.ToUpper(word)
}

func Twin(left int) int {
	right := 1
	return right
}

func Early() int {
	first := total
	second := 1
	return first + second
}

func Kind(v any) int {
	switch value := v.(type) {
	case int:
		return value
	}
	return 0
}
`,
	}
}

// TestRenameLocalBindingProof: the rename of a local name gives the answer of
// the check of the result, and the forms the proof covers make no scratch check.
func TestRenameLocalBindingProof(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(map[string]string)
		mark    string
		newName string
		outcome string
	}{
		{name: "variable of a loop function", mark: "|sum := 0", newName: "result",
			outcome: "proved"},
		{name: "request at a use", mark: "return |sum", newName: "result", outcome: "proved"},
		{name: "variable of a nested block", mark: "|inner := 2", newName: "part",
			outcome: "proved"},
		{name: "outer variable beside a nested block", mark: "|outer := 1", newName: "base",
			outcome: "proved"},
		{name: "parameter", mark: "func Param(|count int)", newName: "amount", outcome: "proved"},
		{name: "named result", mark: "func Named() (|result int)", newName: "out",
			outcome: "proved"},
		{name: "variable captured by a closure", mark: "|acc := 0", newName: "running",
			outcome: "proved"},
		{name: "new name of a package variable that the function does not use",
			mark: "|sum := 0", newName: "total", outcome: "proved"},
		{name: "new name of a field", mark: "|n := 2", newName: "other", outcome: "proved"},
		{name: "new name of an import that the function does not use",
			mark: "|sum := 0", newName: "strings", outcome: "proved"},
		{name: "new name declared in another function", mark: "|x := 1", newName: "part",
			outcome: "proved"},
		{name: "new name of the package variable that the function uses",
			mark: "func ParamTotal(|count int)", newName: "total", outcome: "refuse"},
		{name: "new name of an import that the function uses", mark: "func Both(|word string)",
			newName: "strings", outcome: "refuse"},
		{name: "new name of the package variable that the block uses later",
			mark: "|x := 1", newName: "total", outcome: "refuse"},
		{name: "new name of another variable of the block", mark: "|sum := 0", newName: "items",
			outcome: "refuse"},
		{name: "new name of a built-in function used in the block", mark: "|sum := 0",
			newName: "len", outcome: "proved", setup: func(files map[string]string) {
				files["a/a.gp"] = strings.Replace(files["a/a.gp"],
					"\treturn sum\n", "\treturn sum + cap(items)\n", 1)
			}},
		{name: "new name of a built-in function the block uses", mark: "|sum := 0",
			newName: "cap", outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] = strings.Replace(files["a/a.gp"],
					"\treturn sum\n", "\treturn sum + cap(items)\n", 1)
			}},
		{name: "name of a type switch", mark: "|value := v.(type)", newName: "got",
			outcome: "fallback"},
		{name: "broken file of the package", mark: "|sum := 0", newName: "result",
			outcome: "fallback", setup: func(files map[string]string) {
				files["a/broken.gp"] = "package a\n\nvar Broken = undefinedName\n"
			}},
		{name: "macro of the package", mark: "|sum := 0", newName: "result", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/a.gp"] += "\nfunc Place() string { return #file() }\n"
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := localProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			full, fullChecks := fastRenameFiles(t, files, "a/a.gp", true, tc.mark, tc.newName)
			fast, fastChecks := fastRenameFiles(t, files, "a/a.gp", false, tc.mark, tc.newName)
			if full != fast {
				t.Fatalf("proof changes the full-validator answer\nfull:\n%s\nproof:\n%s", full, fast)
			}
			refused := strings.HasPrefix(fast, "error:")
			if refused != (tc.outcome == "refuse") {
				t.Fatalf("unexpected refusal or success: %s", fast)
			}
			if tc.outcome == "proved" && (fastChecks != 0 || fullChecks == 0) {
				t.Fatalf("proof=%d checks, full=%d; want 0 vs positive", fastChecks, fullChecks)
			}
			if tc.outcome == "fallback" && fastChecks == 0 {
				t.Fatal("unproven form did not fall back to scratch validation")
			}
		})
	}
}

// TestLocalProofChecksTheBlockItself: the proof does not lean on the conflict
// guards that run before it. With the guards' outcome assumed, it holds exactly
// where the block has no name of the new name.
func TestLocalProofChecksTheBlockItself(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mark    string
		newName string
		proved  bool
	}{
		{"free name", "|sum := 0", "result", true},
		{"package variable that the block does not use", "|sum := 0", "total", true},
		{"variable of the same block", "|sum := 0", "items", false},
		{"variable declared in the block", "|sum := 0", "it", false},
		{"package variable that the block uses", "func ParamTotal(|count int)", "total", false},
		{"import that the block uses", "func Both(|word string)", "strings", false},
		{"built-in function that the block does not use", "func Both(|word string)", "len", true},
		{"field of the block's literal", "|n := 2", "other", true},
		{"parameter that the block does not mention again", "|right := 1", "left", false},
		{"package variable used before the declaration", "|second := 1", "total", true},
		{"variable of a nested block", "|outer := 1", "inner", false},
		{"variable of the enclosing function", "|acc := 0", "add", false},
		{"parameter of the closure", "|acc := 0", "n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rn := renamerFor(t, localProofFiles(), "a/a.gp", tc.mark, tc.newName)
			rn.bindingsChecked = true
			dirs := map[string]bool{rn.res.hits[0].v.dir: true}
			if got := rn.localProvesResult(context.Background(), dirs); got != tc.proved {
				t.Fatalf("proved = %v, want %v", got, tc.proved)
			}
		})
	}
}

// TestLocalProofNeedsEveryOccurrence: an occurrence of the variable that the
// search did not find keeps the check of the result.
func TestLocalProofNeedsEveryOccurrence(t *testing.T) {
	rn := renamerFor(t, localProofFiles(), "a/a.gp", "|sum := 0", "result")
	rn.bindingsChecked = true
	dirs := map[string]bool{rn.res.hits[0].v.dir: true}
	if !rn.localProvesResult(context.Background(), dirs) {
		t.Fatal("the complete search is not proved")
	}
	hits := rn.res.hits
	for i, h := range hits {
		if !h.def {
			partial := *rn.res
			partial.hits = append(slices.Clone(hits[:i]), hits[i+1:]...)
			rn.res = &partial
			break
		}
	}
	if rn.localProvesResult(context.Background(), dirs) {
		t.Fatal("a search without one use is proved")
	}
}

// TestLocalProofIsOnlyForLocalNames: names of a package, and a canceled
// request, are not proved by it.
func TestLocalProofIsOnlyForLocalNames(t *testing.T) {
	rn := renamerFor(t, localProofFiles(), "a/a.gp", "var |total = 100", "everything")
	rn.bindingsChecked = true
	dirs := map[string]bool{rn.res.hits[0].v.dir: true}
	if rn.localProvesResult(context.Background(), dirs) {
		t.Fatal("a package variable is proved as a local one")
	}
	local := renamerFor(t, localProofFiles(), "a/a.gp", "|sum := 0", "result")
	local.bindingsChecked = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if local.localProvesResult(ctx, map[string]bool{local.res.hits[0].v.dir: true}) {
		t.Fatal("a canceled request is proved")
	}
	local.bindingsChecked = false
	if local.localProvesResult(context.Background(), map[string]bool{local.res.hits[0].v.dir: true}) {
		t.Fatal("a rename without the guards is proved")
	}
}
