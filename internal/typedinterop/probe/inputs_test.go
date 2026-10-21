package probe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestComposedInputsRejectChangedHeader(t *testing.T) {
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Before {};\n")
	guard, err := CaptureInputs(context.Background(), fake.cfg(), []string{`"api.hpp"`})
	if err != nil || !guard.Unchanged(context.Background()) {
		t.Fatalf("initial input guard = %v", err)
	}
	fake.header(t, "api.hpp", "struct After {};\n")
	if guard.Unchanged(context.Background()) {
		t.Fatal("composed probes accepted different header contents")
	}
}

func TestComposedInputsRejectRequiredDeletion(t *testing.T) {
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Before {};\n")
	guard, err := CaptureInputs(context.Background(), fake.cfg(), []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fake.inc, "api.hpp")); err != nil {
		t.Fatal(err)
	}
	if guard.Unchanged(context.Background()) {
		t.Fatal("composed probes accepted a deleted required header")
	}
}

func TestComposedInputsCompilerSearchShadow(t *testing.T) {
	for _, mode := range []string{"wrapper", "args"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "wrapper" && runtime.GOOS == "windows" {
				t.Skip("test wrapper uses a POSIX shell")
			}
			defer SetRacyWindow(0)()
			dir := t.TempDir()
			early := t.TempDir()
			late := t.TempDir()
			const oldHeader = "struct First { int oldName; }; struct Second {}; First make();\n"
			if err := os.WriteFile(filepath.Join(late, "api.hpp"), []byte(oldHeader), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := clangConfig(t, dir)
			cfg.CacheDir = "off"
			if mode == "wrapper" {
				wrapper := filepath.Join(dir, "clang-wrapper")
				script := fmt.Sprintf("#!/bin/sh\nexec %q -I%q -I%q \"$@\"\n", cfg.CXX.Path, early, late)
				if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GOCPP_CLANG", wrapper)
				var err error
				cfg.CXX, err = FindClang(context.Background())
				if err != nil {
					t.Fatal(err)
				}
			} else {
				cfg.CXX.Args = append(cfg.CXX.Args, "-I"+early, "-I"+late)
			}
			guard, err := CaptureInputs(context.Background(), cfg, []string{"<api.hpp>"})
			if err != nil || !guard.Unchanged(context.Background()) {
				t.Fatalf("initial input guard = %v", err)
			}
			// The type stage proves First using the late header. A new early header keeps
			// First usable but changes make's result, so per-query checks alone cannot join them.
			proof, err := runAux(context.Background(), cfg, []string{"-fsyntax-only"},
				"#include <api.hpp>\nstatic_assert(__is_same(decltype(make()), First));\n")
			if err != nil || proof.exitErr != nil {
				t.Fatalf("receiver proof = %v, %v", proof, err)
			}
			const newHeader = "struct First { int newName; }; struct Second {}; Second make();\n"
			if err := os.WriteFile(filepath.Join(early, "api.hpp"), []byte(newHeader), 0o600); err != nil {
				t.Fatal(err)
			}
			members, err := new(Runner).Members(context.Background(), cfg, []string{"<api.hpp>"},
				MemberQuery{Type: "First", Category: CatLValue})
			if err != nil || !slices.Equal(members.Names(), []string{"newName"}) {
				t.Fatalf("new generation members = %v, %v", members, err)
			}
			if guard.Unchanged(context.Background()) {
				t.Fatal("composed stages accepted a new earlier compiler-search-path header")
			}
		})
	}
}

