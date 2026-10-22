package probe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The header declares sp, sp2, g2, sz and v; the source defines functions of the same names that
// are other functions, and some that are the header's.
const chainHeader = `typedef unsigned long my_t;
int sp(int);
int sp2(int);
int g2(int);
my_t sz(my_t, int*);
extern int v;
int va(int);
int va(int, ...);
`

const chainSource = `static int sp(char c);
static int sp(char c) { return c; }
namespace { int sp2(double); int sp2(double d) { return 1; } }
int g2(int a) { return a; }
int g2(int a);
unsigned long sz(unsigned long n, int* p) { return n; }
int v = 3;
extern int v;
int va(int n, ...) { return n; }
`

func chainLocations(t *testing.T) (*Locations, *Runner, Config) {
	t.Helper()
	dir := t.TempDir()
	writeHeader(t, dir, "api.hpp", chainHeader)
	writeHeader(t, dir, "src.cpp", chainSource)
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	rn := new(Runner)
	got, err := rn.Locate(context.Background(), cfg, chainIncludes,
		LocateRequest{Names: locateNames("", "sp", "sp2", "g2", "sz", "v", "va")})
	if err != nil {
		t.Fatal(err)
	}
	return got, rn, cfg
}

var chainIncludes = []string{`"api.hpp"`, `"src.cpp"`}

func inFile(decls []Declaration, base string) []Declaration {
	var out []Declaration
	for _, d := range decls {
		if strings.HasSuffix(d.File, base) {
			out = append(out, d)
		}
	}
	return out
}

func TestLocateStaticFunctionDoesNotRedeclareTheHeaders(t *testing.T) {
	got, _, _ := chainLocations(t)
	// `static int sp(char); static int sp(char) {...}`: the definition has a previous
	// declaration, in the source itself.
	def := onlyDecl(t, inFile(got.Named("", "sp"), "src.cpp"))
	if !def.Defined || def.Redeclares || def.Params != "char" {
		t.Errorf("static sp = %+v, want a definition that redeclares nothing, with the "+
			"parameter char", def)
	}
	head := onlyDecl(t, inFile(got.Named("", "sp"), "api.hpp"))
	if head.Params != "int" || head.Redeclares {
		t.Errorf("sp in the header = %+v", head)
	}
}

func TestLocateParametersTellOverloadsAndTypedefsApart(t *testing.T) {
	got, rn, cfg := chainLocations(t)
	t.Run("a function of an unnamed namespace has its own parameters", func(t *testing.T) {
		def := onlyDecl(t, inFile(got.Named("", "sp2"), "src.cpp"))
		head := onlyDecl(t, inFile(got.Named("", "sp2"), "api.hpp"))
		if !def.Defined || def.Params != "double" || head.Params != "int" {
			t.Errorf("definition %+v, header %+v, want double against int", def, head)
		}
	})
	t.Run("a variadic function is not the one without the dots", func(t *testing.T) {
		def := onlyDecl(t, inFile(got.Named("", "va"), "src.cpp"))
		if !def.Defined || def.Params != "int;..." {
			t.Errorf("va = %+v, want the parameters int and the dots", def)
		}
		// (the header's int, ... is shown as the source's definition, which is its latest
		// declaration)
		head := onlyDecl(t, inFile(got.Named("", "va"), "api.hpp"))
		if head.Params != "int" {
			t.Errorf("va(int) in the header = %+v", head)
		}
	})
	t.Run("a typedef name is the type under it", func(t *testing.T) {
		def := onlyDecl(t, inFile(got.Named("", "sz"), "src.cpp"))
		if !def.Defined || !def.Redeclares || def.Params != "unsigned long;int *" {
			t.Errorf("sz = %+v, want the canonical parameter types in order", def)
		}
		if def.Type != "unsigned long (unsigned long, int *)" {
			t.Errorf("sz type = %q", def.Type)
		}
		// the header spells my_t, in the type and in the parameters
		only, err := rn.Locate(context.Background(), cfg, chainIncludes[:1],
			LocateRequest{Names: locateNames("", "sz")})
		if err != nil {
			t.Fatal(err)
		}
		head := onlyDecl(t, only.Named("", "sz"))
		if head.Type != "my_t (my_t, int *)" || head.Params != def.Params {
			t.Errorf("sz in the header = %q with parameters %q, want my_t in the type and the "+
				"canonical types in the parameters", head.Type, head.Params)
		}
	})
}

func TestLocateLatestRedeclarationHidesTheDefinition(t *testing.T) {
	got, _, _ := chainLocations(t)
	for _, name := range []string{"g2", "v"} {
		latest := onlyDecl(t, inFile(got.Named("", name), "src.cpp"))
		if latest.Defined || !latest.Redeclares {
			t.Errorf("%s = %+v, want the lookup to show the redeclaration without the body",
				name, latest)
		}
	}
}

func TestLocateChainFindsTheDefinitionBeforeARedeclaration(t *testing.T) {
	_, rn, cfg := chainLocations(t)
	for _, c := range []struct {
		name  string
		body  int // the line (from 1) of the body in src.cpp
		count int
	}{
		{"g2", 4, 3},
		{"v", 7, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			chain, err := rn.LocateChain(context.Background(), cfg, chainIncludes, "", c.name)
			if err != nil {
				t.Fatal(err)
			}
			if len(chain) != c.count {
				t.Fatalf("chain = %+v, want the header's declaration and the two of the source",
					chain)
			}
			var defined []Declaration
			for _, d := range chain {
				if d.Defined {
					defined = append(defined, d)
				}
			}
			if len(defined) != 1 || !defined[0].Redeclares || defined[0].Line != c.body-1 ||
				!strings.HasSuffix(defined[0].File, "src.cpp") {
				t.Errorf("defined = %+v, want the one with the body, at line %d", defined, c.body)
			}
		})
	}
	t.Run("a static function is a chain of its own", func(t *testing.T) {
		chain, err := rn.LocateChain(context.Background(), cfg, chainIncludes, "", "sp")
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range inFile(chain, "src.cpp") {
			if d.Redeclares {
				t.Errorf("static declaration %+v redeclares the header's function", d)
			}
		}
	})
}

func TestLocateChainRefusesWhatItCannotAsk(t *testing.T) {
	_, rn, cfg := chainLocations(t)
	ctx := context.Background()
	if _, err := rn.LocateChain(ctx, cfg, chainIncludes, "", "not a name"); err == nil {
		t.Error("an invalid name was asked")
	}
	cfg.CXX.Family = build.GCC
	_, err := rn.LocateChain(ctx, cfg, chainIncludes, "", "g2")
	if !errors.Is(err, ErrNoClang) {
		t.Errorf("a compiler that is not clang: %v", err)
	}
}
