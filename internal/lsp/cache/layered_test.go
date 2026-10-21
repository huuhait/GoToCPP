package cache

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// tableOf returns a table with n files /p/f0 ... /p/f(n-1) in its base.
func tableOf(n int) *fileTable {
	base := make(map[string]*fileHandle, n)
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("/p/f%d", i)
		base[path] = &fileHandle{path: path}
	}
	return &fileTable{base: base}
}

// fileTable is the layered map of the files of a state, as the tests use it.
type fileTable = layered[string, *fileHandle]

// tget returns the file with the name path in the table, or nil.
func tget(t *fileTable, path string) *fileHandle {
	f, _ := t.get(path)
	return f
}

// neverDrop is the drop function of a clone that keeps every file.
func neverDrop(string, *fileHandle) bool { return false }

func baseOf(t *fileTable) uintptr { return reflect.ValueOf(t.base).Pointer() }

func TestFileTableGetPutSize(t *testing.T) {
	table := tableOf(3)
	if f := tget(table, "/p/f1"); f == nil || f.path != "/p/f1" {
		t.Errorf("get of a file of base: %v", f)
	}
	if f := tget(table, "/p/other"); f != nil {
		t.Errorf("get of a file that is not there: %v", f)
	}
	added := &fileHandle{path: "/p/other"}
	table.put(added.path, added)
	if tget(table, added.path) != added {
		t.Error("the file that was put is not found")
	}
	if got := table.size(); got != 4 {
		t.Errorf("size %d, want 4", got)
	}
}

func TestFileTableCloneDropsAndKeeps(t *testing.T) {
	table := tableOf(10)
	added := &fileHandle{path: "/p/added"}
	table.put(added.path, added)
	dropped := map[string]bool{"/p/f3": true, "/p/added": true}
	clone := table.clone(func(path string, _ *fileHandle) bool { return dropped[path] })

	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("/p/f%d", i)
		if got := tget(clone, path); (got == nil) != dropped[path] {
			t.Errorf("tget(clone, %s) = %v, dropped %v", path, got, dropped[path])
		}
	}
	if tget(clone, "/p/added") != nil {
		t.Error("a dropped file of own is still in the clone")
	}
	if got := clone.size(); got != 9 {
		t.Errorf("clone size %d, want 9", got)
	}

	// The table the clone was made from is as it was.
	if tget(table, "/p/f3") == nil || tget(table, "/p/added") != added || table.size() != 11 {
		t.Errorf("the source table changed: size %d", table.size())
	}
}

// The clone of a table of 1,500 files that drops one copies none of them: it shares the base
// and holds the one name that is hidden (a copy of the table is about 45,000 bytes of garbage
// for every edit).
func TestFileTableCloneSharesBase(t *testing.T) {
	table := tableOf(1500)
	clone := table.clone(func(path string, _ *fileHandle) bool { return path == "/p/f7" })
	if baseOf(clone) != baseOf(table) {
		t.Error("the clone copied the base")
	}
	if len(clone.own) != 0 || len(clone.hidden) != 1 {
		t.Errorf("the clone holds %d own files and %d hidden names, want 0 and 1", len(clone.own), len(clone.hidden))
	}
	if tget(clone, "/p/f7") != nil || tget(clone, "/p/f8") == nil {
		t.Error("the clone has the wrong files")
	}
}

// What a clone adds is not in the table it came from, and the other way round: the requests of
// the old snapshot go on reading files while the new one does.
func TestFileTablesDoNotShareAdditions(t *testing.T) {
	table := tableOf(5)
	clone := table.clone(neverDrop)
	inOld := &fileHandle{path: "/p/old"}
	inNew := &fileHandle{path: "/p/new"}
	table.put(inOld.path, inOld)
	clone.put(inNew.path, inNew)
	if tget(clone, inOld.path) != nil {
		t.Error("the clone sees a file that the old table read after the clone")
	}
	if tget(table, inNew.path) != nil {
		t.Error("the old table sees a file that the clone read")
	}
}

// A file that is dropped and read again is the new file, in the clone and in its clones.
func TestFileTableDroppedFileReadAgain(t *testing.T) {
	table := tableOf(50)
	clone := table.clone(func(path string, _ *fileHandle) bool { return path == "/p/f9" })
	again := &fileHandle{path: "/p/f9", src: []byte("new")}
	if tget(clone, "/p/f9") != nil {
		t.Fatal("the dropped file is still there")
	}
	clone.put("/p/f9", again)
	next := clone.clone(neverDrop)
	if tget(next, "/p/f9") != again {
		t.Error("the clone of a clone lost the file that was read again")
	}
	if tget(table, "/p/f9") == again || tget(table, "/p/f9") == nil {
		t.Error("the file read again leaked into the source table")
	}
	last := next.clone(func(path string, _ *fileHandle) bool { return path == "/p/f9" })
	if tget(last, "/p/f9") != nil {
		t.Error("a file read again is not dropped by the next clone")
	}
	if last.size() != 49 {
		t.Errorf("size %d, want 49", last.size())
	}
}

// The table behaves as a plain map through many clones, drops and additions, and compacts.
func TestFileTableFollowsAModel(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	table := tableOf(300)
	model := map[string]*fileHandle{}
	table.each(func(path string, f *fileHandle) { model[path] = f })
	flattened := 0
	for step := 0; step < 400; step++ {
		switch rng.Intn(3) {
		case 0: // read files that are missing
			for i := 0; i < rng.Intn(5); i++ {
				path := fmt.Sprintf("/p/f%d", rng.Intn(400))
				if tget(table, path) == nil {
					f := &fileHandle{path: path}
					table.put(path, f)
					model[path] = f
				}
			}
		default: // clone, dropping some
			prefix := fmt.Sprintf("/p/f%d", rng.Intn(40))
			before := baseOf(table)
			table = table.clone(func(path string, _ *fileHandle) bool {
				return strings.HasPrefix(path, prefix)
			})
			for path := range model {
				if strings.HasPrefix(path, prefix) {
					delete(model, path)
				}
			}
			if baseOf(table) != before {
				flattened++
			}
		}
		if table.size() != len(model) {
			t.Fatalf("step %d: size %d, want %d", step, table.size(), len(model))
		}
		for path, f := range model {
			if tget(table, path) != f {
				t.Fatalf("step %d: %s is not the file of the model", step, path)
			}
		}
		count := 0
		table.each(func(path string, f *fileHandle) {
			count++
			if model[path] != f {
				t.Fatalf("step %d: each gave %s, which the model does not have", step, path)
			}
		})
		if count != len(model) {
			t.Fatalf("step %d: each gave %d files, want %d", step, count, len(model))
		}
	}
	if flattened == 0 {
		t.Error("the table was never compacted")
	}
}
