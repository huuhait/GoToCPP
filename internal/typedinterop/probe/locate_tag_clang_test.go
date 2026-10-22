package probe

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The bodies of typedef'd tags against the real compiler (step 12.13e).

const tagHeader = `#pragma once
// A 2D vector.
typedef struct Vector2 {
    float x;   // Vector x component
    float y;   // Vector y component
} Vector2;

/** Pixels. */
typedef struct {
    unsigned char r;
    unsigned char g;
    unsigned flags : 3;
    int table[4];
    void (*callback)(int, void *);
    const char *label;
    struct Vector2 *origin;
    union {
        int i;
        float f;
    };
    struct {
        int p, q;
    } inner;
} Pixel;

typedef enum { RED, GREEN = 5, BLUE = RED + 7 } Hue;
typedef enum Mode { MODE_A, MODE_B } ModeT;
typedef struct Plain PlainT;
struct Plain { int n; };
typedef struct Fwd Fwd;
typedef void (*Callback)(int, void *);
typedef int Count;
typedef struct Wide {
`

func tagFixture(t *testing.T, fields int) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString(tagHeader)
	for i := range fields {
		b.WriteString("    int f" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ";\n")
	}
	b.WriteString("} Wide;\n")
	writeHeader(t, dir, "tags.h", b.String())
	return dir, []string{`"tags.h"`}
}

func locateTypedefs(t *testing.T, dir string, includes, flags []string, names ...string,
) []Declaration {
	t.Helper()
	cfg := clangConfig(t, dir, flags...)
	cfg.CacheDir = "off"
	runner := new(Runner)
	got, err := runner.Locate(context.Background(), cfg, includes, locateRequestFor(names...))
	if err != nil {
		t.Fatal(err)
	}
	var decls []Declaration
	for _, name := range names {
		decls = append(decls, onlyDecl(t, got.Named("", name)))
	}
	withBodies, err := runner.LocateBodies(context.Background(), cfg, includes, decls)
	if err != nil {
		t.Fatal(err)
	}
	return withBodies
}

func TestLocateBodiesOfTypedefTags(t *testing.T) {
	dir, includes := tagFixture(t, 3)
	names := []string{"Vector2", "Pixel", "Hue", "ModeT", "PlainT", "Fwd", "Callback", "Count"}
	byName := make(map[string]string)
	for i, d := range locateTypedefs(t, dir, includes, nil, names...) {
		byName[names[i]] = d.Signature
	}
	want := map[string]string{
		"Vector2": "typedef struct Vector2 {\n    float x; // Vector x component\n" +
			"    float y; // Vector y component\n} Vector2;",
		"Pixel": "typedef struct {\n    unsigned char r;\n    unsigned char g;\n" +
			"    unsigned int flags : 3;\n" +
			"    int table[4];\n    void (*callback)(int, void *);\n    const char *label;\n" +
			"    struct Vector2 *origin;\n    union {\n        int i;\n        float f;\n    };\n" +
			"    struct {\n        int p;\n        int q;\n    } inner;\n} Pixel;",
		"Hue":      "typedef enum {\n    RED,\n    GREEN = 5,\n    BLUE = 7,\n} Hue;",
		"ModeT":    "typedef enum Mode ModeT;\n\nenum Mode {\n    MODE_A,\n    MODE_B,\n};",
		"PlainT":   "typedef struct Plain PlainT;\n\nstruct Plain {\n    int n;\n};",
		"Fwd":      "typedef struct Fwd Fwd;",
		"Callback": "typedef void (*Callback)(int, void *);",
		"Count":    "typedef int Count;",
	}
	for _, name := range names {
		if byName[name] != want[name] {
			t.Errorf("%s:\n got %q\nwant %q", name, byName[name], want[name])
		}
	}
}

func TestLocateBodiesKeepDocAndPlace(t *testing.T) {
	dir, includes := tagFixture(t, 1)
	d := locateTypedefs(t, dir, includes, nil, "Vector2")[0]
	if d.Doc != "A 2D vector." || d.Kind != "type" || d.Line != 5 {
		t.Errorf("Vector2 = %+v, want the typedef's own doc, kind and place", d)
	}
}

func TestLocateBodiesAreBounded(t *testing.T) {
	dir, includes := tagFixture(t, 60)
	d := locateTypedefs(t, dir, includes, nil, "Wide")[0]
	fields := strings.Count(d.Signature, "\n    int f")
	if fields != 40 || !strings.Contains(d.Signature, "    // ...\n} Wide;") {
		t.Errorf("Wide has %d fields:\n%s", fields, d.Signature)
	}
}

