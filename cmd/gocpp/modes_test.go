//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of the library build modes (Phase 06 step 6.10, ADR-043): the errors of //export, c-archive
// and c-shared libraries used by C programs (cases of one consumer program, each a subtest), two
// shared libraries in one process, the symbols of a shared library, cpp-lib used by a CMake project,
// and the up-to-date check of library builds. The consumers are compiled with the C compiler that goes
// with the C++ compiler, and run in a process group that dies with the test.

// libGate is the phase of the features that the fixtures use (channels and go statements).
const libGate = "08"

// runCmd runs a command in dir with a time limit, in a process group of its own that is killed when the
// test ends, and returns its standard output and error and its exit status (-1 for a signal).
func runCmd(t *testing.T, dir string, env []string, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Start()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	err = cmd.Wait()
	code = 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v\n%s%s", name, err, out.String(), errOut.String())
	}
	return out.String(), errOut.String(), code
}

// cCompiler returns the C compiler that goes with the compiler c, with its arguments.
func cCompiler(t *testing.T, c testCXX) []string {
	t.Helper()
	cc, err := build.FindCC(context.Background(), c.cxx)
	if err != nil {
		t.Skipf("no C compiler for %s: %v", c.name, err)
	}
	return append([]string{cc.Path}, cc.Args...)
}

// compileC compiles the C file src in dir into the program out with the flags, and fails the test
// when it does not compile.
func compileC(t *testing.T, c testCXX, dir, out, src string, flags ...string) {
	t.Helper()
	cc := cCompiler(t, c)
	args := slices.Concat(cc[1:], []string{"-std=gnu17", "-Wall", "-Wextra", "-I", dir, "-o", out, src}, flags)
	o, e, code := runCmd(t, dir, nil, cc[0], args...)
	if code != 0 {
		t.Fatalf("%s %s: exit %d\n%s%s", cc[0], strings.Join(args, " "), code, o, e)
	}
}

// libFixture is a library of exported functions that exercises the types of the boundary, the
// initialization, a callback from C and a goroutine.
var libFixture = map[string]string{
	"gocpp.mod": "module example.com/lib\n",
	"cb.h":      "#ifndef CB_H\n#define CB_H\nint run_visits(int n);\n#endif\n",
	"cb.c":      "#include \"cb.h\"\nextern long long Visit(int);\nint run_visits(int n) {\n\tlong long s = 0;\n\tfor (int i = 0; i < n; i++) s += Visit(i);\n\treturn (int)s;\n}\n",
	"lib.gp": `package main

import (
	"./cb.h"
	"unsafe"
)

type Celsius float64
type Count int32

var initCount int
var table = makeTable()
var visits int
var visitLock = make(chan struct{}, 1) // threads call Visit at once: the counter needs a lock
var global int32 = 41

func makeTable() []int32 { initCount += 1; return []int32{10, 20, 30} }

func init() { initCount += 100 }

//export Add
func Add(a, b int32) int32 { return a + b }

//export AddInt
func AddInt(a, b int) int { return a + b }

//export Narrow
func Narrow(a uint8, b int8, c uint16, d int16) int64 { return int64(a) + int64(b) + int64(c) + int64(d) }

//export Not
func Not(b bool) bool { return !b }

//export Scale
func Scale(c Celsius, k float64) Celsius { return c * Celsius(k) }

//export Twice
func Twice(n Count) Count { return n * 2 }

//export Half
func Half(f float32) float32 { return f / 2 }

// Upper returns s in upper case (ASCII).
// It is like src/*/x and a/**b, a line that ends with a backslash \
// and a trigraph ??/ (all of them are in a C comment).
// A line can end in a trigraph ??/
// or in three question marks ???/
//
//export Upper
func Upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

//export StrLen
func StrLen(s string) int { return len(s) }

//export Empty
func Empty() string { return "" }

//export Sum
func Sum(xs []int64) (int64, bool) {
	var s int64
	for _, x := range xs {
		s += x
	}
	return s, len(xs) > 0
}

//export Squares
func Squares(n int) []int32 {
	out := make([]int32, n)
	for i := range out {
		out[i] = int32(i * i)
	}
	return out
}

//export Reverse
func Reverse(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[len(b)-1-i] = c
	}
	return out
}

//export Poke
func Poke(p *int32, v int32) { *p = v }

//export Peek
func Peek(p *C.int) C.int { return *p }

//export CTypes
func CTypes(a C.int, b C.long, c C.uchar, d C.double) C.long { return C.long(a) + b + C.long(c) + C.long(d) }

//export Ident
func Ident(p unsafe.Pointer) unsafe.Pointer { return p }

//export GlobalAddr
func GlobalAddr() *int32 { return &global }

//export Leak
func Leak() *int32 {
	x := int32(5)
	return &x
}

//export Visit
func Visit(i int32) int64 {
	visitLock <- struct{}{}
	visits++
	<-visitLock
	return int64(table[int(i)%3])
}

//export Run
func Run(n int32) int32 { return int32(cb.run_visits(C.int(n))) }

//export Visits
func Visits() int { return visits }

//export InitCount
func InitCount() int { return initCount }

//export Boom
func Boom(msg string) { panic(msg) }

//export Index
func Index(i int) int32 { return table[i] }

//export Recovered
func Recovered() (r string) {
	defer func() { r = recover().(string) }()
	panic("inner")
}

//export Async
func Async(n int32) int32 {
	ch := make(chan int32)
	for i := int32(0); i < n; i++ {
		go func() { ch <- i }()
	}
	var s int32
	for i := int32(0); i < n; i++ {
		s += <-ch
	}
	return s
}

func main() {}
`,
}

