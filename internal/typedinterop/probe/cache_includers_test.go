package probe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// Tests of the headers that a cached answer depends on, with the real clang: a header that is created
// later in a place the compiler looks in before the one it found must change the answer, and the cache must
// give the answer a run without cache gives. Each header defines LIBV; the probe asks for its value.

// libHeader writes a header with an old modification time (so that it is not a racy one).
func libHeader(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// libValue returns the value of LIBV after including include (like `<lib.hpp>`), the number of header errors
// and the number of probe runs the Runner made.
func libValue(t *testing.T, cfg Config, include string) (val string, herrs, runs int) {
	t.Helper()
	r := ref(pth("LIBV"))
	r.WantValue = true
	rn := new(Runner)
	tab, err := rn.Probe(context.Background(), cfg, collect.NewSet(r), GenConfig{Includes: []string{include}, Kinds: map[string]interop.Kind{"LIBV": interop.Macro}})
	if err != nil {
		t.Fatal(err)
	}
	val = "?"
	if v := byKey(tab, "::LIBV"); v != nil && v.Value != nil {
		val = v.Value.String()
	} else if v != nil {
		val = "err:" + v.Error
	}
	return val, len(tab.HeaderErrors), rn.Runs()
}

// libWant checks that the answer with the cache is wantVal and the one of a run without cache (the truth)
// is the same. A new Runner each time, so that only the disk cache can serve it.
func libWant(t *testing.T, what string, cfg Config, include, wantVal string) {
	t.Helper()
	v, h, _ := libValue(t, cfg, include)
	off := cfg
	off.CacheDir = "off"
	v2, h2, _ := libValue(t, off, include)
	if v != v2 || h != h2 {
		t.Errorf("%s: STALE: cached %s/%d header errors, truth %s/%d", what, v, h, v2, h2)
	}
	if v2 != wantVal {
		t.Errorf("%s: truth is %s, the test expected %s", what, v2, wantVal)
	}
}

// The package folder is searched for every quoted include, also one of a header found elsewhere
// (-iquote, which the probe adds itself); a header created there hides the one found in a -I folder.
func TestCacheShadowInPackageFolder(t *testing.T) {
	t.Run("nested_include", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/inc2")
		libHeader(t, d+"/inc/lib.hpp", "#include \"dep.h\"\n")
		libHeader(t, d+"/inc2/dep.h", "#define LIBV 1\n")
		libWant(t, "dep.h in inc2", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/dep.h", "#define LIBV 3\n")
		libWant(t, "dep.h created in the package folder", cfg, `<lib.hpp>`, "3")
	})
	t.Run("minus_include_flag", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/leafdir/leaf.h", "#define LIBV 1\n")
		libHeader(t, d+"/inc/lib.hpp", "// nothing\n")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leafdir", "-include", "leaf.h")
		libWant(t, "-include leaf.h found in a -I folder", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/leaf.h", "#define LIBV 2\n")
		libWant(t, "leaf.h created in the package folder", cfg, `<lib.hpp>`, "2")
	})
	t.Run("minus_include_flag_earlier_I", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/leafdir/leaf.h", "#define LIBV 1\n")
		libHeader(t, d+"/inc/lib.hpp", "// nothing\n")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leafdir", "-include", "leaf.h")
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/inc/leaf.h", "#define LIBV 3\n")
		libWant(t, "leaf.h created in an earlier -I folder", cfg, `<lib.hpp>`, "3")
	})
}

