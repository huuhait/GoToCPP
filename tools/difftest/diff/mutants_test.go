package diff

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

// A mutant is a bug injected into a scratch copy of gocpp: one replacement of a text in one
// file. TestSeededBugs builds gocpp from each copy and runs a campaign against it: the
// generator is good enough when every mutant is found within 2000 programs.
type mutant struct {
	name, file, old, new string
}

var mutants = []mutant{
	{"shr-negative-large-count", "runtime/include/gocpp/arith.hpp",
		"return x < 0 ? T(-1) : T(0);", "return T(0);"},
	{"shl-count-equal-to-width", "runtime/include/gocpp/arith.hpp",
		"if (static_cast<uint64_t>(n) >= std::numeric_limits<std::make_unsigned_t<T>>::digits) {\n        return 0;\n    }\n    using U",
		"if (static_cast<uint64_t>(n) > std::numeric_limits<std::make_unsigned_t<T>>::digits) {\n        return 0;\n    }\n    using U"},
	{"append-grows-when-it-fits", "runtime/include/gocpp/slice.hpp",
		"if (static_cast<uint64_t>(new_len) > static_cast<uint64_t>(s.gp_cap())) [[unlikely]] {\n            // A new array. The arguments may refer",
		"if (static_cast<uint64_t>(new_len) >= static_cast<uint64_t>(s.gp_cap())) [[unlikely]] {\n            // A new array. The arguments may refer"},
	{"copy-overlap-forward", "runtime/include/gocpp/slice.hpp",
		"std::memmove(static_cast<void*>(dst), static_cast<const void*>(src),\n                     static_cast<std::size_t>(n) * sizeof(T));\n    } else if",
		"for (int64_t i = 0; i < n; i++) {\n            dst[i] = src[i];\n        }\n    } else if"},
	// the two bugs that the reviewer of step 15.9 found and the first generator missed
	{"uint32-float-through-int32", "runtime/include/gocpp/arith.hpp",
		"} else if constexpr (sizeof(T) == 4 && std::is_unsigned_v<T>) {\n        return static_cast<T>(detail::cvt64(x));",
		"} else if constexpr (sizeof(T) == 4 && std::is_unsigned_v<T>) {\n        return static_cast<T>(detail::cvt32(x));"},
	{"utf8-overlong-e0-accepted", "runtime/src/rt.cpp",
		"case 0xE0:\n        lo = 0xA0;", "case 0xE0:\n        lo = 0x80;"},
	{"loop-variable-shared", "internal/lower/control.go",
		"l.renew = line\n\t\tl.post = s.Post", "_ = line\n\t\tl.post = s.Post"},
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("no go.mod")
	return ""
}

// TestSeededBugs is the power test of the generator: DIFFTEST_MUTANTS=1 go test -run SeededBugs
// ./tools/difftest/diff (in the dev shell, through the heavy-job slot: it builds gocpp five
// times and compiles thousands of programs).
func TestSeededBugs(t *testing.T) {
	if os.Getenv("DIFFTEST_MUTANTS") == "" {
		t.Skip("set DIFFTEST_MUTANTS=1 to run the seeded-bug experiment (it builds gocpp 5 times)")
	}
	root := repoRootForTest(t)
	only := os.Getenv("DIFFTEST_MUTANT")
	for _, m := range mutants {
		if only != "" && !strings.Contains(m.name, only) {
			continue
		}
		t.Run(m.name, func(t *testing.T) {
			dir := t.TempDir()
			tar := exec.Command("sh", "-c", "git -C "+root+" archive HEAD | tar -x -C "+dir)
			if out, err := tar.CombinedOutput(); err != nil {
				t.Fatalf("copying the repository: %v\n%s", err, out)
			}
			path := filepath.Join(dir, m.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if n := bytes.Count(data, []byte(m.old)); n != 1 {
				t.Fatalf("%s: the text to replace occurs %d times", m.file, n)
			}
			os.WriteFile(path, bytes.Replace(data, []byte(m.old), []byte(m.new), 1), 0o644)
			bin := filepath.Join(dir, "gocpp-mutant")
			build := exec.Command("go", "build", "-o", bin, "./cmd/gocpp")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("building the mutant: %v\n%s", err, out)
			}
			work := filepath.Join(dir, "work")
			os.MkdirAll(filepath.Join(work, "tmp"), 0o755)
			ref := &GoBackend{Work: filepath.Join(work, "tmp"), Cache: filepath.Join(os.TempDir(), "difftest-mutants-gocache"), Timeout: time.Minute}
			sub := &GocppBackend{Gocpp: bin, Cxx: "g++", Phase: "09", Work: filepath.Join(work, "tmp"), XDG: filepath.Join(work, "xdg"), Timeout: time.Minute}
			cfg := Config{
				Seed: 77, N: 2000, Batch: 100, Jobs: 3, Files: 8,
				Gen: gen.Config{Features: gen.FeaturesForPhase(9), FatalPercent: 8},
				Ref: ref, Subjects: []Backend{sub}, StopOnMismatch: true, VerifyRef: true,
				StateDir: filepath.Join(work, "state"), Log: os.Stderr,
			}
			c := NewCampaign(cfg)
			st, err := c.Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			mm := c.Found()
			if mm == nil {
				t.Fatalf("the injected bug was not found in %d programs (%+v)", st.Programs, st)
			}
			if mm.Kind == KindBuild || mm.Kind == KindICE {
				// the mutant breaks the generated C++: found, by the compiler of the generated code
				t.Logf("detected by the build: the mutant does not compile a program: %s", mm)
			}
			t.Logf("found: program %d (batch %d of 100): %s", mm.Prog, mm.Prog/100, mm)
		})
	}
}
