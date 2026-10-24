package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestHeaderIndexNames(t *testing.T) {
	index := parseHeaderIndex("Widget\nf\nf\nf::arg\nns\nns::Thing\nWidget::secret\noperator+\n",
		"#define USER(x) x\n#define __BUILTIN 1\n", "#define __BUILTIN 1\n")
	want := []HeaderSymbol{{Name: "USER", Macro: true}, {Name: "Widget"}, {Name: "f"}, {Name: "ns"}}
	if got := index.Names(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("global names = %v, want %v", got, want)
	}
	if got := index.Names("::ns"); !reflect.DeepEqual(got, []HeaderSymbol{{Name: "Thing"}}) {
		t.Fatalf("namespace names = %v", got)
	}
	index.Names("")[0].Name = "changed"
	if index.Names("")[0].Name != "USER" {
		t.Fatal("caller mutated the index")
	}
}

func TestHeaderIndexOwnsRetainedNames(t *testing.T) {
	for _, macro := range []bool{false, true} {
		t.Run(fmt.Sprintf("macro=%v", macro), func(t *testing.T) {
			line := "Kept\n"
			if macro {
				line = "#define Kept 1\n"
			}
			output := strings.Repeat(line, 500000)
			index := parseHeaderIndex(output, "", "")
			if macro {
				index = parseHeaderIndex("", output, "")
			}
			names := index.Names("")
			if !reflect.DeepEqual(names, []HeaderSymbol{{Name: "Kept", Macro: macro}}) {
				t.Fatal(names)
			}
			begin := uintptr(unsafe.Pointer(unsafe.StringData(output)))
			kept := uintptr(unsafe.Pointer(unsafe.StringData(names[0].Name)))
			if kept >= begin && kept < begin+uintptr(len(output)) {
				t.Fatal("retained name pins the discarded compiler output")
			}
			if index.Bytes() > 1024 {
				t.Fatalf("single-name index retained %d estimated bytes", index.Bytes())
			}
		})
	}
}

func TestHeaderIndexOwnsScopeAfterDiscardedDeclarations(t *testing.T) {
	output := strings.Repeat("operator+\n", 500000) + "scope::Kept\n"
	index := parseHeaderIndex(output, "", "")
	if !reflect.DeepEqual(index.Names("scope"), []HeaderSymbol{{Name: "Kept"}}) {
		t.Fatal(index.Names("scope"))
	}
	begin := uintptr(unsafe.Pointer(unsafe.StringData(output)))
	for scope := range index.scopes {
		kept := uintptr(unsafe.Pointer(unsafe.StringData(scope)))
		if kept >= begin && kept < begin+uintptr(len(output)) {
			t.Fatal("retained scope pins discarded declaration output")
		}
	}
}

// TestHeaderIndexAuxiliaryMetadataDoesNotAccumulate indexes more versions of a header than a
// cache entry keeps variants. Every version costs about ten compiler runs (each one starts the
// test binary again, which is slow under -race), so the count is just above the bound and no more.
func TestHeaderIndexAuxiliaryMetadataDoesNotAccumulate(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	header := filepath.Join(fake.inc, "api.hpp")
	runner := new(Runner)
	for version := range maxVariants + 3 {
		if err := os.WriteFile(header, []byte(fmt.Sprintf("Name%d\n", version)), 0o644); err != nil {
			t.Fatal(err)
		}
		index, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`})
		if err != nil || len(index.Names("")) != 2 {
			t.Fatalf("version %d: %v, %v", version, index, err)
		}
	}
	if len(runner.infoMemo) != 0 || len(runner.infoReads) != 0 || len(runner.dirsMemo) != 0 {
		t.Fatalf("index auxiliary metadata accumulated: info=%d reads=%d dirs=%d",
			len(runner.infoMemo), len(runner.infoReads), len(runner.dirsMemo))
	}
	if len(runner.st.mem) > 2 || runner.st.bytes > maxMem {
		t.Fatalf("unexpected retained index variants: entries=%d bytes=%d",
			len(runner.st.mem), runner.st.bytes)
	}
	checked := 0
	for key, body := range runner.st.mem {
		var entry variantFile
		if json.Unmarshal(body, &entry) != nil || entry.Kind != "i" {
			continue
		}
		checked++
		if len(entry.Variants) > maxVariants {
			t.Fatalf("index entry %q keeps %d variants, limit %d",
				key, len(entry.Variants), maxVariants)
		}
	}
	if checked == 0 {
		t.Fatal("no index entry is kept in memory, so the variant cap was not checked")
	}
}

func TestHeaderIndexCoalescesAndReuses(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	header := filepath.Join(fake.inc, "api.hpp")
	if err := os.WriteFile(header, []byte("Thing\nf\nf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := new(Runner)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			index, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`})
			if err != nil || !slices.Contains(index.Names(""), HeaderSymbol{Name: "Thing"}) {
				t.Errorf("index = %v, error = %v", index, err)
			}
		}()
	}
	wg.Wait()
	if got := fake.runs(); got != 2 {
		t.Fatalf("concurrent name runs = %d, want one enumeration and one validation", got)
	}
	if _, err := new(Runner).Index(context.Background(), fake.cfg(), []string{`"api.hpp"`}); err != nil {
		t.Fatal(err)
	}
	if got := fake.runs(); got != 2 {
		t.Fatalf("disk reuse name runs = %d, want 2", got)
	}
}