// A quoted include looks in the folder of the file that has it first. The folder of the includer is
// found from the include tree the preprocessor prints, so every way to write an include is covered: a
// header created next to the includer must hide the one that was found.
func TestCacheShadowNextToIncluder(t *testing.T) {
	forms := []struct{ name, body, after string }{
		{"plain", "#include \"leaf.h\"\n", "2"},
		{"space_after_hash", "#  include \"leaf.h\"\n", "2"},
		{"tab", "#\tinclude\t\"leaf.h\"\n", "2"},
		{"continued_after_keyword", "#include \\\n\"leaf.h\"\n", "2"},
		{"continued_in_keyword", "#inc\\\nlude \"leaf.h\"\n", "2"},
		{"digraph", "%:include \"leaf.h\"\n", "2"},
		{"name_from_a_macro", "#define H \"leaf.h\"\n#include H\n", "2"},
		{"name_from_stringify", "#define STR(x) #x\n#define XSTR(x) STR(x)\n#define NAME leaf.h\n#include XSTR(NAME)\n", "2"},
		{"after_a_comment", "/* c */ #include \"leaf.h\"\n", "2"},
		{"comment_between", "# /* c */ include \"leaf.h\"\n", "2"},
		{"guarded_by_has_include", "#if __has_include(\"leaf.h\")\n#include \"leaf.h\"\n#endif\n", "2"},
		{"import", "#import \"leaf.h\"\n", "2"},
		{"crlf", "#include \"leaf.h\"\r\n", "2"},
		{"include_next", "#include_next \"leaf.h\"\n", "1"}, // (starts after the includer's folder: no effect)
	}
	for _, f := range forms {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			d := t.TempDir()
			cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leafdir")
			libHeader(t, d+"/inc/sub/lib.hpp", f.body)
			libHeader(t, d+"/leafdir/leaf.h", "#define LIBV 1\n")
			libWant(t, "base", cfg, `<sub/lib.hpp>`, "1")
			libWant(t, "again", cfg, `<sub/lib.hpp>`, "1")
			libHeader(t, d+"/inc/sub/leaf.h", "#define LIBV 2\n")
			libWant(t, "leaf.h created next to the includer", cfg, `<sub/lib.hpp>`, f.after)
		})
	}
	// A header with a guard that two files include: the second include is skipped, but still looks
	// in the folder of the second file first.
	t.Run("skipped_second_include", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leafdir")
		libHeader(t, d+"/inc/a/lib.hpp", "#pragma once\n#include \"leaf.h\"\n#include \"../b/two.h\"\n")
		libHeader(t, d+"/inc/b/two.h", "#include \"leaf.h\"\n")
		libHeader(t, d+"/leafdir/leaf.h", "#pragma once\n#define LIBV 1\n")
		libWant(t, "base", cfg, `<a/lib.hpp>`, "1")
		libWant(t, "again", cfg, `<a/lib.hpp>`, "1")
		libHeader(t, d+"/inc/b/leaf.h", "#define LIBV 2\n")
		libWant(t, "leaf.h created next to the second includer", cfg, `<a/lib.hpp>`, "2")
	})
	t.Run("two_levels_deep_missing", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d)
		libHeader(t, d+"/lib.hpp", "#include \"a.hpp\"\n")
		libHeader(t, d+"/a.hpp", "#include \"b.hpp\"\n")
		if v, h, _ := libValue(t, cfg, `"lib.hpp"`); !strings.HasPrefix(v, "err:") || h != 1 {
			t.Fatalf("%s/%d", v, h)
		}
		libHeader(t, d+"/b.hpp", "#define LIBV 5\n")
		libWant(t, "b.hpp appears next to a.hpp", cfg, `"lib.hpp"`, "5")
	})
	t.Run("nested_from_a_sub_folder", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leafdir")
		libHeader(t, d+"/inc/sub/lib.hpp", "#include \"sub2/dep.h\"\n")
		libHeader(t, d+"/inc/sub/sub2/dep.h", "#include \"leaf.h\"\n")
		libHeader(t, d+"/inc/leaf.h", "#define LIBV 1\n")
		libWant(t, "base", cfg, `<sub/lib.hpp>`, "1")
		libHeader(t, d+"/inc/sub/sub2/leaf.h", "#define LIBV 2\n")
		libWant(t, "leaf.h created next to the deep includer", cfg, `<sub/lib.hpp>`, "2")
	})
}

