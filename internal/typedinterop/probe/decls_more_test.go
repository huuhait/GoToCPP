package probe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

var someTypes = []DeclQuery{{DeclType, "geo::Vec"}, {DeclType, "Rect"}, {DeclType, "geo::Old"}, {DeclFunc, "geo::f"}, {DeclType, "Id"}}

// The same queries give the same table, byte for byte, from two runners.
func TestDeclsDeterministic(t *testing.T) {
	cfg, dc := declsSetup(t)
	var out [2][]byte
	for i := range out {
		tab, err := new(Runner).Decls(context.Background(), cfg, dc, someTypes)
		if err != nil {
			t.Fatal(err)
		}
		tab.Timing, tab.Stats, tab.Deps, tab.Runs = Timing{}, Stats{}, nil, 0
		out[i], _ = json.Marshal(tab)
	}
	if string(out[0]) != string(out[1]) {
		t.Errorf("two runs differ:\n%s\n%s", out[0], out[1])
	}
	if !strings.Contains(string(out[0]), `"x"`) {
		t.Errorf("the table has no fields: %s", out[0])
	}
}

// A second call is served from the memo with no compiler run; an edit of a header the
// run read makes it run again; the order of the queries does not matter.
func TestDeclsMemo(t *testing.T) {
	cfg, dc := declsSetup(t)
	rn := new(Runner)
	ctx := context.Background()
	first, err := rn.Decls(ctx, cfg, dc, someTypes)
	if err != nil {
		t.Fatal(err)
	}
	if first.Runs != 5 || first.Cached || rn.DeclRuns() != 5 {
		t.Errorf("first: %d runs, cached %v, %d total", first.Runs, first.Cached, rn.DeclRuns())
	}
	rev := append([]DeclQuery(nil), someTypes...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	second, err := rn.Decls(ctx, cfg, dc, rev)
	if err != nil {
		t.Fatal(err)
	}
	if second.Runs != 0 || !second.Cached || rn.DeclRuns() != 5 {
		t.Errorf("second: %d runs, cached %v, %d total", second.Runs, second.Cached, rn.DeclRuns())
	}
	if second.Records["geo::Vec"] != first.Records["geo::Vec"] {
		t.Error("the memo returned another record")
	}
	// The header changes: the type run of Vec runs again and sees the new field, the others
	// that read the same header run again too (their key has the header's contents).
	writeHeader(t, cfg.Dir, "decls.hpp", strings.Replace(declsHeader, "struct Vec { double x, y;", "struct Vec { double x, y, z;", 1))
	third, err := rn.Decls(ctx, cfg, dc, someTypes)
	if err != nil {
		t.Fatal(err)
	}
	if third.Runs == 0 || len(third.Records["geo::Vec"].Fields) != 4 {
		t.Errorf("third: %d runs, Vec %v", third.Runs, third.Records["geo::Vec"].Fields)
	}
}

func TestDeclsQueryErrors(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclType, ""}, {DeclType, "123"}, {DeclType, "int"}, {DeclType, "const Rect"}, {DeclType, "struct Rect"}, {DeclType, "Rect *"},
		{DeclType, "std::vector<"}, {DeclType, "gocpp_probe_internal_pr::string"}, {DeclType, "(unnamed struct)"}, {DeclType, "Box::(unnamed union)"},
		{DeclType, "not a type"}, {DeclFunc, ""}, {DeclFunc, "Rect *"}, {DeclFunc, "int"}, {"weird", "Rect"},
		{DeclType, "::Rect"}, {DeclType, "Rect"}, {DeclType, "Rect"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"type ", "type 123", "type int", "type const Rect", "type struct Rect", "type Rect *", "type std::vector<",
		"type gocpp_probe_internal_pr::string", "type (unnamed struct)", "type not a type", "func ", "func Rect *", "func int", "weird Rect"} {
		if tab.Errors[k] == "" {
			t.Errorf("no error for %q", k)
		}
	}
	if tab.Records["Rect"] == nil || len(tab.Records) != 1 || tab.Runs != 1 {
		t.Errorf("records %v runs %d (::Rect and the repeats are one query)", tab.Records, tab.Runs)
	}
	if tab.Errors["type Rect"] != "" || tab.Errors["type ::Rect"] != "" {
		t.Errorf("Rect has an error: %v", tab.Errors)
	}
}

