package probe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The type runs and the precompiled header (ADR-117, revised by step 12.14b): a run for the program's
// own types loads the header and is trusted only when every declaration it describes was parsed from
// the text of a program file. The tests count operations and compare answers; none reads a clock.

// pchTypesHeader is declsHeader and the shapes that a location can hide in: a struct made by a macro (the
// location is a macro expansion) and templates of the program's own header instantiated with library types.
const pchTypesHeader = declsHeader + `
#define GP_POINT(NAME) struct NAME { int x, y; }
GP_POINT(MacroPoint);
struct Holder : std::string { int extra; std::vector<int> items; };
`

// pchTypeQueries are the types of pchTypesHeader that a program could ask for, and two of the library's.
func pchTypeQueries() []DeclQuery {
	names := []string{
		"geo::Vec", "geo::Shape", "geo::Body", "geo::Kind", "geo::Old", "Rect", "Pt", "U", "Opaque",
		"Box", "Id", "T", "nn::T", "Nest", "Mixed", "TD1", "TD2", "TD3", "TD4", "TD5", "Deep",
		"Wrap<int>", "Buf<char, 4>", "Wrap<std::string>", "MacroPoint", "Holder",
		"std::vector<geo::Vec>", "std::string",
	}
	var qs []DeclQuery
	for _, name := range names {
		qs = append(qs, DeclQuery{DeclType, name})
	}
	return qs
}

// typeTable makes the declaration table of the queries on a new cache folder and returns it as the tests
// compare tables (without the timing and the statistics), with the runner that made it.
func typeTable(t *testing.T, dir string, qs []DeclQuery) (string, *DeclTable) {
	t.Helper()
	cfg := clangCacheConfig(t, dir)
	rn := new(Runner)
	tab, err := rn.Decls(context.Background(), cfg, DeclConfig{Includes: []string{`"./decls.hpp"`}}, qs)
	if err != nil {
		t.Fatal(err)
	}
	c := *tab
	c.Timing, c.Stats, c.Cached, c.Runs = Timing{}, Stats{}, false, 0
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), tab
}

// pchTypeRuns is what the type runs of pchTypeQueries answer without the header and with it.
type pchTypeRuns struct {
	plain, withHeader string
	plainTab, tab     *DeclTable
	counters          Counters
	queries           []DeclQuery
}

// runPCHTypes makes the table of pchTypeQueries without the header (checking that none loaded it) and with it.
func runPCHTypes(t *testing.T) pchTypeRuns {
	t.Helper()
	dir := t.TempDir()
	writeHeader(t, dir, "decls.hpp", pchTypesHeader)
	var r pchTypeRuns
	r.queries = pchTypeQueries()
	t.Setenv(EnvPCH, "off")
	ResetCacheCounters()
	r.plain, r.plainTab = typeTable(t, dir, r.queries)
	if c := CacheCounters(); c.PCHUses != 0 {
		t.Fatalf("%d runs loaded a header with %s=off", c.PCHUses, EnvPCH)
	}
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	r.withHeader, r.tab = typeTable(t, dir, r.queries)
	r.counters = CacheCounters()
	return r
}

// The declarations of the program's types are the same with the header and without it.
func TestPCHTypeRunsOfProgramTypesAnswerAsWithout(t *testing.T) {
	r := runPCHTypes(t)
	if r.withHeader != r.plain {
		t.Errorf("the declarations differ with the header:\nwithout: %s\nwith:    %s", r.plain, r.withHeader)
	}
}

// Every run of the program's own types without template arguments loads the header; the names of the
// library and the names with arguments do not.
func TestPCHTypeRunsOfProgramTypesLoadTheHeader(t *testing.T) {
	r := runPCHTypes(t)
	lasts := make(map[string]bool) // (a run is for the names with the same last name: T and nn::T)
	for _, q := range r.queries {
		ty, err := ctype.Parse(q.Name)
		if err != nil || strings.Contains(q.Name, "<") || strings.HasPrefix(q.Name, "std::") {
			continue
		}
		lasts[ty.Last()] = true
	}
	if r.counters.PCHUses != int64(len(lasts)) {
		t.Errorf("%d runs loaded the header, want %d", r.counters.PCHUses, len(lasts))
	}
}