func TestHeaderIndexInvalidatesInputs(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	header := filepath.Join(fake.inc, "api.hpp")
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(header, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Before\n")
	runner := new(Runner)
	cfg := fake.cfg()
	check := func(name string, runs int) {
		t.Helper()
		index, err := runner.Index(context.Background(), cfg, []string{`"api.hpp"`})
		if err != nil || !slices.Contains(index.Names(""), HeaderSymbol{Name: name}) {
			t.Fatalf("index %v, error %v; want %s", index, err, name)
		}
		if got := fake.runs(); got != runs {
			t.Fatalf("enumerations = %d, want %d", got, runs)
		}
	}
	check("Before", 2)
	write("After\n")
	check("After", 4)
	cfg.Flags = []string{"-DCHANGED=1"}
	check("After", 6)
	cfg.CXX.Version += " changed"
	check("After", 8)
	cfg.CXX.GOOS = "linux"
	cfg.CXX.GOARCH = "arm64"
	check("After", 10)
}

func TestHeaderIndexRejectsIncompleteResults(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	if err := os.WriteFile(filepath.Join(fake.inc, "api.hpp"), []byte("Thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := new(Runner)
	t.Setenv("FAKE_DI", "cut")
	if index, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`}); err == nil || index != nil {
		t.Fatalf("incomplete result = %v, %v", index, err)
	}
	t.Setenv("FAKE_DI", "")
	if _, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`}); err != nil {
		t.Fatal(err)
	}
	if fake.runs() != 2 {
		t.Fatalf("incomplete discovery was cached: %d name runs", fake.runs())
	}
}

func TestHeaderIndexRejectsFailedCompiler(t *testing.T) {
	fake := newFake(t)
	runner := new(Runner)
	for range 2 {
		index, err := runner.Index(context.Background(), fake.cfg(), []string{`"FAILNOW.hpp"`})
		if err == nil || index != nil {
			t.Fatalf("failed result = %v, %v", index, err)
		}
	}
	if fake.count("E") != 2 {
		t.Fatalf("failed discovery was cached: %d preprocessor runs", fake.count("E"))
	}
}

// A compiler run of the index that is stopped at the time limit reports the deadline, as the member
// completion does (TestMemberQueryTimeoutJoinsCompiler), so a caller can tell a timeout from a
// header that does not index.
func TestHeaderIndexTimeoutWrapsDeadlineExceeded(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	if err := os.WriteFile(filepath.Join(fake.inc, "api.hpp"), []byte("Thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_SLEEP", "60")
	cfg := fake.cfg()
	// The limit applies to every compiler run of the query, so it is well above what a normal
	// fake run needs and is reached only by the sleeping name run.
	cfg.Timeout = 2 * time.Second
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if index != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("index = %v, error = %v, want the deadline", index, err)
	}
}

func TestHeaderIndexCanceledRunIsNotCached(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	if err := os.WriteFile(filepath.Join(fake.inc, "api.hpp"), []byte("Thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("FAKE_START", marker)
	t.Setenv("FAKE_SLEEP", "180") // a cancel that does not stop the run waits three minutes
	runner := new(Runner)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runner.Index(ctx, fake.cfg(), []string{`"api.hpp"`})
		done <- err
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(time.Minute) // (only ends a hang)
	defer deadline.Stop()
started:
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("fake clang did not start")
			break started
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(time.Minute):
		t.Fatal("canceled clang did not stop in a minute")
	}
	t.Setenv("FAKE_SLEEP", "")
	if _, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`}); err != nil {
		t.Fatal(err)
	}
	if fake.runs() != 3 {
		t.Fatalf("canceled result was reused: %d name runs", fake.runs())
	}
}

func TestHeaderIndexDiscardsRacingHeader(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	header := filepath.Join(fake.inc, "api.hpp")
	if err := os.WriteFile(header, []byte("Before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_EDIT", header+"|After\n")
	runner := new(Runner)
	if index, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`}); err == nil || index != nil {
		t.Fatalf("racing result = %v, %v", index, err)
	}
	index, err := runner.Index(context.Background(), fake.cfg(), []string{`"api.hpp"`})
	if err != nil || !slices.Contains(index.Names(""), HeaderSymbol{Name: "After"}) {
		t.Fatalf("replacement result = %v, %v", index, err)
	}
}