func TestDeclsLimits(t *testing.T) {
	cfg, dc := declsSetup(t)
	dc.MaxRuns = 2
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Rect"}, {DeclType, "Pt"}, {DeclType, "U"}, {DeclType, "Box"}})
	if err != nil {
		t.Fatal(err)
	}
	// The runs are taken in the order of the names (Box, Pt, Rect, U): the first two.
	if len(tab.Records) != 2 || tab.Records["Box"] == nil || tab.Records["Pt"] == nil || !strings.Contains(tab.Errors["type U"], "more than 2") || tab.Errors["type Rect"] == "" {
		t.Errorf("records %v errors %v", tab.Records, tab.Errors)
	}
	// A dump over the limit: the names of the run have an error, and it is remembered.
	rn := new(Runner)
	dc = DeclConfig{Includes: dc.Includes, MaxJSON: 1000}
	for i := 0; i < 2; i++ {
		tab, err = rn.Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "geo::Vec"}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(tab.Errors["type geo::Vec"], "larger than the limit") || tab.Records["geo::Vec"] != nil {
			t.Errorf("round %d: %v", i, tab.Errors)
		}
	}
	if rn.DeclRuns() != 1 {
		t.Errorf("%d runs: the refusal was not remembered", rn.DeclRuns())
	}
}

func TestDeclsCancel(t *testing.T) {
	cfg, dc := declsSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := new(Runner).Decls(ctx, cfg, dc, someTypes); err == nil {
		t.Error("a cancelled context gave a table")
	}
}

// Many names at once, in parallel: every record is there and the memo holds (run with -race).
func TestDeclsManyAtOnce(t *testing.T) {
	dir := t.TempDir()
	var h strings.Builder
	h.WriteString("#pragma once\n")
	var qs []DeclQuery
	for i := 0; i < 24; i++ {
		name := "S" + string(rune('a'+i%26)) + string(rune('A'+i/26)) + "x"
		h.WriteString("struct " + name + " { int a; long b; };\n")
		qs = append(qs, DeclQuery{DeclType, name})
	}
	writeHeader(t, dir, "many.hpp", h.String())
	cfg := clangConfig(t, dir)
	rn := new(Runner)
	tab, err := rn.Decls(context.Background(), cfg, DeclConfig{Includes: []string{`"./many.hpp"`}, Parallel: 6}, qs)
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.Records) != 24 || tab.Runs != 24 || len(tab.Errors) != 0 {
		t.Errorf("%d records, %d runs, errors %v", len(tab.Records), tab.Runs, tab.Errors)
	}
	for k, r := range tab.Records {
		if len(r.Fields) != 2 || r.Size != 16 {
			t.Errorf("%s: %+v", k, r)
		}
	}
	// The same names in one run each: the names with the same last part share a run, the others do not.
}

// Names with the same last part share one compiler run.
func TestDeclsShareARun(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "ns.hpp", "#pragma once\nnamespace a { struct Item { int x; }; }\nnamespace b { struct Item { long y, z; }; }\nstruct Item { char c; };\n")
	cfg := clangConfig(t, dir)
	tab, err := new(Runner).Decls(context.Background(), cfg, DeclConfig{Includes: []string{`"./ns.hpp"`}}, []DeclQuery{{DeclType, "a::Item"}, {DeclType, "b::Item"}, {DeclType, "Item"}})
	if err != nil {
		t.Fatal(err)
	}
	if tab.Runs != 1 {
		t.Errorf("%d runs for three names with one last part", tab.Runs)
	}
	if got := fieldList(tab.Records["a::Item"]) + "|" + fieldList(tab.Records["b::Item"]) + "|" + fieldList(tab.Records["Item"]); got != "x:int|y:long z:long|c:char" {
		t.Errorf("the three Items: %s (%v)", got, tab.Errors)
	}
}

