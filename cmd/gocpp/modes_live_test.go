//go:build linux

package main

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The unload of a c-shared library that has goroutines (ADR-043, "Unloading"): a library with a
// running or blocked goroutine is never unmapped (dlclose gives back the host's reference only: no
// crash, no stall, no output), a library without goroutines is unloaded, and the end of the process
// does not wait for goroutines.

// liveFixture is a library whose exported functions start goroutines that stay.
var liveFixture = map[string]string{
	"gocpp.mod": "module example.com/live\n",
	"a.gp": `package main

var block = make(chan int)
var released bool
var counter int64
var done = make(chan struct{}, 1)
var doneCount int

//export Ping
func Ping() int { return 1 }

//export Spawn
func Spawn() {
	go func() {
		<-block
		done <- struct{}{}
		doneCount++
		<-done
	}()
}

//export SpawnMany
func SpawnMany(n int) {
	for i := 0; i < n; i++ {
		Spawn()
	}
}

//export SpawnBusy
func SpawnBusy() {
	go func() {
		for {
			counter++
		}
	}()
}

// Release lets the goroutines of Spawn end.
//
//export Release
func Release() {
	if !released {
		released = true
		close(block)
	}
}

//export Done
func Done() int {
	done <- struct{}{}
	n := doneCount
	<-done
	return n
}

//export Counter
func Counter() int64 { return counter }

// Short starts a goroutine that ends at once.
//
//export Short
func Short() { go func() {}() }

//export Wait1
func Wait1() int {
	c := make(chan int)
	go func() { c <- 1 }()
	return <-c
}

func main() {}
`,
}

// liveHost is the host program: the argument names the case, the others are the libraries.
const liveHost = `#define _GNU_SOURCE
#include <dlfcn.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

typedef struct { void* h; int (*ping)(void); void (*spawn)(void); void (*many)(long); void (*busy)(void);
	void (*release)(void); long (*done)(void); long long (*counter)(void); int (*wait1)(void); } lib;

#define CHECK(c) do { if (!(c)) { printf("FAIL line %d: %s\n", __LINE__, #c); fflush(stdout); exit(1); } } while (0)

static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec + t.tv_nsec / 1e9; }
static void pause_ms(int ms) { struct timespec t = {0, ms * 1000 * 1000}; nanosleep(&t, 0); }

static lib open_lib(const char* path) {
	lib l;
	l.h = dlopen(path, RTLD_NOW | RTLD_LOCAL);
	if (!l.h) { printf("dlopen: %s\n", dlerror()); exit(9); }
	l.ping = (int (*)(void))dlsym(l.h, "Ping");
	l.spawn = (void (*)(void))dlsym(l.h, "Spawn");
	l.many = (void (*)(long))dlsym(l.h, "SpawnMany");
	l.busy = (void (*)(void))dlsym(l.h, "SpawnBusy");
	l.release = (void (*)(void))dlsym(l.h, "Release");
	l.done = (long (*)(void))dlsym(l.h, "Done");
	l.counter = (long long (*)(void))dlsym(l.h, "Counter");
	l.wait1 = (int (*)(void))dlsym(l.h, "Wait1");
	CHECK(l.ping() == 1 && l.wait1() == 1);
	return l;
}

/* mapped reports whether a file with this name is mapped into the process. */
static int mapped(const char* name) {
	FILE* f = fopen("/proc/self/maps", "r");
	char line[4096];
	int found = 0;
	while (fgets(line, sizeof line, f)) if (strstr(line, name)) found = 1;
	fclose(f);
	return found;
}

/* copy_file makes a file of its own from src: another file is another library instance. */
static void copy_file(const char* src, const char* dst) {
	FILE* in = fopen(src, "rb");
	FILE* out = fopen(dst, "wb");
	char buf[65536];
	size_t n;
	CHECK(in && out);
	while ((n = fread(buf, 1, sizeof buf, in)) > 0) CHECK(fwrite(buf, 1, n, out) == n);
	fclose(in);
	fclose(out);
}

static volatile int stop_pingers;
static void* pinger(void* a) {
	lib* l = (lib*)a;
	while (!stop_pingers) l->ping();
	return 0;
}
static void* spawner(void* a) {
	lib* l = (lib*)a;
	for (int i = 0; i < 300; i++) l->wait1(); /* goroutines that start and end while the pingers call */
	return 0;
}

static double t_main_end;
static void at_exit(void) { printf("exit-ms %d\n", (int)((now() - t_main_end) * 1000)); fflush(stdout); }

int main(int argc, char** argv) {
	const char* c = argv[1];
	const char* path = argv[2];
	const char* base = strrchr(path, '/') ? strrchr(path, '/') + 1 : path;
	atexit(at_exit); /* the first registered: runs after the library's own hooks */
	if (!strcmp(c, "quiescent")) {
		lib l = open_lib(path);
		CHECK(mapped(base));
		CHECK(dlclose(l.h) == 0);
		CHECK(!mapped(base)); /* no goroutine: the library is unloaded */
	} else if (!strcmp(c, "busy") || !strcmp(c, "blocked") || !strcmp(c, "many")) {
		lib l = open_lib(path);
		if (!strcmp(c, "busy")) l.busy();
		else if (!strcmp(c, "blocked")) l.spawn();
		else l.many(50);
		double t = now();
		CHECK(dlclose(l.h) == 0);
		CHECK(now() - t < 1.5); /* not the 3 s wait of an unload that waits for the goroutines */
		CHECK(mapped(base));    /* still there: its code runs the goroutines */
		CHECK(l.ping() == 1);   /* and works */
		if (!strcmp(c, "busy")) {
			long long a = l.counter();
			for (int i = 0; i < 5000 && l.counter() <= a; i++) pause_ms(1); /* it gets the CPU in time */
			CHECK(l.counter() > a); /* the busy goroutine still runs */
		}
		/* A library that is opened again is the same one. */
		lib again = open_lib(path);
		CHECK(again.ping == l.ping);
	} else if (!strcmp(c, "released")) {
		/* The goroutine ends: the next call gives the reference back, and dlclose unloads. */
		lib l = open_lib(path);
		l.spawn();
		l.release();
		for (int i = 0; i < 2000 && l.done() != 1; i++) pause_ms(1);
		CHECK(l.done() == 1);
		for (int i = 0; i < 300; i++) { CHECK(l.ping() == 1); pause_ms(1); }
		CHECK(dlclose(l.h) == 0);
		CHECK(!mapped(base));
	} else if (!strcmp(c, "finishing")) {
		/* A goroutine that ends while the host unloads: either way nothing breaks. */
		for (int i = 0; i < 40; i++) {
			lib l = open_lib(path);
			l.spawn();
			l.release();
			CHECK(dlclose(l.h) == 0);
		}
	} else if (!strcmp(c, "threads")) {
		/* Several host threads call the library while goroutines start and end: the last call that
		   starts a goroutine that stays must keep the library mapped, whatever the others do. Each
		   round is a library instance of its own (a copy of the file), because only the first
		   goroutine of an instance can race with the calls that give the reference back. */
		for (int round = 0; round < 24; round++) {
			char copy[4096];
			snprintf(copy, sizeof copy, "%s.copy%d.so", path, round);
			copy_file(path, copy);
			lib l = open_lib(copy);
			const char* cbase = strrchr(copy, '/') + 1;
			pthread_t ps[3], ss[2];
			stop_pingers = 0;
			for (int i = 0; i < 3; i++) pthread_create(&ps[i], 0, pinger, &l);
			for (int i = 0; i < 2; i++) pthread_create(&ss[i], 0, spawner, &l);
			for (int i = 0; i < 2; i++) pthread_join(ss[i], 0);
			l.spawn(); /* a goroutine that stays, started while the pingers still call */
			stop_pingers = 1;
			for (int i = 0; i < 3; i++) pthread_join(ps[i], 0);
			CHECK(dlclose(l.h) == 0);
			CHECK(mapped(cbase)); /* the goroutine is alive: the library is not unmapped */
			CHECK(l.ping() == 1);
			remove(copy);
		}
	} else if (!strcmp(c, "two")) {
		lib a = open_lib(argv[2]);
		lib b = open_lib(argv[3]);
		a.spawn();
		b.wait1();
		CHECK(dlclose(a.h) == 0);
		CHECK(dlclose(b.h) == 0);
		CHECK(mapped("liba.so"));  /* the one with a goroutine stays */
		CHECK(!mapped("libb.so")); /* the other one is unloaded */
		CHECK(a.ping() == 1);
	} else if (!strcmp(c, "exit-blocked") || !strcmp(c, "exit-busy") || !strcmp(c, "exit-many")) {
		lib l = open_lib(path);
		if (!strcmp(c, "exit-busy")) l.busy();
		else if (!strcmp(c, "exit-blocked")) l.spawn();
		else l.many(100);
		/* the process ends with the library loaded and a goroutine alive */
	} else if (!strcmp(c, "exit-closed")) {
		lib l = open_lib(path);
		l.spawn();
		CHECK(dlclose(l.h) == 0);
	} else {
		printf("unknown case %s\n", c);
		return 3;
	}
	printf("pass\n");
	fflush(stdout);
	t_main_end = now();
	return 0;
}
`