// libExports are the names of the exported functions of libFixture.
var libExports = []string{"Add", "AddInt", "Narrow", "Not", "Scale", "Twice", "Half", "Upper", "StrLen", "Empty", "Sum", "Squares",
	"Reverse", "Poke", "Peek", "CTypes", "Ident", "GlobalAddr", "Leak", "Visit", "Run", "Visits", "InitCount", "Boom", "Index",
	"Recovered", "Async"}

// consumerC is the C program that uses libFixture: the first argument names the case.
const consumerC = `#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libfx.h"

#define CHECK(c) do { if (!(c)) { printf("FAIL line %d: %s\n", __LINE__, #c); fflush(stdout); exit(1); } } while (0)

static GoString gs(const char* s) { GoString g = {s, (ptrdiff_t)strlen(s)}; return g; }

static void* thread_run(void* arg) {
	long long total = 0;
	(void)arg;
	for (int i = 0; i < 20; i++) total += Run(5) + Async(10);
	return (void*)(long)total;
}

static void* thread_init(void* arg) {
	(void)arg;
	return (void*)(long)InitCount();
}

static void* thread_panic(void* arg) {
	(void)arg;
	Boom(gs("from a thread"));
	return 0;
}

int main(int argc, char** argv) {
	const char* c = argc > 1 ? argv[1] : "";
	if (!strcmp(c, "numbers")) {
		CHECK(Add(2, 3) == 5);
		CHECK(Add(-7, 3) == -4);
		CHECK(AddInt(1LL << 40, 1) == (1LL << 40) + 1);
		CHECK(Narrow(200, -5, 60000, -3) == 200 - 5 + 60000 - 3);
		CHECK(Not(0) == 1 && Not(1) == 0);
		CHECK(Scale(2.5, 4.0) == 10.0);
		CHECK(Twice(21) == 42);
		CHECK(Half(5.0f) == 2.5f);
	} else if (!strcmp(c, "strings")) {
		GoString r = Upper(gs("hello, world"));
		CHECK(r.n == 12 && memcmp(r.p, "HELLO, WORLD", 12) == 0 && r.p[12] == 0);
		free((void*)r.p);
		char buf[] = {'a', 0, 'b', 0, 'c'};
		GoString in = {buf, 5};
		CHECK(StrLen(in) == 5);
		GoString e = Empty();
		CHECK(e.n == 0 && e.p != NULL && e.p[0] == 0);
		free((void*)e.p);
		GoString none = {NULL, 0};
		GoString u = Upper(none);
		CHECK(u.n == 0);
		free((void*)u.p);
		printf("ok\n");
	} else if (!strcmp(c, "copies")) {
		/* The argument is a copy: C may reuse its memory right after the call, and the result
		   is memory of C's own. */
		char buf[8];
		strcpy(buf, "abc");
		GoString r = Upper(gs(buf));
		strcpy(buf, "xyz");
		CHECK(memcmp(r.p, "ABC", 3) == 0);
		free((void*)r.p);
	} else if (!strcmp(c, "slices")) {
		long long xs[3] = {1, 2, 3};
		GoSlice sl = {xs, 3, 3};
		struct Sum_return s = Sum(sl);
		CHECK(s.r0 == 6 && s.r1 == 1);
		GoSlice nil = {NULL, 0, 0};
		struct Sum_return z = Sum(nil);
		CHECK(z.r0 == 0 && z.r1 == 0);
		GoSlice sq = Squares(5);
		CHECK(sq.len == 5 && sq.cap == 5);
		CHECK(((int*)sq.data)[4] == 16 && ((int*)sq.data)[0] == 0);
		free(sq.data);
		GoSlice none = Squares(0);
		CHECK(none.len == 0);
		free(none.data);
		char hello[5] = {'h', 'e', 'l', 'l', 'o'};
		GoSlice b = {hello, 5, 5};
		GoSlice rv = Reverse(b);
		CHECK(rv.len == 5 && memcmp(rv.data, "olleh", 5) == 0);
		CHECK(memcmp(hello, "hello", 5) == 0);
		free(rv.data);
	} else if (!strcmp(c, "pointers")) {
		int v = 0;
		Poke((GoInt32*)&v, 77);
		CHECK(v == 77);
		CHECK(Peek(&v) == 77);
		CHECK(CTypes(1, 2, 3, 4.9) == 1 + 2 + 3 + 4);
		int x;
		CHECK(Ident(&x) == &x);
		CHECK(Ident(NULL) == NULL);
		GoInt32* g = GlobalAddr();
		CHECK(*g == 41);
		*g = 42;
		CHECK(*GlobalAddr() == 42);
	} else if (!strcmp(c, "init")) {
		/* table (1) and init (100): once, before the first function ran. */
		CHECK(InitCount() == 101);
		CHECK(Visit(1) == 20);
		CHECK(InitCount() == 101);
	} else if (!strcmp(c, "init-threads")) {
		pthread_t th[8];
		for (int i = 0; i < 8; i++) pthread_create(&th[i], 0, thread_init, 0);
		for (int i = 0; i < 8; i++) {
			void* r;
			pthread_join(th[i], &r);
			CHECK((long)r == 101);
		}
	} else if (!strcmp(c, "callback")) {
		CHECK(Run(4) == 10 + 20 + 30 + 10);
		CHECK(Visits() == 4);
		CHECK(Run(0) == 0);
		CHECK(Visits() == 4);
	} else if (!strcmp(c, "threads")) {
		pthread_t th[8];
		for (int i = 0; i < 8; i++) pthread_create(&th[i], 0, thread_run, 0);
		long tot = 0;
		for (int i = 0; i < 8; i++) {
			void* r;
			pthread_join(th[i], &r);
			tot += (long)r;
		}
		/* each thread: 20 * (Run(5) = 10+20+30+10+20 = 90, Async(10) = 45) */
		CHECK(tot == 8 * 20 * (90 + 45));
		CHECK(Visits() == 8 * 20 * 5);
	} else if (!strcmp(c, "recover")) {
		GoString r = Recovered();
		CHECK(r.n == 5 && memcmp(r.p, "inner", 5) == 0);
		free((void*)r.p);
	} else if (!strcmp(c, "panic")) {
		printf("before\n");
		fflush(stdout);
		Boom(gs("kaboom"));
		printf("not reached\n");
	} else if (!strcmp(c, "panic-runtime")) {
		Index(7);
		printf("not reached\n");
	} else if (!strcmp(c, "panic-thread")) {
		pthread_t t;
		pthread_create(&t, 0, thread_panic, 0);
		pthread_join(t, 0);
		printf("not reached\n");
	} else if (!strcmp(c, "leak")) {
		Leak();
		printf("not reached\n");
	} else {
		printf("unknown case %s\n", c);
		return 3;
	}
	printf("pass\n");
	return 0;
}
`