func TestHeaderIndexRealShadowedInclude(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	early := t.TempDir()
	late := t.TempDir()
	if err := os.WriteFile(filepath.Join(late, "api.hpp"), []byte("struct Before {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir, "-I"+early, "-I"+late)
	cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
	runner := new(Runner)
	index, err := runner.Index(context.Background(), cfg, []string{"<api.hpp>"})
	if err != nil || !slices.Contains(index.Names(""), HeaderSymbol{Name: "Before"}) {
		t.Fatalf("original include = %v, %v", index, err)
	}
	if err := os.WriteFile(filepath.Join(early, "api.hpp"), []byte("struct After {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err = runner.Index(context.Background(), cfg, []string{"<api.hpp>"})
	if err != nil || !reflect.DeepEqual(index.Names(""), []HeaderSymbol{{Name: "After"}}) {
		t.Fatalf("shadowed include = %v, %v", index, err)
	}
}

func TestHeaderIndexRequiresCompletedValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX compiler wrapper")
	}
	defer SetRacyWindow(0)()
	for _, exit := range []string{"kill -KILL $$", "exit 1"} {
		t.Run(exit, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "api.hpp"),
				[]byte("struct Thing {}; inline void f(int aaa, int zzz) {}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := clangConfig(t, dir)
			wrapper := filepath.Join(dir, "clang-wrapper")
			script := fmt.Sprintf(`#!/bin/sh
validation=
for arg do
  if [ "$arg" = '-Wno-everything' ]; then validation=1; fi
  src=$arg
done
if [ -n "$validation" ]; then
  printf "%%s:3:36: error: no member named 'aaa' in the global namespace\n" "$src" >&2
  %s
fi
exec %q "$@"
`, exit, cfg.CXX.Path)
			if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			cfg.CXX.Path = wrapper
			cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
			for _, runner := range []*Runner{new(Runner), new(Runner)} {
				if index, err := runner.Index(context.Background(), cfg, []string{`"api.hpp"`}); err == nil || index != nil {
					t.Fatalf("partial validation was accepted: %v, %v", index, err)
				}
			}
		})
	}
}

func TestHeaderIndexRequiredDependencyAtValidationBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX compiler wrapper")
	}
	defer SetRacyWindow(0)()
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace=%v", replace), func(t *testing.T) {
			dir := t.TempDir()
			header := filepath.Join(dir, "api.hpp")
			if err := os.WriteFile(header, []byte("struct Gone {};\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			action := fmt.Sprintf("rm -f -- %q", header)
			if replace {
				replacement := filepath.Join(dir, "replacement.hpp")
				if err := os.WriteFile(replacement, []byte("struct Changed {};\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				action = fmt.Sprintf("if [ -f %q ]; then mv -f -- %q %q; fi", replacement, replacement, header)
			}
			cfg := clangConfig(t, dir)
			wrapper := filepath.Join(dir, "clang-wrapper")
			script := fmt.Sprintf(`#!/bin/sh
%q "$@"
rc=$?
for arg do
  if [ "$arg" = '-Wno-everything' ]; then %s; fi
done
exit $rc
`, cfg.CXX.Path, action)
			if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			cfg.CXX.Path = wrapper
			cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
			runner := new(Runner)
			if index, err := runner.Index(context.Background(), cfg, []string{`"api.hpp"`}); err == nil || index != nil {
				t.Fatalf("changed required header was accepted: %v, %v", index, err)
			}
			for _, next := range []*Runner{runner, new(Runner)} {
				index, err := next.Index(context.Background(), cfg, []string{`"api.hpp"`})
				if replace {
					if err != nil || !reflect.DeepEqual(index.Names(""), []HeaderSymbol{{Name: "Changed"}}) {
						t.Fatalf("replacement result: %v, %v", index, err)
					}
				} else if err == nil || index != nil {
					t.Fatalf("missing header reused a result: %v, %v", index, err)
				}
			}
		})
	}
}

