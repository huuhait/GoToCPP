package probe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The precompiled header with a compiler that is the test binary (cache_fake_test.go, fakeHeaderBuild): the
// tests that need to see when a build starts, how long it lives and whether it ended. They wait for files
// that the fake writes, never for a time, and a wait that gets no file ends the test after a bound.

// pchFake is a fake compiler that builds headers, with files for its markers.
type pchFake struct {
	*fake
	start, beat, end string
}

func newPCHFake(t *testing.T) *pchFake {
	t.Helper()
	f := newFake(t)
	f.header(t, "sys.hpp", "// a library header\n")
	p := &pchFake{fake: f}
	markers := t.TempDir()
	p.start = filepath.Join(markers, "start")
	p.beat = filepath.Join(markers, "beat")
	p.end = filepath.Join(markers, "end")
	t.Setenv("FAKE_PCH_START", p.start)
	t.Setenv("FAKE_PCH_BEAT", p.beat)
	t.Setenv("FAKE_PCH_END", p.end)
	t.Setenv("FAKE_PCH_DEP", filepath.Join(f.inc, "sys.hpp"))
	t.Setenv("FAKE_PCH_SLEEP", "")
	t.Setenv("FAKE_PCH_BYTES", "")
	t.Setenv(EnvPCH, "wait")
	return p
}

func (p *pchFake) sleep(t *testing.T, seconds string) { t.Setenv("FAKE_PCH_SLEEP", seconds) }

// waitStarted waits (at most 20 s) until a build has started.
func (p *pchFake) waitStarted(t *testing.T) {
	t.Helper()
	for i := 0; i < 800; i++ {
		if _, err := os.Stat(p.start); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("no build started")
}

func (p *pchFake) ended() bool {
	_, err := os.Stat(p.end)
	return err == nil
}

// stopsBeating reports whether the fake compiler, that was told to wait, is no longer running: its heartbeat file
// does not grow. (A process that lives would have written 10 times in the 100 ms.)
func (p *pchFake) stopsBeating() bool {
	before, err := os.Stat(p.beat)
	if err != nil {
		return true
	}
	time.Sleep(100 * time.Millisecond)
	after, err := os.Stat(p.beat)
	return err == nil && after.Size() == before.Size()
}

func (p *pchFake) pchDir() string { return filepath.Join(p.cache, "pch") }

func (p *pchFake) filesLeft(t *testing.T, pattern string) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(p.pchDir(), "*", pattern))
	return files
}

// A header that is being made is stopped when the command is (the compiler is killed), when WaitPCH has
// waited for the cap, and nothing of the build stays but the source.
func TestPCHBuildEndsWithTheCommandOrTheCap(t *testing.T) {
	for _, how := range []string{"interrupt", "cap"} {
		t.Run(how, func(t *testing.T) {
			p := newPCHFake(t)
			p.sleep(t, "120")
			defer func(old time.Duration) { pchWaitCap = old }(pchWaitCap)
			pchWaitCap = 100 * time.Millisecond
			command, interrupt := context.WithCancel(context.Background())
			defer interrupt()
			cfg := p.cfg()
			cfg.Timeout = time.Minute // (the build's own limit: a build that ends sooner was stopped by the cause)
			rn := new(Runner)
			rn.SetBuildContext(command)
			rn.Prewarm(context.Background(), cfg)
			p.waitStarted(t)
			if how == "interrupt" {
				pchWaitCap = time.Hour
				interrupt()
			}
			returned := make(chan struct{})
			go func() {
				rn.WaitPCH(context.Background())
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(30 * time.Second):
				t.Fatal("WaitPCH did not return before the limit of the build")
			}
			if p.ended() {
				t.Error("the build ran to its end")
			}
			if !p.stopsBeating() {
				t.Error("the compiler of the build is still running")
			}
			if left := p.filesLeft(t, ".tmp-*"); len(left) != 0 {
				t.Errorf("temporary files left: %v", left)
			}
			if left := p.filesLeft(t, "*.json"); len(left) != 0 {
				t.Errorf("a description of a build that did not end: %v", left)
			}
		})
	}
}

