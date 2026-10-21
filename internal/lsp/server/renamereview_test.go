package server

import (
	"strings"
	"testing"
)

// The cases of the review of the proofs of the result of a rename: a form that the proofs
// took, and the scratch check refuses.

const reviewMod = "example.com/rv"

func reviewImports(pkgs ...string) string {
	var out strings.Builder
	out.WriteString("import (\n")
	for _, pkg := range pkgs {
		out.WriteString("\t\"" + reviewMod + "/" + pkg + "\"\n")
	}
	out.WriteString(")\n\n")
	return out.String()
}

func TestRenameProofReviewCases(t *testing.T) {
	item := "package a\n\ntype Item struct {\n\tTitle string\n}\n\nfunc Make() Item { return Item{} }\n"
	hiddenItem := "package a\n\ntype Item struct {\n\tTitle string\n\tinner int\n}\n\n" +
		"func Make() Item { return Item{} }\n"
	for _, tc := range []struct {
		name    string
		files   map[string]string
		mark    string
		newName string
		outcome string
	}{
		{name: "constraint of a package that a does not import, called by an importer",
			files: map[string]string{
				"a/a.gp": item,
				"c/c.gp": "package c\n\nfunc G[T ~struct{ Title string }](t T) string { return t.Title }\n",
				"b/b.gp": "package b\n\n" + reviewImports("a", "c") +
					"func Use() string { return c.G(a.Item{}) }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "refuse"},
		{name: "function of a package that a does not import returns an unnamed struct",
			files: map[string]string{
				"a/a.gp": item,
				"c/c.gp": "package c\n\n" +
					"func Make() struct{ Title string } { return struct{ Title string }{} }\n",
				"b/b.gp": "package b\n\n" + reviewImports("a", "c") +
					"func Use() a.Item { var o a.Item = c.Make(); return o }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "refuse"},
		{name: "alias of an unnamed struct of a third package used by a package of a",
			files: map[string]string{
				"a/a.gp": item,
				"e/e.gp": "package e\n\ntype Row = struct{ Title string }\n",
				"d/d.gp": "package d\n\n" + reviewImports("a", "e") +
					"func Use() string { var s e.Row = a.Make(); return s.Title }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "refuse"},
		{name: "slice of an unnamed struct of a third package gets a value of the type",
			files: map[string]string{
				"a/a.gp": item,
				"e/e.gp": "package e\n\nvar Rows []struct{ Title string }\n",
				"d/d.gp": "package d\n\n" + reviewImports("a", "e") +
					"func Use() { e.Rows = append(e.Rows, a.Item{}) }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "refuse"},
		{name: "struct with an unexported field is identical to no struct of another package",
			files: map[string]string{
				"a/a.gp": hiddenItem,
				"c/c.gp": "package c\n\n" +
					"func Make() struct{ Title string } { return struct{ Title string }{} }\n",
				"b/b.gp": "package b\n\n" + reviewImports("a", "c") +
					"func Use() string { return c.Make().Title + a.Make().Title }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "proved"},
		{name: "struct of exported fields only has the scratch check",
			files: map[string]string{
				"a/a.gp": item,
				"b/b.gp": "package b\n\n" + reviewImports("a") +
					"func Use() string { return a.Make().Title }\n",
			}, mark: "|Title string", newName: "Heading", outcome: "fallback"},
		{name: "local of a block of a function with a result of the new name, bare return",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() (r int) {\n\t{\n" +
					"\t\tx := 1\n\t\t_ = x\n\t\treturn\n\t}\n}\n",
			}, mark: "|x := 1", newName: "r", outcome: "refuse"},
		{name: "local of a closure with a result of the new name, bare return",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() int {\n" +
					"\tf := func() (r int) {\n\t\tif true {\n\t\t\tx := 1\n" +
					"\t\t\t_ = x\n\t\t\treturn\n\t\t}\n\t\treturn\n\t}\n\treturn f()\n}\n",
			}, mark: "|x := 1", newName: "r", outcome: "refuse"},
		{name: "local of a loop of a function with a result of the new name, bare return",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() (r int) {\n" +
					"\tfor i := 0; i < 2; i++ {\n\t\tx := i\n\t\t_ = x\n" +
					"\t\tif x > 0 {\n\t\t\treturn\n\t\t}\n\t}\n\treturn\n}\n",
			}, mark: "|x := i", newName: "r", outcome: "refuse"},
		{name: "local of a block with a return with an operand",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() (r int) {\n\t{\n" +
					"\t\tx := 1\n\t\t_ = x\n\t\treturn 2\n\t}\n}\n",
			}, mark: "|x := 1", newName: "r", outcome: "proved"},
		{name: "local of a block that ends before the bare return",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() (r int) {\n\t{\n\t\tx := 1\n\t\t_ = x\n\t}\n\treturn\n}\n",
			}, mark: "|x := 1", newName: "r", outcome: "proved"},
		{name: "local beside a closure without results that returns bare",
			files: map[string]string{
				"a/a.gp": "package a\n\nfunc F() (r int) {\n\t{\n" +
					"\t\tx := 1\n\t\tf := func() { _ = x; return }\n" +
					"\t\tf()\n\t}\n\treturn 3\n}\n",
			}, mark: "|x := 1", newName: "r", outcome: "proved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"gocpp.mod": "module " + reviewMod + "\n"}
			for name, text := range tc.files {
				files[name] = text
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