// A header created in a folder of the search that is earlier than the one that had it, whatever flag names
// the folder (-idirafter separate or joined, -isystem, -iquote joined).
func TestCacheShadowFlagForms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags func(d string) []string
	}{
		{"idirafter_separate", func(d string) []string { return []string{"-idirafter", d + "/after", "-isystem", d + "/sys"} }},
		{"idirafter_joined", func(d string) []string { return []string{"-idirafter" + d + "/after", "-isystem" + d + "/sys"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			libHeader(t, d+"/inc/lib.hpp", "#include <dep.h>\n")
			libHeader(t, d+"/after/dep.h", "#define LIBV 1\n")
			cfg := clangCacheConfig(t, d, append([]string{"-I" + d + "/inc"}, tc.flags(d)...)...)
			libWant(t, "dep.h in -idirafter", cfg, `<lib.hpp>`, "1")
			libWant(t, "again", cfg, `<lib.hpp>`, "1")
			libHeader(t, d+"/sys/dep.h", "#define LIBV 6\n")
			libWant(t, "dep.h created in -isystem (before -idirafter)", cfg, `<lib.hpp>`, "6")
			libHeader(t, d+"/inc/dep.h", "#define LIBV 7\n")
			libWant(t, "dep.h created in -I", cfg, `<lib.hpp>`, "7")
		})
	}
	t.Run("iquote_joined", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/inc/lib.hpp", "#include \"dep.h\"\n")
		libHeader(t, d+"/inc2/dep.h", "#define LIBV 1\n")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/inc2", "-iquote"+d+"/q")
		libWant(t, "dep.h in inc", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/q/dep.h", "#define LIBV 8\n")
		libWant(t, "dep.h created in -iquote", cfg, `<lib.hpp>`, "8")
	})
}

// The race window of the cache is two seconds unless a test changes it (a header that was written within
// it may have been read half-written, and its result is not kept); TestMain makes it 0 for the other tests.
func TestRacyWindowDefault(t *testing.T) {
	if initialRacyWindow != 2*time.Second {
		t.Errorf("the default racy window is %v", initialRacyWindow)
	}
	restore := SetRacyWindow(5 * time.Second)
	if racyWindow != 5*time.Second {
		t.Errorf("SetRacyWindow: %v", racyWindow)
	}
	restore()
	if racyWindow != 0 {
		t.Errorf("not restored: %v", racyWindow)
	}
}

// A name with `..` (or `./`) is looked up in the folder of the file that has it as it is spelled: a header
// created at dir(includer)/../foo.h hides the one that another folder had, and one that is missing is looked
// for at the same place.
func TestCacheShadowDotDotNames(t *testing.T) {
	t.Run("found_via_I", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/a/b", "-I"+d+"/inc/x")
		libHeader(t, d+"/a/b/lib.hpp", "#include \"../foo.h\"\n")
		libHeader(t, d+"/inc/foo.h", "#define LIBV 1\n")
		os.MkdirAll(d+"/inc/x", 0o755)
		libWant(t, "foo.h found as inc/x/../foo.h", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/a/foo.h", "#define LIBV 2\n")
		libWant(t, "foo.h created at includer/../foo.h", cfg, `<lib.hpp>`, "2")
	})
	t.Run("missing_then_created", func(t *testing.T) {
		d := t.TempDir()
		os.MkdirAll(d+"/pkg", 0o755)
		cfg := clangCacheConfig(t, d+"/pkg", "-I"+d+"/a/b") // (the package folder is not above the headers)
		libHeader(t, d+"/a/b/lib.hpp", "#include \"../foo.h\"\n")
		if v, h, _ := libValue(t, cfg, `<lib.hpp>`); !strings.HasPrefix(v, "err:") || h != 1 {
			t.Fatalf("%s/%d", v, h)
		}
		if v, h, runs := libValue(t, cfg, `<lib.hpp>`); !strings.HasPrefix(v, "err:") || h != 1 || runs != 0 {
			t.Fatalf("the negative answer is not kept: %s/%d, %d runs", v, h, runs)
		}
		libHeader(t, d+"/a/foo.h", "#define LIBV 2\n")
		libWant(t, "foo.h created at includer/../foo.h", cfg, `<lib.hpp>`, "2")
	})
	t.Run("several_dotdots", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leaf")
		os.MkdirAll(d+"/inc/a", 0o755)
		os.MkdirAll(d+"/inc/b", 0o755)
		libHeader(t, d+"/inc/lib.hpp", "#include \"a/../b/../foo.h\"\n")
		libHeader(t, d+"/leaf/foo.h", "#define LIBV 1\n") // (found as leaf/a/../b/../foo.h: the folders are there)
		os.MkdirAll(d+"/leaf/a", 0o755)
		os.MkdirAll(d+"/leaf/b", 0o755)
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/inc/foo.h", "#define LIBV 2\n")
		libWant(t, "foo.h created at includer/a/../b/../foo.h", cfg, `<lib.hpp>`, "2")
	})
	t.Run("dot_slash", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leaf")
		libHeader(t, d+"/inc/lib.hpp", "#include \"./foo.h\"\n")
		libHeader(t, d+"/leaf/foo.h", "#define LIBV 1\n")
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/inc/foo.h", "#define LIBV 2\n")
		libWant(t, "foo.h created next to the includer", cfg, `<lib.hpp>`, "2")
	})
	t.Run("absolute_name", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc")
		libHeader(t, d+"/inc/lib.hpp", "#include \""+d+"/abs/foo.h\"\n")
		libHeader(t, d+"/abs/foo.h", "#define LIBV 1\n")
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/abs/foo.h", "#define LIBV 4\n")
		libWant(t, "the header is edited", cfg, `<lib.hpp>`, "4")
	})
	t.Run("found_in_the_includers_folder", func(t *testing.T) {
		d := t.TempDir()
		cfg := clangCacheConfig(t, d, "-I"+d+"/a/b", "-I"+d+"/inc/x")
		os.MkdirAll(d+"/inc/x", 0o755)
		libHeader(t, d+"/a/b/lib.hpp", "#include \"../foo.h\"\n")
		libHeader(t, d+"/a/foo.h", "#define LIBV 1\n")
		libHeader(t, d+"/inc/foo.h", "#define LIBV 9\n")
		libWant(t, "foo.h at includer/../foo.h", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/a/foo.h", "#define LIBV 3\n")
		libWant(t, "edited", cfg, `<lib.hpp>`, "3")
		os.Remove(d + "/a/foo.h")
		libWant(t, "removed: the one of -I", cfg, `<lib.hpp>`, "9")
	})
}