// WaitPCH returns when the build is done: the header and its description are there.
func TestWaitPCHReturnsWhenTheBuildIsDone(t *testing.T) {
	p := newPCHFake(t)
	p.sleep(t, "0.3")
	ResetCacheCounters()
	rn := new(Runner)
	rn.Prewarm(context.Background(), p.cfg())
	rn.WaitPCH(context.Background())
	if !p.ended() {
		t.Error("WaitPCH returned before the compiler was done")
	}
	if len(p.filesLeft(t, "*.pch")) != 1 || len(p.filesLeft(t, "*.json")) != 1 || len(p.filesLeft(t, ".tmp-*")) != 0 {
		t.Errorf("files: %v", p.filesLeft(t, "*"))
	}
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Errorf("%d builds", c.PCHBuilds)
	}
}

// A call that makes a header (a probe run, a declaration run) does not return before the build is done.
func TestCallsWaitForTheBuildTheyStarted(t *testing.T) {
	t.Run("Probe", func(t *testing.T) {
		p := newPCHFake(t)
		t.Setenv(EnvPCH, "") // (not "wait": a run that waits for the header would not show the call's own wait)
		p.sleep(t, "1")
		probeWith(t, p.fake, new(Runner), fnSet("f0"), "f0")
		if !p.ended() {
			t.Error("Probe returned before the compiler of the header was done")
		}
	})
	t.Run("Decls", func(t *testing.T) {
		p := newPCHFake(t)
		t.Setenv(EnvPCH, "")
		p.sleep(t, "1")
		dc := DeclConfig{Includes: []string{`"a.hpp"`}}
		p.header(t, "a.hpp", "// a\n")
		if _, err := new(Runner).Decls(context.Background(), p.cfg(), dc, []DeclQuery{{DeclFunc, "f0"}}); err != nil {
			t.Fatal(err)
		}
		if !p.ended() {
			t.Error("Decls returned before the compiler of the header was done")
		}
	})
}

// Two processes that want the same header make it once: the second takes the lock after the first is done and
// finds the files.
func TestPCHIsMadeOnceByTwoRunners(t *testing.T) {
	p := newPCHFake(t)
	p.sleep(t, "0.5")
	ResetCacheCounters()
	first, second := new(Runner), new(Runner)
	first.Prewarm(context.Background(), p.cfg())
	p.waitStarted(t)
	second.Prewarm(context.Background(), p.cfg())
	var wg sync.WaitGroup
	for _, rn := range []*Runner{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rn.WaitPCH(context.Background())
		}()
	}
	wg.Wait()
	if p.count("H") != 1 {
		t.Errorf("the compiler made %d headers", p.count("H"))
	}
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Errorf("%d builds", c.PCHBuilds)
	}
}

// What a run does with a build depends on the age of the build (the 200 ms and 400 ms of the rules are
// variables here, and the age is set by hand): a young build is not waited for, an old one is.
func TestPCHWaitRuleByTheAgeOfTheBuild(t *testing.T) {
	setAge := func(rn *Runner, age time.Duration) {
		rn.mu.Lock()
		defer rn.mu.Unlock()
		for _, st := range rn.pchs {
			st.started = time.Now().Add(-age)
		}
	}
	t.Run("young_is_left_alone", func(t *testing.T) {
		p := newPCHFake(t)
		p.sleep(t, "120")
		t.Setenv(EnvPCH, "")
		defer func(old time.Duration) { pchMaxWait = old }(pchMaxWait)
		pchMaxWait = time.Hour // (a wrong rule would wait for this)
		command, interrupt := context.WithCancel(context.Background())
		rn := new(Runner)
		rn.SetBuildContext(command)
		rn.Prewarm(context.Background(), p.cfg())
		p.waitStarted(t)
		got := make(chan *pchRef, 1)
		go func() { got <- rn.pchPath(context.Background(), p.cfg(), pchUse) }()
		select {
		case ref := <-got:
			if ref != nil {
				t.Error("a header from a build that is not done")
			}
		case <-time.After(20 * time.Second):
			t.Error("a run waited for a build that is young")
		}
		interrupt()
		rn.WaitPCH(context.Background())
	})
	t.Run("old_is_waited_for", func(t *testing.T) {
		p := newPCHFake(t)
		p.sleep(t, "0.5")
		t.Setenv(EnvPCH, "")
		defer func(old time.Duration) { pchMaxWait = old }(pchMaxWait)
		pchMaxWait = time.Minute
		rn := new(Runner)
		rn.Prewarm(context.Background(), p.cfg())
		p.waitStarted(t)
		setAge(rn, time.Hour)
		if ref := rn.pchPath(context.Background(), p.cfg(), pchUse); ref == nil {
			t.Error("a run did not wait for a build that is old")
		}
		rn.WaitPCH(context.Background())
	})
	t.Run("start_never_waits", func(t *testing.T) {
		p := newPCHFake(t)
		p.sleep(t, "120")
		t.Setenv(EnvPCH, "")
		defer func(old time.Duration) { pchMaxWait = old }(pchMaxWait)
		pchMaxWait = time.Hour
		command, interrupt := context.WithCancel(context.Background())
		rn := new(Runner)
		rn.SetBuildContext(command)
		rn.Prewarm(context.Background(), p.cfg())
		p.waitStarted(t)
		setAge(rn, time.Hour)
		got := make(chan *pchRef, 1)
		go func() { got <- rn.pchPath(context.Background(), p.cfg(), pchStart) }()
		select {
		case <-got:
		case <-time.After(20 * time.Second):
			t.Error("Prewarm waited for the build")
		}
		interrupt()
		rn.WaitPCH(context.Background())
	})
}