var exitMs = regexp.MustCompile(`exit-ms (\d+)`)

func TestModesUnloadLive(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", libGate)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		root := t.TempDir()
		var libs []string
		for _, n := range []string{"a", "b"} {
			sub := filepath.Join(root, n)
			writeFiles(t, sub, liveFixture)
			t.Chdir(sub)
			libs = append(libs, filepath.Join(sub, buildLib(t, "c-shared", "lib"+n+".so")))
		}
		writeFiles(t, root, map[string]string{"host.c": liveHost})
		compileC(t, c, root, "host", "host.c", "-ldl", "-pthread")
		run := func(t *testing.T, args ...string) (string, int) {
			out, errOut, code := runCmd(t, root, nil, filepath.Join(root, "host"), args...)
			if errOut != "" {
				t.Errorf("%v: stderr is not empty (a dlclose and an exit print nothing):\n%s", args, errOut)
			}
			return out, code
		}
		for _, cs := range []string{"quiescent", "busy", "blocked", "many", "released", "finishing", "threads"} {
			t.Run(cs, func(t *testing.T) {
				out, code := run(t, cs, libs[0])
				if code != 0 || !strings.Contains(out, "pass\n") {
					t.Errorf("exit %d\n%s", code, out)
				}
			})
		}
		t.Run("two", func(t *testing.T) {
			out, code := run(t, "two", libs[0], libs[1])
			if code != 0 || !strings.Contains(out, "pass\n") {
				t.Errorf("exit %d\n%s", code, out)
			}
		})
		// The end of the process does not wait for goroutines (the wait was 3 s); the margin is large
		// because the machine may be loaded.
		for _, cs := range []string{"exit-blocked", "exit-busy", "exit-many", "exit-closed"} {
			t.Run(cs, func(t *testing.T) {
				out, code := run(t, cs, libs[0])
				m := exitMs.FindStringSubmatch(out)
				if code != 0 || m == nil || !strings.Contains(out, "pass\n") {
					t.Fatalf("exit %d\n%s", code, out)
				}
				if ms, _ := strconv.Atoi(m[1]); ms > 1500 {
					t.Errorf("the library took %d ms to end the process with a goroutine alive", ms)
				}
			})
		}
	})
}