// No run of the program's types is made twice: a declaration that the compiler parsed from the program's
// text has all its members, so its answer is not refused.
func TestPCHTypeRunsOfProgramTypesAreNotMadeAgain(t *testing.T) {
	r := runPCHTypes(t)
	if r.counters.TypeRechecks != 0 {
		t.Errorf("%d type runs were made again without the header (errors: %v)",
			r.counters.TypeRechecks, r.tab.Errors)
	}
}

// The shapes that a location can hide in (a macro, template arguments, a base from the library) have records
// with all their members.
func TestPCHTypeRunsOfProgramTypesHaveAllTheMembers(t *testing.T) {
	r := runPCHTypes(t)
	names := []string{
		"geo::Vec", "Box", "Wrap<std::basic_string<char>>", "Buf<char, 4>", "MacroPoint", "Holder",
	}
	for _, name := range names {
		if r.tab.Records[name] == nil || r.plainTab.Records[name] == nil {
			t.Errorf("no record of %s: %v", name, r.tab.Errors)
		}
	}
	if holder := r.tab.Records["Holder"]; holder == nil || len(holder.Fields) != 2 || len(holder.Bases) != 1 {
		t.Errorf("Holder: %+v", holder)
	}
}

// A run for a name of the library or of its internals does not try the header: its declaration is in it.
func TestPCHTypeRunMayLoadHeader(t *testing.T) {
	cases := []struct {
		names []string
		want  bool
	}{
		{[]string{"geo::Vec"}, true},
		{[]string{"Rect"}, true},
		{[]string{"nn::T", "T"}, true},
		{[]string{"std::vector<int>"}, false},
		{[]string{"std::string"}, false},
		{[]string{"Wrap<int>"}, false},
		{[]string{"geo::Vec", "Buf<char, 4>"}, false},
		{[]string{"_IO_FILE"}, false},
		{[]string{"__gnu_cxx::__normal_iterator<int*, int>"}, false},
		{[]string{"geo::Vec", "std::vector<int>"}, false},
		{[]string{"geo::_Hidden"}, false},
	}
	for _, c := range cases {
		r := new(typeRun)
		for _, name := range c.names {
			ty, err := ctype.Parse(name)
			if err != nil {
				t.Fatal(err)
			}
			r.types = append(r.types, ty)
		}
		if got := r.mayLoadHeader(); got != c.want {
			t.Errorf("%v: mayLoadHeader = %v, want %v", c.names, got, c.want)
		}
	}
}

// A declaration is the program's own when its location is a file that exists and that the header was not made
// of, whatever the spelling of the file name; anything else cannot be told from the header's.
func TestPCHDeclarationsFromProgramFiles(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	sys := filepath.Join(lib, "sys.hpp")
	own := filepath.Join(dir, "own.hpp")
	for _, f := range []string{sys, own} {
		if err := os.WriteFile(f, []byte("// header\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "link.hpp")
	if err := os.Symlink(sys, link); err != nil {
		t.Skipf("no symbolic links: %v", err)
	}
	made := &pchRef{path: filepath.Join(dir, "b.pch"), src: filepath.Join(dir, "b.hpp"), read: []string{sys}}
	cfg := Config{Dir: dir, pch: made}
	at := func(files ...string) *dnode {
		n := &dnode{Loc: &dloc{File: files[0]}}
		if len(files) > 1 {
			n.Loc = &dloc{SpellingLoc: &dloc{File: files[0]}, ExpansionLoc: &dloc{File: files[1]}}
		}
		return n
	}
	cases := []struct {
		name string
		node *dnode
		want bool
	}{
		{"a program file", at(own), true},
		{"a program file, relative to the folder of the run", at("own.hpp"), true},
		{"a macro in a program file", at(own, own), true},
		{"a file of the header", at(sys), false},
		{"a file of the header, spelled another way", at(filepath.Join(lib, "..", "lib", "sys.hpp")), false},
		{"a file of the header, through a link", at(link), false},
		{"a macro of the header expanded in a program file", at(sys, own), false},
		{"a macro of the program expanded in the header", at(own, sys), false},
		{"a file that does not exist", at(filepath.Join(dir, "missing.hpp")), false},
		{"a built-in location", at("<built-in>"), false},
		{"no location", &dnode{}, false},
		{"no node", nil, false},
	}
	for _, c := range cases {
		if got := fromProgramFiles(cfg, c.node); got != c.want {
			t.Errorf("%s: fromProgramFiles = %v, want %v", c.name, got, c.want)
		}
	}
}
