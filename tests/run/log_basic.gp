// phase: 09
// ExitCode: 1
// log: Logger with prefixes and flags (without the time and the file, which differ between runs and between Go and gocpp), SetOutput, Default, Writer, Output, Panic, the standard logger's flags, concurrent writers.

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

func main() {
	var buf bytes.Buffer
	l := log.New(&buf, "app: ", 0)
	l.Println("hello", 42)
	l.Printf("x=%d y=%q", 1, "two")
	l.Print("no newline")
	l.Print("with newline\n")
	l.SetPrefix("[p] ")
	l.Println("changed prefix")
	l.SetFlags(log.Lmsgprefix)
	l.Println("message prefix")
	fmt.Print(buf.String())
	fmt.Println(l.Prefix(), l.Flags() == log.Lmsgprefix, log.LstdFlags, log.Ldate|log.Ltime)

	buf.Reset()
	l = log.New(&buf, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC)
	l.Print("stamp")
	line := buf.String()
	fmt.Println(len(line), line[4] == '/' && line[7] == '/' && line[10] == ' ' && line[13] == ':' && line[16] == ':' && line[19] == '.', strings.HasSuffix(line, " stamp\n"))

	buf.Reset()
	l = log.New(&buf, "x ", log.Llongfile|log.Lshortfile)
	l.Print("file")
	fmt.Println(strings.HasPrefix(buf.String(), "x "), strings.Contains(buf.String(), ":"), strings.HasSuffix(buf.String(), ": file\n"))

	buf.Reset()
	l = log.New(&buf, "out ", 0)
	fmt.Println(l.Output(1, "via Output"), buf.String() == "out via Output\n")

	func() {
		defer func() { fmt.Printf("recovered: %v\n", recover()) }()
		l.Panicf("bad %s", "thing")
	}()
	func() {
		defer func() { fmt.Printf("recovered: %v\n", recover()) }()
		l.Panicln("bad", "line")
	}()
	fmt.Printf("%q\n", buf.String())

	// The standard logger.
	buf.Reset()
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("std: ")
	log.Println("standard logger")
	log.Printf("%05.1f", 3.14159)
	fmt.Printf("%q %q %d\n", buf.String(), log.Prefix(), log.Flags())
	fmt.Printf("%v %q\n", log.Writer() == &buf, log.Default().Prefix())
	log.SetOutput(os.Stdout)
	log.Print("to stdout")
	l2 := log.New(os.Stdout, "", 0)
	l2.SetOutput(&buf)
	buf.Reset()
	l2.Print("rerouted")
	fmt.Printf("%q\n", buf.String())

	// Writers that fail do not stop the program.
	l3 := log.New(failWriter{}, "", 0)
	l3.Print("lost")

	// Concurrent writers: every line is whole.
	buf.Reset()
	l4 := log.New(&syncBuf{b: &buf}, "", 0)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				l4.Printf("goroutine %d line %d", g, i)
			}
		}()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	whole := 0
	for _, ln := range lines {
		if f := strings.Fields(ln); len(f) == 4 && f[0] == "goroutine" && f[2] == "line" {
			whole++
		}
	}
	fmt.Println(len(lines), whole)

	log.SetFlags(0)
	log.SetPrefix("fatal: ")
	defer fmt.Println("deferred calls do not run after Fatal")
	log.Fatalln("exiting with status 1")
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("closed") }

type syncBuf struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// Output:
// app: hello 42
// app: x=1 y="two"
// app: no newline
// app: with newline
// [p] changed prefix
// [p] message prefix
// [p]  true 3 3
// 33 true true
// true true true
// <nil> true
// recovered: bad thing
// recovered: bad line
//
// "out via Output\nout bad thing\nout bad line\n"
// "std: standard logger\nstd: 003.1\n" "std: " 0
// true "std: "
// std: to stdout
// "rerouted\n"
// 400 400
// fatal: exiting with status 1
