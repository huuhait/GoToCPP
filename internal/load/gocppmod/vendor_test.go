package gocppmod

import (
	"reflect"
	"testing"
)

const vendorText = `# example.com/a v1.2.3
## explicit; gocpp 0.1
example.com/a
example.com/a/sub
# example.com/b v0.1.0 => ../b
## explicit
example.com/b
# example.com/c v2.0.0
example.com/c
`

func TestParseVendor(t *testing.T) {
	v, errs := ParseVendor("vendor/modules.txt", []byte(vendorText))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(v.Modules) != 3 {
		t.Fatalf("modules = %+v", v.Modules)
	}
	a, b, c := v.Modules[0], v.Modules[1], v.Modules[2]
	if a.Path != "example.com/a" || a.Version != "v1.2.3" || !a.Explicit || a.Replaced ||
		!reflect.DeepEqual(a.Packages, []string{"example.com/a", "example.com/a/sub"}) || a.Line != 1 {
		t.Errorf("a = %+v", a)
	}
	if !b.Replaced || b.NewPath != "../b" || b.NewVersion != "" || !b.Explicit {
		t.Errorf("b = %+v", b)
	}
	if c.Explicit || c.Replaced || v.Module("example.com/c") != c || v.Module("example.com/zz") != nil {
		t.Errorf("c = %+v", c)
	}
}

func TestParseVendorErrors(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"example.com/a\n", "m.txt:1: malformed line: package before the first module: example.com/a"},
		{"## explicit\n", "m.txt:1: malformed line: annotation before the first module: ## explicit"},
		{"# a\n", "m.txt:1: malformed line: # a"},
		{"# a v1 b\n", "m.txt:1: malformed line: # a v1 b"},
		{"# a v1\n", `m.txt:1: malformed line: invalid module version "v1": # a v1`},
		{"# => 0\n", "m.txt:1: malformed line: # => 0"},
		{"# a v1.0.0 =>\n", "m.txt:1: malformed line: # a v1.0.0 =>"},
		{"# a v1.0.0 => b\n", "m.txt:1: malformed line: the replacement of a must be a folder, or a module with a version: # a v1.0.0 => b"},
		{"# a v1.0.0 => ../b v1.0.0\n", "m.txt:1: malformed line: the replacement of a must be a folder, or a module with a version: # a v1.0.0 => ../b v1.0.0"},
	} {
		_, errs := ParseVendor("m.txt", []byte(tc.in))
		if got := errs.Error(); got != tc.want {
			t.Errorf("%q: errors\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
	// A comment, blank lines and annotations that are not explicit are fine.
	if _, errs := ParseVendor("m.txt", []byte("#comment\n\n# a v1.0.0\n## go 1.20\n")); len(errs) > 0 {
		t.Error(errs)
	}
}

func TestCheckVendor(t *testing.T) {
	const mod = "module m\nrequire example.com/a v1.2.3\nrequire example.com/b v0.1.0\nreplace example.com/b => ../b\n"
	for _, tc := range []struct {
		name, mod, vendor string
		want              []string
	}{
		{"consistent", mod, vendorText[:len(vendorText)-len("# example.com/c v2.0.0\nexample.com/c\n")], nil},
		{"module not vendored", mod, "# example.com/b v0.1.0 => ../b\n## explicit\n", []string{
			"example.com/a@v1.2.3: is explicitly required in gocpp.mod, but not marked as explicit in vendor/modules.txt"}},
		{"not marked explicit", mod, "# example.com/a v1.2.3\n# example.com/b v0.1.0 => ../b\n## explicit\n", []string{
			"example.com/a@v1.2.3: is explicitly required in gocpp.mod, but not marked as explicit in vendor/modules.txt"}},
		{"other version", mod, "# example.com/a v1.0.0\n## explicit\n# example.com/b v0.1.0 => ../b\n## explicit\n", []string{
			"example.com/a@v1.2.3: is required in gocpp.mod, but vendor/modules.txt has version v1.0.0",
			"example.com/a@v1.0.0: is marked as explicit in vendor/modules.txt, but not explicitly required in gocpp.mod"}},
		{"explicit but not required", mod, vendorText[:len(vendorText)-len("# example.com/c v2.0.0\nexample.com/c\n")] +
			"# example.com/d v1.0.0\n## explicit\n", []string{
			"example.com/d@v1.0.0: is marked as explicit in vendor/modules.txt, but not explicitly required in gocpp.mod"}},
		{"replace not marked", mod, "# example.com/a v1.2.3\n## explicit\n# example.com/b v0.1.0\n## explicit\n", []string{
			"example.com/b@v0.1.0: is replaced in gocpp.mod, but not marked as replaced in vendor/modules.txt"}},
		{"replace differs", mod, "# example.com/a v1.2.3\n## explicit\n# example.com/b v0.1.0 => ../other\n## explicit\n", []string{
			"example.com/b@v0.1.0: is replaced by ../b in gocpp.mod, but marked as replaced by ../other in vendor/modules.txt"}},
		{"replaced only in vendor", "module m\nrequire example.com/a v1.2.3\n", "# example.com/a v1.2.3 => ../a\n## explicit\n", []string{
			"example.com/a@v1.2.3: is marked as replaced by ../a in vendor/modules.txt, but not replaced in gocpp.mod"}},
		{"versioned replace", "module m\nrequire example.com/a v1.2.3\nreplace example.com/a v1.2.3 => ../a\n", "# example.com/a v1.2.3 => ../a\n## explicit\n", nil},
		{"replace of a version not required", "module m\nrequire example.com/a v1.2.3\nreplace example.com/a v1.0.0 => ../a\n", "# example.com/a v1.2.3\n## explicit\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mustParse(t, tc.mod)
			v, errs := ParseVendor("m.txt", []byte(tc.vendor))
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			if got := CheckVendor(f, v); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("problems:\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}