func TestHeaderNamespaceRelationshipsRespectScopes(t *testing.T) {
	text := `namespace original {}
namespace api {
inline namespace v1 {}
namespace alias = ::original;
struct Record { const char* text = R"raw("); namespace fake { using namespace missing; )raw"; };
void f() { using namespace local; }
using namespace original;
}
`
	want := []namespaceEdge{
		{from: "original", declare: true},
		{from: "api", declare: true},
		{from: "api::v1", context: "api", declare: true},
		{from: "api", target: "::api::v1"},
		{from: "api::alias", context: "api", target: "::original", alias: true},
		{from: "api", context: "api", target: "original"},
	}
	got, err := indexNamespaceEdges(context.Background(), text)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("namespace relationships = %+v, %v; want %+v", got, err, want)
	}
}

func TestHeaderNamespaceEventPathBudget(t *testing.T) {
	var source strings.Builder
	source.WriteString("namespace ")
	source.WriteString(strings.Repeat("n", 1<<20))
	source.WriteString(" {\n")
	for i := range 20 {
		fmt.Fprintf(&source, "namespace child%d {}\n", i)
	}
	source.WriteString("}\n")
	if _, err := indexNamespaceEdges(context.Background(), source.String()); err == nil {
		t.Fatal("namespace path expansion exceeded its byte budget")
	}
}

func TestHeaderIndexVisibleNamespaceTargets(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := `namespace library { inline namespace v1 { namespace detail { struct Thing {}; } } }
namespace shortname = library;
namespace alias = shortname::detail;
namespace imported { using namespace shortname::detail; }
namespace forwarding { using namespace library; }
namespace through = forwarding::detail;
namespace through_import { using namespace forwarding::detail; }
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
	for _, runner := range []*Runner{new(Runner), new(Runner)} {
		index, err := runner.Index(context.Background(), cfg, []string{`"api.hpp"`})
		if err != nil {
			t.Fatal(err)
		}
		for _, scope := range []string{
			"library::detail", "library::v1::detail", "shortname::detail", "alias", "imported",
			"forwarding::detail", "through", "through_import",
		} {
			if !slices.Contains(index.Names(scope), HeaderSymbol{Name: "Thing"}) {
				t.Fatalf("visible namespace %s = %v; scopes=%v bindings=%v",
					scope, index.Names(scope), index.scopes, index.namespaces)
			}
		}
	}
}

func TestHeaderIndexNamespaceTargetsRespectDeclarationOrder(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := `namespace library { namespace item { struct Earlier {}; } }
namespace context {
using namespace library;
namespace saved = item;
namespace item { struct Later {}; }
namespace current = item;
}
namespace external = context::saved;
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"context::saved", "external"} {
		if got := index.Names(scope); !reflect.DeepEqual(got, []HeaderSymbol{{Name: "Earlier"}}) {
			t.Fatalf("earlier binding %s = %v", scope, got)
		}
	}
	for _, scope := range []string{"context::item", "context::current"} {
		if got := index.Names(scope); !reflect.DeepEqual(got, []HeaderSymbol{{Name: "Later"}}) {
			t.Fatalf("later binding %s = %v", scope, got)
		}
	}
}

