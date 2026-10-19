package build

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The trimming of the object cache (trim) removes the oldest files, whoever uses them: a build
// that looked an entry up and is about to copy its object out, or that is storing one, must see a
// miss at worst, never an error or half an object.

func TestObjCacheEvictedBetweenLookupAndCopy(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		inc := t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		good := objCompile(t, cxx, cache, objTU, "-I", inc)
		if good.err != nil {
			t.Fatal(good.err)
		}
		for _, what := range []string{"the object", "everything"} {
			// The lookup finds the entry valid; then the trimming of another build removes files.
			testHookBeforeCopyOut = func(oc *objCache, e objEntry) {
				switch what {
				case "the object":
					rmFile(oc.objectPath(e.Obj))
				default:
					oc.trim(1) // removes every file
				}
			}
			r := objCompile(t, cxx, cache, objTU, "-I", inc)
			testHookBeforeCopyOut = nil
			if r.err != nil || !r.ran || r.hit() || !bytes.Equal(r.obj, good.obj) {
				t.Errorf("evicted (%s) between the lookup and the copy: ran=%v hit=%v err=%v, want a compile that works", what, r.ran, r.hit(), r.err)
			}
			if r := objCompile(t, cxx, cache, objTU, "-I", inc); r.err != nil || r.ran || !r.hit() || !bytes.Equal(r.obj, good.obj) {
				t.Errorf("evicted (%s): the cache was not filled again: ran=%v hit=%v err=%v", what, r.ran, r.hit(), r.err)
			}
		}
	})
}

// Compiles of one unit while another goroutine trims the cache to nothing all the time: every one
// works and has the right object (the count of compiles is fixed: no timing is looked at).
func TestObjCacheTrimRacesWithBuilds(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		inc := t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		good := objCompile(t, cxx, cache, objTU, "-I", inc)
		if good.err != nil {
			t.Fatal(good.err)
		}
		oc := &objCache{dir: filepath.Join(cache, "obj"), max: 1}
		stop := make(chan struct{})
		var trims sync.WaitGroup
		trims.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					oc.trim(1)
				}
			}
		})
		const workers, rounds = 4, 6
		errs := make([]string, workers*rounds)
		var wg sync.WaitGroup
		for w := range workers {
			wg.Go(func() {
				for i := range rounds {
					r := objCompile(t, cxx, cache, objTU, "-I", inc)
					switch {
					case r.err != nil:
						errs[w*rounds+i] = r.err.Error()
					case !bytes.Equal(r.obj, good.obj):
						errs[w*rounds+i] = "another object"
					}
				}
			})
		}
		wg.Wait()
		close(stop)
		trims.Wait()
		for i, e := range errs {
			if e != "" {
				t.Errorf("compile %d while the cache is trimmed all the time: %s", i, e)
			}
		}
	})
}

func rmFile(p string) { os.Remove(p) }
