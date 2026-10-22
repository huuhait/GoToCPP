package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func fakeMemberCompletion() int {
	if marker := os.Getenv("FAKE_MEMBER_BLOCK"); marker != "" {
		os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600)
		time.Sleep(time.Minute)
		// (only a compiler that was not stopped gets here)
		os.WriteFile(marker+".survived", nil, 0o600)
	}
	switch os.Getenv("FAKE_MEMBER_OUTPUT") {
	case "missing":
		return 0
	case "cut":
		fmt.Print("COMPLETION: Field : [#int#]Fie")
		return 0
	case "bad":
		fmt.Println("unexpected output")
		return 0
	case "long":
		fmt.Print(strings.Repeat("x", maxMemberOutput+1))
		return 0
	case "stderr":
		fmt.Fprint(os.Stderr, strings.Repeat("x", maxStderr+1))
		return 0
	case "crash":
		fmt.Println("COMPLETION: Field : [#int#]Field")
		return 2
	}
	if os.Getenv("FAKE_MEMBER_OUTPUT") == "partial" {
		fmt.Println("COMPLETION: unknown : [#void#]unknown(<#T value#>)")
	}
	fmt.Println("COMPLETION: Field : [#int#]Field")
	fmt.Println("COMPLETION: call : [#void#]call(<#int#>)")
	fmt.Println("COMPLETION: call : [#void#]call(<#double#>)")
	fmt.Println("COMPLETION: hidden (Inaccessible) : [#int#]hidden")
	fmt.Println("COMPLETION: Widget : Widget::")
	return 0
}

func fakeMemberValidation(text, src string) int {
	switch os.Getenv("FAKE_MEMBER_VALIDATION") {
	case "missing":
		return 0
	case "crash":
		return 2
	case "cut":
		fmt.Fprint(os.Stderr, "truncated diagnostic")
		return 1
	}
	line := strings.Count(text, "\n")
	fmt.Fprintf(os.Stderr, "%s:%d:1: error: gocpp_member_validation_complete\n", src, line)
	return 1
}

func TestMemberQueryRejectsUnsafeAndUnsupportedInputs(t *testing.T) {
	fake := newFake(t)
	queries := []MemberQuery{
		{Type: "Widget; int injected", Category: CatLValue},
		{Type: "Widget\n#error injected", Category: CatLValue},
		{Type: "Widget/*comment*/", Category: CatLValue},
		{Type: "Box<(f())>", Category: CatLValue},
		{Type: "Widget &", Category: CatLValue},
		{Type: "Widget **", Category: CatLValue},
		{Type: "Widget[2]", Category: CatLValue},
		{Type: "Widget", Category: CatNone},
		{Type: strings.Repeat("W", 4097), Category: CatLValue},
	}
	for _, query := range queries {
		if _, err := new(Runner).Members(context.Background(), fake.cfg(), nil, query); !errors.Is(err, ErrMemberReceiverUnsupported) {
			t.Errorf("query %+v: %v", query, err)
		}
	}
	_, err := new(Runner).Members(context.Background(), fake.cfg(),
		[]string{"\"api.hpp\"\n#error injected"}, MemberQuery{Type: "Widget", Category: CatLValue})
	if err == nil {
		t.Fatal("accepted injected include")
	}
	if data, _ := os.ReadFile(fake.counter); len(data) != 0 {
		t.Fatalf("invalid input ran a compiler: %s", data)
	}
}

func TestMemberNamesFilterAndOwnResults(t *testing.T) {
	output := strings.Repeat("COMPLETION: hidden (Inaccessible) : [#int#]hidden\n", 1000) +
		"COMPLETION: Field (InBase) : [#int#]Field\n" +
		"COMPLETION: call : [#void#]call()\n" +
		"COMPLETION: call : [#void#]call(<#int#>)\n" +
		"COMPLETION: __internal : [#int#]__internal\n" +
		"COMPLETION: operator= : [#Widget &#]operator=()\n" +
		"COMPLETION: Widget : Widget::\n"
	names, err := parseMemberNames(output)
	if err != nil || !reflect.DeepEqual(names, []string{"Field", "call"}) {
		t.Fatalf("names = %v, error = %v", names, err)
	}
	result := &MemberNames{names: names}
	result.Names()[0] = "changed"
	if result.Names()[0] != "Field" {
		t.Fatal("caller mutated published names")
	}
}