func TestHeaderIndexNamespaceTargetsThroughTransitiveCycles(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := `namespace library { namespace detail { struct Thing {}; } }
namespace left {}
namespace right { using namespace left; }
namespace left { using namespace right; using namespace library; }
namespace first { using namespace right; }
namespace second { using namespace first; }
namespace selected = second::detail;
namespace imported { using namespace second::detail; }
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"left::detail", "second::detail", "selected", "imported"} {
		if !slices.Contains(index.Names(scope), HeaderSymbol{Name: "Thing"}) {
			t.Fatalf("transitive binding %s = %v", scope, index.Names(scope))
		}
	}
	if len(index.namespaces) != 8 {
		t.Fatalf("visibility materialized paths: %d canonical nodes", len(index.namespaces))
	}
}

func TestHeaderNamespaceBindingClosureHonorsCancellation(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &headerCancelContext{Context: base, cancel: cancel}
	graph := newNamespaceGraph(ctx)
	if err := graph.declare("left::child"); err != nil {
		t.Fatal(err)
	}
	if err := graph.declare("right"); err != nil {
		t.Fatal(err)
	}
	if err := graph.use("left", "right"); err != nil {
		t.Fatal(err)
	}
	if err := graph.use("right", "left"); err != nil {
		t.Fatal(err)
	}
	if err := graph.settle(); !errors.Is(err, context.Canceled) {
		t.Fatalf("binding closure cancellation = %v", err)
	}
	if graph.visible != nil || len(graph.direct) != 4 {
		t.Fatal("canceled closure published bindings or materialized new nodes")
	}
}

func TestHeaderIndexQualifiedAliasTargets(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := `namespace outer { namespace inner { struct Thing {}; } }
namespace alias = outer;
namespace indirect = alias::inner;
namespace absolute = ::alias::inner;
namespace imported { using namespace alias::inner; }
namespace nested { namespace later = ::indirect; }
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
	for _, runner := range []*Runner{new(Runner), new(Runner)} {
		index, err := runner.Index(context.Background(), cfg, []string{`"api.hpp"`})
		if err != nil {
			t.Fatal(err)
		}
		for _, scope := range []string{
			"outer::inner", "alias::inner", "indirect", "absolute", "imported", "nested::later",
		} {
			if !slices.Contains(index.Names(scope), HeaderSymbol{Name: "Thing"}) {
				t.Fatalf("qualified namespace %s = %v", scope, index.Names(scope))
			}
		}
		if len(index.namespaces) != 5 {
			t.Fatalf("aliases materialized extra canonical nodes: %d", len(index.namespaces))
		}
	}
}

func TestHeaderIndexNamespaceVisibility(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	text := `namespace original { struct Thing {}; }
namespace alias = original;
namespace imported { using namespace original; }
namespace api __attribute__((visibility("default"))) { inline namespace v1 { struct Visible {}; } }
namespace local { void f() { using namespace original; } }
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"original", "Thing"}, {"alias", "Thing"}, {"imported", "Thing"}, {"api", "Visible"}} {
		if !slices.Contains(index.Names(pair[0]), HeaderSymbol{Name: pair[1]}) {
			t.Errorf("namespace %s = %v, missing %s", pair[0], index.Names(pair[0]), pair[1])
		}
	}
	if slices.Contains(index.Names("local"), HeaderSymbol{Name: "Thing"}) {
		t.Fatal("a function-local using directive leaked into namespace completion")
	}
}

func TestHeaderIndexRejectsEarlySymlinkSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX compiler wrapper and symlinks")
	}
	defer SetRacyWindow(0)()
	for _, ancestor := range []bool{false, true} {
		t.Run(fmt.Sprintf("ancestor=%v", ancestor), func(t *testing.T) {
			dir := t.TempDir()
			first := filepath.Join(dir, "first")
			second := filepath.Join(dir, "second")
			for _, folder := range []string{first, second} {
				if err := os.Mkdir(folder, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(first, "api.hpp"), []byte("void Present();\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(second, "api.hpp"),
				[]byte("void Present(); void Added();\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "api.hpp")
			target := filepath.Join(first, "api.hpp")
			replacement := filepath.Join(second, "api.hpp")
			include := `"api.hpp"`
			if ancestor {
				link = filepath.Join(dir, "headers")
				target = first
				replacement = second
				include = `"headers/api.hpp"`
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			cfg := clangConfig(t, dir)
			wrapper := filepath.Join(dir, "clang-wrapper")
			marker := filepath.Join(dir, "swapped")
			script := fmt.Sprintf(`#!/bin/sh
%q "$@"
rc=$?
for arg do
  if [ "$arg" = '-ast-list' ] && [ ! -f %q ]; then
    ln -sfn %q %q
    printf swapped > %q
  fi
done
exit $rc
`, cfg.CXX.Path, marker, replacement, link, marker)
			if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			cfg.CXX.Path = wrapper
			cfg.CacheDir = filepath.Join(t.TempDir(), "typed-v1")
			runner := new(Runner)
			if index, err := runner.Index(context.Background(), cfg, []string{include}); err == nil || index != nil {
				t.Fatalf("old-target enumeration was published: %v, %v", index, err)
			}
			for _, next := range []*Runner{runner, new(Runner)} {
				index, err := next.Index(context.Background(), cfg, []string{include})
				if err != nil || !slices.Contains(index.Names(""), HeaderSymbol{Name: "Added"}) {
					t.Fatalf("new target inherited an incomplete index: %v, %v", index, err)
				}
			}
		})
	}
}

func TestHeaderIndexCompactsFilteredBackingStorage(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	var header strings.Builder
	header.WriteString("inline void Kept() {\n")
	for i := range 4096 {
		fmt.Fprintf(&header, "int local%d;\n", i)
	}
	header.WriteString("}\n")
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	names := index.scopes[""]
	if !reflect.DeepEqual(names, []HeaderSymbol{{Name: "Kept"}}) || cap(names) != len(names) {
		t.Fatalf("filtered index len=%d cap=%d names=%v", len(names), cap(names), names)
	}
}

func TestHeaderNamespaceVisibilityHasFiniteCanonicalNodes(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := `namespace api {
struct Outer {};
namespace detail { using namespace api; struct Thing {}; }
}
namespace left {}
namespace right { using namespace left; struct Right {}; }
namespace left { using namespace right; struct Left {}; }
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.namespaces) != 5 || len(index.scopes) != 5 {
		t.Fatalf("visibility created synthetic scopes: scopes=%d nodes=%d", len(index.scopes), len(index.namespaces))
	}
	for _, scope := range []string{"left", "right"} {
		for _, name := range []string{"Left", "Right"} {
			if !slices.Contains(index.Names(scope), HeaderSymbol{Name: name}) {
				t.Fatalf("cyclic visibility %s = %v", scope, index.Names(scope))
			}
		}
	}
	before := index.Bytes()
	deep := "api::" + strings.Repeat("detail::", 64)
	if !slices.Contains(index.Names(strings.TrimSuffix(deep, "::")), HeaderSymbol{Name: "Thing"}) {
		t.Fatal("ancestor visibility failed to follow a canonical namespace binding")
	}
	if index.Bytes() != before {
		t.Fatal("namespace lookup materialized new paths")
	}
}

type headerCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
}

func (ctx *headerCancelContext) Err() error {
	ctx.calls++
	if ctx.calls == 4 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestHeaderNamespaceVisibilityHonorsCancellation(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &headerCancelContext{Context: base, cancel: cancel}
	index := parseHeaderIndex("left\nleft::Left\nright\nright::Right\n", "", "")
	before := index.Bytes()
	text := "namespace left { using namespace right; } namespace right { using namespace left; }"
	if _, err := completeIndexNamespaces(ctx, index, text); !errors.Is(err, context.Canceled) {
		t.Fatalf("namespace expansion cancellation = %v", err)
	}
	if index.Bytes() != before || index.namespaces != nil {
		t.Fatal("canceled visibility work changed the published index")
	}
}

func TestHeaderNamespaceDigitSeparators(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	header := "constexpr int count = 1'000;\nnamespace original { struct Thing {}; }\nnamespace alias = original;\n"
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil || !slices.Contains(index.Names("alias"), HeaderSymbol{Name: "Thing"}) {
		t.Fatalf("digit separator lost the namespace alias: %v, %v", index, err)
	}
}

func TestHeaderIndexRealTinyHeader(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	text := "#define API_VALUE 42\nstruct Widget { private: int secret; };\n" +
		"inline int f(int x) { return x; }\ninline double f(double x) { return x; }\n" +
		"constexpr int Value = 7;\nnamespace ns { struct Thing {}; constexpr int Answer = 3; }\n"
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	index, err := new(Runner).Index(context.Background(), cfg, []string{`"api.hpp"`})
	if err != nil {
		t.Fatal(err)
	}
	want := []HeaderSymbol{{Name: "API_VALUE", Macro: true}, {Name: "Value"}, {Name: "Widget"}, {Name: "f"}, {Name: "ns"}}
	if got := index.Names(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("real global names = %v, want %v", got, want)
	}
	if got := index.Names("ns"); !reflect.DeepEqual(got, []HeaderSymbol{{Name: "Answer"}, {Name: "Thing"}}) {
		t.Fatalf("real namespace names = %v", got)
	}
}

func TestHeaderIndexRunsItsFourCompilerRunsTogether(t *testing.T) {
	defer SetRacyWindow(0)()
	t.Setenv(EnvJobs, "8")
	fake := newFake(t)
	fake.header(t, "api.hpp", "Thing\nf\n")
	t.Setenv("FAKE_RENDEZVOUS", t.TempDir())
	t.Setenv("FAKE_RENDEZVOUS_RUNS", "4")
	t.Setenv("FAKE_RENDEZVOUS_KIND", "index")
	index, err := new(Runner).Index(context.Background(), fake.cfg(), []string{`"api.hpp"`})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if !slices.Contains(index.Names(""), HeaderSymbol{Name: "Thing"}) {
		t.Fatalf("names = %v", index.Names(""))
	}
}