func TestComposedInputsRelativeCompilerSearchShadow(t *testing.T) {
	for _, mode := range []string{"wrapper", "args"} {
		for _, spelling := range []string{"plain", "dot", "parent", "symlink-parent"} {
			t.Run(mode+"/"+spelling, func(t *testing.T) {
				if mode == "wrapper" && runtime.GOOS == "windows" {
					t.Skip("test wrapper uses a POSIX shell")
				}
				defer SetRacyWindow(0)()
				root := t.TempDir()
				early := filepath.Join(root, "early")
				late := filepath.Join(root, "late")
				for _, dir := range []string{early, late, filepath.Join(root, "walk")} {
					if err := os.Mkdir(dir, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				dir := root
				earlyArg := "early"
				lateArg := "late"
				switch spelling {
				case "dot":
					earlyArg = "./walk/../early"
					lateArg = "./walk/../late"
				case "parent":
					dir = filepath.Join(root, "walk")
					earlyArg = "../early"
					lateArg = "../late"
				case "symlink-parent":
					if runtime.GOOS == "windows" {
						t.Skip("symlink test needs local symlink support")
					}
					physical := t.TempDir()
					early = filepath.Join(physical, "early")
					late = filepath.Join(physical, "late")
					child := filepath.Join(physical, "child")
					for _, path := range []string{early, late, child} {
						if err := os.Mkdir(path, 0o700); err != nil {
							t.Fatal(err)
						}
					}
					walk := filepath.Join(root, "walk")
					if err := os.Remove(walk); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(child, walk); err != nil {
						t.Fatal(err)
					}
					earlyArg = "./walk/../early"
					lateArg = "./walk/../late"
				}
				if err := os.WriteFile(filepath.Join(late, "api.hpp"),
					[]byte("struct First {};\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				cfg := clangConfig(t, dir)
				cfg.CacheDir = "off"
				if mode == "wrapper" {
					wrapper := filepath.Join(root, "clang-wrapper")
					script := fmt.Sprintf("#!/bin/sh\nexec %q -I%q -I%q \"$@\"\n",
						cfg.CXX.Path, earlyArg, lateArg)
					if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
						t.Fatal(err)
					}
					t.Setenv("GOCPP_CLANG", wrapper)
					var err error
					cfg.CXX, err = FindClang(context.Background())
					if err != nil {
						t.Fatal(err)
					}
				} else {
					cfg.CXX.Args = append(cfg.CXX.Args, "-I"+earlyArg, "-I"+lateArg)
				}
				// A process-relative Abs(searchDir) would now target the wrong directory.
				t.Chdir(t.TempDir())
				guard, err := CaptureInputs(context.Background(), cfg, []string{"<api.hpp>"})
				if err != nil || !guard.Unchanged(context.Background()) {
					t.Fatalf("initial relative guard = %v", err)
				}
				shadow := filepath.Join(early, "api.hpp")
				watched := slices.ContainsFunc(guard.deps, func(path string) bool {
					if !filepath.IsAbs(path) || filepath.Base(path) != "api.hpp" {
						return false
					}
					parent, err := filepath.EvalSymlinks(dirRaw(path))
					return err == nil && parent == early
				})
				if !watched {
					t.Fatalf("missing absolute shadow path %s in %v", shadow, guard.deps)
				}
				if err := os.WriteFile(shadow, []byte("struct Second {};\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if guard.Unchanged(context.Background()) {
					t.Fatal("relative compiler search root lost its new earlier header")
				}
			})
		}
	}
}

func TestComposedInputsKeepRawDirectorySpellings(t *testing.T) {
	stderr := []byte("#include <...> search starts here:\n ./link/../early\nEnd of search list.\n")
	if got := rawSearchDirs(stderr); !slices.Equal(got, []string{"./link/../early"}) {
		t.Fatalf("strict discovery changed traversal: %v", got)
	}
	if got := parseSearchDirs(stderr); !slices.Equal(got, []string{"early"}) {
		t.Fatalf("legacy parser changed its normalization: %v", got)
	}
}

func TestComposedInputsRejectFailedSearchDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Value {};\n")
	cfg := fake.cfg()
	wrapper := filepath.Join(t.TempDir(), "failed-search")
	script := fmt.Sprintf("#!/bin/sh\n%q \"$@\"\nstatus=$?\nfor arg do\n  [ \"$arg\" = -v ] && exit 2\ndone\nexit $status\n", cfg.CXX.Path)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.CXX.Path = wrapper
	dirs, err := new(Runner).searchDirsChecked(context.Background(), cfg)
	if err == nil || len(dirs) == 0 {
		t.Fatalf("failed search status was lost: %v, %v", dirs, err)
	}
	if dirs := new(Runner).searchDirs(context.Background(), cfg); len(dirs) == 0 {
		t.Fatal("legacy best-effort search lost useful paths")
	}
	if _, err := CaptureInputs(context.Background(), cfg, []string{`"api.hpp"`}); err == nil {
		t.Fatal("input guard accepted failed search-path discovery")
	}
}

func TestComposedInputsHonorCancellation(t *testing.T) {
	fake := newFake(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CaptureInputs(ctx, fake.cfg(), []string{`"api.hpp"`}); err == nil {
		t.Fatal("canceled input discovery succeeded")
	}
}