func TestMemberNamesAcceptsCompleteEmptyAnswer(t *testing.T) {
	names, err := parseMemberNames("COMPLETION: Empty : Empty::\n")
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("empty answer = %v, %v", names, err)
	}
}

func TestMemberNamesRejectsNameBudget(t *testing.T) {
	var output strings.Builder
	for i := range maxMemberNames + 1 {
		fmt.Fprintf(&output, "COMPLETION: field%d : [#int#]field%d\n", i, i)
	}
	if _, err := parseMemberNames(output.String()); err == nil {
		t.Fatal("accepted too many names")
	}
}

func TestMemberQueryRejectsIncompleteProducts(t *testing.T) {
	for _, failure := range []string{"missing", "cut", "bad", "long", "stderr", "crash"} {
		t.Run(failure, func(t *testing.T) {
			defer SetRacyWindow(0)()
			fake := newFake(t)
			fake.header(t, "api.hpp", "struct Widget {};\n")
			t.Setenv("FAKE_MEMBER_OUTPUT", failure)
			runner := new(Runner)
			query := MemberQuery{Type: "Widget", Category: CatLValue}
			if result, err := runner.Members(context.Background(), fake.cfg(),
				[]string{`"api.hpp"`}, query); err == nil || result != nil {
				t.Fatalf("accepted failed product: %v, %v", result, err)
			}
			t.Setenv("FAKE_MEMBER_OUTPUT", "")
			result, err := runner.Members(context.Background(), fake.cfg(), []string{`"api.hpp"`}, query)
			if err != nil || !reflect.DeepEqual(result.Names(), []string{"Field", "call"}) {
				t.Fatalf("failure poisoned later query: %v, %v", result, err)
			}
			if fake.count("member") != 2 {
				t.Fatalf("member runs = %d, want 2", fake.count("member"))
			}
		})
	}
}

func TestMemberQueryCoalescesAndReuses(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	runner := new(Runner)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := runner.Members(context.Background(), fake.cfg(), []string{`"api.hpp"`},
				MemberQuery{Type: "Widget", Category: CatLValue})
			if err != nil || !reflect.DeepEqual(result.Names(), []string{"Field", "call"}) {
				t.Errorf("query = %v, %v", result, err)
			}
		}()
	}
	group.Wait()
	_, err := new(Runner).Members(context.Background(), fake.cfg(), []string{`"api.hpp"`},
		MemberQuery{Type: "Widget", Category: CatLValue})
	if err != nil || fake.count("member") != 1 {
		t.Fatalf("disk reuse: runs=%d, error=%v", fake.count("member"), err)
	}
	if len(runner.infoMemo) != 0 || len(runner.infoReads) != 0 {
		t.Fatal("retained auxiliary metadata")
	}
}

func TestMemberQueryInvalidatesContext(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	runner := new(Runner)
	cfg := fake.cfg()
	query := MemberQuery{Type: "Widget", Category: CatLValue}
	check := func(want int) {
		t.Helper()
		if _, err := runner.Members(context.Background(), cfg, []string{`"api.hpp"`}, query); err != nil {
			t.Fatal(err)
		}
		if fake.count("member") != want {
			t.Fatalf("runs = %d, want %d", fake.count("member"), want)
		}
	}
	check(1)
	fake.header(t, "api.hpp", "struct Widget { int added; };\n")
	check(2)
	cfg.Flags = []string{"-DCHANGED=1"}
	check(3)
	cfg.CXX.GOOS = "linux"
	cfg.CXX.GOARCH = "arm64"
	check(4)
	cfg.CXX.Version += " changed"
	check(5)
	query.Category = CatXValue
	check(6)
}

func TestMemberQueryCancelsAndJoinsCompiler(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	marker := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_MEMBER_BLOCK", marker)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := new(Runner).Members(ctx, fake.cfg(), []string{`"api.hpp"`},
			MemberQuery{Type: "Widget", Category: CatLValue})
		done <- err
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	pid := 0
	for pid == 0 {
		data, _ := os.ReadFile(marker)
		pid, _ = strconv.Atoi(string(data))
		if pid != 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("query ended before blocked compiler: %v", err)
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("compiler did not enter controlled block")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-deadline.C:
		t.Fatal("canceled query did not join compiler")
	}
	if pidAlive(pid) {
		t.Fatalf("compiler %d survived cancellation", pid)
	}
}