// A ".." after a symlinked folder is resolved by the system on the folder's target, not by the text: dir/link/..
// is the parent of what link points to. The cache watches the paths as they are spelled, so that the system
// resolves them the way the compiler's lookup did.
func TestCacheShadowDotDotAfterSymlink(t *testing.T) {
	link := func(t *testing.T, target, name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("no symbolic links: %v", err)
		}
	}
	t.Run("includer_folder_is_a_link", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/real/sub/lib.hpp", "#include \"../foo.h\"\n")
		libHeader(t, d+"/foo.h", "#define LIBV 1\n") // (found as inc/../foo.h)
		link(t, d+"/real/sub", d+"/inc/link")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc")
		libWant(t, "base", cfg, `<link/lib.hpp>`, "1")
		libWant(t, "again", cfg, `<link/lib.hpp>`, "1")
		libHeader(t, d+"/real/foo.h", "#define LIBV 2\n") // inc/link/../foo.h
		libWant(t, "foo.h created where the system takes inc/link/../foo.h", cfg, `<link/lib.hpp>`, "2")
	})
	t.Run("name_goes_through_a_link_missing", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/p/q/inc/lib.hpp", "#include \"lnk/../foo.h\"\n")
		libHeader(t, d+"/p/q/real/x", "")
		libHeader(t, d+"/p/q/inc2/foo.h", "#define LIBV 1\n")
		link(t, d+"/p/q/real", d+"/p/q/inc/lnk")
		cfg := clangCacheConfig(t, d, "-I"+d+"/p/q/inc", "-I"+d+"/p/q/inc2")
		if v, h, _ := libValue(t, cfg, `<lib.hpp>`); !strings.HasPrefix(v, "err:") || h != 1 {
			t.Fatalf("%s/%d", v, h)
		}
		libHeader(t, d+"/p/q/foo.h", "#define LIBV 4\n") // inc/lnk/.. is p/q
		libWant(t, "foo.h created where the system takes inc/lnk/../foo.h", cfg, `<lib.hpp>`, "4")
	})
	t.Run("name_goes_through_a_link_found_later", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/p/q/inc/lib.hpp", "#include \"lnk/../foo.h\"\n")
		libHeader(t, d+"/p/q/real/x", "")
		libHeader(t, d+"/p/q/inc2/sub/x", "")
		libHeader(t, d+"/p/q/inc2/foo.h", "#define LIBV 1\n")
		link(t, d+"/p/q/real", d+"/p/q/inc/lnk")
		link(t, d+"/p/q/inc2/sub", d+"/p/q/inc2/lnk")
		cfg := clangCacheConfig(t, d, "-I"+d+"/p/q/inc", "-I"+d+"/p/q/inc2")
		libWant(t, "foo.h at inc2/lnk/..", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/p/q/foo.h", "#define LIBV 4\n") // inc/lnk/.. is p/q: looked at first
		libWant(t, "foo.h created where the system takes inc/lnk/../foo.h", cfg, `<lib.hpp>`, "4")
	})
	t.Run("search_folder_goes_through_a_link", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/inc/lib.hpp", "#include <dep.h>\n")
		libHeader(t, d+"/other/sub/x", "")
		libHeader(t, d+"/inc3/dep.h", "#define LIBV 1\n")
		link(t, d+"/other/sub", d+"/inc/lnk")
		cfg := clangCacheConfig(t, d, "-Iinc", "-Iinc/lnk/..", "-Iinc3") // (relative to the package folder; inc/lnk/.. is other)
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/other/dep.h", "#define LIBV 2\n")
		libWant(t, "dep.h created in the folder -I names through a link", cfg, `<lib.hpp>`, "2")
	})
	t.Run("includer_path_goes_through_a_link", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/inc/main.hpp", "#include \"lnk/../sub/in.hpp\"\n") // = p/sub/in.hpp
		libHeader(t, d+"/p/real/x", "")
		libHeader(t, d+"/p/sub/in.hpp", "#include \"foo.h\"\n")
		libHeader(t, d+"/leaf/foo.h", "#define LIBV 1\n")
		link(t, d+"/p/real", d+"/inc/lnk")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc", "-I"+d+"/leaf")
		libWant(t, "base", cfg, `<main.hpp>`, "1")
		libHeader(t, d+"/p/sub/foo.h", "#define LIBV 2\n") // next to the includer, as the system sees it
		libWant(t, "foo.h created next to in.hpp", cfg, `<main.hpp>`, "2")
	})
	t.Run("a_header_that_was_read_through_a_link", func(t *testing.T) {
		d := t.TempDir()
		libHeader(t, d+"/inc/lib.hpp", "#include \"link/../x.h\"\n")
		libHeader(t, d+"/real/sub/y", "")
		libHeader(t, d+"/real/x.h", "#define LIBV 1\n")
		link(t, d+"/real/sub", d+"/inc/link")
		cfg := clangCacheConfig(t, d, "-I"+d+"/inc")
		libWant(t, "base", cfg, `<lib.hpp>`, "1")
		libWant(t, "again", cfg, `<lib.hpp>`, "1")
		libHeader(t, d+"/real/x.h", "#define LIBV 5\n")
		libWant(t, "the header behind the link is edited", cfg, `<lib.hpp>`, "5")
	})
}

// The paths that the cache builds are joined and cut as text, whatever they hold.
func TestRawPaths(t *testing.T) {
	sep := string(os.PathSeparator)
	for _, c := range []struct{ dir, name, want string }{
		{"/a/b", "../c.h", "/a/b" + sep + "../c.h"},
		{"/a/b/", "x/../c.h", "/a/b/x/../c.h"},
		{"", "c.h", "c.h"},
		{"/", "c.h", "/c.h"},
	} {
		if got := joinRaw(c.dir, c.name); got != c.want {
			t.Errorf("joinRaw(%q, %q) = %q, want %q", c.dir, c.name, got, c.want)
		}
	}
	for _, c := range []struct{ file, want string }{
		{"/a/b/c.h", "/a/b"},
		{"/a/link/../c.h", "/a/link/.."},
		{"c.h", "."},
		{"/c.h", "/"},
	} {
		if got := dirRaw(c.file); got != c.want {
			t.Errorf("dirRaw(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}