// The in-process check: a header that a library header makes stale while the Runner lives is made again, not
// loaded from the Runner's memory.
func TestPCHInProcessStalenessMakesTheHeaderAgain(t *testing.T) {
	p := newPCHFake(t)
	ResetCacheCounters()
	rn := new(Runner)
	rn.Prewarm(context.Background(), p.cfg())
	rn.WaitPCH(context.Background())
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Fatalf("%d builds", c.PCHBuilds)
	}
	if ref := rn.pchPath(context.Background(), p.cfg(), pchUse); ref == nil {
		t.Fatal("no header from the build that is done")
	}
	p.header(t, "sys.hpp", "// the library header changed\n")
	ref := rn.pchPath(context.Background(), p.cfg(), pchUse)
	rn.WaitPCH(context.Background())
	if c := CacheCounters(); c.PCHBuilds != 2 {
		t.Errorf("%d builds in all, want 2 (ref %v)", c.PCHBuilds, ref != nil)
	}
}

// blockedInfoCompiler keeps -dI alive until released, well beyond the tests' failure bounds.
func blockedInfoCompiler(t *testing.T) (*fake, Config, string, string, string) {
	t.Helper()
	f := newFake(t)
	f.header(t, "lib.hpp", "// lib\n")
	markers := t.TempDir()
	blocked := filepath.Join(markers, "blocked")
	release := filepath.Join(markers, "release")
	t.Setenv("FAKE_DI_BLOCKED", blocked)
	t.Setenv("FAKE_DI_RELEASE", release)
	cfg := f.cfg()
	cfg.Timeout = time.Minute
	return f, cfg, "// title\n#include \"lib.hpp\"\n", blocked, release
}

func waitInfoBlocked(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the includes compiler did not reach the barrier")
}

func awaitIncludeInfo(t *testing.T, done <-chan *includeInfo) *includeInfo {
	t.Helper()
	select {
	case info := <-done:
		return info
	case <-time.After(5 * time.Second):
		t.Fatal("includeInfo did not return before the blocked compiler's timeout")
		return nil
	}
}

// includeWaitContext signals when includeInfo reaches its select, rather than racing its entry.
type includeWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *includeWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func waitIncludeSelect(t *testing.T, ctx *includeWaitContext) {
	t.Helper()
	select {
	case <-ctx.waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("includeInfo did not start waiting for the blocked compiler")
	}
}