// libCases are the cases of consumerC that end normally.
var libCases = []string{"numbers", "strings", "copies", "slices", "pointers", "init", "init-threads", "callback", "threads", "recover"}

// libPanics are the cases that end the process: the message that stderr starts with.
var libPanics = []struct{ name, want string }{
	{"panic", "panic: kaboom\n\ngoroutine "},
	{"panic-runtime", "panic: runtime error: index out of range [7] with length 3\n\ngoroutine "},
	{"panic-thread", "panic: from a thread\n\ngoroutine "},
	{"leak", "fatal error: gocpp: an exported function returned a pointer to gocpp memory to C"},
}

// lsanNoise is the warning of LeakSanitizer's check at exit when a thread of the runtime (a worker that
// ran goroutines) ends at the very moment the check stops the threads: it can't read the registers of a
// thread that is gone, and the check goes on (the exit status is not changed).
var lsanNoise = regexp.MustCompile(`(?m)^==\d+==Unable to get registers from thread \d+\.\n`)

// buildLib builds the files in the current folder as a library of the mode (extra gocpp flags first), and
// returns the file of the library.
func buildLib(t *testing.T, mode, out string, flags ...string) string {
	t.Helper()
	args := slices.Concat([]string{"build", "-buildmode=" + mode, "-o", out}, flags, []string{"."})
	r := gocppRun("", false, args...)
	if r.code != 0 || strings.Contains(r.stderr, "warning") {
		t.Fatalf("gocpp %s: exit %d\n%s%s", strings.Join(args, " "), r.code, r.stdout, r.stderr)
	}
	return out
}

func TestModesCArchive(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		for _, asan := range []bool{false, true} {
			name := "plain"
			var flags []string
			if asan {
				name = "asan"
				flags = []string{"-asan"}
			}
			t.Run(name, func(t *testing.T) {
				dir := inDir(t, libFixture)
				writeFiles(t, dir, map[string]string{"consumer/main.c": consumerC})
				lib := buildLib(t, "c-archive", "libfx.a", flags...)
				if _, err := os.Stat("libfx.h"); err != nil {
					t.Fatalf("the header is not next to the archive: %v", err)
				}
				prog := "prog"
				cflags := []string{lib, "-lstdc++", "-pthread", "-lm", "-ldl"}
				if asan {
					cflags = append([]string{"-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all"}, cflags...)
				}
				compileC(t, c, dir, prog, "consumer/main.c", cflags...)
				env := []string{"ASAN_OPTIONS=detect_leaks=1"}
				for _, cs := range libCases {
					t.Run(cs, func(t *testing.T) {
						out, errOut, code := runCmd(t, dir, env, filepath.Join(dir, prog), cs)
						errOut = lsanNoise.ReplaceAllString(errOut, "")
						if code != 0 || !strings.HasSuffix(out, "pass\n") || errOut != "" {
							t.Errorf("case %s: exit %d\nstdout: %s\nstderr: %s", cs, code, out, errOut)
						}
					})
				}
				for _, p := range libPanics {
					t.Run(p.name, func(t *testing.T) {
						out, errOut, code := runCmd(t, dir, env, filepath.Join(dir, prog), p.name)
						if code != 2 || !strings.HasPrefix(errOut, p.want) || strings.Contains(out, "not reached") {
							t.Errorf("case %s: exit %d, want 2 and stderr starting %q\nstdout: %s\nstderr: %s", p.name, code, p.want, out, errOut)
						}
					})
				}
				// A panic must show where it came from: the Go position of the exported function.
				_, errOut, _ := runCmd(t, dir, env, filepath.Join(dir, prog), "panic")
				if !strings.Contains(errOut, "lib.gp:") {
					t.Errorf("the trace of a panic in an exported function has no .gp position:\n%s", errOut)
				}
			})
		}
	})
}

// exportedSymbols returns the defined global symbols of the shared library lib (nm -D).
func exportedSymbols(t *testing.T, lib string) []string {
	t.Helper()
	nm, err := exec.LookPath("nm")
	if err != nil {
		t.Skip("no nm")
	}
	out, _, code := runCmd(t, filepath.Dir(lib), nil, nm, "-D", "--defined-only", lib)
	if code != 0 {
		t.Fatalf("nm -D %s: exit %d", lib, code)
	}
	var syms []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 3 {
			syms = append(syms, f[2])
		}
	}
	slices.Sort(syms)
	return syms
}