// A typedef name gives the canonical type: of a record, of a builtin.
func TestDeclsCanon(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "FILE"}, {DeclType, "Id"}, {DeclType, "Rect"}, {DeclType, "geo::Vec"}})
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"FILE": "_IO_FILE", "Id": "unsigned int", "Rect": "Rect", "geo::Vec": "geo::Vec"} {
		if tab.Canon[k] != want {
			t.Errorf("Canon[%s] = %q, want %q", k, tab.Canon[k], want)
		}
	}
	if tab.Records["_IO_FILE"] == nil || tab.Records["FILE"] != nil || tab.Errors["type Id"] != "" || len(tab.Errors) != 0 {
		t.Errorf("records %v errors %v", tab.Records, tab.Errors)
	}
}

// The text of a query is not written into the probe file as it is: a name with a comment, a
// directive, a newline or a semicolon is refused and does not change the other answers.
func TestDeclsInjection(t *testing.T) {
	cfg, dc := declsSetup(t)
	evil := []string{
		"T<1\n#error injected\n>", "T<1 //\n>", "T<1 /* >", "Rect; int x", "Rect /* */", "Rect\x00", "Rect\r\nint y", "A<\"s\">",
		"A<1 */>", "Rect}", "Rect{", "Rect\\", "A<`>", "A<#>", "A<1 // >", "A<1\x7f>",
	}
	var qs []DeclQuery
	for _, e := range evil {
		qs = append(qs, DeclQuery{DeclType, e}, DeclQuery{DeclFunc, e})
	}
	// good queries around the bad ones, in both kinds of runs
	qs = append([]DeclQuery{{DeclType, "Rect"}, {DeclFunc, "geo::f"}}, qs...)
	qs = append(qs, DeclQuery{DeclType, "geo::Vec"}, DeclQuery{DeclFunc, "fopen"})
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evil {
		for _, k := range []DeclKind{DeclType, DeclFunc} {
			if tab.Errors[QueryKey(DeclQuery{k, e})] == "" {
				t.Errorf("%s %q has no error", k, e)
			}
		}
	}
	if tab.Records["Rect"] == nil || tab.Records["geo::Vec"] == nil || tab.Funcs["geo::f"] == nil || tab.Funcs["fopen"] == nil || len(tab.HeaderErrors) != 0 {
		t.Errorf("a good query was spoiled: %v %v\nheader errors %v", tab.Records, tab.Funcs, tab.HeaderErrors)
	}
	for k, e := range tab.Errors {
		if strings.Contains(k, "Rect") && !strings.Contains(k, "Rect;") && strings.Contains(e, "error") && !strings.Contains(e, "probe file") {
			t.Errorf("%q: %s", k, e)
		}
	}
	// The names of a run share one file: bad names with the same last part as good ones must
	// not spoil them (a comment that is not closed would swallow the lines after it).
	same := []string{"T<1 /* >", "T<1 //\n>", "T<1\n#error injected\n>", "T<1; int zz>", "T<'a' /* '>", "T<\"x>", "nn::T<1 /* >", "T</* */1>", "T<1> /* "}
	qs = []DeclQuery{{DeclType, "T"}, {DeclType, "nn::T"}}
	for _, e := range same {
		qs = append(qs, DeclQuery{DeclType, e})
	}
	qs = append(qs, DeclQuery{DeclType, "n::T"}) // a name that is only wrong
	tab, err = new(Runner).Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := tab.Records["T"], tab.Records["nn::T"]; a == nil || b == nil || fieldList(a) != "a:int" || fieldList(b) != "b:long" || len(tab.HeaderErrors) != 0 {
		t.Errorf("good names with the same last part were spoiled: %v %v errors %v header %v", a, b, tab.Errors, tab.HeaderErrors)
	}
	for _, e := range same {
		if tab.Errors["type "+e] == "" {
			t.Errorf("%q has no error", e)
		}
	}
	// A template-id for a function is not a function name; a legal template argument that is an expression is written back normalized.
	tab, err = new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclFunc, "geo::g<int>"}, {DeclType, "Buf<char,  8>"}, {DeclType, "Wrap< int >"}})
	if err != nil {
		t.Fatal(err)
	}
	if tab.Errors["func geo::g<int>"] != "not a function name" || tab.Records["Buf<char, 8>"] != nil && tab.Records["Buf<char, 8>"].Size != 8 {
		t.Errorf("%v %v", tab.Errors, tab.Records)
	}
}
