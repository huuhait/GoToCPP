package load

import (
	"path/filepath"
	"testing"
)

// A module from the module cache may have no gocpp.mod at its root (a plain
// C or C++ repository), but a gocpp.mod file below its root starts another
// module, whose packages need a require line of their own (a monorepo).
func TestCachedModuleWithNestedModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"app/gocpp.mod":                  "module example.com/app\nrequire example.org/lib v1.2.3\n",
		"app/main.gp":                    "package main\n\nimport (\n\t\"example.org/lib/pkg\"\n\t\"example.org/lib/sub\"\n)\n\nfunc main() { pkg.F(); sub.G() }\n",
		"cache/lib@v1.2.3/pkg/p.gp":      "package pkg\n\nfunc F() {}\n",
		"cache/lib@v1.2.3/sub/gocpp.mod": "module example.org/lib/sub\n",
		"cache/lib@v1.2.3/sub/s.gp":      "package sub\n\nfunc G() {}\n",
	})
	cfg := Config{ModuleDir: func(path, version string) (string, error) {
		return filepath.Join(root, "cache", "lib@v1.2.3"), nil
	}}
	prog := loadIn(t, cfg, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "# example.com/app\napp/main.gp:5:2: no required module provides package example.org/lib/sub\n")
}