func TestModesCShared(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, libFixture)
		writeFiles(t, dir, map[string]string{"consumer/main.c": consumerC})
		lib := buildLib(t, "c-shared", "libfx.so")
		compileC(t, c, dir, "prog", "consumer/main.c", "-L.", "-lfx", "-Wl,-rpath,"+dir, "-pthread")
		for _, cs := range libCases {
			t.Run(cs, func(t *testing.T) {
				out, errOut, code := runCmd(t, dir, nil, filepath.Join(dir, "prog"), cs)
				if code != 0 || !strings.HasSuffix(out, "pass\n") || errOut != "" {
					t.Errorf("case %s: exit %d\nstdout: %s\nstderr: %s", cs, code, out, errOut)
				}
			})
		}
		for _, p := range libPanics {
			t.Run(p.name, func(t *testing.T) {
				_, errOut, code := runCmd(t, dir, nil, filepath.Join(dir, "prog"), p.name)
				if code != 2 || !strings.HasPrefix(errOut, p.want) {
					t.Errorf("case %s: exit %d, stderr:\n%s", p.name, code, errOut)
				}
			})
		}
		t.Run("symbols", func(t *testing.T) {
			// Only the exports are visible: the runtime, the C++ library's templates and the
			// packages' own functions are not.
			got := exportedSymbols(t, filepath.Join(dir, lib))
			want := slices.Sorted(slices.Values(libExports))
			if !slices.Equal(got, want) {
				t.Errorf("nm -D shows\n%v\nwant only the exports\n%v", got, want)
			}
		})
		t.Run("soname", func(t *testing.T) {
			readelf, err := exec.LookPath("readelf")
			if err != nil {
				t.Skip("no readelf")
			}
			out, _, _ := runCmd(t, dir, nil, readelf, "-d", filepath.Join(dir, lib))
			if !strings.Contains(out, "Library soname: [libfx.so]") {
				t.Errorf("no soname libfx.so:\n%s", out)
			}
		})
	})
}

// dlopenC opens each library named in the arguments, calls Run and Calls-like functions of both, closes
// them, and does it again: two gocpp runtimes in one process, loaded and unloaded.
const dlopenC = `#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>

typedef int (*run_t)(int);
typedef long long (*visits_t)(void);
typedef long long (*initcount_t)(void);

static void* open1(const char* p) {
	void* h = dlopen(p, RTLD_NOW | RTLD_LOCAL);
	if (!h) { fprintf(stderr, "dlopen %s: %s\n", p, dlerror()); exit(1); }
	return h;
}

int main(int argc, char** argv) {
	if (argc < 3) return 3;
	for (int round = 0; round < 4; round++) {
		void* a = open1(argv[1]);
		void* b = open1(argv[2]);
		run_t ra = (run_t)dlsym(a, "Run"), rb = (run_t)dlsym(b, "Run");
		visits_t va = (visits_t)dlsym(a, "Visits"), vb = (visits_t)dlsym(b, "Visits");
		initcount_t ia = (initcount_t)dlsym(a, "InitCount"), ib = (initcount_t)dlsym(b, "InitCount");
		if (!ra || !rb || !va || !vb || !ia || !ib) { fprintf(stderr, "dlsym failed\n"); return 1; }
		ra(3); ra(3); rb(1);
		/* each library has its own globals: 6 and 1 calls of Visit */
		if (va() != 6 || vb() != 1) { printf("FAIL visits %lld %lld\n", va(), vb()); return 1; }
		if (ia() != 101 || ib() != 101) { printf("FAIL init %lld %lld\n", ia(), ib()); return 1; }
		if (dlclose(a) != 0) { printf("FAIL dlclose a\n"); return 1; }
		/* the other one is alive */
		if (rb(1) != 10 || vb() != 2) { printf("FAIL b after close of a\n"); return 1; }
		if (dlclose(b) != 0) { printf("FAIL dlclose b\n"); return 1; }
	}
	printf("pass\n");
	return 0;
}
`

func TestModesTwoSharedLibraries(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		root := t.TempDir()
		var libs []string
		for _, n := range []string{"a", "b"} {
			sub := filepath.Join(root, n)
			writeFiles(t, sub, libFixture)
			t.Chdir(sub)
			libs = append(libs, filepath.Join(sub, buildLib(t, "c-shared", "lib"+n+".so")))
		}
		writeFiles(t, root, map[string]string{"d.c": dlopenC})
		compileC(t, c, root, "d", "d.c", "-ldl")
		out, errOut, code := runCmd(t, root, nil, filepath.Join(root, "d"), libs[0], libs[1])
		if code != 0 || out != "pass\n" || errOut != "" {
			t.Errorf("two libraries: exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
		}
	})
}