// Successful waiters share immutable scanner output, not caller-dependent completion state.
func TestIncludeInfoConcurrentLiveWaiters(t *testing.T) {
	f, cfg, text, blocked, release := blockedInfoCompiler(t)
	rn := new(Runner)
	const count = 8
	waiting := make([]*includeWaitContext, count)
	var shared *prefetch
	for i := range count {
		ctx, end := rn.prefetchInfo(context.Background(), cfg, text)
		defer end()
		pend := prefetchedInfo(ctx, infoText(text))
		if shared == nil {
			shared = pend.prefetch
		} else if pend.prefetch != shared {
			t.Fatal("the live callers did not share a prefetch")
		}
		waiting[i] = &includeWaitContext{Context: ctx, waiting: make(chan struct{})}
	}
	waitInfoBlocked(t, blocked)
	done := make(chan *includeInfo, count)
	read := []string{filepath.Join(f.inc, "lib.hpp")}
	for i := range count {
		go func() { done <- rn.includeInfo(waiting[i], cfg, text, read, false, "") }()
	}
	for i := range count {
		waitIncludeSelect(t, waiting[i])
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	answers := make(map[*includeInfo]bool)
	for range count {
		info := awaitIncludeInfo(t, done)
		if !info.complete || len(info.entered) != 1 || info.entered[0] != read[0] {
			t.Errorf("the live caller lost its answer: %+v", info)
		}
		if info == &shared.sc.info || answers[info] {
			t.Error("callers shared mutable completion state")
		}
		answers[info] = true
	}
	if shared.sc.info.complete {
		t.Error("a caller changed the published scanner's completion state")
	}
	if f.count("D") != 1 {
		t.Errorf("%d includes questions, want one shared run", f.count("D"))
	}
}

// A canceled last waiter releases its reference and stops the detached auxiliary compiler.
func TestIncludeInfoCancellationReleasesLastWaiter(t *testing.T) {
	f, cfg, text, blocked, _ := blockedInfoCompiler(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rn := new(Runner)
	ictx, end := rn.prefetchInfo(ctx, cfg, text)
	defer end()
	pend := prefetchedInfo(ictx, infoText(text))
	waitInfoBlocked(t, blocked)
	waiting := &includeWaitContext{Context: ictx, waiting: make(chan struct{})}
	done := make(chan *includeInfo, 1)
	go func() {
		done <- rn.includeInfo(waiting, cfg, text, []string{filepath.Join(f.inc, "lib.hpp")}, false, "")
	}()
	waitIncludeSelect(t, waiting)
	cancel()
	if info := awaitIncludeInfo(t, done); info.complete {
		t.Fatal("the canceled question returned a complete answer")
	}
	select {
	case <-pend.done:
		if pend.err == nil {
			t.Error("the blocked compiler was not canceled")
		}
	default:
		t.Error("the last waiter left its compiler running")
	}
	rn.mu.Lock()
	refs := pend.refs
	kept := len(rn.infoMemo)
	pending := len(rn.prefetches)
	rn.mu.Unlock()
	if refs != 0 || kept != 0 || pending != 0 {
		t.Errorf("after cancellation: %d references, %d cached answers, %d prefetches", refs, kept, pending)
	}
	end() // releasing again must not decrement another caller's reference
}

// Canceling one caller must not stop a shared question that still has a live waiter.
func TestIncludeInfoCancellationPreservesSharedWaiter(t *testing.T) {
	f, cfg, text, blocked, release := blockedInfoCompiler(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rn := new(Runner)
	first, endFirst := rn.prefetchInfo(ctx, cfg, text)
	defer endFirst()
	second, endSecond := rn.prefetchInfo(context.Background(), cfg, text)
	defer endSecond()
	pend := prefetchedInfo(first, infoText(text))
	if prefetchedInfo(second, infoText(text)).prefetch != pend.prefetch {
		t.Fatal("the callers did not share a prefetch")
	}
	waitInfoBlocked(t, blocked)
	read := []string{filepath.Join(f.inc, "lib.hpp")}
	waiting := &includeWaitContext{Context: first, waiting: make(chan struct{})}
	canceled := make(chan *includeInfo, 1)
	live := make(chan *includeInfo, 1)
	go func() { canceled <- rn.includeInfo(waiting, cfg, text, read, false, "") }()
	go func() { live <- rn.includeInfo(second, cfg, text, read, false, "") }()
	waitIncludeSelect(t, waiting)
	cancel()
	if info := awaitIncludeInfo(t, canceled); info.complete {
		t.Fatal("the canceled caller received a complete answer")
	}
	endFirst()
	rn.mu.Lock()
	refs := pend.refs
	kept := len(rn.infoMemo)
	rn.mu.Unlock()
	if refs != 1 || kept != 0 {
		t.Errorf("after first cancellation: %d references, %d cached answers", refs, kept)
	}
	select {
	case <-pend.done:
		t.Fatal("canceling one caller stopped the shared compiler")
	default:
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if info := awaitIncludeInfo(t, live); !info.complete || len(info.entered) != 1 {
		t.Fatalf("the live caller lost the shared answer: %+v", info)
	}
	if f.count("D") != 1 {
		t.Errorf("%d includes questions, want one shared run", f.count("D"))
	}
	endSecond()
	rn.mu.Lock()
	refs = pend.refs
	kept = len(rn.infoMemo)
	pending := len(rn.prefetches)
	rn.mu.Unlock()
	if refs != 0 || kept != 1 || pending != 0 {
		t.Errorf("after completion: %d references, %d cached answers, %d prefetches", refs, kept, pending)
	}
}

// Nothing is memorized from a question that was asked before a header changed: the answer is of the old
// contents and the key has the new.
func TestIncludeInfoIsNotKeptWhenAHeaderChangedUnderIt(t *testing.T) {
	for _, edit := range []bool{false, true} {
		name := "unchanged"
		if edit {
			name = "edited_after_the_question_started"
		}
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			f.header(t, "lib.hpp", "// lib\n")
			read := []string{filepath.Join(f.inc, "lib.hpp")}
			text := "// title\n#include \"lib.hpp\"\n"
			rn := new(Runner)
			ictx, end := rn.prefetchInfo(context.Background(), f.cfg(), text)
			if edit {
				// (the headers the question saw are those before its start: a change after is not seen)
				later := time.Now().Add(time.Second)
				os.Chtimes(read[0], later, later)
			}
			rn.includeInfo(ictx, f.cfg(), text, read, false, "")
			end()
			kcfg := f.cfg()
			kcfg.MaxDecl, kcfg.MaxJSON = 0, 0
			kept := rn.hasInfo(PreKey(kcfg, &Source{Text: "#include \"lib.hpp\"\n"}))
			if kept == edit {
				t.Errorf("kept %v for a header that was edited: %v", kept, edit)
			}
		})
	}
}

// The files of a header that is in the cache are used only when they are whole.
func TestLookPCHChecksTheFiles(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib.hpp")
	if err := os.WriteFile(lib, []byte("// lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(t *testing.T, mutate func(*pchFile)) string {
		base := filepath.Join(t.TempDir(), "ab", "cdef")
		if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
			t.Fatal(err)
		}
		pf := pchFile{
			Format: 1, Created: 1, Read: []string{lib}, Deps: []string{lib}, DepsHash: DepsHash([]string{lib}),
			Size: 12,
		}
		if mutate != nil {
			mutate(&pf)
		}
		body, err := json.Marshal(pf)
		if err != nil {
			t.Fatal(err)
		}
		if !writeEntry(base+".json", body) {
			t.Fatal("no meta file")
		}
		for _, ext := range []string{".pch", ".hpp"} {
			if err := os.WriteFile(base+ext, []byte("twelve bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return base
	}
	cases := []struct {
		name   string
		mutate func(*pchFile)
		after  func(base string)
		want   bool
	}{
		{"whole", nil, nil, true},
		{"cut_off_pch", nil, func(b string) { os.WriteFile(b+".pch", []byte("twelve"), 0o644) }, false},
		{"grown_pch", nil, func(b string) { os.WriteFile(b+".pch", []byte("twelve bytes and more"), 0o644) }, false},
		{"no_size", func(pf *pchFile) { pf.Size = 0 }, nil, false},
		{"no_source", nil, func(b string) { os.Remove(b + ".hpp") }, false},
		{"no_pch", nil, func(b string) { os.Remove(b + ".pch") }, false},
		{"files_changed", func(pf *pchFile) { pf.DepsHash = "other" }, nil, false},
		{"nothing_read", func(pf *pchFile) { pf.Read = nil }, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := write(t, c.mutate)
			if c.after != nil {
				c.after(base)
			}
			if _, _, _, ok := lookPCH(base); ok != c.want {
				t.Errorf("lookPCH = %v, want %v", ok, c.want)
			}
		})
	}
}

// What a killed build leaves is removed once it is old; what a running build in another process writes is not.
func TestPrunePCHRemovesWhatAKilledBuildLeft(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "ab")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	put := func(name string, mod time.Time) {
		p := filepath.Join(folder, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, mod, mod)
	}
	now := time.Now()
	put(".tmp-old.pch", old)
	put(".tmp-old.hpp", old)
	put(".tmp-new.pch", now)
	put("orphan-old.pch", old)
	put("orphan-old.hpp", old)
	put("orphan-new.pch", now)
	put("kept.pch", old) // (the one being made: keepBase)
	put("kept.hpp", old)
	put("whole.pch", old)
	put("whole.hpp", old)
	put("whole.json", old)
	prunePCH(root, 4, filepath.Join(folder, "kept"))
	left, _ := filepath.Glob(filepath.Join(folder, "*"))
	var names []string
	for _, p := range left {
		names = append(names, filepath.Base(p))
	}
	want := ".tmp-new.pch kept.hpp kept.pch orphan-new.pch whole.hpp whole.json whole.pch"
	sort.Strings(names)
	if got := strings.Join(names, " "); got != want {
		t.Errorf("left %q", got)
	}
}

// The source of the header is written once: the same text is not written again (a process that loaded the
// header names the file), and another text replaces it whole.
func TestWriteSourceKeepsTheFileThatIsRight(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "block.hpp")
	if err := writeSource(path, "one\n"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	os.Chtimes(path, old, old)
	if err := writeSource(path, "one\n"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().After(old.Add(time.Minute)) {
		t.Error("a file in use was not touched")
	}
	if err := writeSource(path, "two\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "two\n" {
		t.Errorf("contents %q", b)
	}
	if entries, _ := os.ReadDir(folder); len(entries) != 1 {
		t.Errorf("%d files in the folder", len(entries))
	}
}

// A search path that is spelled relative to the working directory means no header: the build runs in another
// folder than the runs do.
func TestRelativeSearchPathMeansNoHeader(t *testing.T) {
	abs := t.TempDir()
	cases := []struct {
		name  string
		env   map[string]string
		flags []string
		want  bool
	}{
		{"none", nil, nil, false},
		{"absolute_flags", nil, []string{"-I", abs, "-isystem" + abs, "-DX=1", "-std=c++20"}, false},
		{"relative_I_joined", nil, []string{"-Iinclude"}, true},
		{"relative_I_apart", nil, []string{"-I", "include"}, true},
		{"relative_isystem", nil, []string{"-isystem", "vendor"}, true},
		{"relative_include", nil, []string{"-include", "pre.h"}, true},
		{"relative_sysroot", nil, []string{"--sysroot=root"}, true},
		{"include_pch_is_not_an_include", nil, []string{"-include-pch", "x"}, false},
		{"absolute_CPATH", map[string]string{"CPATH": abs}, nil, false},
		{"relative_CPATH", map[string]string{"CPATH": abs + string(os.PathListSeparator) + "inc"}, nil, true},
		{"empty_entry_CPATH", map[string]string{"CPATH": abs + string(os.PathListSeparator)}, nil, true},
		{"relative_CPLUS", map[string]string{"CPLUS_INCLUDE_PATH": "inc"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, name := range searchPathVariables {
				t.Setenv(name, c.env[name])
			}
			cfg := Config{Flags: c.flags}
			if got := relativeSearchPath(cfg); got != c.want {
				t.Errorf("relativeSearchPath = %v, want %v", got, c.want)
			}
		})
	}
}

// A warm call of Decls asks the compiler for nothing, including auxiliary questions.
func TestWarmTypeRunsAskNoCompiler(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// a\n")
	t.Setenv(EnvPCH, "")
	dc := DeclConfig{Includes: []string{`"a.hpp"`}}
	queries := []DeclQuery{{DeclType, "Thing"}}
	if _, err := new(Runner).Decls(context.Background(), f.cfg(), dc, queries); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(f.counter)
	rn := new(Runner)
	decls, err := rn.Decls(context.Background(), f.cfg(), dc, queries)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(f.counter)
	if string(after) != string(before) {
		t.Errorf("a warm call ran the compiler:\n%s", strings.TrimPrefix(string(after), string(before)))
	}
	if !decls.Cached || rn.DeclRuns() != 0 {
		t.Errorf("cached %v, %d runs", decls.Cached, rn.DeclRuns())
	}
}

// The header is for one target: the compiler's target system and flags are in its key.
func TestPCHKeyHasTheTarget(t *testing.T) {
	base := Config{}
	base.CXX.Path = "clang++"
	same := pchKey(base, toolchain{})
	windows := base
	windows.CXX.GOOS, windows.CXX.GOARCH = "windows", "amd64"
	if pchKey(windows, toolchain{}) == same {
		t.Error("the key of the Windows target is the host's")
	}
	flags := base
	flags.CXX.Args = []string{"--target=x86_64-w64-mingw32"}
	if pchKey(flags, toolchain{}) == same {
		t.Error("the key does not have the compiler's own arguments")
	}
	if pchKey(base, toolchain{}) != same {
		t.Error("the key changes between calls")
	}
}