// locateScoped asks for the declaration of the names (scope, name) in a header and adds the bodies
// of their tags.
func locateScoped(t *testing.T, header string, names ...LocateName) []Declaration {
	t.Helper()
	dir := t.TempDir()
	writeHeader(t, dir, "scoped.h", header)
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	runner := new(Runner)
	includes := []string{`"scoped.h"`}
	got, err := runner.Locate(context.Background(), cfg, includes, LocateRequest{Names: names})
	if err != nil {
		t.Fatal(err)
	}
	var decls []Declaration
	for _, name := range names {
		decls = append(decls, onlyDecl(t, got.Named(name.Scope, name.Name)))
	}
	out, err := runner.LocateBodies(context.Background(), cfg, includes, decls)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLocateBodiesOfSameNamedTagsInOtherNamespaces(t *testing.T) {
	const header = `#pragma once
namespace a { struct Thing { int ax; }; }
namespace b { struct Thing { int by; }; typedef struct Thing ThingT; }
namespace c { typedef struct a::Thing AThing; }
typedef struct Thing { int top; } Thing;
`
	got := locateScoped(t, header,
		LocateName{Scope: "b", Name: "ThingT"},
		LocateName{Scope: "c", Name: "AThing"},
		LocateName{Name: "Thing"})
	for i, want := range []string{"int by;", "int ax;", "int top;"} {
		body := got[i].Signature
		if !strings.Contains(body, want) || strings.Count(body, "int ") != 1 {
			t.Errorf("declaration %d = %q, want the one field %q", i, body, want)
		}
	}
}

func TestLocateBodiesCommentsBelongToOneField(t *testing.T) {
	const header = `#pragma once
typedef struct Line {
    int alone;   // one
    int a; int b; // two fields share this line
    int first, second; // both of them
    int last;    /**< last */
} Line;
`
	got := locateScoped(t, header, LocateName{Name: "Line"})[0].Signature
	want := "typedef struct Line {\n    int alone; // one\n    int a;\n    int b;\n" +
		"    int first;\n    int second;\n    int last; // last\n} Line;"
	if got != want {
		t.Errorf("Line:\n got %q\nwant %q", got, want)
	}
}

func TestLocateBodiesLeaveShortNamesPlain(t *testing.T) {
	const header = `#pragma once
typedef struct V2 { float x; float y; } V2;
`
	got := locateScoped(t, header, LocateName{Name: "V2"})[0]
	if got.Tag != nil || strings.Contains(got.Signature, "float x") {
		t.Errorf("V2 = %+v, want the plain typedef (its filter would match most headers)", got)
	}
}

// pkgConfigFlags returns the compiler flags of a library from pkg-config, or skips the test.
func pkgConfigFlags(t *testing.T, library string) []string {
	t.Helper()
	path, err := exec.LookPath("pkg-config")
	if err != nil {
		t.Skip("no pkg-config")
	}
	out, err := exec.Command(path, "--cflags", library).Output()
	if err != nil {
		t.Skipf("pkg-config does not know %s: %v", library, err)
	}
	return strings.Fields(string(out))
}

// TestLocateBodiesOfRaylibTypes is the case that started the step: raylib's Vector2 and Color
// are `typedef struct Vector2 {...} Vector2;`, and the library's own header must give their fields.
func TestLocateBodiesOfRaylibTypes(t *testing.T) {
	flags := pkgConfigFlags(t, "raylib")
	dir := t.TempDir()
	names := []string{"Vector2", "Color", "Rectangle", "ConfigFlags", "TraceLogCallback"}
	decls := locateTypedefs(t, dir, []string{"<raylib.h>"}, flags, names...)
	got := make(map[string]string)
	for i, d := range decls {
		got[names[i]] = d.Signature
	}
	for name, want := range map[string]string{
		"Vector2": "typedef struct Vector2 {\n    float x; // Vector x component\n" +
			"    float y; // Vector y component\n} Vector2;",
		"Color": "typedef struct Color {\n    unsigned char r; // Color red value\n" +
			"    unsigned char g; // Color green value\n" +
			"    unsigned char b; // Color blue value\n" +
			"    unsigned char a; // Color alpha value\n} Color;",
		"Rectangle": "typedef struct Rectangle {\n" +
			"    float x; // Rectangle top-left corner position x\n" +
			"    float y; // Rectangle top-left corner position y\n" +
			"    float width; // Rectangle width\n" +
			"    float height; // Rectangle height\n} Rectangle;",
	} {
		if got[name] != want {
			t.Errorf("%s:\n got %q\nwant %q", name, got[name], want)
		}
	}
	sig := got["ConfigFlags"]
	if !strings.HasPrefix(sig, "typedef enum {\n    FLAG_VSYNC_HINT = 64,\n") ||
		!strings.Contains(sig, "FLAG_MSAA_4X_HINT = 32") ||
		!strings.HasSuffix(sig, "} ConfigFlags;") {
		t.Errorf("ConfigFlags = %q", sig)
	}
	if !strings.HasPrefix(got["TraceLogCallback"], "typedef void (*TraceLogCallback)(") ||
		strings.Contains(got["TraceLogCallback"], "{") {
		t.Errorf("TraceLogCallback = %q, want the plain function pointer typedef",
			got["TraceLogCallback"])
	}
}

func TestLocateMarksTheDefinitionsThatRedeclareTheHeader(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "h.h", "#pragma once\n#include <stddef.h>\nint ovr(int);\nint lone(int);\n"+
		"int real(int);\nextern int table[];\nsize_t width(size_t);\n")
	writeHeader(t, dir, "h.cpp", "#include \"h.h\"\nstatic int ovr(char c) { return c; }\n"+
		"namespace { int lone(double d) { return 1; } }\nint real(int x) { return x; }\n"+
		"int table[3] = {1, 2, 3};\nunsigned long width(unsigned long n) { return n; }\n")
	source := `"` + filepath.Join(dir, "h.cpp") + `"`
	got := locateReal(t, dir, []string{`"h.h"`, source},
		LocateRequest{Names: locateNames("", "ovr", "lone", "real", "table", "width")})
	redeclared := func(name string) []Declaration {
		var out []Declaration
		for _, d := range got.Named("", name) {
			if d.Defined && d.Redeclares {
				out = append(out, d)
			}
		}
		return out
	}
	for _, name := range []string{"ovr", "lone"} {
		if out := redeclared(name); len(out) != 0 {
			t.Errorf("%s: %+v, want no definition that redeclares the header's function", name, out)
		}
		if len(got.Named("", name)) < 2 {
			t.Errorf("%s: %+v, want the header's declaration and the helper",
				name, got.Named("", name))
		}
	}
	for _, name := range []string{"real", "table", "width"} {
		if out := redeclared(name); len(out) != 1 || filepath.Base(out[0].File) != "h.cpp" {
			t.Errorf("%s: %+v, want the definition of h.cpp", name, out)
		}
	}
}