// A runtime check at dlclose: the libraries must not leave threads behind.
func TestModesSharedUnloadEndsThreads(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, libFixture)
		writeFiles(t, dir, map[string]string{"consumer/t.c": `#include <dlfcn.h>
#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
static int threads(void) {
	int n = 0;
	DIR* d = opendir("/proc/self/task");
	struct dirent* e;
	while ((e = readdir(d))) if (e->d_name[0] != '.') n++;
	closedir(d);
	return n;
}
int main(int argc, char** argv) {
	if (argc < 2) return 3;
	int before = threads();
	void* h = dlopen(argv[1], RTLD_NOW | RTLD_LOCAL);
	if (!h) { fprintf(stderr, "%s\n", dlerror()); return 1; }
	int (*async)(int) = (int (*)(int))dlsym(h, "Async");
	if (async(20) != 190) { printf("FAIL async\n"); return 1; }
	if (dlclose(h) != 0) { printf("FAIL dlclose\n"); return 1; }
	int after = threads();
	if (after != before) { printf("FAIL threads before %d after %d\n", before, after); return 1; }
	printf("pass\n");
	return 0;
}
`})
		lib := buildLib(t, "c-shared", "libfx.so")
		compileC(t, c, dir, "tprog", "consumer/t.c", "-ldl")
		out, errOut, code := runCmd(t, dir, nil, filepath.Join(dir, "tprog"), filepath.Join(dir, lib))
		if code != 0 || out != "pass\n" {
			t.Errorf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
		}
	})
}

// cppLibFixture is two packages used by a C++ program.
var cppLibFixture = map[string]string{
	"gocpp.mod": "module example.com/geo\n",
	"shapes/shapes.gp": `package shapes

type Point struct{ X, Y float64 }

func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

func Origin() Point { return Point{} }

var Counter int

func Next() int { Counter++; return Counter }

var started = start()

func start() int { return 7 }

func Started() int { return started }
`,
	"text/text.gp": `package text

import "example.com/geo/shapes"

func Shout(s string) string { return s + "!" }

func Squares(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i * i
	}
	return out
}

func Label(p shapes.Point) string { return "pt" }
`,
}

const cppConsumer = `#include <geo.hpp>
#include <cstdio>
#include <string>

int main() {
    gocpp_geo::init();
    gocpp_geo::init(); // again: nothing happens
    auto p = shapes::Origin().Add(shapes::Point{1, 2});
    if (p.X != 1 || p.Y != 2) return 1;
    if (shapes::Next() != 1 || shapes::Next() != 2) return 2;
    if (shapes::Started() != 7) return 3;
    auto s = text::Shout(gp::string("hi"));
    if (std::string(s.gp_view()) != "hi!") return 4;
    auto sq = text::Squares(5);
    if (sq.gp_len() != 5 || sq[4] != 16) return 5;
    if (std::string(text::Label(p).gp_view()) != "pt") return 6;
    std::printf("pass\n");
    return 0;
}
`

func TestModesCPPLib(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, cppLibFixture)
		r := gocppRun("", false, "build", "-buildmode=cpp-lib", "-o", "dist", "./shapes", "./text")
		if r.code != 0 || r.stderr != "" {
			t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
		}
		files := map[string]bool{}
		filepath.WalkDir(filepath.Join(dir, "dist"), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(filepath.Join(dir, "dist"), p)
				files[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		for _, want := range []string{"lib/libgeo.a", "include/geo.hpp", "include/gocpp/rt.hpp", "include/gpkg/example.com/geo/shapes/pkg.hpp",
			"include/gpkg/example.com/geo/text/pkg.hpp", "lib/cmake/geo/geoConfig.cmake", "lib/pkgconfig/geo.pc"} {
			if !files[want] {
				t.Errorf("the tree has no %s", want)
			}
		}
		writeFiles(t, dir, map[string]string{"app/main.cpp": cppConsumer, "app/CMakeLists.txt": `cmake_minimum_required(VERSION 3.20)
project(app CXX)
find_package(geo CONFIG REQUIRED)
add_executable(app main.cpp)
target_link_libraries(app PRIVATE gocpp::geo)
`})
		app := filepath.Join(dir, "app")
		t.Run("cmake", func(t *testing.T) {
			cmake, err := exec.LookPath("cmake")
			if err != nil {
				t.Skip("no cmake")
			}
			cxxPath := c.cxx.Path
			out, errOut, code := runCmd(t, app, nil, cmake, "-S", ".", "-B", "build", "-DCMAKE_PREFIX_PATH="+filepath.Join(dir, "dist"), "-DCMAKE_CXX_COMPILER="+cxxPath)
			if code != 0 {
				t.Fatalf("cmake: exit %d\n%s%s", code, out, errOut)
			}
			out, errOut, code = runCmd(t, app, nil, cmake, "--build", "build")
			if code != 0 {
				t.Fatalf("cmake --build: exit %d\n%s%s", code, out, errOut)
			}
			out, errOut, code = runCmd(t, app, nil, filepath.Join(app, "build", "app"))
			if code != 0 || out != "pass\n" {
				t.Errorf("app: exit %d\n%s%s", code, out, errOut)
			}
		})
		t.Run("pkg-config", func(t *testing.T) {
			pc, err := exec.LookPath("pkg-config")
			if err != nil {
				t.Skip("no pkg-config")
			}
			env := []string{"PKG_CONFIG_PATH=" + filepath.Join(dir, "dist", "lib", "pkgconfig")}
			flags, errOut, code := runCmd(t, app, env, pc, "--cflags", "--libs", "geo")
			if code != 0 {
				t.Fatalf("pkg-config: exit %d\n%s", code, errOut)
			}
			// the program first: a static library only resolves what the objects before it need
			args := append([]string{"main.cpp"}, strings.Fields(flags)...)
			args = append(args, "-o", "app-pc")
			o, e, code := runCmd(t, app, nil, c.cxx.Path, append(c.cxx.Args, args...)...)
			if code != 0 {
				t.Fatalf("%s %v: exit %d\n%s%s", c.cxx.Path, args, code, o, e)
			}
			out, errOut, code := runCmd(t, app, nil, filepath.Join(app, "app-pc"))
			if code != 0 || out != "pass\n" {
				t.Errorf("app: exit %d\n%s%s", code, out, errOut)
			}
		})
		t.Run("config-text", func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(dir, "dist", "lib", "cmake", "geo", "geoConfig.cmake"))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"gocpp::geo", "-std=c++20", "-fwrapv", "-fno-strict-aliasing", "Threads::Threads"} {
				if !strings.Contains(string(b), want) {
					t.Errorf("geoConfig.cmake does not have %q", want)
				}
			}
		})
	})
}

