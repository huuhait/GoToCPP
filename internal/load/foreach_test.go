package load

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
)

// forEachFixture returns n files with trees of different sizes.
func forEachFixture(n int) []*File {
	files := make([]*File, n)
	for i := range files {
		files[i] = &File{AST: &ast.File{FileStart: 1, FileEnd: token.Pos(1 + 100*(i+1))}}
	}
	return files
}

func TestForEachFileErrorOrder(t *testing.T) {
	files := forEachFixture(9)
	record := func(i int, f *File, errs *diag.List) {
		for round := range 1 + i%3 {
			errs.Errorf(token.Position{Filename: fmt.Sprint("f", i)}, "error %d of file %d", round, i)
		}
	}
	var serial, parallel diag.List
	forEachFileWith(files, &serial, 1, 0, record)
	forEachFileWith(files, &parallel, 4, 0, record)
	if !reflect.DeepEqual(serial, parallel) {
		t.Fatalf("the errors of the parallel run differ from the serial ones:\nserial:   %v\nparallel: %v",
			serial.Diagnostics(), parallel.Diagnostics())
	}
	if len(serial.Diagnostics()) != 3*(1+2+3) {
		t.Fatalf("%d errors, want %d", len(serial.Diagnostics()), 3*(1+2+3))
	}
}

func TestForEachFileBelowThresholdIsSerial(t *testing.T) {
	files := forEachFixture(4) // 100 + 200 + 300 + 400 = 1000 bytes
	var running, most atomic.Int32
	var errs diag.List
	forEachFileWith(files, &errs, 4, 1001, func(int, *File, *diag.List) {
		now := running.Add(1)
		most.Store(max(most.Load(), now))
		time.Sleep(time.Millisecond)
		running.Add(-1)
	})
	if most.Load() != 1 {
		t.Fatalf("%d files ran at once below the threshold", most.Load())
	}
}

func TestForEachFileRunsTogether(t *testing.T) {
	files := forEachFixture(4)
	entered := make(chan struct{}, len(files))
	release := make(chan struct{})
	observed := make(chan bool, 1)
	go func() {
		defer close(release)
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for range 2 {
			select {
			case <-entered:
			case <-timer.C:
				observed <- false
				return
			}
		}
		observed <- true
	}()
	var errs diag.List
	forEachFileWith(files, &errs, 4, 0, func(int, *File, *diag.List) {
		entered <- struct{}{}
		<-release
	})
	if !<-observed {
		t.Fatal("two files were never in flight at once")
	}
}

func TestForEachFilePanic(t *testing.T) {
	files := forEachFixture(6)
	var finished atomic.Int32
	var caught any
	func() {
		defer func() { caught = recover() }()
		var errs diag.List
		forEachFileWith(files, &errs, 3, 0, func(i int, f *File, errs *diag.List) {
			defer finished.Add(1)
			if i == 1 || i == 4 {
				panic(fmt.Sprint("panic of file ", i))
			}
		})
	}()
	if caught != "panic of file 1" {
		t.Fatalf("the caller recovered %v, want the panic of the first file", caught)
	}
	if finished.Load() != int32(len(files)) {
		t.Fatalf("%d of %d files finished before the panic left the caller", finished.Load(), len(files))
	}
}

func TestForEachFileNoRace(t *testing.T) {
	files := forEachFixture(16)
	var mu sync.Mutex
	seen := map[int]bool{}
	var errs diag.List
	forEachFileWith(files, &errs, 8, 0, func(i int, f *File, errs *diag.List) {
		mu.Lock()
		seen[i] = true
		mu.Unlock()
		errs.Errorf(token.Position{}, "file %d", i)
	})
	if len(seen) != len(files) {
		t.Fatalf("%d files visited, want %d", len(seen), len(files))
	}
}