func TestMemberQueryRealReceivers(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	const header = `struct Base { int inherited; };
struct HiddenBase { int inaccessibleBase; };
struct Widget : Base, private HiddenBase {
 int field;
 void change();
 void read() const;
 void overload(int);
 void overload(double);
 void left() &;
 void right() &&;
 protected: int protectedField;
 private: int secret;
};
template<class T> struct Box { int primary; };
template<> struct Box<long> { int specialized; };
struct Empty {};
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	for _, test := range []struct {
		name  string
		query MemberQuery
		want  []string
	}{
		{"lvalue", MemberQuery{"Widget", CatLValue},
			[]string{"change", "field", "inherited", "left", "overload", "read"}},
		{"xvalue", MemberQuery{"Widget", CatXValue},
			[]string{"change", "field", "inherited", "overload", "read", "right"}},
		{"prvalue", MemberQuery{"Widget", CatPRValue},
			[]string{"change", "field", "inherited", "overload", "read", "right"}},
		{"const", MemberQuery{"const Widget", CatLValue}, []string{"field", "inherited", "read"}},
		{"volatile", MemberQuery{"volatile Widget", CatLValue}, []string{"field", "inherited"}},
		{"pointer", MemberQuery{"const Widget *", CatPRValue}, []string{"field", "inherited", "read"}},
		{"constPointer", MemberQuery{"Widget *const", CatLValue},
			[]string{"change", "field", "inherited", "left", "overload", "read"}},
		{"specialization", MemberQuery{"Box<long>", CatLValue}, []string{"specialized"}},
		{"empty", MemberQuery{"Empty", CatLValue}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := new(Runner).Members(context.Background(), cfg, []string{`"api.hpp"`}, test.query)
			if err != nil || !reflect.DeepEqual(result.Names(), test.want) {
				t.Fatalf("members = %v, want %v; error = %v", result.Names(), test.want, err)
			}
		})
	}
}

func TestMemberQueryRejectsRacingInput(t *testing.T) {
	defer SetRacyWindow(0)()
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	header := filepath.Join(fake.inc, "api.hpp")
	t.Setenv("FAKE_EDIT", header+"|struct Widget { int added; };\n")
	t.Setenv("FAKE_EDIT_KEEP_MTIME", "1")
	runner := new(Runner)
	query := MemberQuery{Type: "Widget", Category: CatLValue}
	result, err := runner.Members(context.Background(), fake.cfg(), []string{`"api.hpp"`}, query)
	if err == nil || result != nil {
		t.Fatalf("accepted racing input: %v, %v", result, err)
	}
	if _, err := runner.Members(context.Background(), fake.cfg(), []string{`"api.hpp"`}, query); err != nil {
		t.Fatalf("racing input poisoned retry: %v", err)
	}
	if fake.count("member") != 2 {
		t.Fatalf("member runs = %d, want 2", fake.count("member"))
	}
}

// TestMemberQueryTimeoutJoinsCompiler ends the time limit of a blocked completion run by hand:
// no wall-clock time decides, so a slow machine or the race detector cannot change the outcome.
func TestMemberQueryTimeoutJoinsCompiler(t *testing.T) {
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	marker := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_MEMBER_BLOCK", marker)
	limitReached := make(chan struct{})
	restore := auxRunContext
	t.Cleanup(func() { auxRunContext = restore })
	auxRunContext = func(ctx context.Context, cfg Config) (context.Context, context.CancelFunc) {
		limited, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-limitReached:
				cancel()
			case <-limited.Done():
			}
		}()
		return limited, cancel
	}
	type answer struct {
		result *MemberNames
		err    error
	}
	done := make(chan answer, 1)
	go func() {
		result, err := new(Runner).Members(context.Background(), fake.cfg(), []string{`"api.hpp"`},
			MemberQuery{Type: "Widget", Category: CatLValue})
		done <- answer{result, err}
	}()
	pid := waitForPid(t, marker, done)
	close(limitReached)
	got := <-done
	if !errors.Is(got.err, context.DeadlineExceeded) || got.result != nil {
		t.Fatalf("accepted timed-out query or lost status: %v, %v", got.result, got.err)
	}
	if pidAlive(pid) {
		t.Fatalf("timeout did not join the blocked compiler: pid=%d", pid)
	}
	if _, err := os.Stat(marker + ".survived"); err == nil {
		t.Fatal("the blocked compiler ran to its end: the timeout did not stop it")
	}
}

// waitForPid waits until the blocked fake compiler wrote its process id to marker. The wait ends
// early if the query returned (the test then fails); the long bound only stops a hung test.
func waitForPid[A any](t *testing.T, marker string, done <-chan A) int {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	hung := time.NewTimer(time.Minute)
	defer hung.Stop()
	for {
		data, _ := os.ReadFile(marker)
		if pid, _ := strconv.Atoi(string(data)); pid != 0 {
			return pid
		}
		select {
		case early := <-done:
			t.Fatalf("query ended before the compiler blocked: %v", early)
		case <-ticker.C:
		case <-hung.C:
			t.Fatal("compiler did not enter controlled block")
		}
	}
}

func TestMemberQueryRejectsHeaderErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"),
		[]byte("struct Widget {};\n#error invalid header\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	result, err := new(Runner).Members(context.Background(), cfg, []string{`"api.hpp"`},
		MemberQuery{Type: "Widget", Category: CatLValue})
	if err == nil || result != nil {
		t.Fatalf("accepted invalid header: %v, %v", result, err)
	}
}

func TestMemberQueryRealShadowedInclude(t *testing.T) {
	defer SetRacyWindow(0)()
	dir := t.TempDir()
	fallback := t.TempDir()
	if err := os.WriteFile(filepath.Join(fallback, "api.hpp"),
		[]byte("struct Widget { int before; };\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	cfg.Flags = append(cfg.Flags, "-I", fallback)
	runner := new(Runner)
	query := MemberQuery{Type: "Widget", Category: CatLValue}
	before, err := runner.Members(context.Background(), cfg, []string{`"api.hpp"`}, query)
	if err != nil || !reflect.DeepEqual(before.Names(), []string{"before"}) {
		t.Fatalf("fallback = %v, %v", before, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"),
		[]byte("struct Widget { int after; };\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := runner.Members(context.Background(), cfg, []string{`"api.hpp"`}, query)
	if err != nil || !reflect.DeepEqual(after.Names(), []string{"after"}) {
		t.Fatalf("shadow = %v, %v", after, err)
	}
}

func TestMemberNamesDeduplicateAtNameLimit(t *testing.T) {
	var output strings.Builder
	for i := range maxMemberNames {
		fmt.Fprintf(&output, "COMPLETION: field%d : [#void#]field%d()\n", i, i)
	}
	output.WriteString("COMPLETION: field0 : [#void#]field0(<#int#>)\n")
	names, err := parseMemberNames(output.String())
	if err != nil || len(names) != maxMemberNames {
		t.Fatalf("dedup at name limit: %d names, %v", len(names), err)
	}
}

func TestMemberQueryValidationMustComplete(t *testing.T) {
	for _, failure := range []string{"missing", "cut", "crash"} {
		t.Run(failure, func(t *testing.T) {
			fake := newFake(t)
			fake.header(t, "api.hpp", "struct Widget {};\n")
			t.Setenv("FAKE_MEMBER_VALIDATION", failure)
			runner := new(Runner)
			query := MemberQuery{Type: "Widget", Category: CatLValue}
			if result, err := runner.Members(context.Background(), fake.cfg(),
				[]string{`"api.hpp"`}, query); result != nil || err == nil {
				t.Fatalf("accepted incomplete validation: %v, %v", result, err)
			}
			t.Setenv("FAKE_MEMBER_VALIDATION", "")
			if result, err := runner.Members(context.Background(), fake.cfg(),
				[]string{`"api.hpp"`}, query); result == nil || err != nil {
				t.Fatalf("validation failure poisoned retry: %v, %v", result, err)
			}
			if fake.count("member") != 2 {
				t.Fatalf("member runs = %d, want 2", fake.count("member"))
			}
		})
	}
}

func TestMemberQueryPartialStatusSurvivesDiskReuse(t *testing.T) {
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	t.Setenv("FAKE_MEMBER_OUTPUT", "partial")
	for range 2 {
		result, err := new(Runner).Members(context.Background(), fake.cfg(), []string{`"api.hpp"`},
			MemberQuery{Type: "Widget", Category: CatLValue})
		if err != nil || !result.Incomplete() ||
			!reflect.DeepEqual(result.Names(), []string{"Field", "call"}) {
			t.Fatalf("partial result lost its contract: %v, %v", result, err)
		}
	}
	if fake.count("member") != 1 {
		t.Fatalf("partial disk reuse runs = %d, want 1", fake.count("member"))
	}
}

func TestMemberQueryProvesUnqualifiedGroups(t *testing.T) {
	dir := t.TempDir()
	const header = `struct Left { int same; void clash(); int unique; };
struct Right { int same; void clash(); };
struct Ambiguous : Left, Right { void ok(); };
struct Picked : Left, Right { using Left::same; using Left::clash; };
struct A { void overload(int); };
struct B { void overload(double); };
struct Combined : A, B { using A::overload; using B::overload; };
struct Virtual { int shared; void method(); };
struct VLeft : virtual Virtual {};
struct VRight : virtual Virtual {};
struct Diamond : VLeft, VRight {};
struct Argument {};
struct Mixed {
 int good;
 void deleted() = delete;
 void unavailable() __attribute__((unavailable));
 void choice(int) = delete;
 void choice(double);
 void available(int) __attribute__((unavailable));
 void available(double);
 void unknown(Argument);
};
struct Unknown { void unknown(Argument); };
struct Known { void call(int); template<class T> void call(T); };
struct Empty {};
`
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := clangConfig(t, dir)
	for _, test := range []struct {
		typ        string
		names      []string
		incomplete bool
	}{
		{"Ambiguous", []string{"ok", "unique"}, false},
		{"Picked", []string{"clash", "same", "unique"}, false},
		{"Combined", []string{"overload"}, false},
		{"Diamond", []string{"method", "shared"}, false},
		{"Mixed", []string{"available", "choice", "good"}, true},
		{"Unknown", []string{}, true},
		{"Known", []string{"call"}, false},
		{"Empty", []string{}, false},
	} {
		t.Run(test.typ, func(t *testing.T) {
			result, err := new(Runner).Members(context.Background(), cfg, []string{`"api.hpp"`},
				MemberQuery{Type: test.typ, Category: CatLValue})
			if err != nil || !reflect.DeepEqual(result.Names(), test.names) ||
				result.Incomplete() != test.incomplete {
				t.Fatalf("result = %v, %v; want %v, incomplete=%v",
					result, err, test.names, test.incomplete)
			}
		})
	}
}

func TestMemberQueryRealStandardReceiver(t *testing.T) {
	defer SetRacyWindow(0)()
	cfg := clangConfig(t, t.TempDir())
	cfg.CacheDir = "off"
	result, err := new(Runner).Members(context.Background(), cfg, []string{"<vector>"},
		MemberQuery{Type: "const std::vector<long>", Category: CatLValue})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Names(), "size") || slices.Contains(result.Names(), "push_back") {
		t.Fatalf("const vector members = %v", result.Names())
	}
	if !result.Incomplete() {
		t.Fatal("unresolved method-template candidates were not marked incomplete")
	}
}

func TestMemberQueryRunsValidationAndCompletionTogether(t *testing.T) {
	defer SetRacyWindow(0)()
	t.Setenv(EnvJobs, "8")
	fake := newFake(t)
	fake.header(t, "api.hpp", "struct Widget {};\n")
	t.Setenv("FAKE_RENDEZVOUS", t.TempDir())
	t.Setenv("FAKE_RENDEZVOUS_RUNS", "2")
	t.Setenv("FAKE_RENDEZVOUS_KIND", "member")
	result, err := new(Runner).Members(context.Background(), fake.cfg(), []string{`"api.hpp"`},
		MemberQuery{Type: "Widget", Category: CatLValue})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !reflect.DeepEqual(result.Names(), []string{"Field", "call"}) {
		t.Fatalf("names = %v", result.Names())
	}
}