// The errors of the modes: nothing is compiled, the message is at the .gp line and the status is 1.
func TestModesExportErrors(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	t.Setenv("GOCPP_CXX", anyCXX(t).name)
	cases := []struct {
		name, src, want string
	}{
		{"struct", "type S struct{ a int }\n\n//export F\nfunc F(s S) {}\n", "a struct can't cross the boundary"},
		{"map", "//export F\nfunc F(m map[string]int) {}\n", "a map can't cross the boundary"},
		{"func", "//export F\nfunc F(f func()) {}\n", "a func can't cross the boundary"},
		{"interface", "//export F\nfunc F(x interface{}) {}\n", "an interface can't cross the boundary"},
		{"error", "//export F\nfunc F() error { return nil }\n", "an error can't cross the boundary"},
		{"chan", "//export F\nfunc F(c chan int) {}\n", "a channel can't cross the boundary"},
		{"array", "//export F\nfunc F(a [3]int) {}\n", "an array can't cross the boundary"},
		{"variadic", "//export F\nfunc F(a ...int) {}\n", "a variadic function can't be exported"},
		{"method", "type T struct{}\n\n//export M\nfunc (T) M() {}\n", "a method can't be exported"},
		{"generic", "//export G\nfunc G[T any](x T) {}\n", "a generic function can't be exported"},
		{"name", "//export Other\nfunc F() {}\n", "the name must be the function's own: F"},
		{"blank-line", "//export F\n\nfunc F() {}\n", "//export must be in the doc comment of a function"},
		{"reserved", "//export GoInt\nfunc GoInt() {}\n", "the name is one that gocpp's header uses"},
		{"c-keyword", "//export auto\nfunc auto() {}\n", "it is a C or C++ keyword"},
		{"pointer-to-struct", "type S struct{ a int }\n\n//export F\nfunc F(p *S) {}\n", "a pointer to a Go type with gocpp's own layout"},
		{"slice-of-strings", "//export F\nfunc F(x []string) {}\n", "only a slice of numbers or bools can cross the boundary"},
		{"pointer-to-string", "//export F\nfunc F(p *string) {}\n", "a pointer to a Go type with gocpp's own layout"},
		{"complex", "//export F\nfunc F(c complex128) {}\n", "complex numbers are not supported"},
		{"init", "//export init\nfunc init() {}\n", "init can't be exported"},
		{"libc-name", "//export write\nfunc write(fd int) int { return fd }\n", "the name is one of the C library"},
		{"libc-symbol-not-in-list", "//export getpagesize\nfunc getpagesize(n int) int { return n }\n", "is a symbol of the C library"},
		{"libc-symbol-64", "//export mmap64\nfunc mmap64(n int) int { return n }\n", "is a symbol of the C library"},
		{"reserved-name", "//export __x\nfunc __x() {}\n", "the name is one of the C library or reserved"},
		{"usage", "//export \nfunc F() {}\n", "usage: //export Name"},
		{"two-names", "//export F G\nfunc F() {}\n", "usage: //export Name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inDir(t, map[string]string{"gocpp.mod": "module example.com/e\n", "a.gp": "package main\n\n" + tc.src + "\nfunc main() {}\n"})
			for _, mode := range []string{"c-archive", "c-shared"} {
				r := gocppRun("", false, "build", "-buildmode="+mode, "-o", "libe", ".")
				if r.code != 1 || !strings.Contains(r.stderr, tc.want) || !strings.Contains(r.stderr, "a.gp:") {
					t.Errorf("%s: exit %d, stderr:\n%s\nwant status 1 and %q at a.gp", mode, r.code, r.stderr, tc.want)
				}
				if _, err := os.Stat("libe.a"); err == nil {
					t.Errorf("%s wrote a library", mode)
				}
			}
		})
	}
}

