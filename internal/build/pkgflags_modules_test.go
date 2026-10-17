package build_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestReadPackageFlagsModuleDirs checks that the folders of the modules whose
// headers a package imports are -iquote compile flags after those of the
// directives (Phase 06 step 6.1b), and that a package without such headers
// gets none.
func TestReadPackageFlagsModuleDirs(t *testing.T) {
	lp, prog := loadPkg(t, map[string]string{"main.gp": "//gocpp:cflags -DX\npackage main\n\nfunc main() {}\n"})
	flags, _, _ := build.ReadPackageFlags(context.Background(), prog.Fset, lp, nil)
	if !slices.Equal(flags.Compile, []string{"-DX"}) {
		t.Errorf("without module headers: %q", flags.Compile)
	}
	dir1, dir2 := filepath.Join(lp.Dir, "a"), filepath.Join(lp.Dir, "b c")
	lp.IncludeDirs = []string{dir1, dir2}
	flags, _, _ = build.ReadPackageFlags(context.Background(), prog.Fset, lp, nil)
	if want := []string{"-DX", "-iquote", dir1, "-iquote", dir2}; !slices.Equal(flags.Compile, want) {
		t.Errorf("with module headers: %q, want %q", flags.Compile, want)
	}
	if !slices.Equal(build.ProbeFlags(build.EnvFlags{}, flags), slices.Concat(flags.Compile, build.LanguageFlags())) {
		t.Errorf("the probe flags don't have the include folders")
	}
}