func TestModesOtherErrors(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	t.Setenv("GOCPP_CXX", anyCXX(t).name)
	t.Run("unknown-mode", func(t *testing.T) {
		inDir(t, map[string]string{"a.gp": "package main\n\nfunc main() {}\n"})
		r := gocppRun("", false, "build", "-buildmode=plugin", "a.gp")
		if r.code != 2 || !strings.Contains(r.stderr, `unknown build mode "plugin" (one of: exe, c-archive, c-shared, cpp-lib)`) {
			t.Errorf("exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("no-exports", func(t *testing.T) {
		inDir(t, map[string]string{"a.gp": "package main\n\nfunc F() {}\n\nfunc main() {}\n"})
		r := gocppRun("", false, "build", "-buildmode=c-archive", "a.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "the program has no //export functions") {
			t.Errorf("exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("no-main", func(t *testing.T) {
		inDir(t, map[string]string{"a.gp": "package main\n\n//export F\nfunc F() {}\n"})
		r := gocppRun("", false, "build", "-buildmode=c-shared", "a.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "function main is undeclared in the main package") {
			t.Errorf("exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("several-errors", func(t *testing.T) {
		inDir(t, map[string]string{"a.gp": "package main\n\n//export F\nfunc F(m map[int]int) {}\n\n//export G\nfunc G(c chan int) {}\n\nfunc main() {}\n"})
		r := gocppRun("", false, "build", "-buildmode=c-archive", "a.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "a.gp:3:1:") || !strings.Contains(r.stderr, "a.gp:6:1:") {
			t.Errorf("both errors are expected, in order:\n%s", r.stderr)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		inDir(t, map[string]string{
			"gocpp.mod": "module example.com/d\n",
			"a/a.gp":    "package a\n\n//export F\nfunc F() {}\n\nfunc X() {}\n",
			"main.gp":   "package main\n\nimport \"example.com/d/a\"\n\n//export F\nfunc F() { a.X() }\n\nfunc main() {}\n",
		})
		r := gocppRun("", false, "build", "-buildmode=c-archive", "-o", "libd.a", ".")
		if r.code != 1 || !strings.Contains(r.stderr, "//export F: the name is exported twice") {
			t.Errorf("exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("export-in-exe", func(t *testing.T) {
		// A program ignores the directive (the gate lets it through at this phase).
		inDir(t, map[string]string{"a.gp": "package main\n\n//export F\nfunc F() {}\n\nfunc main() { F(); println(\"ran\") }\n"})
		r := gocppRun("", true, "run", "a.gp")
		if r.code != 0 || r.stdout != "ran\n" {
			t.Errorf("exit %d\n%s", r.code, r.stdout)
		}
	})
	t.Run("run-has-no-buildmode", func(t *testing.T) {
		inDir(t, map[string]string{"a.gp": "package main\n\nfunc main() {}\n"})
		r := gocppRun("", false, "run", "-buildmode=c-archive", "a.gp")
		if r.code != 2 {
			t.Errorf("exit %d, want the flag error 2\n%s", r.code, r.stderr)
		}
	})
}

func TestModesExeExplicit(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		r := gocppRun("", false, "build", "-buildmode=exe", "-o", "hello", "hello.gp")
		if r.code != 0 {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
		if out, _, code := runCmd(t, ".", nil, "./hello"); code != 0 || out != "hello, gocpp 12 true\nafter\n" {
			t.Errorf("exit %d\n%s", code, out)
		}
	})
}

// The up-to-date check of library builds: the mode is in the key, and a missing header is built again.
func TestModesUpToDate(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	t.Setenv("GOCPP_NOUPTODATE", "")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/u\n", "a.gp": "package main\n\n//export F\nfunc F() int32 { return 1 }\n\nfunc main() {}\n"})
		build1 := func(mode, out string) string {
			r := gocppRun("", false, "build", "-x", "-buildmode="+mode, "-o", out, ".")
			if r.code != 0 {
				t.Fatalf("%s: exit %d\n%s", mode, r.code, r.stderr)
			}
			return r.stderr
		}
		first := build1("c-archive", "libu.a")
		if strings.Contains(first, "# up to date") {
			t.Fatalf("the first build was up to date:\n%s", first)
		}
		if again := build1("c-archive", "libu.a"); !strings.Contains(again, "# up to date: "+filepath.Join(dir, "libu.a")) {
			t.Errorf("the second build was not up to date:\n%s", again)
		}
		// Another mode with the same name is another build.
		if sh := build1("c-shared", "libu.a"); strings.Contains(sh, "# up to date") {
			t.Errorf("c-shared with the file of the c-archive was up to date:\n%s", sh)
		}
		if again := build1("c-archive", "libu.a"); strings.Contains(again, "# up to date") {
			t.Errorf("the archive was overwritten by c-shared, the build can't be up to date:\n%s", again)
		}
		// The header is an output too.
		os.Remove("libu.h")
		if again := build1("c-archive", "libu.a"); strings.Contains(again, "# up to date") {
			t.Errorf("a missing header must make the build run:\n%s", again)
		} else if _, err := os.Stat("libu.h"); err != nil {
			t.Errorf("the header was not written again: %v", err)
		}
		if again := build1("c-archive", "libu.a"); !strings.Contains(again, "# up to date") {
			t.Errorf("not up to date after the rebuild:\n%s", again)
		}
		// A changed header is not trusted.
		os.WriteFile("libu.h", []byte("// changed\n"), 0o644)
		if again := build1("c-archive", "libu.a"); strings.Contains(again, "# up to date") {
			t.Errorf("a changed header must make the build run:\n%s", again)
		}
		// A source change that changes the library.
		os.WriteFile("a.gp", []byte("package main\n\n//export F\nfunc F() int32 { return 2 }\n\nfunc main() {}\n"), 0o644)
		if again := build1("c-archive", "libu.a"); strings.Contains(again, "# up to date") {
			t.Errorf("a changed source must make the build run:\n%s", again)
		}
	})
}

// The actions of a library build: -x shows the archive command and the flags of the mode.
func TestModesTraceActions(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"a.gp": "package main\n\n//export F\nfunc F() int32 { return 1 }\n\nfunc main() {}\n"})
		r := gocppRun("", false, "build", "-x", "-buildmode=c-archive", "-o", "liba.a", "a.gp")
		if r.code != 0 {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, " -fPIC ") {
			t.Errorf("no -fPIC in the commands:\n%s", r.stderr)
		}
		if !regexp.MustCompile(`(?m)^\S*(gcc-ar|llvm-ar|/ar) rcsD `).MatchString(r.stderr) {
			t.Errorf("no archive command in -x:\n%s", r.stderr)
		}
		if strings.Contains(r.stderr, "-fvisibility=hidden") {
			t.Errorf("a static archive is not hidden:\n%s", r.stderr)
		}
		r = gocppRun("", false, "build", "-x", "-buildmode=c-shared", "-o", "liba.so", "a.gp")
		if r.code != 0 {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
		for _, want := range []string{"-fvisibility=hidden", "-fvisibility-inlines-hidden", "-shared", "-Wl,--exclude-libs,ALL", "-Wl,-soname,liba.so", "-Wl,--version-script="} {
			if !strings.Contains(r.stderr, want) {
				t.Errorf("no %s in the commands of c-shared:\n%s", want, r.stderr)
			}
		}
		if c.cxx.Family == build.GCC && !strings.Contains(r.stderr, "-fno-gnu-unique") {
			t.Errorf("g++ needs -fno-gnu-unique for c-shared")
		}
	})
}

// The archive is deterministic: two builds from the same objects give the same bytes (no times or owners
// in the members). The object cache is on: a compile writes the folder of its build into the object.
func TestModesArchiveDeterministic(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"a.gp": "package main\n\n//export F\nfunc F() int32 { return 1 }\n\nfunc main() {}\n"})
		var sums [][]byte
		// The name of the file is in the header and the glue: the same name in two folders.
		for _, out := range []string{"one/libd.a", "two/libd.a"} {
			os.MkdirAll(filepath.Dir(out), 0o755)
			if r := gocppRun("", false, "build", "-buildmode=c-archive", "-o", out, "a.gp"); r.code != 0 {
				t.Fatalf("exit %d\n%s", r.code, r.stderr)
			}
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			sums = append(sums, b)
		}
		if !bytes.Equal(sums[0], sums[1]) {
			t.Errorf("two builds of the same program give different archives (%d and %d bytes)", len(sums[0]), len(sums[1]))
		}
	})
}

// The header: cgo's names, valid C and C++, with the doc comment of the function.
func TestModesHeader(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, libFixture)
		buildLib(t, "c-archive", "libfx.a")
		h, err := os.ReadFile("libfx.h")
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"typedef long long GoInt64;", "typedef GoInt64 GoInt;", "typedef _GoString_ GoString;", "} GoSlice;",
			"extern GOCPP_EXPORT GoInt32 Add(GoInt32 a, GoInt32 b);", "extern GOCPP_EXPORT GoString Upper(GoString s);",
			"struct Sum_return {", "extern GOCPP_EXPORT struct Sum_return Sum(GoSlice xs);", "Upper returns s in upper case (ASCII).",
			"extern GOCPP_EXPORT void Boom(GoString msg);", "extern GOCPP_EXPORT int Peek(int* p);", "extern GOCPP_EXPORT void* Ident(void* p);",
			"extern GOCPP_EXPORT GoInt32* GlobalAddr(void);", "extern GOCPP_EXPORT GoUint8 Not(GoUint8 b);", "extern GOCPP_EXPORT long CTypes(int a, long b, unsigned char c, double d);"} {
			if !strings.Contains(string(h), want) {
				t.Errorf("the header does not have %q:\n%s", want, h)
			}
		}
		// It compiles as C (the consumer) and as C++, with the C++ compiler of the build.
		writeFiles(t, dir, map[string]string{"consumer/h.cpp": "#include \"libfx.h\"\nint main() { return Add(1, 2) == 3 ? 0 : 1; }\n"})
		o, e, code := runCmd(t, dir, nil, c.cxx.Path, append(slices.Clone(c.cxx.Args), "-std=c++20", "-Wall", "-Wextra", "-Wcomment", "-Wtrigraphs", "-Werror", "-Wno-unused-command-line-argument", "-I", dir, "-fsyntax-only", "consumer/h.cpp")...)
		if code != 0 {
			t.Errorf("the header does not compile as C++: exit %d\n%s%s", code, o, e)
		}
		cc := cCompiler(t, c)
		o, e, code = runCmd(t, dir, nil, cc[0], append(slices.Clone(cc[1:]), "-std=c99", "-pedantic", "-Wall", "-Wextra", "-Wcomment", "-Wtrigraphs", "-trigraphs", "-Werror", "-Wno-unused-command-line-argument", "-fsyntax-only", "-x", "c", "libfx.h")...)
		if code != 0 {
			t.Errorf("the header does not compile as C99: exit %d\n%s%s", code, o, e)
		}
	})
}

// Library names: -o picks the file and the header.
func TestModesNames(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	t.Setenv("GOCPP_CXX", anyCXX(t).name)
	src := "package main\n\n//export F\nfunc F() int32 { return 1 }\n\nfunc main() {}\n"
	inDir(t, map[string]string{"hello.gp": src})
	if r := gocppRun("", false, "build", "-buildmode=c-archive", "hello.gp"); r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	for _, f := range []string{"libhello.a", "libhello.h"} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("default names: %v", err)
		}
	}
	if err := os.Mkdir("out", 0o755); err != nil {
		t.Fatal(err)
	}
	if r := gocppRun("", false, "build", "-buildmode=c-shared", "-o", "out/", "hello.gp"); r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	for _, f := range []string{"out/libhello.so", "out/libhello.h"} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("-o folder: %v", err)
		}
	}
	if r := gocppRun("", false, "build", "-buildmode=c-archive", "-o", "out/mylib.a", "hello.gp"); r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	if _, err := os.Stat("out/mylib.h"); err != nil {
		t.Errorf("-o file: %v", err)
	}
}
